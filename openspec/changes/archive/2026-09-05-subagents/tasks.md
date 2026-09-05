## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1200 - 1600 lines (across config, core, subagents engine, tools, doctor, and tests) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Config, Port Policy & Ephemeral Repo) → PR 2 (Subagent Engine, Semaphore & Concurrency) → PR 3 (Tool Runner, PolicyGuard Integration & Main Wiring) → PR 4 (Doctor Probe, Integration Tests & Docs) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

---

## Work Units

### Unit 1: Configuration, Port Policy & Ephemeral Repository Foundation
- [x] Implement and verify RED test for `SubagentsConfig` loading and hard boundary clamping (`internal/config/config_test.go`). <!-- sdd-owner: implementation -->
- [x] Implement `SubagentsConfig` struct with explicit defaults (`Enabled: true`, `MaxConcurrent: 3`, `MaxDepth: 1`, `DefaultTimeout: 60s`, `MaxTurns: 8`) and boundary clamping in `internal/config/config.go`. <!-- sdd-owner: implementation -->
- [x] Implement and verify RED test for `core.CategoryExecution` constant and `PolicyGuard` routing for backend `"subagent"` (`internal/core/port_policy_test.go`). <!-- sdd-owner: implementation -->
- [x] Add `CategoryExecution = "execution"` constant in `internal/core/port_policy.go` and update `PolicyGuard` evaluate logic. <!-- sdd-owner: implementation -->
- [x] Implement and verify RED test for `ephemeralRepository` in `internal/subagents/ephemeral_repo_test.go` confirming isolation from parent storage. <!-- sdd-owner: implementation -->
- [x] Implement `ephemeralRepository` wrapping `core.Repository` in `internal/subagents/ephemeral_repo.go`. <!-- sdd-owner: implementation -->

### Unit 2: Subagent Engine, Semaphore, Depth & Context Management
- [x] Implement and verify RED test for `subagents.Engine` concurrency limit enforcement (semaphore pool) and context timeout propagation in `internal/subagents/engine_test.go`. <!-- sdd-owner: implementation -->
- [x] Implement `Engine` structure with semaphore channel (`chan struct{}`), timeout handling, depth tracking (`subagentDepthKey`), and child brain instantiation in `internal/subagents/engine.go`. <!-- sdd-owner: implementation -->
- [x] Implement and verify RED test for recursion depth clamping (max depth `2`) and child tool inheritance filtering (excluding `delegate_task` at max depth) in `internal/subagents/engine_depth_test.go`. <!-- sdd-owner: implementation -->
- [x] Implement child brain tool filtering and depth increment logic in `internal/subagents/engine.go`. <!-- sdd-owner: implementation -->

### Unit 3: Tool Runner, PolicyGuard Wiring & Execution Logic
- [x] Implement and verify RED test for `delegate_task` tool runner parameter validation, empty task rejection, `max_turns` clamping, and output synthesis in `internal/tools/subagent_test.go`. <!-- sdd-owner: implementation -->
- [x] Implement `delegate_task` tool runner (`subagentRunner`) with backend `"subagent"`, input schema, and execution invocation in `internal/tools/subagent.go`. <!-- sdd-owner: implementation -->
- [x] Implement and verify RED test for `PolicyGuard` evaluation of `"subagent"` tool under Sandbox, Standard, and Full postures in `internal/policy/guard_subagent_test.go`. <!-- sdd-owner: implementation -->
- [x] Wire `subagentRunner` into `internal/tools/registry.go` and ensure `PolicyGuard` correctly inspects `CategoryExecution`. <!-- sdd-owner: implementation -->

### Unit 4: Health Check Probe, Main Integration & Documentation
- [x] Implement and verify RED test for `doctor` subagent probe verification (`checkSubagents`) in `internal/doctor/subagents_test.go` covering enabled, disabled, and clamped states. <!-- sdd-owner: implementation -->
- [x] Implement `checkSubagents` probe in `internal/doctor/subagents.go` and register it in `internal/doctor/doctor.go`. <!-- sdd-owner: implementation -->
- [x] Wire `subagents.Engine` initialization into `cmd/agis/main.go` with configuration and policy guards. <!-- sdd-owner: implementation -->
- [x] Update `docs/configuration.md`, `docs/cli.md`, and `README.md` to document subagent delegation configuration and tool usage. <!-- sdd-owner: implementation -->
