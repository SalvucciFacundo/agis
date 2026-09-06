# Archive Report: gateways-and-tool-search

## Change Overview
- **Name**: `gateways-and-tool-search`
- **Archived Date**: 2026-09-05
- **Status**: Completed & Archived
- **Mode**: Automatic (`auto`)
- **Artifact Store**: Hybrid (`openspec/` + Engram)
- **Delivery Strategy**: `auto-chain` (`stacked-to-main`)

## Summary of Accomplishments
1. **Dynamic Tool Search & Lazy Schema Loading (`internal/tools`, `internal/core`)**:
   - Implemented `ToolSearchRunner` (`tool_search`) and `LoadToolRunner` (`load_tool`) satisfying `core.ToolRunner`.
   - Implemented threshold-based schema pruning in `core.Brain`: when total registered tools exceed the threshold (default 8), the Brain presents only Core Tools (`web_search`, `web_fetch`, `delegate_task`, `local`) + `tool_search` + `load_tool`, dramatically reducing prompt tokens.
   - Dynamic schema injection: `load_tool` loads the full JSON schema of requested non-core tools on demand during multi-round turns.
2. **Slack Gateway Adapter (`internal/gateway/slack.go`)**:
   - Built Slack Events API adapter with constant-time `X-Slack-Signature` HMAC verification (`crypto/subtle.ConstantTimeCompare`) and timestamp freshness check (< 300s).
   - Handled URL verification challenge handshake, event callbacks with bot filtering, thread replies (`thread_ts`), user allowlists, and rune-based 4000-character chunking.
3. **WhatsApp Gateway Adapter (`internal/gateway/whatsapp.go`)**:
   - Built Meta Cloud API webhook adapter with GET verification handshake and POST `X-Hub-Signature-256` HMAC validation.
   - Handled inbound text and voice note audio downloads with Whisper transcription (`core.Transcriber`), user allowlists, and 4096-character rune chunking.
4. **Multiplexer, Diagnostics & Documentation (`internal/gateway`, `internal/doctor`, `docs/`)**:
   - Integrated Slack and WhatsApp into the concurrent `gateway.Multiplexer` alongside Telegram and Discord.
   - Added diagnostic probes `checkSlackGateway`, `checkWhatsAppGateway`, and `checkToolSearch` in `internal/doctor`.
   - Updated `docs/cli.md`, `docs/configuration.md`, `docs/gateway.md`, and `README.md`.
   - Synced master specification to `openspec/specs/gateways/spec.md`.

## Verification Results
- **Strict TDD Compliance**: 100% verified across all 4 work units.
- **Specification Requirements**: 18/18 requirements and scenarios satisfied (PASS).
- **Test Suite**: 26/26 Go packages passing with `go test -race -count=1 ./...` and clean `go vet ./...` (zero races, zero leaks, zero vet warnings).

## Final State Facts
- Packages added/modified: `internal/config`, `internal/tools`, `internal/core`, `internal/gateway`, `internal/doctor`, `cmd/agis`.
- Artifacts archived to: `openspec/changes/archive/2026-09-05-gateways-and-tool-search/`
- Master spec at: `openspec/specs/gateways/spec.md`
