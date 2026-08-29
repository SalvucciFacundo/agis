## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1800 - 2500 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 → PR 4 |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

---

## Task Breakdown

### PR 1: Gateway Substrate, Multiplexer & Adapters (Telegram, Discord, Allowlist, Auto-deny approver)
- [x] 1.1 Implement config extensions for `gateway` in `internal/config/config.go`. <!-- sdd-owner: implementation -->
- [x] 1.2 Implement `internal/gateway/adapter.go` defining the `Adapter` interface and static allowlist validation (RED → GREEN). <!-- sdd-owner: implementation -->
- [x] 1.3 Implement non-interactive `AutoDenyApprover` in `internal/gateway/approver.go` to auto-deny `DecisionAsk` under sandbox policy (RED → GREEN). <!-- sdd-owner: implementation -->
- [x] 1.4 Implement Telegram adapter in `internal/gateway/telegram.go` with polling/webhook updates, 4096-char chunking, and session routing. <!-- sdd-owner: implementation -->
- [x] 1.5 Implement Discord adapter in `internal/gateway/discord.go` with message creation listeners, 2000-char splitting, and session routing. <!-- sdd-owner: implementation -->
- [x] 1.6 Implement `internal/gateway/multiplexer.go` to orchestrate adapters concurrently and handle graceful shutdown via `context.Context`. <!-- sdd-owner: implementation -->
- [x] 1.7 Add unit tests and integration tests under `internal/gateway/` and verify with `go test ./internal/gateway/...`. <!-- sdd-owner: implementation -->
- [x] 1.8 Implement Cobra subcommand `agis gateway` in `cmd/agis/gateway.go`. <!-- sdd-owner: implementation -->

### PR 2: Cron Scheduler Engine & Delivery Target
- [x] 2.1 Implement config extensions for `cron` in `internal/config/config.go`. <!-- sdd-owner: implementation -->
- [x] 2.2 Implement cron expression and duration interval parser with validation in `internal/cron/scheduler.go` (RED → GREEN). <!-- sdd-owner: implementation -->
- [x] 2.3 Implement background scheduler engine in `internal/cron/engine.go` executing prompts via `core.Brain.Step` with sandbox policy and session binding. <!-- sdd-owner: implementation -->
- [x] 2.4 Implement cron target notification delivery forwarding outputs to the Gateway Multiplexer or logger. <!-- sdd-owner: implementation -->
- [x] 2.5 Add unit tests under `internal/cron/` and verify with `go test ./internal/cron/...`. <!-- sdd-owner: implementation -->
- [x] 2.6 Implement Cobra subcommands `agis cron run` and `agis cron list` in `cmd/agis/cron.go`. <!-- sdd-owner: implementation -->

### PR 3: Plugin Manager & Webhook Listener (HMAC verification)
- [x] 3.1 Implement config extensions for `plugins` and `webhook` in `internal/config/config.go`. <!-- sdd-owner: implementation -->
- [x] 3.2 Implement plugin manifest parsing and schema validation in `internal/plugins/manifest.go` (RED → GREEN). <!-- sdd-owner: implementation -->
- [x] 3.3 Implement Plugin Manager lifecycle (`Load`, `List`, `Enable`, `Disable`, and tool/skill registry bridge) in `internal/plugins/manager.go`. <!-- sdd-owner: implementation -->
- [x] 3.4 Implement Webhook HTTP server and endpoint routing in `internal/webhook/server.go`. <!-- sdd-owner: implementation -->
- [x] 3.5 Implement HMAC-SHA256 signature verification using `crypto/subtle.ConstantTimeCompare` in `internal/webhook/server.go` (RED → GREEN). <!-- sdd-owner: implementation -->
- [x] 3.6 Implement webhook event ingestion and brain dispatching. <!-- sdd-owner: implementation -->
- [x] 3.7 Add unit tests under `internal/plugins/` and `internal/webhook/` and verify with `go test ./internal/plugins/... ./internal/webhook/...`. <!-- sdd-owner: implementation -->
- [x] 3.8 Implement Cobra subcommands `agis plugins` and `agis webhook` in `cmd/agis/plugins.go` and `cmd/agis/webhook.go`. <!-- sdd-owner: implementation -->

### PR 4: Config Extensions, CLI Subcommands, End-to-End Verification & Docs
- [x] 4.1 Perform end-to-end integration testing across gateway, cron, plugins, and webhooks with `go test ./...` and `go test -race ./...`. <!-- sdd-owner: implementation -->
- [x] 4.2 Verify binary build and CLI subcommands execution with `go build -o bin/agis ./cmd/agis`. <!-- sdd-owner: implementation -->
- [x] 4.3 Update repository documentation and finalize all ecosystem change artifacts. <!-- sdd-owner: implementation -->
