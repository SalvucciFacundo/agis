# SDD Archive Report: m8-mcp-client

## Milestone 8 — Model Context Protocol (MCP) Client

- **Change Name:** `m8-mcp-client`
- **Archived Date:** 2026-08-29
- **Final Status:** COMPLETED & MERGED
- **Store Mode:** Hybrid (OpenSpec + Engram)
- **TDD Mode:** Strict TDD (100% verified across 18 packages)

---

## 1. Executive Summary

Milestone 8 introduces a **native Model Context Protocol (MCP) Client** to AGIS, allowing the autonomous agent to connect dynamically to external tool servers via standard JSON-RPC 2.0 over `stdio` (local subprocesses) or `sse` (networked endpoints).

Key components delivered:
1. **JSON-RPC 2.0 Protocol Engine (`internal/mcp/protocol.go`)**: Pure Go request, response, notification, and error framing with typed error mapping and atomic request ID correlation.
2. **Stdio Process Transport (`internal/mcp/transport/stdio.go`)**: Subprocess execution with POSIX process group supervision (`Setpgid: true`), `WaitDelay = 100ms`, concurrent `stderr` draining to prevent buffer deadlocks, and leak-free process termination.
3. **SSE Network Transport (`internal/mcp/transport/sse.go`)**: HTTP Server-Sent Events client with dynamic session endpoint discovery and HTTP POST message dispatching.
4. **Native MCP Client (`internal/mcp/client.go`)**: Connection lifecycle handshake (protocol version `"2024-11-05"`: `initialize` -> `notifications/initialized`), paginated tool discovery (`tools/list` with `nextCursor`), and tool execution (`tools/call` with multi-block content parsing).
5. **Multi-Server Manager (`internal/mcp/manager.go`)**: Concurrent server pool management with `errgroup.Group`, health checks, tool aggregation (`ListAllTools`), and routing.
6. **ToolRunner Bridge & Policy Guard (`internal/tools/mcp.go`, `internal/policy/guard.go`)**: Dynamic registration of MCP tools under backend `mcp:<server_name>`, evaluated by `PolicyGuard` multi-tier security rules (`sandbox`, `standard`, `full`) and `AutoDenyApprover`.
7. **CLI Subcommands (`cmd/agis/mcp.go`)**: `agis mcp list` and `agis mcp test <server> <tool> [args]`.

---

## 2. Pull Request Delivery Sequence (Stacked to Main)

| PR | Title | Commits | Lines Changed | Status |
|---|---|---|---|---|
| **#28** | `feat(mcp): M8 PR1 — JSON-RPC 2.0 protocol, stdio and sse transports, and config extensions` | `185653f` | +1,885 / -9 | Merged |
| **#29** | `feat(mcp): M8 PR2 — native MCP client, paginated tool discovery, and multi-server manager` | `f297088` | +1,321 / -17 | Merged |
| **#30** | `feat(mcp): M8 PR3 — ToolRunner bridge, PolicyGuard integration, CLI subcommands and docs` | `8cca052` | +1,624 / -49 | Merged |

---

## 3. Capabilities Synced to `openspec/specs/`

- `mcp/spec.md` (NEW): JSON-RPC 2.0 protocol, stdio transport, SSE transport, tool discovery, and tool execution.
- `config-loader/spec.md` (MODIFIED): `mcp` configuration block schema and defaults.
- `cli/spec.md` (MODIFIED): `agis mcp list` and `agis mcp test` CLI subcommands.

---

## 4. Verification Evidence

- `go test -race -count=1 ./...` PASSED across all 18 packages:
  `cmd/agis`, `internal/adapters/llm`, `internal/adapters/tui`, `internal/config`, `internal/core`, `internal/cron`, `internal/gateway`, `internal/mcp`, `internal/mcp/transport`, `internal/memory`, `internal/persona`, `internal/plugins`, `internal/policy`, `internal/scan`, `internal/session`, `internal/skills`, `internal/tools`, `internal/webhook`.
- 0 data races detected.
- 0 goroutine leaks confirmed via `go.uber.org/goleak`.
- End-to-end integration tests in `cmd/agis/mcp_integration_test.go` verified tool invocation against an active stdio subprocess server.
