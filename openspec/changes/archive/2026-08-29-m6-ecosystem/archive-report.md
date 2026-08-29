# SDD Archive Report: m6-ecosystem

## Milestone 6 — Ecosystem (Gateway, Cron, Plugins, Webhooks)

- **Change Name:** `m6-ecosystem`
- **Archived Date:** 2026-08-29
- **Final Status:** COMPLETED & MERGED
- **Store Mode:** Hybrid (OpenSpec + Engram)
- **TDD Mode:** Strict TDD (100% verified across 16 packages)

---

## 1. Executive Summary

Milestone 6 expands AGIS into an extensible, multi-interface, event-driven autonomous agent. It delivers:
1. **Chat Gateway Multiplexer & Adapters (`internal/gateway/`)**: Telegram (4096-char UTF-8 chunking) and Discord (2000-char chunking) with static user allowlists, sandbox `AutoDenyApprover` policy, and session routing via `SessionManager` & `Brain.Step`.
2. **Cron Scheduler Engine (`internal/cron/`)**: Background periodic scheduler supporting 5-field cron syntax and `@every <duration>` intervals, executing prompts in sandbox sessions, and routing outputs to Gateway notification targets.
3. **Plugin Manager (`internal/plugins/`)**: Discovery and lifecycle management (`Load`, `List`, `Enable`, `Disable`) for `plugin.json` manifests, bridging tools via `core.ToolRunner` and registering markdown skills into the Skill Hub with atomic `state.json` persistence.
4. **Webhook Listener (`internal/webhook/`)**: `net/http` endpoint accepting JSON events, verifying authenticity using constant-time `HMAC-SHA256` (`crypto/subtle.ConstantTimeCompare`), enforcing 1MB body limits, and dispatching events into `Brain.Step`.
5. **Config Loader & CLI (`internal/config/`, `cmd/agis/`)**: Ecosystem YAML configuration schema with safe zero defaults and Cobra CLI subcommands (`agis gateway run`, `agis cron run/list`, `agis plugins list/enable/disable/inspect`, `agis webhook run`).

---

## 2. Pull Request Delivery Sequence (Stacked to Main)

| PR | Title | Commits | Lines Changed | Status |
|---|---|---|---|---|
| **#21** | `feat(gateway): M6 PR1 — gateway substrate, multiplexer and adapters` | `a110df4` | +3,020 / -13 | Merged |
| **#22** | `feat(cron): M6 PR2 — cron scheduler engine and target delivery` | `6e49c1c` | +2,092 / -39 | Merged |
| **#23** | `feat(plugins,webhook): M6 PR3 — plugin manager and webhook listener with HMAC` | `892718e` | +2,483 / -36 | Merged |
| **#24** | `docs(m6): M6 PR4 — ecosystem integration tests and documentation` | `3e80db1` | +769 / -142 | Merged |

---

## 3. Capabilities Synced to `openspec/specs/`

- `gateway/spec.md` (NEW): Gateway Multiplexer, Telegram adapter, Discord adapter, allowlist, sandbox AutoDeny, session routing.
- `cron/spec.md` (NEW): Cron scheduler engine, cron expression parser, Brain prompt execution, gateway notification target delivery.
- `plugins/spec.md` (NEW): Plugin manifest schema, plugin lifecycle manager, tool runner & skill hub bridges.
- `webhook/spec.md` (NEW): Webhook HTTP server, HMAC-SHA256 constant-time verification, event ingestion & dispatch.
- `cli/spec.md` (NEW): Cobra CLI subcommands for gateway, cron, plugins, and webhooks.
- `config-loader/spec.md` (MODIFIED): Ecosystem YAML configuration blocks and default values.

---

## 4. Verification Evidence

- `go test -race -count=1 ./...` PASSED across all 16 packages:
  `cmd/agis`, `internal/adapters/llm`, `internal/adapters/tui`, `internal/config`, `internal/core`, `internal/cron`, `internal/gateway`, `internal/memory`, `internal/persona`, `internal/plugins`, `internal/policy`, `internal/scan`, `internal/session`, `internal/skills`, `internal/tools`, `internal/webhook`.
- 0 data races detected.
- 0 goroutine leaks confirmed via `go.uber.org/goleak`.
- Static binary compilation verified (`go build -o /dev/null ./cmd/agis`).
