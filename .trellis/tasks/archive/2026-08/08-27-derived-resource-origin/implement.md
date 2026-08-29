# Implementation plan

## 1. Prove the indexed-media path

- Resolve one selected video-only MP4/H.264 representation and one audio-only
  M4A/AAC representation without relying on a fixed YouTube player client.
- Inspect both immutable indexes and derive one stable timeline and revision.
- Verify with mpv/ffprobe: initial playback and a distant seek through the
  generated HLS VOD manifest.
- Record first playable time, cold seek time, stream-copy mode, and actual
  upstream bytes fetched. Do not claim Emby support; provide the resulting URL
  for the user's terminal test.

## 2. Establish the generic origin owner

- Create a separate RuleGo plugin repository/task for the source-neutral
  `resourceOrigin` component; do not place its implementation in this FFmpeg
  transport repository.
- Consume the host's immutable Plugin ABI release record as the only build/load
  compatibility authority.
- Carry this PRD, design, native-composition audit, and only the generic portions
  of the YouTube research into that task.
- Keep one Go package until file size or tests demonstrate a real package
  boundary.

Rollback point: no runtime code in this repository changes at this stage.

## 3. Implement the minimal resource catalog

- Implement strict decoding for `acquire`, `commit`, and `fail` operations.
- Implement filesystem-safe identity derivation, opaque generation tokens, one
  shared manager per canonical root, and same-process singleflight/waiters.
- Persist versioned JSON records with atomic replacement.
- Validate relative regular-file members and total size, then atomically move a
  complete staging directory into the mapped ready root.
- Implement absolute-expiry cleanup, origin-wide retained-byte rejection, and
  restart reconciliation confined to origin-owned directories.

Checks: one focused test per state transition; concurrent acquire/commit/fail;
stale generation rejection; traversal/symlink rejection; oversize rejection;
restart recovery; atomic visibility; and cleanup that never touches unrelated
files. A child acquire after parent expiry is rejected, and a child committed
later still expires no later than its parent.

## 4. Compose with RuleGo's native HTTP data plane

- Add a minimal RuleGo example routing `acquire -> Produce -> provider ->
  commit`, with producer failures routed to `fail`.
- Add a stable member route that maps one requested member to its deterministic
  bounded production unit, waits for shared production within the endpoint
  deadline, and redirects only to a committed static member.
- Configure static mapping only for the origin's `ready` directory.
- Verify full GET, valid/invalid Range, exact response lengths, conditional
  retrieval, atomic readiness, and that staging paths are unreachable. Do not
  add an HTTP listener or copy `ServeContent` into the plugin.

Checks: two equivalent concurrent requests execute one production path and
receive the same resource URL; a different fingerprint receives a distinct
resource; node recreation and process restart preserve only valid ready state.

## 5. Preserve the FFmpeg capability boundary

- Keep `ffmpegOverIp` as the generic incremental process-stream node and
  `ffmpegOverIpProducer` as the generic bounded file-producing node.
- Publish the ffmpeg-over-ip client as a versioned leaf module that may be used
  by an atomic transformation owner without importing the RuleGo entrypoint.
- Let `indexedVod` use that client to stream-copy exactly one complete member
  inside the origin-issued staging directory and byte/deadline limits.
- Keep the independent origin commit-time validation. No URL, catalog,
  retention, HLS profile, YouTube field, or player session enters the FFmpeg
  plugin.

Checks: cancellation, producer failure, incomplete members, and a stale
generation cannot publish bytes.

## 6. Implement the selected YouTube path

- Keep the existing yt-dlp HTTP wrapper unchanged; it owns extraction.
- Keep the format selector, cookie path, and stable URL construction in the
  RuleGo chain rather than plugin configuration.
- Normalize selected separate video/audio formats into the generic indexed
  lease, inspect their immutable indexes, and publish a virtual HLS VOD
  manifest whose stable routes produce one requested member at a time.
- Produce every member by bounded source-range reads and FFmpeg stream-copy;
  never run to EOF or pre-generate an adjacent window.
- Retry one stale-lease failure after re-resolution and revision verification
  without changing the public resource identity. Do not retry deterministic
  format or mux errors.

Checks: initial and distant seek with mpv, URL-expiry refresh, concurrent window
singleflight, bounded retained bytes, cancellation, and no signed URL in public
manifests/logs/errors. The user performs the final Emby/Kodi matrix.

## 7. CI, review, and release

- Run formatting, unit tests, race tests, and integration tests in GitHub CI;
  do not perform heavyweight local builds.
- Build the generic plugin with the matching RuleGo Plugin SDK and smoke-load
  it in the matching runtime image on supported architectures.
- Build the ffmpeg-over-ip RuleGo entrypoint from its independent `plugin`
  module without a local replacement, compare its compiled shared-client
  identity with indexed-vod, and co-load the ffmpeg-over-ip, indexed-vod, and
  resource-origin release artifacts in both relevant filename orders.
- Run a Trellis implementation review and an independent completion audit
  mapping every PRD acceptance criterion to current evidence.
- Publish versioned `.so`, checksum, ABI sidecar, compatibility record, and the
  minimal generic example. Keep provider and origin commits/releases separate.
- Build the ffmpeg-over-ip RuleGo entrypoint from its nested module with
  `GOWORK=off` semantics and an exact released parent-module dependency; do not
  use a local replacement for the shared client.
- Compare the candidate and supported indexed-vod artifacts' compiled client
  dependency records, then load the candidate, indexed-vod, and
  resource-origin release plugins together in both relevant filename orders.
  Require all four component types and both response processors, with no
  plugin-load errors, before publishing the patch release.

Rollback point: remove the origin `.so` and routes, then rename its ready root
out of static mapping. Existing direct `ffmpegOverIp` playback remains usable.
