# Proposal: Lazy MCP Spawning & Standby Mode (lazy-mcp-standby)

## Intent
The goal of this change is to implement **Lazy MCP Spawning and Standby Mode** in AGIS. Currently, configuring multiple Model Context Protocol (MCP) servers requires either eagerly launching external stdio child processes or opening persistent SSE network connections at boot time, consuming system memory (RAM) and delaying startup. By introducing Standby mode, tool schemas are discovered once and persisted in a local schema cache, allowing AGIS to start with 0 active subprocesses and 0% idle RAM overhead. External servers are spawned strictly on demand when a tool is called (`CallTool`) and automatically shut down after a configurable inactivity period (`idle_timeout: 5m`).

## Scope
1. **Schema Caching**:
   - Persist discovered MCP tool schemas in `$AGIS_HOME/cache/mcp/<server>.json`.
   - On startup, populate available tools from cache without launching any external process.
   - If cache is absent, execute a transient discovery handshake and persist schemas before returning to Standby.
2. **On-Demand Lifecycle Management**:
   - Support explicit lifecycle states per server (`StateStandby`, `StateStarting`, `StateRunning`, `StateStopping`, `StateDisabled`).
   - Spawn transport and client lazily upon the first invocation of `CallTool`.
   - Implement an inactivity watchdog timer per running server that closes the connection and transitions the server back to `StateStandby` after `idle_timeout`.
3. **Configuration & Defaults**:
   - Add `standby: bool` (default: `true`), `idle_timeout: string` (default: `"5m"`), and `cache_dir: string` to `MCPConfig` and `MCPServerConfig`.
4. **Runtime & CLI Integration**:
   - Wire `mcp.Manager` into runtime commands (`cmd/agis/main.go`, `serve.go`, `gateway.go`).
   - Update `agis mcp list` to display `[standby]` / `[running]` status and support `--refresh`.
   - Add diagnostic checks for MCP Standby and cache health in `internal/doctor`.

## Affected Areas
- `internal/config/config.go`, `internal/config/config_test.go`
- `internal/mcp/cache.go`, `internal/mcp/cache_test.go` (new files)
- `internal/mcp/manager.go`, `internal/mcp/manager_test.go`
- `internal/tools/mcp.go`, `internal/tools/mcp_test.go`
- `internal/doctor/doctor.go`, `internal/doctor/doctor_test.go`
- `cmd/agis/mcp.go`, `cmd/agis/main.go`, `cmd/agis/serve.go`, `cmd/agis/gateway.go`
- `cmd/agis/mcp_integration_test.go`
- `docs/development/hermes-parity-roadmap.md`, `docs/mcp.md`

## Risks & Mitigations
- **First-Call Latency**: Spawning a process on the first `CallTool` takes ~100-300ms. Mitigated by keeping servers alive for the duration of the `idle_timeout` window so consecutive turns incur 0 startup latency.
- **Cache Drift**: External MCP server tools could change while cached. Mitigated by offering `agis mcp list --refresh`, `agis mcp sync`, and manual invalidation.
- **Race Conditions on Concurrent Tool Calls**: Multiple concurrent tool calls to the same standby server must not trigger duplicate process spawns. Mitigated by mutex synchronization per server during state transitions.

## Success Criteria
- [ ] AGIS starts with configured MCP servers without spawning any subprocess or SSE connection when cache exists.
- [ ] `CallTool` reliably spawns the server on-demand and returns execution results.
- [ ] Inactivity watchdog terminates child processes after `idle_timeout` and resets state to `StateStandby`.
- [ ] `agis mcp list` reflects live states (`[standby]`, `[running]`, `[disabled]`) and cached counts.
- [ ] Full test suite passes under `go test -race ./...` with zero goroutine leaks.
