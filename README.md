# rulego-ffmpeg-over-ip

A native RuleGo plugin for one authenticated ffmpeg-over-ip v5.2.1
protocol-v6 invocation per RuleGo message. It exposes the `ffmpegOverIp` node
and the `ffmpegOverIpResponse` REST output processor.

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
timeout leaves the deadline to the RuleGo request context.

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

Both stdout and stderr are emitted synchronously on RuleGo's `Stream` relation
in wire order. Metadata `ffmpegOverIp.channel` is `stdout` or `stderr`.
The terminal message uses `Success` for exit 0 and `Failure` otherwise, with
`ffmpegOverIp.exitCode` when an exit status is known.

The REST processor writes and flushes only stdout. The generic example is in
[`examples/rest-streaming/chain.json`](examples/rest-streaming/chain.json).

## Build

Release binaries are built in GitHub Actions. Go plugin ABI requirements make
local ad-hoc builds unsafe for deployment; see [COMPATIBILITY.md](COMPATIBILITY.md).

This plugin does not provide a server, local-process fallback, TLS, retry,
failover, media discovery, codec/container policy, or response headers beyond
the generic example's standard binary content type.
