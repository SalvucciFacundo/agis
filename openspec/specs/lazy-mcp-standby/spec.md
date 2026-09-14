# Specification: Lazy MCP Spawning & Standby Mode (lazy-mcp-standby)

## Purpose
Define the functional and architectural contracts for on-demand MCP server process lifecycle, disk schema caching, auto-shutdown on inactivity, and runtime CLI integration.

---

## Requirements

### Requirement MCP-STB-001: Configuration Attributes
1. `config.MCPConfig` MUST include:
   - `Standby bool` (`yaml:"standby"`): defaults to `true`.
   - `IdleTimeout string` (`yaml:"idle_timeout,omitempty"`): defaults to `"5m"`.
   - `CacheDir string` (`yaml:"cache_dir,omitempty"`): defaults to empty (which resolves to `$AGIS_HOME/cache/mcp`).
2. `config.MCPServerConfig` MUST include:
   - `Standby *bool` (`yaml:"standby,omitempty"`): optional per-server override.
   - `IdleTimeout *string` (`yaml:"idle_timeout,omitempty"`): optional per-server override.
3. `MCPConfig.ParsedIdleTimeout()` MUST parse the string into `time.Duration`, falling back to `5 * time.Minute` on error or when empty.

### Requirement MCP-STB-002: Schema Disk Cache (`SchemaCache`)
1. The system MUST provide an interface `SchemaCache`:
   ```go
   type SchemaCache interface {
       Load(serverName string) ([]Tool, bool, error)
       Save(serverName string, tools []Tool) error
       Invalidate(serverName string) error
       Clear() error
   }
   ```
2. `DiskSchemaCache` MUST store cached definitions in `<cacheDir>/<serverName>.json`.
3. Writing to cache MUST be atomic (write to temp file then rename).
4. Corrupted cache files MUST be safely invalidated without crashing the process.

### Requirement MCP-STB-003: Standby Startup Semantics
1. When `Manager.Start(ctx)` runs:
   - If `Standby` is enabled:
     - For each server with a cache hit, `Manager` MUST load tools into memory without initializing a transport or spawning a child process. State is set to `StateStandby`.
     - For each server with a cache miss, `Manager` MUST launch a temporary client, run `Initialize` and `ListTools`, persist the tools to `SchemaCache`, and immediately `Close()` the client, transitioning to `StateStandby`.
   - If `Standby` is disabled, `Manager` starts all servers eagerly as before (`StateRunning`).
2. `Manager.ListAllTools()` MUST return the cached tools even when all servers are in `StateStandby`.

### Requirement MCP-STB-004: On-Demand Spawning on `CallTool`
1. When `Manager.CallTool(ctx, serverName, toolName, args)` is invoked:
   - If the server is in `StateStandby` or disconnected:
     - `Manager` MUST instantiate the transport and client via `clientFactory`.
     - `Manager` MUST execute `client.Initialize(ctx)`.
     - `Manager` MUST transition state to `StateRunning`.
     - `Manager` MUST start an inactivity watchdog timer for `idleTimeout`.
   - If the server is already in `StateRunning`:
     - `Manager` MUST reset the inactivity watchdog timer.
   - `Manager` MUST forward the execution to `client.CallTool(ctx, toolName, args)`.
2. Multiple concurrent calls to `CallTool` for the same server MUST be synchronized such that exactly one connection is opened.

### Requirement MCP-STB-005: Inactivity Watchdog Auto-Shutdown
1. A running server MUST start a timer set to its resolved `idle_timeout`.
2. Each tool invocation on that server MUST reset the timer.
3. When the timer expires:
   - The manager MUST close the client connection (`client.Close()`).
   - The manager MUST set the active client to `nil` and transition the server to `StateStandby`.
   - The manager MUST log the standby transition at `slog.LevelInfo`.

### Requirement MCP-STB-006: Server Status Inspection & Synchronization
1. `Manager` MUST expose:
   - `ServerStatus(serverName string) (ServerStatus, bool)`
   - `RefreshTools(ctx context.Context, serverName string) ([]Tool, error)`
2. `ServerStatus` struct MUST include:
   - `Name string`
   - `State ServerState` (`"standby"`, `"starting"`, `"running"`, `"stopping"`, `"disabled"`)
   - `ToolCount int`
   - `LastActivity time.Time`
   - `Transport string` (`"stdio"` or `"sse"`)

### Requirement MCP-STB-007: CLI and Diagnostic Updates
1. `agis mcp list` MUST show status `[standby]` or `[running]` along with cached tool counts.
2. `agis mcp list --refresh` (or `-r`) MUST force cache invalidation and refresh tools.
3. `internal/doctor` MUST report MCP Standby capability and cache directory health.
4. `cmd/agis/main.go`, `cmd/agis/serve.go`, and `cmd/agis/gateway.go` MUST integrate `mcp.Manager` into runtime `runners`.
