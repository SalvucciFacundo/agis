# Apply Progress: Subagent Memory Learning (subagent-memory-learning)

## Overview
Implementation of passive knowledge distillation from subagents to parent SQLite memory store and audit log.

- **Status**: Completed
- **Strategy**: single PR (`stacked-to-main`)
- **TDD Mode**: Strict TDD (RED -> GREEN -> REFACTOR/VERIFY)

---

## TDD Cycle Evidence

| Unit | Phase | Test / Action | Outcome |
|------|-------|---------------|---------|
| Unit 1 | RED | `TestLoad_SubagentsDefaultsAndClamping` + `TestExtractKeyLearnings_*` + `TestSanitizeTaskSlug` | Failed as expected (missing `LearningEnabled`, `MaxObservations`, `ExtractKeyLearnings`, `sanitizeTaskSlug`) |
| Unit 1 | GREEN | Added `LearningEnabled` and `MaxObservations` in `internal/config/config.go`, implemented `internal/subagents/distill.go` | Passed (`go test -v -race ./internal/config/... ./internal/subagents/...`) |
| Unit 1 | REFACTOR | Added reflection test cases in `internal/config/accessor_test.go`, verified slug boundary conditions | All Unit 1 tests passed cleanly |
| Unit 2 | RED | `TestEngine_DistillationAndPersistence`, `TestEngine_DistillationSkippedWhenLearningDisabled`, `TestEngine_DistillationGracefulDegradationOnSaveError`, `TestEngine_DistillationGracefulDegradationOnAuditError` | Failed as expected (`parent.observations len = 0, want 2`) |
| Unit 2 | GREEN | Integrated distillation hook in `internal/subagents/engine.go` before return with non-blocking error logging | All Unit 2 tests passed with race detector |
| Unit 2 | REFACTOR | Verified context cancellation, depth limits, and `goleak` verification | Clean execution |
| Unit 3 | RED | `TestDoctor_CheckSubagents_Enabled` & `TestDoctor_CheckSubagents_LearningDisabledAndClamping` | Failed as expected (missing learning status and ceiling details) |
| Unit 3 | GREEN | Updated `internal/doctor/subagents.go` (`checkSubagents`), updated documentation in `docs/cli.md`, `docs/configuration.md`, and `README.md` | Doctor probe tests passed |
| Unit 3 | VERIFY | Ran full suite across entire repo: `go test -race -count=1 ./...` and `go vet ./...` | All 26 packages passed with 0 race conditions or vet warnings |

---

## Files Changed
- `internal/config/config.go`: Added `LearningEnabled` and `MaxObservations` fields with defaults and clamping `[1, 5]`.
- `internal/config/config_test.go`: Added test cases for learning defaults and boundary clamping.
- `internal/config/accessor_test.go`: Added get/set reflection tests for new config keys.
- `internal/subagents/distill.go` (new): Implemented `ExtractKeyLearnings` and `sanitizeTaskSlug`.
- `internal/subagents/distill_test.go` (new): Comprehensive table-driven tests for parsing markdown headings, lists, sanitization, and clamping.
- `internal/subagents/engine.go`: Hooked knowledge distillation and parent persistence before returning from `Spawn`.
- `internal/subagents/engine_test.go`: Added engine persistence, skipped learning, and error degradation tests.
- `internal/subagents/ephemeral_repo_test.go`: Added mock error injection for `SaveObservations` and `AppendAudit`.
- `internal/doctor/subagents.go`: Added learning enabled status and observation ceiling probe details.
- `internal/doctor/subagents_test.go`: Added test cases for doctor probe reporting.
- `docs/cli.md`: Updated doctor diagnostic documentation.
- `docs/configuration.md`: Documented `subagents.learning_enabled` and `subagents.max_observations`.
- `README.md`: Included `subagents` configuration sample.
- `openspec/changes/subagent-memory-learning/tasks.md`: Marked all implementation tasks complete.

---

## Verification Commands Run
- `go test -v -race -count=1 ./internal/config/... ./internal/subagents/...`
- `go test -v -race -count=1 ./internal/doctor/...`
- `go test -race -count=1 ./...`
- `go vet ./...`
