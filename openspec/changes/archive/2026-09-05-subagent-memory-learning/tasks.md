# Tasks: Subagent Memory Learning (subagent-memory-learning)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~250-350 additions, ~20 deletions |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

```text
Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Low
```

---

## Task Breakdown & Work Units

### Unit 1: Configuration & Distillation Parser
- [x] **Task 1.1 (RED)**: Write unit tests in `internal/config/config_test.go` and `internal/subagents/distill_test.go` covering `SubagentsConfig` defaults, clamping bounds for `max_observations`, and markdown extraction cases (`## Key Learnings`, `## Decisions`, list bullets/numbers, sanitization, and max bounds). <!-- sdd-owner: implementation -->
- [x] **Task 1.2 (GREEN)**: Update `internal/config/config.go` to add `LearningEnabled` and `MaxObservations` with defaults and clamping rules (`[1, 5]`), and implement `internal/subagents/distill.go` with `ExtractKeyLearnings` and `sanitizeTaskSlug`. <!-- sdd-owner: implementation -->
- [x] **Task 1.3 (REFACTOR & VERIFY)**: Verify tests pass with `go test -v -race ./internal/config/... ./internal/subagents/...`. <!-- sdd-owner: implementation -->

### Unit 2: Engine Integration, Persistence Pipeline & Audit Logging
- [x] **Task 2.1 (RED)**: Write unit tests in `internal/subagents/engine_test.go` validating that successful subagent runs trigger `SaveObservations` and audit logging when `LearningEnabled: true`, skip when `false`, and gracefully handle memory I/O errors without failing `Spawn`. <!-- sdd-owner: implementation -->
- [x] **Task 2.2 (GREEN)**: Update `internal/subagents/engine.go` to invoke `ExtractKeyLearnings` upon successful completion, saving distilled observations to `e.parent` and recording learning audit entries via `e.parent.AppendAudit` with `Backend: "subagent"`, `Category: "learning"`, catching and logging save errors as warnings. <!-- sdd-owner: implementation -->
- [x] **Task 2.3 (REFACTOR & VERIFY)**: Run tests with `go test -v -race ./internal/subagents/...` to ensure all execution and distillation integration tests pass cleanly. <!-- sdd-owner: implementation -->

### Unit 3: Doctor Diagnostics Probe & Documentation
- [x] **Task 3.1 (RED)**: Write unit tests in `internal/doctor/subagents_test.go` verifying `checkSubagents` includes learning enabled status and observation ceilings correctly across enabled, disabled, and clamped states. <!-- sdd-owner: implementation -->
- [x] **Task 3.2 (GREEN)**: Update `internal/doctor/subagents.go` (`checkSubagents`) to include learning status and observation limits in the check details. <!-- sdd-owner: implementation -->
- [x] **Task 3.3 (REFACTOR & DOCS)**: Update documentation files (`docs/cli.md`, `docs/configuration.md`, `README.md`) to document `learning_enabled` and `max_observations` subagent parameters. Verify full project test suite with `go test -race ./...`. <!-- sdd-owner: implementation -->
