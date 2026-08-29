# Design: Derived Resource Origin

## 1. Boundary

The system has three independent responsibilities:

```text
source adapter -> transformation provider -> resource origin -> RuleGo static files
 yt-dlp facts       ffmpeg-over-ip          lifecycle/state       GET/HEAD/Range
```

- A source adapter resolves short-lived provider inputs. YouTube/yt-dlp is one
  adapter, not part of the origin contract.
- `ffmpegOverIp` and `ffmpegOverIpProducer` remain transformation providers.
  They own remote process execution and closed-file notification, not public
  URLs or retained-resource state.
- A separate source-neutral RuleGo component owns resource identity,
  publication, expiry, retained-byte limits, sharing, and restart
  reconciliation.
- RuleGo's existing static mapping owns ready-file HTTP transfer. No plugin
  opens another listener or reimplements Range handling.

The generic origin is a separate plugin/repository. This repository owns only
the provider-side integration contract and the cross-component acceptance
evidence. It must not absorb the origin merely because FFmpeg is the first
producer.

## 2. Minimum public contract

One `resourceOrigin` node type exposes three message operations against a
shared configured origin:

### Acquire

```json
{
  "operation": "acquire",
  "key": "caller-owned stable identity",
  "fingerprint": "caller-owned production revision",
  "parentResourceId": "optional parent resource set",
  "ttlMs": 300000,
  "maxBytes": 67108864
}
```

The component hashes `key` plus `fingerprint` into a filesystem-safe resource
ID. It never parses either value as media policy.

- Ready: return a descriptor containing the resource ID, entrypoint, public
  URL, members, size, and expiry.
- Absent: reserve one generation and emit `Produce` with the resource ID, an
  origin-owned staging directory, and an opaque generation token.
- Pending: wait on the existing generation up to the caller context, then
  return the same ready/failure result. It does not start duplicate work.
- Expired/failed: transition deterministically to a new reservation when the
  caller asks to acquire again.

When `parentResourceId` is present, the parent must be ready and unexpired.
The acquired unit's absolute expiry is capped to the parent's expiry; a later
member request never extends the published set's lifetime.

### Commit

```json
{
  "operation": "commit",
  "resourceId": "...",
  "generation": "...",
  "entrypoint": "index.m3u8"
}
```

Commit accepts only the current generation. Every member must be a relative,
regular, closed file below that generation's staging directory. Symlinks,
absolute paths, traversal, missing files, duplicate names, and size overflow
are rejected. The origin discovers the committed members itself and treats
their contents as opaque.

### Fail

```json
{
  "operation": "fail",
  "resourceId": "...",
  "generation": "...",
  "kind": "producer_failed"
}
```

Fail records one bounded, non-secret reason and wakes current waiters. A stale
generation cannot publish or fail a replacement.

### Demand member route

A manifest does not point directly at a guessed future file. Each potentially
unmaterialized member uses a stable RuleGo REST route whose path identifies the
logical set and member. The source/rule adapter maps that member to a
deterministic bounded production-unit key (for example, one fixed-size segment
window) and calls `acquire`:

```text
GET stable member URL
  -> adapter derives bounded unit key and requested member
  -> resourceOrigin acquire
       ready   -> redirect to the member's mapped static URL
       absent  -> Produce -> provider -> commit -> redirect
       pending -> wait for that generation -> redirect or explicit failure
```

The origin does not understand segment numbers or timelines. Window grouping
is source/transformation policy supplied by the rule graph. Every request for
the same logical unit shares one generation, and every production path remains
bounded. A member route passes its parent resource ID to `acquire`, refuses the
request once that parent expires, and publishes each child with
`child expiry <= parent expiry`. The endpoint deadline is part of acceptance:
if a missing member
cannot become ready before the player's measured request tolerance, that
production strategy is rejected rather than hidden behind `202` responses that
ordinary media clients do not understand.

The exact relation names and metadata keys are fixed in the owning plugin's
PRD before implementation. This design does not add a profile registry or a
second generic payload envelope.

## 3. Storage and publication

Use the filesystem and Go standard library; no database or object-store
abstraction is needed for the single-instance release.

```text
<root>/
  catalog/<resource-id>.json
  staging/<resource-id>/<generation>/...
  ready/<resource-id>/...
  trash/<resource-id>-<generation>/...
```

- `staging` is not included in RuleGo's static mapping.
- A producer writes only beneath the supplied staging directory and closes
  each stored member before commit.
- For a materialized set, commit validates the whole stored set and atomically
  renames its directory into `ready`; its manifest is therefore never public
  before those stored members.
- A virtual manifest may instead reference stable demand routes. Its logical
  members are not part of that manifest commit; each is a separately committed
  bounded unit whose acquire request is bound to the manifest resource's
  absolute expiry.
- RuleGo maps one URL prefix to `<root>/ready`. A ready descriptor is the only
  place the public URL is disclosed.
- Expiry first renames the ready directory out of the mapped tree, then removes
  its catalog entry and bytes. A failed deletion cannot make an expired URL
  readable again.
- Startup loads valid ready records, converts abandoned pending generations to
  failed/absent state, and deletes only unreferenced artifacts beneath the
  origin-owned roots.

Catalog records are small versioned JSON files written through temp-file plus
rename. One in-process manager per canonical root is shared through RuleGo's
native shared-node mechanism. Cross-host coordination and network filesystems
are out of scope.

## 4. Bounds and concurrency

- The provider owns transformation runtime and upstream-transfer bounds. The
  origin passes these through the graph but does not pretend it can police a
  remote FFmpeg fetch.
- `maxBytes` is enforced twice: the file-producing provider counts the
  high-water logical extents of unique output files for the generation and
  cancels the remote job before crossing the hard limit;
  the origin independently validates member sizes at commit. Recipe-level
  duration/member limits remain required so a valid low-bitrate job cannot run
  to EOF accidentally.
- A configured origin-wide retained-byte ceiling evicts expired resources
  first and otherwise rejects publication. MVP does not implement LRU because
  native static reads bypass the origin and cannot provide trustworthy access
  timestamps.
- TTL begins at successful publication. The catalog stores an absolute expiry
  so restart does not reset retention.
- Equivalent `key` plus `fingerprint` requests share one generation. Different
  fingerprints have different resource IDs and never overwrite ready bytes.
- Cancellation removes only that waiter. A reserved generation is canceled
  only when its production path fails/cancels or its finite production deadline
  expires; player sessions are not modeled.

## 5. HTTP composition

The origin does not serve bytes. A RuleGo REST route invokes `acquire` and:

- returns or redirects to the static URL only after `ready`;
- maps invalid input, timeout, failed production, and expired state to explicit
  application responses; and
- leaves `GET`, byte ranges, lengths, `Last-Modified`, and conditional requests
  to `http.FileServer`/`http.ServeContent`.

The released `v0.37.0-plugin` host registers only `GET` for static mappings.
The host must add the missing `HEAD` registration (already present in the later
local server source) before AC2 can pass. This is a host fix, not a reason to
duplicate its file server.

## 6. FFmpeg provider integration

`indexedVod` owns one atomic indexed-member production operation. It receives
the origin-issued staging directory, byte limit, and publication deadline;
reads only the selected indexed source ranges; invokes the public
ffmpeg-over-ip client to stream-copy them into one closed MPEG-TS member; and
returns that member to the rule for commit. Producer failure, stale source,
deadline, and byte-limit outcomes remain explicit rule relations.

The generic `ffmpegOverIp` and `ffmpegOverIpProducer` RuleGo nodes remain
independent execution capabilities. `indexedVod` may use the versioned public
ffmpeg-over-ip client as an implementation dependency because it atomically
owns one bounded member production. It does not absorb the generic RuleGo
FFmpeg node, public resource naming, publication, or lifecycle.

The ffmpeg-over-ip transport has two packaging surfaces: a versioned leaf
client module and an independent RuleGo entrypoint module. The entrypoint and
other co-loaded consumers depend on the same released client identity. The
Plugin ABI release fixes the host toolchain and dependencies; the
ffmpeg-over-ip release fixes the shared client identity. Release builds consume
both contracts without local replacements.

## 7. YouTube specialization

The RuleGo chain owns YouTube URL construction, yt-dlp invocation, cookie and
format policy, and normalization of the selected separate video/audio formats
into a short-lived indexed-media lease. Signed URLs, headers, cookies, itags,
player-client names, and SABR details never enter the origin contract or public
manifest.

The selected H.264 MP4 and AAC M4A representations expose compatible indexes.
`indexedVod` derives one stable timeline and produces only the requested
complete MPEG-TS member from its exact source ranges. The HLS VOD manifest is
virtual: it contains stable demand routes for all indexed members but does not
cause them to be produced in advance.

The chain keeps a complete normalized lease only as a short-lived performance
hint. A cache miss or process restart resolves a fresh lease. `source_stale`
evicts the hint, resolves once more, re-inspects the immutable index evidence,
and proceeds only if the revision still matches. Public resource identity is
therefore stable across signed-URL refresh without treating a provider lease as
durable state. Final Emby/Kodi playback validation remains user-owned.

## 8. Delivery boundary

Planning and integration evidence live in this task. Implementation is split
at ownership boundaries:

- the generic origin is released from its own RuleGo plugin repository against
  the same immutable Plugin ABI contract used by the host;
- the host release supplies static `HEAD` plus its existing GET/Range data
  plane;
- this repository supplies the versioned ffmpeg-over-ip client and independent
  generic RuleGo entrypoint; and
- the source-neutral `indexedVod` component lives in its own repository while
  the YouTube adapter remains a rule-chain/source concern.

Release validation co-loads the ffmpeg-over-ip, indexed-vod, and
resource-origin release artifacts in both relevant filename orders. Isolated
plugin smoke loading is insufficient evidence for a Go plugin deployment.

## 9. Rollback

The origin is additive. Removing its `.so` and rule routes restores the current
forward-only and producer behaviors. Its published root is isolated; rollback
renames that root out of the static mapping before optional cleanup. No
existing FFmpeg invocation contract or user media file is migrated in place.

## 10. Shared client package identity

The `ffmpeg-over-ip` RuleGo entrypoint is built from an independent nested Go
module. That module consumes the same released
`github.com/killbus/rulego-ffmpeg-over-ip` client module version as every
co-loaded plugin; it never resolves the client through the entrypoint's main
module or a local replacement. The client remains an internal implementation
dependency of the transformation components and `indexedVod`; no invocation
arguments, filesystem paths, or producer lifecycle move into the rule graph.

Release CI proves the compiled module identity directly from both plugin
artifacts, then co-loads the candidate ffmpeg-over-ip plugin with the supported
indexed-vod and resource-origin release artifacts in both relevant filename
orders. The gate requires all four node types and both REST response processors
to be registered by one pinned RuleGo host without plugin-load errors.
