# Exploration: M1 — Thinking agent with memory (Go skeleton)

**Change name**: `m1-skeleton`
**Milestone**: M1 — Go skeleton, hexagonal layout, Brain loop, multi-provider LLM port (OpenAI + Ollama), SQLite + FTS5 storage, session persist/restore, minimal TUI.
**Scope authority**: `spec.md` — sections 1 (Brain loop), 2 (LLM provider port), 3 (Memory system), 6 (TUI), 7 (Session management), Milestones.
**Baseline**: greenfield. No go.mod, no git repo, no code. Go 1.26.5 toolchain detected. Strict TDD disabled (`openspec/config.yaml` → `tdd: false`).
**Verified live**: modernc.org/sqlite v1.56.0 + FTS5 `unicode61 remove_diacritics 1` smoke-tested (accent-insensitive MATCH + AND queries pass). GAIA's `go.mod` read as architectural DNA reference (module `gaia`, go 1.26.5, bubbletea v1.3.10, bubbles v1.0.0, lipgloss v1.1.0, go-openai v1.41.2, modernc.org/sqlite v1.49.1, yaml.v3 v3.0.1, goleak v1.3.0, anthropic-sdk-go, telebot, discordgo).

---

## Current State

No code exists. Only `spec.md` (full AGIS spec), `openspec/config.yaml` (hybrid storage, TDD off, `go test ./...`), `.atl/skill-registry.md`, and Engram context (`sdd/agis/testing-capabilities`, `sdd-init/agis`) are present. M1 is the first implementation milestone: it must produce a compilable hexagonal skeleton where the Brain loop actually talks to OpenAI/Ollama, persists to SQLite+FTS5, restores the last session, and runs in a minimal Bubbletea TUI.

## Affected Areas (all new files — nothing to modify)

- `go.mod` / `go.sum` — new module + pinned deps
- `cmd/agis/main.go` — minimal entry point (flags, config load, wiring, `tea.Program`)
- `internal/config/config.go` — YAML config load (`~/.agis/config.yaml`, 0600), defaults
- `internal/core/types.go` — domain types: `Role`, `Message`, `Conversation`, `ChatRequest`, `ChatResponse`, `Token`, `ModelInfo`
- `internal/core/port_llm.go` — `Provider` port (spec §2)
- `internal/core/port_repository.go` — `Repository` port (spec §3)
- `internal/core/brain.go` — Brain loop (spec §1)
- `internal/core/session.go` — session lifecycle: create/restore (spec §7)
- `internal/adapters/llm/client.go` — shared OpenAI-compatible client wrapper
- `internal/adapters/llm/openai.go`, `internal/adapters/llm/ollama.go` — two thin adapters
- `internal/adapters/tui/app.go` — minimal Bubbletea model
- `internal/memory/sqlite.go` — Repository implementation (modernc.org/sqlite)
- `internal/memory/fts.go` — FTS5 search + explicit index sync
- `internal/memory/migrations/*.sql` — embedded migrations (single `0001_init.sql` in M1)
- `*_test.go` alongside each (brain, sqlite, fts, config, llm wrapper)
- `Makefile`, `.gitignore`, `.golangci.yml` (project-layout essentials)

## 1. Module & Layout

**Module path**: `github.com/kuno/agis` — **OPEN DECISION** (see Risks #1). Follows golang-project-layout: lowercase, hyphen-free single word, must match eventual repo URL. Until the repo exists, this is a proposal-phase confirmation with the user.

**M1-trimmed hexagonal skeleton** (spec layout trimmed to what M1 executes; no placeholder dirs):

```
cmd/agis/main.go              # entry: flags, config, wiring, tea.Program
internal/config/config.go     # ~/.agis/config.yaml loader (yaml.v3)
internal/core/                # domain + ports + brain loop (core NEVER imports adapters)
    types.go                  # Role, Message, Conversation, ChatRequest/Response, Token, ModelInfo
    port_llm.go               # Provider port
    port_repository.go        # Repository port
    brain.go                  # Brain loop
    session.go                # session create/restore
internal/adapters/llm/        # OpenAI-compatible client + openai/ollama adapters
internal/adapters/tui/        # minimal Bubbletea app
internal/memory/              # SQLite Repository impl + migrations/ + fts.go
pkg/                          # NOT created in M1 (no shared packages yet)
```

**Deferred dirs — do NOT create in M1**: `internal/skills` (M3), `internal/tools` + `internal/policy` (M4), `internal/persona` (M3), `internal/gateway` (M6), `internal/mcp` + `internal/plugins` + `internal/webhook` + `internal/cron` (future). Empty Go dirs add noise, not value; create them when their milestone lands.

**Layout deltas vs spec.md** (flag for proposal): `internal/config` is added (spec layout omits it; config loading is required in M1). Everything else follows the spec exactly: TUI under `internal/adapters`, memory store under `internal/memory`, ports + brain under `internal/core`.

**CLI**: stdlib `flag` only (e.g. `-config`), NO Cobra/Viper in M1 — single entry point, no subcommands. Cobra lands in M4 with `agis policy` / `agis config` / `agis model`. Viper never needed (config is one YAML file + env).

**DI**: manual constructor injection (GAIA-style). No DI framework.

## 2. Ports in M1

| Port | Status | Shape |
|---|---|---|
| `core.Provider` (LLM) | **REAL in M1** | `Chat(ctx, ChatRequest) (ChatResponse, error)` · `Stream(ctx, ChatRequest) (<-chan Token, error)` · `Models() []ModelInfo` — exact spec §2 signature |
| `core.Repository` (memory) | **REAL in M1** | `CreateConversation` · `GetConversation` · `LatestConversation` (restore) · `ListConversations(limit)` · `AppendMessage` · `Messages(convID, limit)` · `Search(query, limit)` (FTS5) · `Close` |
| Tool port | Deferred → M4 | spec §5 `Tool` interface; tool routing is a stub in M1 |
| Curator / Summarizer | Deferred → M2 | no ports in M1; session summary left empty |
| Skills / Gateway | Deferred → M3 / M6 | — |

Per golang-structs-interfaces: interfaces defined where consumed (`internal/core`), small (Provider = 3 methods, Repository = 7). Both are justified now (Provider has 2 adapters + a fake for tests; Repository has a real impl + is the seam for later bolt-ons like embeddings). No premature interfaces beyond these two.

## 3. Brain Loop Skeleton

`internal/core/brain.go` — **real in M1**:
1. Receive user message (from TUI) → `Brain.Step(ctx, input)`.
2. Persist the user message immediately (crash-safe).
3. Load context: current conversation's messages (tail, bounded). M1 recall is limited to the active conversation; cross-session FTS5 recall is a M2 concern (curator/summarizer). The `Search` port exists and is tested, but the loop does not auto-inject prior-session hits in M1.
4. Build prompt: minimal system message ("You are AGIS, a general-purpose assistant…") + bounded history.
5. Call `Provider.Stream`; stream tokens to the TUI via a `<-chan Token`.
6. On completion, persist the assistant message (single append with full text).
7. Update conversation `updated_at` + `message_count`.

**Stubs in M1**: tool routing (if the response contains `tool_calls`, log "no tools in M1" and stop — the loop is not autonomous yet); curator nudges, summarizer, skills, policy — all M2+.

**Clarification for the proposal**: in M1 the "loop" is one-user-message-in → one-streamed-response-out, driven by the TUI. The autonomous multi-step agentic loop (LLM → tool → result → LLM) requires the Tool port and is M4. M1's loop is the skeleton the M2–M4 features attach to.

## 4. SQLite + FTS5

**Driver**: `modernc.org/sqlite` (pure Go, zero cgo — matches spec §3 and the single-binary goal). FTS5 confirmed available in v1.56.0 via smoke test.

**M1 schema** (`0001_init.sql`), SQLite-flavored:

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE conversations (
    id            TEXT PRIMARY KEY,                -- uuid v4
    title         TEXT NOT NULL DEFAULT 'New session',
    created_at    TEXT NOT NULL,                   -- ISO-8601 UTC (RFC3339)
    updated_at    TEXT NOT NULL,
    summary       TEXT NOT NULL DEFAULT '',        -- filled by M2 summarizer
    message_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK (role IN ('user','assistant','system','tool')),
    content         TEXT NOT NULL,
    created_at      TEXT NOT NULL
);
CREATE INDEX idx_messages_conv ON messages(conversation_id, id);

CREATE TABLE observations (                         -- schema only in M1; written from M2
    id          TEXT PRIMARY KEY,                  -- uuid v4
    topic_key   TEXT NOT NULL,
    type        TEXT NOT NULL DEFAULT 'note',
    content     TEXT NOT NULL,
    importance  INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    source_ref  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_observations_topic ON observations(topic_key);

-- Single FTS5 index over messages + observations (standalone; explicit sync, no triggers)
CREATE VIRTUAL TABLE memory_fts USING fts5(
    doc_type UNINDEXED,                            -- 'message' | 'observation'
    doc_id   UNINDEXED,                            -- message id or observation uuid
    content,
    tokenize = 'unicode61 remove_diacritics 1'
);
```

Notes:
- `unicode61 remove_diacritics 1` → accent-insensitive search (verified: `configuración` matches `configuracion`) — right default for the user's Spanish + English text.
- **FTS5 architecture decision**: ONE standalone `memory_fts` table (doc_type/doc_id/content) rather than two external-content FTS tables over `messages` and `observations`. Rationale: FTS5 external-content mode can reference only ONE base table, so the spec's "over observations + messages" forces either two tables + triggers or this discriminator table. The single table matches the task's singular "FTS5 virtual table", keeps SQL explicit, and avoids triggers — per golang-database's "avoid hidden SQL features" rule. Repository `AppendMessage` / (future) `SaveObservation` write the FTS row in the SAME transaction as the base row; a `RebuildFTS()` method provides backfill.

**Migrations — embedded, no external tooling** (spec vision: single binary, zero external services; golang-database's "external migration tool" rule targets shared production DBs, not embedded single-writer SQLite):
- `//go:embed migrations/*.sql` (numbered: `0001_init.sql`).
- Applier in `internal/memory`: read `PRAGMA user_version` → apply files with number > version inside one transaction each → `PRAGMA user_version = N`. Atomic and testable.
- Deliberate deviation from golang-database §Migrations — flag in proposal.

**Connection handling**: single writer path in M1 (TUI is the only surface). `SetMaxOpenConns(1)` (SQLite WAL still allows concurrent readers; keep it simple), busy timeout `PRAGMA busy_timeout = 5000`.

## 5. Multi-Provider (OpenAI + Ollama)

Both expose the OpenAI-compatible `/v1/chat/completions` (SSE) API. **Recommended: one shared OpenAI-compatible client, two thin adapters** — this is exactly what the task asks to weigh, and it is what GAIA proves in production:

- `internal/adapters/llm/client.go`: wraps `sashabaranov/go-openai` (`NewClientWithConfig(ClientConfig{BaseURL, APIKey, HTTPClient{Timeout}})`). Exposes `Chat`/`Stream`/`Models` that satisfy `core.Provider` via a small shared struct with a `Model` field.
- `openai.go`: `NewOpenAI(cfg) *Provider` → `BaseURL: https://api.openai.com/v1`, API key from config/env.
- `ollama.go`: `NewOllama(cfg) *Provider` → `BaseURL: http://localhost:11434/v1`, no API key. Ollama's OpenAI-compat endpoint returns the same SSE chunk format go-openai's stream parser consumes.
- Provider selection at runtime from config (`config.llm.provider: openai|ollama` + `model`) — no code change to switch, per spec §2.
- Anthropic gets its own SDK adapter in a later milestone (GAIA uses `anthropic-sdk-go`); out of M1 scope.

**Streaming shape**: `Provider.Stream` returns `<-chan Token` per spec. **OPEN DECISION** (Risks #7): a bare `<-chan Token` cannot carry terminal errors; recommend amending the spec to `Stream(ctx, ChatRequest) (<-chan StreamEvent, error)` with `StreamEvent{Text string; Err error}` (channel closed by producer; `Err` set on stream failure; cancellation via ctx). This is a small, justified deviation — proposal must confirm.

## 6. Session Persist/Restore — M1 Subset

Spec §7 slash commands (`/new`, `/save`, `/list`, `/restore`, `/compress`, `/snapshot`, `/rename`) are M5 (full TUI). **M1 owns the persistence substrate only**:

- **Create**: on boot, `SessionManager.RestoreOrCreate(ctx)` → load the latest conversation (`LatestConversation`) if it exists, else create one. A session IS a conversation in M1.
- **Persist**: every user + assistant message appended incrementally to SQLite as the exchange happens (crash-safe, spec §7 "Run"); `updated_at` and `message_count` maintained by `AppendMessage`.
- **Restore**: the loaded conversation's tail of messages is rendered into the TUI viewport on start; full history stays queryable via FTS5 `Search`.
- **Close**: on app exit nothing special runs in M1 — `summary` stays `''` (curator + summarizer are M2), session-scoped permission grants don't exist yet (M4).
- No slash commands in the M1 TUI.

## 7. Minimal TUI

`internal/adapters/tui/app.go` — smallest Bubbletea app that "starts a conversation, streams a response, can be extended":

- Model: `viewport` (scrollable history) + `textinput` (bubbles v1.0.0) + `spinner` while streaming + minimal lipgloss styling.
- Messages: `userEnteredMsg` (from input Enter), `tokenMsg` (each streamed token), `streamDoneMsg` (error or completion).
- Enter → persist user msg → `Brain.Step` in a goroutine → tokens arrive on the channel → `tokenMsg` updates the viewport.
- Structure (model + message types + update function) is exactly the seam M5 extends with slash commands, autocomplete, session browse, interrupt-and-redirect.
- NOT in M1 (deferred to M5): slash commands, autocomplete, session browser, interrupt, approval prompts, `/permisos`.

## 8. Dependencies (M1, exact)

| Module | Version | Why | Source |
|---|---|---|---|
| `modernc.org/sqlite` | **v1.56.0** | pure-Go SQLite + FTS5 (smoke-tested) | GAIA: v1.49.1; latest v1.56.0 |
| `github.com/charmbracelet/bubbletea` | **v1.3.10** | TUI runtime | GAIA-proven |
| `github.com/charmbracelet/bubbles` | **v1.0.0** | textinput, viewport, spinner | GAIA-proven |
| `github.com/charmbracelet/lipgloss` | **v1.1.0** | styling | GAIA-proven |
| `github.com/sashabaranov/go-openai` | **v1.42.0** | shared OpenAI-compatible client (OpenAI + Ollama) | GAIA: v1.41.2 |
| `gopkg.in/yaml.v3` | **v3.0.1** | config file | GAIA-proven |
| `github.com/google/uuid` | **v1.6.0** | conversation/observation ids | GAIA-proven |
| `github.com/stretchr/testify` | **v1.11.1** | test asserts (test-only) | latest |
| `go.uber.org/goleak` | **v1.3.0** | goroutine leak checks on the stream goroutine (test-only) | GAIA-proven |

Not in M1 (deliberately): Cobra/Viper (M4), `anthropic-sdk-go` (Anthropic adapter, later), `charmbracelet/x/exp/teatest` (TUI golden tests — optional, GAIA uses it; defer to M5 when the TUI stabilizes). Per golang-dependency-management: commit `go.sum`, `go mod tidy` before every commit, `govulncheck ./...` before release. No vendoring needed (single machine dev; revisit for releases).

## 9. Testing Strategy (M1)

All pure Go → no `//go:build integration` needed; every test runs in plain `go test ./...`. No network in tests (httptest + fakes).

1. **Brain loop** (`internal/core/brain_test.go`): hand-written fake `core.Provider` (spec: "mock interfaces, not concrete types") + in-memory or temp-dir repo. Table-driven: user input → fake receives expected prompt/history; streamed response persisted as assistant message; provider error → graceful done event, no panic. `goleak.VerifyTestMain` (stream goroutine).
2. **Repository** (`internal/memory/sqlite_test.go`): `t.TempDir()` DB + embedded migrations. CRUD: create conversation, append messages, order, `message_count`/`updated_at` maintenance, `LatestConversation`, cascade delete. Each test independently runnable.
3. **FTS5** (`internal/memory/fts_test.go`): MATCH over appended messages; accent-insensitive (`configuración` ≈ `configuracion`); AND queries; doc_type filtering; FTS row synced in the same transaction as the base write; `RebuildFTS` backfill.
4. **LLM adapters** (`internal/adapters/llm/client_test.go`): `httptest.Server` returning canned SSE `chat.completion.chunk` payloads → assert `Stream` yields expected tokens and terminates; assert base URL hit for both openai and ollama configs (no real network).
5. **Config** (`internal/config/config_test.go`): defaults when file missing, provider/model parse, unknown fields ignored.
6. Optional: TUI smoke test with fake provider (teatest or plain `tea.NewProgram` in test) — cheap and catches wiring errors; keep minimal in M1.

Coverage threshold 0 (per config.yaml). `test_command: go test ./...`, `build_command: go build ./cmd/agis/...`.

## 10. Risks & Open Decisions (for the proposal phase)

1. **Module path** — propose `github.com/kuno/agis`; must confirm GitHub owner + whether to `git init` in M1 (recommended: yes, initial conventional commit, no AI attribution per project rules).
2. **`Stream` signature** — spec's `<-chan Token` cannot surface stream errors; recommend `<-chan StreamEvent` (Text/Err) → small spec amendment to confirm.
3. **FTS5 tokenizer** — recommended `unicode61 remove_diacritics 1` (verified; ES+EN). Alternative `trigram` (infix matching, ~3x index size). Confirm.
4. **FTS5 architecture** — single standalone `memory_fts` discriminator table (recommended, explicit sync, no triggers) vs two external-content tables + triggers (spec-literal). Confirm.
5. **Embedded migrations** — `//go:embed` + `PRAGMA user_version` deviates from golang-database's external-tools rule; justified by single-binary/zero-services vision. Confirm.
6. **Config layout** — `internal/config` package added to spec layout; `~/.agis/config.yaml` (0600) + `AGIS_HOME` env + `-config` flag; API key via config or `AGIS_OPENAI_API_KEY` env (spec §Security: secrets never logged). Confirm.
7. **`Models()` in M1** — static list from config (recommended) vs live fetch (Ollama `/api/tags`). Recommend static.
8. **Review budget (800 lines)** — the full M1 skeleton likely exceeds a single 800-line PR; `sdd-tasks` must forecast and likely propose chained PRs (e.g. 1: skeleton+config+core ports+brain, 2: memory+fts+migrations, 3: llm adapters, 4: tui+main). Decision: chained vs one big PR.
9. **observations table** — present in M1 schema but unwritten (curator is M2). Confirm no agent writes to it in M1.

## Approaches (summary)

1. **GAIA-aligned minimal skeleton** (recommended): exact GAIA-proven deps, one OpenAI-compatible client + 2 thin adapters, single FTS5 table, embedded migrations, stdlib flags. Pros: lowest risk (every dep proven in GAIA at near-identical versions), smallest surface, compiles to one static binary day one. Cons: slight spec deviations to confirm (#2/#3/#4/#5). Effort: Medium.
2. **Spec-literal**: two external-content FTS tables + triggers, official `openai/openai-go` SDK, `<-chan Token` verbatim, Cobra CLI now. Pros: closer to spec text. Cons: triggers violate golang-database guidance, openai-go is untested in GAIA, Cobra is dead weight pre-M4, error-handling gap in the stream shape. Effort: Medium-High.
3. **Maximum future-proofing**: create all spec dirs now, full models live-fetch, teatest TUI tests, vendor/ dir. Pros: layout matches spec.md literally. Cons: empty dirs, unused abstractions, premature complexity against golang-design-patterns "don't abstract prematurely". Effort: High.

## Recommendation

Approach 1: a GAIA-aligned, minimal-but-real M1 — both ports real, Brain loop streaming end-to-end, SQLite+FTS5 with a single explicit-sync FTS table, two thin OpenAI-compatible adapters, embedded migrations, a small extensible Bubbletea app, and the 6 test areas above. Confirm the 9 open decisions in the proposal (the only hard blocker is #1 module path). Keep the skeleton honest: everything created is executed and tested; everything deferred is a stub or absent by design.

## Risks

- Module path + git init unconfirmed (blocker for `go mod init`).
- 800-line review budget likely exceeded by the full skeleton → tasks phase must forecast chained PRs.
- FTS5 design (single table vs external content) and tokenizer are the two storage decisions most costly to reverse later — settle in proposal.
- Stream error contract (StreamEvent vs Token) is a spec amendment; get it right in M1 or it ripples through M2–M6.

## Ready for Proposal

**Yes** — exploration is complete, all 10 questions answered concretely, FTS5 verified empirically, deps pinned, GAIA alignment confirmed. The proposal phase must resolve the 9 open decisions (esp. module path) before spec/design.

---

## Key Learnings

1. modernc.org/sqlite v1.56.0 includes working FTS5 with unicode61 remove_diacritics, verified by a standalone smoke test.
2. GAIA's go.mod pins the proven AGIS M1 stack: bubbletea v1.3.10, bubbles v1.0.0, lipgloss v1.1.0, go-openai v1.41.2, modernc.org/sqlite v1.49.1.
3. One OpenAI-compatible client (sashabaranov/go-openai) serves both OpenAI and Ollama adapters via BaseURL configuration alone.
4. FTS5 external-content mode can reference only one base table, so a spec-literal "FTS over observations + messages" forces either two tables plus triggers or a single doc_type discriminator table.
5. The M1 brain loop is one-user-message-in/one-streamed-response-out; the autonomous tool-driven loop is deferred to M4 with the Tool port.
