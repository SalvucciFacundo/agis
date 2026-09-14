# Apply Progress: Lazy MCP Spawning & Standby Mode (lazy-mcp-standby)

## Overview
Implementation of on-demand MCP child process lifecycle, local schema caching, inactivity watchdog auto-shutdown, and runtime CLI integration.

- **Status**: Completed
- **Strategy**: Single PR stacked to main
- **TDD Mode**: Strict TDD (RED -> GREEN -> REFACTOR)

---

## Work Units Progress

| Unit | Focus Area | RED / Failure Verification | Implementation | Validation & Tests | Status |
|---|---|---|---|---|---|
| **1.1** | Config Models (`internal/config/config.go`) | `cfg.MCP.Standby undefined`, `ParsedIdleTimeout undefined` | Added `Standby`, `IdleTimeout`, `CacheDir` to `MCPConfig` and `MCPServerConfig` with duration parsers and defaults | Unit tests in `internal/config/config_test.go` | PASS |
| **1.2** | Schema Disk Cache (`internal/mcp/cache.go`) | `NewDiskSchemaCache undefined` | Implemented `DiskSchemaCache` with atomic write (`os.CreateTemp` + rename) and corrupt recovery | Unit tests in `internal/mcp/cache_test.go` | PASS |
| **1.3** | Lazy Lifecycle & Watchdog (`internal/mcp/manager.go`) | `WithSchemaCache undefined`, `ServerStatus undefined` | Implemented `serverSession` state machine, warm-cache Standby, on-demand activation in `CallTool`, and idle auto-shutdown | Unit tests in `internal/mcp/manager_test.go` under `-race` | PASS |
| **1.4** | CLI & Diagnostics (`cmd/agis`, `internal/doctor`) | Integration test assertion and flag validation | Updated `agis mcp list` (`--refresh`, `sync`), wired `mcp.Manager` into `main.go`, `serve.go`, `gateway.go`, and updated `checkMCP` | `cmd/agis/mcp_test.go`, `cmd/agis/mcp_integration_test.go`, `internal/doctor/doctor_test.go` | PASS |
| **1.5** | Documentation & Roadmap | N/A | Updated `docs/mcp.md` and `docs/development/hermes-parity-roadmap.md` | Verification of roadmap sync | PASS |

---

## File Changes Summary
- `internal/config/config.go`: Added Standby, IdleTimeout, CacheDir fields and helper methods.
- `internal/config/config_test.go`: Added test cases for MCP standby configuration defaults and overrides.
- `internal/mcp/cache.go`: Created `SchemaCache` interface and `DiskSchemaCache` implementation.
- `internal/mcp/cache_test.go`: Unit tests for cache hit/miss, corruption recovery, and atomic clear.
- `internal/mcp/manager.go`: Implemented Standby lifecycle, on-demand spawning, and inactivity watchdog.
- `internal/mcp/manager_test.go`: Added unit tests for warm cache, cold cache transient probe, idle auto-shutdown, and concurrency.
- `internal/tools/mcp_test.go`: Updated mock manager for interface parity.
- `internal/doctor/doctor.go`: Updated `checkMCP` to report Standby mode and schema cache path.
- `cmd/agis/mcp.go`: Added `-refresh` / `-r` flag, `sync` subcommand, and standby cache display.
- `cmd/agis/main.go`: Wired `mcp.Manager` into interactive TUI runtime and runners.
- `cmd/agis/serve.go`: Wired `mcp.Manager` into HTTP API server daemon.
- `cmd/agis/gateway.go`: Wired `mcp.Manager` into chat gateway daemon.
- `docs/development/hermes-parity-roadmap.md`: Updated parity matrix and marked Fase 9 as shipped.
- `docs/mcp.md`: Documented Standby mode, schema caching, and auto-shutdown.
