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

