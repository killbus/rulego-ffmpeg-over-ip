# YouTube source specialization boundary

## Responsibility map

```text
RuleGo route
  owns stable YouTube URL construction, yt-dlp request, cookies, and format policy
      |
      v
normalized indexed-media lease
  carries sourceKey plus current video/audio URL, headers, container, and codec
      |
      v
indexedVod
  validates immutable indexes and produces exactly one requested member
      |
      v
resourceOrigin
  owns stable public identity, readiness, expiry, storage bounds, and publication
```

The YouTube-specific facts end at lease normalization. Neither `indexedVod`
nor `resourceOrigin` contains a cookie path, yt-dlp argument, format selector,
video ID rule, player-client name, or YouTube URL template.

## Stable identity and rotating access

`sourceKey` names the logical provider asset. It does not select cached signed
URLs. `indexedVod` caches inspection by the complete normalized lease and
derives its revision from the stable source key, declared codecs/containers,
and immutable index bytes. URLs and headers are excluded from revision evidence.

The RuleGo chain may retain the complete normalized lease briefly as a
performance hint. Correctness never depends on that cache:

- cache hit: produce from the exact inspected lease;
- cache miss or RuleGo restart: call yt-dlp and inspect the fresh lease;
- 401/403/404/410 while reading a representation: emit `source_stale`;
- `source_stale`: evict, resolve once, re-inspect, and require the same
  revision before producing;
- changed immutable index evidence: emit `revision_changed` rather than attach
  different media to the published resource identity.

The public manifest contains only stable RuleGo demand routes. Signed URLs and
headers never appear in the manifest, resource ID, logs, or error payloads.

## Selected media operation

The proven sample supplies separate indexed MP4/H.264 and M4A/AAC
representations. `indexedVod` reads the two initialization sections and only
the indexed ranges overlapping the requested video member. FFmpeg then
stream-copies those bounded local fragments into one closed MPEG-TS file.

One request therefore has one atomic result:

```text
logical segment N
  -> exact indexed video range + overlapping audio range(s)
  -> one staging/N.ts
  -> close
  -> resourceOrigin commit
  -> 307 to native static mapping
```

No request starts a continuous session, generates adjacent members, or runs to
EOF. Concurrent requests for the same parent/revision/member share the same
origin generation. Different members remain independently addressable and
independently bounded.

FFmpeg is used for container compatibility, not source discovery, seek
planning, public serving, or lifecycle. The generic `ffmpegOverIp` RuleGo node
remains available for low-startup forward-only playback.

## Manifest and terminal contract

The rule builds an HLS VOD media playlist from the immutable indexed timeline:

- `EXT-X-PLAYLIST-TYPE:VOD` and `EXT-X-ENDLIST` expose total duration;
- every `EXTINF` uses the corresponding indexed duration;
- every URI points to a stable demand route containing video ID, revision, and
  segment number; and
- each demand route returns only a committed MPEG-TS member through
  `resourceOriginResponse`.

The terminal representation is H.264/AAC MPEG-TS HLS, not DASH. mpv/ffprobe
are automated integration evidence; final Emby/Kodi behavior remains the
operator's acceptance step.

## Upstream variants

Provider-supplied pre-merged HLS may be used only as an observed optimization.
It cannot define the baseline because player-client behavior changes and
higher resolutions commonly remain split. SABR and any particular yt-dlp
client are resolver concerns, not plugin contracts.

The baseline is the separately selected indexed video/audio lease. A provider
whose normalized representations do not expose compatible supported indexes
receives a typed unsupported-source result; the generic plugins do not invent
a codec profile or silently change the caller's selection policy.

## Operational bounds

- yt-dlp invocation timeout and parallelism belong to the RuleGo resolver node.
- Index inspection and member production deadlines belong to `indexedVod`.
- Per-resource bytes, total retained bytes, production deadline, and absolute
  expiry belong to `resourceOrigin`.
- HTTP byte serving and Range semantics belong to RuleGo's static mapping.
- Transient disconnects, 429 responses, and 5xx responses receive bounded
  retries; deterministic format, index, revision, path, and mux errors do not.

The measured playback, transfer, concurrency, restart, and expiry evidence is
recorded in `youtube-index-native-feasibility.md`.
