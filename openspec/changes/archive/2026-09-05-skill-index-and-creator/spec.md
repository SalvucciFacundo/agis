# Specification: Skill Index, Lazy Loading & Autonomous Creator (skill-index-and-creator)

## Purpose

Optimize prompt token consumption by introducing lazy loading and a compact Markdown index table for skills in system prompts, provide native model tools (`read_skill`, `create_skill`) for autonomous procedural knowledge discovery and creation conforming to the `agentskills.io` standard, introduce the `agis skill` CLI command suite for local skill lifecycle management, and expand system diagnostics in `internal/doctor`.

---

## 1. Skill Hub & Lazy Loading (`internal/skills`, `internal/core`, `internal/config`)

### Requirement SKL-IDX-001: Lazy Loading Configuration & System Prompt Index
The system MUST support a `SkillsConfig.LazyLoading` boolean configuration flag (default: `true`) in `internal/config/config.go`.
1. When `LazyLoading` is `true` (default), `Brain.Step` / prompt assembly MUST NOT inject full skill Markdown contents. Instead, when matching skills are present, it MUST inject a compact Markdown table index listing matched skills with columns: `| Skill | Trigger / Description | Scope / Version |` followed by the exact instructional notice:
   `"To read complete procedural instructions, call read_skill(name)"`.
2. When `LazyLoading` is `false`, the system MUST maintain backward compatibility and inject the full Markdown body of all matched skills (`- <name>: <content>`).

#### Scenario: Prompt index generated when lazy loading is enabled
- GIVEN `skills.lazy_loading: true` and two matched skills (`"git-rebase"`, `"deploy-staging"`)
- WHEN the Brain prepares the system messages for a conversation turn
- THEN the system prompt contains a Markdown table with `"git-rebase"` and `"deploy-staging"` and their descriptions
- AND the system prompt includes `"To read complete procedural instructions, call read_skill(name)"`
- AND the full body instructions of `"git-rebase"` are NOT present in the system prompt

#### Scenario: Full body injection when lazy loading is disabled
- GIVEN `skills.lazy_loading: false` and matched skill `"git-rebase"` with body `"## Steps\n1. git pull"`
- WHEN the Brain prepares the system messages
- THEN the system prompt contains the full body text `"## Steps\n1. git pull"` for `"git-rebase"`

---

### Requirement SKL-IDX-002: `core.SkillHub` Interface Extension & Dynamic Reload
The `core.SkillHub` interface in `internal/core/port_learning.go` MUST be extended with:
- `GetSkill(name string) (*Skill, bool)`: Retrieves a specific skill by exact case-sensitive or normalized name from memory/repository. Returns `(skill, true)` if found, or `(nil, false)` if not found.
- `Reload(ctx context.Context, dir string) error`: Scans the specified skills directory, imports all valid skill files, re-indexes the in-memory skill list, and updates `.atl/skill-registry.md`.

The concrete implementation `*skills.Hub` in `internal/skills/hub.go` MUST implement these methods thread-safely and gracefully handle missing or empty directories.

#### Scenario: GetSkill returns existing skill
- GIVEN a loaded skill `"docker-build"` in `Hub`
- WHEN `GetSkill("docker-build")` is called
- THEN it returns a pointer to the `core.Skill` struct and `true`

#### Scenario: GetSkill returns not found for non-existent skill
- GIVEN an empty `Hub`
- WHEN `GetSkill("unknown-skill")` is called
- THEN it returns `nil` and `false`

#### Scenario: Reload re-indexes skills after disk changes
- GIVEN a newly written skill file `skills/new-tool/SKILL.md`
- WHEN `hub.Reload(ctx, dir)` is invoked
- THEN `hub.Skills()` includes `"new-tool"` and `hub.GetSkill("new-tool")` returns the skill

---

### Requirement SKL-IDX-003: Nested Directory & Standard File Layout Support
`skills.LoadDir` in `internal/skills/loader.go` MUST support two file structures inside the skills directory:
1. Flat layout: `$AGIS_HOME/skills/<name>.md`
2. Nested directory layout: `$AGIS_HOME/skills/<name>/SKILL.md` (or `$AGIS_HOME/skills/<name>/<name>.md`)

Files or directories with invalid names or unparseable frontmatter MUST be skipped with logged warnings, never causing startup or loading failure.

#### Scenario: Nested SKILL.md layout loads correctly
- GIVEN a directory `~/.agis/skills/code-review/SKILL.md` with valid YAML frontmatter
- WHEN `skills.LoadDir` scans `~/.agis/skills`
- THEN the `"code-review"` skill is parsed, validated, and returned

#### Scenario: Mixed flat and nested layouts load simultaneously
- GIVEN flat file `skills/deploy.md` and nested file `skills/test/SKILL.md`
- WHEN `skills.LoadDir` is executed
- THEN both `"deploy"` and `"test"` skills are returned

---

### Requirement SKL-CRT-001: agentskills.io Standard Conformance & Validation
The system MUST enforce the `agentskills.io` standard for all skill definitions:
1. **Frontmatter Constraints**:
   - `name`: String, required, MUST match regex `^[a-zA-Z0-9_-]+$`, maximum length 40 characters.
   - `description`: String, required, non-empty, maximum length 500 characters.
   - `trigger`: String, optional.
   - `license`: String, optional (default: `"Apache-2.0"` if omitted).
   - `metadata`: Optional YAML map/object containing `author` (string) and `version` (string, e.g. `"1.0"`).
2. **Body Section Constraints**:
   - The Markdown body MUST contain the following level-2 headers:
     - `## When to Use` (or `## Description & Intent`)
     - `## Critical Rules` (or `## Rules`)
     - `## Workflow` (or `## Steps` / `## Procedure`)
     - `## Examples` (or `## Reference`)

Validation helpers MUST be exported in `internal/skills` (e.g., `ValidateSkill(skill core.Skill) error` or `ValidateSkillContent(raw string) error`) to provide detailed diagnostic error messages indicating exactly which required field or section is missing.

#### Scenario: Fully valid agentskills.io skill passes validation
- GIVEN a skill content with valid YAML frontmatter (`name: "go-lint"`, `description: "Run linter"`, `metadata: {author: "tester", version: "1.0"}`) and sections `## When to Use`, `## Critical Rules`, `## Workflow`, and `## Examples`
- WHEN `skills.ValidateSkillContent` is executed
- THEN validation succeeds with no error

#### Scenario: Invalid skill name rejected
- GIVEN frontmatter with `name: "invalid name with spaces!"`
- WHEN validation runs
- THEN it returns a validation error indicating invalid name characters

#### Scenario: Missing required section rejected
- GIVEN frontmatter that is valid, but markdown body missing `## Critical Rules`
- WHEN validation runs
- THEN it returns a validation error detailing missing section `"## Critical Rules"`

---

## 2. Model Tools for Skills (`internal/tools`)

### Requirement TOL-SKL-001: `read_skill` Tool Runner
The system MUST provide a tool runner `ReadSkillRunner` in `internal/tools/read_skill.go` implementing `core.ToolRunner`:
1. `Name()` MUST return `"read_skill"`.
2. `Description()` MUST return `"Fetch the full procedural instructions, rules, and examples of a specified skill by name."`.
3. `Backend()` MUST return `"internal"`.
4. `Run(ctx, input)`:
   - Input MUST be a JSON object: `{"name": "<skill_name>"}`.
   - If `name` is missing or empty, it MUST return an error indicating `"skill name is required"`.
   - It MUST query `core.SkillHub.GetSkill(name)`.
   - If the skill is found, it MUST record skill usage via `SkillHub.RecordUse(ctx, name)` and return a JSON payload:
     ```json
     {
       "status": "found",
       "name": "<skill_name>",
       "description": "<description>",
       "trigger": "<trigger>",
       "content": "<full_markdown_body>"
     }
     ```
   - If the skill is not found, it MUST return a JSON payload with `"status": "not_found"` or an error `fmt.Errorf("skill %q not found", name)` allowing the LLM to understand that the requested skill does not exist.

#### Scenario: `read_skill` successfully fetches skill body
- GIVEN a registered skill `"golang-testing"` with body `"## Workflow\n1. Write test"`
- WHEN `read_skill` is executed with `{"name": "golang-testing"}`
- THEN the tool output returns JSON containing `"status": "found"` and the full content
- AND `SkillHub.RecordUse` is invoked for `"golang-testing"`

#### Scenario: `read_skill` returns error when skill does not exist
- GIVEN no skill named `"nonexistent-skill"`
- WHEN `read_skill` is executed with `{"name": "nonexistent-skill"}`
- THEN the tool returns an error or JSON indicating skill not found

---

### Requirement TOL-SKL-002: `create_skill` Tool Runner
The system MUST provide a tool runner `CreateSkillRunner` in `internal/tools/create_skill.go` implementing `core.ToolRunner`:
1. `Name()` MUST return `"create_skill"`.
2. `Description()` MUST return `"Create or update a reusable procedural skill adhering to the agentskills.io standard."`.
3. `Backend()` MUST return `"internal"`.
4. `Run(ctx, input)`:
   - Input MUST be a JSON object:
     ```json
     {
       "name": "skill-name",
       "description": "Short summary",
       "trigger": "optional trigger keywords",
       "body": "Markdown content with required sections (## When to Use, ## Critical Rules, ## Workflow, ## Examples)",
       "overwrite": false,
       "license": "Apache-2.0",
       "author": "agent"
     }
     ```
   - Validation: It MUST validate the name regex (`^[a-zA-Z0-9_-]{1,40}$`), non-empty description, and presence of the standard Markdown sections.
   - Destination Path: It MUST resolve the file path to `$AGIS_HOME/skills/<name>/SKILL.md` (or `$AGIS_HOME/skills/<name>.md`).
   - Conflict Prevention: If the target file exists and `overwrite` is `false`, it MUST return an error indicating the skill already exists and requesting confirmation/overwrite flag.
   - Atomicity: It MUST construct the standard YAML frontmatter + body and write atomically via a temporary file with `0600` permissions.
   - Index Sync: Upon successful write, it MUST invoke `SkillHub.Reload(ctx, skillsDir)` and regenerate `.atl/skill-registry.md`.
   - Output: It MUST return a JSON payload confirming creation:
     ```json
     {
       "status": "created",
       "name": "<name>",
       "path": "<saved_path>"
     }
     ```

#### Scenario: `create_skill` creates a compliant skill
- GIVEN valid skill arguments with name `"docker-deploy"`, description `"Build and deploy docker"`, and complete sections
- WHEN `create_skill` runs with `overwrite: false`
- THEN a new file is created at `skills/docker-deploy/SKILL.md` with valid YAML frontmatter and sections
- AND `hub.Reload` is called
- AND the tool returns `{"status": "created", "name": "docker-deploy", "path": "..."}`

#### Scenario: `create_skill` rejects invalid section structure
- GIVEN skill arguments missing the `## Examples` section
- WHEN `create_skill` runs
- THEN the tool returns an error indicating missing section `"## Examples"` without writing to disk

#### Scenario: `create_skill` prevents unintended overwrite
- GIVEN an existing skill file `skills/git-flow/SKILL.md`
- WHEN `create_skill` runs with `name: "git-flow"` and `overwrite: false`
- THEN the tool returns an error indicating skill `"git-flow"` already exists and aborts write

---

## 3. CLI Management (`cmd/agis/skill.go`)

### Requirement CLI-SKL-001: `agis skill` Subcommand Tree
The CLI entry points in `cmd/agis/` MUST provide the `agis skill` command router with subcommands:
1. `agis skill list [--json]`:
   - Lists all installed skills in a tabular format (Columns: `Name`, `Trigger`, `Source`, `Uses`, `Description`).
   - With `--json` flag, outputs the list as formatted JSON array.
   - Exits with status 0.
2. `agis skill create <name> [--desc "<description>"] [--trigger "<trigger>"]`:
   - Validates `<name>` against regex `^[a-zA-Z0-9_-]{1,40}$`.
   - Creates a scaffolded `SKILL.md` file in `$AGIS_HOME/skills/<name>/SKILL.md` populated with boilerplate YAML frontmatter and template markdown sections (`## When to Use`, `## Critical Rules`, `## Workflow`, `## Examples`).
   - If the file already exists, it MUST return an error unless `--force` / `--overwrite` is specified.
   - Reloads the skill hub and syncs the registry.
   - Exits with status 0 on success, non-zero on error.
3. `agis skill show <name> [--raw] [--json]`:
   - Shows the skill metadata and formatted Markdown body.
   - With `--raw`, prints the exact underlying file contents including frontmatter.
   - With `--json`, prints structured JSON representation of the skill.
   - Exits with status 0 if found, status 1 if not found.
4. `agis skill delete <name> [--force] [--yes]`:
   - Deletes the skill file or directory (`$AGIS_HOME/skills/<name>/` or `$AGIS_HOME/skills/<name>.md`).
   - Prompts for confirmation when interactive unless `--force` or `--yes` is passed.
   - Removes the skill from the database repository and reloads the skill hub.
   - Exits with status 0 on success.

#### Scenario: `agis skill list` prints table of installed skills
- GIVEN skills `"golang-testing"` and `"deploy-prod"` installed
- WHEN `agis skill list` is executed
- THEN it prints a formatted table containing both skills with exit code 0

#### Scenario: `agis skill create` scaffolds a new standard skill
- GIVEN no existing skill `"k8s-rollback"`
- WHEN `agis skill create k8s-rollback --desc "Rollback k8s deployment"` is executed
- THEN file `skills/k8s-rollback/SKILL.md` is created with valid YAML frontmatter and template sections
- AND exit code is 0

#### Scenario: `agis skill show` displays skill details
- GIVEN installed skill `"k8s-rollback"`
- WHEN `agis skill show k8s-rollback` runs
- THEN it outputs the skill metadata and markdown body to stdout with exit code 0

#### Scenario: `agis skill delete` removes skill and cleans up
- GIVEN installed skill `"temporary-skill"`
- WHEN `agis skill delete temporary-skill --yes` runs
- THEN the skill file is deleted, the repository row is removed, and exit code is 0

---

## 4. Diagnostics & Health Probe (`internal/doctor`)

### Requirement DOC-SKL-001: Skills Health & agentskills.io Conformance Probe
The `internal/doctor/doctor.go` subsystem MUST update its `checkSkills` probe:
1. It MUST inspect all files in `$AGIS_HOME/skills/` (both flat and nested layouts).
2. For each discovered skill, it MUST check:
   - YAML frontmatter validity (`name`, `description`).
   - Name regex format compliance (`^[a-zA-Z0-9_-]{1,40}$`).
   - Standard markdown section compliance (`## When to Use`, `## Critical Rules`, `## Workflow`, `## Examples`).
3. It MUST report total loaded skills, any formatting warnings or errors, and the status of `.atl/skill-registry.md`.
4. If invalid skills are found, `checkSkills` MUST return status `WARNING` with actionable remediation guidance, never crashing.

#### Scenario: Doctor reports healthy skills
- GIVEN 5 fully compliant skills in `$AGIS_HOME/skills/`
- WHEN `agis doctor` runs
- THEN the `skills` check reports `OK` with `"5 skills loaded and validated"`

#### Scenario: Doctor flags non-compliant skill with warning
- GIVEN a skill file missing required sections
- WHEN `agis doctor` runs
- THEN the `skills` check reports `WARNING` and specifies the problematic file and missing sections
