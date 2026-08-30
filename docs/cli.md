# AGIS CLI Reference

This document provides a comprehensive reference for all command-line interface (CLI) subcommands, flags, and daemon runners provided by the `agis` binary.

---

## Overview of Commands

```bash
agis [flags]                               # Launch interactive TUI (default)
agis gateway [run] [flags]                 # Chat Gateway Multiplexer daemon (Telegram & Discord)
agis cron [run|list] [flags]               # Autonomous Cron Scheduler daemon & job inspector
agis plugins [list|enable|disable|inspect] # External Plugin Manager & tool bridge
agis webhook [run] [flags]                 # Secure HTTP Webhook event listener daemon
agis policy [init|show|set|rm|tier|test]   # Policy Guard security & permissions manager
agis mcp [list|test] [flags]               # Model Context Protocol (MCP) server & tool inspection
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

## Exit Codes

All `agis` CLI commands follow standard POSIX exit codes:
- **`0`**: Successful execution or clean graceful shutdown on `SIGINT`/`SIGTERM`.
- **`1`**: Runtime error, invalid configuration, missing required flags, or fatal initialization failure.
