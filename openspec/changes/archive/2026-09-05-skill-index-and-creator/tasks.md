## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~950 - 1200 lines |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Core Hub, Loader & Validator) → PR 2 (Model Tools & Config) → PR 3 (CLI Subcommand Suite & Doctor Probe) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

---

## Task Breakdown & Work Units

Each work unit follows strict TDD (RED → GREEN → TRIANGULATE → REFACTOR) and is fully tested with race detection and memory checks.

### Work Unit 1: Skill Hub Extensions, Nested Loader & Validator (`internal/skills`, `internal/core`)

- [x] **Task 1.1 (RED):** Write unit tests in `internal/skills/validator_test.go` asserting conformance with the `agentskills.io` standard (frontmatter regex `^[a-zA-Z0-9_-]{1,40}$`, required fields `name`, `description`, and required Markdown level-2 headers `## When to Use`, `## Critical Rules`, `## Workflow`, `## Examples`). Assert that invalid names or missing sections return explicit errors. <!-- sdd-owner: implementation -->
- [x] **Task 1.2 (GREEN):** Implement `internal/skills/validator.go` providing `ValidateSkillContent(raw string) error` and `ValidateSkill(skill core.Skill) error` using globally compiled regexes. <!-- sdd-owner: implementation -->
- [x] **Task 1.3 (RED):** Write unit tests in `internal/skills/loader_test.go` asserting support for both flat layouts (`skills/<name>.md`) and nested directory layouts (`skills/<name>/SKILL.md`), graceful skipping of malformed files, and frontmatter parsing. <!-- sdd-owner: implementation -->
- [x] **Task 1.4 (GREEN):** Implement `internal/skills/loader.go` with `LoadDir` supporting flat and nested layouts via `filepath.WalkDir` or standard directory iteration, robust YAML unmarshaling into `frontMatter`, and warning logs for skipped non-compliant files. <!-- sdd-owner: implementation -->
- [x] **Task 1.5 (RED):** Write unit tests in `internal/skills/hub_test.go` asserting thread-safe `Hub` methods including `GetSkill`, `Skills`, `RecordUse`, and `Reload` using `t.TempDir()` and `-race`. <!-- sdd-owner: implementation -->
- [x] **Task 1.6 (GREEN):** Extend `core.SkillHub` interface in `internal/core/port_learning.go` with `GetSkill(name string) (*Skill, bool)` and `Reload(ctx context.Context, dir string) error`. Implement them in `internal/skills/hub.go` protected by `sync.RWMutex`. <!-- sdd-owner: implementation -->

---

### Work Unit 2: Lazy Loading, Config & Model Tools (`internal/config`, `internal/core`, `internal/tools`)

- [x] **Task 2.1 (RED):** Write unit tests in `internal/config/config_test.go` and `internal/core/brain_test.go` verifying `SkillsConfig.LazyLoading` boolean configuration (default `true`) and prompt system message rendering (compact Markdown index table vs full body injection). <!-- sdd-owner: implementation -->
- [x] **Task 2.2 (GREEN):** Update `internal/config/config.go` with `SkillsConfig.LazyLoading` and update prompt assembly in `internal/core/brain.go` (or learning ports) to inject a compact Markdown table and the instructional prompt when `LazyLoading` is true, falling back to full injection when false. <!-- sdd-owner: implementation -->
- [x] **Task 2.3 (RED):** Write unit tests in `internal/tools/read_skill_test.go` asserting `ReadSkillRunner` correctly looks up skills via `SkillHub.GetSkill`, records usage, and returns formatted JSON status (`found` vs `not_found`). <!-- sdd-owner: implementation -->
- [x] **Task 2.4 (GREEN):** Implement `ReadSkillRunner` in `internal/tools/read_skill.go` implementing `core.ToolRunner` with `"internal"` backend. <!-- sdd-owner: implementation -->
- [x] **Task 2.5 (RED):** Write unit tests in `internal/tools/create_skill_test.go` asserting `CreateSkillRunner` validates skill schema, prevents accidental overwrites when `overwrite: false`, performs atomic file writes (`0600` permissions via temporary file and `os.Rename`), and invokes `SkillHub.Reload`. <!-- sdd-owner: implementation -->
- [x] **Task 2.6 (GREEN):** Implement `CreateSkillRunner` in `internal/tools/create_skill.go` implementing `core.ToolRunner` conforming to the `agentskills.io` standard. <!-- sdd-owner: implementation -->

---

### Work Unit 3: CLI Subcommand Suite (`cmd/agis/skill.go`, `cmd/agis/main.go`)

- [x] **Task 3.1 (RED):** Write unit and integration tests in `cmd/agis/skill_test.go` verifying Cobra subcommands `list`, `create`, `show`, and `delete` with flags `--json`, `--desc`, `--trigger`, `--raw`, and `--force`/`--yes`, asserting correct stdout/stderr outputs and Unix exit codes. <!-- sdd-owner: implementation -->
- [x] **Task 3.2 (GREEN):** Implement `cmd/agis/skill.go` containing the Cobra subcommand hierarchy for `agis skill` (`list`, `create`, `show`, `delete`) with rich table output (`tablewriter`), colorized output (`fatih/color`), and JSON serialization support. <!-- sdd-owner: implementation -->
- [x] **Task 3.3 (GREEN):** Wire up the `skill` command group in `cmd/agis/main.go` and verify end-to-end command registration. <!-- sdd-owner: implementation -->

---

### Work Unit 4: Doctor Diagnostics Probe, Documentation & Final Verification (`internal/doctor`, `docs/`)

- [x] **Task 4.1 (RED):** Write unit tests in `internal/doctor/doctor_test.go` verifying `checkSkills` probe correctly inspects `$AGIS_HOME/skills/` (flat and nested), validates agent skills conformance, reports loaded counts, and returns `WARNING` on malformed skills without crashing. <!-- sdd-owner: implementation -->
- [x] **Task 4.2 (GREEN):** Update `internal/doctor/doctor.go`'s `checkSkills` to run schema validation on all discovered skills and report detailed diagnostics and remediation paths. <!-- sdd-owner: implementation -->
- [x] **Task 4.3 (IMPLEMENTATION):** Update documentation in `docs/cli.md`, `docs/configuration.md`, and `README.md` detailing the new lazy loading config, `agis skill` commands, `read_skill` / `create_skill` tools, and diagnostic probes. <!-- sdd-owner: implementation -->
- [x] **Task 4.4 (VERIFICATION):** Run full test suite with race detector enabled (`go test -race ./...`), verify zero data races, check that code coverage meets project thresholds, and confirm all requirements in `spec.md` are fully met. <!-- sdd-owner: implementation -->
