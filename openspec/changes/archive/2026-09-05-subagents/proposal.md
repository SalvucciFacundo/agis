# SDD Proposal: Native Subagent Delegation (`delegate_task`)

## Intent
Introduce native subagent delegation into the `agis` `core.Brain` to allow spawned, isolated child agent instances running their own bounded thinking loop (`core.Brain.Step`). This addresses token bloat and context contamination during heavy exploratory, debugging, research, or multi-step execution tasks by keeping the detailed steps in an ephemeral child scope and only returning the compressed outcome to the parent.

## Scope
### In Scope
- **Native Tool `delegate_task`**: Implementation of `core.ToolRunner` named `delegate_task` with backend `"subagent"` and a new policy category `CategoryExecution`.
- **Isolation & Bounding**: Ephemeral conversation state for child agents, bounded turns (default max 10), and timeout inheritance from the parent context.
- **Recursion & Concurrency Guard**: Bounded recursion (max depth default 1, hard limit 2) and concurrency limitation (semaphore pool, max 3 concurrent) via new configuration variables.
- **Outcome Compression**: The returned execution result from the child agent is passed back to the parent as the tool's string result.
- **Observability**: Tool executions audited via existing `PolicyGuard` and `AuditEntry`, plus diagnostic probe added to `internal/doctor` for subsystem status.

### Out of Scope
- Distributed subagents (running in other processes or machines).
- Inter-subagent direct messaging (subagents only communicate with their spawner via return value).
- Advanced memory sharing besides inheriting the same LLM client and config.
- State persistence across different subagent invocations (child memory is ephemeral).

## Affected Areas
- `internal/config/config.go`: Add `Subagents` configuration block.
- `internal/core/port_policy.go`: Add `CategoryExecution` constant.
- `internal/core/brain.go`: Integration of subagent spawning within the tool execution path or providing the `Brain` capability to spawn children.
- `internal/tools/subagent.go` (new): The `ToolRunner` implementation for `delegate_task`.
- `internal/doctor/doctor.go` & `internal/doctor/web.go`: Add diagnostic probe for the subagents subsystem.

## Business Rules & Constraints
1. **Recursion Limits**: Hard maximum depth limit prevents infinite self-spawning loops. Max depth defaults to 1.
2. **Concurrency Limits**: A semaphore restricts the global or per-session concurrent subagents to prevent exhaustion of LLM provider rate limits.
3. **Audit**: Each `delegate_task` invocation runs through `PolicyGuard` like any other tool, ensuring visibility and explicit authorization via the backend `"subagent"`.
4. **Context Bounds**: Child turn loop halts when the maximum turn count is hit, returning a partial result or error to avoid blocking the parent indefinitely.

## Edge Cases
- **Parent Cancellation**: If the parent context cancels, the child context MUST cancel immediately.
- **Tool Failure Loop in Child**: The child must respect its turn boundary (`max_turns`) regardless of whether it's stuck in a tool error loop.
- **Rate Limit Hit**: If concurrency limit is reached, `delegate_task` must block with a context timeout or return a rate-limit error to the parent rather than panic.

## Risks and Rollback
- **Risk: Resource Exhaustion**: Although bounded by config, deep concurrency could consume API rate limits or memory quickly. **Mitigation**: Sensible defaults (depth 1, max concurrent 3) and `context.Context` enforcement.
- **Risk: Context Leakage**: Returning too much data from the child can defeat the compression goal. **Mitigation**: System prompt in child explicitly targets concise synthesis upon exit.
- **Rollback**: Disable `enabled` flag in the new `Subagents` config block, immediately causing `delegate_task` to return `"subagent delegation is disabled"`.

## Success Criteria
- Subagent spawns successfully execute a bounded multi-turn task and return only the final response to the parent.
- Configuration correctly limits concurrent spawns and maximum depth.
- PolicyGuard intercepts `"subagent"` calls, allowing rules to be applied.
- `internal/doctor` accurately reports the subsystem status (e.g., active workers, configured limits).