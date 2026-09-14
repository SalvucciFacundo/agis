# Archive Report: lazy-mcp-standby

## Change Overview
- **Name**: `lazy-mcp-standby`
- **Archived Date**: 2026-09-13
- **Status**: Completed & Archived
- **Mode**: Automatic (`auto`)
- **Artifact Store**: Hybrid (`openspec/` + Engram)

## Accomplishments
1. **Standby Mode & 0% Idle RAM**: MCP servers now boot in `Standby` mode, preserving AGIS's sub-5ms cold-start time and consuming 0 MB idle memory for uninvoked servers.
2. **Schema Disk Cache (`DiskSchemaCache`)**: Discovered tool schemas are cached to `$AGIS_HOME/cache/mcp/<server>.json` using atomic temporary file writes and corruption-tolerant fallbacks.
3. **On-Demand Subprocess Spawning**: Transport connections and stdio processes are started strictly on the first `CallTool` execution, with full thread safety and concurrency guards under `-race`.
4. **Inactivity Watchdog**: Configurable `idle_timeout` (default 5m) automatically terminates inactive child processes and resets server state to Standby.
5. **Runtime Integration**: Fully integrated into `cmd/agis/main.go` (TUI), `cmd/agis/serve.go` (OpenAI REST API), and `cmd/agis/gateway.go` (Chat gateways), with support for Dynamic Tool Search.
6. **CLI & Diagnostics**: Updated `agis mcp list` to display `[online (standby)]` status, added `-refresh` / `-r` flag, added `agis mcp sync`, and updated `internal/doctor` checks.
