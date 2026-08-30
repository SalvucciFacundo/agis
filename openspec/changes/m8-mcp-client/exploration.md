# MCP Client Architecture Exploration (M8)

## Executive Summary
This document outlines the architectural implementation of a Model Context Protocol (MCP) Client within AGIS. The MCP client will enable AGIS to interact with any MCP-compliant tool server via `stdio` (local process) or `sse` (networked) transport, seamlessly integrating into the existing `core.ToolRunner` ecosystem.

## 1. MCP Protocol & Transports
- **Protocol**: JSON-RPC 2.0 based request/response and notification system.
- **Transports**:
    - `stdio`: Spawns a child process; communicates via stdin/stdout; stderr redirected for logging.
    - `sse`: Connects to an HTTP endpoint; listens for Server-Sent Events (SSE) and issues HTTP POSTs for tool calls.
- **Client Architecture**: We will implement a `Client` interface in `internal/mcp/` capable of managing both transports. The `Client` will handle JSON-RPC message framing, lifecycle management, and tool discovery (listing tools).

## 2. Integration with AGIS
- **ToolRunner**: The `core.ToolRunner` will be extended to recognize `mcp:<server_name>` backends. When a tool call is intercepted, the `ToolRunner` will route the request to `internal/mcp/client.go` `CallTool(ctx, name, args)`.
- **Policy Guard**: MCP tool calls will pass through existing `PolicyGuard` infrastructure. Because MCP tools are external, they present an attack surface; calls will be treated as `full` permission requests by default until specific granular policy is defined.

## 3. Configuration
The `Config` struct in `internal/config/config.go` will be extended:
```go
type MCPConfig struct {
    Enabled bool                      `yaml:"enabled"`
    Servers map[string]MCPServerConfig `yaml:"servers"`
}

type MCPServerConfig struct {
    Command  string            `yaml:"command"`
    Args     []string          `yaml:"args"`
    Env      map[string]string `yaml:"env"`
    URL      string            `yaml:"url"`
    Disabled bool              `yaml:"disabled"`
}
```

## 4. CLI Subcommands
- `agis mcp list`: Iterates over `config.MCP.Servers`, probes each (if `stdio`), and displays available tools.
- `agis mcp test <server> <tool> [args]`: Executes the tool directly without LLM orchestration, useful for troubleshooting.

## 5. Lifecycle & Security
- **Lifecycle**: Use `context.Context` to propagate cancellation to `stdio` processes (SIGTERM -> SIGKILL). Implement `goleak` tests to ensure processes are reaped on exit.
- **Security**: Strict validation of tool arguments against `PolicyGuard` rules before sending to MCP server.

## 6. Architectural Decisions
1.  **Transport Separation**: Use `internal/mcp/transport/` subpackage to isolate `stdio` and `sse` logic.
2.  **Tool Mapping**: Map MCP tool schemas to AGIS `internal/core.Skill` structures.
3.  **Process Management**: Wrap `os/exec` in a supervised goroutine; avoid orphan processes.
4.  **Error Handling**: MCP errors should be translated into AGIS-friendly diagnostic messages rather than raw JSON-RPC errors.
5.  **Versioning**: Client initialization should handle `initialize` and `notifications/initialized` with version negotiation.
6.  **Plugin Integration**: Treat MCP servers as first-class, dynamic tools separate from statically linked ones.

## Risks
- **Zombie Processes**: If not properly managed via lifecycle hooks.
- **Policy Evasion**: Malicious MCP servers bypassing `PolicyGuard` via side channels (unlikely but possible if not validated).
- **Tool Definition Mismatch**: Mapping MCP JSON schema to AGIS dynamic tool registration might lose fidelity.
