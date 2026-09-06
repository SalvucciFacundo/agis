# SDD Apply Progress: skill-index-and-creator

## Progress Summary

- **Batch 1 Completed**:
  - Work Unit 1: Skill Hub Extensions, Nested Loader & agentskills.io Validator (`internal/skills`, `internal/core`)
  - Work Unit 2: Lazy Loading, Config & Model Tools (`internal/config`, `internal/core`, `internal/tools`)
- **Batch 2 Completed (Final Implementation Slice)**:
  - Work Unit 3: CLI Subcommand Suite (`cmd/agis/skill.go`, `cmd/agis/main.go`)
  - Work Unit 4: Doctor Diagnostics Probe, Main Integration & Documentation (`internal/doctor`, `cmd/agis/main.go`, `cmd/agis/serve.go`, `cmd/agis/gateway.go`, `docs/`, `README.md`)

## TDD Cycle Evidence

| Work Unit | Stage | Target / Test File | Evidence / Details |
|---|---|---|---|
| WU 1 | RED | `internal/skills/validator_test.go`, `loader_test.go`, `hub_test.go` | Asserted schema validation, nested dir layouts (`skills/<name>/SKILL.md`), thread-safe `GetSkill`/`Reload` concurrency. Tests failed to compile as expected. |
| WU 1 | GREEN | `internal/skills/validator.go`, `loader.go`, `hub.go`, `internal/core/port_learning.go` | Implemented `ValidateSkillContent`/`ValidateSkill`, nested loader with YAML frontmatter parsing, `sync.RWMutex` on `Hub`, and extended `core.SkillHub` interface. |
| WU 1 | REFACTOR | `internal/skills/registry.go`, `hub.go` | Unified `SyncRegistry` using thread-safe `Skills()` snapshot. Tests passed under `-race`. |
| WU 2 | RED | `internal/config/config_test.go`, `internal/core/brain_context_test.go`, `internal/tools/read_skill_test.go`, `internal/tools/create_skill_test.go` | Asserted `SkillsConfig.LazyLoading` default true, compact Markdown index table prompt generation vs full body fallback, `read_skill` lookup + usage tracking, `create_skill` atomic write + agentskills.io validation + overwrite protection. Tests failed to compile as expected. |
| WU 2 | GREEN | `internal/config/config.go`, `internal/core/brain.go`, `internal/core/port_learning.go`, `internal/tools/read_skill.go`, `internal/tools/create_skill.go`, `internal/tools/registry.go` | Added `LazyLoading` config, lazy index system prompt rendering, `ReadSkillRunner` & `CreateSkillRunner` tools conforming to `agentskills.io`, tool registration helpers. |
| WU 2 | REFACTOR | `internal/tools/registry.go` | Exported `SkillRunners` helper. Ran full test suite with `-race` detection. |
| WU 3 | RED | `cmd/agis/skill_test.go` | Asserted `agis skill` CLI subcommands (`list`, `create`, `show`, `delete`), flags (`-json`, `-desc`, `-trigger`, `-force`, `-raw`, `-yes`), stdin confirmation, stream separation, and POSIX exit codes (0, 1, 2). Test compilation failed as expected. |
| WU 3 | GREEN | `cmd/agis/skill.go`, `cmd/agis/main.go`, `internal/core/port_repository.go`, `internal/memory/sqlite.go` | Implemented `RunSkillCLI` and `RunSkillCLIWithIn`, subcommands `list`, `create` (with agentskills.io scaffolding), `show` (formatted/raw/json), and `delete` (file removal, repository deletion, hub/registry sync). Extended `core.Repository` with `DeleteSkill`. Wired `case "skill", "skills"` in `cmd/agis/main.go`. |
| WU 3 | REFACTOR | `cmd/agis/skill_test.go` | Verified CLI execution with race detector enabled (`go test -race -count=1 ./cmd/agis/...`). |
| WU 4 | RED | `internal/doctor/doctor_test.go` | Asserted `checkSkills` health probe validates `agentskills.io` standard across flat and nested layouts, reports loaded skill counts, and flags malformed skills with `WARN` status and remediation guidance. Tests failed as expected. |
| WU 4 | GREEN | `internal/doctor/doctor.go`, `cmd/agis/main.go`, `cmd/agis/serve.go`, `cmd/agis/gateway.go` | Updated `checkSkills` probe to validate YAML frontmatter and required H2 sections, wired `tools.SkillRunners` in TUI, API server, and gateway, updated `docs/skills.md`, `docs/cli.md`, `docs/configuration.md`, and `README.md`. |
| WU 4 | REFACTOR | `internal/doctor/doctor.go` | Validated full test suite (`go test -race -count=1 ./...`) and `go vet ./...` with 100% pass across all 26 packages. |

## Completed Tasks

- [x] Task 1.1 (RED): Unit tests in `internal/skills/validator_test.go` for agentskills.io conformance.
- [x] Task 1.2 (GREEN): `internal/skills/validator.go` with `ValidateSkillContent` and `ValidateSkill`.
- [x] Task 1.3 (RED): Unit tests in `internal/skills/loader_test.go` for flat and nested directory layouts.
- [x] Task 1.4 (GREEN): `internal/skills/loader.go` with flat and nested directory loader (`SKILL.md` / `<name>.md`).
- [x] Task 1.5 (RED): Unit tests in `internal/skills/hub_test.go` for `GetSkill`, `Skills`, `RecordUse`, and `Reload` concurrency.
- [x] Task 1.6 (GREEN): Extended `core.SkillHub` in `internal/core/port_learning.go` and implemented in `internal/skills/hub.go` with `sync.RWMutex`.
- [x] Task 2.1 (RED): Unit tests in `internal/config/config_test.go` and `internal/core/brain_context_test.go` for lazy loading config & prompt rendering.
- [x] Task 2.2 (GREEN): `internal/config/config.go` with `SkillsConfig.LazyLoading` and `internal/core/brain.go` lazy prompt index table injection.
- [x] Task 2.3 (RED): Unit tests in `internal/tools/read_skill_test.go` for skill lookup and usage tracking.
- [x] Task 2.4 (GREEN): `internal/tools/read_skill.go` implementing `core.ToolRunner`.
- [x] Task 2.5 (RED): Unit tests in `internal/tools/create_skill_test.go` for atomic write, overwrite protection, schema validation.
- [x] Task 2.6 (GREEN): `internal/tools/create_skill.go` implementing `core.ToolRunner`.
- [x] Task 3.1 (RED): Unit and integration tests in `cmd/agis/skill_test.go` verifying subcommands `list`, `create`, `show`, `delete`.
- [x] Task 3.2 (GREEN): `cmd/agis/skill.go` with `RunSkillCLI` and `RunSkillCLIWithIn`.
- [x] Task 3.3 (GREEN): Wires `case "skill", "skills"` in `cmd/agis/main.go`.
- [x] Task 4.1 (RED): Unit tests in `internal/doctor/doctor_test.go` verifying `checkSkills` probe.
- [x] Task 4.2 (GREEN): `internal/doctor/doctor.go`'s `checkSkills` probe with agentskills.io validation.
- [x] Task 4.3 (IMPLEMENTATION): Updated `docs/cli.md`, `docs/configuration.md`, `docs/skills.md`, and `README.md`.
- [x] Task 4.4 (VERIFICATION): Full test suite across the entire project with race detector enabled (`go test -race -count=1 ./...`) and static analysis (`go vet ./...`).

## Files Changed

- `cmd/agis/skill.go` (new)
- `cmd/agis/skill_test.go` (new)
- `cmd/agis/main.go` (modified)
- `cmd/agis/serve.go` (modified)
- `cmd/agis/gateway.go` (modified)
- `internal/core/port_repository.go` (modified)
- `internal/core/port_learning.go` (modified)
- `internal/core/brain.go` (modified)
- `internal/core/brain_test.go` (modified)
- `internal/core/brain_context_test.go` (modified)
- `internal/memory/sqlite.go` (modified)
- `internal/memory/fakes_test.go` (modified)
- `internal/skills/validator.go` (new)
- `internal/skills/validator_test.go` (new)
- `internal/skills/loader.go` (modified)
- `internal/skills/loader_test.go` (modified)
- `internal/skills/hub.go` (modified)
- `internal/skills/hub_test.go` (modified)
- `internal/skills/registry.go` (modified)
- `internal/tools/read_skill.go` (new)
- `internal/tools/read_skill_test.go` (new)
- `internal/tools/create_skill.go` (new)
- `internal/tools/create_skill_test.go` (new)
- `internal/tools/registry.go` (modified)
- `internal/tools/registry_test.go` (modified)
- `internal/config/config.go` (modified)
- `internal/config/config_test.go` (modified)
- `internal/doctor/doctor.go` (modified)
- `internal/doctor/doctor_test.go` (modified)
- `internal/persona/persona_test.go` (modified)
- `internal/server/chat_test.go` (modified)
- `internal/subagents/ephemeral_repo.go` (modified)
- `internal/subagents/ephemeral_repo_test.go` (modified)
- `internal/adapters/tui/app_test.go` (modified)
- `docs/cli.md` (modified)
- `docs/configuration.md` (modified)
- `docs/skills.md` (modified)
- `README.md` (modified)
- `openspec/changes/skill-index-and-creator/tasks.md` (modified)
- `openspec/changes/skill-index-and-creator/apply-progress.md` (modified)

## Verification Evidence

- `go test -race -count=1 ./cmd/agis/...` -> PASS (all tests passing)
- `go test -race -count=1 ./internal/doctor/...` -> PASS (all tests passing)
- `go test -race -count=1 ./...` -> PASS (all 26 packages passing with 0 data races and 0 memory leaks)
- `go vet ./...` -> PASS (clean static analysis)

## Remaining Tasks

None. All implementation tasks in `tasks.md` (Work Units 1, 2, 3, and 4) are 100% complete.
