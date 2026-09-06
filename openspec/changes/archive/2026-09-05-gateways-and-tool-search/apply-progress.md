# Apply Progress: gateways-and-tool-search (Cumulative: Batches 1 & 2)

## Summary of Completed Work

### Work Unit 1: Configuration, Secret Masking & Dynamic Tool Search
- Expanded `GatewayConfig` with `SlackConfig` and `WhatsAppConfig` structs in `internal/config/config.go`.
- Expanded `ToolsConfig` with `ToolSearchConfig` (`Enabled: false`, `Threshold: 8`) in `internal/config/config.go`.
- Added secret masking for `Slack.BotToken`, `Slack.SigningSecret`, `WhatsApp.APIToken`, `WhatsApp.VerifyToken`, and `WhatsApp.AppSecret` in `internal/config/mask.go`.
- Added dot-notation accessor support for Slack, WhatsApp, and ToolSearch in `internal/config/accessor.go`.
- Implemented `ToolSearchRunner` (`internal/tools/tool_search.go`) and `LoadToolRunner` (`internal/tools/load_tool.go`) implementing `core.ToolRunner`.
- Implemented threshold-based schema pruning and dynamic tool expansion in `internal/core/brain.go`.

### Work Unit 2: Slack Gateway Adapter
- Implemented `SlackAdapter` in `internal/gateway/slack.go` satisfying `gateway.Adapter`.
- Implemented constant-time HMAC-SHA256 signature verification (`X-Slack-Signature`) using `crypto/subtle.ConstantTimeCompare`.
- Implemented timestamp freshness validation (< 300s window) for replay protection.
- Handled Slack URL verification handshake (`url_verification`).
- Implemented event callback processing with bot filtering and user allowlist verification.
- Implemented outbound message chunking (max 4000 runes) and Slack Web API `chat.postMessage` delivery.

### Work Unit 3: WhatsApp Gateway Adapter
- Implemented `WhatsAppAdapter` in `internal/gateway/whatsapp.go` satisfying `gateway.Adapter`.
- Implemented Meta Webhook verification handshake (`hub.mode`, `hub.verify_token`, `hub.challenge`) via constant-time token comparison (`crypto/subtle.ConstantTimeCompare`).
- Implemented inbound webhook payload HMAC-SHA256 verification (`X-Hub-Signature-256`) with `crypto/subtle.ConstantTimeCompare`.
- Implemented inbound text and audio/voice message parsing with allowlist verification (`gateway.IsAllowed`) and session mapping (`gateway:whatsapp:<phone>`).
- Implemented Meta Graph API audio media downloading with bounded size enforcement (<= 25MB) and transcription via Whisper (`core.Transcriber`).
- Implemented outbound message chunking (max 4096 runes) and delivery via Meta Graph API (`POST /v21.0/<PhoneNumberID>/messages`).

### Work Unit 4: Multiplexer Integration, Doctor Probes, Main Wiring & Documentation
- Verified multi-adapter lifecycle management and session routing across all 4 adapters (`telegram`, `discord`, `slack`, `whatsapp`) in `internal/gateway/multiplexer.go`.
- Implemented `checkSlackGateway` and `checkWhatsAppGateway` diagnostic probes in `internal/doctor/gateway.go`.
- Implemented `checkToolSearch` diagnostic probe in `internal/doctor/tools.go`.
- Registered probes in `internal/doctor/doctor.go` and verified in test suite.
- Wired Slack, WhatsApp, and dynamic ToolSearch into `cmd/agis/main.go` and `cmd/agis/gateway.go`.
- Updated documentation in `docs/cli.md`, `docs/configuration.md`, `docs/gateway.md`, and `README.md`.

---

## TDD Cycle Evidence

| Phase | Target | Test File | Impl File | Status | Notes |
|-------|--------|-----------|-----------|--------|-------|
| RED | Config & Masking | `internal/config/config_test.go`, `mask_test.go`, `accessor_test.go` | `internal/config/config.go`, `mask.go` | PASS | Tests failed on missing fields/methods |
| GREEN | Config & Masking | `internal/config/config_test.go`, `mask_test.go`, `accessor_test.go` | `internal/config/config.go`, `mask.go` | PASS | All config tests passing |
| RED | Tool Search & Loading | `internal/tools/tool_search_test.go`, `load_tool_test.go` | `internal/tools/tool_search.go`, `load_tool.go` | PASS | Tests failed on missing runners |
| GREEN | Tool Search & Loading | `internal/tools/tool_search_test.go`, `load_tool_test.go` | `internal/tools/tool_search.go`, `load_tool.go` | PASS | All tool search/load tests passing |
| RED | Brain Schema Pruning | `internal/core/brain_tools_test.go` | `internal/core/brain.go` | PASS | Tests failed on missing `WithToolSearch` |
| GREEN | Brain Schema Pruning | `internal/core/brain_tools_test.go` | `internal/core/brain.go` | PASS | Pruning and dynamic tool loading verified |
| RED | Slack Gateway Adapter | `internal/gateway/slack_test.go` | `internal/gateway/slack.go` | PASS | Tests failed on missing `NewSlackAdapter` |
| GREEN | Slack Gateway Adapter | `internal/gateway/slack_test.go` | `internal/gateway/slack.go` | PASS | Full lifecycle, HMAC verification, chunking pass |
| RED | WhatsApp Gateway Adapter | `internal/gateway/whatsapp_test.go` | `internal/gateway/whatsapp.go` | PASS | Tests failed on missing `NewWhatsAppAdapter` |
| GREEN | WhatsApp Gateway Adapter | `internal/gateway/whatsapp_test.go` | `internal/gateway/whatsapp.go` | PASS | Full lifecycle, GET challenge, POST HMAC, voice transcription & chunking pass |
| RED | Doctor Probes | `internal/doctor/gateway_test.go`, `tools_test.go` | `internal/doctor/gateway.go`, `tools.go` | PASS | Tests failed on missing `checkSlackGateway`, `checkWhatsAppGateway`, `checkToolSearch` |
| GREEN | Doctor Probes | `internal/doctor/gateway_test.go`, `tools_test.go`, `doctor_test.go` | `internal/doctor/gateway.go`, `tools.go`, `doctor.go` | PASS | All doctor probes passing with PASS/FAIL/WARN checks |
| RED | Gateway CLI Wiring | `cmd/agis/gateway_test.go` | `cmd/agis/gateway.go`, `main.go` | PASS | Tested 4-adapter daemon initialization & shutdown |
| GREEN | Gateway CLI Wiring | `cmd/agis/gateway_test.go` | `cmd/agis/gateway.go`, `main.go` | PASS | Daemon cleanly runs & stops with Slack & WhatsApp |

---

## Files Changed

- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/config/mask.go`
- `internal/config/mask_test.go`
- `internal/config/accessor_test.go`
- `internal/tools/tool_search.go`
- `internal/tools/tool_search_test.go`
- `internal/tools/load_tool.go`
- `internal/tools/load_tool_test.go`
- `internal/core/brain.go`
- `internal/core/brain_tools_test.go`
- `internal/gateway/slack.go`
- `internal/gateway/slack_test.go`
- `internal/gateway/whatsapp.go`
- `internal/gateway/whatsapp_test.go`
- `internal/gateway/multiplexer_test.go`
- `internal/doctor/gateway.go`
- `internal/doctor/gateway_test.go`
- `internal/doctor/tools.go`
- `internal/doctor/tools_test.go`
- `internal/doctor/doctor.go`
- `internal/doctor/doctor_test.go`
- `cmd/agis/main.go`
- `cmd/agis/gateway.go`
- `cmd/agis/gateway_test.go`
- `docs/cli.md`
- `docs/configuration.md`
- `docs/gateway.md`
- `README.md`
- `openspec/changes/gateways-and-tool-search/tasks.md`
- `openspec/changes/gateways-and-tool-search/apply-progress.md`

---

## Test Commands Run

- `go test -race -count=1 ./internal/config/...` -> PASS (0 race conditions)
- `go test -race -count=1 ./internal/tools/...` -> PASS (0 race conditions)
- `go test -race -count=1 ./internal/core/...` -> PASS (0 race conditions)
- `go test -race -count=1 ./internal/gateway/...` -> PASS (0 race conditions)
- `go test -race -count=1 ./internal/doctor/...` -> PASS (0 race conditions)
- `go test -race -count=1 ./cmd/agis/...` -> PASS (0 race conditions)
- `go vet ./...` -> PASS (0 issues)
- `go test -race -count=1 ./...` -> PASS (all packages in repository passing cleanly)

---

## Deviations from Design

None. Implementation strictly followed specifications GTW-SLK-001 through GTW-SLK-004, GTW-WHA-001 through GTW-WHA-004, TLS-SRC-001 through TLS-SRC-004, CFG-GTW-001, CFG-SEC-001, and DOC-GTW-001 through DOC-TLS-001.

---

## Remaining Tasks (Next Slices)

None. All implementation tasks across Work Units 1, 2, 3, and 4 are 100% complete.
