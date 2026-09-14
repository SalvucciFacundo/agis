# Tasks: Lazy MCP Spawning & Standby Mode (lazy-mcp-standby)

## Review Workload Forecast
- Estimated changed lines: ~400-550 additions, ~30 deletions
- 400-line budget risk: Medium
- Chained PRs recommended: No (Single cohesive architectural unit)
- Delivery strategy: Single PR stacked to main
- TDD mode: Strict TDD (RED -> GREEN -> REFACTOR)

---

## Work Units

### Work Unit 1: Configuration Extensions (`internal/config`)
- [ ] Add `Standby`, `IdleTimeout`, `CacheDir` to `MCPConfig` and `MCPServerConfig`.
- [ ] Add `ParsedIdleTimeout()` helper.
- [ ] Write unit tests in `internal/config/config_test.go` covering defaults, YAML parsing, and duration validation.
- [ ] Verify test pass: `go test -race ./internal/config/...`.

### Work Unit 2: Schema Disk Cache Subsystem (`internal/mcp/cache.go`)
- [ ] Define `SchemaCache` interface in `internal/mcp/cache.go`.
- [ ] Implement `DiskSchemaCache` with atomic writing (`os.CreateTemp` + `os.Rename`) and corrupt-tolerant loading.
- [ ] Write unit tests in `internal/mcp/cache_test.go` (hit, miss, atomic save, corrupt recovery, clear).
- [ ] Verify test pass: `go test -race ./internal/mcp/...`.

### Work Unit 3: Lazy Lifecycle & Inactivity Watchdog in Manager (`internal/mcp/manager.go`)
- [ ] Introduce `serverSession` state machine and mutexes.
- [ ] Implement Standby startup in `Start()` (loads cache without spawning child processes; transient probe on miss).
- [ ] Implement on-demand activation in `CallTool()` (spawns client, inits, runs call).
- [ ] Implement inactivity watchdog (`time.Timer`) for auto-shutdown back to Standby.
- [ ] Implement `ServerStatus()`, `ListAllTools()`, and `RefreshTools()`.
- [ ] Write unit tests in `internal/mcp/manager_test.go` verifying:
  - Startup in Standby with 0 clients created.
  - On-demand client spawning upon `CallTool`.
  - Timer expiration auto-shutdown to Standby.
  - Concurrent `CallTool` calls under `-race`.
  - Graceful `Stop()`.

### Work Unit 4: CLI & Diagnostic Integration (`cmd/agis`, `internal/doctor`)
- [ ] Update `agis mcp list` in `cmd/agis/mcp.go` to display `[standby]` / `[running]` status and tool count.
- [ ] Wire `mcp.Manager` into `cmd/agis/main.go`, `cmd/agis/serve.go`, and `cmd/agis/gateway.go`.
- [ ] Update `checkMCP` in `internal/doctor/doctor.go` to report Standby capability and cache health.
- [ ] Write tests in `internal/doctor/doctor_test.go` and `cmd/agis/mcp_test.go`.

### Work Unit 5: Verification, Documentation & Roadmap Update
- [ ] Update `docs/mcp.md` and `docs/development/hermes-parity-roadmap.md` (mark Fase 9 ✅ DONE).
- [ ] Run complete test suite: `go test -race ./...` and `go vet ./...`.
- [ ] Commit with conventional commits and push.
