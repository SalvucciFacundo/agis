# Model Context Protocol (MCP) Client Guide

AGIS includes native, pure Go support for the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) specification (protocol version `2024-11-05`). This enables AGIS to seamlessly discover and execute tools exposed by external MCP servers across local subprocesses (`stdio`) and networked servers (`sse`).

All MCP tools are dynamically mapped to AGIS's `core.ToolRunner` port and strictly governed by `PolicyGuard`.

---

## 1. Overview & Architecture

MCP client support is implemented across the following packages:

| Package | Responsibility |
|---|---|
| `internal/mcp` | JSON-RPC 2.0 wire models, lifecycle handshakes (`initialize` -> `notifications/initialized`), request/response multiplexing, paginated tool discovery (`tools/list`), tool execution (`tools/call`), and multi-server `Manager`. |
| `internal/mcp/transport` | Stream transports: `stdio` (local subprocess lifecycle with process group cleanup) and `sse` (Server-Sent Events HTTP event streams). |
| `internal/tools` | `MCPRunner` bridge implementing `core.ToolRunner`, mapping MCP tool calls to `mcp:<server_name>` backends for Policy Guard evaluation. |
| `cmd/agis` | CLI subcommands: `agis mcp list` and `agis mcp test <server> <tool> [args]`. |

### Tool Execution Flow

```
┌────────────────────────────────────────────────────────────────────────┐
│                              core.Brain                                │
│                                                                        │
│  1. LLM requests MCP tool (e.g. mcp_filesystem_read_file)              │
│  2. PolicyGuard evaluates GuardRequest{Backend: "mcp:filesystem",      │
│                                       Subject: "read_file"}            │
│  3. Decision: Allow -> Proceed | Ask -> Confirm/Auto-Deny | Deny       │
│  4. MCPRunner executes CallTool via mcp.Manager -> Transport           │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ JSON-RPC 2.0
                        ┌───────────┴───────────┐
                        ▼                       ▼
            ┌───────────────────────┐ ┌───────────────────┐
            │ Stdio Transport (pipe)│ │ SSE Transport     │
            │  subprocess (npx/bin) │ │  HTTP GET / POST  │
            └───────────────────────┘ └───────────────────┘
```

---

## 2. Configuration (`config.yaml`)

MCP servers are configured in the optional `mcp` root block of `$AGIS_HOME/config.yaml`.

```yaml
mcp:
  enabled: true # Opt-in switch (default: false)
  servers:
    filesystem:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/workspace"]
      env:
        DEBUG: "mcp:*"
      disabled: false

    github:
      command: "docker"
      args: ["run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN", "mcp/github"]
      env:
        GITHUB_PERSONAL_ACCESS_TOKEN: "${GITHUB_TOKEN}"
      disabled: false

    remote-tools:
      url: "http://localhost:8080/sse"
      disabled: false
```

### Configuration Fields

| Field | Type | Description |
|---|---|---|
| `mcp.enabled` | `bool` | Master toggle for MCP tool client initialization. Defaults to `false`. |
| `mcp.servers` | `map[string]MCPServerConfig` | Keyed dictionary of MCP server configurations. |
| `servers.<name>.command` | `string` | Binary path or command for `stdio` transport. |
| `servers.<name>.args` | `[]string` | Arguments passed to `command`. |
| `servers.<name>.env` | `map[string]string` | Environment variables passed to the subprocess. |
| `servers.<name>.url` | `string` | HTTP endpoint URL for `sse` transport. |
| `servers.<name>.disabled` | `bool` | When `true`, the server is skipped during initialization. Defaults to `false`. |

---

## 3. Supported Transports

### Stdio Subprocess Transport (`stdio`)
- Spawns subprocesses using `os/exec.CommandContext`.
- Enforces process group isolation (`Setpgid: true` on POSIX) so child processes spawned by server binaries are cleanly reaped on shutdown.
- Reads line-delimited JSON-RPC from `stdout` while routing `stderr` to `slog`, preventing parser corruption.
- Enforces `WaitDelay = 100ms` for graceful shutdown before termination.

### SSE Network Transport (`sse`)
- Establishes a persistent HTTP `GET` connection with `Accept: text/event-stream`.
- Discovers the session posting endpoint via the initial `endpoint` SSE event.
- Transmits JSON-RPC requests via HTTP `POST` to the session URI and receives asynchronous responses via the SSE event stream.
- Supports custom HTTP headers, proxy environments, and timeout cancellation.

---

## 4. Policy Guard & Security

All MCP tool invocations are policy-evaluated under the backend identifier `mcp:<server_name>`.

### Posture Enforcement
- **`sandbox` (default)**: Denies all MCP tool invocations unless an explicit `allow` rule exists for `mcp:<server_name>` in `policy.yaml`.
- **`standard`**: Prompts the user interactively via `AskApprover` when an unlisted tool is invoked.
- **`full`**: Unrestricted execution for the current session.
- **`AutoDenyApprover`**: In headless daemons (Gateway, Cron, Webhook), any unapproved MCP tool invocation requiring interactive approval is automatically blocked (`"blocked by policy"`), preventing background execution hangs.

### Setting Policy Rules for MCP Tools

Use `agis policy` CLI to manage access to MCP tools:

```bash
# Allow read_file on filesystem MCP server
agis policy set -b mcp:filesystem "read_file" allow

# Allow all tools on sqlite server
agis policy set -b mcp:sqlite "*" allow

# Explicitly deny dangerous operations
agis policy set -b mcp:github "delete_repo" deny
```

---

## 5. CLI Subcommands

### `agis mcp list`
Lists all configured MCP servers, transport types, connectivity statuses, and discovered tools.

```bash
agis mcp list [--config config.yaml]
```

Example output:
```text
Configured MCP Servers (2):

  • filesystem           [stdio] [online] - 2 tool(s) discovered:
      - read_file            - Read contents of a file
      - write_file           - Write data to a file
  • remote-tools         [sse]   [offline] (init failed: connection refused)
```

### `agis mcp test`
Directly invokes an MCP tool without LLM orchestration, reporting response text and execution duration.

```bash
agis mcp test <server> <tool> [json_args] [--config config.yaml]
```

Example invocation:
```bash
agis mcp test filesystem read_file '{"path": "/tmp/sample.txt"}'
```

Example output:
```text
Tool: read_file (server: filesystem)
Execution time: 14.2ms

Result:
Hello from MCP filesystem server!
```
