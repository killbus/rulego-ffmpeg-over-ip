# Native composition audit

## Decision

`ffmpegOverIpProducer` plus RuleGo's configured static-file mapping is a useful
composition, but it does **not** satisfy the Derived Resource Origin contract by
itself. It can produce closed files and let the existing RuleGo HTTP server
serve those files with standard byte-range behavior. It does not own a durable
resource identity or state machine, and it cannot distinguish pending, failed,
expired, and unknown resources across rule/node/process lifetimes.

The smallest coherent boundary is therefore:

1. keep `ffmpegOverIp` and `ffmpegOverIpProducer` as transformation providers;
2. keep RuleGo's existing REST/static service as the ready-byte data plane;
3. add a source-neutral resource-origin capability that owns the publication
   catalog, readiness barrier, lifecycle limits, reconciliation, and status
   projection; and
4. add no second HTTP listener or duplicate Range implementation.

That origin belongs in a separate generic RuleGo plugin/component, not in this
FFmpeg transport plugin. FFmpeg is only one producer, while the missing
contract applies equally to any process or node that publishes files.

## Exact source inspected

- Plugin checkout: current `rulego-ffmpeg-over-ip` worktree.
- Released plugin host: RuleGo Server `v0.37.0`, source revision
  `a7be24c4c1f649d422b41eb623d1d6e314b20c58`.
- Host's pinned RuleGo Core module:
  `v0.36.1-0.20260802040353-2ec085f29027`.
- Static router dependency: `github.com/julienschmidt/httprouter v1.3.0`.

The later local RuleGo `main` checkout contains a different static handler
with explicit `GET` and `HEAD` registration. It is not evidence for the
currently released `v0.37.0-plugin` runtime and is deliberately excluded from
the current-capability conclusion.

## What composes today

### Transformation and closed-file readiness

`producer.go` already supplies useful provider-level mechanics:

- `producerRequest` carries a caller key, exact invocation, awaited path,
  finite run timeout, and cache TTL (`producer.go:28-35`).
- `currentJob` and `ensureJob` singleflight identical fingerprints within one
  node instance; a different fingerprint under the same key cancels and
  replaces the old job (`producer.go:200-234`).
- multiple waiters share a job, and cancellation only stops the job when the
  last waiting request aborts (`producer.go:314-348`).
- the protocol callback records every file closed by the remote process and
  wakes waiters for the requested member (`producer.go:237-283`).
- the file protocol marks a written path ready only after a successful close;
  a rename carries readiness to the new path (`client/files.go:192-211`,
  `client/files.go:260-283`).
- `awaitOnly: true` lets the graph wait for a completed file without copying it
  through the RuleGo message stream (`producer.go:128-140`).

This is enough for a transformation provider to say "this named member was
closed". It is not yet a public-resource state transition.

### Existing static data plane

The released server reads `resource_mapping` and calls
`RegisterStaticFiles` (`server/internal/endpoint/server.go:187-190` at the
server revision above). The pinned core implementation maps a URL prefix to an
`http.Dir` through `httprouter.Router.ServeFiles`:

- [RuleGo `RegisterStaticFiles`](https://github.com/rulego/rulego/blob/2ec085f2902777f10d0f867fa4295b11629257ad/endpoint/rest/rest.go#L1248-L1273)
- [`httprouter.ServeFiles`](https://github.com/julienschmidt/httprouter/blob/v1.3.0/router.go#L286-L306)

`ServeFiles` delegates to Go `http.FileServer`, whose regular-file path reaches
`serveContent`. Consequently, a ready immutable file served through a mapped
directory already gets correct full `GET`, satisfiable `206` ranges,
unsatisfiable `416` with `Content-Range: bytes */size`, `Accept-Ranges`, accurate
lengths, `Last-Modified`, and conditional `If-Modified-Since` handling. There
is no reason to reproduce that byte-serving logic in this plugin.

There is one released-host gap: `httprouter.ServeFiles` registers only `GET`.
The exact `v0.37.0-plugin` runtime therefore has no matching static `HEAD`
route and normally returns `405`, even though `http.FileServer` would handle
HEAD correctly if invoked. This is a small host-owned route-registration fix,
not justification for another server.

A viable current composition for a ready single file is therefore:

```text
producer writes and closes file under a dedicated publish root
    -> origin atomically marks the resource ready and discloses its URL
    -> RuleGo resource_mapping serves the immutable path
```

The static route must not be treated as the publication signal. A path can
exist while FFmpeg is still writing it; knowing or guessing that path bypasses
the producer's close-based readiness check.

## What does not compose today

| PRD property | Current evidence | Result |
| --- | --- | --- |
| Stable resource identity and state | `key` and `jobs` exist only inside one `ffmpegOverIPProducerNode`; static serving knows only paths | Missing |
| Pending/ready/failed/expired outcomes | producer waiters receive RuleGo relations; static misses are all HTTP 404 | Missing |
| Rule hot-deploy/process restart | `Destroy` cancels node jobs; job maps and `time.AfterFunc` cleanup timers are in memory | Missing |
| Startup orphan reconciliation | no on-disk ownership catalog or startup scan exists | Missing |
| Storage and inactivity bounds | request has run timeout and post-run TTL only; no byte quota or inactivity policy exists | Missing |
| Strict retention bound | TTL starts after `client.Run` returns, so already-ready members can live for runtime plus TTL | Incomplete |
| Cross-instance reuse | fingerprint singleflight is scoped to one node's `jobs` map | Incomplete |
| Safe publication | file close is detected, but mapped paths can be fetched before the origin publishes them | Missing publication barrier |
| Resource set | all closed paths are observed, but no durable set membership or atomic manifest/member commit exists | Missing |
| Ready-file Range | native `FileServer`/`ServeContent` supplies GET and Range semantics | Satisfied for immutable files |
| Ready-file HEAD | released static router registers GET only | Missing in current host |
| Sequential response | `ffmpegOverIp` plus `ffmpegOverIpResponse` streams/flushed stdout | Already satisfied; keep separate |

The current cleanup is intentionally conservative: after the remote process
ends, it snapshots size and mtime and later removes only unchanged files
(`producer.go:252-297`). That prevents deletion of a replacement, but a process
restart loses the timer and leaves files permanently. It also has no dedicated
root boundary: invocation paths are used with the RuleGo process's filesystem
permissions. A generic origin must own one configured root and reconcile only
artifacts carrying its own durable ownership record.

## Smallest missing behavior

The missing unit is a persistent resource catalog plus publication protocol,
not a media session and not a byte server. Its minimum responsibilities are:

- derive/return one stable resource ID independently of a RuleGo node instance;
- persist `pending | ready | failed | expired`, expiry and owned relative
  member paths under one configured root;
- singleflight publication by canonical identity across rule instances;
- publish a materialized manifest/set only after its stored members are closed
  and committed, using rename/atomic metadata replacement where possible; a
  virtual manifest may reference stable demand routes whose separately
  committed members are capped to the parent's absolute expiry;
- enforce runtime, stored-byte, retention and inactivity bounds;
- reconcile the catalog and owned root at startup, preserving valid resources
  and deleting only proven orphan artifacts;
- project deterministic status responses through an ordinary RuleGo REST rule;
  when ready, return or redirect to the native static URL; and
- expire catalog state before deleting bytes so stale files cannot revive a
  resource.

RuleGo already exposes response body, status, and header mutation in its REST
endpoint/output processor APIs, so the status/redirect projection can share
the current HTTP endpoint. The static mapping remains responsible only for
ready immutable bytes. The host needs explicit static HEAD registration (or a
host version that includes it); all other lifecycle behavior is origin-owned.

The producer should emit a provider-neutral completion descriptor to the
origin (job identity plus closed member paths and terminal result). It should
not absorb public URLs, HTTP statuses, resource-set policy, restart catalogs,
or source-specific refresh semantics. Conversely, the origin should not know
FFmpeg arguments, codecs, YouTube formats, or how the bytes were produced.

## YouTube specialization note

An empirical probe on 2026-08-27 found that `web_safari` exposed a muxed
AVC/AAC HLS VOD for `Z4tHPyZBC8g`: the master contained combined audio/video
variants through 1080p60, and the selected media playlist had
`#EXT-X-PLAYLIST-TYPE:VOD`, a complete `EXTINF` timeline, and
`#EXT-X-ENDLIST`. This proves a valuable zero-remux fast path for that source
at that time.

It does **not** establish an architectural baseline. YouTube client behavior is
moving toward web/SABR, `web_safari` HLS availability is unstable, and other
clients such as current `visionos` may expose high-resolution HTTPS/HLS inputs
without a pre-merged audio/video representation. The YouTube adapter must
therefore capability-detect this fast path, account for signed URL expiry, and
fall back to the generic separate-input publication path. No `web_safari`, HLS,
codec, or yt-dlp field belongs in the generic origin contract.

## Acceptance impact

- Native composition can cover the data-plane part of AC2 after the small
  static HEAD fix, and it preserves AC8.
- The existing producer contributes part of AC3/AC4 (closed members and
  node-local reuse) but does not complete them.
- AC1 and AC3-AC7 require the generic origin catalog/publication behavior
  listed above.
- AC9/AC10 remain specialization experiments; the opportunistic muxed-HLS
  observation is one candidate, not a substitute for the required fallback.
- AC11 resolves clearly: reuse the current RuleGo HTTP server and static
  `FileServer`; do not add a new HTTP server or custom Range implementation.
