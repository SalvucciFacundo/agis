# SDD Architecture and Design: Skill Index and Creator

## 1. Architecture Decision Records (ADRs)

### D1: Skill Index Generation & Lazy Injection Architecture
- **Context**: Passing full skill bodies in system prompts consumes excessive tokens and risks context overflow.
- **Decision**: Introduce `SkillsConfig.LazyLoading` boolean toggle (default: `true`) in `internal/config/config.go`. In `internal/core/brain.go` (or `port_learning.go`), when `LazyLoading` is true, map matched skills to a compact Markdown table (`| Skill | Trigger / Description | Scope / Version |`). Append the instruction: `"To read complete procedural instructions, call read_skill(name)"`. If false, fallback to full content injection for backward compatibility.

### D2: Hub Interface Extension
- **Context**: The model tools and CLI need dynamic access to specific skills and the ability to reload the cache.
- **Decision**: Extend `core.SkillHub` with `GetSkill(name string) (*Skill, bool)` and `Reload(ctx context.Context, dir string) error`. Implement thread safety in `*skills.Hub` using `sync.RWMutex` to protect internal maps during `Reload` writes and concurrent `GetSkill` reads.

### D3: Dynamic Nested & Flat Loader Pattern
- **Context**: Skills can be single Markdown files or directories containing a `SKILL.md` file.
- **Decision**: Update `internal/skills/loader.go`'s `LoadDir` to use `filepath.WalkDir` or standard directory reading to support both `$AGIS_HOME/skills/<name>.md` and `$AGIS_HOME/skills/<name>/SKILL.md`. Map standard `agentskills.io` frontmatter (`name`, `description`, `trigger`, `license`, `metadata.author`, `metadata.version`) into the expanded `frontMatter` Go struct.

### D4: `agentskills.io` Standard Validator
- **Context**: Skills must conform to a strict schema to guarantee interoperability and safety.
- **Decision**: Create `internal/skills/validator.go` with `ValidateSkillContent(raw string) error` and `ValidateSkill(skill core.Skill) error`. Check skill names against `^[a-zA-Z0-9_-]{1,40}$`. Validate the presence of Markdown level-2 headers (`## When to Use`, `## Critical Rules`, `## Workflow`, `## Examples`) using globally compiled `regexp` to minimize allocations. Return specific, actionable error messages.

### D5: Model Tools Architecture
- **Context**: The LLM needs native ability to read and write skills.
- **Decision**: Implement `internal/tools/read_skill.go` and `internal/tools/create_skill.go` fulfilling the `core.ToolRunner` interface with a `"internal"` backend.
  - `read_skill`: Looks up skills by name via `core.SkillHub` and returns full Markdown content. Records skill usage via `SkillHub.RecordUse`.
  - `create_skill`: Performs input validation, verifies the target file path, rejects overwrites unless `overwrite: true`, performs an atomic file write to disk, and triggers `Hub.Reload()`.

### D6: CLI Subcommand Suite
- **Context**: Users require local commands to manage the skill lifecycle.
- **Decision**: Add `cmd/agis/skill.go` implementing `agis skill list`, `agis skill create`, `agis skill show`, and `agis skill delete`. Utilize `github.com/spf13/cobra` for routing, `github.com/fatih/color` for CLI coloring, and `github.com/olekukonko/tablewriter` for terminal tables. Exit cleanly with `0` on success and appropriate Unix exit codes (e.g., `1` for general errors, `2` for usage) on failure. Support `--json` flags for machine-readable output.

### D7: Doctor Diagnostic Probes
- **Context**: Non-compliant or malformed skills should be surfaced to the user without crashing the system.
- **Decision**: Update the `checkSkills` probe in `internal/doctor/doctor.go` to iterate over all loaded skills, invoking `ValidateSkillContent`. Aggregate findings and emit a `WARNING` (rather than a fatal error) if any skill is malformed, providing explicit paths and missing fields for remediation.

---

## 2. Component Interactions & Sequence Diagrams

**Agent Tool Call Sequence (create_skill):**
1. **Agent** generates JSON payload: `{"name": "test-skill", "description": "...", "body": "...", "overwrite": false}`
2. **Brain** dispatches payload to `create_skill` `ToolRunner`.
3. `create_skill` calls `validator.ValidateSkillContent(body)`.
   - *If invalid*: Returns detailed JSON error back to agent.
4. `create_skill` validates file existence.
   - *If exists & !overwrite*: Returns JSON error.
5. `create_skill` writes to temporary file (`.tmp`), chmods to `0600`, and `os.Rename`s to `skills/test-skill/SKILL.md`.
6. `create_skill` calls `SkillHub.Reload(ctx, dir)`.
7. **SkillHub** re-indexes skills and updates `.atl/skill-registry.md`.
8. `create_skill` returns success JSON payload to the Agent.

**Prompt Injection Sequence (Lazy Loading):**
1. **Brain** prepares system prompt.
2. `Brain` calls `Hub.Skills()` and matches relevant skills via triggers/context.
3. Checks `SkillsConfig.LazyLoading`.
4. *If true*: Generates `| Skill | Description | Scope |` Markdown table + usage instruction.
5. *If false*: Generates full Markdown blocks (`- name:\n content`).

---

## 3. Data Structures, Types & Method Signatures

```go
// internal/core/port_learning.go
type SkillHub interface {
    Skills() []Skill
    RecordUse(ctx context.Context, name string)
    GetSkill(name string) (*Skill, bool)
    Reload(ctx context.Context, dir string) error
}

// internal/skills/hub.go
type Hub struct {
    mu     sync.RWMutex
    skills map[string]*core.Skill
    dir    string
    // ...
}

// internal/skills/loader.go
type frontMatter struct {
    Name        string            `yaml:"name"`
    Description string            `yaml:"description"`
    Trigger     string            `yaml:"trigger"`
    License     string            `yaml:"license"`
    Metadata    map[string]string `yaml:"metadata"` // Contains 'author', 'version'
}

// internal/tools/read_skill.go
type ReadSkillInput struct {
    Name string `json:"name"`
}

// internal/tools/create_skill.go
type CreateSkillInput struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Trigger     string `json:"trigger,omitempty"`
    Body        string `json:"body"`
    Overwrite   bool   `json:"overwrite"`
    License     string `json:"license,omitempty"`
    Author      string `json:"author,omitempty"`
}
```

---

## 4. Security, Threat Modeling & Defensive File System I/O

- **Path Traversal Prevention**: The `name` input in `create_skill`, `read_skill`, and the CLI commands is strictly validated against the regular expression `^[a-zA-Z0-9_-]{1,40}$`. This structurally eliminates injection of slashes (`/`), parent directories (`../`), or relative path manipulation.
- **Atomic Writes**: `create_skill` writes all new skills to a temporary file (e.g. `filepath.Join(dir, name, "SKILL.tmp")`), securely flushes the buffer, applies `0600` permissions via `os.Chmod`, and utilizes `os.Rename` to atomically move it into place. This prevents race conditions or partially written files during concurrent loads.
- **Resource Exhaustion**: Regular expressions used for Markdown section validation are globally compiled (`regexp.MustCompile`) to avoid excessive memory allocations during parsing.
- **Overwrite Protection**: User-authored skills are protected by default. The `create_skill` tool will fail predictably if the target skill path already exists, demanding an explicit `"overwrite": true` flag to proceed.

---

## 5. Testing Strategy

1. **Unit Tests (Table-Driven)**: Apply strict table-driven testing with `t.Run()` named subtests for `validator.go`, `loader.go`, and tool runners. Subtests must be independently runnable and check positive matches alongside edge cases (e.g. invalid frontmatter, missing Markdown headers).
2. **Integration Tests & Temp Directories**: Use `t.TempDir()` in tests that exercise `create_skill` and `loader.go` behavior. Validate the fallback handling between nested (`SKILL.md`) and flat (`name.md`) architectures without mocking the file system. Use `//go:build integration` tags if tests exceed unit time limits (<1ms).
3. **Goroutine Leaks & Race Detection**: Embed `goleak.VerifyTestMain(m)` inside `TestMain` functions for the `internal/skills` and `internal/tools` packages. Run all tests with the `-race` flag during CI to ensure `Hub.mu` correctly guards concurrent `Reload()` and `GetSkill()` operations.
4. **CLI Tests**: Utilize `cmd.OutOrStdout()` and `cmd.ErrOrStderr()` inside Cobra commands. Write test cases that invoke the commands directly using buffered outputs to assert accurate CLI outputs (JSON structure, tabular data formatting, correct BSD exit codes `0`, `1`, `2`).