# Proposal: M4 — Tools, backends & permissions

## Intent

AGIS thinks and remembers but cannot act. M4 turns it into an agent that executes real work — shell commands, file operations, remote execution — with the Policy Guard as the single enforcement point and the user in control of every escalation. This closes spec §5 and §11 and activates the security model designed in docs/permissions.md and docs/security.md.

## Scope

### In Scope

- Policy Guard core: `PolicyGuard` port, YAML store at `$AGIS_HOME/policy.yaml`, categories (commands/files/network/backends), postures per backend (`sandbox` default), decision flow allow/deny/ask, session grants (`once`/`session`/`always`/deny), audit recording.
- Migration 0004: dedicated `audit_log` table + port methods.
- `agis policy` CLI subcommands: init, set, rm, show, tier, test.
- Tool-calling wire format: `StreamEvent` gains optional `ToolCall`; `ChatRequest` advertises available tools; provider adapters parse OpenAI/Ollama `tool_calls`.
- Brain tool loop: execute through guard, feed `RoleTool` result back, hard cap 8 rounds per turn.
- Backends: Local (shell + filesystem) first; Docker (ephemeral exec) and SSH second.
- TUI: interactive approval prompts (once/session/always/deny) and `/permisos` panel (browse rules, toggle, tier preview, audit view, revoke always).
- Config: `tools`/`backends` block; docs updates; README/roadmap.

### Out of Scope

- Gateway surfaces (auto-deny rules only) — M6.
- Cron-scheduled tools — M6.
- Sandboxing beyond Docker (namespaces/seccomp profiles).
- Non-OpenAI-compatible tool protocols.

## Capabilities

### New Capabilities
- `policy-guard`: policy model, guard decisions, grants, audit log, CLI management.
- `tools-backends`: Tool port and local/docker/ssh execution backends.
- `tool-calling`: LLM wire format for tool calls and the brain execution loop.

### Modified Capabilities
- `brain-loop`: Step executes requested tool calls through the guard with a bounded loop.
- `llm-provider-port`: StreamEvent/ChatRequest gain tool-call representation; adapters parse it.
- `minimal-tui`: interactive approval prompts and the `/permisos` panel.
- `config-loader`: `tools` block (enabled backends, docker/ssh settings).

## Approach

House patterns throughout: consumer-side ports in core, adapter packages per concern (`internal/policy`, `internal/tools`), options-based construction, table-driven RED tests for every security boundary. The model's output is data — it flows into guard *evaluation* as the subject of a decision, never into policy mutation APIs; type signatures make self-granting unrepresentable. Streaming tool-calls ride an additive optional field so M1–M3 flows are untouched zero-value cases.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/core/port_policy.go` | New | Guard port + decision types |
| `internal/policy/` | New | YAML store, guard, session grants |
| `internal/tools/` | New | Registry + local/docker/ssh backends |
| `internal/core/port_llm.go`, `types.go`, `brain.go` | Modified | Wire format, tool loop |
| `internal/adapters/llm/provider.go` | Modified | tool_calls parsing |
| `internal/adapters/tui/app.go` | Modified | Approval prompts, `/permisos` panel |
| `cmd/agis/main.go` | Modified | Subcommand router + wiring |
| `internal/memory/migrations/0004_audit.sql` | New | Audit table |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Guard bypass or self-grant via model output | Low | Type-level separation; RED tests assert no path from model data to policy mutation |
| Shell pattern footguns | Medium | Conservative matching on the exact command string; documented limitations; sandbox default denies |
| Runaway tool loops | Medium | Hard cap 8 rounds/turn, audited |
| Approval prompt vs streaming UX conflicts | Medium | Reuse quit/drain machinery; prompt only between rounds |
| Provider variance on tool_calls | Medium | Parse defensively; unknown shapes degrade to text |

## Rollback Plan

Chained stacked PRs, each green and independently revertable. Migration 0004 is additive-only. Kill switches: `tools.enabled=false` removes registration entirely; postures default to sandbox so merged-but-unconfigured installs deny everything.

## Dependencies

- None external. Docker/SSH binaries required only when those backends are enabled.

## Success Criteria

- [ ] `agis policy init/set/tier/test` manage a real policy.yaml; `test` previews allow/deny/ask correctly.
- [ ] In sandbox posture, every tool action denies by default; standard asks outside allowlists; full trusts session-only.
- [ ] The model can request a local command, get approved interactively, run it, and use the result in its reply.
- [ ] Every decision, grant, and revocation lands in the audit log.
- [ ] Full suite green; all slice reviews approved.

## Proposal question round

Round held: owner confirmed streaming tool-calls (Fork 1). Assumptions from spec §5/§11 verbatim: sandbox default everywhere, always-grants as sole persistent approvals, gateway auto-deny deferred to M6.
