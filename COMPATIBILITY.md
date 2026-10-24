# Compatibility

The v0.5.2 release consumes the immutable RuleGo Plugin ABI contract in
[`plugin-abi-release.json`](plugin-abi-release.json). That record is the single
authority for the exact SDK and runtime image digests, ABI ID, lock digest, and
packaging revision; it is shipped unchanged with the plugin release.
The compatible formal RuleGo release is `v1.0.0`.

CI uses the digest-pinned SDK to build the native Linux amd64 and arm64 plugins
and their `.sha256` and `.abi.json` sidecars. It then uses the digest-pinned
runtime to load this plugin alone and verifies that it registers
`ffmpegOverIp`, `ffmpegOverIpProducer`, and the `ffmpegOverIpResponse` out
processor. Co-existence with the other published plugins is a property of the
plugin *set*, not of this member: it is verified by
[`killbus/rulego-fleet`](https://github.com/killbus/rulego-fleet), which
declares the set and loads it into the pinned runtime. Install only the
architecture-matching plugin whose checksum and ABI sidecar verify against the
bundled release record.

Go plugins require the host and plugin to share the complete ABI contract.
Stock RuleGo binaries and other hosts are not claimed as compatible. The
RuleGo entrypoint is an independent Go module whose dependency on
`github.com/killbus/rulego-ffmpeg-over-ip v0.5.0` CI pins to a single compiled
identity (version + hash); any plugin loading the shared client at a different
version is rejected by the Go plugin loader, and `killbus/rulego-fleet`
additionally compares the compiled dependency records across the set.
The ffmpeg-over-ip wire contract remains v5.2.1, protocol `0x06`, revision
`ab7adfeedf2a50f7e5807beef9088609cce645d6`.
