# Compatibility

The v0.1.1 plugin ABI is pinned to this tuple:

- Go 1.25.0, Linux, `CGO_ENABLED=1`, `-buildmode=plugin`
- RuleGo server revision `3bf4ac47bb49aff9fe048e35644a6bca6e8e2af3`
- RuleGo core `v0.37.1-0.20260816112453-8995627f6da7`
  (`8995627f6da7bd6d819475373c324cf249af0a13`)
- ffmpeg-over-ip v5.2.1, protocol `0x06`, revision
  `ab7adfeedf2a50f7e5807beef9088609cce645d6`

Go plugins require the host and plugin to use the same toolchain, architecture,
build mode, and dependency graph. The stock RuleGo release binaries are built
with `CGO_ENABLED=0` and cannot load this plugin. Use the source-built host
verified by CI; CI builds both the host and plugin from one Go workspace so
their selected dependency graph is identical. Choose the artifact matching
`linux-amd64` or `linux-arm64`.
