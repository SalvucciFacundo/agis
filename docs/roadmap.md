# Roadmap

Milestone scopes come from `spec.md` §Milestones. M1 and M2 are shipped; M3–M6 are planned. The archived M1 requirements live as synced OpenSpec capability specs under `openspec/specs/` (`config-loader`, `repository-memory`, `llm-provider-port`, `brain-loop`, `minimal-tui`), joined by the M2 capabilities (`memory-curator`, `session-summarizer`, `user-model`).

## M1 — Thinking agent with memory ✅ DONE

Change `m1-skeleton`, archived **2026-08-15**, delivered as 4 stacked PRs merged to main (PR #1 skeleton → #2 memory → #3 LLM adapters → #4 TUI).

**Shipped:**

- Hexagonal skeleton: `cmd/agis`, `internal/core`, `internal/config`, `internal/memory`, `internal/adapters/llm`, `internal/adapters/tui`.
- `Brain.Step` loop: persist user message → load 50-message tail → stream → persist assistant reply; tool calls logged and ignored.
- `Provider` port (`Chat` / `Stream` / `Models`) with OpenAI and Ollama adapters over one shared OpenAI-compatible client (`/chat/completions`, SSE). `StreamEvent{Text, Err}` contract; the provider always closes the channel, even after a terminal error.
- SQLite + FTS5 Repository: `conversations`, `messages`, `observations`, standalone `memory_fts` (`doc_type` discriminator, `unicode61 remove_diacritics 1`), same-transaction FTS sync, embedded migrations (`//go:embed` + `PRAGMA user_version`), WAL, foreign keys.
- Config loader: YAML, `-config` > `AGIS_HOME` > `~/.agis/config.yaml`, defaults `ollama` / `llama3.2`, 0600 permission warning.
- Minimal Bubbletea TUI: viewport, input, spinner, streaming, latest-conversation restore.

**Verification:** 9/9 requirements, 11/11 scenarios, all tests green (50 test cases across 6 packages, measured 2026-08-15: 44 top-level + 6 subtests, 0 failures). `go vet`, `golangci-lint`, and the go-arch architecture checks pass.

## M2 — Learning loop ✅ DONE

Change `m2-learning-loop`, archived **2026-08-24**, delivered as 4 stacked PRs merged to main (PR #5 memory substrate → #6 curator/summarizer/user-model → #7 recall/nudge/CloseSession → #8 TUI close hook + config + wiring).

**Shipped:**

- Curator: one LLM call per nudge extracts durable observations as JSON; fence-stripping parse; malformed responses log-and-skip; fires every N assistant messages with session-event records.
- Session summarizer: single combined close call returns `{summary, observations[]}` and persists all three write products (summary, observations, user model).
- Topic-key observations with upsert (stable `topic_key`, importance clamped 1–5, default 3) over migration 0002 (`user_model`, `session_events`, UNIQUE `topic_key`).
- User model: pure aggregation of `user/*` observations with 0.7/0.3 confidence blending.
- Recall injection: top-N observations enter every turn as a system message.
- `Brain.CloseSession` + TUI close hook: bounded, non-fatal close on quit; streaming quit cancels and drains the partial reply; second press force-quits. Suite runs under goleak.
- Config `memory` block: `learning_enabled`, `recall_limit`, `nudge_every`, `close_timeout`.

**Verification:** independent bounded reviews approved per slice (PR3 with zero findings after one abandoned iteration was fixed); `go build ./...`, `go vet ./...`, `go test ./...` green. "Done" criteria met: the curator writes observations, `summary` is populated at close, and `user_model` rows exist with confidence.

## M3 — Skills & persona ✅ DONE

Change `m3-skills-persona`, delivered as 4 stacked PRs merged to main (PR #9 skills substrate → #10 skill hub → #11 persona → integration PR).

**Shipped:**

- Skill hub: agentskills.io-compatible Markdown loading with strict frontmatter validation, AND-term matching over name/trigger/description with stop-word filtering, usage tracking (`usage_count`, `last_used`), atomic `.atl/skill-registry.md` regeneration in `$AGIS_HOME`.
- Agent-created skills: one bounded LLM call at session close distills reusable procedures into `source=agent` skills; malformed answers log-and-skip; `skill` session events recorded.
- SOUL.md durable identity: embedded-default seed at first run (0600), never overwritten, fallback on empty/unreadable, prompt-injection scanning.
- Personality overlays: built-ins (concise/teacher/technical/creative) + config presets; `/personality none|default|neutral` clears; session-scoped only.
- Derived evolution: top-5 user-model rows by confidence as a guidance layer; `/persona freeze|reset|status`; evolution never rewrites SOUL.md.

**Verification:** independent bounded reviews approved per slice (PR2 and PR3 with zero findings after self-caught fixes); full suite green under goleak. "Done" criteria met: dropped files load and match, close-time creation persists agent skills, first run seeds SOUL.md, `/personality` switches voice next turn, freeze disables evolution.

## M4 — Tools, backends & permissions

- Local tools with Policy Guard, Docker backend, SSH backend.
- `agis policy` CLI and `/permisos` TUI panel, interactive approval in the TUI.
- Full design in [docs/permissions.md](docs/permissions.md) and the security controls in [docs/security.md](docs/security.md). "Done" means Policy Guard is the single enforcement point for real tool calls, `agis policy` works end-to-end, and the audit log records decisions.

## M5 — Full TUI

- Slash commands (`/new`, `/save`, `/list`, `/restore`, `/compress`, `/snapshot`, `/rename`), session browse, interrupt-and-redirect. Design in [docs/sessions.md](docs/sessions.md). "Done" means every command in the table there is wired to the Session Manager and the Repository.

## M6 — Gateway + cron + ecosystem

- Telegram/Discord gateway first (WhatsApp, Signal, Slack, Email adapters follow), scheduled automations, plugin manager, webhook listener.

"Done" for M6 means one gateway platform is live and it drives the same `Brain` and Repository as the TUI — surfaces stay interchangeable front-ends, never parallel agent paths.

## M1 review follow-ups queued for M2

From the M1 review, deferred deliberately:

- **FTS delete sync** — deleting a conversation/message orphans its `memory_fts` rows (no delete path yet).
- **Stream cancel/abandon leak** — the caller can still abandon a stream before the provider closes its channel; needs explicit cancel propagation.
- **Multi-word phrase search** — `ftsQuery` wraps the whole query as one phrase; free-form FTS5 query syntax is future work.
- **UUID tie-break** — `LatestConversation` orders by `updated_at DESC, id DESC`; two conversations with identical timestamps rely on UUID ordering.
- **Hand-rolled client vs pinned SDK** — the OpenAI-compatible client is hand-rolled; revisit if the SDK would reduce maintenance.
- **`tui.New` signature drift** — the TUI constructor takes `(*core.Brain, core.Repository, chan string)`; revisit when the surface contract stabilizes.

These six are the M1 review debt; the next milestone (M2) should close the first three at minimum.
