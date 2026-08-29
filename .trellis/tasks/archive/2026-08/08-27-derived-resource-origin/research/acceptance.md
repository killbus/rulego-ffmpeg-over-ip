# Acceptance evidence

Evidence was collected on 2026-08-29 against the published plugin releases and
the pinned RuleGo v0.37.0 plugin runtime.

| Criterion | Status | Evidence |
| --- | --- | --- |
| AC1 | Pass | `resourceOrigin` publishes a generic file independently of the caller node; unit and runtime tests resolve it through the shared owner until absolute expiry. |
| AC2 | Host prerequisite | Full GET, valid `206`, invalid `416`, accurate lengths, `Last-Modified`, and conditional `304` passed. The pinned RuleGo host registers only GET for static mappings and returns `405` for HEAD; HEAD remains host-owned. |
| AC3 | Pass | Unit tests cover multi-member atomic publication and parent-bound demand. The live HLS manifest exposed 524 stable member routes; only demanded complete MPEG-TS members appeared below `ready/`. |
| AC4 | Pass | Origin unit tests prove one generation and waiter behavior. Two simultaneous live requests for member 417 completed in 1.60 s with identical 2,534,992-byte output and SHA-256. |
| AC5 | Pass | Provider and origin tests enforce runtime, publication, extent, retained-byte, and expiry limits. Live static storage contained only requested members rather than an EOF artifact. |
| AC6 | Pass | Restart/reconciliation unit tests preserve valid ready sets and remove owned orphans. A live RuleGo restart preserved byte-identical `0.ts` and `417.ts`; a new cold distant member completed afterward. |
| AC7 | Pass | Unit tests cover pending, failed, expired, unknown, stale generation, and timeout outcomes. A 15-second live parent returned `parent_unavailable` after expiry and its old static member returned 404. |
| AC8 | Pass | `ffmpegOverIp` remains a stateless incremental stdout stream. The online rule retains both forward-only mux routes and uses no origin state on that branch. |
| AC9 | Pass | `youtube-index-native-feasibility.md` records first-ready/cold-seek times, stream-copy mode, exact indexed upstream bytes, and a same-lease generic FFmpeg comparison. |
| AC10 | Pass, terminal owner pending | Separate H.264/AAC inputs produced a standards-compliant 524-entry HLS VOD manifest. ffprobe and mpv distant seek passed; stale-lease retry and revision checks are covered by tests. Emby/Kodi validation remains user-owned. |
| AC11 | Pass | `resourceOrigin` uses RuleGo static mapping and opens no listener. `indexedVod` exists only because stable source-range planning and atomic member assembly are not supplied by the origin, static server, or generic execution node. |
| AC12 | Pass | Published ffmpeg-over-ip v0.5.1, indexed-vod v0.2.0, and resource-origin v0.1.1 artifacts co-loaded on amd64 and arm64 in both filename orders. All four nodes and both response processors registered; CI and release workflows succeeded. |

## Runtime observations

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
