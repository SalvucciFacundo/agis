# Apply Progress: m6-ecosystem

## PR Slice: PR 2 — Cron Scheduler Engine & Target Delivery

### Status
- Mode: auto-chain
- Strategy: stacked-to-main
- Current PR: PR 2 (Cron Scheduler Engine & Target Delivery)
- Work Unit: Tasks 2.1 - 2.6
- Outcome: Completed & Verified

---

### Completed Tasks

#### PR 1 (Gateway Substrate, Multiplexer & Adapters)
- [x] 1.1 Implement config extensions for `gateway` in `internal/config/config.go`.
- [x] 1.2 Implement `internal/gateway/adapter.go` defining the `Adapter` interface and static allowlist validation (RED → GREEN).
- [x] 1.3 Implement non-interactive `AutoDenyApprover` in `internal/gateway/approver.go` to auto-deny `DecisionAsk` under sandbox policy (RED → GREEN).
- [x] 1.4 Implement Telegram adapter in `internal/gateway/telegram.go` with polling/webhook updates, 4096-char chunking, and session routing.
- [x] 1.5 Implement Discord adapter in `internal/gateway/discord.go` with message creation listeners, 2000-char splitting, and session routing.
- [x] 1.6 Implement `internal/gateway/multiplexer.go` to orchestrate adapters concurrently and handle graceful shutdown via `context.Context`.
- [x] 1.7 Add unit tests and integration tests under `internal/gateway/` and verify with `go test ./internal/gateway/...`.
- [x] 1.8 Implement Cobra subcommand `agis gateway` in `cmd/agis/gateway.go`.

#### PR 2 (Cron Scheduler Engine & Target Delivery)
- [x] 2.1 Implement config extensions for `cron` in `internal/config/config.go`.
- [x] 2.2 Implement cron expression and duration interval parser with validation in `internal/cron/scheduler.go` (RED → GREEN).
- [x] 2.3 Implement background scheduler engine in `internal/cron/engine.go` executing prompts via `core.Brain.Step` with sandbox policy and session binding.
- [x] 2.4 Implement cron target notification delivery forwarding outputs to the Gateway Multiplexer or logger.
- [x] 2.5 Add unit tests under `internal/cron/` and verify with `go test ./internal/cron/...`.
- [x] 2.6 Implement Cobra subcommands `agis cron run` and `agis cron list` in `cmd/agis/cron.go`.

---

### TDD Cycle Evidence

| Unit / Component | Phase | Test Case | Status | Evidence |
|---|---|---|---|---|
| `internal/config` (PR1) | RED | `TestLoad_GatewayDefaultsAndExplicit` | FAIL | Field undefined on `*Config` |
| `internal/config` (PR1) | GREEN | `TestLoad_GatewayDefaultsAndExplicit` | PASS | `GatewayConfig` added with safe zero defaults |
| `internal/gateway/adapter` | RED | `TestIsAllowed` | FAIL | Package missing |
| `internal/gateway/adapter` | GREEN | `TestIsAllowed` | PASS | Static allowlist comparison (fail-closed) |
| `internal/gateway/approver` | RED | `TestAutoDenyApprover` | FAIL | `NewAutoDenyApprover` undefined |
| `internal/gateway/approver` | GREEN | `TestAutoDenyApprover` | PASS | Auto-denies `DecisionAsk` with warning log |
| `internal/gateway/telegram` | RED | `TestSplitMessage`, `TestTelegramAdapter_LifecycleAndPolling`, `TestTelegramAdapter_Send_Chunking` | FAIL | Types undefined |
| `internal/gateway/telegram` | GREEN | `TestSplitMessage`, `TestTelegramAdapter_LifecycleAndPolling`, `TestTelegramAdapter_Send_Chunking` | PASS | Polling, chunking at 4096 runes, allowlist drop |
| `internal/gateway/discord` | RED | `TestDiscordAdapter_Send_Chunking`, `TestDiscordAdapter_LifecycleAndIngest` | FAIL | Types undefined |
| `internal/gateway/discord` | GREEN | `TestDiscordAdapter_Send_Chunking`, `TestDiscordAdapter_LifecycleAndIngest` | PASS | Polling, chunking at 2000 runes, allowlist drop |
| `internal/gateway/multiplexer` | RED | `TestMultiplexer_StartStop_MultipleAdapters`, `TestMultiplexer_Send`, `TestMultiplexer_HandleEvent_SessionRoutingAndBrainExecution` | FAIL | Multiplexer undefined |
| `internal/gateway/multiplexer` | GREEN | `TestMultiplexer_StartStop_MultipleAdapters`, `TestMultiplexer_Send`, `TestMultiplexer_HandleEvent_SessionRoutingAndBrainExecution` | PASS | Concurrent adapter mgmt, brain routing |
| `cmd/agis/gateway` | RED | `TestGatewayCLI_HelpAndInvalidFlags`, `TestGatewayCLI_DisabledGateway`, `TestGatewayCLI_RunWithContextCancel` | FAIL | Entrypoints undefined |
| `cmd/agis/gateway` | GREEN | `TestGatewayCLI_HelpAndInvalidFlags`, `TestGatewayCLI_DisabledGateway`, `TestGatewayCLI_RunWithContextCancel` | PASS | Subcommand daemon lifecycle with signals |
| `internal/config` (PR2) | RED | `TestLoad_CronDefaultsAndExplicit` | FAIL | Field `Cron` undefined on `*Config` |
| `internal/config` (PR2) | GREEN | `TestLoad_CronDefaultsAndExplicit` | PASS | `CronConfig`, `CronJobConfig`, `CronTargetConfig` added |
| `internal/cron/scheduler` | RED | `TestParseSchedule_Valid`, `TestParseSchedule_Invalid`, `TestSchedule_NextCalculation`, `TestValidateJob` | FAIL | Package and types undefined |
| `internal/cron/scheduler` | GREEN | `TestParseSchedule_Valid`, `TestParseSchedule_Invalid`, `TestSchedule_NextCalculation`, `TestValidateJob` | PASS | 5-field cron, macros (`@hourly`, etc.), `@every` duration, next calculation |
| `internal/cron/scheduler` | TRIANGULATE | Leap year (Feb 29), step ranges `1-10/3`, `@annually`, `@monthly` | PASS | `go test ./internal/cron/...` passed |
| `internal/cron/engine` | RED | `TestEngine_StartStop_GracefulShutdown`, `TestEngine_AddJob_Validation`, `TestEngine_TriggerExecution_EphemeralSession`, `TestEngine_TriggerExecution_BoundSession`, `TestEngine_NoTarget_LogsOnly`, `TestEngine_BrainError_LoggedGracefully`, `TestEngine_ConcurrentJobs` | FAIL | `NewEngine`, `WithEngine*` undefined |
| `internal/cron/engine` | GREEN | `TestEngine_StartStop_GracefulShutdown`, `TestEngine_AddJob_Validation`, `TestEngine_TriggerExecution_EphemeralSession`, `TestEngine_TriggerExecution_BoundSession`, `TestEngine_NoTarget_LogsOnly`, `TestEngine_BrainError_LoggedGracefully`, `TestEngine_ConcurrentJobs` | PASS | Background loop, `core.Brain.Step` execution, ephemeral/bound session IDs, target dispatch |
| `internal/cron/edge_cases` | TRIANGULATE | `TestEngine_ClosedEngine_AddJob`, `TestEngine_TargetSendError`, `TestEngine_NoBrainOrRepo_SafeHandling`, `TestEngine_JobsList`, `goleak.VerifyTestMain` | PASS | 0 goroutine leaks, clean error isolation |
| `cmd/agis/cron` | RED | `TestCronCLI_HelpAndInvalidFlags`, `TestCronCLI_List_EmptyAndPopulated`, `TestCronCLI_DisabledCron`, `TestCronCLI_RunWithContextCancel` | FAIL | `RunCronCLI` undefined |
| `cmd/agis/cron` | GREEN | `TestCronCLI_HelpAndInvalidFlags`, `TestCronCLI_List_EmptyAndPopulated`, `TestCronCLI_DisabledCron`, `TestCronCLI_RunWithContextCancel` | PASS | `agis cron run` and `agis cron list` subcommands wired to `cmd/agis/main.go` |

---

### Files Changed (PR 2)
- `internal/config/config.go` (Modified: Added `CronConfig`, `CronJobConfig`, `CronTargetConfig`)
- `internal/config/config_test.go` (Modified: Added cron config tests)
- `internal/cron/scheduler.go` (New: `Job`, `Target`, `Scheduler`, `Schedule`, 5-field cron & `@every` duration parser, validator, `Next` calculator)
- `internal/cron/scheduler_test.go` (New: Comprehensive parser and validator unit tests)
- `internal/cron/engine.go` (New: Background scheduler engine, `core.Brain.Step` execution, ephemeral/bound session routing, target dispatch)
- `internal/cron/engine_test.go` (New: Scheduler engine lifecycle, concurrency, mock execution tests)
- `internal/cron/edge_cases_test.go` (New: Goleak verification and error handling tests)
- `cmd/agis/cron.go` (New: `agis cron [run|list]` CLI subcommand implementation)
- `cmd/agis/cron_test.go` (New: CLI unit and integration tests)
- `cmd/agis/main.go` (Modified: Wired `cron` subcommand router)
- `openspec/changes/m6-ecosystem/tasks.md` (Modified: Checked off tasks 2.1 - 2.6)
- `openspec/changes/m6-ecosystem/apply-progress.md` (Modified: Updated progress report)

---

### Verification Commands & Results
- `go test ./internal/cron/...` -> PASS (ok 0.293s)
- `go test ./cmd/agis/...` -> PASS (ok 0.205s)
- `go test -race -count=1 ./...` -> PASS (all packages ok, 0 data races, 0 goroutine leaks)

---

### Deviations from Design
None. Followed ADR D3 (concurrency & lifecycle), D4 (cron scheduling), D7 (config extensions), and D8 (subcommand wiring) precisely.

---

### Remaining Tasks (PR 3, 4)
- [ ] 3.1 Implement config extensions for `plugins` and `webhook` in `internal/config/config.go`. <!-- sdd-owner: implementation -->
- [ ] 3.2 Implement plugin manifest parsing and schema validation in `internal/plugins/manifest.go` (RED → GREEN). <!-- sdd-owner: implementation -->
- [ ] 3.3 Implement Plugin Manager lifecycle (`Load`, `List`, `Enable`, `Disable`, and tool/skill registry bridge) in `internal/plugins/manager.go`. <!-- sdd-owner: implementation -->
- [ ] 3.4 Implement Webhook HTTP server and endpoint routing in `internal/webhook/server.go`. <!-- sdd-owner: implementation -->
- [ ] 3.5 Implement HMAC-SHA256 signature verification using `crypto/subtle.ConstantTimeCompare` in `internal/webhook/server.go` (RED → GREEN). <!-- sdd-owner: implementation -->
- [ ] 3.6 Implement webhook event ingestion and brain dispatching. <!-- sdd-owner: implementation -->
- [ ] 3.7 Add unit tests under `internal/plugins/` and `internal/webhook/` and verify with `go test ./internal/plugins/... ./internal/webhook/...`. <!-- sdd-owner: implementation -->
- [ ] 3.8 Implement Cobra subcommands `agis plugins` and `agis webhook` in `cmd/agis/plugins.go` and `cmd/agis/webhook.go`. <!-- sdd-owner: implementation -->
- [ ] 4.1 Perform end-to-end integration testing across gateway, cron, plugins, and webhooks with `go test ./internal/...`. <!-- sdd-owner: implementation -->
- [ ] 4.2 Verify binary build and CLI subcommands execution with `go build -o bin/agis ./cmd/agis`. <!-- sdd-owner: implementation -->
- [ ] 4.3 Clean up documentation and finalize all ecosystem change artifacts. <!-- sdd-owner: implementation -->
