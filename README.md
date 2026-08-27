# rulego-ffmpeg-over-ip

A native RuleGo plugin for authenticated ffmpeg-over-ip v5.2.1 protocol-v6
invocations. It exposes the `ffmpegOverIp` node for one invocation per message,
the `ffmpegOverIpProducer` node for bounded file-producing jobs, and the
`ffmpegOverIpResponse` REST output processor.

## Node configuration

```json
{
  "address": "ffmpeg.example:5050",
  "authSecret": "${global.ffmpeg_over_ip_auth_secret}",
  "dialTimeoutMs": 5000,
  "sessionTimeoutMs": 0
}
```

`address` accepts TCP `host:port` and `unix:/path/to/socket`. A zero session
timeout leaves the deadline to the RuleGo request context. The producer node
uses the same connection fields but does not expose `sessionTimeoutMs`; each
producer request carries its finite runtime instead.

## Invocation

```json
{
  "program": "ffmpeg",
  "args": ["-i", "input name", "-f", "matroska", "pipe:1"],
  "stdinBase64": ""
}
```

Only `ffmpeg` and `ffprobe` are accepted. Arguments remain an exact ordered
vector and never pass through a shell. Optional stdin is decoded and sent in
bounded chunks, followed by protocol EOF.

The session also implements the protocol's client-side file operations using
the RuleGo process's existing filesystem permissions. Configure that process
with only the filesystem access remote invocations should have.

Use the separate `ffmpegOverIpProducer` node when a finite file-producing job
must outlive one HTTP request:

```json
{
  "key": "asset:output-profile",
  "invocation": {
    "program": "ffmpeg",
    "args": ["-t", "30", "-f", "hls", "/app/data/hls/window/index.m3u8"]
  },
  "awaitFile": "/app/data/hls/window/0.ts",
  "awaitOnly": false,
  "awaitCompletion": true,
  "maxBytes": 67108864,
  "runTimeoutMs": 60000,
  "cacheTtlMs": 120000
}
```

The producer node keeps at most one job for a key. Identical invocations share
it; a different invocation cancels and replaces it. The request returns the
complete `awaitFile` on the `Stream` relation as soon as ffmpeg closes or
atomically renames that file, while the bounded job continues. Files created
by the job are deleted after the TTL if they have not since been replaced. The
last disconnected waiter cancels a job that has not produced its requested
file. The rule chain owns window size and must bound ffmpeg with `-t`.
`maxBytes` is a required hard aggregate limit for bytes accepted through the
remote file protocol; exceeding it rejects the write and cancels the remote
invocation. Set `awaitCompletion` when the result will be published: success
then requires both the requested closed file and a successful terminal process
result, so a later producer failure cannot publish an incomplete resource set.
Set `awaitOnly` to `true` when a downstream node only needs the completed file;
the producer then emits `Success` without reading or emitting the file on
`Stream`.

The `ffmpegOverIp` node remains stateless across messages. Its stdout and
stderr are emitted synchronously on RuleGo's `Stream` relation in wire order.
Metadata `ffmpegOverIp.channel` is `stdout` or `stderr`. The terminal message
uses `Success` for exit 0 and `Failure` otherwise, with
`ffmpegOverIp.exitCode` when an exit status is known.

The REST processor writes and flushes only stdout. The generic example is in
[`examples/rest-streaming/chain.json`](examples/rest-streaming/chain.json).

## Build

Release binaries are built in GitHub Actions with the digest-pinned SDK in
[`plugin-abi-release.json`](plugin-abi-release.json), then smoke-loaded by its
matching runtime. Each `.so` ships with `.sha256` and `.abi.json` sidecars. Go
plugin ABI requirements make local ad-hoc builds unsafe for deployment; see
[COMPATIBILITY.md](COMPATIBILITY.md).

This plugin does not provide a server, local-process fallback, TLS, retry,
failover, media discovery, codec/container policy, or response headers beyond
the generic example's standard binary content type.
