# RuleGo integration facts

Evidence sources: RuleGo server repository revision
`3bf4ac47bb49aff9fe048e35644a6bca6e8e2af3` and its pinned core module
`8995627f6da7bd6d819475373c324cf249af0a13`.

The exact core commit archive was retrieved from GitHub with SHA-256
`2669d74900c819da02ecb14f7cbca83fcfe05f569088880379439052ba5968ae`.
Its `engine/rule_context.go`, `engine/registry.go`,
`endpoint/impl/endpoint.go`, `endpoint/rest/rest.go`, and
`builtin/processor/processor.go` are byte-identical to the corresponding files
at the inspected server repository revision. The contracts below therefore
apply to the server's actual core dependency, not only repository HEAD.

## Plugin and node lifecycle

- `api/types/types.go` defines `PluginRegistry`: a Go plugin exports `Plugins`
  and returns components from `Components() []types.Node`.
- `engine/registry.go` loads `Plugins` and registers the returned nodes. The
  loader does not call `PluginRegistry.Init()`. Any companion output-processor
  registration therefore has to occur during Go package initialization until
  RuleGo changes that loader contract.
- `types.Node.OnMsg` must eventually call `TellSuccess`, `TellFailure`,
  `TellNext`, or `DoOnEnd`. `RuleContext.GetContext()` exposes the execution
  context and its cancellation.

## Streaming model

- `types.Stream` is a relation name. There is no stream object and
  `RuleContext` has no `TellStream` method; a node emits a chunk with
  `ctx.TellNext(chunk, types.Stream)`.
- `engine/rule_context.go` executes `Stream` successors synchronously to retain
  chunk order. Non-stream relations are submitted asynchronously.
- Both stdout and stderr therefore have to use `Stream` with channel metadata
  when downstream-observed wire order is part of the contract. A custom
  `Stderr` relation cannot provide that guarantee.
- `components/common/end_node.go` preserves the incoming relation when invoking
  the rule-chain end callback. A `Stream` edge from the client node to an end
  node therefore reaches endpoint post-processors as repeated callbacks.
- `RuleMsg` stores payload data as a string, including `BINARY` data. Converting
  it back to `[]byte` preserves arbitrary bytes.

## REST response behavior

- `endpoint/rest/rest.go` writes every `SetBody` call immediately and exposes
  `Flush()` when the underlying writer supports `http.Flusher`.
- For synchronous routes (`to.wait=true`), the REST endpoint passes the HTTP
  request context into the rule execution; disconnect cancellation therefore
  reaches `RuleContext.GetContext()`. Asynchronous routes deliberately replace
  it with a background context.
- A non-positive REST `writeTimeout` selects RuleGo's 10-second default rather
  than disabling the deadline, so long-stream examples must set an explicit
  operator-appropriate positive horizon.
- `builtin/processor/processor.go`'s `responseToBody` writes callback data but
  does not call `Flush`, and it writes errors into the body. It is insufficient
  for a progressive media body and can contaminate stdout with stderr or
  terminal errors.
- `endpoint/impl/endpoint.go` invokes `to.processors` for every end callback and
  wraps each callback message in `ScopedMessage`, which delegates `SetBody`
  and `Flush` to the underlying HTTP response.
- The server's `GET /api/v1/components` response exposes output processor names
  at `builtins.endpoints.outProcessors`, so CI can verify both the node and the
  package-init processor registration after loading the plugin.

## Design consequence

The plugin needs one formal Node plus one small output processor registered at
package load time. The node remains the capability boundary. The processor is
only the RuleGo REST projection: it writes stdout chunks, flushes them, and
does not serialize stderr or terminal records into the media body.

The server repository's release workflow builds stock binaries with
`CGO_ENABLED=0`, so those binaries cannot be claimed as compatible Go-plugin
hosts. Compatibility must name a source-built plugin-enabled server and the
exact core module, Go toolchain, architecture, and build mode shared with the
plugin.
