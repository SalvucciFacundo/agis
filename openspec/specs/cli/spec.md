# CLI Commands Spec

## Purpose

Define the command-line interface subcommands exposed by AGIS via Cobra, covering daemon run commands, management tools, and headless automation.

## Requirements

### Requirement AGIS-M6-CLI-002: Ecosystem CLI Subcommands (`gateway`, `cron`, `plugins`, `webhook`)
The `cmd/agis/` CLI entry points MUST provide the following subcommands:
1. `agis gateway [run]`: Starts the Gateway multiplexer daemon with enabled chat adapters. Accepts `--config` flag. Listens for `SIGINT`/`SIGTERM` for graceful shutdown.
2. `agis cron [run|list]`:
   - `agis cron run`: Starts the cron scheduler in the background.
   - `agis cron list`: Prints all configured cron jobs with their schedule and target.
3. `agis plugins [list|enable|disable|inspect]`:
   - `agis plugins list`: Lists all discovered plugins, enabled status, and versions.
   - `agis plugins enable <name>`: Enables the specified plugin.
   - `agis plugins disable <name>`: Disables the specified plugin.
   - `agis plugins inspect <name>`: Displays manifest details, declared tools, and permissions.
4. `agis webhook [run]`: Starts the Webhook HTTP server listener daemon. Accepts `--port`, `--host`, `--path`, and `--config` flags.

All daemon subcommands MUST exit with code 0 on clean shutdown via signal and non-zero on fatal initialization failure.

#### Scenario: `agis gateway` runs and terminates on SIGINT
- GIVEN valid gateway configuration
- WHEN `agis gateway` is launched and sent `SIGINT`
- THEN the gateway shuts down all adapters cleanly and exits with status code 0

#### Scenario: `agis plugins list` displays plugin statuses
- GIVEN two plugins in the plugins directory (`"weather"` enabled, `"jira"` disabled)
- WHEN `agis plugins list` is executed
- THEN output lists both plugins with their correct names, versions, and enabled statuses

#### Scenario: `agis webhook` starts listener on custom port
- GIVEN flag `--port 9090`
- WHEN `agis webhook` runs
- THEN the server binds to `127.0.0.1:9090` and handles HTTP requests


cli (MODIFIED)

### Requirement AGIS-M8-CLI-001: MCP CLI Subcommands (`mcp list`, `mcp test`)
The `cmd/agis/` CLI entry points MUST provide the following subcommands:
1. `agis mcp list`: Initializes configured MCP servers, establishes connections, queries `tools/list`, and prints an aggregated table of servers, statuses, and discovered tools.
2. `agis mcp test <server_name> <tool_name> [json_arguments]`: Connects directly to the specified MCP server, executes `tools/call` with the provided JSON arguments, and prints the raw tool output without LLM orchestration.

#### Scenario: `agis mcp list` displays active servers and tools
- GIVEN a configured stdio MCP server `"postgres"` with tool `"query"`
- WHEN `agis mcp list` is executed
- THEN output lists server `"postgres"`, status `CONNECTED`, and tool `"query"`

#### Scenario: `agis mcp test` executes tool directly
- GIVEN a running MCP server `"postgres"`
- WHEN `agis mcp test postgres query '{"sql": "SELECT 1"}'` runs
- THEN the tool output is printed to stdout and exit code is 0

### Requirement CLI-SESS-001: Session Lifecycle Management Subcommands (`session`)
The `cmd/agis/` CLI entry points MUST provide the `agis session` subcommand router with:
1. `agis session list [--limit N] [--json]`: Lists recent conversations in tabular or JSON format.
2. `agis session show <id> [--json]`: Displays conversation details and message turns.
3. `agis session delete <id> [--yes]`: Deletes a conversation with cascading removal and interactive/non-interactive confirmation guards.
4. `agis session rename <id> <title>`: Renames a conversation with prompt injection filtering.
5. `agis session export <id> [--format json|markdown|txt] [--output file]`: Exports session history.
6. `agis session snapshot <id>`: Captures point-in-time snapshot.

#### Scenario: `agis session list` displays tabular sessions
- GIVEN conversations in SQLite DB
- WHEN `agis session list` is executed
- THEN output prints formatted table of sessions ordered by update time with exit code 0

#### Scenario: `agis session delete` with non-interactive guard
- GIVEN an unattached terminal or piped input without `-yes`
- WHEN `agis session delete conv-1` runs
- THEN it rejects execution with error on stderr and exit code 1

### Requirement CLI-UPD-001: Self-Updater Subcommand (`update`)
The `cmd/agis/` CLI entry points MUST provide the `agis update` subcommand for self-updating:
1. `agis update --check`: Queries GitHub Releases API, compares version against local `internal/version`, and reports availability without modifying the executable.
2. `agis update [--backup] [--version <tag>] [--force]`: Downloads target release asset, validates SHA-256 against `checksums.txt`, backs up `$AGIS_HOME` state to `.tar.gz` (if `--backup`), and performs atomic in-place executable replacement.

#### Scenario: `agis update --check` discovers newer version
- GIVEN current binary at `v1.3.0` and latest release at `v1.4.0`
- WHEN `agis update --check` runs
- THEN it writes update availability notice to stdout and exits with code 0

#### Scenario: `agis update --backup` archives state
- GIVEN active `$AGIS_HOME` with database and configuration
- WHEN `agis update --backup` runs
- THEN it creates a timestamped archive under `$AGIS_HOME/backups/` before binary swap

### Requirement CLI-CFG-001: Configuration Management Subcommands (`config`)
The `cmd/agis/` CLI entry points MUST provide the `agis config` subcommand router:
1. `agis config show [--json] [--reveal]`: Displays active configuration in YAML or JSON, masking sensitive credentials (`llm.api_key`, tokens, secrets) unless `--reveal` is passed.
2. `agis config get <key> [--json] [--reveal]`: Retrieves specific configuration value using dot notation (e.g., `llm.provider`, `llm.model`, `embeddings.enabled`).
3. `agis config set <key> <value>`: Updates and atomically persists a configuration key with strict type validation (bool, int, duration, string, string slice) and `0600` file permissions.
4. `agis config path`: Prints the resolved configuration file path.

#### Scenario: `agis config show` masks secrets by default
- GIVEN configuration with API keys and secrets
- WHEN `agis config show` runs without `-reveal`
- THEN it outputs YAML with sensitive fields masked as `"[MASKED]"`

#### Scenario: `agis config set` updates typed configuration atomically
- GIVEN configuration key `embeddings.enabled`
- WHEN `agis config set embeddings.enabled true` runs
- THEN it persists boolean `true` with `0600` file permissions and exit code 0

