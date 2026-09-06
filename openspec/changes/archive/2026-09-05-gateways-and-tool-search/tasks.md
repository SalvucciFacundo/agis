# Tasks: Messaging Gateways & Dynamic Tool Search (gateways-and-tool-search)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1,450 - 1,850 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Core Config, Masking & Dynamic Tool Search) → PR 2 (Slack Gateway Adapter) → PR 3 (WhatsApp Gateway Adapter) → PR 4 (Multiplexer Integration, Doctor Probes, Wiring & Docs) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

---

## Work Units

### Work Unit 1: Configuration, Secret Masking & Dynamic Tool Search
- [x] Implement and verify `ToolSearchConfig` and expanded `GatewayConfig` (`SlackConfig`, `WhatsAppConfig`) in `internal/config/config.go`. <!-- sdd-owner: implementation -->
- [x] Implement and verify secret masking for Slack and WhatsApp tokens/secrets in `internal/config/mask.go`. <!-- sdd-owner: implementation -->
- [x] Implement and verify dot-notation config getters and setters for gateway and tool search settings in `internal/config/accessor.go`. <!-- sdd-owner: implementation -->
- [x] Implement and verify tool metadata registry and indexing in `internal/tools/registry.go` (or `internal/core/registry.go`). <!-- sdd-owner: implementation -->
- [x] Implement RED test, then GREEN implementation, then REFACTOR for `tool_search` native tool runner in `internal/tools/tool_search.go`. <!-- sdd-owner: implementation -->
- [x] Implement RED test, then GREEN implementation, then REFACTOR for `load_tool` native tool runner in `internal/tools/load_tool.go`. <!-- sdd-owner: implementation -->
- [x] Implement RED test, then GREEN implementation, then REFACTOR for threshold-based schema pruning and dynamic tool expansion in `internal/core/brain.go`. <!-- sdd-owner: implementation -->
- [x] Run unit tests for config, mask, tools, and brain modules with `-race` and verify zero race conditions or test failures. <!-- sdd-owner: implementation -->

### Work Unit 2: Slack Gateway Adapter
- [x] Implement RED test, then GREEN implementation, then REFACTOR for Slack gateway configuration validation and lifecycle (`Start`, `Stop`, `Send`) in `internal/gateway/slack.go`. <!-- sdd-owner: implementation -->
- [x] Implement constant-time HMAC-SHA256 signature verification (`X-Slack-Signature`) and timestamp freshness check (< 300s) using `crypto/subtle.ConstantTimeCompare` in `internal/gateway/slack.go`. <!-- sdd-owner: implementation -->
- [x] Implement Slack Events API URL verification handshake (`url_verification`) and event callback message parsing with allowlist filtering and session mapping (`gateway:slack:<channel>:<user>`) in `internal/gateway/slack.go`. <!-- sdd-owner: implementation -->
- [x] Implement outbound rune-based message chunking (`SplitMessage`, max 4000 runes) and thread-bound replies (`thread_ts`) via Slack Web API `chat.postMessage` in `internal/gateway/slack.go`. <!-- sdd-owner: implementation -->
- [x] Write comprehensive unit tests with `httptest` covering valid/invalid signatures, expired timestamps, challenge handoffs, message chunking, and mock API delivery. <!-- sdd-owner: implementation -->
- [x] Run `go test -race ./internal/gateway/...` and ensure all Slack gateway tests pass cleanly. <!-- sdd-owner: implementation -->

### Work Unit 3: WhatsApp Gateway Adapter
- [x] Implement RED test, then GREEN implementation, then REFACTOR for WhatsApp gateway configuration and lifecycle (`Start`, `Stop`, `Send`) in `internal/gateway/whatsapp.go`. <!-- sdd-owner: implementation -->
- [x] Implement Meta Webhook GET verification handshake (`hub.mode`, `hub.verify_token`, `hub.challenge`) and POST payload HMAC verification (`X-Hub-Signature-256`) using `crypto/subtle.ConstantTimeCompare` in `internal/gateway/whatsapp.go`. <!-- sdd-owner: implementation -->
- [x] Implement WhatsApp message parsing for text and voice notes/audio (including Meta Graph API media downloading, size limit checks, and audio transcription via `core.Transcriber`) with allowlist verification and session mapping (`gateway:whatsapp:<phone>`) in `internal/gateway/whatsapp.go`. <!-- sdd-owner: implementation -->
- [x] Implement outbound rune-based message chunking (`SplitMessage`, max 4096 runes) and delivery via Meta Cloud API (`https://graph.facebook.com/v21.0/...`) in `internal/gateway/whatsapp.go`. <!-- sdd-owner: implementation -->
- [x] Write comprehensive unit tests with `httptest` covering webhook handshakes, signature validation, text/audio message processing, transcription, and outbound delivery. <!-- sdd-owner: implementation -->
- [x] Run `go test -race ./internal/gateway/...` and ensure zero failures. <!-- sdd-owner: implementation -->

### Work Unit 4: Multiplexer Integration, Doctor Probes, Main Wiring & Documentation
- [x] Implement and verify Slack and WhatsApp adapter initialization within the Multiplexer in `internal/gateway/multiplexer.go`. <!-- sdd-owner: implementation -->
- [x] Implement RED test, then GREEN implementation, then REFACTOR for `checkSlackGateway`, `checkWhatsAppGateway`, and `checkToolSearch` diagnostic probes in `internal/doctor/gateway.go` and `internal/doctor/tools.go`. <!-- sdd-owner: implementation -->
- [x] Wire gateway listeners and doctor checks into `cmd/agis/main.go` and `cmd/agis/gateway.go`. <!-- sdd-owner: implementation -->
- [x] Update documentation in `docs/cli.md`, `docs/configuration.md`, and `README.md` covering Slack and WhatsApp gateway configuration and dynamic tool search settings. <!-- sdd-owner: implementation -->
- [x] Run complete test suite across all packages (`go test -race ./...`) and verify all tests pass without failures or goroutine leaks. <!-- sdd-owner: implementation -->
