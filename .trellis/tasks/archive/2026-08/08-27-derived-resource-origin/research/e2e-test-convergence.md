# Research: Hermetic end-to-end HLS seek convergence

- Query: What is the minimum repeatable regression that proves real SIDX range reads, real ffmpeg-over-ip production, pinned RuleGo, all three plugins, manifest/static redirects, and a distant HLS seek?
- Scope: mixed (five checked-out repositories plus immutable published image/release contracts)
- Date: 2026-08-29
- Status: implemented in `rulego-indexed-vod@46d981f`; GitHub CI run
  `33250440019` passed on the exact revision, including the `Hermetic HLS seek`
  job.

## Findings

### Conclusion

The regression is implemented without live YouTube, yt-dlp, a new product
server, or a committed media blob. One Linux/amd64 Docker integration job now:

1. uses one digest-pinned RuleGo runtime whose exact revision has first been proven to support the shared-main-server contract, plus ABI-matched plugin artifacts;
2. loads `ffmpegOverIp`/`ffmpegOverIpProducer`, `indexedVod`, and `resourceOrigin` together;
3. starts the real ffmpeg-over-ip v5.2.1 server;
4. generates small H.264 and AAC fragmented MP4 fixtures through the real ffmpeg-over-ip client/server, using `-movflags +dash+global_sidx`, fixed two-second fragments, fixed GOPs, and deterministic lavfi inputs;
5. serves those files from a local fault-injecting HTTP resolver/range fixture under Docker DNS name `ytdlp`, using the released `examples/youtube-hls/chain.json` without maintaining a second composition;
6. enables `share_http_server = true`, maps `/resources/` only to the origin `ready/` directory, installs the shared `indexed-vod` and `resource-origin` owners in `node_pool.json`, and imports the chain;
7. fetches the manifest, requests a member near 75% of the timeline, follows the `307` to the native static mapping, and seeks/decodes both streams through the pinned remote FFmpeg client; and
8. asserts from the fixture request log that SIDX and media requests were byte ranges, the distant nonzero member was produced, no full source was fetched, and injected `429`, `503`, early-body disconnect, and connection-close failures each recovered within the implementation's three-attempt bound.

This crosses every real boundary while keeping the media source and failure
schedule local and deterministic.

### Existing test infrastructure

- **`indexedVod` owner:** the pre-existing jobs in `rulego-indexed-vod/.github/workflows/ci.yml:20-146` validate metadata/JSON, run format/vet/unit/race tests, build the plugin with the pinned SDK, and smoke-load `indexedVod` plus a shared-node borrower. `source_test.go:175-225` unit-tests bounded retries for `429`, `5xx`, network errors, truncated bodies, and early EOF. `producer_test.go:68-99` uses strict local range bytes and a mocked ffmpeg call; `:188-221` tests ffmpeg transport/server/timeout retry policy with a mock. The new job adds the previously missing cross-plugin media execution.
- **`resourceOrigin` owner:** `rulego-resource-origin/.github/workflows/ci.yml:20-38` runs metadata, unit, and race checks; `:40-137` builds and individually smoke-loads the origin plugin. `origin_test.go:40-93` proves acquire/commit/singleflight and stable URLs in process, while `:95-141` and `:261-384` cover multi-member restart, parent expiry, and reconciliation. No test crosses an HTTP endpoint or native static mapping.
- **ffmpeg-over-ip owner:** `rulego-ffmpeg-over-ip/.github/workflows/ci.yml:46-95` runs a real pinned upstream ffmpeg-over-ip server against the client library. `:157-221` downloads verified indexed-vod v0.2.0 and resource-origin v0.1.1 releases and checks both plugins use the identical `github.com/killbus/rulego-ffmpeg-over-ip v0.5.0` client identity. `:231-290` co-loads all three plugins in both filename orders and asserts four nodes, two response processors, and no loader errors. This remains the ABI/co-load owner, but it stops at registration.
- `rulego-server-docker/scripts/verify-plugin-runtime.sh:15-70` provides reusable pinned-runtime plugin-load mechanics, but accepts only one plugin. `scripts/verify-build.sh:84-113` proves the packaged server starts on port 9090. The repository does not contain a cross-plugin rule-flow test.
- `ffmpeg-over-ip-docker/scripts/verify-integration.sh:14-42` proves published client-to-server `ffmpeg -version` on amd64 and arm64 only. Its retry library classifies transient curl failures plus HTTP `408`, `425`, `429`, and `5xx` (`scripts/lib.sh:43-61`) and implements bounded exponential backoff (`:64-120`, `:203-215`).

`rulego-indexed-vod/.github/workflows/ci.yml` now performs SIDX discovery,
media-range acquisition, remote muxing, origin commit, HTTP redirect/static
delivery, and distant HLS seek in one run. The pre-existing unit and release
checks above remain the owning narrower gates rather than being duplicated in
other repositories.

### Runtime and composition evidence

- `rulego-indexed-vod/sidx.go:41-76` discovers a top-level SIDX from byte ranges; `:109-182` rejects indirect/empty references and requires SAP type 1 for video. `producer.go:178-202` copies only initialization bytes plus selected indexed references, and `:205-270` requires correct `206 Content-Range` responses.
- Source HTTP operations retry at most three times with 100 ms then 200 ms delays (`source.go:196-247`); range-body disconnects and `429`/`5xx` are retryable, while `401`, `403`, `404`, and `410` are immediate stale-lease failures. The produced FFmpeg invocation is also retried at most three times only for transport/server/timeout classes (`producer.go:109-127`, `:272-300`).
- Actual segment production stream-copies one video reference and only overlapping audio references into one MPEG-TS file (`producer.go:53-108`), so a nonzero requested segment is direct evidence of distant indexed production rather than download-to-EOF.
- The shipped HLS chain already exposes a manifest and member route on `server: "ref://:9090"` (`examples/youtube-hls/chain.json:11-48`), builds a VOD manifest containing stable same-authority `/youtube/...` member URLs (`:120-125`), and maps ready resources through `resourceOriginResponse` (`:37-45`). It resolves/produces the initial member separately and acquires nonzero children under the parent (`:142-194`).
- `resourceOriginResponse` emits `307 Location: <relative static URL>` for ready resources (`rulego-resource-origin/plugin.go:29-68`). Origin commit atomically renames staging to ready and returns the configured `/resources/<id>/<member>` URL (`rulego-resource-origin/origin.go:321-386`; URL contract tested at `origin_test.go:40-91`).
- All three plugins record the same ABI ID and exact RuleGo runtime digest: `ghcr.io/killbus/rulego-server@sha256:8594e773b9d0cf2afa1fd8af0744b9eea5a500006e8847c149ed36cc1fcb559a` (`plugin-abi-release.json:2-6` in each plugin repository). The formal host release is RuleGo v0.37.0 (`compatibility.json:4` in each repository).

### `share_http_server` and `ref://:9090`

Yes: this is RuleGo Server's native same-origin path, not a second listener. The packaged configuration sets `server = :9090`, documents the capability, and leaves the deployment opt-in at `share_http_server = false` (`rulego-server-docker/build/amd64/rootfs/config.conf`). The exact pinned RuleGo v0.37.0 source revision `a7be24c4c1f649d422b41eb623d1d6e314b20c58` contains the configuration field, creates the main REST endpoint from `config.Server`, and injects it into each user pool when sharing is enabled (`server/config/config.go`, `server/bootstrap/bootstrap.go`, `server/internal/engine/manager.go`). The HLS chain references that exact resource ID as `server: "ref://:9090"`.

Because manifest/member paths and origin `Location` values are relative, client requests remain on the same RuleGo authority. The production host uses `server = :80`; after enabling `share_http_server` and restarting, its system pool exposed `:80`, and the production chain was changed from `:6333` to `ref://:80`. The different `:9090` identity below belongs only to the digest-pinned local fixture because that fixture's configured main listener is `:9090`.

The exact released image `ghcr.io/killbus/rulego-server@sha256:8594e773b9d0cf2afa1fd8af0744b9eea5a500006e8847c149ed36cc1fcb559a` was started locally with only `share_http_server = true` changed. A rule using `server: "ref://:9090"` saved successfully and its route returned HTTP 200 with body `shared-ok` and `Access-Control-Allow-Origin: *`. The pinned runtime therefore already supplies the capability; no ABI migration, plugin change, or new image variant is required. The production `ref://:80` cutover subsequently returned manifest `200`, member `307`, same-authority static Range `206`, and a successful mpv seek; the former `:6333` listener closed.

### Proven fixture shape

A local experiment against the immutable multi-arch images succeeded:

- ffmpeg-over-ip server: `ghcr.io/killbus/ffmpeg-over-ip-server@sha256:61eb8c18b031b01d4d5a3de8ccc1691314fccb03404d7c6192b436b97ad63427`
- ffmpeg-over-ip client: `ghcr.io/killbus/ffmpeg-over-ip-client@sha256:58b5061521d705e1bc808d487242759914541f330de3b33be98b73ce367d8f2a`
- A 16-second, 160x90 H.264 fixture generated through that pair was 591,686 bytes, contained eight `moof` boxes, and had `sidx` at byte 779.
- Its 48 kHz AAC/M4A companion was 133,486 bytes, contained eight `moof` boxes, and had `sidx` at byte 716.

Both exceed the plugin's 64 KiB initial probe and therefore exercise a real `206` response of exactly the requested length (`sidx.go:13-16`, `:185-207`). Eight two-second fragments are already sufficient to select a clearly nonzero target around 75% of the 16-second timeline; the committed regression can retain this small, deterministic shape rather than committing a media blob or lengthening the fixture without evidence.

### Implemented surface

The ownership-aligned home is `rulego-indexed-vod`: that repository owns
indexed seek semantics and the complete HLS composition example.
ffmpeg-over-ip CI remains the ABI/co-load owner. The implementation adds only:

1. `tests/e2e/fixture-server.go`: standard-library server implementing `/run`, strict byte ranges for `/video.mp4` and `/audio.m4a`, deterministic one-shot fault injection keyed by path/range, and a JSON request log/counters endpoint. It returns the existing yt-dlp-shaped boundary with local media URLs.
2. `tests/e2e/fixture-server_test.go`: focused tests for strict Range parsing and the fixture-only SIDX SAP normalization.
3. `tests/e2e/config.conf` and `tests/e2e/node-pool.json`: the smallest shared-HTTP runtime fixture with the two shared plugin owners.
4. `tests/e2e-hls-seek.sh`: one fail-fast repository command with traps and unique Docker names/network/temp directory. It pins the ffmpeg-over-ip images; reads the RuleGo runtime digest from `plugin-abi-release.json`; generates the 16-second fixtures through the real daemon; combines the candidate indexed-vod plugin with checksum/ABI-verified ffmpeg-over-ip v0.5.1 and resource-origin v0.1.1 releases; imports a temporary copy of `examples/youtube-hls/chain.json` with a short TTL only for the `expiry01` fixture; runs the assertions; and prints RuleGo/fixture/ffmpeg logs on failure. No second maintained chain is added.
5. `.github/workflows/ci.yml`: one amd64-only hermetic E2E job after the candidate plugin is built and smoke-loaded. It reuses the current run's build artifact and checkout example and fetches the two peer releases with bounded retry and checksum/ABI verification. It does not add a second architecture matrix or product image.

No product Go code, resource-origin code, RuleGo server code, docker packaging code, or new HTTP server is required for this regression.

### Enforced assertions and retry discipline

- Runtime images are digest-pinned; peer plugins are versioned and verified by checksum plus ABI sidecar; the candidate comes from the same CI run's amd64 build artifact.
- Image pulls, fixture-generation startup probes, and peer release downloads use bounded retries. Readiness polling has a fixed deadline and failure emits container logs.
- The fixture returns one `429`, one `503`, one truncated body, and one connection abort, then succeeds. The gate requires exactly two attempts for each injected failure; deterministic failures are not retried by the product unit tests.
- `/api/v1/components` must contain all four node types and both response processors, and startup logs must not contain Go-plugin fingerprint/load errors.
- The manifest must return `200`, contain VOD/endlist tags and at least eight relative members, and disclose no fixture URL, lease header, or secret.
- Two simultaneous requests for the 75%-timeline member must return identical bytes while request counts prove one eligible production. Every source request must be a closed byte range strictly smaller than its source file.
- The pinned remote FFmpeg client must seek to 12 seconds through the shared HLS URL and decode both the video and audio streams successfully.
- Restarting RuleGo with the same origin volume must preserve the manifest, static URL, and member bytes without another non-index media production.
- A short-lived fixture parent must expire; the member route then returns `parent_unavailable`, the old static URL returns `404`, and no additional source request occurs.
- Native static delivery must return `206` with exact range length, `416` for an unsatisfiable range, and `304` for `If-Modified-Since`. Manifest and member route responses must carry CORS from the shared RuleGo endpoint.
- A controlled broken `resource_mapping` restart must retain the plugin redirect but make its `/resources/` target return `404`, proving that the gate detects the deployment invariant.

### External references / versions

- RuleGo Server v0.37.0 / core pseudo-version `v0.36.1-0.20260802040353-2ec085f29027`, pinned by `rulego-server-docker/plugin-abi.lock:36-47`.
- ffmpeg-over-ip v5.2.1, protocol v6, upstream revision `ab7adfeedf2a50f7e5807beef9088609cce645d6`, recorded at `rulego-ffmpeg-over-ip/compatibility.json:5-9`.
- Go `net/http.ServeContent` is the native range/conditional implementation reached by RuleGo static mapping; the later checked-out server source calls it at `stream-prism/temp/server/internal/endpoint/static.go:98-127`.

### Related specs

- Task PRD R2/R6/R8/R10-R13 and AC2/AC3/AC5/AC9/AC10/AC12.
- Task design sections 2 (demand route), 3 (publication), 5 (HTTP composition),
  6 (provider integration), and 7 (YouTube specialization).
- Task implementation plan steps 1, 4, 6, and 7.
- `.trellis/spec/backend/quality-guidelines.md` and `error-handling.md` are placeholders; no additional project-specific test convention is currently documented.

## Caveats / Not Found

- The checked-in configuration defaults `share_http_server` to false. Deployments and the hermetic test must explicitly set it true and restart the process. A rule must reference the actual configured listener ID (`ref://:80` in production, `ref://:9090` in the fixture), not infer it from an external proxy or example default.
- The immutable v0.37.0 runtime returns `405` for static-resource `HEAD`; method
  registration beyond the task-owned GET/Range/conditional contract is
  host-version behavior and does not require a plugin or RuleGo-core change for
  this delivery. The runtime also omits CORS on static GET/Range responses;
  same-origin shared routing removes that as a browser prerequisite here.
- Existing source retry backoff is bounded but very short (100 ms, then 200 ms), and the example resolver path retries only once after 200 ms (`chain.json:72-95`). The deterministic fixture should remain within those bounds; production tuning is a separate product decision.
- The hermetic gate covers resource expiry, but signed source-lease expiry and
  re-resolution remain covered by focused stale-source/cache/revision tests
  rather than a second stateful resolver scenario.
- Real YouTube remains an opt-in, credential-driven probe. Cookies, provider/client availability, signed leases, and throttling are nondeterministic and must not gate pull requests; the local resolver/range fixture is the merge gate.
- Final Emby/Kodi behavior remains user-owned per the task PRD; the automated gate uses protocol assertions and FFmpeg decoding only.
