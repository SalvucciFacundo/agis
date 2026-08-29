# Roadmap

Milestone scopes come from `spec.md` §Milestones. M1, M2, M3, M4, M5, and M6 are shipped. The archived requirements live as synced OpenSpec capability specs under `openspec/specs/` (`config-loader`, `repository-memory`, `llm-provider-port`, `brain-loop`, `minimal-tui`, `memory-curator`, `session-summarizer`, `user-model`, `skill-hub`, `persona`, `policy-guard`, `tools-backends`, `tool-calling`, `session-manager`, `gateway`, `cron`, `plugins`, `webhook`).

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

## M4 — Tools, backends & permissions ✅ DONE

Change `m4-tools-permissions`, archived **2026-08-26**, delivered as 5 stacked PRs merged to main (PR #13 policy core+audit → #14 CLI → #15 wire+loop+local → #16 docker+ssh → #17 panel+docs).

**Shipped:**

- Policy Guard: fail-closed store at `$AGIS_HOME/policy.yaml`, postures `sandbox`/`standard`/`full` (full session-only), decision flow `allow|deny|ask` with `deny` beating `allow`, scopes `once|session|always|deny`, `always` persisting exact-subject allow rules, audit log of every decision/grant/revocation.
- `agis policy` CLI: `init` (sandbox defaults), `set`/`rm` (per-backend with `-b` flag), `show`, `tier` (refuses `full`), `test` (dry-run preview).
- Tool-calling wire: additive `ChatRequest.Tools` + `StreamEvent.ToolCall` + `Message.ToolCalls/ToolCallID`; provider accumulates streamed `tool_calls` fragments per index, emits once at `finish_reason`, malformed degrades to text.
- Bounded brain loop: up to 8 rounds of evaluate→approve→execute→RoleTool feedback; cap audited with user notice; trust boundary kept type-level (brain sees only `PolicyGuard` + `Approver` callback).
- Backends: local shell (`sh -c`, 60s timeout), docker (`--rm` ephemeral, `alpine:3` default), ssh (strict host-key, optional key) — all behind injectable `cmdExec` seam, graceful degradation when binaries missing or settings incomplete, per-backend routing via `shell-<backend>` tool names.
- TUI: interactive approval prompt (`[a]llow once [s]ession a[l]ways [n]o`, `CtrlC` denies without quitting, watcher re-arms), and `/permisos` panel (rules by category, postures, live preview via guard, audit tail; `space` toggle, `r` revoke `always`, `q` close).

**Verification:** independent bounded reviews approved per slice (PR3 HIGH with 3 advisories, PR4 HIGH with 1 suggestion); full suite green under `goleak`. "Done" means the single enforcement point holds for real tool calls, the CLI manages policy end-to-end, and decisions are audited.

## M5 — Full TUI ✅ DONE

Change `m5-full-tui`, archived **2026-08-26**, delivered as 3 stacked PRs merged to main (PR #18 repository+manager → #19 TUI slash commands + wiring → #20 docs + polish).

**Shipped:**

- Session Manager (`internal/session`): active session id owned independent of surface, 7 operations (`NewSession`, `Save`, `List` ordered `updated_at DESC, id DESC`, `Restore`, `Rename` with injection scan, `Compress` early summarizer, `Snapshot` point-in-time copy with `messages_json`), share with TUI/gateway/cron.
- Repository extensions: `ListConversations`/`GetConversation`/`RenameConversation` (bumps `updated_at`, scanned title, empty rejected), `CreateSnapshot`/`ListSnapshots` via `snapshots` table (`internal/memory/migrations/0005_snapshots.sql`).
- Brain delegation: `SetActiveConversation(id)` and `ensureConversation` prefers manager id when set, falling back to `LatestConversation`.
- TUI: 7 slash branches in `runCommand` (`/new`/`/reset`, `/save`, `/list` inline, `/restore <id>` reloads history, `/compress` gated, `/snapshot`, `/rename <title>`), all gated `!streaming && !closing`, feedback via `commandFeedbackPrefix`, session list view, interrupt-and-redirect reuse.

**Verification:** independent bounded reviews approved per slice; full suite green under `goleak`. "Done" means every slash command in `docs/sessions.md` is wired and the manager is surface-agnostic.

## M6 — Gateway + cron + ecosystem ✅ DONE

Change `m6-ecosystem`, delivered as 4 stacked PRs merged to main (PR #1 gateway substrate & adapters → #2 cron scheduler engine → #3 plugin manager & webhook listener → #4 integration tests & documentation).

**Shipped:**

- Gateway Substrate & Multiplexer (`internal/gateway`): concurrent chat adapter orchestration, graceful shutdown via `context.Context`, session routing via `session.Manager` mapping (`gateway:<adapter>:<chatID>`), non-interactive `AutoDenyApprover` fail-safe for sandbox policy, and fail-closed static user ID allowlists.
- Telegram Adapter (`internal/gateway/telegram.go`): Telegram Bot API integration with polling updates, 4096-character outbound chunking, and session isolation.
- Discord Adapter (`internal/gateway/discord.go`): Discord Gateway integration with channel/DM listeners, 2000-character message splitting, and session isolation.
- Cron Scheduler Engine (`internal/cron`): background cron engine supporting 5-field cron syntax (`"0 9 * * *"`, `"*/15 * * * *"`, step intervals, macros) and duration intervals (`"@every 1h"`), non-interactive `core.Brain.Step` execution under sandbox policy, and notification forwarding to Gateway targets or logs.
- Plugin Manager (`internal/plugins`): dynamic discovery and lifecycle (`Load`, `List`, `Enable`, `Disable`, `inspect`) from `$AGIS_HOME/plugins/`, JSON schema manifest validation (`plugin.json`), persistent status in `state.json`, `core.ToolRunner` bridge execution, and skill extraction into Skill Hub.
- Webhook Listener Server (`internal/webhook`): HTTP server with constant-time HMAC-SHA256 signature verification (`X-Hub-Signature-256`), JSON event extraction, `core.Brain.Step` dispatch, and Gateway target forwarding.
- CLI Subcommands (`cmd/agis`): `agis gateway run`, `agis cron [run|list]`, `agis plugins [list|enable|disable|inspect]`, and `agis webhook run`.

**Verification:** full end-to-end integration test suite green across gateway, cron, plugins, and webhooks (`cmd/agis/ecosystem_integration_test.go`); 100% test suite green with race detection (`go test -race ./...`); 0 goroutine leaks under `goleak`. "Done" criteria met: gateway platforms drive the same `Brain` and Repository as the TUI, surfaces stay interchangeable front-ends.

## M1 review follow-ups queued for M2

From the M1 review, deferred deliberately:

- **FTS delete sync** — deleting a conversation/message orphans its `memory_fts` rows (no delete path yet).
- **Stream cancel/abandon leak** — the caller can still abandon a stream before the provider closes its channel; needs explicit cancel propagation.
- **Multi-word phrase search** — `ftsQuery` wraps the whole query as one phrase; free-form FTS5 query syntax is future work.
- **UUID tie-break** — `LatestConversation` orders by `updated_at DESC, id DESC`; two conversations with identical timestamps rely on UUID ordering.
- **Hand-rolled client vs pinned SDK** — the OpenAI-compatible client is hand-rolled; revisit if the SDK would reduce maintenance.
- **`tui.New` signature drift** — the TUI constructor takes `(*core.Brain, core.Repository, chan string)`; revisit when the surface contract stabilizes.

These six are the M1 review debt; the next milestone (M2) should close the first three at minimum.

---

## Future Horizons (Post-v1 / v2 Backlog)

Future capabilities identified for post-v1 expansion:

### 1. Automated CI/CD & Cross-Platform Releases
- GitHub Actions CI pipeline running `go test -race ./...`, `go vet ./...`, and build checks on pull requests.
- Multi-platform binary release pipeline via GitHub Actions for Linux (`amd64`, `arm64`), macOS (`amd64`, `arm64`), and Windows (`amd64`).
- Quickstart single-command install scripts (`install.sh` for Unix/macOS, `install.ps1` for Windows).

### 2. Hybrid Search (Vector Embeddings via Ollama)
- Complement BM25 / FTS5 lexical full-text search with local dense vector embeddings (`nomic-embed-text` via Ollama or `sqlite-vec`).
- Combine lexical keyword scores and cosine similarity behind the domain `Repository.Search` port without breaking single-binary ergonomics.

### 3. Model Context Protocol (MCP) Client
- Implement an MCP client adapter in `internal/mcp` to dynamically connect AGIS to community MCP servers (database explorers, web browsers, GitHub tools, custom APIs) without writing bespoke plugins.

### 4. Multimodal Ingestion (Vision & Audio)
- Extend `core.ChatRequest` and gateway message event pipelines to accept image and audio attachments.
- Route image payloads to multimodal LLMs (e.g. `llama3.2-vision`, `gpt-4o`) and audio to local Whisper/Ollama transcription endpoints.

