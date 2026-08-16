```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:24384ec0b05f6e967c3e408208f3db019e96bca103c10c7787c009453fba577c
verdict: pass
blockers: 0
critical_findings: 0
requirements: 9/9
scenarios: 11/11
test_command: go test -count=1 ./...
test_exit_code: 0
test_output_hash: sha256:7e329e7b55c4484975f6979f338ca769d79b94a2a8262329dfe8f9a83774d200
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

# Verify Report: m1-skeleton (M1 — Thinking agent with memory)

- Change: `m1-skeleton` · Repo: `github.com/SalvucciFacundo/agis` · Verified HEAD: `a35f351` (all 4 PRs merged)
- Execution mode: `auto` · Artifact store: `hybrid` · Review budget: 800
- Authoritative spec counts: **9 requirements** (CONF-001, REPO-001..004, LLM-001..002, BRAIN-001, TUI-001) · **11 scenarios** (LLM-001 ×2, BRAIN-001 ×2, others ×1)
- Result: **PASS** — 9/9 requirements, 11/11 scenarios verified against code, tests, and a live-produced database.

## Proof Suite

| Check | Command | Result | Evidence |
|---|---|---|---|
| Vet | `go vet ./...` | exit 0 | clean, no output |
| Tests | `go test -count=1 ./...` | exit 0 | 47 tests, 5 packages, all `ok`; hash `7e329e…` |
| Race | `go test -race -count=1 ./...` | exit 0 | 5 packages `ok`, no data races |
| Format | `gofmt -l .` | empty | no unformatted files |
| Build all | `go build ./...` | exit 0 | no output (empty hash `e3b0c4…`) |
| Build binary | `go build -o /tmp/opencode/agis-verify-bin ./cmd/agis` | exit 0 | 17,403,994-byte ELF x86-64 |
| CGO-free | `CGO_ENABLED=0 go build` | exit 0 | statically linked binary (modernc.org/sqlite is pure Go) |
| Smoke run | `AGIS_HOME=$(mktemp -d) timeout 3 /tmp/opencode/agis-verify-bin` | exit 1, graceful | `agis: could not open a new TTY: open /dev/tty: no such device or address` — expected TUI non-TTY behavior, NOT a failure |
| Live DB proof | sqlite3 on the binary-produced `$AGIS_HOME/agis.db` | pass | `user_version=1`; tables `conversations`, `messages`, `observations`, `memory_fts` present; `memory_fts` DDL matches spec |

The smoke run proves the startup path executes before the TTY gate: config load (AGIS_HOME precedence + defaults → `db.path`), repository open, and embedded migrations all ran — the produced database contains the full schema at `user_version=1` with WAL mode active.

## Requirement Walkthrough (9/9 PASS)

### AGIS-M1-CONF-001 — Load configuration from YAML — **PASS**
- Precedence `-config` flag > `AGIS_HOME` > default `~/.agis/config.yaml`: `internal/config/config.go` `resolvePath` (L115–120) + `agisDir` (L124–133).
- 0600 warning: `warnPerms` (L141–153) warns to stderr when mode grants group/other bits; expectedPerm `0o600` (L25).
- Defaults `ollama` / `llama3.2` / `~/.agis/agis.db`: consts L18–26, `defaults()` L88–98, `applyDefaults` L102–112.
- M1 fields `llm.provider`, `llm.model`, `llm.api_key`, `db.path`: Config structs L29–44.
- Scenario "Config loads with defaults" (missing file → built-in defaults): `TestLoad_MissingFileUsesDefaults` (config_test.go L21–40).
- Also covered: `TestLoad_FlagOverridesAGISHome`, `TestLoad_AGISHomeOverridesDefault`, `TestLoad_PartialConfigKeepsDefaults`, `TestLoad_LoosePermissionsWarn`, `TestLoad_TightPermissionsNoWarn`, `TestLoad_InvalidYAMLErrors`, `TestResolvePath_Precedence`.

### AGIS-M1-REPO-001 — Repository port with M1 subset — **PASS**
- Port exposes exactly the 6 methods: `CreateConversation`, `LatestConversation`, `AppendMessage`, `Messages(convID, limit)`, `Search(query, limit)`, `Close` — `internal/core/port_repository.go` L14–21.
- `AppendMessage` updates `conversations.updated_at` and `message_count` transactionally: single `BeginTx` wraps message INSERT + FTS row + `UPDATE conversations SET updated_at = ?, message_count = message_count + 1` — `internal/memory/sqlite.go` L105–139.
- Scenario "Persist and retrieve messages": `TestAppendAndMessages_Order` (order + IDs), `TestCreateAndLatestConversation`, `TestMessages_TailLimit`, `TestAppendMessage_UpdatesCountAndTimestamp`, `TestLatestConversation_ReturnsMostRecentlyUpdated`, `TestCascadeDelete`, `TestAppendMessage_MissingConversationErrors`.
- Compile-time port check: `var _ core.Repository = (*Repository)(nil)` (sqlite.go L30).

### AGIS-M1-REPO-002 — SQLite schema — **PASS**
- Three tables `conversations`, `messages`, `observations`: `internal/memory/migrations/0001_init.sql` L4–31.
- Role CHECK restricted to `user`, `assistant`, `system`, `tool`: `messages.role ... CHECK (role IN ('user','assistant','system','tool'))` (L16); `core.Role` consts in `internal/core/types.go` L13–18.
- Foreign keys enforced: `PRAGMA foreign_keys = ON` in migration (L2) and re-applied per connection in `configureDB` (`migrations.go` L30) because FK pragma is connection-scoped (verified: a fresh sqlite3 process reports `foreign_keys=0`, the app connection has it ON).
- Scenario "Schema created by migrations": `TestMigrations` (all 4 tables exist), `TestMigrations_EnforcesForeignKeys` (INSERT to missing conversation rejected), `TestCascadeDelete`.

### AGIS-M1-REPO-003 — Single FTS5 table with doc_type discriminator — **PASS**
- Standalone `memory_fts` FTS5 table `(doc_type, doc_id, content)` with `tokenize = 'unicode61 remove_diacritics 1'`: `0001_init.sql` L33–38; `doc_type`/`doc_id` UNINDEXED.
- `Search` matches both `message` and `observation` doc types: `searchMatches` (`internal/memory/fts.go` L33–62) has no doc_type filter; doc_type constants `message`/`observation` (L13–16).
- FTS row synced in the same transaction as the base write (no triggers): `insertFTSRow` (fts.go L21–29) called from `AppendMessage` inside the tx.
- Scenario "Accent-insensitive search" (`configuración` ↔ `configuracion`): `TestSearch_AccentInsensitive` (fts_test.go L10–36).
- Also: `TestSearch_ReturnsBothDocTypes` (spans message+observation), `TestSearch_EmptyQueryReturnsEmpty`, `TestSearch_Limit`, `TestAppendMessage_FailureRollsBackFTS` (no orphan FTS row on failed append), `TestSearch_ImmediatelyVisibleAfterAppend`.
- User queries escaped as FTS5 phrases (`ftsQuery`, fts.go L67–68) — no operator injection.

### AGIS-M1-REPO-004 — Embedded migrations — **PASS**
- `//go:embed migrations/*.sql`: `migrations.go` L23–24.
- Applier reads `PRAGMA user_version` (L111–117), executes files with numeric prefix > current version, each inside its own transaction, then bumps `PRAGMA user_version` (L121–139); `loadMigrations` sorts by version (L77–96).
- Scenario "Migrations are idempotent" (version 0 → apply 0001 → user_version 1): `TestMigrations` (v=1 after apply) and `TestMigrations_Idempotent` (second apply is a no-op).
- Live proof: the binary-produced DB reports `PRAGMA user_version = 1` (verified via sqlite3 CLI).

### AGIS-M1-LLM-001 — Provider port and adapters — **PASS**
- `Provider` port exposes `Chat`, `Stream`, `Models`: `internal/core/port_llm.go` L7–11.
- `Stream` returns `(<-chan StreamEvent, error)` with `StreamEvent{Text, Err}`: L9, L13–18 (amendment from spec §2 `<-chan Token`; recorded in proposal decision 2 and design).
- OpenAI and Ollama adapters via a shared OpenAI-compatible client: `internal/adapters/llm/client.go` (shared HTTP+SSE client speaking `/chat/completions`), `openai.go` (api.openai.com/v1), `ollama.go` (localhost:11434/v1). `NewProvider` selects by `cfg.Provider` (provider.go L14–19); both adapters satisfy `var _ core.Provider`.
- Scenario "Stream emits text events" (fake provider "hello"+"world" → both tokens then close): `TestStream_TokenOrder` (Hel+lo → "Hello", channel closes).
- Scenario "Stream surfaces mid-stream errors" (`StreamEvent{Err}` then close): `TestStream_MidStreamError` (text then error event; channel closes).
- Also: `TestStream_HTTPError` (non-200 → immediate error), `TestChat`, `TestOllama_Chat`, `TestNewProvider_SelectsAdapter`, `TestAdapterBaseURLs`. goleak guards goroutines (`TestMain` L17–19).

### AGIS-M1-LLM-002 — Static Models list — **PASS**
- `Models()` returns the static model from `llm.model`: `staticModels` (provider.go L23–27), used by `OpenAI.Models()` (openai.go L38–40) and `Ollama.Models()` (ollama.go L39–41). Live enumeration explicitly deferred to M4 (provider.go comment).
- Scenario "Models returns configured entry": `TestModels` — one `ModelInfo{ID: "gpt-4o-mini", Provider: "openai"}` and one `{ID: "llama3.2", Provider: "ollama"}`.

### AGIS-M1-BRAIN-001 — Brain.Step loop — **PASS**
- `Brain.Step(ctx, input)` persists user message → loads tail (`tailLimit=50`) → `Provider.Stream` → forwards tokens to sink → persists assistant message: `internal/core/brain.go` L47–89.
- Provider error path: user message stays persisted, no assistant message written; immediate `Stream` error (L62–65) and mid-stream `StreamEvent.Err` (L69–77, drains channel before returning per port contract).
- Tool calls "logged and ignored in M1": documented on Brain (brain.go L18–20); no tool port exists in M1 (out of scope per proposal).
- Scenario "Step streams and persists" (fake streams "Hi", Step("Hello") → both persisted, sink gets "Hi"): `TestBrainStep_StreamsAndPersists`; token accumulation: `TestBrainStep_AccumulatesTokens`.
- Scenario "Step handles provider errors" (error returned, user persisted, no assistant): `TestBrainStep_ImmediateStreamError` + `TestBrainStep_MidStreamError` (exactly 1 message, role user).
- Also: `TestBrainStep_ReusesLatestConversation` (ensureConversation L93–101). goleak in TestMain (L13–15).

### AGIS-M1-TUI-001 — Minimal Bubbletea TUI — **PASS**
- Renders viewport, text input, spinner: `Model` (app.go L42–67) with `viewport.Model`, `textinput.Model`, `spinner.Model`; `View()` renders viewport + spinner/status + input (L176–182).
- Enter sends input to `Brain.Step` and streams tokens into the viewport: `submit()` (L188–209) starts a goroutine running `Brain.Step` with the sink writing to a token channel; `tokenMsg`/`waitToken`/`refresh` paint tokens live (L151–154, L213–221, L278–281).
- Restores latest conversation on startup: `Init` → `loadHistory` → `restoreHistory` (L116–118, L241–266).
- Scenario "Send a message" (type + Enter → persisted + streamed response appears): `TestEnter_StreamsIntoViewport` (user line + streamed "Hello" reply + streaming=false at end).
- Also: `TestRestoreHistory`, `TestRestoreHistory_EmptyIsFine`, `TestEnter_BlankInputIsIgnored`, `TestEnter_StreamErrorShowsError`. goleak in TestMain.
- Wiring matches design composition root: `cmd/agis/main.go` — `-config` flag → `config.Load` → `memory.NewRepository` (embedded migrations) → `llm.NewProvider` → `core.NewBrain` (with sink) → `tui.New` → `tea.NewProgram(app).Run()`.

## Scope Creep Check — **PASS (no creep)**

- No `internal/{skills,tools,policy,persona,gateway,mcp,cron,plugins,webhook}` directories; no `pkg/`. `internal/` contains only `adapters`, `config`, `core`, `memory`.
- Grep across all `*.go` for M2+ concepts (`curator|nudge|summariz|user model|persona|SOUL.md|skill hub|tool port|policy guard|gateway|cron|webhook|plugin|mcp`): **NO MATCHES**.
- go.mod direct deps match the proposal's pinned list (bubbles v1.0.0, bubbletea v1.3.10, lipgloss v1.1.0, uuid v1.6.0, goleak v1.3.0, yaml.v3 v3.0.1, modernc.org/sqlite v1.56.0) — one deviation, see risks.

## Risks

- **SUGGESTION — dependency deviation from proposal pins**: proposal pinned `sashabaranov/go-openai v1.42.0` (shared client) and `testify v1.11.1` (test asserts). Implementation instead hand-rolls the OpenAI-compatible HTTP+SSE client (`adapters/llm/client.go`) and uses stdlib `testing`. LLM-001 is satisfied behaviorally — the shared client IS an OpenAI-compatible client (`/chat/completions` + SSE), and behavior is covered by httptest SSE tests (token order, mid-stream `Err`, non-200, auth header). Fewer dependencies and full control of mid-stream error surfacing; the go-openai pin can be dropped from proposal records or the deviation documented at archive.
- **SUGGESTION — `tui.New` signature**: design wiring shows `tui.New(brain, repo)`; implementation is `tui.New(brain, repo, stream)` (app.go L88). The injected token channel lets the model own stream lifecycle; matches design data-flow intent. Cosmetic doc drift only.
- **INFO — `foreign_keys=0` via a separate sqlite3 process**: expected — FK pragma is connection-scoped; the app connection sets it ON in `configureDB` before migrations/DML and `TestMigrations_EnforcesForeignKeys` proves enforcement.
- **INFO — smoke run exit code 1**: the TUI binary starts, loads config, runs migrations (proven by the produced DB), then exits 1 with a graceful non-TTY message. Expected behavior outside a terminal, not a failure.
- **INFO — `golang-database` skill recommends external migration tooling**: deviation is deliberate and documented (proposal decision 5, design rationale) — single static binary, embedded SQLite is single-writer, not a shared production DB.

## Verdict

**PASS** — 9/9 requirements and 11/11 scenarios verified. Proof suite green (vet, test, race, gofmt, build ×2, cgo-free build, live DB schema proof). No scope creep. No blockers, no critical findings. Ready for archive.

Next step: `archive` — sync delta specs into `openspec/specs/` and close `m1-skeleton`.
