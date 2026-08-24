# Delivery Governance Evidence

Canonical contract: `/home/agent/Src/stream-prism/.trellis/spec/guides/delivery-governance.md`

## Goal Record

- Goal ID: `ffmpeg-over-ip-client-plugin-v1`
- Observable outcome: a released RuleGo `.so` exposes one native node that can
  execute a complete ffmpeg-over-ip client invocation and progressively route
  its output without an external client binary or local FFmpeg.
- Acceptance identity: R1-R13 and AC1-AC12 in `prd.md` at the reviewed task
  revision.
- Scope: RuleGo adapter, client wire/session implementation, REST stdout
  projection, tests, CI build, and release artifacts.
- Fundamental facts: only RuleGo's `Stream` relation is delivered synchronously;
  ffmpeg-over-ip is a multiplexed authenticated process protocol; RuleGo Go
  plugins are ABI-bound and stock server releases disable Go plugin support.
- Invariants: exact argv; no shell; channel-tagged stdout/stderr in wire order;
  exactly one terminal outcome after stream delivery; bounded session/output
  buffering; cancellation closes the remote session; no media policy in the
  plugin.
- Assumptions: the first release target is the Linux RuleGo server source
  revision and core module pinned by CI, built with Go plugin support; operators
  grant the RuleGo process only the filesystem access they intend the remote
  FFmpeg session to use.
- Open decisions: none blocking planning.
- Selected decisions: one formal node, one REST projection processor, protocol
  v6 compatibility, both output channels on synchronous `Stream`, MIT-only
  client adaptation, a protocol-sized file-read ceiling, pinned-upstream
  integration, and a CI-built `.so` plus checksum.
- Explicit non-goals: server deployment, caller-specific media policy,
  local-process fallback, stock `CGO_ENABLED=0` hosts, and compatibility with
  unpinned RuleGo builds.
- Mechanism-to-fact links: channel-tagged stdout and stderr both use `Stream`
  because it is RuleGo's only synchronous ordered path; the output processor
  exists because the built-in body processor does not flush or filter channels;
  protocol code exists because upstream client packages are internal.
- Governance activation evidence: product, plugin, transport, CI/release, and
  downstream HTTP behavior cross independently owned boundaries.

## Boundary and Authority Ledger

| Boundary | Owner | User-owned authority | Facts/invariants | Permitted decisions | Forbidden inferences | Inputs | Outputs | Authoritative evidence | Reversibility/recovery |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Product | User | Complete client capability, not stdout-only | Generic remote ffmpeg/ffprobe session | Define observable node contract | Add media/provider policy | Invocation request | Stream and terminal events | `prd.md` | Revise before activation |
| RuleGo/Core | RuleGo | None | Only `Stream` is synchronous; Go plugin ABI must match | Adapt to public interfaces | Modify RuleGo core in this task | RuleMsg/context | Channel-tagged Stream + terminal relations | pinned server/core source | Rebuild host and plugin as one ABI tuple |
| Plugin | This repository | Approve implementation after plan review | One node owns one session | Minimal adapter and tests | Add speculative framework | Config + message | RuleGo events | plugin tests | Revert commit |
| Transport | ffmpeg-over-ip | Server address and secret | Protocol v6 and HMAC framing | Implement compatible client loop | Change server protocol | frames | frames | v5.2.1 source/tests | Pin or deliberately upgrade |
| Deployment | Operator | Address, secret, process filesystem permissions | Plugin shares host process authority | Document required placement/config | Invent host/network policy | `.so`, config | loaded component | deployment smoke test | remove plugin/restart |
| Build/CI | Repository | GitHub workflow authorization | Go plugin ABI requires matching toolchain/deps/build mode; stock host disables plugins | Pin and build a plugin-enabled host plus `.so` in CI | Claim stock or arbitrary ABI compatibility | server/core revisions | source-built host + `.so` | load smoke logs/artifact hashes | rebuild exact tuple |
| Release | User | Push/tag/publication | Release bytes must match tested bytes | Draft artifacts after authorization | Publish without later authority | CI artifacts | release assets | GitHub release state | draft/delete before publication |
| Downstream validation | User | Application acceptance | Not authoritative for plugin protocol correctness | Provide generic REST proof | Claim application-specific acceptance | HTTP request | streamed bytes | integration test; later user test | adjust calling graph |

## Decision and Chatroom Convergence Record

- Decision ID: `D1-complete-client-node`
- Goal ID: `ffmpeg-over-ip-client-plugin-v1`
- Bounded question: whether the plugin is only a stdout Stream producer or the
  complete ffmpeg-over-ip client expressed through RuleGo.
- Repository-answerable facts: both host and wire contracts are documented in
  `research/`.
- User-owned decisions: the user selected complete client semantics.
- Unresolved expert judgment: none blocking design.
- Lenses and theses: RuleGo integration, ffmpeg-over-ip protocol, and consumer
  path require a complete invocation; both output channels use the native
  ordered stream relation while HTTP/media policy remains outside the node.
- Pressure-test rounds: stdout-only was rejected because it omits stderr,
  terminal state, cancellation, stdin, keepalive, and file operations.
- Explicit disagreements: none after user confirmation.
- Falsifiers: inability to represent distinct output relations or to cancel via
  `RuleContext.GetContext()` would invalidate the selected node mapping; pinned
  source confirms both are available.
- Selected alternative: one complete client node, channel-tagged output on the
  synchronous `Stream` relation, and the minimum REST stdout projection needed
  for progressive responses.
- Rejected alternatives and reasons: shelling out to the client adds a runtime
  binary; embedding local FFmpeg changes the product; stdout-only is incomplete;
  a media-specific node leaks caller policy.
- Decision owner: user.
- Convergence evidence: user confirmation on 2026-08-24.
- Termination evidence: design may be drafted; implementation still requires
  the later Trellis activation approval.

## Material Gate Report

- Goal ID: `ffmpeg-over-ip-client-plugin-v1`
- Gate ID: `FRAME-DECIDE-2026-08-24`
- Proposed transition: `FRAME -> DECIDE`
- Delivery identity: planning task, no product revision yet.
- Responsible reviewer identity: `ffmpeg_boundary` and `rulego_boundary`, each
  read-only and bounded to its owning contract.
- Required authoritative evidence: pinned upstream sources, current PRD, user
  confirmation.
- Present evidence and cursor: RuleGo server `3bf4ac47`, exact core `8995627`
  archive SHA-256 `2669d749...5968ae`; ffmpeg-over-ip `ab7adfeed` (`v5.2.1`);
  current task artifacts.
- Rejected or missing claims: no claim of downstream application validation or
  ABI compatibility outside the pinned build target.
- Verdict: CONTINUE to final planning review.
- Rationale: both reviewers returned `CONTINUE`; the capability boundary,
  ordered channel mapping, bounded protocol resources, exact upstream behavior,
  and plugin-enabled host ABI are tied to pinned source evidence.
- Next or recovery action: obtain approval of the final planning summary before
  `task.py start`.
- Negative-test omission: remove the REST projection processor evidence.
- Negative-test verdict: BLOCK, because small stdout writes would not be proven
  progressive and terminal/error data could enter the media body.

## Failure and Retry Journal

- Goal ID: `ffmpeg-over-ip-client-plugin-v1`
- Prior governance phase: DECIDE
- Operation/action digest: bounded read-only agent review of host and protocol
  boundaries.
- Target worker or service: existing architecture research sessions.
- Approval intent: advisory evidence only.
- Delivery identity: none.
- Idempotency key: read-only task and pinned revisions.
- Authoritative state-read mechanism: direct repository inspection.
- Failure classification: TRANSIENT for provider stream disconnect/rate limit.
- HTTP/provider code: 429 / stream disconnect.
- Backoff basis: bounded retries on the same worker goals while coordinator
  continued read-only repository research.
- Evidence cursor: pinned revisions above.
- Resume at: provider availability within the active planning window.
- Scheduler/runtime adapter: coordinator-observed native child retry.
- Diagnosis or automatic-resume evidence: retries preserved the same goal and
  scope until both bounded reviewers returned `CONTINUE`; planning facts were
  independently verified from pinned sources during cooldown windows.

## Completion Matrix

- Goal ID: `ffmpeg-over-ip-client-plugin-v1`
- Delivery identity: not assigned during planning.
- Auditor identity: to be assigned at completion.

| Requirement or constraint ID | Authoritative source | Current evidence | Evidence identity | Status | Auditor rationale |
| --- | --- | --- | --- | --- | --- |
| R1-R13, AC1-AC12 | `prd.md` | Planning only | no implementation revision | INCOMPLETE | Completion audit occurs after delivery |

- Missing or contradicted claims: all implementation and release evidence.
- Final verdict: not evaluated during planning.
