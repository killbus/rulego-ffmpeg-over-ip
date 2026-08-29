# Debt: Emby STRM/HLS terminal acceptance

## Status

The initial Emby HTTP 500 is resolved after correcting deployment-network
reachability from the Emby server/container to the RuleGo HLS route. No
Emby-specific change is indicated in `resourceOrigin`, `indexedVod`, or
`ffmpeg-over-ip`.

Interactive Emby Web seek was subsequently exercised after the shared-origin
cutover. The repeatable failure is inside Emby's own remux/transcode startup
path rather than the RuleGo member path; it does not justify an Emby-shaped
prefetch or session contract in any of the generic plugins. Emby for Kodi Next
Gen remains operator-owned validation. Static `HEAD` support is an independent
host-version compatibility observation, not part of this task's acceptance.

## Failure and cause

On 2026-08-29, Emby Server 4.9.5.0 initially represented the `.strm` source as
`Container: "strm"` with an empty `MediaStreams` array. Its generated FFmpeg
command consequently contained `-vn -an -sn` and `/live.m3u8` returned HTTP
500.

A controlled reproduction through the dedicated Emby test user established
the cause: Emby's FFmpeg timed out connecting to the RuleGo host on port 6333.
The same HLS URL was reachable from the diagnostic host, so the failure was
specific to deployment-network reachability from Emby rather than the media
resource.

## Successful retest

After the network was corrected, the same Emby item and unchanged `.strm` URL
produced the following results:

- `PlaybackInfo` classified the source as `Container: "hls"`;
- runtime was 94.260831 seconds;
- stream 0 was 1920x1080 H.264 video;
- stream 1 was stereo 44.1 kHz AAC audio;
- Emby's `/live.m3u8` returned HTTP 200 in 9.831 seconds with an HLS event
  manifest;
- the first Emby-produced segment was 2,611,132 bytes; and
- `ffprobe` found the expected H.264 video and AAC audio in that segment.

A separate cold seek probe followed Emby's documented
`master.m3u8 -> main.m3u8` path with `StartTimeTicks=600000000`. The generated
VOD media playlist preserved the request as
`#EXT-X-START:TIME-OFFSET=60`. Requesting the corresponding target member
(`20.ts`) immediately in the new session returned HTTP 200 in 6.209 seconds.
The 2,832,596-byte result contained H.264 and AAC with media timestamps in the
requested interval. This verifies the Emby server-side seek data path for the
94-second test item; it does not substitute for interactive Web/Kodi seek
acceptance or prove constant-cost behavior for a long terminal-side remux.

This also proves that `HEAD index.m3u8 -> 405` did not prevent Emby 4.9.5.0
from probing this source once it could connect; it is not the cause of this
Emby failure.

## Emby Web seek diagnosis after shared origin

The browser requested Emby's own
`/emby/videos/47835/hls1/main/23.ts` and HLS.js eventually reported
`fragLoadTimeOut`. The canceled browser request is a consequence, not an
upstream RuleGo segment request.

A fresh authenticated reproduction followed Emby's generated
`master.m3u8 -> main.m3u8 -> 23.ts` path. Emby advertised 32 three-second VOD
members and translated member 23 into an FFmpeg input seek at 69 seconds. With
the browser's observed 6.93 Mbps output limit, Emby explicitly disabled video
and audio stream copy and invoked `libx264` for the 1920x1080 59.94 fps input.
The request returned HTTP 500 after approximately 30.29 seconds because Emby
had not emitted its first output member before its own startup watchdog.

This remained reproducible after the exact RuleGo members Emby consumed were
already committed: members 14 through 16 each followed `307` to static `200`
in 12--17 ms and passed `ffprobe`, while three fresh Emby member-23 attempts
still failed at approximately 30.29 seconds. That A/B result excludes current
RuleGo production latency as the persistent cause.

A second controlled request raised only the Emby bitrate ceiling and allowed
stream copy. Emby then selected `-c:v copy -c:a copy`, confirming that
re-encoding was a terminal policy decision. Its direct-stream path nevertheless
uses a shorter, approximately 15-second startup bound and opens the first HLS
members for input discovery before jumping to the requested member. This is a
valid terminal implementation choice, but making the source origin preproduce
terminal-specific probe members would contradict the selected one-member-per-
demand contract. No such project change was retained.

One independent transient RuleGo demand returned `502` before a bounded retry
obtained the same committed member through `307 -> 200`; later cold members
completed in 1.66--4.01 seconds. That isolated recoverable event did not explain
the deterministic warm-member Emby failures and is not attributed to them.

## Debt to close

1. If desired, validate Emby Web with an operator-selected playback profile
   that permits direct play for the original H.264/AAC bitrate. This is an
   Emby policy check, not a plugin acceptance gate.
2. Repeat playback/seek through Emby for Kodi Next Gen.
3. Do not add terminal-specific bootstrap members, sessions, or transcoding
   policy to the generic RuleGo plugins to compensate for Emby's watchdog.

## Shared-origin deployment

The deployment now exposes both `/youtube/` and `/resources/` through
`http://rulego.docker.pve` on the shared main RuleGo endpoint. Rule
`sxYw0hmQDtSX` references that owner as `ref://:80`; the prior dedicated
`:6333` listener is gone. A main-origin manifest, relative member route,
same-origin static Range request, and mpv seek to 60 seconds all passed.

This removes cross-origin access from the active browser path, so adding CORS
to static bytes is no longer a prerequisite for this deployment. Existing
`.strm` files that include `:6333` must be changed to the main-origin URL before
terminal validation. Emby Web's observed seek failure is owner-scoped above;
Emby for Kodi Next Gen remains untested.
