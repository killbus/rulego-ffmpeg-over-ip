# RuleGo ffmpeg-over-ip client plugin

## Goal

Provide ffmpeg-over-ip client semantics as a native RuleGo plugin so a rule
chain can start one authenticated remote `ffmpeg` or `ffprobe` invocation,
observe its streams and terminal result, and cancel it through the RuleGo
execution lifecycle.

The plugin is a remote-process capability. Media discovery, source selection,
FFmpeg recipes, container choice, and downstream-player behavior remain in the
calling rule graph and deployment configuration.

## Confirmed Facts

- RuleGo's supported Go plugin entry point exports `Plugins`, whose
  `Components` method returns RuleGo nodes.
- A RuleGo node can emit incremental messages through the `Stream` relation and
  terminate through `Success` or `Failure`.
- The ffmpeg-over-ip client protocol carries an authenticated program and
  ordered argv, stdin, stdout, stderr, exit status, cancellation, keepalive, and
  client-side file operations.
- The current ffmpeg-over-ip reusable implementation is under Go `internal/`
  packages and its executable entry point uses process-global CLI facilities;
  it cannot be imported unchanged by an independent plugin module.
- The RuleGo host distributor publishes one lock-derived Plugin ABI release
  record containing immutable SDK and runtime image digests. The SDK builds
  the plugin and its ABI sidecar; the matching runtime is the load target.

## Requirements

- **R1 — Native component:** Publish a RuleGo-loadable Go plugin whose component
  identity describes the ffmpeg-over-ip client capability rather than any one
  media workflow.
- **R2 — Invocation fidelity:** Accept only the supported `ffmpeg` and
  `ffprobe` programs and preserve every argv element exactly without shell
  parsing or reconstruction.
- **R3 — Protocol compatibility:** Authenticate and communicate with a pinned
  ffmpeg-over-ip server protocol version using behavior compatible with the
  corresponding client release.
- **R4 — Full session semantics:** Support stdin forwarding and EOF, stdout and
  stderr delivery, exit status, keepalive, cancellation, and client-side file
  operations as one coherent remote client session.
- **R5 — Stream fidelity:** Emit stdout and stderr incrementally through
  RuleGo's synchronous `Stream` relation, distinguish them with channel
  metadata, and preserve their protocol wire order without buffering the
  completed process.
- **R6 — Terminal fidelity:** Report the remote exit code exactly once; exit
  zero terminates through `Success`, while nonzero exit or transport/protocol
  failure terminates through `Failure`.
- **R7 — Lifecycle:** Bind request cancellation, configured timeout, and node
  destruction to a graceful remote cancel, then release sockets, goroutines,
  and open local resources.
- **R8 — Configuration authority:** Keep server address, authentication secret,
  and timeout in operator-controlled node configuration, including values
  resolved from RuleGo globals. Client-side file operations run with the RuleGo
  process's normal filesystem authority; the plugin adds no second allowlist
  policy.
- **R9 — Concurrency:** Each RuleGo message owns an independent session. The
  implementation must not introduce global serialization or unbounded output
  buffering. Beyond the finite, caller-owned input `RuleMsg`, per-session
  working memory must be bounded by fixed protocol buffers rather than output
  duration; optional base64 stdin is decoded incrementally without a second
  full-size decoded copy, and file-read requests that cannot fit the protocol's
  maximum response frame are rejected before allocation.
- **R10 — Domain neutrality:** The plugin must not discover media, select
  formats/codecs, construct domain-specific FFmpeg recipes, select containers,
  publish URLs, or contain caller-specific policy.
- **R11 — Delivery:** Produce a versioned `.so`, SDK-generated checksum and ABI
  sidecar, the consumed Plugin ABI release record, and a minimal generic RuleGo
  example. CI builds with the record's immutable SDK digest and load-tests with
  its matching runtime digest. Heavy compilation does not run locally.
- **R12 — REST projection:** Register the minimum RuleGo output processor needed
  for a REST Endpoint to write only stdout chunks and flush after each chunk.
  stderr and terminal records must remain available to rule relations without
  entering the media response body.
- **R13 — Transport fidelity:** Support the upstream TCP and `unix:` address
  forms. Do not add TLS, HTTP, retry/failover, or local-process fallback to the
  ffmpeg-over-ip wire connection.

## Acceptance Criteria

- [ ] **AC1:** On native Linux amd64 and arm64 runners, the digest-pinned RuleGo
  Plugin SDK builds the `.so` and sidecar, and the matching digest-pinned
  runtime loads it through its normal plugin loader and advertises both
  `ffmpegOverIp` and `ffmpegOverIpResponse`.
- [ ] **AC2:** A test server observes the exact selected program and argv vector,
  including arguments containing spaces and punctuation; no shell is involved.
- [ ] **AC3:** Against an actual pinned ffmpeg-over-ip v5.2.1 server, correct
  authentication succeeds over both TCP and Unix sockets, incorrect
  authentication fails, a direct binary `pipe:1` invocation round-trips exact
  bytes, and the plugin exposes neither secrets nor argv in its logs or error
  responses.
- [ ] **AC4:** stdout and stderr become observable before remote process
  completion, remain distinguishable by channel metadata, and arrive at the
  synchronous `Stream` consumer in protocol wire order.
- [ ] **AC5:** stdin reaches the remote process and EOF is propagated.
- [ ] **AC6:** exit `0`, nonzero exit, server error, malformed frame, disconnect,
  timeout, and caller cancellation each produce one deterministic terminal
  outcome without leaking session resources. Deterministic clock-driven tests
  cover the upstream 30-second idle-send interval, 150-second receive timeout,
  and ping-payload echo.
- [ ] **AC7:** cancellation sends the protocol cancel message before closing the
  connection, and the test server observes remote-session termination within a
  five-second grace interval. Racing caller cancellation, session timeout, and
  node destruction still send at most one cancel and one terminal outcome.
- [ ] **AC8:** Concurrent invocations remain isolated; after accepting the
  caller-owned input message, each session's working memory is bounded by fixed
  protocol buffers rather than output duration. An oversized file-read request
  is rejected before allocating its requested size.
- [ ] **AC9:** a generic REST Endpoint example progressively returns stdout from
  an ffmpeg-over-ip invocation without the plugin containing HTTP route or media
  provider policy.
- [ ] **AC10:** CI publishes Linux amd64 and arm64 `.so`, `.sha256`, and
  `.abi.json` files together with the exact Plugin ABI release record; the
  plugin repository publishes no container image.
- [ ] **AC11:** The node accepts a JSON invocation containing `program`, exact
  `args`, and optional base64 stdin, forwards stdin then EOF, emits raw stdout
  and stderr on `Stream` with distinct channel metadata, and emits exactly one
  structured terminal result on `Success` or `Failure` after prior stream
  callbacks finish.
- [ ] **AC12:** A REST client disconnect cancels the RuleGo context; the node
  sends `MsgCancel`, performs bounded cleanup, and does not continue a detached
  transcode.

## Scope Boundary

- The plugin owns the RuleGo component adapter and the ffmpeg-over-ip client
  session it creates.
- Rule graphs own construction of program arguments and routing of stream and
  terminal relations.
- Operators own server deployment, credentials, endpoint selection, concurrency
  policy, and the RuleGo process/container filesystem permissions.
- Consumers own protocol-specific response headers and playback validation.

## Out of Scope

- Server deployment and media-workflow policy.
- Local-process execution or shell execution.
- A stateful multi-message interactive stdin API. One invocation may carry an
  optional finite stdin payload; the node forwards it and then sends EOF.
- Compatibility with hosts outside the consumed Plugin ABI release record.
