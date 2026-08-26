# Tasks: m4-tools-permissions

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~2600-3000 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 policy core+audit -> PR2 CLI -> PR3 wire+loop+local -> PR4 docker+ssh -> PR5 panel+docs |
| Delivery strategy | auto-chain (owner precedent M1-M3: stacked-to-main) |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

| Unit (PR) | Focused test | Runtime harness | Rollback boundary |
|-----------|--------------|-----------------|-------------------|
| PR1 | go test ./internal/policy/ ./internal/memory/ | N/A - library slice; CLI harness is PR2 | migration 0004 additive; ports additive |
| PR2 | go build ./... + agis policy test git status vs temp AGIS_HOME | real init->set->show->test->rm round-trip | CLI package standalone |
| PR3 | go test ./internal/core/ ./internal/adapters/llm/ ./internal/tools/ | scripted-provider loop tests; manual: approved git status reaches reply | tools.enabled=false removes registration |
| PR4 | go test ./internal/tools/ -run Backend | manual docker/ssh smoke when binaries present | backends opt-in per config |
| PR5 | go test ./... | manual /permisos navigation, approval keys, revoke always | panel is additive UI |

## Phase 1: Policy core + audit (PR1)

- [x] T1.1 RED security tests in internal/policy/guard_test.go: fail-closed corrupt store; sandbox default deny; standard ask-outside-allowlist; deny-beats-allow; sandbox network-class refusal; session-grant suppression + clear-on-close. AC: POL-001/002/003
- [x] T1.2 Create internal/core/port_policy.go: Decision/Posture/Scope/GuardRequest/RuleView/AuditEntry types; PolicyGuard + PolicyAdmin ports. AC: design D1
- [x] T1.3 Create internal/policy/store.go: atomic YAML load/save, exact-or-prefix matcher (no regex), tiers map, deny-all mode on parse error with exposed error. AC: POL-001, D2/D3
- [x] T1.4 Create internal/policy/guard.go: Evaluate posture->rules->override flow; session grants set keyed backend+category+subject; ResolveAsk scopes (once/session/always/deny); ClearSessionGrants. AC: POL-002/003
- [x] T1.5 Write internal/memory/migrations/0004_audit.sql (audit_log table) + port methods AppendAudit/AuditTail in core port, sqlite impl, all fakes. Tests: migration idempotent v3->v4, tail ordering. AC: POL-005

## Phase 2: CLI (PR2)

- [x] T2.1 Route subcommands in cmd/agis/main.go before flag parsing: agis policy <sub> dispatches to internal/policy/cli. AC: POL-004
- [x] T2.2 Implement internal/policy/cli.go: init (safe defaults, refuses overwrite unless --force), set, rm, show (table output), tier (refuses full with session-only guidance), test (dry-run decision print). AC: POL-004
- [x] T2.3 CLI tests against temp AGIS_HOME: init/set/show/test/rm round-trip; tier full refusal; exit codes 0/1. AC: POL-004 scenarios

## Phase 3: Wire format, loop, local backend (PR3)

- [x] T3.1 RED parsing tests internal/adapters/llm: SSE fixtures with tool_calls deltas (accumulate arguments), finish_reason=tool_calls emission, malformed shape degrade-to-text, channel always closes. AC: LLM-001
- [x] T3.2 Extend core wire: ChatRequest.Tools []ToolDef; StreamEvent.ToolCall *ToolCall{ID,Name,Arguments}; Message.ToolCallID field. Zero-value regression test: no tools = byte-identical events. AC: TOL-001
- [x] T3.3 Implement provider tool_calls accumulation/parsing (OpenAI-compatible SSE). AC: LLM-001
- [x] T3.4 RED loop tests internal/core: scripted provider emitting tool calls; allow executes backend and feeds RoleTool result; deny informs model; cap at 8 rounds forces answer; every round audited. AC: TOL-002, BRN-004
- [x] T3.5 Brain tool loop in brain.go: WithTools registration, WithApprover callback, bounded rounds, RoleTool append with ToolCallID. AC: TOL-002
- [x] T3.6 Create internal/tools/local.go shell tool (exec.CommandContext, output capture, timeout) guarded by registry order local-first. Sandbox destructive-class refusal lives in guard matcher (T1.1). AC: TLS-002
- [x] T3.7 TUI approval prompt: WithApprover implementation rendering action + keys a=once s=session l=always n=deny; CtrlC resolves deny; blocks Step goroutine on channels without freezing update loop. RED test: interrupt denies + audits. AC: TUI-002
- [x] T3.8 Config tools block: tools.enabled (default false), backends settings. Wiring in main.go behind kill switch. AC: CONF-002

## Phase 4: Remote backends (PR4)

- [x] T4.1 Create internal/tools/docker.go: ephemeral docker run --rm image sh -c cmd; binary detection skip-with-warning; teardown on failure. Tests with fake runner asserting --rm and cleanup call. AC: TLS-003
- [x] T4.2 Create internal/tools/ssh.go: ssh -i key user@host -- cmd; connection errors surface as tool errors. Tests with fake runner. AC: TLS-004
- [x] T4.3 Registry registration order + graceful degradation tests (enabled-but-missing binaries warn and skip). AC: TLS-001

## Phase 5: Panel + docs (PR5)

- [x] T5.1 /permisos panel sub-model in internal/adapters/tui/panel.go: sections rules-by-category / postures / preview / audit; navigation j-k, tab section switch, space toggle action, r revoke always, q close. AC: TUI-003
- [x] T5.2 Panel tests through drive helpers: revoke always updates store + audits; preview reflects live decisions. AC: TUI-003
- [x] T5.3 Docs: update docs/permissions.md header (implemented in M4), docs/configuration.md tools block, README roadmap M4 DONE, roadmap.md M4 section.
- [x] T5.4 Full suite green: go build ./..., go vet ./..., go test ./... under goleak.

## Dependency Ordering

T1.x -> T2.x -> {T3.x} -> {T4.x || T5.x}. Threat matrix rows all N/A (design); spec security scenarios carried as RED tasks T1.1, T3.1, T3.4, T3.7.
