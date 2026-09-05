# Specification: Setup Wizard & Multi-Profile Management Subsystem

## Purpose

Define the functional requirements, CLI interfaces, and package contracts for the interactive/non-interactive Setup Wizard (`agis setup` / `agis init`) and the Multi-Profile Management Subsystem (`agis profile` and global `--profile` integration). This enables turnkey onboarding for new users, live provider validation, secure configuration creation (`0600`), and clean multi-tenant/multi-agent context isolation (distinct configuration, memory databases, personas, skills, and policies per profile).

---

## Requirements

### Domain 1: Setup Wizard (`agis setup` / `agis init`)

#### Requirement SETUP-001: Setup Command Entrypoint and Alias
The system MUST provide an `agis setup` subcommand and an identical alias `agis init` in `cmd/agis/setup.go` to guide users through initial setup or update existing configuration.
- Routing: `agis setup` and `agis init` MUST be intercepted before TUI launch and executed via `RunSetupCLI(args []string, stdin io.Reader, stdout, stderr io.Writer) int`.
- Help and Usage: When invoked with `--help`, `-h`, or `-help`, the command MUST print usage information to `stdout` and exit with code `0`.
- Flags:
  - `-provider <name>`: LLM provider name (`ollama`, `openai`, `openrouter`, `anthropic`). Default is `"ollama"`.
  - `-model <name>`: LLM model name (e.g., `llama3.2`, `gpt-4o`, `anthropic/claude-3.5-sonnet`).
  - `-api-key <key>`: API key or token for the chosen provider (optional/skipped for Ollama).
  - `-base-url <url>`: Custom API base URL (e.g., `http://localhost:11434` for Ollama).
  - `-non-interactive`: Flag to bypass interactive terminal prompts and run in headless automation mode.
  - `-force`: Skip live connectivity verification or overwrite existing configuration without prompting.
  - `-config <path>`: Custom configuration file destination path override.
  - `-profile <name>`: Target profile name to configure (defaulting to the active or root profile).

##### Scenario: Setup help displayed
- GIVEN the user executes `agis setup --help` or `agis init --help`
- WHEN the command line arguments are parsed
- THEN usage syntax, available flags, and provider options are printed to `stdout` and exit code is `0`

---

#### Requirement SETUP-002: Interactive Wizard Flow
When `stdin` is a character terminal (TTY) and `-non-interactive` is `false`, `agis setup` MUST execute an interactive prompt loop.
- Step 1 (Provider): Prompt the user to select an LLM provider from supported choices (`ollama`, `openai`, `openrouter`, `anthropic`).
- Step 2 (API Key): Prompt for the API key with masked input (echoing asterisks `*` or hiding input). If `ollama` is selected, the API key prompt MUST be skipped or marked optional.
- Step 3 (Model): Prompt for the model name, presenting sensible defaults for the selected provider (`llama3.2` for Ollama, `gpt-4o` for OpenAI, `anthropic/claude-3.5-sonnet` for OpenRouter, `claude-3-5-sonnet-20241022` for Anthropic).
- Step 4 (Base URL): Prompt for an optional custom endpoint URL, pre-filled with standard defaults (e.g. `http://localhost:11434` for Ollama).
- Step 5 (Validation Probe): Execute a live bounded connectivity probe (Requirement `SETUP-004`). If probe fails, display the error and prompt the user to retry, edit values, or abort.
- Step 6 (Persistence): Atomically write the resulting configuration to `$AGIS_HOME/config.yaml` (or the profile's `config.yaml`) with file mode `0600`.

##### Scenario: Interactive wizard completes successfully
- GIVEN an interactive TTY terminal and no pre-existing configuration
- WHEN the user runs `agis setup`, selects `ollama`, accepts default model `llama3.2`, and passes connectivity check
- THEN `config.yaml` is created with mode `0600`, confirmation is printed to `stdout`, and exit code is `0`

##### Scenario: Interactive wizard masks API key entry
- GIVEN the user selects `openai` in the interactive wizard
- WHEN the user inputs an API key
- THEN the keystrokes are masked in terminal output and not echoed in plaintext

---

#### Requirement SETUP-003: Non-Interactive Setup Automation
When `-non-interactive` is `true` (or when `stdin` is not a TTY without explicit interactive flags), `agis setup` MUST execute headlessly using provided CLI flags.
- Validation: The command MUST validate that required parameters for the specified provider are supplied (e.g., `-api-key` required for `openai` and `openrouter`).
- Overwrite Protection: If the target configuration file already exists and `-force` is omitted, the command MUST output an error to `stderr` indicating the file exists and exit with code `1`.
- If `-force` is provided, the command MUST overwrite the target configuration file.
- Connectivity Check: The command MUST execute the live connectivity probe (Requirement `SETUP-004`) unless `-force` is specified. If the probe fails without `-force`, it MUST write the failure to `stderr` and exit with code `1`.

##### Scenario: Non-interactive setup succeeds with flags
- GIVEN environment parameters and `-non-interactive` flag
- WHEN `agis setup -non-interactive -provider openai -model gpt-4o -api-key "sk-valid-key" -force` is executed
- THEN configuration is written atomically with mode `0600` and exit code is `0`

##### Scenario: Non-interactive setup fails on missing required API key
- GIVEN `-non-interactive` and `-provider openai` without `-api-key`
- WHEN `agis setup -non-interactive -provider openai` runs
- THEN the system writes `"agis setup: -api-key is required for provider 'openai'"` to `stderr` and exits with code `2`

##### Scenario: Non-interactive setup fails when config exists without force
- GIVEN an existing `config.yaml`
- WHEN `agis setup -non-interactive -provider ollama` runs without `-force`
- THEN the system writes an error to `stderr` stating config already exists and exits with code `1`

---

#### Requirement SETUP-004: Live Connectivity Probe
The setup subsystem MUST validate provider credentials and reachability before persisting configuration.
- The probe MUST execute with a bounded timeout of 5 seconds (`context.WithTimeout(ctx, 5*time.Second)`).
- Probe implementations:
  - For `ollama`: Probe `GET /api/tags` or `GET /api/version` on the configured base URL.
  - For `openai` / `openrouter` / `anthropic`: Probe `GET /v1/models` (or lightweight endpoint) with the `Authorization: Bearer <key>` header.
- Error reporting: On connection failure, HTTP 401/403 (unauthorized), or timeout, the probe MUST return a structured error detailing the failure cause.

##### Scenario: Connectivity probe succeeds against reachability check
- GIVEN a running Ollama instance on `http://localhost:11434`
- WHEN the setup probe runs for provider `ollama`
- THEN the probe completes within 5 seconds with status PASS

##### Scenario: Connectivity probe fails on invalid API key
- GIVEN an invalid OpenAI API key `"sk-invalid"`
- WHEN the setup probe executes against OpenAI endpoint
- THEN the probe returns an authentication failure error and setup does not persist invalid credentials without `-force`

---

#### Requirement SETUP-005: Atomic Configuration Persistence and Permissions
The setup command MUST ensure atomic creation and strict POSIX permissions for generated files.
- Directory creation: Any missing parent directories MUST be created with file mode `0700` (`-rwx------`).
- Atomic Write: Data MUST be written to a temporary file (e.g. `config.yaml.tmp.<pid>`), flushed to disk (`Sync()`), chmoded to `0600` (`-rw-------`), and atomically renamed over the target file path.
- Mode Verification: The resulting configuration file MUST have mode `0600`.

##### Scenario: Setup creates config with 0600 permissions
- GIVEN target path `$AGIS_HOME/config.yaml` does not exist
- WHEN `agis setup` successfully writes configuration
- THEN `$AGIS_HOME` directory has mode `0700`, `config.yaml` has mode `0600`, and exit code is `0`

---

### Domain 2: Multi-Profile Subsystem & Path Resolution

#### Requirement PROF-001: Profile Directory Layout and State Isolation
The system MUST support multi-profile state isolation under `$AGIS_HOME/profiles/<name>/`.
- Root/Default Profile:
  - Base directory: `$AGIS_HOME` (or `~/.agis`).
  - Contains: `config.yaml`, `agis.db`, `SOUL.md`, `skills/`, `policy.yaml`.
- Named Profile (`<name>`):
  - Base directory: `$AGIS_HOME/profiles/<name>/`.
  - Encapsulates isolated state:
    - Configuration: `$AGIS_HOME/profiles/<name>/config.yaml`
    - Memory Database: `$AGIS_HOME/profiles/<name>/agis.db`
    - Persona Soul: `$AGIS_HOME/profiles/<name>/SOUL.md`
    - Custom Skills: `$AGIS_HOME/profiles/<name>/skills/`
    - Tool/Security Policy: `$AGIS_HOME/profiles/<name>/policy.yaml`
- Profiles MUST remain completely isolated: operations within profile `A` MUST NOT read, mutate, or lock databases or files belonging to profile `B` or the root profile.

##### Scenario: Profile state isolation
- GIVEN named profile `"work"` exists under `$AGIS_HOME/profiles/work/`
- WHEN AGIS runs with active profile `"work"`
- THEN all database reads/writes target `$AGIS_HOME/profiles/work/agis.db` and persona loads from `$AGIS_HOME/profiles/work/SOUL.md`

---

#### Requirement PROF-002: Active Profile Resolution Precedence
The system MUST resolve the active profile according to the following strict precedence (highest to lowest):
1. **Explicit CLI Flag**: Global `--profile <name>` or `-profile <name>` argument.
2. **Environment Variable**: `AGIS_PROFILE` environment variable.
3. **Active Profile Pointer**: Content of `$AGIS_HOME/.active_profile` file.
4. **Default Profile**: Empty string `""` or `"default"`, resolving to the root `$AGIS_HOME` directory.

- The resolved profile name MUST be sanitized and validated (Requirement `PROF-003`).

##### Scenario: CLI flag overrides environment variable and pointer file
- GIVEN `$AGIS_HOME/.active_profile` contains `"personal"` and `AGIS_PROFILE="work"`
- WHEN `agis --profile research session list` is executed
- THEN the active profile is resolved as `"research"`

##### Scenario: Environment variable overrides pointer file
- GIVEN `$AGIS_HOME/.active_profile` contains `"personal"` and `AGIS_PROFILE="work"`
- WHEN `agis session list` is executed without `--profile` flag
- THEN the active profile is resolved as `"work"`

##### Scenario: Pointer file used when flag and environment variable are absent
- GIVEN `$AGIS_HOME/.active_profile` contains `"personal"`, `AGIS_PROFILE` is unset, and no `--profile` flag is passed
- WHEN `agis session list` is executed
- THEN the active profile is resolved as `"personal"`

##### Scenario: Root profile used when all overrides are absent
- GIVEN `$AGIS_HOME/.active_profile` does not exist, `AGIS_PROFILE` is unset, and no `--profile` flag is passed
- WHEN `agis session list` is executed
- THEN the active profile resolves to the default root `$AGIS_HOME`

---

#### Requirement PROF-003: Profile Name Validation & Security
The system MUST validate all profile names to prevent directory traversal and filesystem attacks.
- Allowed Pattern: Profile names MUST match the regular expression `^[a-zA-Z0-9_-]+$`.
- Length: Profile names MUST be between 1 and 32 characters in length.
- Forbidden Characters: The system MUST reject profile names containing path separators (`/`, `\`), path traversal segments (`..`), spaces, dots, control characters, or non-ASCII characters.
- Reserved Names: The name `"default"` is reserved to indicate the root profile.

##### Scenario: Valid profile names accepted
- GIVEN profile names `"work"`, `"dev_env"`, `"test-123"`
- WHEN `ValidateProfileName(name)` is called
- THEN validation succeeds with `nil` error

##### Scenario: Path traversal in profile name rejected
- GIVEN profile name `"../../etc"` or `"work/project"`
- WHEN `ValidateProfileName(name)` is called
- THEN validation fails with a descriptive error

---

#### Requirement PROF-004: Profile CLI Router (`agis profile`)
The system MUST provide an `agis profile` subcommand router in `cmd/agis/profile.go` supporting subcommands: `list`, `create`, `show`, `use` (alias `switch`), and `delete`.
- Unrecognized Subcommands: MUST write an error to `stderr`, show usage syntax, and exit with code `2`.
- Help Flag: `--help`, `-h`, or `help` MUST write profile CLI usage to `stdout` and exit with code `0`.

##### Scenario: Profile help displayed
- GIVEN the user executes `agis profile --help`
- WHEN parsed by the CLI router
- THEN usage syntax and available subcommands (`list`, `create`, `show`, `use`, `delete`) are printed to `stdout` and exit code is `0`

---

#### Requirement PROF-005: Profile List Subcommand (`agis profile list`)
The `agis profile list` subcommand MUST enumerate all available profiles and indicate which profile is active.
- Flags:
  - `-json`: Outputs profile list as formatted JSON to `stdout`.
- Tabular Output: When `-json` is false, output a formatted table with columns: `ACTIVE`, `NAME`, `PATH`. The currently active profile MUST be marked with an asterisk `*` or checkmark.
- Discovery: Profiles MUST be discovered by scanning `$AGIS_HOME/profiles/` directory entries plus the default root profile.

##### Scenario: List profiles in tabular format
- GIVEN profiles `"work"` and `"personal"` exist, with `"work"` active
- WHEN the user executes `agis profile list`
- THEN output lists `default`, `personal`, and `* work` with exit code `0`

##### Scenario: List profiles in JSON format
- GIVEN active profile `"work"`
- WHEN the user executes `agis profile list -json`
- THEN valid JSON array of profile objects containing `name`, `path`, and `is_active` is written to `stdout` and exit code is `0`

---

#### Requirement PROF-006: Profile Create Subcommand (`agis profile create`)
The `agis profile create <name>` subcommand MUST scaffold a new profile directory under `$AGIS_HOME/profiles/<name>/`.
- Arguments: Exactly one positional argument `<name>`.
- Flags:
  - `-clone <source>`: Optional source profile name to copy configuration, soul, skills, and policy from.
- Scaffolding:
  - Creates `$AGIS_HOME/profiles/<name>/` with mode `0700`.
  - Creates `skills/` subdirectory with mode `0700`.
  - If `-clone` is specified: Copies `config.yaml`, `SOUL.md`, `policy.yaml`, and `skills/` from the source profile. Database `agis.db` MUST NOT be copied (starts fresh).
  - If `-clone` is omitted: Generates default `config.yaml` with mode `0600`, copies or creates default `SOUL.md`, and default `policy.yaml`.
- Duplicate Guard: If `$AGIS_HOME/profiles/<name>/` already exists, the command MUST output an error to `stderr` and exit with code `1`.

##### Scenario: Create new blank profile
- GIVEN profile `"dev"` does not exist
- WHEN the user executes `agis profile create dev`
- THEN `$AGIS_HOME/profiles/dev/` is created with mode `0700`, default `config.yaml` is written with mode `0600`, and exit code is `0`

##### Scenario: Create profile by cloning existing profile
- GIVEN existing profile `"work"` with custom `SOUL.md` and `config.yaml`
- WHEN the user executes `agis profile create work-copy -clone work`
- THEN `$AGIS_HOME/profiles/work-copy/` is created containing cloned `config.yaml` and `SOUL.md`, but without copying `work/agis.db`

##### Scenario: Create profile with duplicate name fails
- GIVEN profile `"work"` already exists
- WHEN the user executes `agis profile create work`
- THEN the command writes `"agis profile: profile 'work' already exists"` to `stderr` and exits with code `1`

---

#### Requirement PROF-007: Profile Show Subcommand (`agis profile show`)
The `agis profile show [name]` subcommand MUST display path and status information for the specified profile (or active profile if omitted).
- Arguments: Zero or one positional argument `[name]`.
- Flags:
  - `-json`: Outputs profile details as JSON to `stdout`.
- Information displayed: Profile name, active status, base directory path, config file path, database path, soul path, skills directory path, and policy path.
- Error Handling: If the specified profile does not exist, writes an error to `stderr` and exits with code `1`.

##### Scenario: Show active profile details
- GIVEN active profile `"work"`
- WHEN `agis profile show` is executed
- THEN path locations for `config.yaml`, `agis.db`, `SOUL.md`, and `skills/` for `"work"` are written to `stdout` and exit code is `0`

##### Scenario: Show nonexistent profile
- GIVEN profile `"missing"` does not exist
- WHEN `agis profile show missing` is executed
- THEN an error is written to `stderr` and exit code is `1`

---

#### Requirement PROF-008: Profile Use/Switch Subcommand (`agis profile use` / `agis profile switch`)
The `agis profile use <name>` (and alias `agis profile switch <name>`) subcommand MUST set the default active profile pointer.
- Arguments: Exactly one positional argument `<name>`.
- Persistence:
  - If `<name>` is a valid existing named profile, writes `<name>\n` to `$AGIS_HOME/.active_profile` atomically with file mode `0600`.
  - If `<name>` is `"default"`, removes or empties `$AGIS_HOME/.active_profile`.
- Validation: If `<name>` is not `"default"` and `$AGIS_HOME/profiles/<name>/` does not exist, the command MUST write an error to `stderr` and exit with code `1` without modifying `.active_profile`.

##### Scenario: Switch active profile successfully
- GIVEN profile `"research"` exists
- WHEN the user executes `agis profile use research`
- THEN `$AGIS_HOME/.active_profile` contains `"research"`, confirmation is printed to `stdout`, and exit code is `0`

##### Scenario: Switch to default profile
- GIVEN `$AGIS_HOME/.active_profile` currently points to `"research"`
- WHEN the user executes `agis profile use default`
- THEN `$AGIS_HOME/.active_profile` is removed or reset to default, confirmation is printed to `stdout`, and exit code is `0`

##### Scenario: Switch to non-existent profile fails
- GIVEN profile `"ghost"` does not exist
- WHEN the user executes `agis profile use ghost`
- THEN the system writes `"agis profile: profile 'ghost' does not exist"` to `stderr` and exits with code `1`

---

#### Requirement PROF-009: Profile Delete Subcommand (`agis profile delete`)
The `agis profile delete <name>` subcommand MUST remove a named profile directory and all its encapsulated files.
- Arguments: Exactly one positional argument `<name>`.
- Flags:
  - `-force`: Deletes profile even if it is currently marked as the active profile in `.active_profile`.
- Guards:
  - Attempting to delete `"default"` root profile MUST fail with exit code `1`.
  - If `<name>` is the currently active profile and `-force` is false, MUST reject deletion with an error on `stderr` advising the user to switch profiles first or pass `-force`, exiting with code `1`.
  - If `<name>` is active and `-force` is true, deletes `$AGIS_HOME/profiles/<name>/` and resets `$AGIS_HOME/.active_profile` to default.
- Nonexistent profile: If `<name>` does not exist, writes an error to `stderr` and exits with code `1`.

##### Scenario: Delete non-active profile
- GIVEN profile `"temp"` exists and active profile is `"work"`
- WHEN the user executes `agis profile delete temp`
- THEN `$AGIS_HOME/profiles/temp/` is removed, confirmation is printed to `stdout`, and exit code is `0`

##### Scenario: Delete active profile without force rejected
- GIVEN active profile is `"temp"`
- WHEN the user executes `agis profile delete temp` without `-force`
- THEN deletion is rejected with error on `stderr` and exit code is `1`

##### Scenario: Delete active profile with force succeeds and resets active profile
- GIVEN active profile is `"temp"`
- WHEN the user executes `agis profile delete temp -force`
- THEN `$AGIS_HOME/profiles/temp/` is removed, `$AGIS_HOME/.active_profile` is reset, and exit code is `0`

---

### Domain 3: Global CLI & Subsystem Integration

#### Requirement INT-001: Universal Profile Flag & Context Propagation
The root command in `cmd/agis/main.go` and all CLI subcommands MUST support the global `--profile <name>` flag and resolve paths dynamically.
- Subcommands affected: `config`, `policy`, `gateway`, `cron`, `plugins`, `webhook`, `mcp`, `doctor`, `session`, `update`, and default TUI.
- Flag parsing: CLI entry points MUST parse `-profile` / `--profile` early and configure the active profile context before loading `config.yaml`, connecting to SQLite `agis.db`, initializing `SOUL.md`, or reading `skills/`.
- Path resolution functions in `internal/config` (`AgisHome()`, `ResolvePath()`, `defaultDBPath()`, `defaultSkillsDir()`, `defaultPluginsDir()`) MUST return paths scoped to the active profile when a named profile is active.

##### Scenario: Subcommand inherits global profile flag
- GIVEN named profile `"work"` with custom database
- WHEN `agis --profile work session list` is executed
- THEN `session list` queries `$AGIS_HOME/profiles/work/agis.db`

##### Scenario: Config subcommand modifies named profile configuration
- GIVEN named profile `"work"`
- WHEN `agis --profile work config set llm.model gpt-4o` is executed
- THEN `$AGIS_HOME/profiles/work/config.yaml` is updated with `0600` permissions without affecting the default profile

---

### Domain 4: Observability & Doctor Diagnostics

#### Requirement DOCT-001: Profile and Configuration Security Doctor Probes
The `internal/doctor` package MUST include health check probes verifying active profile status and file permission compliance.
- Check `profile`:
  - Name: `"profile"`
  - Title: `"Active Profile & Path Resolution"`
  - Status:
    - `PASS`: Active profile is valid, profile directory exists, and resolved paths are accessible.
    - `WARN`: Active profile directory is missing optional assets (e.g. `SOUL.md` using default fallback).
    - `FAIL`: `.active_profile` references a profile that does not exist on disk.
- Check `config_permissions`:
  - Name: `"config_perms"`
  - Title: `"Configuration File Permissions"`
  - Status:
    - `PASS`: Configuration file has mode `0600` (`-rw-------`).
    - `WARN`: Configuration file exists but has looser permissions (e.g., `0644`), exposing potential API keys to other local users.
    - `PASS` (with detail): Config file does not exist (using safe in-memory defaults).

##### Scenario: Doctor reports passing profile and 0600 config check
- GIVEN active profile `"work"` with `0600` `config.yaml`
- WHEN `agis doctor` is executed
- THEN doctor output shows `[PASS] Active Profile & Path Resolution (work)` and `[PASS] Configuration File Permissions (0600)`

##### Scenario: Doctor reports fail on broken active profile pointer
- GIVEN `$AGIS_HOME/.active_profile` points to `"deleted_profile"` which does not exist
- WHEN `agis doctor` is executed
- THEN doctor output flags `[FAIL] Active Profile` and report `HasFailures()` returns `true`

##### Scenario: Doctor reports warning on loose config permissions
- GIVEN `config.yaml` has file mode `0644`
- WHEN `agis doctor` is executed
- THEN doctor output flags `[WARN] Configuration File Permissions` with a warning message recommending `chmod 600`

---

## Technical Package APIs (`internal/config/profile.go`)

### Profile Management API Contracts
```go
package config

// ProfileInfo holds metadata for a discovered AGIS profile.
type ProfileInfo struct {
    Name     string `json:"name"`
    Path     string `json:"path"`
    IsActive bool   `json:"is_active"`
}

// ActiveProfile returns the resolved active profile name.
func ActiveProfile() string

// SetActiveProfile sets the in-process active profile override (from flag or env).
func SetActiveProfile(name string) error

// ListProfiles returns all available profiles including the default profile.
func ListProfiles() ([]ProfileInfo, error)

// CreateProfile scaffolds a new profile directory, optionally cloning from source.
func CreateProfile(name string, cloneSource string) error

// DeleteProfile removes a profile directory and handles active profile reset.
func DeleteProfile(name string, force bool) error

// SwitchProfile updates the .active_profile pointer file.
func SwitchProfile(name string) error

// ValidateProfileName validates the profile name against security rules.
func ValidateProfileName(name string) error

// ProfileDir returns the root directory for the specified profile name.
func ProfileDir(name string) string

// CurrentProfileDir returns the directory of the currently active profile.
func CurrentProfileDir() string
```

---

## Exit Codes & POSIX CLI Discipline

All newly introduced commands (`setup`, `init`, `profile`) MUST adhere to standard Unix/POSIX exit codes:
- `0`: Success (operation completed, valid probe, JSON emitted, help text printed).
- `1`: Operational/Runtime failure (connectivity probe failure, profile already exists, missing profile directory, I/O error).
- `2`: Syntax/Usage error (invalid flag, unknown subcommand, missing required flag/argument, invalid profile name syntax).
- Streams: Pure data/tables/JSON to `stdout`; diagnostics, progress, warnings, and errors to `stderr`.
