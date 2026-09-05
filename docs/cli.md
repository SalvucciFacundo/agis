# AGIS CLI Reference

This document provides a comprehensive reference for all command-line interface (CLI) subcommands, flags, and daemon runners provided by the `agis` binary.

---

## Overview of Commands

```bash
agis [flags]                               # Launch interactive TUI (default)
agis doctor [flags]                        # System diagnostics & environment health probe
agis gateway [run] [flags]                 # Chat Gateway Multiplexer daemon (Telegram & Discord)
agis cron [run|list] [flags]               # Autonomous Cron Scheduler daemon & job inspector
agis plugins [list|enable|disable|inspect] # External Plugin Manager & tool bridge
agis webhook [run] [flags]                 # Secure HTTP Webhook event listener daemon
agis policy [init|show|set|rm|tier|test]   # Policy Guard security & permissions manager
agis mcp [list|test] [flags]               # Model Context Protocol (MCP) server & tool inspection
agis session [list|show|delete|rename|export|snapshot] # Conversation session manager, export & backups
agis update [flags]                        # In-place self-updater & release inspector
agis config [show|get|set|path] [flags]    # Inspect, query, and safely modify configuration
```

---

## Global Flags & Environment

Every `agis` command accepts the global configuration flag:

| Flag | Type | Description | Default |
|---|---|---|---|
| `-config` | `string` | Explicit path to `config.yaml` file | `$AGIS_HOME/config.yaml` or `~/.agis/config.yaml` |

### Environment Variables
- `AGIS_HOME`: Root directory for AGIS runtime files, database (`agis.db`), policy rules (`policy.yaml`), identity (`SOUL.md`), plugins (`plugins/`), and skills (`skills/`). Defaults to `~/.agis`.

---

## 1. Interactive Terminal (TUI)

```bash
# Launch interactive TUI with default configuration
agis

# Launch interactive TUI with a custom configuration file
agis -config /path/to/custom-config.yaml
```

The default command launches the Bubbletea-based terminal user interface with streaming token display, Markdown rendering, conversation history, and in-session slash commands (see [docs/tui-commands.md](tui-commands.md)).

---

## 2. Chat Gateway Daemon (`agis gateway`)

Runs the multi-surface chat gateway multiplexer, connecting AGIS concurrently to configured chat platforms (Telegram, Discord) with static user allowlists and non-interactive sandbox policy enforcement.

```bash
# Start the chat gateway daemon
agis gateway run [--config config.yaml]

# Default alias for 'run'
agis gateway
```

### Behavior & Lifecycle
- Listens for `SIGINT` (Ctrl+C) and `SIGTERM` for graceful teardown.
- Translates inbound platform messages into internal `MessageEvent` structs.
- Routes messages through `session.SessionManager` conversations (e.g. `gateway:telegram:<chatID>`).
- Rejects unauthorized senders immediately (fail-closed static allowlist).
- Auto-denies tool calls requiring interactive confirmation (`AutoDenyApprover`).

---

## 3. Cron Scheduler Daemon (`agis cron`)

Manages background scheduled jobs configured in `config.yaml`, executing autonomous prompts via `Brain.Step` and delivering responses to Telegram/Discord notification targets.

```bash
# List all configured cron jobs and their next run schedules
agis cron list [--config config.yaml]

# Start the background cron scheduler daemon
agis cron run [--config config.yaml]
```

### Example Output (`agis cron list`):
```text
Configured Cron Jobs (2):
- Name:     daily-health
  Schedule: @every 1h
  Prompt:   Check system health and pending tasks
  Session:  cron:daily-health (ephemeral)
  Target:   telegram -> 123456789

- Name:     backup-sync
  Schedule: 0 3 * * *
  Prompt:   Run database backup verification
  Session:  backup-session
  Target:   discord -> channel-alerts
```

---

## 4. Plugin Manager (`agis plugins`)

Discovers, inspects, and manages external tool and skill bundles located in `$AGIS_HOME/plugins/`.

```bash
# List all discovered plugins, enabled status, and versions
agis plugins list [--dir ~/.agis/plugins]

# Inspect detailed manifest information, tools, and permissions for a plugin
agis plugins inspect <plugin_name> [--dir ~/.agis/plugins]

# Enable an installed plugin
agis plugins enable <plugin_name> [--dir ~/.agis/plugins]

# Disable an installed plugin
agis plugins disable <plugin_name> [--dir ~/.agis/plugins]
```

### Example Manifest Inspection:
```text
Plugin: weather
  Version:     1.0.0
  Description: Real-time weather reports and forecast tools
  Status:      ENABLED
  Entrypoint:  bin/weather-cli
  Tools (1):
    - get_weather: Fetch current weather for a city
  Skills (1):
    - weather-forecast.md
  Permissions (1):
    - network:outbound
```

---

## 5. Webhook HTTP Listener Daemon (`agis webhook`)

Starts the secure HTTP event listener for ingesting third-party webhooks with constant-time HMAC-SHA256 signature verification.

```bash
# Start webhook server on default host/port (127.0.0.1:8080/webhook)
agis webhook run

# Start webhook server with custom network bindings
agis webhook run --host 0.0.0.0 --port 9090 --path /events --config config.yaml
```

### Flags
| Flag | Type | Description | Default |
|---|---|---|---|
| `--host` | `string` | IP address to bind HTTP listener | `127.0.0.1` |
| `--port` | `int` | Port number to bind HTTP listener | `8080` |
| `--path` | `string` | HTTP POST endpoint path | `/webhook` |
| `--config` | `string` | Configuration file path | `~/.agis/config.yaml` |

---

## 6. Policy Guard CLI (`agis policy`)

Manages AGIS's multi-tier security posture and exact-subject allow/deny rules stored in `$AGIS_HOME/policy.yaml`.

```bash
# Initialize a secure policy file with sandbox defaults
agis policy init

# View the active policy posture, defined rules, and audit log tail
agis policy show

# Add a persistent allow or deny rule (optionally scoped to a specific backend)
agis policy set "git pull" allow
agis policy set "rm -rf /" deny
agis policy set -b docker "apt-get update" allow
agis policy set -b ssh "systemctl status" allow

# Remove an existing persistent rule
agis policy rm "git pull"

# Change the baseline policy tier (sandbox | standard)
agis policy tier sandbox
agis policy tier standard

# Test how a command would be evaluated without executing it
agis policy test "docker run alpine sh"
agis policy test -b ssh "cat /etc/passwd"
```

### Policy Tiers:
- **`sandbox`**: Read-only, safe commands only. Shell executions requiring root or filesystem mutation require confirmation (`ask`) or are auto-denied in background daemons.
- **`standard`**: Developer mode with standard file and shell access. High-risk actions (privilege escalation, system destruction) require confirmation.
- **`full`**: Unrestricted execution. Allowed as an in-session override only; `agis policy tier full` is rejected by default to prevent permanent privilege escalation.

---

## 7. Model Context Protocol CLI (`agis mcp`)

Discovers, inspects, and directly executes tools on configured Model Context Protocol (MCP) servers (see [docs/mcp.md](mcp.md)).

```bash
# List all configured MCP servers, connectivity status, and discovered tools
agis mcp list [--config config.yaml]

# Directly test/invoke an MCP tool without LLM orchestration
agis mcp test <server> <tool> [json_args] [--config config.yaml]
```

### Examples:
```bash
# List discovered tools
agis mcp list

# Test tool execution with JSON arguments
agis mcp test filesystem read_file '{"path": "/tmp/test.txt"}'
agis mcp test sqlite query '{"sql": "SELECT COUNT(*) FROM users;"}'
```

---

## 8. System Diagnostics (`agis doctor`)

Runs an end-to-end diagnostic suite checking environment configuration, SQLite storage health, `SOUL.md` persona status, skill registrations, Policy Guard integrity, LLM/embeddings provider connectivity, and tool backends.

```bash
# Run interactive diagnostic report
agis doctor

# Run doctor with a custom configuration file
agis doctor -config /path/to/custom-config.yaml

# Output diagnostic report as JSON (for CI/CD or automation)
agis doctor -json

# Disable ANSI color formatting
agis doctor -no-color
```

### Verified Diagnostic Probes:
1. **Configuration & Environment**: Validates `$AGIS_HOME`, `config.yaml` existence, and file permissions (`0600`).
2. **SQLite & Persistent Memory**: Checks database file connectivity, PRAGMA integrity, migration schema versions (`0001` through `0007`), and total record counts.
3. **Agent Identity (`SOUL.md`)**: Verifies presence and readability of the persistent agent persona.
4. **Skill Hub & Registry**: Scans `$AGIS_HOME/skills/` and validates frontmatter metadata on all Markdown skills.
5. **Policy Guard & Permissions**: Validates `policy.yaml` parsing and active security postures across local, docker, and ssh tiers.
6. **LLM Provider Connectivity**: Tests live connectivity to Ollama (`/api/tags`), OpenAI (`/v1/models`), or OpenRouter endpoints, verifying model availability.
7. **Vector Embeddings & Hybrid Search**: Checks hybrid search configuration and embedding model parameters when enabled.
8. **Model Context Protocol (MCP)**: Validates command paths for `stdio` subprocesses and URL syntax for `sse` transports.
9. **Execution Backends & System Tools**: Validates local shell (`sh`), `docker` CLI, and `ssh` client availability.

---

## 9. Session Management CLI (`agis session`)

Provides headless inspection, management, deletion, export, and point-in-time snapshotting of conversation sessions without launching the interactive TUI.

```bash
# List recent conversation sessions (table format)
agis session list [--limit 20] [--json]

# Show detailed metadata and complete message history of a session
agis session show <id> [--json]

# Delete a session and its cascaded records (messages, snapshots, attachments)
agis session delete <id> [--yes]

# Rename a session title (with automatic prompt injection sanitization)
agis session rename <id> "<new_title>"

# Export session message history to Markdown, JSON, or Plaintext
agis session export <id> [--format markdown|json|txt] [--output /path/to/file]

# Capture a point-in-time snapshot of a conversation
agis session snapshot <id> [--json]
```

### Subcommands & Flags:

#### `agis session list`
- `-limit <N>`: Maximum number of sessions to return (integer > 0, default: `20`).
- `-json`: Outputs list of sessions as a JSON array to `stdout`.
- `-config <path>`: Custom configuration file path.

#### `agis session show <id>`
- Positional: Session UUID `<id>`.
- `-json`: Outputs conversation metadata and full message array as JSON to `stdout`.
- `-config <path>`: Custom configuration file path.

#### `agis session delete <id>`
- Positional: Session UUID `<id>`.
- `-yes`, `-y`: Skip confirmation prompt (required in non-interactive / automated environments).
- `-config <path>`: Custom configuration file path.

#### `agis session rename <id> <title>`
- Positional: Session UUID `<id>` and new title `<title>`.
- Automatically sanitizes prompt injection markers via `scan.Lines`.
- `-config <path>`: Custom configuration file path.

#### `agis session export <id>`
- Positional: Session UUID `<id>`.
- `-format <json|markdown|txt>`: Output format (default: `markdown`).
- `-output <path>`: Destination file path. If omitted, outputs directly to `stdout`.
- `-config <path>`: Custom configuration file path.

#### `agis session snapshot <id>`
- Positional: Session UUID `<id>`.
- `-json`: Outputs snapshot metadata as JSON to `stdout`.
- `-config <path>`: Custom configuration file path.

---

## 10. Self-Updater (`agis update`)

Provides in-place binary self-updating from GitHub Releases with SHA-256 checksum verification, state backup snapshots, version inspection, and atomic executable replacement across Linux, macOS, and Windows.

```bash
# Check for available updates without modifying the binary
agis update --check

# Update to latest GitHub release (with automated $AGIS_HOME state backup)
agis update --backup

# Force update or re-install to a specific release tag
agis update --version v0.4.0 --force
```

### Subcommand Flags:

- `-check`: Check for available updates and compare against local binary without modifying the filesystem.
- `-backup`: Archive critical `$AGIS_HOME` state files (`agis.db`, `config.yaml`, `policy.yaml`, `SOUL.md`, `skills/`, `plugins/`) into `$AGIS_HOME/backups/agis-backup-<timestamp>.tar.gz` before updating.
- `-version <tag>`: Target a specific release tag (e.g. `v0.4.0`) rather than the latest release, allowing upgrades and downgrades.
- `-force`: Force re-download, verification, and replacement even if already up to date.
- `-config <path>`: Custom configuration file path (to resolve `$AGIS_HOME`).
- `-h`, `--help`: Show usage instructions.

---

## 11. Configuration Management (`agis config`)

Inspect, read, and safely modify AGIS configuration without manually editing YAML files. Enforces atomic file writes, parent directory creation (`0700`), strict file mode (`0600`), type validation, and credential masking.

```bash
# Display active configuration with masked secrets (YAML or JSON)
agis config show
agis config show -json
agis config show -reveal

# Query specific configuration key using dot notation
agis config get llm.model
agis config get memory.recall_limit
agis config get llm.api_key -reveal

# Update and persist a configuration key with strict type validation
agis config set llm.model llama3.3
agis config set agent.evolution_enabled false
agis config set memory.close_timeout 45s
agis config set gateway.telegram.allowlist "admin,ops,user"

# Print resolved active configuration file path
agis config path
```

### Subcommands & Flags:

#### `agis config show`
- `-config <path>`: Custom configuration file path.
- `-json`: Outputs configuration as formatted JSON with 2-space indentation.
- `-reveal`: Displays sensitive credentials (`llm.api_key`, `gateway.telegram.token`, `gateway.discord.token`, `webhook.secret`) in plaintext rather than `"[MASKED]"`.

#### `agis config get <key>`
- Queries a scalar or complex configuration value using dot notation.
- `-config <path>`: Custom configuration file path.
- `-reveal`: Exposes plaintext secret value if querying a sensitive key.
- `-json`: Serializes value as JSON.

#### `agis config set <key> <value>`
- Validates and updates the configuration key, saving atomically to disk with strict `0600` permissions.
- Accepts `bool` (`true`/`false`/`1`/`0`), `int`, `time.Duration` (`30s`, `1m`), `string`, and `[]string` (comma-separated or JSON array).
- `-config <path>`: Custom configuration file path.

#### `agis config path`
- Outputs the resolved configuration file path according to precedence (`-config` flag > `AGIS_HOME` env > `~/.agis/config.yaml`).
- `-config <path>`: Custom configuration file path.

---

## Exit Codes

All `agis` CLI commands follow standard POSIX exit codes:
- **`0`**: Successful execution (update applied, binary up to date, check completed, or clean graceful shutdown on `SIGINT`/`SIGTERM`).
- **`1`**: Runtime error, invalid configuration, network failure, checksum mismatch, backup failure, or fatal initialization failure.
- **`2`**: Command-line usage error (unrecognized flag or unexpected positional arguments).
