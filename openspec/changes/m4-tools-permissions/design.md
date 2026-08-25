# Design: m4-tools-permissions

## Technical Approach

Three new adapter packages behind consumer-side core ports (`internal/policy`, `internal/tools`), an additive wire-format extension in the LLM layer, and a bounded tool loop inside `Brain.Step`. Every security boundary from the spec lands first as a RED test. Model output stays data end-to-end: it becomes the subject of guard evaluations and never reaches policy mutation APIs; the type graph makes self-granting unrepresentable.

## Architecture Decisions

| # | Decision | Alternatives | Rationale |
|---|----------|--------------|-----------|
| D1 | Split consumer ports: `PolicyGuard` (Evaluate) for the brain; `PolicyAdmin` (rules CRUD, tiers, audit read, ResolveAsk) for CLI/TUI | One fat port | The brain must be unable to mutate policy even by accident; segregation is the trust boundary made type-level |
| D2 | Command matching: exact match OR prefix-with-space on the rule pattern (`git` matches `git status`); no regex v1 | Regex patterns | Deterministic, auditable, avoids catastrophic backtracking and quoting footguns; limitations documented |
| D3 | Corrupt/unreadable store = deny-all mode with exposed error (fail closed); init writes defaults atomically | Silent permissive fallback | POL-001 fail-closed mandate |
| D4 | Session grants in-memory set keyed backend+category+subject; cleared via `ClearSessionGrants()` wired into CloseSession | Persisted grants table | Spec: only always persists; session scope dies at close |
| D5 | Wire format additive: `ChatRequest.Tools []ToolDef`; `StreamEvent.ToolCall *ToolCall{ID,Name,Arguments}`; providers accumulate streamed argument deltas and emit one ToolCall at finish_reason=tool_calls; unknown shapes degrade to text | Per-delta emission; separate tool channel | OpenAI/Ollama compatible; zero-value events keep M1–M3 flows byte-identical |
| D6 | Brain loop: drain stream collecting ToolCalls → up to 8 rounds of evaluate/approve/execute/append RoleTool → final round omits Tools forcing a text answer | Inline mid-stream execution | Bounded, auditable, testable with scripted providers; cap stops runaways |
| D7 | Approval via injected callback `WithApprover(func(GuardRequest) GrantScope)`; TUI implementation blocks on channels while Bubbletea renders the prompt; CtrlC/interrupt resolves deny | Brain owns prompt UI | Layering preserved: brain knows nothing about widgets; interrupt-safe default is deny (TUI-002) |
| D8 | Backends as one `shell` tool per enabled provider (local/docker/ssh), selected by registry order; docker runs ephemeral `docker run --rm image sh -c cmd`; ssh runs `ssh -i key user@host -- cmd` | fs-specific tools; pooled containers | Smallest honest v1 surface; fs ops expressible as shell; pool lifecycle deferred |
| D9 | CLI routing before flag parsing: `agis policy <sub>` dispatches to internal/policy/cli with its own flagset; exit codes 0/1 | Cobra/urfave | No new deps; single-binary ethos |

## Data Flow

    model reply (tool_call)
      -> Brain.Step loop (round < 8)
           -> PolicyGuard.Evaluate(backend,category,subject)
                allow  -> tools.Executer.Run -> RoleTool message
                deny   -> RoleTool "blocked by policy"
                ask    -> Approver callback (TUI prompt) -> once/session/always/deny
           -> re-stream with appended messages
      -> final round without Tools forces text answer
    every branch -> repo.AppendAudit

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/core/port_policy.go` | Create | Guard/Admin ports, Decision/Scope/Posture/Rule types |
| `internal/policy/store.go` | Create | YAML load/save (fail-closed), rules, tiers |
| `internal/policy/guard.go` | Create | Evaluate flow, session grants, ResolveAsk |
| `internal/policy/cli.go` | Create | init/set/rm/show/tier/test |
| `internal/memory/migrations/0004_audit.sql` | Create | audit_log table |
| `internal/core/port_repository.go` + sqlite | Modify | AppendAudit/AuditLog methods |
| `internal/core/port_llm.go`, `types.go` | Modify | ToolDef/ToolCall, Message.ToolCallID |
| `internal/adapters/llm/provider.go` | Modify | tool_calls accumulation/parsing |
| `internal/tools/{registry,local,docker,ssh}.go` | Create | Backends behind one shell tool |
| `internal/core/brain.go` | Modify | Tool loop + approver option |
| `internal/adapters/tui/app.go` (+panel file) | Modify | Approval keys, /permisos panel |
| `cmd/agis/main.go` | Modify | Subcommand router + wiring |
| `docs/permissions.md` header | Modify | Drop "designed, not implemented" note |

## Interfaces / Contracts

```go
type Decision int // DecisionAllow, DecisionDeny, DecisionAsk
type Posture string // sandbox, standard, full

type GuardRequest struct { Backend, Category, Subject string }
type PolicyGuard interface {
    Evaluate(ctx context.Context, req GuardRequest) Decision
}
type PolicyAdmin interface {
    SetRule(ctx context.Context, category, backend, pattern, action string) error
    RemoveRule(ctx context.Context, category, backend, pattern string) error
    Rules(ctx context.Context) []RuleView
    SetTier(ctx context.Context, backend string, p Posture) error
    Preview(ctx context.Context, req GuardRequest) Decision
    AuditTail(ctx context.Context, n int) ([]AuditEntry, error)
    ResolveAsk(ctx context.Context, req GuardRequest, scope Scope) error
    ClearSessionGrants()
}
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Store parse/fail-closed, matcher classes, grant lifecycles, CLI subcommands (fs-backed), wire parsing fixtures | Table-driven; golden SSE chunks incl. malformed shapes |
| Unit | Guard decision matrix (postures x rules x overrides), sandbox destructive-class refusal | RED-first security tests from spec scenarios |
| Integration | Brain loop with scripted provider emitting N tool calls, fake guard/backend; approval callback scripted | Assert round cap, audit rows, RoleTool feedback |
| E2E-ish | TUI approval keys and panel navigation through drive helpers; goleak held | Existing harness |

## Threat Matrix

Template rows target VCS/PR automation; M4 builds generic policy-governed command execution instead:

| Boundary | Applicability | Reason |
|---|---|---|
| Documentation-like paths | N/A | No executable-file classification in scope; commands are explicit strings governed by the policy matcher |
| Git repository selection | N/A | No VCS-specific automation; `git -C ...` is just a matched subject string |
| Commit state | N/A | Not automated here |
| Push state | N/A | Not automated here |
| PR commands | N/A | Not automated here |

The design's own security boundaries are carried as spec-mandated RED tests regardless: fail-closed corrupt store (POL-001), sandbox destructive/network class refusal (TLS-002), deny-beats-allow precedence (POL-002), model-output-inertness (BRN-004), malformed tool-call degradation (LLM-001), interrupt-denies prompt (TUI-002).

## Migration / Rollout

Migration 0004 additive-only. `tools.enabled` default false: merged-but-unconfigured installs register nothing and stream exactly as today. Postures default sandbox everywhere. Each PR reverts independently.

## Open Questions

- Docker default image choice (defer to implementation review; candidate `alpine:3`).
- SSH known_hosts handling: default strict checking on; bypass flag explicitly out of scope.
