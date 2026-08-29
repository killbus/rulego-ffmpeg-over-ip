# YouTube indexed VOD acceptance evidence

## Selected capability

The accepted path is:

```text
stable YouTube URL
  -> rule-owned yt-dlp resolution and format policy
  -> normalized short-lived video/audio lease
  -> indexedVod inspect or produce one requested member
  -> resourceOrigin publication and lifecycle
  -> RuleGo native static mapping
```

The selected live sample is `Z4tHPyZBC8g`. The rule selected separate HTTPS
representations:

| Role | Format | Container | Codec | Resolution | Reported full size |
| --- | --- | --- | --- | --- | ---: |
| video | 299 | MP4 | H.264 High (`avc1.64002a`) | 1920x1080, 60 fps | 1,312,863,446 B |
| audio | 140 | M4A | AAC-LC (`mp4a.40.2`) | audio only | 40,930,848 B |

Both representations expose usable ISO BMFF indexes. `indexedVod` validates
the indexes, derives one immutable revision from stable media facts, and
returns a 524-member, 2,528.966663-second timeline. Direct URLs and headers are
access leases only: they are excluded from the revision and are refreshed by
the rule when stale.

For one requested member, `indexedVod` reads only both initialization sections
and the indexed video/audio ranges overlapping that member. It then invokes
FFmpeg with `-c copy` to mux those bounded local fragments into one MPEG-TS
member. It neither transcodes nor materializes either complete representation.

This is a source-neutral indexed-media capability. YouTube remains in the
resolver and rule policy; `resourceOrigin` remains unaware of providers,
formats, timelines, codecs, FFmpeg, and HLS.

## Reproducible runtime shape

The local production-shaped runtime used the released plugin-enabled RuleGo
host with:

- one shared `indexedVod` owner;
- one shared `resourceOrigin` owner;
- `resource_mapping = /resources/=./data/resource-origin/ready/`;
- `share_http_server = true`; and
- the released YouTube HLS example rule.

The resolver was `http://ytdlp.docker.pve:8080/run`. Cookies, signed URLs,
headers, and the FFmpeg secret remained in ignored `tmp/` or runtime files and
are not reproduced here.

## Playback and latency evidence

| Operation | Result |
| --- | --- |
| cold manifest, including yt-dlp, index inspection, and initial member | HTTP 200 in 8.525 s; 524-entry VOD manifest |
| equivalent concurrent requests for member 417 | both HTTP 200 in 1.60 s; both 2,534,992 B with identical SHA-256 |
| warm distant member 372 | HTTP 200 in 1.798 s; 3,132,456 B; 5.407711 s |
| cold member 300 immediately after RuleGo restart | HTTP 200 in 8.505 s; 5,407,444 B; 6.941089 s |
| mpv `--start=1800 --length=3` | exit 0; requested members 372-374, whose first timeline starts at 1,796.999992 s |

`ffprobe` identified the produced members as MPEG-TS containing 1920x1080
H.264 video and AAC audio. The observed manifest target duration is 7 seconds.
The mpv run reached the distant indexed position directly; it did not replay
from zero.

## Actual upstream bytes

A temporary read-only counting transport exercised the released source logic
against a fresh normalized lease. The probe was removed after the measurement.

| Indexed operation | Upstream bytes read |
| --- | ---: |
| inspect both indexes | 131,072 |
| produce member 0 inputs | 2,585,396 |
| cold member 0 total, including inspection | 2,716,468 |
| produce member 372 inputs | 3,215,880 |
| cold member 372 total, including inspection | 3,346,952 |

For comparison, local FFmpeg was given the same already-resolved leases and
asked to seek and stream-copy without indexed prefetch:

| Generic FFmpeg media stage | Wall time | Upstream bytes read | Output |
| --- | ---: | ---: | --- |
| initial 3.066667 s request | 7.287 s | 2,768,880 | 2,785,220 B / 3.083333 s |
| 1,797 s seek with 5.4 s limit | 37.753 s | 7,006,731 | 6,458,176 B / 10.435866 s |

The indexed path is the accepted seek implementation because it uses the
provider's immutable byte index to bound both transfer and output before
FFmpeg starts. FFmpeg remains a compatibility muxer, not the seek planner.

## Demand, publication, and lifecycle evidence

- The manifest advertises stable demand routes, not signed provider URLs.
- Before mpv playback, static storage contained only committed `0.ts` and the
  explicitly requested `417.ts`. The mpv seek added only requested members
  `372.ts`, `373.ts`, and `374.ts`; no complete upstream file or run-to-EOF
  artifact appeared.
- Two concurrent requests for member 417 returned the same bytes and resource
  identity, demonstrating one shared generation.
- Static publication returned `206` with
  `Content-Range: bytes 0-1023/2580488`, `416` with
  `Content-Range: bytes */2580488`, and `304` for an unchanged
  `If-Modified-Since` request.
- After a RuleGo process restart, previously committed `0.ts` and `417.ts`
  remained addressable and byte-identical. A new distant request resolved a
  fresh source lease and completed normally.
- A production-shaped rule with a 15-second parent TTL returned a complete
  manifest, then rejected the member route with `parent_unavailable` after
  expiry; the former static member URL returned 404. The temporary rule was
  deleted after the check.

## Release and deployment evidence

The published releases are:

- `rulego-ffmpeg-over-ip` v0.5.1;
- `rulego-indexed-vod` v0.2.0; and
- `rulego-resource-origin` v0.1.1.

Their GitHub CI and release workflows succeeded. The pinned amd64 and arm64
runtime checks co-loaded the supported release set in both filename orders.
The online RuleGo instance registers `ffmpegOverIp`,
`ffmpegOverIpProducer`, `indexedVod`, `resourceOrigin`,
`ffmpegOverIpResponse`, and `resourceOriginResponse` together.

The online `sxYw0hmQDtSX` rule now retains its two forward-only mux routes and
uses the indexed path for `/youtube/:videoId/index.m3u8` and stable member
routes. The RuleGo host publishes ready members through the configured mapping
while preserving its editor mappings:

```ini
resource_mapping = /resources/=./data/resource-origin/ready/,/editor/=./editor,/images/=./editor/images
```

After host restart, the manifest returned HTTP 200, a member route projected a
ready descriptor through `307`, and its `/resources/...` target returned the
complete MPEG-TS member with HTTP 200. A range request returned the expected
HTTP 206 and exact `Content-Range`. This setting belongs to the RuleGo host
because `resourceOrigin` deliberately does not serve bytes or open a listener.

Final Emby/Kodi playback remains operator-owned acceptance. The direct mpv
command after the host mapping is active is:

```text
mpv --no-ytdl http://rulego.docker.pve:6333/youtube/Z4tHPyZBC8g/index.m3u8
```
