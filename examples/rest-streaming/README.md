# Generic REST streaming example

From a repository checkout, load `examples/rest-streaming/chain.json`. From a
release bundle, load `example-chain.json`. Use the pinned, plugin-enabled
RuleGo host, place the matching `.so` under its configured `data/plugins`
directory, and define the global `ffmpeg_over_ip_auth_secret`.

The route accepts an invocation JSON body at `POST /ffmpeg`. Its
`setBinaryDataType` input processor selects the generic
`application/octet-stream` response type. Replace that route-level processor
when the caller requires a more specific media `Content-Type`.

`to.wait=true` keeps the HTTP request context attached to the invocation.
Disconnecting the client therefore cancels the RuleGo context and sends the
protocol cancel frame. `ffmpegOverIpResponse` writes and flushes stdout chunks;
stderr and terminal records still reach the end callback but never enter the
HTTP body.

The example sets `writeTimeout` to 86400 seconds so long streams are not cut
off by RuleGo's 10-second default (`writeTimeout: 0` selects that default).

Example request (the arguments are illustrative, not plugin policy):

```sh
curl --no-buffer http://127.0.0.1:8080/ffmpeg \
  -H 'Content-Type: application/json' \
  --data-binary '{"program":"ffmpeg","args":["-i","input","-f","matroska","pipe:1"]}' \
  --output output.mkv
```

The pinned REST endpoint reads the request body before creating the RuleGo
message. Deployments accepting large `stdinBase64` values must enforce their
request-size limit before RuleGo.
