# SDD Proposal: m8-mcp-client

## Capabilities Contract
- `mcp` (NEW): Native Model Context Protocol client (`internal/mcp/`), `stdio` & `sse` transports, JSON-RPC 2.0 protocol, dynamic tool discovery (`tools/list`), and execution (`tools/call`).
- `tools` (MODIFIED): Bridge discovered MCP tools into `core.ToolRunner` under `mcp:<server_name>`, evaluated by `PolicyGuard`.
- `config-loader` (MODIFIED): `mcp` root configuration schema with `enabled` and `servers` map.
- `cli` (MODIFIED): `agis mcp list` and `agis mcp test <server> <tool> [args]` subcommands.

## Architectural Decisions
- **D1**: JSON-RPC 2.0 wire protocol & message framing in pure Go without external runtime dependencies.
- **D2**: Process supervision for `stdio` transport: graceful shutdown via context, `WaitDelay`, and process group teardown to prevent zombies.
- **D3**: Dynamic tool mapping into `core.ToolRunner` and `tools.Registry`.
- **D4**: `PolicyGuard` integration (enforcing `sandbox`, `standard`, `full`, and `AutoDenyApprover` for background daemons).
- **D5**: SSE / HTTP transport support for remote MCP endpoints.
- **D6**: Configuration layering & defaults in `internal/config/config.go`.
- **D7**: CLI subcommands for inspection and standalone testing.
- **D8**: Chained PR strategy & Review Workload forecast (~700-900 lines total across 3 PR slices).

## Security & Guardrails
- **Deny-by-default**: Unapproved MCP tools require explicit policy allows.
- **Fail-closed**: Policy evaluation failures deny the action.
- **Sanitized Errors**: Tool error messages sanitized to prevent data leaks.
- **Limits**: Payload size limits on JSON-RPC framing to prevent OOM.

## Compatibility & Rollback
- **Backward Compatible**: 100% backward compatible (MCP disabled by default `enabled: false`).
- **Rollback**: Rollback by setting `mcp.enabled: false` in `config.yaml`.
