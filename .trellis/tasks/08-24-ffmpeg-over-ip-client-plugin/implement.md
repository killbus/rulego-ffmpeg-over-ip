# Implementation plan

## 1. Establish the pinned module and legal boundary

- Create the Go module at the repository's real import path.
- Pin RuleGo server revision `3bf4ac47bb49aff9fe048e35644a6bca6e8e2af3`,
  core module `8995627f6da7bd6d819475373c324cf249af0a13`, and the
  selected Go toolchain/build mode.
- Add the upstream MIT notice and a machine-readable source/version note for
  ffmpeg-over-ip `v5.2.1` / `ab7adfeed`.
- Do not import or copy upstream GPL `fio/` or `patches/` content.

Verification: inspect `go.mod`, license/notice files, and `go list -m all` in CI.

## 2. Implement the protocol-v6 client session

- Add frame constants, bounded frame parsing/writing, command encoding, random
  nonce creation, and HMAC signing.
- Validate all unsigned 16-bit command fields before encoding.
- Add the concrete session loop for stdin/EOF, stdout, stderr, exit/error,
  ping/pong, cancel, and client-side file requests.
- Reject file reads larger than `100 MiB - 2 bytes` before allocation so every
  `MsgReadOk` response fits the upstream frame limit.
- Keep one serialized writer and one file table per session; make terminal
  delivery and cancel emission independently idempotent and cleanup bounded.
- Adapt only the required MIT upstream behavior; retain attribution next to
  adapted source.

Verification: focused tests for exact argv and HMAC, malformed/oversized frames,
the file-read pre-allocation cap, all frame types, file operations, disconnects,
and single terminal completion. Clock-driven tests cover the 30-second idle-send,
150-second receive timeout, ping-payload echo, five-second cancel grace, and
racing cancellation sources without wall-clock sleeps.

Rollback point: protocol/client files are self-contained and can be reverted
before RuleGo integration.

## 3. Implement the RuleGo plugin surface

- Export `Plugins` with one `ffmpegOverIp` node.
- Validate node configuration and strict invocation JSON.
- Decode optional base64 stdin incrementally from the caller-owned input string.
- Bind `RuleContext.GetContext()`, configured timeout, and `Destroy` to session
  cancellation.
- Map stdout and stderr to synchronous `Stream` messages with distinct channel
  metadata, exit zero to `Success`, and all other terminal outcomes to `Failure`
  using the documented payload/metadata.
- Register `ffmpegOverIpResponse` at package load because the pinned RuleGo
  loader does not invoke `PluginRegistry.Init()`.

Verification: node tests cover invalid inputs, cross-channel wire ordering,
concurrency isolation, bounded working buffers, no secret/argv disclosure, and
response filtering.

## 4. Prove the end-to-end RuleGo path

- Add a deterministic in-process ffmpeg-over-ip protocol server fixture.
- In CI, build and start the actual pinned upstream v5.2.1 server and exercise
  authenticated TCP and Unix-socket sessions whose real FFmpeg process writes
  deterministic binary bytes to `pipe:1`.
- Source-build and exercise the unmodified target RuleGo server with Go plugin
  support and the pinned ABI tuple; document that stock `CGO_ENABLED=0` binaries
  cannot load Go plugins.
- Add a generic REST Endpoint example and integration test showing that the
  first stdout chunk is flushed before exit, stderr is excluded from the body,
  terminal data is excluded, and HTTP cancellation reaches `MsgCancel`.
- Exercise concurrent requests and assert bounded buffers/session cleanup.

Verification: the in-process fixture proves deterministic faults; the pinned
upstream server proves protocol compatibility. CI only; do not run heavyweight
builds locally.

## 5. Add CI and release artifacts

- CI runs formatting, vet, unit/integration tests, and race checks.
- Native Linux amd64 and arm64 jobs build target-qualified `.so` files and run
  the compatible-host load smoke test.
- Release workflow packages the exact CI-produced `.so` files, SHA-256 files,
  license/compatibility notes, and example without rebuilding them.
- Release publication remains a separate authorized external transition.

## Final gates

- Run Trellis quality check against R1-R13 and AC1-AC12.
- Verify no server/deployment implementation, caller-specific media policy, or
  unbounded output aggregation entered the diff.
- Persistent path review checks the exact implementation revision before
  commit/release transitions.
- A fresh completion auditor maps every requirement to current CI and release
  evidence before completion is claimed.
