# SDD Verification Report: Lazy MCP Spawning & Standby Mode (lazy-mcp-standby)

## Summary
- **Change**: `lazy-mcp-standby`
- **Project**: `agis`
- **Status**: `PASS`
- **TDD Mode**: Strict TDD (`ACTIVE`)
- **Artifact Store**: `hybrid` (`openspec/` + Engram)

---

## Executive Summary
The implementation for `lazy-mcp-standby` (Fase 9 of the Hermes Agent Parity Roadmap) has been completed and verified with zero compilation errors, zero test failures, and zero data races across all packages.

AGIS now starts with 0 external child processes when configured with external stdio/SSE MCP servers. Tool schemas are discovered once and persisted locally (`$AGIS_HOME/cache/mcp/<server>.json`). When tools are called via `core.Brain` or `agis mcp test`, child processes are started on demand and automatically shut down after 5 minutes of inactivity (`idle_timeout: 5m`), achieving 0% idle RAM usage in production.

---

## Test Executions

### 1. Unit Tests (`internal/config`, `internal/mcp`, `internal/tools`, `internal/doctor`)
```bash
go test -v -race ./internal/config/... ./internal/mcp/... ./internal/tools/... ./internal/doctor/...
```
- **Result**: `PASS`
- **Details**:
  - `TestDiskSchemaCache_HitMissAndInvalidate`: PASS
  - `TestDiskSchemaCache_CorruptedFileRecovery`: PASS
  - `TestDiskSchemaCache_Clear`: PASS
  - `TestManager_Standby_WarmCacheNoSubprocess`: PASS (0 clients created at boot, on-demand activation verified)
  - `TestManager_Standby_ColdCacheTransientProbe`: PASS (transient probe correctly closes client and saves cache)
  - `TestManager_Standby_InactivityWatchdogAutoShutdown`: PASS (watchdog closes client after timeout)
  - `TestManager_Standby_ConcurrentCalls`: PASS (10 concurrent requests cleanly multiplexed to 1 instance without race)
  - `TestManager_RefreshTools`: PASS

### 2. Integration Tests (`cmd/agis`)
```bash
go test -v -race ./cmd/agis/... -run "TestMCP"
```
- **Result**: `PASS`
- **Details**:
  - `TestMCP_EndToEnd_BrainAndPolicyGuard`: PASS
  - `TestMCP_EndToEnd_CLISubcommands`: PASS

### 3. Full Regression Suite
```bash
go test -race ./...
```
- **Result**: `PASS` across all 26 packages.
- **Data Races**: 0
- **Goroutine Leaks (`goleak`)**: 0
