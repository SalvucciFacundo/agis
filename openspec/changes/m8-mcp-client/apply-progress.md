# Apply Progress: m8-mcp-client (PR 1)

## Status
- Current slice: PR 1 — JSON-RPC 2.0 Protocol, Transports (stdio & sse) & Config Extensions
- Delivery Strategy: auto-chain
- Chain Strategy: stacked-to-main
- PR Boundary: PR 1 of 3 (Foundation layer: Config + Protocol + Transports)

## TDD Cycle Evidence

| Task | Component | RED Test Command | GREEN Implementation | TRIANGULATE / REFACTOR | Status |
|------|-----------|------------------|----------------------|------------------------|--------|
| 1.1, 1.2 | Config extensions (`MCPConfig`, `MCPServerConfig`) | `go test ./internal/config` (undefined `cfg.MCP`) | Added `MCPConfig` & `MCPServerConfig` to `internal/config/config.go` with safe defaults (`enabled: false`) | Added full table-driven tests for stdio & sse in `config_test.go` | PASS |
| 1.3, 1.4 | JSON-RPC 2.0 Protocol & Error Codes | `go test ./internal/mcp/...` (missing package/types) | Implemented `protocol.go` (`JSONRPCRequest`, `JSONRPCResponse`, `JSONRPCNotification`, `JSONRPCError`, `ClassifyMessage`, `ParseResponse`) | Added tests for normalization of numeric/string IDs, classification, and validation | PASS |
| 1.5, 1.6 | Stdio Subprocess Transport | `go test ./internal/mcp/transport/...` (undefined `NewStdio`) | Implemented `stdio.go` with `exec.Command`, `Setpgid: true` (POSIX), `WaitDelay = 100ms`, `stderr` drain goroutine, and clean shutdown | Verified stream exchange, stderr isolation, context cancellation, and `goleak.VerifyNone` | PASS |
| 1.7, 1.8 | SSE Network Transport | `go test ./internal/mcp/transport/...` (undefined `NewSSE`) | Implemented `sse.go` with HTTP GET event stream parser, session `endpoint` event discovery, and HTTP POST message dispatcher | Verified handshake, event streaming, POST dispatching, error paths, and `goleak.VerifyNone` | PASS |

## Files Changed

### Created
- `internal/mcp/protocol.go`: JSON-RPC 2.0 wire models, error codes, serialization and parsing helpers.
- `internal/mcp/protocol_test.go`: Protocol unit tests covering requests, notifications, responses, errors, and message classification.
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
- `openspec/changes/m8-mcp-client/tasks.md`: Checked off tasks 1.1 through 1.8.

## Verification Evidence
- `go test -count=1 -race ./...`: All 17 packages pass cleanly with race detection and zero goroutine leaks.
- `go test -v -race ./internal/mcp/... ./internal/mcp/transport/...`: 100% pass across protocol, stdio transport, and sse transport.

## Remaining Tasks (PR 2 & PR 3)

### PR 2: MCP Client, Discovery, Tool Calling & Multi-Server Manager
- [ ] 2.1 MCP Client: Implement `internal/mcp/client.go` with request ID tracking, lifecycle handshake (`initialize` -> `notifications/initialized`), `tools/list` discovery with pagination, and `tools/call` execution.
- [ ] 2.2 Client tests: Add unit tests in `internal/mcp/client_test.go` verifying initialization handshake, tool discovery with pagination, timeout handling, and connection lifecycle.
- [ ] 2.3 Multi-Server Manager: Implement `internal/mcp/manager.go` managing configured server pools, concurrent `Start(ctx)` and `Stop()`, server health checks, and aggregated tool listings.
- [ ] 2.4 Manager tests: Add unit tests in `internal/mcp/manager_test.go` verifying concurrent server startup, tool aggregation across servers, and leak-free lifecycle (`goleak.VerifyNone`).

### PR 3: ToolRunner Bridge, Policy Guard Integration, CLI Subcommands & Docs
- [ ] 3.1 Core interface update: Add `Name() string` and `Description() string` to `core.ToolRunner` interface in `internal/core/port_learning.go` and update existing local/plugin runners.
- [ ] 3.2 ToolRunner Bridge: Implement `internal/tools/mcp.go` wrapping MCP tools into `core.ToolRunner` (`Backend() = "mcp:<server>"`) and dynamic tool definitions.
- [ ] 3.3 Policy Guard Integration: Ensure MCP tool executions pass through `PolicyGuard` with `AutoDenyApprover` support in background daemons and audit trail logging.
- [ ] 3.4 CLI Subcommands: Implement `cmd/agis/mcp.go` with `agis mcp list` and `agis mcp test <server> <tool> [args]` (wire into `cmd/agis/main.go`).
- [ ] 3.5 Integration Tests: Implement `cmd/agis/mcp_integration_test.go` verifying end-to-end MCP tool calling from Brain and CLI under `go test -race ./...`.
- [ ] 3.6 Documentation Updates: Create `docs/mcp.md`, update `docs/cli.md`, `docs/architecture.md`, `docs/configuration.md`, `docs/roadmap.md` (M8 DONE), and `README.md`.
