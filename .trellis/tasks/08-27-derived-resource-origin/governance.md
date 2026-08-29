# Delivery Governance Evidence

Canonical contract: `stream-prism:.trellis/spec/guides/delivery-governance.md`

## Goal Record

- Goal ID: `derived-resource-origin-2026-08-27`
- Observable outcome: A RuleGo deployment can expose bounded, seekable HLS from indexed separate media inputs while generic resource lifecycle, transformation transport, and source resolution remain independently owned.
- Acceptance identity: `prd.md` R1-R13 and AC1-AC12; downstream Emby/Kodi UI behavior remains owner-scoped rather than a plugin acceptance gate.
- Scope: `rulego-ffmpeg-over-ip`, `rulego-resource-origin`, `rulego-indexed-vod`, their pinned RuleGo host contract, release artifacts, composition rule, and final evidence.
- Fundamental facts: upstream leases may expire; terminal HLS members must be independently addressable; RuleGo static mapping already owns byte-range delivery; ffmpeg-over-ip owns remote execution; indexed-vod owns one bounded indexed member; resource-origin owns publication lifecycle.
- Invariants: no YouTube/media policy in generic plugins; no HTTP listener in plugins; no unbounded source read; no incomplete member exposure; no credential or signed-URL disclosure; exact ABI/client identity across co-loaded plugins.
- Assumptions: the operator supplies a reachable resolver, ffmpeg-over-ip service, shared storage, and externally coherent routing.
- Open decisions: none within the task boundary. Emby for Kodi Next Gen validation remains user-owned. Emby Web's observed seek failure is assigned to its own remux/transcode startup policy after a warm-member A/B excluded RuleGo member production. Static CORS is unnecessary after the completed same-origin cutover but remains a host concern for deployments that intentionally split origins.
- Selected decisions: shared RuleGo HTTP plus relative routes; native static mapping; deterministic virtual HLS manifest; one indexed member per demand; source leases refreshed outside the origin contract.
- Explicit non-goals: player sessions, a new media server, a duplicate byte server, embedded yt-dlp policy, DASH-only terminal output, or cluster scheduling.
- Mechanism-to-fact links: `design.md` sections 1-8 and `research/native-composition.md`.
- Governance activation evidence: the result crosses source, plugin, host HTTP, deployment, CI/release, and downstream-player boundaries and includes external publication.

## Boundary and Authority Ledger

| Boundary | Owner | User-owned authority | Facts/invariants | Permitted decisions | Forbidden inferences | Inputs | Outputs | Authoritative evidence | Reversibility/recovery |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Product | User/task PRD | Terminal outcome and acceptance | Seekable bounded HLS; direct stream remains | Choose observable contract | Infer sessions/profiles | Requirements | AC matrix | `prd.md` | Return to planning |
| Source adapter | Rule graph/operator | Resolver URL, cookies, format policy | Leases expire; video/audio may be separate | Resolve/refresh/normalize | Put provider fields in generic plugins | Stable asset URL | Indexed lease | Example chain and runtime evidence | Re-resolve lease |
| Indexed transform | `rulego-indexed-vod` | Publication bounds | One member is an atomic bounded unit | Inspect indexes, Range-read, mux | Own public URL/lifecycle | Indexed lease and demand | Closed TS member | Unit and hermetic E2E | Fail generation |
| Transport | `rulego-ffmpeg-over-ip` | Service endpoint/secret | Generic remote FFmpeg execution | Retry transport-class failure | Own resource/session/source semantics | Invocation | Stream/files/result | Client tests and release CI | Retry same invocation within bounds |
| Resource lifecycle | `rulego-resource-origin` | Root and limits | Atomic ready state and absolute expiry | Acquire/commit/fail/resolve | Understand media/provider policy | Key, fingerprint, files | Stable descriptor | Origin tests and E2E | Expire/hide/reconcile owned bytes |
| Host HTTP | RuleGo server/operator proxy | Public authority/routing | Native static GET/Range; shared endpoint is opt-in | Configure mapping/CORS/proxy | Push host gaps into plugins | Ready path | HTTP responses | Exact-image probes | Config rollback/restart |
| Build/CI | Owning repositories | Push authorization | Go plugin ABI is exact | Build with pinned SDK/runtime | Claim cross-version compatibility | Revision and ABI lock | Artifacts/checks | GitHub runs and sidecars | Correct and rerun |
| Release | Owning repositories | Publish authorization | Artifacts are immutable identities | Publish verified version | Rewrite published history | Green revision | Release assets | GitHub release state | New patch release if bytes change |
| Downstream validation | User | Emby/Kodi interactive acceptance | Protocol success is not UI proof | Run terminal matrix | Claim terminal UI success from ffprobe | Public HLS URL | Playback observation | User result plus server logs | Diagnose owning boundary |

## Decision and Chatroom Convergence Record

- Decision ID: `indexed-vod-owner`
- Goal ID: `derived-resource-origin-2026-08-27`
- Bounded question: Which smallest owner supplies seek without coupling provider policy or HTTP serving to ffmpeg-over-ip?
- Repository-answerable facts: RuleGo owns static ranges; resource-origin owns durable publication; indexed MP4 ranges enable one-member production.
- User-owned decisions: accept HLS as the terminal contract and keep terminal-specific playback policy outside the generic plugins.
- Unresolved expert judgment: none for the implemented boundary.
- Selected alternative: source-neutral indexed-vod owner composed with resource-origin and shared RuleGo HTTP.
- Rejected alternatives and reasons: session state in ffmpeg-over-ip violates ownership; complete download violates bounded demand; DASH alone does not cover required terminals.
- Decision owner: user.
- Convergence evidence: `design.md`, `research/native-composition.md`, and `research/youtube-index-native-feasibility.md`.
- Termination evidence: all task-owned implementation decisions are selected;
  terminal validation remains explicitly owner-scoped.

## Material Gate Report

- Goal ID: `derived-resource-origin-2026-08-27`
- Gate ID: `indexed-vod-e2e-candidate`
- Proposed transition: commit and push the hermetic cross-plugin HLS regression, then accept GitHub CI evidence.
- Delivery identity: `rulego-indexed-vod@46d981f`.
- Responsible reviewer identity: coordinator, completed Trellis checker goal `derived-resource-origin-e2e-quality-2026-08-29`, and path-review goal `derived-resource-origin-path-review-2026-08-29`.
- Required authoritative evidence: current diff, unit/vet/shell checks, passing hermetic Docker E2E, exact ABI sidecars, GitHub CI conclusion.
- Present evidence and cursor: checker-approved local Go, shell, workflow, and full E2E checks; path-review `CONTINUE`; pushed commit `46d981f591aae8676a8c050f1965520fa7f992cb`; GitHub CI run `33250440019` with all jobs green, including `Hermetic HLS seek` job `99095241826`.
- Rejected or missing claims: host-version-specific HTTP method registration and CORS are not claimed by the plugin regression; the plugin regression does not claim Emby/Kodi UI behavior. Emby Web was diagnosed separately and no terminal-specific project workaround was retained; Kodi remains user-owned.
- Verdict: `CONTINUE` to final evidence and completion audit.
- Rationale: the exact pushed revision passed local and external gates without production/API changes or boundary drift.
- Next or recovery action: refresh final-state evidence, resolve task-owned delivery items, then run a fresh completion audit.
- Negative-test omission: none; the harness includes an intentionally broken static mapping and requires its `/resources/` fetch to fail.
- Negative-test verdict: locally proven.

## Failure and Retry Journal

- Goal ID: `derived-resource-origin-2026-08-27`
- Operation/action digest: independent read-only quality check.
- Target worker or service: Trellis checker provider.
- Delivery identity: pre-commit indexed-vod E2E worktree, now committed as `46d981f`.
- Failure classification: `TRANSIENT` provider rate limit/stream disconnect.
- Attempt window and attempt: the quality checker and path reviewer were resumed under their unchanged bounded goals until each returned a final verdict; no interrupted response was accepted as approval.
- Backoff basis: provider-owned rate limit; main coordinator continued repository-verifiable checks without changing scope.
- Scheduler/runtime adapter: existing native child resumed with the same goal.
- Diagnosis or automatic-resume evidence: the same child goal was resumed until it completed; its scoped fixes passed actionlint, ShellCheck, Go test/vet/race fixture checks, and the full hermetic E2E.

- Goal ID: `derived-resource-origin-shared-http-2026-08-29`.
- Operation/action digest: authoritative source and read-only deployment audit
  for shared main-HTTP ownership.
- Target worker or service: persistent native child reviewer.
- Delivery identity: RuleGo v0.37.0 source revision `a7be24c4` and live rule
  `sxYw0hmQDtSX` at update time `2026/08/29 04:35:51`.
- Failure classification: repeated `TRANSIENT` provider rate-limit/stream
  disconnects before a result was emitted.
- Attempt window and attempt: the same child and unchanged goal were resumed;
  the coordinator continued only read-only repository and runtime probes.
- Backoff basis: provider-owned throttling; no approval or partial result was
  inferred from interrupted attempts.
- Scheduler/runtime adapter: existing native child resumed under the same
  stable goal identity.
- Diagnosis or automatic-resume evidence: the resumed child established from
  source that the config API cannot enable sharing and that cutover must wait
  for an INI change plus restart. The later production gate used the actual
  configured listener identity, `:80`.

## Shared HTTP Integration Gate

- Goal ID: `derived-resource-origin-2026-08-27`.
- Gate ID: `shared-main-http-cutover`.
- Proposed transition: replace the rule-owned `:6333` listener with the
  verified shared main-listener reference on the production rule.
- Delivery identity: RuleGo v0.37.0 plugin runtime and rule
  `sxYw0hmQDtSX`.
- Present evidence: the exact runtime image locally exposes `:9090` in the
  system pool and serves the full relative HLS/static route when
  `share_http_server = true`; source revision `a7be24c4` injects that endpoint
  only during startup.
- Present production evidence: after the operator enabled the INI flag and
  restarted, `GET /api/v1/shared-nodes/%3A80/endpoint` returned `200`. The
  coordinator changed the sole endpoint field from `:6333` to `ref://:80`,
  authoritative readback matched, the old listener closed, and the main
  authority returned manifest `200`, member `307`, static Range `206`, and a
  successful three-second mpv seek beginning at 60 seconds.
- Verdict: `CONTINUE`; production cutover is complete.
- Recovery: restore the same rule field to `:6333` only if the shared main
  endpoint disappears; otherwise retain the single-authority route.

## Completion Matrix

- Goal ID: `derived-resource-origin-2026-08-27`
- Evidence snapshot: 2026-08-29 16:31 UTC plus the scoped C12/C17 addendum.
- Delivery identity: resource-origin `v0.1.1@2489c730`; indexed-vod
  `v0.2.0@88434c89` plus acceptance/composition revision `51ad2602`; ffmpeg plugin
  `v0.5.1@2729a778`; RuleGo `v0.37.0@a7be24c4`; ABI `abi-d4fc741b...`;
  live rule `sxYw0hmQDtSX` with `ref://:80`.
- Auditor identity: fresh read-only goal
  `derived-resource-origin-completion-audit-2026-08-29`.

| Requirement or constraint ID | Authoritative source | Current evidence | Evidence identity | Status | Auditor rationale |
| --- | --- | --- | --- | --- | --- |
| R1 | PRD | Durable origin identity/catalog/lifecycle | origin `2489c730` | PROVEN | Resource-centered ownership is implemented. |
| R2 | PRD | Origin opens no listener; RuleGo owns static bytes | origin + host | PROVEN | Native composition is retained. |
| R3 | PRD | Stateless stream plus resource/resource-set forms | F/O/I releases | PROVEN | All response forms exist. |
| R4 | PRD | Pending/ready/failed/expired/not-found tests | origin tests | PROVEN | Readiness is explicit. |
| R5 | PRD | Runtime/extent/storage/TTL and bounded-range gates | F/O/I CI | PROVEN | Production and retention are bounded. |
| R6 | PRD | Singleflight/waiter tests and concurrent E2E | indexed CI `33250440019` | PROVEN | Equivalent demand shares production. |
| R7 | PRD | Reconciliation plus restart-stable E2E/live bytes | O/I tests | PROVEN | Ownership survives restart. |
| R8 | PRD | GET/206/416/304, exact lengths, atomic ready-only exposure | indexed CI + origin tests | PROVEN | Native static delivery and publication satisfy the task-owned HTTP contract. |
| R9 | PRD | No media/provider policy in origin contract | released APIs | PROVEN | Transformation neutrality holds. |
| R10 | PRD | Same-source indexed/generic latency/CPU/byte study | feasibility research | PROVEN | YouTube specialization was measured. |
| R11 | PRD | Separate leases, expiry-bound reuse, refresh, revision checks, neutral origin | indexed `51ad2602`, CI `33263193567`, live rule | PROVEN | Provider policy and lease ownership stay in the rule graph. |
| R12 | PRD | Every component maps to a demonstrated gap | design + E2E | PROVEN | No duplicate server/session/scheduler was added. |
| R13 | PRD | Both architectures/orders co-load all plugins | release CI | PROVEN | ABI coexistence is verified. |
| AC1 | PRD | Generic publication/shared-owner/restart evidence | O/I tests | PROVEN | Published bytes outlive caller nodes. |
| AC2 | PRD | GET/206/416/304 with exact lengths plus staging/atomic-readiness controls | indexed CI + origin tests | PROVEN | The complete task-owned retrieval matrix passes. |
| AC3 | PRD | Atomic multi-member and eight-member demand E2E | indexed CI | PROVEN | Members are independently materialized. |
| AC4 | PRD | Waiter tests and one-production concurrent request | indexed CI | PROVEN | Concurrent reuse is verified. |
| AC5 | PRD | Bounds tests and closed sub-file ranges only | F/O/I CI | PROVEN | No demand runs to EOF. |
| AC6 | PRD | Restart preservation and owned-orphan cleanup | O/I tests | PROVEN | Unrelated files remain untouched. |
| AC7 | PRD | Deterministic states, expiry rejection, old URL 404 | O/I tests | PROVEN | Expired state cannot revive. |
| AC8 | PRD | Stateless stream and both live forward-only routes | F + live rule | PROVEN | Direct streaming remains independent. |
| AC9 | PRD | First-ready/seek/mode/byte measurements | feasibility research | PROVEN | Comparative experiment is complete. |
| AC10 | PRD | Separate H.264/AAC, refresh, HLS, faults, ffprobe/mpv | indexed CI + live | PROVEN | Emby Web failure is isolated to its own startup policy; Kodi remains explicitly user-owned. |
| AC11 | PRD | Ownership-to-gap map and broken-mapping control | design + indexed CI | PROVEN | Native server remains the data plane. |
| AC12 | PRD | Four nodes/two processors, both orders/architectures | release CI | PROVEN | Supported plugin set co-loads. |
| C1-C11 | PRD scope/non-goals | Ownership, neutrality, boundedness, no session/server/scheduler | released APIs + tests | PROVEN | All explicit architectural constraints hold. |
| C12 | Design/implement secrecy | Public manifest has no URL/auth/cookie/transport/secret marker | live 1,055-line manifest | PROVEN | Admin API access control is separate operator-owned host debt. |
| C13-C16 | User/governance delivery constraints | UI unclaimed; no host fork; `ref://:80`; old port refused | live probes | PROVEN | Delivery identity and authority are accurate. |
| C17 | Final-history constraint | One coherent task-owned final-state narrative | amended task commit + archive | PROVEN | History records the selected architecture and verified outcome only. |
| C18-C19 | Governance audit constraints | Full matrix retained; auditor was read-only | audit goal | PROVEN | No green subset or audit mutation substituted for proof. |
| Named plugin releases | Goal | Three immutable releases and sidecars are public; CI green | F/O/I tags/runs | PROVEN | Required releases are verified. |

- Missing or contradicted claims: none.
- Final verdict: `COMPLETE`; every task-owned requirement, acceptance criterion,
  delivery constraint, and named release is proven by current evidence.
