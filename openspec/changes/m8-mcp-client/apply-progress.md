# Apply Progress: m8-mcp-client (PR 1 & PR 2)

## Status
- Current slice: PR 2 — MCP Client, Discovery, Tool Calling & Multi-Server Manager
- Delivery Strategy: auto-chain
- Chain Strategy: stacked-to-main
- PR Boundary: PR 2 of 3 (Core Client Layer: MCP Client + Tool Discovery/Execution + Multi-Server Manager)

## TDD Cycle Evidence

| Task | Component | RED Test Command | GREEN Implementation | TRIANGULATE / REFACTOR | Status |
|------|-----------|------------------|----------------------|------------------------|--------|
| 1.1, 1.2 | Config extensions (`MCPConfig`, `MCPServerConfig`) | `go test ./internal/config` (undefined `cfg.MCP`) | Added `MCPConfig` & `MCPServerConfig` to `internal/config/config.go` with safe defaults (`enabled: false`) | Added full table-driven tests for stdio & sse in `config_test.go` | PASS |
| 1.3, 1.4 | JSON-RPC 2.0 Protocol & Error Codes | `go test ./internal/mcp/...` (missing package/types) | Implemented `protocol.go` (`JSONRPCRequest`, `JSONRPCResponse`, `JSONRPCNotification`, `JSONRPCError`, `ClassifyMessage`, `ParseResponse`) | Added tests for normalization of numeric/string IDs, classification, and validation | PASS |
| 1.5, 1.6 | Stdio Subprocess Transport | `go test ./internal/mcp/transport/...` (undefined `NewStdio`) | Implemented `stdio.go` with `exec.Command`, `Setpgid: true` (POSIX), `WaitDelay = 100ms`, `stderr` drain goroutine, and clean shutdown | Verified stream exchange, stderr isolation, context cancellation, and `goleak.VerifyNone` | PASS |
| 1.7, 1.8 | SSE Network Transport | `go test ./internal/mcp/transport/...` (undefined `NewSSE`) | Implemented `sse.go` with HTTP GET event stream parser, session `endpoint` event discovery, and HTTP POST message dispatcher | Verified handshake, event streaming, POST dispatching, error paths, and `goleak.VerifyNone` | PASS |
| 2.1, 2.2 | MCP Client (`client.go`, `client_test.go`) | `go test ./internal/mcp/...` (undefined `NewClient`, `Client`, `Tool`) | Implemented `client.go` with request ID tracking, `initialize` handshake, `notifications/initialized`, paginated `tools/list`, and `tools/call` parsing | Added tests for single & multi-page pagination, error responses, context deadlines, concurrent multiplexing, and `goleak.VerifyNone` | PASS |
| 2.3, 2.4 | Multi-Server Manager (`manager.go`, `manager_test.go`) | `go test ./internal/mcp/...` (undefined `NewManager`, `Manager`) | Implemented `manager.go` with concurrent server initialization using `errgroup`, disabled server skipping, tool aggregation (`ListAllTools`), routing (`CallTool`), and graceful shutdown (`Stop`) | Verified concurrent start/stop, failure cleanup, unknown server handling, and `goleak.VerifyNone` | PASS |

## Files Changed

### Created
- `internal/mcp/protocol.go`: JSON-RPC 2.0 wire models, error codes, serialization and parsing helpers.
- `internal/mcp/protocol_test.go`: Protocol unit tests covering requests, notifications, responses, errors, and message classification.
- `internal/mcp/client.go`: MCP Client interface, handshake lifecycle, tool discovery with pagination, tool invocation, and atomic request multiplexing.
- `internal/mcp/client_test.go`: Unit tests for MCP Client with mock transport, lifecycle handshake, paginated discovery, execution results, error mapping, and `goleak.VerifyNone`.
- `internal/mcp/manager.go`: Multi-server MCP Manager coordinating server configurations, concurrent startup via `errgroup`, graceful teardown, and tool routing.
- `internal/mcp/manager_test.go`: Unit tests for Manager covering concurrent startup, skipping disabled servers, tool discovery aggregation, routing, and cleanup.
- `internal/mcp/transport/transport.go`: `Transport` interface definition (`Send`, `Receive`, `Close`).
- `internal/mcp/transport/stdio.go`: Local process lifecycle, stdin/stdout line pipes, stderr draining.
- `internal/mcp/transport/stdio_unix.go`: POSIX process group configuration (`Setpgid: true`).
- `internal/mcp/transport/stdio_windows.go`: Windows compatibility stub.
- `internal/mcp/transport/stdio_test.go`: Stdio transport unit tests with `goleak.VerifyNone`.
- `internal/mcp/transport/sse.go`: SSE event stream receiver and HTTP POST dispatcher.
- `internal/mcp/transport/sse_test.go`: SSE transport unit tests with `httptest.Server` and `goleak.VerifyNone`.

### Modified
- `internal/config/config.go`: Added `MCPConfig` and `MCPServerConfig` types; default `enabled: false`.
- `internal/config/config_test.go`: Added unit tests for stdio and sse config parsing.
- `internal/cron/edge_cases_test.go`: Fixed flaky timing test under `-race` using `require.Eventually`.
- `openspec/changes/m8-mcp-client/tasks.md`: Checked off tasks 1.1 through 1.8 and 2.1 through 2.4.
- `go.mod`, `go.sum`: Added `golang.org/x/sync` for `errgroup`.

## Verification Evidence
- `go test -count=1 -race ./...`: 18/18 packages pass cleanly with race detection and zero goroutine leaks.
- `go test -v -race ./internal/mcp/... ./internal/mcp/transport/...`: 100% pass across protocol, client, manager, stdio transport, and sse transport.

## Remaining Tasks (PR 3)

### PR 3: ToolRunner Bridge, Policy Guard Integration, CLI Subcommands & Docs
- [ ] 3.1 Core interface update: Add `Name() string` and `Description() string` to `core.ToolRunner` interface in `internal/core/port_learning.go` and update existing local/plugin runners.
- [ ] 3.2 ToolRunner Bridge: Implement `internal/tools/mcp.go` wrapping MCP tools into `core.ToolRunner` (`Backend() = "mcp:<server>"`) and dynamic tool definitions.
- [ ] 3.3 Policy Guard Integration: Ensure MCP tool executions pass through `PolicyGuard` with `AutoDenyApprover` support in background daemons and audit trail logging.
- [ ] 3.4 CLI Subcommands: Implement `cmd/agis/mcp.go` with `agis mcp list` and `agis mcp test <server> <tool> [args]` (wire into `cmd/agis/main.go`).
- [ ] 3.5 Integration Tests: Implement `cmd/agis/mcp_integration_test.go` verifying end-to-end MCP tool calling from Brain and CLI under `go test -race ./...`.
- [ ] 3.6 Documentation Updates: Create `docs/mcp.md`, update `docs/cli.md`, `docs/architecture.md`, `docs/configuration.md`, `docs/roadmap.md` (M8 DONE), and `README.md`.
