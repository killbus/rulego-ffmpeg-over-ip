# Compatibility

The v0.5.1 release consumes the immutable RuleGo Plugin ABI contract in
[`plugin-abi-release.json`](plugin-abi-release.json). That record is the single
authority for the exact SDK and runtime image digests, ABI ID, lock digest, and
packaging revision; it is shipped unchanged with the plugin release.
The compatible formal RuleGo release is `v0.37.0`.

CI uses the digest-pinned SDK to build the native Linux amd64 and arm64 plugins
and their `.sha256` and `.abi.json` sidecars. It then uses the digest-pinned
runtime to co-load this plugin with indexed-vod v0.2.0 and resource-origin
v0.1.1 in both relevant load orders. The gate verifies `ffmpegOverIp`,
`ffmpegOverIpProducer`, `indexedVod`, `resourceOrigin`,
`ffmpegOverIpResponse`, and `resourceOriginResponse`. Install only the
architecture-matching plugin whose checksum and ABI sidecar verify against the
bundled release record.

Go plugins require the host and plugin to share the complete ABI contract.
Stock RuleGo binaries and other hosts are not claimed as compatible. The
RuleGo entrypoint is an independent Go module whose dependency on
`github.com/killbus/rulego-ffmpeg-over-ip v0.5.0` is intentionally identical
to indexed-vod v0.2.0; CI compares the compiled dependency records before the
co-load gate. This preserves one package identity for the shared client.
The ffmpeg-over-ip wire contract remains v5.2.1, protocol `0x06`, revision
`ab7adfeedf2a50f7e5807beef9088609cce645d6`.
