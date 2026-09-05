# Specification: Config CLI Subcommands and Configuration Management Extension

## Purpose

Define the command-line interface (`agis config`) and underlying configuration management API contracts in `internal/config` for inspecting, reading, and safely modifying AGIS configurations. This enables non-interactive configuration inspection, headless automation, type-validated configuration updates, credential masking, and guaranteed secure file permissions (`0600`) without requiring manual YAML file edits.

---

## Requirements

### CLI Subcommands (`cmd/agis`)

#### Requirement CLI-CFG-001: Config Root Subcommand & Routing
The `cmd/agis` CLI MUST provide an `agis config` subcommand router using the standard library `flag` package, invoked prior to Bubbletea TUI initialization.
- The command router MUST accept subcommands: `show`, `get`, `set`, and `path`.
- It MUST accept a `-config <path>` flag across subcommands to override the configuration file path.
- When invoked without subcommands, with `--help`, `-h`, `-help`, or `help`, it MUST write comprehensive usage instructions listing all available subcommands and flags to `stdout` and exit with code `0`.
- When invoked with an unrecognized subcommand, it MUST write an error message to `stderr`, output usage instructions, and exit with code `2`.

##### Scenario: Config root command displays usage and help
- GIVEN the user executes `agis config --help` or `agis config` without arguments
- WHEN the command is evaluated
- THEN the system outputs usage syntax, available subcommands (`show`, `get`, `set`, `path`), and flag documentation to `stdout` and exits with code 0

##### Scenario: Unrecognized subcommand returns usage error
- GIVEN the user executes `agis config invalid_cmd`
- WHEN the command router evaluates the arguments
- THEN the system writes `"agis config: unknown subcommand 'invalid_cmd'"` and usage instructions to `stderr` and exits with code 2

---

#### Requirement CLI-CFG-002: Config Show Subcommand (`show`)
The `agis config show` subcommand MUST load and display the active configuration in YAML or JSON format.
- Flags:
  - `-config <path>`: Path to a custom configuration file.
  - `-json`: Outputs the configuration as formatted JSON to `stdout`.
  - `-reveal`: Exposes raw secret values without masking. Default is `false` (secrets masked).
- By default (when `-reveal` is false), the system MUST mask all sensitive credential fields with `"[MASKED]"` or `"********"`. Sensitive fields MUST include:
  - `llm.api_key`
  - `gateway.telegram.token`
  - `gateway.discord.token`
  - `webhook.secret`
- When `-reveal` is provided, all sensitive credential fields MUST be printed in plaintext.
- When `-json` is provided, the output MUST be valid JSON formatted with two-space indentation.
- When `-json` is omitted, the output MUST be formatted YAML.
- If the configuration file does not exist on disk, `show` MUST display the effective built-in default configuration and exit with code 0.
- If the configuration file exists but is corrupted or contains invalid YAML syntax, the command MUST output an error to `stderr` and exit with code 1.

##### Scenario: Show configuration with default secret masking in YAML
- GIVEN a configuration file containing `llm.api_key: "sk-secret-12345"` and `llm.provider: "openai"`
- WHEN the user executes `agis config show`
- THEN the output to `stdout` contains `provider: openai` and `api_key: "[MASKED]"` (or `"********"`), sensitive values are hidden, and exit code is 0

##### Scenario: Show configuration with --reveal flag
- GIVEN a configuration file containing `llm.api_key: "sk-secret-12345"`
- WHEN the user executes `agis config show --reveal`
- THEN the output to `stdout` contains `api_key: "sk-secret-12345"` in plaintext and exit code is 0

##### Scenario: Show configuration in JSON format
- GIVEN a valid active configuration
- WHEN the user executes `agis config show --json`
- THEN the output to `stdout` is a valid JSON object representation of the configuration and exit code is 0

##### Scenario: Show with non-existent config file
- GIVEN no configuration file exists at the default path
- WHEN the user executes `agis config show`
- THEN the system outputs the default configuration to `stdout` and exits with code 0

##### Scenario: Show with corrupted config file
- GIVEN a configuration file with invalid YAML syntax
- WHEN the user executes `agis config show`
- THEN the system writes an error to `stderr` and exits with code 1

---

#### Requirement CLI-CFG-003: Config Get Subcommand (`get`)
The `agis config get <key>` subcommand MUST retrieve and output the value of a specific configuration key using dot notation.
- Arguments: Exactly one positional argument `<key>` representing the dot-separated field path (e.g. `llm.provider`, `db.path`, `memory.recall_limit`, `tools.docker.image`, `webhook.port`).
- Missing or extra positional arguments MUST output a usage error to `stderr` and exit with code 2.
- Flags:
  - `-config <path>`: Path to a custom configuration file.
  - `-reveal`: Exposes raw secret value if the queried key is sensitive. Default is `false`.
  - `-json`: Serializes the retrieved value as JSON.
- Secret Masking:
  - If the requested key is a sensitive field (`llm.api_key`, `gateway.telegram.token`, `gateway.discord.token`, `webhook.secret`), the output MUST be masked (`"[MASKED]"`) unless `-reveal` is explicitly supplied.
- Key Resolution:
  - Dot notation keys MUST map to corresponding configuration struct fields in a case-insensitive manner for common path segments (e.g., `llm.provider`, `LLM.Provider`, `llm.model`).
  - If the key resolves to a scalar value (string, int, bool, float, duration), the system MUST print the string representation to `stdout` followed by a newline.
  - If the key resolves to a complex type (slice or map/struct), the system MUST output the YAML or JSON representation to `stdout`.
  - If the specified key does not exist in the configuration schema, the system MUST write an error `"agis config: unknown configuration key '<key>'"` to `stderr` and exit with code 1.

##### Scenario: Get existing scalar configuration key
- GIVEN an active configuration with `llm.model` set to `"llama3.2"`
- WHEN the user executes `agis config get llm.model`
- THEN the system writes `"llama3.2\n"` to `stdout` and exits with code 0

##### Scenario: Get sensitive key without --reveal
- GIVEN an active configuration with `llm.api_key` set to `"sk-abcdef"`
- WHEN the user executes `agis config get llm.api_key`
- THEN the system writes `"[MASKED]\n"` (or `"********\n"`) to `stdout` and exits with code 0

##### Scenario: Get sensitive key with --reveal
- GIVEN an active configuration with `llm.api_key` set to `"sk-abcdef"`
- WHEN the user executes `agis config get llm.api_key --reveal`
- THEN the system writes `"sk-abcdef\n"` to `stdout` and exits with code 0

##### Scenario: Get non-existent key
- GIVEN an active configuration
- WHEN the user executes `agis config get invalid.nested.key`
- THEN the system writes an error to `stderr` indicating the key was not found and exits with code 1

##### Scenario: Get without required key argument
- GIVEN no key argument is passed
- WHEN the user executes `agis config get`
- THEN the system writes usage syntax to `stderr` and exits with code 2

---

#### Requirement CLI-CFG-004: Config Set Subcommand (`set`)
The `agis config set <key> <value>` subcommand MUST validate, update, and persist a configuration key to disk with atomic write guarantees and strict `0600` file permissions.
- Arguments: Exactly two positional arguments: `<key>` (dot notation path) and `<value>` (string representation of target value).
- Missing or extra positional arguments MUST output a usage error to `stderr` and exit with code 2.
- Flags:
  - `-config <path>`: Path to custom configuration file.
- Type Validation:
  - The system MUST inspect the expected type of the target key:
    - **Boolean** (`bool`): MUST accept `"true"`, `"false"`, `"1"`, `"0"`, `"t"`, `"f"`, `"TRUE"`, `"FALSE"`. Invalid values MUST be rejected.
    - **Integer** (`int`): MUST parse as valid decimal integer. If the target field has bounds (e.g. `recall_limit > 0`, `port > 0`), values violating bounds MUST be rejected.
    - **Duration** (`time.Duration`): MUST parse via `time.ParseDuration` (e.g. `"30s"`, `"5m"`, `"1h"`). Invalid durations MUST be rejected.
    - **String** (`string`): MUST accept any valid string.
    - **String Slice** (`[]string`): MUST accept comma-separated strings or valid JSON arrays (e.g. `"admin,ops"` or `["admin", "ops"]`).
  - If validation fails, the system MUST write a descriptive error to `stderr` and exit with code 1 without modifying the configuration file.
- Persistence and Security:
  - If the target file or directory does not exist, the system MUST create parent directories with mode `0700` and the file with mode `0600`.
  - Atomic File Replacement: The system MUST write serialized YAML data to a temporary file in the same directory (e.g. `.config.yaml.tmp.<pid>`), sync the file descriptor to disk, set permissions to `0600` (`-rw-------`), and atomically rename (`os.Rename`) the temporary file over the target config file.
  - File Mode Enforcement: The resulting config file MUST have file mode `0600` (`-rw-------`).
- Confirmation:
  - On successful write, the system MUST write a confirmation notice (e.g., `"Updated 'llm.model' to 'llama3.3'"`) to `stdout` and exit with code 0.

##### Scenario: Set string configuration value successfully
- GIVEN a valid configuration file
- WHEN the user executes `agis config set llm.model llama3.3`
- THEN the file is updated atomically with `llm.model: llama3.3`, permissions are `0600`, confirmation is printed to `stdout`, and exit code is 0

##### Scenario: Set boolean value with type validation
- GIVEN a configuration file with `skills.enabled: true`
- WHEN the user executes `agis config set skills.enabled false`
- THEN the file is updated with `skills.enabled: false` and exit code is 0

##### Scenario: Set boolean value with invalid string
- GIVEN a configuration file
- WHEN the user executes `agis config set skills.enabled not_a_boolean`
- THEN the system writes `"agis config: invalid boolean value 'not_a_boolean' for key 'skills.enabled'"` to `stderr`, file remains untouched, and exit code is 1

##### Scenario: Set integer value with invalid string
- GIVEN a configuration file
- WHEN the user executes `agis config set webhook.port abc`
- THEN the system writes `"agis config: invalid integer value 'abc' for key 'webhook.port'"` to `stderr`, file remains untouched, and exit code is 1

##### Scenario: Set duration value with valid format
- GIVEN a configuration file
- WHEN the user executes `agis config set memory.close_timeout 45s`
- THEN the file is updated with `memory.close_timeout: 45s` and exit code is 0

##### Scenario: Set duration value with invalid format
- GIVEN a configuration file
- WHEN the user executes `agis config set memory.close_timeout 45lightyears`
- THEN the system writes an error to `stderr`, file remains untouched, and exit code is 1

##### Scenario: Set unknown configuration key
- GIVEN a configuration file
- WHEN the user executes `agis config set unknown.nonexistent.field value`
- THEN the system writes `"agis config: unknown configuration key 'unknown.nonexistent.field'"` to `stderr`, file is unchanged, and exit code is 1

##### Scenario: Set creates config file and directory if missing
- GIVEN neither `~/.agis` directory nor `config.yaml` exists
- WHEN the user executes `agis config set llm.provider openai`
- THEN `~/.agis` is created with mode `0700`, `config.yaml` is created with mode `0600` containing default configuration plus `llm.provider: openai`, and exit code is 0

---

#### Requirement CLI-CFG-005: Config Path Subcommand (`path`)
The `agis config path` subcommand MUST print the absolute or resolved filesystem path of the active configuration file.
- Flags:
  - `-config <path>`: Custom configuration path override.
- Precedence: The resolved path MUST strictly observe the documented precedence:
  1. `-config` flag value (if supplied)
  2. `AGIS_HOME/config.yaml` (if `AGIS_HOME` environment variable is set)
  3. `~/.agis/config.yaml` (default user home directory)
- Output: The resolved path MUST be written to `stdout` with a trailing newline.
- Exit code: The command MUST exit with code 0.

##### Scenario: Print default configuration path
- GIVEN no `-config` flag and `AGIS_HOME` is unset
- WHEN the user executes `agis config path`
- THEN the system writes the full path `~/.agis/config.yaml` (expanded) to `stdout` and exits with code 0

##### Scenario: Print custom configuration path via flag
- GIVEN the user executes `agis config path -config /custom/agis/config.yaml`
- WHEN the path is resolved
- THEN the system writes `"/custom/agis/config.yaml\n"` to `stdout` and exits with code 0

##### Scenario: Print configuration path using AGIS_HOME
- GIVEN environment variable `AGIS_HOME=/opt/agis` is set and `-config` flag is omitted
- WHEN the user executes `agis config path`
- THEN the system writes `"/opt/agis/config.yaml\n"` to `stdout` and exits with code 0

---

#### Requirement CLI-CFG-006: POSIX Exit Codes and Stream Discipline
All `agis config` subcommands MUST adhere strictly to POSIX exit code conventions and stdout/stderr separation.
- Exit Codes:
  - `0`: Success (operation completed normally, query fulfilled, help displayed).
  - `1`: Operational or domain failure (key not found, validation error, file I/O error, corrupt YAML).
  - `2`: Usage or flag error (unknown flag, invalid flag argument, missing positional argument, unrecognized subcommand).
- Stream Separation:
  - `stdout`: Pure data streams, structured output (YAML/JSON), retrieved key values, paths, and successful confirmation notices.
  - `stderr`: Error messages, diagnostics, usage messages on syntax errors, and permission warnings. Under no circumstances SHALL logs or errors be written to `stdout`.

##### Scenario: Stream separation on valid query
- GIVEN `agis config get llm.model` is run in a piped shell command
- WHEN the command executes
- THEN only the value string is emitted on `stdout`, allowing clean pipeline redirection (`echo $(agis config get llm.model)`)

##### Scenario: Stream separation on error
- GIVEN `agis config get invalid.key` is executed
- WHEN the command fails
- THEN nothing is emitted on `stdout`, error details are emitted on `stderr`, and exit code is 1

---

### Package `internal/config` Contracts

#### Requirement CFG-PKG-001: Configuration Accessor, Mutation, and Serialization APIs
The `internal/config` package MUST expose APIs supporting dot-notation traversal, type-validated updates, atomic persistence, and secret masking.
1. **Path Resolution**: `ResolvePath(flagPath string) string` MUST return the effective config file path considering flag, environment, and user home.
2. **Dot-Notation Getter**: `Get(cfg *Config, key string) (any, error)` MUST resolve dot-separated key paths against `Config` fields. If the key is not recognized, it MUST return a descriptive error.
3. **Dot-Notation Setter**: `Set(cfg *Config, key string, valStr string) error` MUST parse and validate `valStr` according to the target field's type and update the in-memory struct. Invalid types, unknown keys, or out-of-range values MUST return an error without modifying the struct.
4. **Atomic Save**: `Save(path string, cfg *Config) error` MUST serialize `Config` to YAML and atomically write to `path` using a temporary file with mode `0600` (`-rw-------`). If the destination directory does not exist, it MUST be created with mode `0700`.
5. **Secret Masking**: `MaskSecrets(cfg *Config) *Config` (or `cfg.Sanitized() *Config`) MUST return a clone of `Config` where `LLM.APIKey`, `Gateway.Telegram.Token`, `Gateway.Discord.Token`, and `Webhook.Secret` (and any future credential fields) are replaced with `"[MASKED]"`.

##### Scenario: In-memory dot-notation getter resolves nested struct fields
- GIVEN a `Config` struct with `Tools.Docker.Image = "alpine:3"`
- WHEN `config.Get(cfg, "tools.docker.image")` is invoked
- THEN it returns string `"alpine:3"` and `nil` error

##### Scenario: In-memory dot-notation setter validates and updates integer
- GIVEN a `Config` struct with `Memory.RecallLimit = 10`
- WHEN `config.Set(cfg, "memory.recall_limit", "25")` is invoked
- THEN `cfg.Memory.RecallLimit` is updated to `25` and error is `nil`

##### Scenario: In-memory dot-notation setter rejects invalid type
- GIVEN a `Config` struct with `Agent.EvolutionEnabled = true`
- WHEN `config.Set(cfg, "agent.evolution_enabled", "invalid")` is invoked
- THEN `cfg.Agent.EvolutionEnabled` remains `true` and a type validation error is returned

##### Scenario: Save persists file with 0600 mode atomically
- GIVEN a modified `Config` instance and target path `/tmp/test-agis/config.yaml`
- WHEN `config.Save(path, cfg)` is called
- THEN the file is written to `/tmp/test-agis/config.yaml`, file permissions are exactly `0600`, and content is valid YAML matching the struct

##### Scenario: MaskSecrets protects credential fields
- GIVEN a `Config` instance with `LLM.APIKey = "secret-token"`, `Gateway.Telegram.Token = "tg-token"`, and `Webhook.Secret = "wh-secret"`
- WHEN `sanitized := config.MaskSecrets(cfg)` is called
- THEN `sanitized.LLM.APIKey`, `sanitized.Gateway.Telegram.Token`, and `sanitized.Webhook.Secret` all equal `"[MASKED]"` while non-secret fields retain their original values

---
