# Apply Progress: m6-ecosystem

## PR Slice: PR 3 — Plugin Manager & Webhook Listener (HMAC verification)

### Status
- Mode: auto-chain
- Strategy: stacked-to-main
- Current PR: PR 3 (Plugin Manager & Webhook Listener with HMAC verification)
- Work Unit: Tasks 3.1 - 3.8
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

#### PR 3 (Plugin Manager & Webhook Listener)
- [x] 3.1 Implement config extensions for `plugins` and `webhook` in `internal/config/config.go`.
- [x] 3.2 Implement plugin manifest parsing and schema validation in `internal/plugins/manifest.go` (RED → GREEN).
- [x] 3.3 Implement Plugin Manager lifecycle (`Load`, `List`, `Enable`, `Disable`, and tool/skill registry bridge) in `internal/plugins/manager.go`.
- [x] 3.4 Implement Webhook HTTP server and endpoint routing in `internal/webhook/server.go`.
- [x] 3.5 Implement HMAC-SHA256 signature verification using `crypto/subtle.ConstantTimeCompare` in `internal/webhook/server.go` (RED → GREEN).
- [x] 3.6 Implement webhook event ingestion and brain dispatching.
- [x] 3.7 Add unit tests under `internal/plugins/` and `internal/webhook/` and verify with `go test ./internal/plugins/... ./internal/webhook/...`.
- [x] 3.8 Implement Cobra subcommands `agis plugins` and `agis webhook` in `cmd/agis/plugins.go` and `cmd/agis/webhook.go`.

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
| `internal/config` (PR3) | RED | `TestLoad_PluginsDefaultsAndExplicit`, `TestLoad_WebhookDefaultsAndExplicit` | FAIL | Fields `Plugins` and `Webhook` undefined on `*Config` |
| `internal/config` (PR3) | GREEN | `TestLoad_PluginsDefaultsAndExplicit`, `TestLoad_WebhookDefaultsAndExplicit` | PASS | `PluginsConfig`, `WebhookConfig`, `WebhookTargetConfig` added with safe defaults |
| `internal/plugins/manifest` | RED | `TestParseManifest_Valid`, `TestParseManifest_Invalid`, `TestParseManifestFile` | FAIL | Package `internal/plugins` undefined |
| `internal/plugins/manifest` | GREEN | `TestParseManifest_Valid`, `TestParseManifest_Invalid`, `TestParseManifestFile` | PASS | JSON manifest parser, name regex validation (`^[a-z0-9-_]+$`), version and tool name validation |
| `internal/plugins/manager` | RED | `TestManager_LoadAndList`, `TestManager_EnableDisableAndPersistence`, `TestManager_ToolRunners`, `TestManager_Skills` | FAIL | `NewManager`, `WithStateDir` undefined |
| `internal/plugins/manager` | GREEN | `TestManager_LoadAndList`, `TestManager_EnableDisableAndPersistence`, `TestManager_ToolRunners`, `TestManager_Skills` | PASS | Dynamic discovery, `state.json` persistence, `core.ToolRunner` bridge (`Backend()`, `Run()`), skill extraction |
| `internal/plugins/edge_cases` | TRIANGULATE | `TestManager_EdgeCases`, `goleak.VerifyTestMain` | PASS | 0 leaks, robust missing directory and missing entrypoint handling |
| `internal/webhook/server` | RED | `TestVerifySignature`, `TestWebhookServer_HTTPHandler`, `TestWebhookServer_LifecycleGracefulShutdown` | FAIL | Package `internal/webhook` undefined |
| `internal/webhook/server` | GREEN | `TestVerifySignature`, `TestWebhookServer_HTTPHandler`, `TestWebhookServer_LifecycleGracefulShutdown` | PASS | HMAC-SHA256 signature verification via `ConstantTimeCompare`, POST routing (405 on non-POST), Brain dispatch, target notification forwarding, graceful shutdown |
| `internal/webhook/edge_cases` | TRIANGULATE | `TestWebhookServer_EdgeCases`, `goleak.VerifyTestMain` | PASS | 0 leaks, unstarted server stop safety, error sender handling, missing repo fallback |
| `cmd/agis/plugins` & `cmd/agis/webhook` | RED | `TestPluginsCLI_Help`, `TestPluginsCLI_ListEnableDisableInspect`, `TestWebhookCLI_Help`, `TestWebhookCLI_DisabledWebhook`, `TestWebhookCLI_RunWithContextCancel` | FAIL | `RunPluginsCLI`, `RunWebhookCLI` undefined |
| `cmd/agis/plugins` & `cmd/agis/webhook` | GREEN | `TestPluginsCLI_Help`, `TestPluginsCLI_ListEnableDisableInspect`, `TestWebhookCLI_Help`, `TestWebhookCLI_DisabledWebhook`, `TestWebhookCLI_RunWithContextCancel` | PASS | `agis plugins [list|enable|disable|inspect]` and `agis webhook [run]` wired to `cmd/agis/main.go` |

---

### Files Changed (PR 3)
- `internal/config/config.go` (Modified: Added `PluginsConfig`, `WebhookConfig`, `WebhookTargetConfig`)
- `internal/config/config_test.go` (Modified: Added plugins and webhook config tests)
- `internal/plugins/manifest.go` (New: `Manifest`, `Tool`, `ParseManifest`, `ParseManifestFile`, `Validate`)
- `internal/plugins/manifest_test.go` (New: Manifest parsing and validation unit tests)
- `internal/plugins/manager.go` (New: Plugin `Manager`, `PluginInfo`, `PluginRunner`, `state.json` persistence, skill extraction)
- `internal/plugins/manager_test.go` (New: Lifecycle, state persistence, tool runners, and skills unit tests)
- `internal/plugins/edge_cases_test.go` (New: Goleak verification and edge case tests)
- `internal/webhook/server.go` (New: Webhook HTTP `Server`, HMAC-SHA256 verification, event ingestion, `Brain.Step` dispatch, target delivery)
- `internal/webhook/server_test.go` (New: Webhook handler, signature verification, and lifecycle unit tests)
- `internal/webhook/edge_cases_test.go` (New: Webhook edge case and error handling tests)
- `cmd/agis/plugins.go` (New: `agis plugins [list|enable|disable|inspect]` CLI subcommand)
- `cmd/agis/plugins_test.go` (New: Plugins CLI unit tests)
- `cmd/agis/webhook.go` (New: `agis webhook [run]` CLI subcommand)
- `cmd/agis/webhook_test.go` (New: Webhook CLI unit tests)
- `cmd/agis/main.go` (Modified: Wired `plugins` and `webhook` subcommands router)
- `openspec/changes/m6-ecosystem/tasks.md` (Modified: Checked off tasks 3.1 - 3.8)
- `openspec/changes/m6-ecosystem/apply-progress.md` (Modified: Merged progress report for PR 1, PR 2, and PR 3)

---

### Verification Commands & Results
- `go test ./internal/plugins/...` -> PASS (ok 0.003s)
- `go test ./internal/webhook/...` -> PASS (ok 0.057s)
- `go test ./cmd/agis/...` -> PASS (ok 0.309s)
- `go test -race -count=1 ./...` -> PASS (all packages ok, 0 data races, 0 goroutine leaks)

---

### Deviations from Design
None. Followed ADR D5 (Plugin Discovery), D6 (Webhook Security & HMAC), D7 (Config Extensions), and D8 (Subcommand Wiring) precisely.

---

### Remaining Tasks (PR 4)
- [ ] 4.1 Perform end-to-end integration testing across gateway, cron, plugins, and webhooks with `go test ./internal/...`. <!-- sdd-owner: implementation -->
- [ ] 4.2 Verify binary build and CLI subcommands execution with `go build -o bin/agis ./cmd/agis`. <!-- sdd-owner: implementation -->
- [ ] 4.3 Clean up documentation and finalize all ecosystem change artifacts. <!-- sdd-owner: implementation -->
