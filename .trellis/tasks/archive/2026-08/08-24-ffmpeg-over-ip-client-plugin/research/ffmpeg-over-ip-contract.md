# ffmpeg-over-ip client facts

Evidence source: `steelbrain/ffmpeg-over-ip` release `v5.2.1`, commit
`ab7adfeedf2a50f7e5807beef9088609cce645d6`.

## Wire and session contract

- `internal/protocol/wire.go` declares protocol version `0x06`, length-prefixed
  frames, the `ffmpeg` and `ffprobe` program identifiers, stdin/stdout/stderr,
  exit, error, ping/pong, cancel, and all client-side file-operation frames.
- `internal/auth/auth.go` signs version, random nonce, program, argument count,
  and every length-prefixed argument with HMAC-SHA256. Arguments are not shell
  text and must remain an ordered string vector.
- `cmd/client/main.go` opens one connection per invocation, forwards stdin and
  EOF, handles file requests locally, forwards stdout and stderr independently,
  maintains keepalive state, sends cancel on termination, and returns the
  server's exit code.
- `internal/session/writer.go` serializes concurrent frame writes. The plugin
  needs the same single-writer invariant because stdin, keepalive, cancel, and
  file responses share one connection.
- The client sends a keepalive after 30 seconds without output, treats 150
  seconds without input as a dead session, echoes ping payloads in pong frames,
  and gives cancellation five seconds to receive an exit before closing.
- `internal/filehandler/handler.go` implements open, read, write, seek, close,
  stat, truncate, unlink, rename, and mkdir against the client process's local
  filesystem. This is part of the client behavior that makes remote FFmpeg
  operate on client-side files.

## Reuse and licensing

- The reusable code is below Go `internal/`, so a module at this repository's
  honest import path cannot import it directly.
- The upstream license states that `fio/` and `patches/` are GPL-3.0 and all
  other files are MIT. A RuleGo client plugin does not need the server-side
  patched FFmpeg, `fio/`, patches, bundled media binaries, Docker assets, local
  fallback, config-file search, or CLI signal/process behavior.
- The minimum independent implementation is an attributed adaptation of the
  MIT client-side protocol/auth/file-handler behavior and a session loop shaped
  for a RuleGo context. It should stay pinned to protocol v6 / upstream v5.2.1
  and be covered by compatibility tests rather than pretending to import
  inaccessible `internal` packages.

## Performance consequences

- Read frames can be large, but stdout/stderr are already emitted by the server
  in 32 KiB chunks. The plugin should forward each received output frame and
  never aggregate output by process duration.
- A `MsgReadOk` payload prefixes file bytes with a two-byte request ID while the
  wire reader accepts at most 100 MiB. The adapted client must reject requested
  reads above `100 MiB - 2 bytes` before allocation; upstream's unchecked
  `uint32` allocation is not a safe bound to copy.
- One read loop preserves wire order; one serialized writer prevents frame
  interleaving. Each RuleGo invocation owns its own connection and file table,
  so unrelated sessions need no global lock.
