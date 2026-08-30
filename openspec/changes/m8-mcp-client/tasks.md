## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~750 lines |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

---

## Task Breakdown (`m8-mcp-client`)

### PR 1: JSON-RPC 2.0 Protocol, Transports (stdio & sse) & Config Extensions

- [x] 1.1 Config extensions: Add `MCPConfig` and `MCPServerConfig` structs with safe defaults (`enabled: false`) to `internal/config/config.go`. <!-- sdd-owner: implementation -->
- [x] 1.2 Config tests: Add unit tests in `internal/config/config_test.go` verifying parsing of stdio and sse server configurations and default values. <!-- sdd-owner: implementation -->
- [x] 1.3 JSON-RPC 2.0 Protocol: Implement `internal/mcp/protocol.go` defining request, response, notification, and error structures, JSON framing, and standard error code mappings (RED → GREEN). <!-- sdd-owner: implementation -->
- [x] 1.4 Protocol tests: Add unit tests in `internal/mcp/protocol_test.go` verifying serialization, deserialization, ID tracking, and error code parsing. <!-- sdd-owner: implementation -->
- [x] 1.5 Stdio Transport: Implement `internal/mcp/transport/stdio.go` with process group supervision (`Setpgid: true` on POSIX), `WaitDelay = 100ms`, stdin/stdout line scanner, stderr redirection, and context cancellation (RED → GREEN). <!-- sdd-owner: implementation -->
- [x] 1.6 Stdio tests: Add unit tests in `internal/mcp/transport/stdio_test.go` verifying process execution, stream exchange, process group cleanup, and `goleak.VerifyNone`. <!-- sdd-owner: implementation -->
- [x] 1.7 SSE Transport: Implement `internal/mcp/transport/sse.go` with HTTP GET event stream reader, session endpoint discovery, and HTTP POST message dispatcher (RED → GREEN). <!-- sdd-owner: implementation -->
- [x] 1.8 SSE tests: Add unit tests in `internal/mcp/transport/sse_test.go` verifying SSE stream connection, event parsing, and POST dispatching with `goleak`. <!-- sdd-owner: implementation -->

### PR 2: MCP Client, Discovery, Tool Calling & Multi-Server Manager

- [x] 2.1 MCP Client: Implement `internal/mcp/client.go` with request ID tracking, lifecycle handshake (`initialize` -> `notifications/initialized`), `tools/list` discovery with pagination, and `tools/call` execution (RED → GREEN). <!-- sdd-owner: implementation -->
- [x] 2.2 Client tests: Add unit tests in `internal/mcp/client_test.go` verifying initialization handshake, tool discovery with pagination, timeout handling, and connection lifecycle. <!-- sdd-owner: implementation -->
- [x] 2.3 Multi-Server Manager: Implement `internal/mcp/manager.go` managing configured server pools, concurrent `Start(ctx)` and `Stop()`, server health checks, and aggregated tool listings (RED → GREEN). <!-- sdd-owner: implementation -->
- [x] 2.4 Manager tests: Add unit tests in `internal/mcp/manager_test.go` verifying concurrent server startup, tool aggregation across servers, and leak-free lifecycle (`goleak.VerifyNone`). <!-- sdd-owner: implementation -->

### PR 3: ToolRunner Bridge, Policy Guard Integration, CLI Subcommands & Docs

- [x] 3.1 Core interface update: Add `Name() string` and `Description() string` to `core.ToolRunner` interface in `internal/core/port_learning.go` and update existing local/plugin runners. <!-- sdd-owner: implementation -->
- [x] 3.2 ToolRunner Bridge: Implement `internal/tools/mcp.go` wrapping MCP tools into `core.ToolRunner` (`Backend() = "mcp:<server>"`) and dynamic tool definitions. <!-- sdd-owner: implementation -->
- [x] 3.3 Policy Guard Integration: Ensure MCP tool executions pass through `PolicyGuard` with `AutoDenyApprover` support in background daemons and audit trail logging. <!-- sdd-owner: implementation -->
- [x] 3.4 CLI Subcommands: Implement `cmd/agis/mcp.go` with `agis mcp list` and `agis mcp test <server> <tool> [args]` (wire into `cmd/agis/main.go`). <!-- sdd-owner: implementation -->
- [x] 3.5 Integration Tests: Implement `cmd/agis/mcp_integration_test.go` verifying end-to-end MCP tool calling from Brain and CLI under `go test -race ./...`. <!-- sdd-owner: implementation -->
- [x] 3.6 Documentation Updates: Create `docs/mcp.md`, update `docs/cli.md`, `docs/architecture.md`, `docs/configuration.md`, `docs/roadmap.md` (M8 DONE), and `README.md`. <!-- sdd-owner: implementation -->
