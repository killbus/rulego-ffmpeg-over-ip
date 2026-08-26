# Delivery Governance Evidence

Canonical contract: `/home/agent/Src/stream-prism/.trellis/spec/guides/delivery-governance.md`

## Final Delivery Record (v0.2.0)

- Goal ID: `ffmpeg-over-ip-client-plugin-v1`
- Observable outcome: publish the same complete ffmpeg-over-ip RuleGo client
  capability as SDK-built native plugins that load in the host distributor's
  matching plugin runtime.
- Acceptance identity: R1-R13 and AC1-AC12 as currently recorded in `prd.md`.
- Authoritative build input: TEAM B workflow run `32948070511`, revision
  `a498c9e29b0121e5495557ecc88c4a5df9150637`, and its reviewed
  `plugin-abi-release.json` artifact.
- Delivered product identity: commit
  `318858fe8184db79645715e7246c1299c0ff7f6b`, annotated tag `v0.2.0` (peeled
  to that commit), and release REST ID `377052355`.
- Scope delivered: dependency alignment, release metadata, SDK build,
  matching-runtime load verification, and a new immutable plugin release.
- Non-goals: plugin behavior changes, another ABI schema, another build helper,
  a plugin container image, or rewriting `v0.1.1`.
- TEAM B authority: workflow run `32948070511`, packaging revision
  `a498c9e29b0121e5495557ecc88c4a5df9150637`, ABI record SHA-256
  `4054332fd2c9ba70630d556d69c95c0c0bda1958c8f6ded9230980b9d1d6f36b`.
- CI evidence: run `32957784185` completed successfully at the exact product
  commit. Its metadata, test/race, pinned-upstream, amd64 SDK/runtime, and arm64
  SDK/runtime jobs all succeeded.
- Release evidence: workflow `32958685299` verified the CI run and exact tag
  commit, downloaded its artifacts, checked SHA-256 files, and published
  without rebuilding.
- Delivered state: public non-draft, non-prerelease release at
  <https://github.com/killbus/rulego-ffmpeg-over-ip/releases/tag/v0.2.0>.
- Asset evidence: all 13 expected assets were downloaded under ignored
  `./tmp/release-v0.2.0-validation/`; both checksums passed, both ABI sidecars
  match the release record and plugin bytes, and `file` identified Linux x86-64
  and ARM aarch64 shared objects.
- Completion audit: fresh read-only auditor `sdk_completion_audit` (goal thread
  `01a03daa-f93e-7203-a896-700f248d39f2`) independently evaluated every row
  below and returned `CONTINUE` to `COMPLETE`.

## Gate and Retry Evidence

- TEAM B release evidence: workflow run `32948070511` succeeded at
  `a498c9e29b0121e5495557ecc88c4a5df9150637`; the downloaded release record is
  byte-identical to the committed `plugin-abi-release.json` and resolves to
  native Linux amd64/arm64 SDK and runtime indexes.
- Quality gate: `PASS`; lightweight module, formatting, JSON, version, stale
  reference, shell, workflow syntax, and diff checks passed. Heavy build and
  runtime checks remain assigned to GitHub Actions.
- Pre-commit path gate: independent read-only reviewer returned `CONTINUE` for
  candidate cursor `866db09d…e67f8ae`; the capability is unchanged, the ABI
  authority is singular, and no helper or local build path was added.
- Pre-tag path gate: the same reviewer returned `CONTINUE` for commit
  `318858fe8184db79645715e7246c1299c0ff7f6b` and CI run `32957784185`, after
  confirming `v0.2.0` was absent and `v0.1.1` remained unchanged.
- Retry evidence: implementer, quality checker, and path reviewer each resumed
  the same `ffmpeg-over-ip-client-plugin-v1` goal after provider disconnects or
  rate limits; no retry changed scope, authority, or candidate intent.
- Final transition: `DELIVER -> COMPLETE`; preconditions, delivered identity,
  and the full requirement matrix were proven.

## v0.2.0 Completion Matrix

- Goal ID: `ffmpeg-over-ip-client-plugin-v1`
- Delivery identity: `318858fe8184db79645715e7246c1299c0ff7f6b` /
  `v0.2.0` / release `377052355`.
- Auditor identity: fresh read-only `sdk_completion_audit`, goal thread
  `01a03daa-f93e-7203-a896-700f248d39f2`.

| ID | Current authoritative evidence | Status before audit |
| --- | --- | --- |
| R1 | Both native SDK artifacts loaded in the matching runtime; component API exposed `ffmpegOverIp` | PROVEN |
| R2 | Exact-argv and invocation-boundary tests passed in CI test job `98143204392` | PROVEN |
| R3 | Pinned ffmpeg-over-ip v5.2.1 integration job `98143204534` passed | PROVEN |
| R4 | Complete client session suite and race suite passed in CI | PROVEN |
| R5 | Stream ordering/channel tests passed in CI | PROVEN |
| R6 | Terminal outcome tests passed in CI | PROVEN |
| R7 | Cancellation/lifecycle tests passed in CI | PROVEN |
| R8 | Configuration tests passed; product configuration code is unchanged from v0.1.1 | PROVEN |
| R9 | Concurrency/resource tests and race suite passed in CI | PROVEN |
| R10 | Capability source is unchanged from v0.1.1 except its version literal; no media policy was added | PROVEN |
| R11 | Release exposes both native `.so`, SDK checksum/ABI sidecars, ABI record, compatibility/license files, and example | PROVEN |
| R12 | Runtime API exposed `ffmpegOverIpResponse`; REST projection tests passed | PROVEN |
| R13 | Pinned-upstream TCP and Unix integration passed | PROVEN |
| AC1 | Native amd64 job `98143204520` and arm64 job `98143204524` built via SDK, validated sidecars, and loaded via matching runtime | PROVEN |
| AC2 | Exact argv tests passed in CI | PROVEN |
| AC3 | Actual pinned-server auth/binary/error coverage passed in CI | PROVEN |
| AC4 | Progressive ordered stream tests passed in CI | PROVEN |
| AC5 | stdin/EOF test passed in CI | PROVEN |
| AC6 | Terminal and clock-driven liveness tests passed in CI | PROVEN |
| AC7 | Cancel/grace/race tests passed in CI | PROVEN |
| AC8 | Concurrent-session and bounded-allocation tests passed, including race mode | PROVEN |
| AC9 | Generic REST example is present in the release and its projection tests passed | PROVEN |
| AC10 | Release contains both architectures and all `.so`, `.sha256`, `.abi.json` artifacts; no plugin image was published | PROVEN |
| AC11 | Public node-contract tests passed in CI | PROVEN |
| AC12 | REST disconnect cancellation test passed in CI | PROVEN |

- Missing or contradicted claims: none.
- Final verdict: `CONTINUE` from `DELIVER` to `COMPLETE`.

## Prior Delivery Record (v0.1.1)

- Goal ID: `ffmpeg-over-ip-client-plugin-v1`
- Observable outcome: the released RuleGo plugin provides one complete
  ffmpeg-over-ip client session per message, streams stdout and stderr through
  RuleGo, reports one terminal result, and propagates cancellation without a
  local FFmpeg or an external client binary.
- Delivered product identity: revision and tag `v0.1.1` resolve to
  `8f7bd87aa0aceb30da1db272f0a778470c6fc673`.
- Branch history: remote `master` resolved to that product revision when the
  release was published; its only advancement from the release point is this
  administrative archive/journal commit, which changes no product code or
  release artifact.
- CI evidence: run `32787459529` completed successfully at that exact commit;
  all five jobs (`test`, `pinned-upstream`, `release-metadata`, Linux amd64
  build, and Linux arm64 build) succeeded.
- Release evidence: workflow `32788525850` completed successfully at that
  exact commit, verified and downloaded run `32787459529` artifacts, checked
  their SHA-256 files, and published without rebuilding.
- Delivered state: public non-draft, non-prerelease release REST ID
  `376044646`, tag `v0.1.1`, at
  <https://github.com/killbus/rulego-ffmpeg-over-ip/releases/tag/v0.1.1>.
- Asset evidence: the release API exposes ten assets with GitHub SHA-256
  digests. The downloaded assets under ignored
  `./tmp/release-v0.1.1-validation/` passed both checksum files; `file`
  identified the plugins as Linux x86-64 and ARM aarch64 shared objects; the
  compatibility metadata, license/notice, and generic RuleGo example matched
  the release contract.
- Completion audit: a fresh read-only completion auditor evaluated every row
  below against the exact delivered state and returned `CONTINUE` to
  `COMPLETE`; all R1-R13 and AC1-AC12 rows are `PROVEN`.

## Residual Boundary Ledger

- Compatibility is limited to the ABI tuple declared by release
  `compatibility.json`: Go 1.25.0, Linux amd64/arm64, `CGO_ENABLED=1`, plugin
  build mode, RuleGo server `3bf4ac47bb49aff9fe048e35644a6bca6e8e2af3`,
  and core `8995627f6da7bd6d819475373c324cf249af0a13`. Stock
  `CGO_ENABLED=0` RuleGo binaries are not compatible hosts.
- Operators own ffmpeg-over-ip server deployment, address, secret, concurrency
  policy, and the RuleGo process/container filesystem authority.
- Calling rule graphs own argv construction, media/container policy, response
  headers, routes, and downstream-player validation.
- The plugin owns only the RuleGo adapter, protocol-v6 client session, and the
  generic stdout REST projection. It does not claim server deployment or
  application-specific playback acceptance.
- The published release is immutable. Any semantic or artifact change requires
  a new commit, CI identity, tag, and release rather than rewriting `v0.1.1`.

## Failure and Retry Evidence

- Goal and acceptance identity remained
  `ffmpeg-over-ip-client-plugin-v1` / R1-R13 and AC1-AC12 throughout retries.
- Provider stream disconnects, HTTP 429, configured retryable 5xx, and HTTP 403
  carrying explicit rate-limit evidence were classified as `TRANSIENT` under
  GOV-09 and resumed automatically with the same bounded goal. They conferred
  no approval and did not change scope, implementation identity, or delivery
  identity.
- Authoritative repository and GitHub state was re-read after retry windows;
  the final identities recorded above, rather than an interrupted response,
  are the completion evidence.

## Completion Matrix

- Goal ID: `ffmpeg-over-ip-client-plugin-v1`
- Delivery identity: `8f7bd87aa0aceb30da1db272f0a778470c6fc673` /
  `v0.1.1` / release `376044646`.
- Auditor identity: fresh read-only completion auditor, final verdict
  `CONTINUE` to `COMPLETE`.

| ID | Authoritative source | Current evidence | Evidence identity | Status | Auditor rationale |
| --- | --- | --- | --- | --- | --- |
| R1 | `prd.md`; RuleGo plugin contract | `plugin.go` exports `Plugins`; both build jobs loaded `ffmpegOverIp` through the pinned host | commit `8f7bd87`; CI `32787459529` | PROVEN | The delivered artifacts are native RuleGo plugins exposing the generic client component. |
| R2 | `prd.md`; protocol command contract | Strict invocation validation and exact command encoding are covered by `TestInvocationBoundary` and `TestCommandPreservesArgvAndSignature` | commit `8f7bd87`; CI test job | PROVEN | Only `ffmpeg`/`ffprobe` are accepted and argv is signed and transmitted element-for-element without a shell. |
| R3 | `prd.md`; ffmpeg-over-ip v5.2.1 protocol | Protocol-v6 framing/HMAC implementation plus the actual pinned-upstream job | commit `8f7bd87`; CI pinned-upstream job | PROVEN | Authentication and wire behavior were exercised against `ab7adfeedf2a50f7e5807beef9088609cce645d6`. |
| R4 | `prd.md`; protocol session contract | `client/` implements stdin/EOF, stdout/stderr, exit/error, keepalive, cancel, and file operations; focused tests cover each path | commit `8f7bd87`; CI test job | PROVEN | The node implements the complete coherent client-session contract. |
| R5 | `prd.md`; RuleGo relation contract | `TestSessionStreamsInWireOrderAndForwardsStdin` and `TestRuleGoStreamsBothChannelsBeforeOneTerminalResult` | commit `8f7bd87`; CI test/race jobs | PROVEN | Both channels are emitted incrementally with channel metadata in protocol order before terminal delivery. |
| R6 | `prd.md`; terminal contract | Exit, server error, malformed frame, and disconnect tests cover single terminal routing | commit `8f7bd87`; CI test/race jobs | PROVEN | Exit zero uses `Success`; nonzero and session failures use `Failure`, exactly once. |
| R7 | `prd.md`; lifecycle contract | `TestCancellationSendsOneCancel`, `TestRuleGoCancellationSourcesSendOneRemoteCancel`, and `TestRESTDisconnectCancelsRemoteSession` | commit `8f7bd87`; CI test/race jobs | PROVEN | Caller cancellation, timeout, and destruction converge on bounded single-cancel cleanup. |
| R8 | `prd.md`; RuleGo configuration contract | `node.go` owns address/secret/timeouts and RuleGo global resolution; `TestPluginAndConfiguration` validates the boundary | commit `8f7bd87`; CI test job | PROVEN | Operator configuration remains authoritative and no second filesystem policy was introduced. |
| R9 | `prd.md`; concurrency/resource contract | `TestConcurrentSessionsRemainIsolated`, bounded frame checks, incremental stdin decoding, and `TestFileOperationsAndReadCap` | commit `8f7bd87`; CI test/race jobs | PROVEN | Sessions are independent and working buffers are protocol-bounded rather than output-duration-bounded. |
| R10 | `prd.md`; scope boundary | The delivered source contains only the generic RuleGo adapter/client/projection surface; the fresh full-scope audit found no media workflow policy | commit `8f7bd87`; completion audit | PROVEN | Discovery, format selection, FFmpeg recipes, containers, URLs, and player policy remain outside the plugin. |
| R11 | `prd.md`; delivery contract | CI produced versioned amd64/arm64 `.so` files and checksums; release `376044646` publishes those tested bytes and the generic example | CI `32787459529`; release workflow `32788525850`; release `376044646` | PROVEN | The required CI-built release artifacts exist; no image or local heavyweight build is part of delivery. |
| R12 | `prd.md`; REST projection contract | `ffmpegOverIpResponse` is registered at load; `TestResponseProcessorWritesOnlyStdoutAndFailuresAreBodyless` covers filtering and flush behavior | commit `8f7bd87`; CI test job | PROVEN | REST writes and flushes only stdout while stderr and terminal records stay out of the body. |
| R13 | `prd.md`; upstream transport contract | Client dialing supports TCP and `unix:`; the pinned-upstream job passed authenticated sessions over both | commit `8f7bd87`; CI pinned-upstream job | PROVEN | The plugin preserves upstream transports without TLS, HTTP, retry/failover, or local fallback. |
| AC1 | `prd.md` AC1 | Both architecture jobs source-built the pinned unmodified RuleGo host with `CGO_ENABLED=1`, loaded each `.so`, and found the node and processor | commit `8f7bd87`; CI build jobs | PROVEN | The declared ABI tuple is load-tested and stock `CGO_ENABLED=0` incompatibility is documented. |
| AC2 | `prd.md` AC2 | `TestInvocationBoundary` and `TestCommandPreservesArgvAndSignature` include exact argv with punctuation/spacing | commit `8f7bd87`; CI test job | PROVEN | The server-observed command preserves program and argv exactly with no shell. |
| AC3 | `prd.md` AC3 | Actual v5.2.1 TCP/Unix integration covers successful and failed auth plus exact binary `pipe:1`; safe-error tests check disclosure | commit `8f7bd87`; CI pinned-upstream and test jobs | PROVEN | Pinned-server authentication, binary fidelity, and secret/argv non-disclosure are all exercised. |
| AC4 | `prd.md` AC4 | Session and RuleGo stream-order tests observe stdout/stderr before releasing remote completion | commit `8f7bd87`; CI test/race jobs | PROVEN | Output is progressive, channel-distinct, synchronous, and wire-ordered. |
| AC5 | `prd.md` AC5 | `TestSessionStreamsInWireOrderAndForwardsStdin` verifies stdin payload followed by protocol EOF | commit `8f7bd87`; CI test job | PROVEN | Remote stdin and EOF propagation are proven. |
| AC6 | `prd.md` AC6 | Deterministic terminal tests cover exit 0/nonzero, server error, malformed frame, disconnect, timeout, and cancellation; clock-driven liveness tests cover 30s/150s and ping echo | commit `8f7bd87`; CI test/race jobs | PROVEN | Every specified termination path yields one bounded outcome without wall-clock sleeps. |
| AC7 | `prd.md` AC7 | Client and RuleGo cancellation tests verify cancel-before-close, five-second grace behavior, and racing sources | commit `8f7bd87`; CI test/race jobs | PROVEN | At most one `MsgCancel` and one terminal outcome occur with bounded cleanup. |
| AC8 | `prd.md` AC8 | Concurrency isolation, frame-limit, incremental input, and pre-allocation file-read cap tests all pass under normal and race runs | commit `8f7bd87`; CI test/race jobs | PROVEN | Concurrent sessions do not share state and memory ceilings are enforced before allocation. |
| AC9 | `prd.md` AC9 | Released generic REST chain uses `to.wait=true`, `Stream -> end`, `ffmpegOverIpResponse`, and an operator-selected content type; REST tests prove progressive projection | release `376044646`; commit `8f7bd87`; CI test job | PROVEN | The example streams stdout progressively without embedding route or provider policy in the plugin. |
| AC10 | `prd.md` AC10 | All five CI jobs succeeded; release assets include Linux amd64/arm64 `.so` files and matching checksums validated after download | CI `32787459529`; release `376044646`; ignored `./tmp/release-v0.1.1-validation/` | PROVEN | Both target architectures and checksum artifacts are publicly delivered with no container-image requirement. |
| AC11 | `prd.md` AC11 | Strict JSON/input tests plus RuleGo channel/terminal tests cover program, args, optional base64 stdin, raw channel streams, and one structured terminal message | commit `8f7bd87`; CI test/race jobs | PROVEN | The complete public node contract is exercised end-to-end at the RuleGo adapter boundary. |
| AC12 | `prd.md` AC12 | `TestRESTDisconnectCancelsRemoteSession` observes `MsgCancel` after client disconnect and bounded session termination | commit `8f7bd87`; CI test/race jobs | PROVEN | A disconnected REST client cannot leave a detached remote transcode. |

- Missing or contradicted claims: none.
- Final verdict: `CONTINUE` from `DELIVER` to `COMPLETE`.
