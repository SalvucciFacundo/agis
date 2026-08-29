# Milestone 6 — Ecosystem: Gateway (Telegram + Discord), Cron Scheduler, Plugin Manager, and Webhooks

This change expands AGIS into a multi-interface, event-driven agent. It enables AGIS to interact with external platforms, perform scheduled background jobs, ingest external events securely, and extend its capabilities via an external plugin manager.

## Capabilities Contract

| Area | Component | Description |
|---|---|---|
| **NEW** | `gateway` | Multiplexer + Telegram & Discord adapters. Uses static user ID allowlist, sandbox-only auto-deny policy, and session routing via `session.SessionManager` and `core.Brain`. |
| **NEW** | `cron` | Background scheduler for periodic jobs configured via `config.yaml`. Supports session binding, execution through `Brain.Step`, and delivery to gateway targets. |
| **NEW** | `plugins` | Plugin manifest and lifecycle manager (`internal/plugins/`) for loading external tool and skill bundles. |
| **NEW** | `webhook` | HTTP server endpoint (`internal/webhook/`) with HMAC-SHA256 signature verification for external event ingestion. |
| **MODIFIED** | `config-loader` | Extends `internal/config/config.go` with `gateway`, `cron`, `plugins`, and `webhook` configuration structs and defaults. |
| **MODIFIED** | `cli` | Adds CLI subcommands in `cmd/agis/` for gateway, cron, webhook, and plugin management (`agis gateway`, `agis cron`, etc.). |

## Security & Guardrails

- **Identity Validation**: Static user ID allowlist for Telegram and Discord; unauthorized senders are ignored or rejected.
- **Policy Guard Enforcement**: Gateway sessions run under the `sandbox` tier by default with no interactive prompts. This triggers an auto-deny on anything above sandbox unless a persistent `always` rule exists.
- **Payload Verification**: Webhook HTTP server enforces HMAC-SHA256 verification using a secret token.

## Compatibility & Rollback

- **Compatibility**: Zero breaking changes to the existing TUI, CLI `agis` commands, or database schema.
- **Rollback**: Clean rollback achieved by dropping the new packages/subcommands.
