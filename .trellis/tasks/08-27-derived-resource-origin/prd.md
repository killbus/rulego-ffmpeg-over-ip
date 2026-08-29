# Derived Resource Origin Capability

## Goal

Allow a RuleGo flow to publish the result of a transformation as an
addressable, reusable, lifecycle-bound HTTP resource or resource set. A
consumer can therefore retrieve or seek within produced output without tying
resource lifetime to one REST request, one RuleGo node instance, or one
continuously running player session.

The capability is source- and container-neutral. YouTube/yt-dlp remains a
required specialization track: if its fragmented/indexed upstream media offers
a materially better path, the implementation may exploit that structure
without turning the generic resource contract into a YouTube contract.

## Confirmed Facts

- `ffmpegOverIp` already exposes one remote process as ordered, incremental
  stdout/stderr plus one terminal result. This is the sequential-stream
  response form and needs no resource origin to work.
- `ffmpegOverIpProducer` already provides fingerprint-based singleflight,
  bounded invocation time, multi-file readiness, waiter cancellation, TTL
  cleanup, and complete-file delivery.
- Producer state is owned by an ephemeral RuleGo node instance. Files can
  outlive that owner after a crash or rule hot-deploy, while URLs and cleanup
  do not have an independent resource lifecycle.
- RuleGo's existing REST/static-file path uses Go `http.ServeContent` and
  already implements standard Range handling. The task must test composition
  with that native path before adding another byte-range server.
- The retained YouTube probe artifact is fragmented MP4
  (`ftyp`/`moov`/`moof`/`mdat`); it does not itself prove a usable `sidx`.
  Upstream index and fragment alignment must be verified per selected
  representation. Naive fixed-time stream-copy is not exact at arbitrary
  boundaries, and changing HLS segment duration did not remove the roughly
  30-second cold target-window cost.
- yt-dlp direct media URLs can expire. A durable public resource cannot assume
  that one resolved upstream URL remains usable for its entire lifetime.
- YouTube player-client behavior is not a stable media contract. `web_safari`
  has exposed pre-merged HLS for the tested sample but is converging toward the
  `web`/SABR path; nightly `visionos` exposes HTTPS and HLS formats, including
  higher resolutions, but its audio and video are not pre-merged. A client- or
  sample-specific muxed manifest is therefore only an opportunistic candidate,
  not the baseline capability.
- DASH is useful as an upstream structure but is not a sufficient terminal
  contract because required consumers, including Emby paths, may not support
  it. HLS has not been ruled out as a terminal resource-set representation.

## Requirements

- **R1 — Resource-centered contract:** Model the public result as a resource
  identity and lifecycle, not as a player session, YouTube video, HLS profile,
  or long-lived RuleGo node instance.
- **R2 — Native composition first:** Reuse RuleGo's existing static serving and
  HTTP Range semantics wherever they satisfy the contract. Add serving code
  only for behavior that cannot be composed from those facilities.
- **R3 — Response forms:** Keep the existing sequential stream available and
  support publication of:
  1. one stable HTTP resource with standard byte-range semantics; and
  2. a resource set whose manifest references independently addressable
     members under the same lifecycle.
- **R4 — Explicit readiness:** A consumer must be able to distinguish pending,
  ready, expired, and failed resources without interpreting partial files or
  process-local state.
- **R5 — Bounded lifecycle:** Resource production and retention must have
  caller/operator-owned limits for time, stored bytes, and expiry. No request
  may implicitly cause unbounded production to end-of-input merely to preserve
  seekability. The origin validates publication and retained-byte limits;
  transformation runtime/input limits remain provider-owned.
- **R6 — Demand and reuse:** Equivalent concurrent requests share eligible
  in-flight or ready work. When the requested portion is outside currently
  available output, its stable member route acquires one deterministic bounded
  production unit, waits within the request deadline, and then resolves the
  requested member; it must not silently create detached duplicate jobs.
- **R7 — Stable ownership:** Resource naming, serving, and cleanup must remain
  coherent across rule hot-deploy and process restart. Startup reconciliation
  must reclaim orphaned owned artifacts without touching unrelated files.
- **R8 — Correct HTTP behavior:** A ready single resource supports full `GET`,
  validators, satisfiable and unsatisfiable Range requests, accurate response
  lengths/statuses, and is not exposed before atomic readiness. A resource-set
  manifest may advertise not-yet-materialized members only through stable
  demand routes that can acquire and serve their bounded production unit under
  the same published lifecycle. A demanded
  member's absolute expiry cannot exceed its parent set's expiry, and its route
  refuses acquisition after the parent expires.
- **R9 — Transformation neutrality:** Callers supply transformation arguments
  and publication bounds. Core code must not embed codec selection, resolution,
  YouTube format expressions, HLS segment recipes, or terminal-specific media
  policy.
- **R10 — YouTube specialization research:** Before choosing the media path,
  evaluate whether yt-dlp's selected fragmented/indexed streams can be exposed,
  repackaged, or indexed with less startup work and bounded transfer than a
  generic FFmpeg-from-zero job. The decision must account for expiring direct
  URLs, player-client/SABR volatility, audio/video separation, arbitrary seek,
  Emby-compatible terminal output, startup latency, CPU, and bytes fetched.
- **R11 — Specialization boundary:** A YouTube-specific resolver/adapter may
  refresh or interpret yt-dlp source information, but it must publish through
  the same generic resource/resource-set contract. Any pre-merged manifest is
  selected by observed capabilities and must have a separate-stream fallback;
  provider clients or facts must not become fields or profiles in the core
  origin API.
- **R12 — Evidence before expansion:** The selected design must identify the
  smallest missing behavior after composing the current producer with RuleGo's
  native static serving. No new origin server, scheduler, media session model,
  or storage abstraction is allowed without evidence that the existing pieces
  cannot meet an acceptance criterion.
- **R13 — Plugin coexistence:** The ffmpeg-over-ip client is a versioned leaf
  module, and the RuleGo entrypoint is an independent plugin module. Every
  co-loaded consumer uses the same released client identity. Release validation
  exercises the complete supported plugin set.

## Acceptance Criteria

- [ ] **AC1:** A generic non-YouTube transformation can publish one resource,
  return its address before the caller's RuleGo node is destroyed, and serve it
  until its declared expiry independently of that node instance.
- [ ] **AC2:** A ready single resource passes HTTP checks for full `GET`, a valid
  byte range (`206` with correct `Content-Range` and length), an invalid range
  (`416`), validator-based conditional retrieval, and atomic readiness.
- [ ] **AC3:** A generic transformation can publish a manifest plus at least
  two independently retrievable members. Requesting a non-materialized member
  triggers one bounded production unit and resolves to the completed member;
  every advertised member shares the resource set's lifecycle and is never
  exposed as a partially written file. A member cannot be acquired or served
  after its parent set expires.
- [ ] **AC4:** Two concurrent equivalent publication requests execute at most
  one eligible transformation and observe the same resource identity; waiter
  cancellation does not cancel work still demanded by another waiter.
- [ ] **AC5:** Configured transformation runtime, publication-size, retained
  storage, and expiry bounds are enforced. Provider-side logical file-extent
  accounting stops an active file-producing job before it exceeds its output limit, and a
  demand-limited test proves the producer does not continue to EOF after
  reaching its allowed ahead/retention boundary.
- [ ] **AC6:** After simulated restart or rule hot-deploy, valid published
  resources remain coherently addressable for their declared lifetime and
  orphaned owned artifacts are reclaimed. Unrelated files are untouched.
- [ ] **AC7:** Expired, pending, and failed resource requests have deterministic
  machine-readable outcomes; expired resources cannot revive merely because
  stale files remain on disk.
- [ ] **AC8:** The existing direct `ffmpegOverIp` stdout streaming path remains
  a forward-only, low-startup option and does not acquire resource/session
  state merely to satisfy this task.
- [ ] **AC9:** A recorded YouTube/yt-dlp experiment compares the generic path
  with at least one upstream-structure-aware candidate using the same video and
  reports first playable time, cold seek time, CPU/transcode mode, and upstream
  bytes fetched.
- [ ] **AC10:** The selected YouTube path supports separate selected video and
  audio inputs, refreshes expired yt-dlp source resolution when required, and
  produces a standards-compliant terminal HLS resource verified with protocol
  inspection plus mpv/ffprobe, without exposing provider fields in the generic
  origin contract. A disappearing `web_safari`-style pre-merged HLS format does
  not break that baseline path. Final Emby/Kodi validation is user-owned and is
  not an automated acceptance gate for this task.
- [ ] **AC11:** Design evidence maps every new component to a failed composition
  attempt or an unmet acceptance criterion. If RuleGo static serving plus the
  current producer is sufficient, no duplicate HTTP serving layer is added.
- [ ] **AC12:** The released ffmpeg-over-ip, indexed-vod, and resource-origin
  plugins co-load in the pinned RuleGo runtime in both relevant filename load
  orders. Their four node types and two response processors are registered,
  and no shared-package fingerprint error is logged.

## Scope Boundary

- The generic capability owns published resource identity, readiness,
  lifecycle, publication/retention bounds, and the HTTP contract of the
  resource or resource set.
- Transformation providers own process execution and production of bytes or
  named members. They do not own the public resource lifecycle.
- Source adapters may resolve and refresh provider-specific inputs, including
  yt-dlp results, but do not define response profiles.
- Rule graphs own transformation arguments, routing, and application policy.
- Operators own storage/time limits and deployment-level access control.

## Out of Scope

- A player-session API or per-player playback history.
- Hard-coded YouTube format selection, codec/resolution policy, or an
  `hls_vod`-style static profile in the generic contract.
- Reimplementing HTTP Range handling already supplied by RuleGo/Go.
- Requiring complete media download as the default means of making a resource
  seekable.
- Treating DASH as the required terminal format.
- A general distributed scheduler, object-store abstraction, or cluster cache
  before a single-instance implementation proves those are necessary.

## Ownership Decision

- `ffmpegOverIp` and `ffmpegOverIpProducer` remain transformation providers.
- RuleGo's existing static mapping remains the ready-byte HTTP data plane.
- The missing durable catalog, publication barrier, lifecycle, bounds, and
  reconciliation belong to a separate, source-neutral RuleGo resource-origin
  component. They do not belong in this FFmpeg transport plugin.
- The origin reuses RuleGo's HTTP server and static mapping; it does not add a
  listener or duplicate Range serving.

This boundary follows the native-composition audit: static serving already
solves byte delivery, while provider-local state cannot own a resource across
node and process lifetimes. Packaging and implementation of the generic origin
must therefore be carried by its own repository/task; this task owns the
cross-component contract and any strictly FFmpeg-provider-side integration.
