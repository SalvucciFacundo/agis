# Apply Progress: m8-mcp-client (Complete: PR 1, PR 2 & PR 3)

## Status
- Current slice: PR 3 — ToolRunner Bridge, Policy Guard Integration, CLI Subcommands & Docs
- Delivery Strategy: auto-chain
- Chain Strategy: stacked-to-main
- PR Boundary: Complete (PR 1: Protocol & Transports -> PR 2: Client & Manager -> PR 3: Bridge, Policy, CLI & Docs)

## TDD Cycle Evidence

| Task | Component | RED Test Command | GREEN Implementation | TRIANGULATE / REFACTOR | Status |
|------|-----------|------------------|----------------------|------------------------|--------|
| 1.1, 1.2 | Config extensions (`MCPConfig`, `MCPServerConfig`) | `go test ./internal/config` (undefined `cfg.MCP`) | Added `MCPConfig` & `MCPServerConfig` to `internal/config/config.go` with safe defaults (`enabled: false`) | Added full table-driven tests for stdio & sse in `config_test.go` | PASS |
| 1.3, 1.4 | JSON-RPC 2.0 Protocol & Error Codes | `go test ./internal/mcp/...` (missing package/types) | Implemented `protocol.go` (`JSONRPCRequest`, `JSONRPCResponse`, `JSONRPCNotification`, `JSONRPCError`, `ClassifyMessage`, `ParseResponse`) | Added tests for normalization of numeric/string IDs, classification, and validation | PASS |
| 1.5, 1.6 | Stdio Subprocess Transport | `go test ./internal/mcp/transport/...` (undefined `NewStdio`) | Implemented `stdio.go` with `exec.Command`, `Setpgid: true` (POSIX), `WaitDelay = 100ms`, `stderr` drain goroutine, and clean shutdown | Verified stream exchange, stderr isolation, context cancellation, and `goleak.VerifyNone` | PASS |
| 1.7, 1.8 | SSE Network Transport | `go test ./internal/mcp/transport/...` (undefined `NewSSE`) | Implemented `sse.go` with HTTP GET event stream parser, session `endpoint` event discovery, and HTTP POST message dispatcher | Verified handshake, event streaming, POST dispatching, error paths, and `goleak.VerifyNone` | PASS |
| 2.1, 2.2 | MCP Client (`client.go`, `client_test.go`) | `go test ./internal/mcp/...` (undefined `NewClient`, `Client`, `Tool`) | Implemented `client.go` with request ID tracking, `initialize` handshake, `notifications/initialized`, paginated `tools/list`, and `tools/call` parsing | Added tests for single & multi-page pagination, error responses, context deadlines, concurrent multiplexing, and `goleak.VerifyNone` | PASS |
| 2.3, 2.4 | Multi-Server Manager (`manager.go`, `manager_test.go`) | `go test ./internal/mcp/...` (undefined `NewManager`, `Manager`) | Implemented `manager.go` with concurrent server initialization using `errgroup`, disabled server skipping, tool aggregation (`ListAllTools`), routing (`CallTool`), and graceful shutdown (`Stop`) | Verified concurrent start/stop, failure cleanup, unknown server handling, and `goleak.VerifyNone` | PASS |
| 3.1 | Core Interface Update (`ToolRunner`) | `go test ./internal/tools/... ./internal/plugins/...` (missing `Name()` / `Description()`) | Added `Name() string` and `Description() string` to `core.ToolRunner` in `internal/core/port_learning.go`; updated `Local`, `Docker`, `SSH`, and `PluginRunner` | Table-driven assertions across unit tests in `local_test.go`, `backends_test.go`, and `manager_test.go` | PASS |
| 3.2 | ToolRunner Bridge (`internal/tools/mcp.go`) | `go test ./internal/tools/...` (undefined `MCPRunner`, `FromMCPManager`) | Implemented `MCPRunner` bridging `mcp.Tool` into `core.ToolRunner` under `mcp:<server_name>` with JSON parameter parsing; implemented `FromMCPManager` aggregator | Table-driven unit tests in `internal/tools/mcp_test.go` covering JSON arguments, empty args, invalid JSON, client error mapping, and manager aggregation with deterministic sorting | PASS |
| 3.3 | Policy Guard Integration | `go test -v ./internal/core -run TestBrainLoop_MCPTool` (empty command error in `executeTool`) | Updated `Brain.executeTool` in `internal/core/brain.go` to route `mcp:<server_name>` tool calls, evaluate `GuardRequest{Backend: "mcp:<server>", Category: "commands", Subject: toolName}`, and support `AutoDenyApprover` in background daemons | Added unit tests in `brain_tools_test.go` and `internal/policy/guard_test.go` for sandbox allow/deny, standard ask, and audit log tracking | PASS |
| 3.4 | CLI Subcommands (`cmd/agis/mcp.go`) | `go test ./cmd/agis/... -run TestRunMCPCLI` (undefined `RunMCPCLI`) | Implemented `cmd/agis/mcp.go` with `agis mcp list` and `agis mcp test <server> <tool> [args]`; wired into `cmd/agis/main.go` | Table-driven unit tests in `cmd/agis/mcp_test.go` for usage, unknown subcommands, empty servers, disabled servers, missing arguments, and server validation | PASS |
| 3.5 | End-to-End Integration Tests | `go test -v -race ./cmd/agis -run TestMCP_EndToEnd` | Created `cmd/agis/mcp_integration_test.go` spinning up a real stdio MCP server subprocess | Verified end-to-end tool calling via Brain, sandbox auto-deny with `AutoDenyApprover`, audit log persistence, CLI list/test execution, and `goleak.VerifyNone` | PASS |
| 3.6 | Documentation Updates | N/A | Authored `docs/mcp.md` (comprehensive guide); updated `docs/cli.md`, `docs/architecture.md`, `docs/configuration.md`, `docs/roadmap.md` (M8 DONE), and `README.md` | Verified markdown links and code examples | PASS |

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
- `internal/tools/mcp.go`: `MCPRunner` bridge implementing `core.ToolRunner` and `FromMCPManager` aggregator.
- `internal/tools/mcp_test.go`: Unit tests for `MCPRunner` and `FromMCPManager`.
- `cmd/agis/mcp.go`: CLI subcommand runner for `agis mcp list` and `agis mcp test`.
- `cmd/agis/mcp_test.go`: CLI subcommand unit tests.
- `cmd/agis/mcp_integration_test.go`: End-to-end integration tests for MCP Brain tool execution, PolicyGuard, and CLI subcommands under `goleak`.
- `docs/mcp.md`: Comprehensive guide to Model Context Protocol in AGIS.

### Modified
- `internal/config/config.go`: Added `MCPConfig` and `MCPServerConfig` types; default `enabled: false`.
- `internal/config/config_test.go`: Added unit tests for stdio and sse config parsing.
- `internal/core/port_learning.go`: Added `Name() string` and `Description() string` to `core.ToolRunner`.
- `internal/core/brain.go`: Updated `runnerFor` and `executeTool` to dynamically route and evaluate `mcp:<server_name>` tools under `PolicyGuard`.
- `internal/core/brain_tools_test.go`: Added unit tests for MCP tool routing, sandbox policy evaluation, and `AutoDenyApprover`.
- `internal/tools/local.go`: Implemented `Name()` and `Description()`.
- `internal/tools/local_test.go`: Added assertions for `Name()` and `Description()`.
- `internal/tools/backends.go`: Implemented `Name()` and `Description()` on `Docker` and `SSH`.
- `internal/tools/backends_test.go`: Added assertions for `Name()` and `Description()`.
- `internal/plugins/manager.go`: Implemented `Name()` and `Description()` on `PluginRunner`.
- `internal/plugins/manager_test.go`: Added assertions for `Name()` and `Description()`.
- `internal/policy/guard.go`: Updated sandbox evaluation to permit explicit allow rules on `mcp:*` backends.
- `internal/policy/guard_test.go`: Added tests for MCP policy evaluation in sandbox and standard postures.
- `cmd/agis/main.go`: Wired `agis mcp` subcommand into root routing.
- `internal/cron/edge_cases_test.go`: Fixed timing test under `-race` using `require.Eventually`.
- `docs/cli.md`: Added documentation for `agis mcp` subcommands.
- `docs/architecture.md`: Documented `internal/mcp/`, `internal/mcp/transport/`, and `MCPRunner`.
- `docs/configuration.md`: Added `mcp` root configuration block specification.
- `docs/roadmap.md`: Marked Milestone 8 (M8: Model Context Protocol Client) as DONE.
- `README.md`: Added MCP client features, CLI subcommands, documentation links, and M8 status.
- `openspec/changes/m8-mcp-client/tasks.md`: Checked off tasks 1.1 through 3.6.
- `go.mod`, `go.sum`: Added `golang.org/x/sync` for `errgroup`.

## Verification Evidence
- `go test -count=1 -race ./...`: 18/18 packages pass cleanly with race detection and zero goroutine leaks.
- `go vet ./...`: 0 issues found.
- `go build ./cmd/agis`: Clean compilation of binary.

## Remaining Tasks
None. All tasks across PR 1, PR 2, and PR 3 are 100% complete.
