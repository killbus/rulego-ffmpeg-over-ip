# Design: RuleGo ffmpeg-over-ip client plugin

## 1. Capability boundary

The repository produces a RuleGo Go plugin. Its primary exported capability is
one node, `ffmpegOverIp`, which represents one authenticated remote
`ffmpeg`/`ffprobe` process invocation. It does not model caller-specific media
policy.

The plugin also registers one output processor, `ffmpegOverIpResponse`, because
RuleGo's built-in `responseToBody` does not flush and would serialize non-stdout
callbacks into an HTTP media body. This processor is a projection of the node's
events onto an existing REST response, not a second execution capability.

## 2. Public contracts

### 2.1 Node configuration

```json
{
  "address": "ffmpeg.example:5050",
  "authSecret": "${global.ffmpeg_over_ip_auth_secret}",
  "dialTimeoutMs": 5000,
  "sessionTimeoutMs": 0
}
```

- `address` and `authSecret` are required. RuleGo resolves ordinary global
  configuration placeholders before initializing the node.
- `dialTimeoutMs` bounds connection establishment. `sessionTimeoutMs=0` means
  the caller context is the only session deadline.
- Protocol version, keepalive interval, frame limit, output chunking, and cancel
  grace use the pinned upstream protocol behavior and are not configurable.

### 2.2 Invocation message

The input `RuleMsg` must contain JSON:

```json
{
  "program": "ffmpeg",
  "args": ["-i", "https://example/video", "-f", "matroska", "pipe:1"],
  "stdinBase64": ""
}
```

- `program` is exactly `ffmpeg` or `ffprobe`.
- `args` is an ordered array of strings. No element is split, joined, rewritten,
  interpolated by a shell, or logged by the plugin.
- `stdinBase64` is optional. When present, it is decoded and sent in bounded
  chunks directly from the already-resident input string, without allocating a
  second full decoded payload; whether present or absent, the client sends
  `MsgStdinClose` after the payload. This is the complete stdin lifecycle for
  one message-owned session.
- Unknown fields are rejected to keep the trust boundary explicit.
- Protocol-size limits are validated before encoding so argument counts or
  elements cannot wrap their unsigned 16-bit wire fields.

### 2.3 Output relations

| Relation | Payload | Metadata | Meaning |
| --- | --- | --- | --- |
| `Stream` | raw stdout bytes, `BINARY` | `ffmpegOverIp.channel=stdout` | zero or more ordered chunks |
| `Stream` | raw stderr bytes, `BINARY` | `ffmpegOverIp.channel=stderr` | zero or more ordered chunks |
| `Success` | terminal JSON | `ffmpegOverIp.exitCode=0` | one clean exit |
| `Failure` | terminal JSON + RuleGo error | exit code when known | one nonzero exit or session failure |

Both output channels use `Stream` because the pinned RuleGo host executes only
that relation synchronously; custom relations are asynchronous and cannot
preserve downstream order. The single protocol read loop emits output in wire
order without a reorder buffer. It emits the terminal result only after the
exit frame and after all prior synchronous stream callbacks have finished.

The terminal JSON is small and stable:

```json
{"program":"ffmpeg","exitCode":0}
```

Transport/protocol failures add a machine-readable `kind` and a non-secret
message. Raw arguments, authentication material, and signed command bytes are
never included.

Outputs preserve caller metadata. Stream records overwrite the channel key;
terminal records remove it and overwrite or remove the exit-code key so stale
or caller-forged projection metadata cannot enter the REST response body.

### 2.4 REST output processor

`ffmpegOverIpResponse` consumes the callback messages produced by a RuleGo REST
Endpoint's `to.processors` stage:

- stdout: set the raw body chunk, then call `Flush()`;
- stderr: write nothing;
- successful terminal result: write nothing;
- failure: attempt to set HTTP status `502` without writing a response body;
  RuleGo ignores the status change if stdout has already committed the headers.

The processor is stateless. It identifies stdout by the node's channel metadata;
the route owns `Content-Type`, cache policy, and other response headers.

## 3. Internal structure

Use the fewest packages that keep trust boundaries testable:

```text
plugin.go                 exported Plugins value and processor registration
node.go                   RuleGo config/input/output/lifecycle adapter
client/
  client.go               one connection/session loop
  protocol.go             protocol-v6 frames and HMAC command encoding
  files.go                client-side file request handling
```

The client package is an attributed MIT adaptation of only the upstream
client-side code needed by the node. Do not copy CLI configuration discovery,
local fallback, server process management, `fio/`, `patches/`, bundled FFmpeg,
website, or Docker files. A root `NOTICE` records the upstream version, commit,
source paths, and MIT notice.

No interface/factory is introduced for a single client implementation. Tests
use `net.Pipe` or a loopback listener at the concrete session boundary.

## 4. Lifecycle and concurrency

1. `OnMsg` validates and decodes the invocation before opening a socket.
2. One session object, connection, serialized writer, file table, and context
   are created for that RuleGo message.
3. The command is nonce-signed and sent. Stdin is written in bounded chunks and
   followed by EOF while the read loop handles multiplexed server frames.
4. The read loop immediately emits stdout/stderr as channel-tagged `Stream`
   messages and services file requests. It does not retain prior output.
5. Exit emits exactly one terminal relation. A terminal `sync.Once` prevents
   racing disconnect, timeout, and cancellation paths from double completion.
6. Caller cancellation, timeout, or `Destroy` first attempts `MsgCancel`, waits
   for the upstream five-second grace interval, then closes the connection. A
   separate `sync.Once` permits at most one cancel frame across those sources.
7. `Destroy` cancels all sessions owned by that node instance and waits only for
   the bounded grace interval. It does not serialize independent sessions.

Backpressure is intentional: synchronous `Stream` delivery and socket reads
couple downstream consumption to the remote process instead of buffering an
entire media output in memory. File reads and base64 stdin decoding reuse
bounded buffers. The caller-owned `RuleMsg` is already resident before the node
runs and is not counted as session buffering.

Protocol frames are capped at the upstream 100 MiB payload limit. A file-read
response contains a two-byte request ID, so a requested read greater than
`100 MiB - 2 bytes` returns `FioERANGE` before allocation. The reusable file-read
buffer can grow only to that wire-valid ceiling.

The liveness loop follows the pinned client constants: after 30 seconds without
an outbound frame it sends `MsgPing`; after 150 seconds without an inbound frame
it terminates the session; an inbound `MsgPing` is answered with `MsgPong` and
the identical payload. The concrete state transitions accept an observed time
so unit tests exercise boundary instants without real multi-minute sleeps.

## 5. Security and authority

- Only `ffmpeg` and `ffprobe` program identifiers are accepted.
- The plugin never invokes a shell or a local FFmpeg executable.
- Authentication uses a fresh cryptographic nonce and HMAC-SHA256 over the
  exact protocol-v6 command fields.
- The wire connection is the upstream raw TCP or Unix-domain socket transport;
  the plugin adds no TLS backend, HTTP tunnel, retry, or failover semantics.
- The shared secret and argv are excluded from logs and returned errors.
- Client-side file operations use the RuleGo process's existing OS/container
  permissions, matching the upstream client. The plugin does not create a
  second filesystem policy language.
- Address authorization, network egress, endpoint authentication, and which
  invocation fields callers may influence belong to RuleGo deployment and the
  rule graph.

## 6. Compatibility and release

- Pin ffmpeg-over-ip behavior to `v5.2.1`, protocol `0x06`, commit
  `ab7adfeedf2a50f7e5807beef9088609cce645d6`.
- Commit the host distributor's reviewed `plugin-abi-release.json` unchanged.
  Its immutable SDK and runtime digests, ABI ID, and lock digest are the only
  RuleGo build-compatibility authority consumed by this repository.
- GitHub CI, on native Linux amd64 and arm64 runners, runs unit/integration/race
  checks and builds target-qualified `.so` files. Local work runs only cheap
  static checks; no heavy local compilation.
- The SDK produces each `.so`, checksum, and ABI sidecar. The matching runtime
  loads that exact plugin and exposes both registered extension points.
- A separate CI integration starts the actual pinned ffmpeg-over-ip v5.2.1
  server and exercises authenticated TCP and Unix-socket sessions with a real
  FFmpeg binary producing deterministic binary output on `pipe:1`. The in-process
  fixture remains for fault injection, not as compatibility authority.
- A release contains the two `.so` files and their SDK-generated checksum and
  ABI sidecars, the consumed ABI release record, license/compatibility notes,
  and the generic RuleGo example. There is no plugin image.

## 7. Generic RuleGo example

The example contains:

- a REST Endpoint accepting the invocation JSON;
- a synchronous `to.wait=true` route so the HTTP request context remains bound
  to the RuleGo execution and client disconnect cancels the session;
- a REST server write timeout suitable for the longest intended stream (the
  generic example uses 86400 seconds; zero selects RuleGo's 10-second default,
  not an unlimited deadline);
- one `ffmpegOverIp` node;
- `Stream -> end` for both channel-tagged output streams;
- stderr reaches the end callback but is ignored by
  `ffmpegOverIpResponse`;
- `Success` and `Failure` terminal paths for observability;
- `to.processors: ["ffmpegOverIpResponse"]` and an operator-selected media
  `Content-Type` header.

It proves progressive bytes and cancellation with a synthetic protocol test
server. It does not prescribe the caller's arguments or response media type.
Because the pinned REST endpoint reads a complete request body before creating
the `RuleMsg`, deployments that accept large base64 stdin must enforce their
desired request-size limit before RuleGo; the generic mux example sends no
stdin payload.

## 8. Rollback

The plugin is additive. Rollback removes the `.so` from RuleGo's plugin
directory and restarts the host. No stored schema or external service state is
created.
