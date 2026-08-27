# Compatibility

The v0.4.1 release consumes the immutable RuleGo Plugin ABI contract in
[`plugin-abi-release.json`](plugin-abi-release.json). That record is the single
authority for the exact SDK and runtime image digests, ABI ID, lock digest, and
packaging revision; it is shipped unchanged with the plugin release.
The compatible formal RuleGo release is `v0.37.0`.

CI uses the digest-pinned SDK to build the native Linux amd64 and arm64 plugins
and their `.sha256` and `.abi.json` sidecars. It then uses the digest-pinned
runtime to load each plugin and verify `ffmpegOverIp`,
`ffmpegOverIpProducer`, and `ffmpegOverIpResponse`. Install only the
architecture-matching plugin whose checksum and ABI sidecar verify against the
bundled release record.

Go plugins require the host and plugin to share the complete ABI contract.
Stock RuleGo binaries and other hosts are not claimed as compatible. The
ffmpeg-over-ip wire contract remains v5.2.1, protocol `0x06`, revision
`ab7adfeedf2a50f7e5807beef9088609cce645d6`.
