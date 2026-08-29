# Apply Progress: m6-ecosystem

## PR Slice: PR 1 — Gateway Substrate, Multiplexer & Adapters

### Status
- Mode: auto-chain
- Strategy: stacked-to-main
- Current PR: PR 1 (Gateway Substrate, Multiplexer & Adapters)
- Work Unit: Tasks 1.1 - 1.8
- Outcome: Completed & Verified

---

### Completed Tasks
- [x] 1.1 Implement config extensions for `gateway` in `internal/config/config.go`.
- [x] 1.2 Implement `internal/gateway/adapter.go` defining the `Adapter` interface and static allowlist validation (RED → GREEN).
- [x] 1.3 Implement non-interactive `AutoDenyApprover` in `internal/gateway/approver.go` to auto-deny `DecisionAsk` under sandbox policy (RED → GREEN).
- [x] 1.4 Implement Telegram adapter in `internal/gateway/telegram.go` with polling/webhook updates, 4096-char chunking, and session routing.
- [x] 1.5 Implement Discord adapter in `internal/gateway/discord.go` with message creation listeners, 2000-char splitting, and session routing.
- [x] 1.6 Implement `internal/gateway/multiplexer.go` to orchestrate adapters concurrently and handle graceful shutdown via `context.Context`.
- [x] 1.7 Add unit tests and integration tests under `internal/gateway/` and verify with `go test ./internal/gateway/...`.
- [x] 1.8 Implement Cobra subcommand `agis gateway` in `cmd/agis/gateway.go`.

---

### TDD Cycle Evidence

| Unit / Component | Phase | Test Case | Status | Evidence |
|---|---|---|---|---|
| `internal/config` | RED | `TestLoad_GatewayDefaultsAndExplicit` | FAIL | Field undefined on `*Config` |
| `internal/config` | GREEN | `TestLoad_GatewayDefaultsAndExplicit` | PASS | `GatewayConfig` added with safe zero defaults |
| `internal/config` | TRIANGULATE | Table-driven partial configs & defaults | PASS | `go test ./internal/config/...` ok |
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

---

### Files Changed
- `internal/config/config.go` (Modified: Added `GatewayConfig`, `TelegramConfig`, `DiscordConfig`)
- `internal/config/config_test.go` (Modified: Added gateway config tests)
- `internal/gateway/adapter.go` (New: `Adapter` interface, `MessageEvent`, `IsAllowed`)
- `internal/gateway/adapter_test.go` (New: Allowlist unit tests)
- `internal/gateway/approver.go` (New: `NewAutoDenyApprover` implementation)
- `internal/gateway/approver_test.go` (New: Auto-deny tests)
- `internal/gateway/telegram.go` (New: Telegram adapter with polling and 4096-char chunking)
- `internal/gateway/telegram_test.go` (New: Telegram adapter tests)
- `internal/gateway/discord.go` (New: Discord adapter with polling and 2000-char splitting)
- `internal/gateway/discord_test.go` (New: Discord adapter tests)
- `internal/gateway/multiplexer.go` (New: Gateway Multiplexer orchestrating adapters and Brain execution)
- `internal/gateway/multiplexer_test.go` (New: Multiplexer unit tests)
- `internal/gateway/edge_cases_test.go` (New: Goroutine leak tests with goleak & edge cases)
- `cmd/agis/gateway.go` (New: `agis gateway [run]` subcommand daemon)
- `cmd/agis/gateway_test.go` (New: CLI tests with signal handling)
- `cmd/agis/main.go` (Modified: Wired `gateway` subcommand into CLI router)
- `openspec/changes/m6-ecosystem/tasks.md` (Modified: Marked tasks 1.1 - 1.8 as checked)

---

### Verification Commands & Results
- `go test ./internal/gateway/...` -> PASS (ok 0.052s)
- `go test -race ./...` -> PASS (ok for all packages, 0 data races, 0 goroutine leaks)

---

### Remaining Tasks (PR 2, 3, 4)
- [ ] 2.1 Implement config extensions for `cron` in `internal/config/config.go`. <!-- sdd-owner: implementation -->
- [ ] 2.2 Implement cron expression and duration interval parser with validation in `internal/cron/scheduler.go` (RED → GREEN). <!-- sdd-owner: implementation -->
- [ ] 2.3 Implement background scheduler engine in `internal/cron/engine.go` executing prompts via `core.Brain.Step` with sandbox policy and session binding. <!-- sdd-owner: implementation -->
- [ ] 2.4 Implement cron target notification delivery forwarding outputs to the Gateway Multiplexer or logger. <!-- sdd-owner: implementation -->
- [ ] 2.5 Add unit tests under `internal/cron/` and verify with `go test ./internal/cron/...`. <!-- sdd-owner: implementation -->
- [ ] 2.6 Implement Cobra subcommands `agis cron run` and `agis cron list` in `cmd/agis/cron.go`. <!-- sdd-owner: implementation -->
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
