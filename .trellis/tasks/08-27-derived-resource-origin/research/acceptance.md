# Acceptance evidence

Evidence was collected on 2026-08-29 against the published plugin releases and
the pinned RuleGo v0.37.0 plugin runtime.

| Criterion | Status | Evidence |
| --- | --- | --- |
| AC1 | Pass | `resourceOrigin` publishes a generic file independently of the caller node; unit and runtime tests resolve it through the shared owner until absolute expiry. |
| AC2 | Pass | Hermetic CI proves full GET, valid `206`, invalid `416`, exact lengths, `Last-Modified`, conditional `304`, and ready-only publication. |
| AC3 | Pass | Unit tests cover multi-member atomic publication and parent-bound demand. Hermetic CI publishes an eight-member virtual HLS manifest, produces only requested complete MPEG-TS members, and proves a broken static mapping makes the final fetch fail. |
| AC4 | Pass | Origin unit tests prove one generation and waiter behavior. Hermetic CI sends two simultaneous distant-member requests, receives byte-identical results, and observes only one bounded production; the live 524-member probe independently matched this behavior. |
| AC5 | Pass | Provider and origin tests enforce runtime, publication, extent, retained-byte, and expiry limits. Hermetic CI requires every source request to be a closed Range strictly smaller than the source and observes only demanded members rather than an EOF artifact. |
| AC6 | Pass | Restart/reconciliation unit tests preserve valid ready sets and remove owned orphans. Hermetic CI restarts RuleGo with the same origin volume and preserves the manifest, static URL, and member bytes without another non-index media production. |
| AC7 | Pass | Unit tests cover pending, failed, expired, unknown, stale generation, and timeout outcomes. Hermetic CI uses a short-lived parent and proves `parent_unavailable`, old-static-URL `404`, and no additional source read after expiry. |
| AC8 | Pass | `ffmpegOverIp` remains a stateless incremental stdout stream. The online rule retains both forward-only mux routes and uses no origin state on that branch. |
| AC9 | Pass | `youtube-index-native-feasibility.md` records first-ready/cold-seek times, stream-copy mode, exact indexed upstream bytes, and a same-lease generic FFmpeg comparison. |
| AC10 | Pass for protocol scope; terminal behavior owner-scoped | Hermetic CI uses separate H.264/AAC indexed inputs, survives `429`, `503`, truncated-body and connection-abort faults within two attempts, and remotely seeks/decodes both streams through the terminal HLS. Focused tests cover stale-lease refresh and revision checks. The live manifests passed ffprobe/mpv and Emby's server-side seek path. Emby Web's interactive failure reproduced with already-ready RuleGo members and was isolated to Emby's own remux/transcode startup watchdog; Kodi remains user-owned and is not claimed here. |
| AC11 | Pass | `resourceOrigin` uses RuleGo static mapping and opens no listener. `indexedVod` exists only because stable source-range planning and atomic member assembly are not supplied by the origin, static server, or generic execution node. |
| AC12 | Pass | Published ffmpeg-over-ip v0.5.1, indexed-vod v0.2.0, and resource-origin v0.1.1 artifacts co-loaded on amd64 and arm64 in both filename orders. The hermetic amd64 job additionally loads all four nodes and both response processors before exercising the full media path. |

## Runtime observations

- Hermetic acceptance owner: `rulego-indexed-vod@46d981f`, CI run
  `33250440019`, job `Hermetic HLS seek` (`99095241826`), all green.
- Hermetic media: eight two-second members; concurrent member 6 requests were
  byte-identical and one eligible production; remote FFmpeg sought to 12
  seconds and decoded both streams.
- Hermetic fault schedule: one `429`, one `503`, one truncated body, and one
  connection abort, each recovered on the second attempt. Every media request
  was a closed Range smaller than its source.
- Hermetic restart/expiry: stable manifest, member URL, and bytes survived
  restart; an expired parent returned `parent_unavailable`, its old static URL
  returned `404`, and no source read occurred. The broken-mapping control also
  returned `404` at the final data plane.

- Manifest: HTTP 200, 62,911 B, 524 segments, 2,528.966663 s total.
- mpv seek: `--start=1800 --length=3` exited 0 and requested members
  372-374; member 372 starts at 1,796.999992 s.
- Produced media: MPEG-TS, H.264 1920x1080 plus AAC.
- Indexed cold transfer: 131,072 B index inspection plus 3,215,880 B
  member-372 input.
- Generic FFmpeg 1,797-second stream-copy seek: 37.753 s and 7,006,731 B
  input, producing a 10.435866-second result for a 5.4-second request.
- Static data plane: `206 bytes 0-1023/2580488`,
  `416 bytes */2580488`, and conditional `304`.

## Published artifacts

- `rulego-ffmpeg-over-ip` v0.5.1, commit `2729a77`, CI
  `33233394310`, release workflow `33233607236`.
- `rulego-indexed-vod` v0.2.0, commit `88434c8`, CI
  `33228435892`, release workflow `33228711756`.
- `rulego-resource-origin` v0.1.1, commit `2489c73`, CI
  `33122215716`, release workflow `33122654665`.

The indexed-vod release bytes remain unchanged. Post-release acceptance commit
`46d981f` added the hermetic CI/tests, and `51ad260` added only the YouTube
example-chain lease/manifest reuse plus its script regressions; neither changes
the `indexedVod` node implementation. The latter passed GitHub CI run
`33263193567`, while the former verified the v0.2.0 release artifact and its
two published peers in run `33250440019`. A new plugin release is therefore
neither required nor implied by these composition/test-only commits.

## Online deployment state

The online RuleGo instance loads the exact four node types and the two response
processors. Shared owners `resource-origin` and `indexed-vod` are persisted,
and rule `sxYw0hmQDtSX` retains its two forward-only mux routes while its HLS
branch uses the released indexed path. The host publishes ready resources with
the following static mapping alongside its editor mappings:

```ini
resource_mapping = /resources/=./data/resource-origin/ready/,/editor/=./editor,/images/=./editor/images
```

After restart, the online manifest returned HTTP 200. Member 0 returned the
expected `307` projection and its `/resources/` target returned HTTP 200 with a
2,580,488-byte MPEG-TS file. A range request returned `206 bytes
0-1023/2580488`. Two concurrent requests for distant member 417 produced
identical 2,534,992-byte results with SHA-256
`393325e97cd2850ce2f3c26a0372d231df9941f854df070803f809bf9f3ce6be`.
Finally, mpv `--start=1800 --length=4` exited 0 through the public manifest,
confirming that the online host can acquire and serve a distant seek window.

The production host now uses its main `:80` endpoint as the shared owner. The
operator enabled the startup-level `share_http_server` INI flag and restarted
the host; `GET /api/v1/shared-nodes/%3A80/endpoint` then returned `200`. The
coordinator verified that rule `sxYw0hmQDtSX` still had exactly one HTTP
endpoint with `server: ":6333"`, changed only that field to `ref://:80`, and
confirmed the authoritative rule read returned the new value. The old
dedicated `:6333` listener stopped accepting connections.

The shared route then returned manifest `200`, member projection `307`, and a
same-authority `/resources/...` target with an exact 32-byte `206` range.
`mpv --no-ytdl --start=60 --length=3` against
`http://rulego.docker.pve/youtube/Z4tHPyZBC8g/index.m3u8` exited `0`. This is
not a plugin limitation or dynamic global: `share_http_server` is read from
the startup INI, while `/api/v1/config/global` only persists expression
globals to `data/config.json`.

The resulting 1,055-line production manifest contained no upstream URL,
authorization/cookie marker, transport name, or secret marker. This proves the
public media response boundary; RuleGo administrative configuration endpoint
access remains a separate host/operator authentication concern.

The production RuleGo composition now retains the normalized YouTube source
lease and its generated manifest until the earliest selected signed-URL expiry
minus a five-minute margin, capped at six hours; sources without a usable
expiry retain the conservative ten-minute fallback. The manifest cache is
accepted only while its matching revision-bound source lease exists, and
neither manifest nor member cache hits extend the absolute lease expiry. After
deployment, the first manifest request completed in 12.209 s and the next in
7.1 ms with identical 2,392-byte content and SHA-256
`ddb3e37429f6df42a100af92bc5fe6fbbfa14f25016ad0bfeb251dffa3b0a6a9`.
A cold request for member 18 produced only `18.ts` in 11.703 s; its ready read
then completed in 9.9 ms. Two simultaneous cold requests for member 9 waited
for one generation and returned the same resource ID
`fd34615d22e8c2f14fe7b66e00fa2299d389392cf04f02f66fe6ce1bcde5b67f`.
An mpv seek to 86 s decoded successfully through the public manifest. These
changes are confined to the YouTube rule graph and do not add state or policy
to `indexedVod`, `resourceOrigin`, or `ffmpeg-over-ip`.

The digest-pinned local runtime independently proves the same shape. With the
same plugin set, static mapping, `share_http_server = true`, and an endpoint
referencing the configured main listener, the system pool contained that
endpoint and the YouTube HLS route returned `200` on the main listener. Its member route
returned a relative `307` to `/resources/...`, so manifest, demand route, and
static bytes shared one authority without a second HTTP server.

The pinned v0.37.0 host returned `405` for static `HEAD`; this is an
informational host-version compatibility observation, not part of the
task-owned GET/Range/conditional contract and not a plugin or release
prerequisite. Static GET/Range responses also lacked CORS headers; same-origin
shared routing removes CORS as a browser prerequisite for this deployment.
