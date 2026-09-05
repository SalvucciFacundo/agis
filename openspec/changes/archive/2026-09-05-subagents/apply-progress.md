# Apply Progress: Native Subagent Delegation (`subagents`)

## Completed Tasks (All Units Complete)

### Unit 1: Configuration, Port Policy & Ephemeral Repository Foundation
- [x] Implement and verify RED test for `SubagentsConfig` loading and hard boundary clamping (`internal/config/config_test.go`).
- [x] Implement `SubagentsConfig` struct with explicit defaults (`Enabled: true`, `MaxConcurrent: 3`, `MaxDepth: 1`, `DefaultTimeout: 60s`, `MaxTurns: 8`) and boundary clamping in `internal/config/config.go`.
- [x] Implement and verify RED test for `core.CategoryExecution` constant and `PolicyGuard` routing for backend `"subagent"` (`internal/policy/guard_test.go`).
- [x] Add `CategoryExecution = "execution"` constant in `internal/core/port_policy.go` and update `PolicyGuard` evaluate logic.
- [x] Implement and verify RED test for `ephemeralRepository` in `internal/subagents/ephemeral_repo_test.go` confirming isolation from parent storage.
- [x] Implement `ephemeralRepository` wrapping `core.Repository` in `internal/subagents/ephemeral_repo.go`.

### Unit 2: Subagent Engine, Semaphore, Depth & Context Management
- [x] Implement and verify RED test for `subagents.Engine` concurrency limit enforcement (semaphore pool) and context timeout propagation in `internal/subagents/engine_test.go`.
- [x] Implement `Engine` structure with semaphore channel (`chan struct{}`), timeout handling, depth tracking (`subagentDepthKey`), and child brain instantiation in `internal/subagents/engine.go`.
- [x] Implement and verify RED test for recursion depth clamping (max depth `2`) and child tool inheritance filtering (excluding `delegate_task` at max depth) in `internal/subagents/engine_depth_test.go`.
- [x] Implement child brain tool filtering and depth increment logic in `internal/subagents/engine.go`.

### Unit 3: Tool Runner, PolicyGuard Wiring & Execution Logic
- [x] Implement and verify RED test for `delegate_task` tool runner parameter validation, empty task rejection, `max_turns` clamping, and output synthesis in `internal/tools/subagent_test.go`.
- [x] Implement `delegate_task` tool runner (`subagentRunner`) with backend `"subagent"`, input schema, and execution invocation in `internal/tools/subagent.go`.
- [x] Implement and verify RED test for `PolicyGuard` evaluation of `"subagent"` tool under Sandbox, Standard, and Full postures in `internal/policy/guard_subagent_test.go`.
- [x] Wire `subagentRunner` into `internal/tools/registry.go` and ensure `PolicyGuard` correctly inspects `CategoryExecution`.

### Unit 4: Health Check Probe, Main Integration & Documentation
- [x] Implement and verify RED test for `doctor` subagent probe verification (`checkSubagents`) in `internal/doctor/subagents_test.go` covering enabled, disabled, and clamped states.
- [x] Implement `checkSubagents` probe in `internal/doctor/subagents.go` and register it in `internal/doctor/doctor.go`.
- [x] Wire `subagents.Engine` initialization into `cmd/agis/main.go` with configuration and policy guards.
- [x] Update `docs/configuration.md`, `docs/cli.md`, and `README.md` to document subagent delegation configuration and tool usage.

---

## TDD Cycle Evidence

| Unit | Phase | Target | Test Evidence / Outcome |
|------|-------|--------|-------------------------|
| Unit 1 | RED | `internal/config/config_test.go`, `internal/config/accessor_test.go` | Build failed: `cfg.Subagents undefined` |
| Unit 1 | RED | `internal/policy/guard_test.go` | Build failed: `undefined: core.CategoryExecution` |
| Unit 1 | RED | `internal/subagents/ephemeral_repo_test.go` | Build failed: `package subagents has no non-test Go files` |
| Unit 1 | GREEN | `internal/config/config.go`, `internal/core/port_policy.go`, `internal/policy/guard.go`, `internal/subagents/ephemeral_repo.go` | All unit tests pass: `go test -race -count=1 ./internal/config/... ./internal/policy/... ./internal/subagents/...` (PASS) |
| Unit 1 | REFACTOR | `internal/subagents/ephemeral_repo.go` | Concurrency-safe in-memory maps, proper locking, zero race conditions |
| Unit 2 | RED | `internal/subagents/engine_test.go`, `internal/subagents/engine_depth_test.go` | Build failed: `undefined: subagents.NewEngine`, `undefined: subagents.ContextWithDepth` |
| Unit 2 | GREEN | `internal/core/brain.go`, `internal/subagents/engine.go` | Concurrency semaphore, timeout derivation, tool depth filtering, child brain execution with output synthesis |
| Unit 2 | REFACTOR | `internal/core/brain.go`, `internal/subagents/engine.go` | Context cancellation propagation check, turn limit reached detection, 100% test pass with `goleak.VerifyNone` |
| Unit 3 | RED | `internal/tools/subagent_test.go`, `internal/policy/guard_subagent_test.go`, `internal/core/brain_tools_test.go` | Build failed: `undefined: NewSubagentRunner`, `undefined: FromSubagentsEngine`, `guard evaluated 0 times, want 1` |
| Unit 3 | GREEN | `internal/tools/subagent.go`, `internal/core/brain.go` | `SubagentRunner` (`delegate_task`), parameter validation, `max_turns` clamping, `core.CategoryExecution` routing in `executeTool` |
| Unit 3 | REFACTOR | `internal/core/brain_tools_test.go`, `internal/tools/subagent_test.go` | Added long task string truncation (256 chars) and comprehensive edge case validation |
| Unit 4 | RED | `internal/doctor/subagents_test.go` | Build failed: `doc.checkSubagents undefined` |
| Unit 4 | GREEN | `internal/doctor/subagents.go`, `internal/doctor/doctor.go`, `cmd/agis/main.go` | `checkSubagents` probe wired into Doctor suite, `subagents.Engine` initialized in `main.go` |
| Unit 4 | REFACTOR | `docs/configuration.md`, `docs/cli.md`, `README.md` | Full documentation synchronization across CLI reference, config guide, and root README |

---

## Changed Files
- `internal/config/config.go`: Added `SubagentsConfig` and boundary clamping in `applyDefaults`.
- `internal/config/config_test.go`: Added tests for subagents default loading and boundary clamping.
- `internal/config/accessor_test.go`: Added tests for `Get` and `Set` on `subagents.*` keys.
- `internal/core/port_policy.go`: Added `CategoryExecution = "execution"`.
- `internal/core/brain.go`: Added `WithMaxTurns`, `TurnLimitReached()`, subagent tool execution routing with `CategoryExecution` and task truncation.
- `internal/core/brain_tools_test.go`: Added `TestBrainLoop_SubagentTool_EvaluationAndExecution` and `TestBrainLoop_SubagentTool_LongTaskTruncationInGuard`.
- `internal/policy/guard.go`: Updated sandbox evaluation to permit allowed execution for `subagent` backend.
- `internal/policy/guard_test.go`: Added tests for `subagent` backend under sandbox, standard, and full postures.
- `internal/policy/guard_subagent_test.go`: Dedicated security tier and audit tests for subagent delegation.
- `internal/subagents/ephemeral_repo.go`: In-memory isolated repository implementing `core.Repository`.
- `internal/subagents/ephemeral_repo_test.go`: Tests for ephemeral repository isolation and delegation.
- `internal/subagents/engine.go`: Subagent execution engine with semaphore, depth tracking, and child brain execution.
- `internal/subagents/engine_test.go`: Concurrency, timeout, empty task, and leak verification tests.
- `internal/subagents/engine_depth_test.go`: Depth tracking, recursion limit rejection, tool filtering, and turn limit warning tests.
- `internal/tools/subagent.go`: `SubagentRunner` (`delegate_task`), `SubagentSpawner` interface, and parameter parsing.
- `internal/tools/subagent_test.go`: Comprehensive unit tests for `delegate_task` tool runner.
- `internal/doctor/subagents.go`: Diagnostic probe for subagent subsystem.
- `internal/doctor/subagents_test.go`: Unit tests for subagents diagnostic probe.
- `internal/doctor/doctor.go`: Wired `checkSubagents` into health check probe suite.
- `internal/doctor/doctor_test.go`: Added `"subagents"` to verified diagnostic checks list.
- `cmd/agis/main.go`: Wired `subagents.Engine` initialization and `delegate_task` tool into main application brain.
- `docs/configuration.md`: Documented `subagents` YAML schema, defaults, and boundary constraints.
- `docs/cli.md`: Documented subagent doctor probe in CLI reference.
- `README.md`: Added Native Subagent Delegation to core capabilities.
- `openspec/changes/subagents/tasks.md`: Marked all implementation tasks complete.

---

## Test Commands Run
- `go test -race -count=1 ./internal/config/... ./internal/subagents/... ./internal/policy/... ./internal/core/... ./internal/tools/... ./internal/doctor/... ./cmd/agis/...`
- `go test -race -count=1 ./...` (All 24 packages PASS)
- `go vet ./...` (Clean, zero findings)

---

## Deviations from Design
None. The design and specifications were implemented with full conformance.

---

## Remaining Tasks
None. All units in `tasks.md` are 100% completed.

---

## Workload / PR Boundary
- **Batch 1 (Units 1 & 2)**: Foundation (Config, Policy, Ephemeral Repo, Subagent Engine, Concurrency & Depth).
- **Batch 2 (Units 3 & 4)**: Integration (Tool Runner, PolicyGuard Routing, Doctor Probe, Main Wiring & Docs).
- Total implementation complete and ready for verification phase.
