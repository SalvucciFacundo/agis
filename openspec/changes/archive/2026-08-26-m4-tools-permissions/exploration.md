# Exploration: M4 — Tools, backends & permissions

**Change name**: `m4-tools-permissions` · **Baseline**: HEAD main `f60fe43` (M3 archived, schema v3).

## Scope: IN

Per spec §5 (tool execution), §11 (permission system), docs/permissions.md, docs/security.md, roadmap M4:

- Policy Guard: single enforcement point. `policy.yaml` model (categories commands/files/network/backends), baseline postures per backend (`sandbox` default | `standard` | `full`), decision flow allow/deny/**ask**, approval scopes (`once`/`session`/`always`/deny), session grant state (in-memory), audit log of every decision and grant.
- Tool port + backends: `Tool` interface (Name/Description/Execute(ctx,args,PolicyGuard)); **Local** (shell + filesystem ops), **Docker** (isolated container exec), **SSH** (remote exec).
- Real tool calls end-to-end: LLM emits a tool call → brain evaluates through the guard → TUI approval when asked → result fed back to the model.
- `agis policy` CLI: init/set/rm/show/tier/test.
- `/permisos` TUI panel + interactive approval prompts.
- Audit log persistence (migration 0004).

## Current State

- `StreamEvent{Text, Err}` carries no tool-call representation; `RoleTool` exists in the role enum (anticipated since M1). Brain comment: "tool calls are logged and ignored".
- No subcommand infrastructure in `cmd/agis` (flags only) — `agis policy ...` needs argument routing before flag parsing.
- Schema v3 has `session_events` (CHECK kind IN nudge/summary/skill) but no audit table; grants are designed to live in-memory, so only the audit log persists.
- The repo's own `.git/gentle-ai` experience confirms: policy file must be user-owned plain text at `$AGIS_HOME/policy.yaml`, never project-local.
- Security doc lists ten defenses; M1 ships only the config-0600 check. Trust boundary invariant: **the model can never grant itself permissions** — rule changes come from CLI/TUI only.

## Affected Areas

- `internal/core/port_policy.go` (NEW): PolicyGuard port, Decision/Scope/Posture types, Rule model.
- `internal/policy/` (NEW): YAML store + guard implementation + session grants.
- `internal/core/port_llm.go`, `types.go`: tool-call wire representation; `internal/adapters/llm/provider.go`: OpenAI/Ollama tool_calls parsing.
- `internal/core/brain.go`: tool-execution loop inside Step (call arrives → guard → execute on backend → feed result back).
- `internal/tools/` (NEW): Tool registry + local/docker/ssh backends.
- `cmd/agis/main.go`: subcommand router (`policy ...` vs TUI); `internal/config`: backends/tools block.
- `internal/adapters/tui/app.go`: approval prompt state machine + `/permisos` panel; audit viewer.
- Migration 0004: `audit_log` table.

## Approaches

### Fork 1 — Tool-calling wire format

1. **Extend StreamEvent** with an optional `ToolCall` field (name+args JSON); providers emit it when the model requests a tool; brain loops (execute → append RoleTool message → re-stream). 
   - Pros: streaming UX preserved; one code path; matches OpenAI/Ollama semantics.
   - Cons: touches both adapters; loop bound needed (max N tool rounds).
   - Effort: High
2. **Non-streaming Chat for tool turns** — detect tool-capable requests, use Chat (already returns full response), keep Stream for plain replies.
   - Pros: simpler parsing first iteration.
   - Cons: two execution models; loses streaming during agent work.
   - Effort: Medium

**Recommendation: 1**, with a hard cap (e.g. 8 tool rounds per turn) to bound loops. The wire format is additive; old behavior is the zero-value case, so M1-M3 flows regress nothing.

### Fork 2 — Policy Guard placement

1. **Port in core, adapter `internal/policy`** (like Repository/memory): brain depends on `PolicyGuard` interface only.
   - Pros: dependency rule intact; testable fakes; consistent with house style.
   - Cons: one more port.
   - Effort: Low
2. **Concrete type injected directly**.
   - Pros: less indirection.
   - Cons: breaks the hexagonal pattern every other subsystem follows.

**Recommendation: 1.**

### Fork 3 — Audit storage

1. Dedicated `audit_log` table via migration 0004 (ts, backend, category, subject, decision, scope, persisted bool).
   - Pros: queryable, separable lifecycle from session_events; CHECK-free growth.
   - Cons: one more migration.
   - Effort: Low
2. Reuse `session_events` with new kinds.
   - Pros: zero migration.
   - Cons: CHECK constraint blocks new kinds without ALTER anyway; mixes learning telemetry with security audit.

**Recommendation: 1.**

### Fork 4 — Backend slice order

Local first (PR3), Docker+SSH together later (PR4): each backend is an isolated `Executer` behind the same Tool contract, so rollback boundaries stay clean. SSH needs host config schema; Docker needs container lifecycle decisions (per-call ephemeral vs long-lived pool) — defer detail to design phase.

## Proposed slicing (forecast ≈ 2600–3000 lines → chain required)

| Unit | Content | Est. |
|---|---|---|
| PR1 | Policy core: port, YAML store, guard decision flow, session grants, migration 0004 audit + port methods, tests | ~600 |
| PR2 | `agis policy` CLI subcommand router (init/set/rm/show/tier/test) + tests | ~450 |
| PR3 | Tool-calling wire: LLM port extension, provider parsing (OpenAI+Ollama), brain tool loop through guard, local backend shell/fs tools, TUI approval prompts | ~800 |
| PR4 | Docker + SSH backends + config wiring | ~400 |
| PR5 | `/permisos` panel (browse rules, toggle, tier preview, audit view, revoke always) + docs + README/roadmap | ~550 |

## Risks

- **Security-critical slice**: the guard is the enforcement point; every RED test here is a security test. Injection of policy via model output must be impossible by construction (ports point the wrong way already — model output is data, never reaches policy APIs).
- Shell parsing footguns (quoting, `&&` chains) in the local backend matcher — pattern matching operates on the command string with conservative prefix/regex rules; document limitations explicitly.
- Docker/SSH availability varies by machine — backends must be opt-in config, absent binaries degrade gracefully at registration time.
- Approval prompt UX inside Bubbletea mid-stream — pause/resume semantics need care (stream cancel + resume after decision, mirroring the existing quit/drain machinery).
- Loop bounds: runaway tool rounds must be capped and audited.

## Ready for Proposal

Yes — one product decision pending: confirm Fork 1 recommendation (streaming tool-calls through StreamEvent, hard cap 8 rounds) vs the simpler non-streaming tool-turn path.
