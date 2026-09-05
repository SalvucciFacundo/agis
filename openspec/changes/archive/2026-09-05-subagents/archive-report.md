# Archive Report: subagents

## Change Overview
- **Name**: `subagents`
- **Archived Date**: 2026-09-05
- **Status**: Completed & Archived
- **Mode**: Automatic (`auto`)
- **Artifact Store**: Hybrid (`openspec/` + Engram)
- **Delivery Strategy**: `auto-chain` (`stacked-to-main`)

## Summary of Accomplishments
1. **Configuration & Foundation (`internal/config`, `internal/core`)**:
   - Added `SubagentsConfig` with safe defaults (`Enabled: true`, `MaxConcurrent: 3`, `MaxDepth: 1`, `DefaultTimeout: 60s`, `MaxTurns: 8`) and boundary clamping (`MaxConcurrent` [1, 10], `MaxDepth` [1, 2], `MaxTurns` [1, 15], `DefaultTimeout` [5s, 300s]).
   - Added `CategoryExecution = "execution"` in `internal/core/port_policy.go`.
2. **Ephemeral Repository & Subagent Engine (`internal/subagents`)**:
   - Implemented thread-safe in-memory `ephemeralRepository` implementing `core.Repository`, isolating child conversations and message scratchpads from the main database while sharing search, observations, skills, and audit logging.
   - Built `subagents.Engine` with semaphore concurrency pooling (`chan struct{}`), timeout propagation, recursion depth clamping (`MaxDepth` up to 2), tool inheritance filtering (stripping `delegate_task` at max depth), and output synthesis with turn exhaustion warning handling.
3. **Tool Runner & PolicyGuard Integration (`internal/tools`, `internal/policy`, `internal/core`)**:
   - Implemented `SubagentRunner` (`internal/tools/subagent.go`) for tool `delegate_task` under backend `"subagent"` and category `core.CategoryExecution`.
   - Wired `delegate_task` into `tools.Registry` and `PolicyGuard` with subject truncation (up to 256 chars) and full audit trail logging.
4. **Diagnostics & Documentation (`internal/doctor`, `docs/`)**:
   - Implemented `checkSubagents` diagnostic probe in `internal/doctor/subagents.go` and registered it in the doctor test suite.
   - Wired `subagents.Engine` into `cmd/agis/main.go`.
   - Updated `docs/configuration.md`, `docs/cli.md`, and `README.md`.
   - Synced master specification to `openspec/specs/subagents/spec.md`.

## Verification Results
- **Strict TDD Compliance**: 100% verified across all 4 work units.
- **Specification Requirements**: 14/14 requirements and scenarios satisfied (PASS).
- **Test Suite**: 24/24 Go packages passing with `go test -race -count=1 ./...` and clean `go vet ./...` (zero races, zero leaks, zero vet warnings).

## Final State Facts
- Packages added/modified: `internal/config`, `internal/core`, `internal/subagents`, `internal/tools`, `internal/policy`, `internal/doctor`, `cmd/agis`.
- Artifacts archived to: `openspec/changes/archive/2026-09-05-subagents/`
- Master spec at: `openspec/specs/subagents/spec.md`
