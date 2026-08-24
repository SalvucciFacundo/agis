# Proposal: M2 — Learning loop (curator + nudges, session summarization, topic-key observations, user model)

## Intent

M1 delivered the persistence substrate; M2 closes the learning loop. The agent curates observations via LLM-driven prompts, persists them with topic-key upsert semantics, summarizes sessions at close, aggregates user-model rows, and injects top-N observations into Step context. This transforms write-only memory into a genuine closed loop.

## Scope

### In Scope
- **Curator**: LLM decides what to persist; structured JSON (`topic_key`, `type`, `content`, `importance`); agent-curated per spec §3.1
- **Nudges**: Message-count triggered (default every 10 assistant messages)
- **Session summarizer**: ONE LLM `Chat` call at close returns `{summary, observations[]}`
- **Topic-key observations**: `SaveObservations` with upsert (same `topic_key` updates existing row), FTS delete-on-replace
- **User model**: `user_model` table + aggregation of `user/*` observations with confidence
- **Session close hook**: TUI quit → `Brain.CloseSession` before exit
- **Minimal recall**: Top-N observations into Step system prompt (closes the loop)
- **M1 follow-ups absorbed**: FTS delete sync, stream cancel/abandon, multi-word AND search, UUID tie-break test

### Out of Scope
- Skills hub, skill creation, agentskills.io, registry (M3)
- SOUL.md, persona overlays, evolution (M3)
- Tool port, Policy Guard, backends (M4)
- Slash commands `/new /save /list /restore /compress /snapshot /rename`, session manager UI (M5)
- Gateway, cron, MCP, plugins, webhooks (M6)
- Honcho-style dialectic (propositions/antitheses/syntheses, multi-layer user model) — deferred; M2 = single row per key

## Capabilities

### New Capabilities
- `memory-curator`: LLM-driven observation curation with structured JSON prompts, nudge cadence, and close-time summary+observation extraction
- `session-summarizer`: Close-time session compression into `conversations.summary` with candidate observations
- `user-model`: Aggregation of `user/*` observations into `user_model` rows with confidence scoring

### Modified Capabilities
- `repository-memory`: Add `SaveObservations`, `Observations`, `UpdateConversationSummary`, `UpsertUserModel` methods; observation upsert with FTS delete-sync
- `brain-loop`: Add recall injection in `Step` (top-N observations); new `CloseSession(ctx)` orchestrating summarizer+curator+aggregation
- `minimal-tui`: Add CtrlC/Esc close sequence (status line, close hook, then quit); cancel stream when streaming

## Approach

LLM-curated loop with combined close call: curator + summarizer share one structured prompt returning `{summary, observations[]}` in one `Chat` call (halves cost vs two calls). Message-count nudges trigger mid-session curation. Unique-topic upsert with FTS delete-sync ensures 1 evolving fact per topic. Minimal top-N recall injects observations into Step context (~30 lines, closes the loop). Pure user-model aggregation of `user/*` observations. Synchronous non-fatal close with 30s timeout bounds the wait; errors logged but never block exit.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/core/types.go` | Modified | Add `Observation`, `UserModel` domain types (with `UpdatedAt`) |
| `internal/core/port_repository.go` | Modified | Extend port: `SaveObservations`, `Observations`, `UpdateConversationSummary`, `UpsertUserModel` |
| `internal/core/brain.go` | Modified | Recall injection in `Step`; new `CloseSession(ctx)`; stream-cancel handling |
| `internal/memory/sqlite.go` | Modified | Implement new repo methods; observation upsert + FTS delete/insert in one tx |
| `internal/memory/fts.go` | Modified | `deleteFTSRow` helper; multi-word AND query build (split+quote each term) |
| `internal/memory/curator.go` | New | Curator + nudge prompt/parse |
| `internal/memory/summarizer.go` | New | Close-prompt (summary + candidate observations) |
| `internal/memory/usermodel.go` | New | Pure aggregation function + upsert |
| `internal/memory/migrations/0002_learning.sql` | New | `observations.updated_at` ALTER, unique `topic_key` index, `user_model`, `session_events` |
| `internal/adapters/tui/app.go` | Modified | CtrlC/Esc close sequence (status line, then quit; cancel stream when streaming) |
| `internal/config/config.go` | Modified | `MemoryConfig` block with defaults |

## Migration 0002 Plan

1. `ALTER TABLE observations ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''`
2. `UPDATE observations SET updated_at = created_at WHERE updated_at = ''` (backfill — SQLite cannot default to another column)
3. `DROP INDEX idx_observations_topic;`
4. `CREATE UNIQUE INDEX idx_observations_topic_key ON observations(topic_key);` (enforces 1 evolving fact per topic)
5. `CREATE TABLE user_model (id TEXT PK, key TEXT NOT NULL UNIQUE, value TEXT NOT NULL, confidence REAL NOT NULL DEFAULT 0, updated_at TEXT NOT NULL);`
6. `CREATE TABLE session_events (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL, kind TEXT NOT NULL CHECK (kind IN ('nudge','summary','skill')), payload TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL);` — include `skill` kind for M3 (CHECK cannot be altered without rebuild)

**No `skills` table** — that is M3.

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Close timeout (30s) insufficient for slow providers | Medium | Config-tunable `close_timeout`; non-fatal errors (log + continue to quit) |
| LLM returns malformed JSON | High | Lenient parse: on failure, log + skip persist, never fail the turn |
| Multi-word search behavior change (exact-phrase → AND-join) | Low | Document in changelog; exact-phrase can return later via quoted variant if needed |
| Background goroutine loses writes at process exit | Low | Synchronous close with timeout (not fire-and-forget) |

## Rollback Plan

Revert the merge commit. Migration 0002 is additive (no destructive schema changes); `user_version` rollback is not needed because 0002 only adds columns/tables, never removes. If rollback is needed after 0002 is applied, the new tables (`user_model`, `session_events`) and column (`observations.updated_at`) can remain without breaking M1 behavior — M2 code paths are gated by `memory.learning_enabled` config (default `true`, but settable to `false` to disable all LLM calls at close).

## Dependencies

- **No new external dependencies.** Stdlib `encoding/json` for structured output parsing; existing `modernc.org/sqlite` for schema changes.
- **M1 complete, verified, archived.** Schema at `user_version = 1` (`0001_init.sql`). Repository port has 5 data methods; Brain.Step is a single-turn loop; TUI quits on CtrlC/Esc with no close hook.

## Success Criteria

- [ ] Curator persists observations with topic-key upsert (same key → 1 row, latest content, `created_at` preserved, `updated_at` bumped)
- [ ] Session close writes summary to `conversations.summary` and candidate observations in ONE LLM call
- [ ] Top-N observations injected into `Brain.Step` system prompt (closes the loop)
- [ ] TUI CtrlC/Esc triggers `Brain.CloseSession` before exit; never blocks quit beyond `close_timeout`
- [ ] User-model aggregation of `user/*` observations with confidence scoring (first write = importance/5, update = 0.7*old + 0.3*new)
- [ ] All 47 M1 tests still green + new tests for curator, summarizer, observation writes, FTS delete-sync, user model, close sequence
- [ ] Migration 0002 applies cleanly to M1 schema; `user_version = 2`

## Open Decisions (Resolved)

### 1. Close blocks TUI quit
**Decision**: Synchronous + 30s `close_timeout` + non-fatal errors.

**Rationale**: A fire-and-forget goroutine risks losing writes at process exit (no flush guarantee). Synchronous is simple, observable, and the timeout bounds the wait. Errors are non-fatal: log + continue to quit. Data is already persisted incrementally (crash-safe); a failed summary never blocks exit.

**Quit-while-streaming**: CtrlC during an in-flight `Step` first cancels the Step context (drains the provider stream), then runs the close sequence. Implement as: CtrlC → if streaming, cancel + wait for `streamDoneMsg`; second CtrlC quits immediately without closing.

### 2. LLM cost per session
**Decision**: 1 combined close call (summary+observations) + `ceil(msgs/10)` nudges; `memory.learning_enabled` kill switch in config (default `true`).

**Rationale**: The close call is unavoidable (spec §3.3 requires summary + candidate observations). Combining them in ONE `Chat` call halves cost. Message-count nudges (not timer) are deterministic in tests and avoid mid-turn interruption. The kill switch (`learning_enabled: false`) reverts to M1 behavior (no LLM calls at close) for cost-sensitive deployments.

### 3. topic_key uniqueness
**Decision**: Unique index on `observations(topic_key)`, 1 evolving fact per topic (spec-literal "same-topic updates existing row").

**Rationale**: The spec explicitly states "same-topic observations update the existing row instead of duplicating — the upsert model used by Engram." A unique index enforces this at the schema level. If multiple types per topic are ever needed, it requires a migration.

### 4. Multi-word search behavior change
**Decision**: M1 exact-phrase → AND-join per-term. `"coffee preference"` becomes `"coffee" AND "preference"`.

**Rationale**: Observation recall makes multi-word queries the common case. AND-join returns more relevant results (all terms must match). Exact-phrase match can return later via a quoted variant if needed. This is a user-visible behavior change and must be documented in the changelog.

**Implementation**: Replace whole-query phrase wrapping with per-term quoting joined by `AND` — split the query on whitespace, quote each term, join with `AND`.

### 5. Recall scope
**Decision**: Top-N relevant observations injected into `Brain.Step` context in M2 (closes the loop).

**Rationale**: The milestone is named "learning **loop**" — without a read path, the loop is open. Recall is ~30 lines of code and proves the observations are usable. Deferring to M5 leaves the loop open for 3 milestones. Top-N (default 10) is cheap and configurable via `memory.recall_limit`.

**Implementation**: In `Brain.Step`, after loading the 50-message tail, call `repo.Observations(ctx, recall_limit)` and prepend the top-N observations to the system prompt.

### 6. session_events table
**Decision**: Create + write. Include `kind CHECK (kind IN ('nudge','summary','skill'))` with `skill` reserved for M3.

**Rationale**: It is the only observability into learning-loop LLM activity/cost. Without it, debugging nudges and summaries requires parsing logs. The `skill` kind is included now because SQLite CHECK constraints cannot be altered without a rebuild — adding it later requires dropping and recreating the table. `session_id` = conversation UUID (session IS a conversation in M2).

### 7. Component placement
**Decision**: Curator/summarizer/usermodel under `internal/memory` (spec-authoritative layout).

**Rationale**: The spec's Repository layout explicitly lists "internal/memory — memory store, curator, summarizer, user model." This is the spec-authoritative placement. Core stays ports-only (`internal/core` = domain logic, ports, kernel, brain loop). The memory package imports core for ports/types — the dependency direction is correct.

### 8. Structured-output parse failure
**Decision**: Skip persist, never fail the turn.

**Rationale**: LLM output (especially local models via Ollama) can be unpredictable — the model may return prose, malformed JSON, or fenced blocks. We must never block the user's workflow. On parse failure: log the error, skip the persist, continue. The next nudge or close attempt will retry.

**Implementation**: Strip fences (```json ... ```), parse with `encoding/json`. On error: log + return empty observations (no rows persisted). Never return an error to the caller.

### 9. user_model.key
**Decision**: Full `topic_key` (traceable).

**Rationale**: Easier to debug when you can trace back to the source observation. An observation with `topic_key = "user/preferences/coffee"` becomes a `user_model` row with `key = "user/preferences/coffee"`. The prefix filter (`user/*`) works either way; keeping the full key preserves traceability.

### 10. Review-budget slicing (800 lines)
**Decision**: Chained PRs — exploration proposed (PR1 memory+fts+migration, PR2 curator+summarizer+usermodel+brain, PR3 TUI close+config).

**Refined line estimates**:
- **PR1**: Memory substrate + FTS + migration 0002 (~350 lines)
  - `sqlite.go`: new repo methods (`SaveObservations`, `Observations`, `UpdateConversationSummary`, `UpsertUserModel`) — ~150 lines
  - `fts.go`: `deleteFTSRow` helper + AND query build — ~50 lines
  - `0002_learning.sql`: migration — ~30 lines
  - `types.go`, `port_repository.go`: types + port extension — ~50 lines
  - Tests: observation writes, FTS delete-sync, upsert, batch atomicity — ~70 lines
- **PR2**: Curator + summarizer + usermodel + brain (~450 lines)
  - `curator.go`: curator + nudge prompt/parse — ~100 lines
  - `summarizer.go`: close-prompt (summary + candidate observations) — ~80 lines
  - `usermodel.go`: pure aggregation function — ~60 lines
  - `brain.go`: recall injection in `Step` + `CloseSession(ctx)` + stream-cancel handling — ~120 lines
  - Tests: curator, summarizer, user model, brain close sequence, recall — ~90 lines
- **PR3**: TUI close + config (~200 lines)
  - `app.go`: CtrlC/Esc close sequence (status line, then quit; cancel stream when streaming) — ~100 lines
  - `config.go`: `MemoryConfig` block with defaults — ~40 lines
  - Tests: TUI quit, close hook, stream cancel — ~60 lines

**Total**: ~1000 lines across 3 PRs (each under review budget).

## Implementation Notes

### Config
```yaml
memory:
  learning_enabled: true   # kill switch: false = M1 behavior, no LLM calls at close
  nudge_every: 10          # messages between curator nudges; 0 = disable nudges
  recall_limit: 10         # top-N observations injected into Step context
  close_timeout: 30s       # bound on the close-sequence LLM call
```

### Testing Strategy
All pure Go, no integration tag (M1 pattern: `t.TempDir()` DBs, hand-written fakes, goleak):
1. **Curator**: fake `Provider.Chat` returns canned JSON → parsed observations correct; absent importance → 3; malformed JSON → error, zero writes
2. **Summarizer**: fake returns `{summary, observations}` → `UpdateConversationSummary` + `SaveObservations` called; provider error → close fails gracefully
3. **Observation writes**: topic-key upsert (same key twice → 1 row, `created_at` preserved, `updated_at` bumped); importance clamp; batch atomicity
4. **FTS observations**: `Search` matches saved observation; delete-sync regression (after upsert, old content no longer matches); multi-word AND query
5. **User model**: pure `AggregateUserModel` — `user/*` included, others excluded, confidence `=importance/5` on first write, blend `0.7/0.3` on update
6. **Brain.CloseSession**: order of operations (summary → observations → user_model); stream-abandon (slow fake provider + canceled ctx → provider goroutine terminates); recall: Step prepends top-N observations
7. **TUI quit**: CtrlC idle → close cmd runs → Quit; CtrlC streaming → cancel, no hang

### UpdateConversationSummary Behavior
`UpdateConversationSummary` must NOT bump `conversations.updated_at` (summary is metadata, not activity) — keeps `LatestConversation` ordering stable.

### M1 Follow-ups Disposition
| # | Follow-up | Verdict | Note |
|---|---|---|---|
| 1 | FTS delete sync | **IN** | Required by observation upsert; regression test: replaced observation's old content no longer matches |
| 2 | Stream cancel/abandon leak | **IN** | TUI quit-during-stream exercises it; add ctx-cancel drain test + goleak |
| 3 | Multi-word phrase search | **IN** | AND-join per-term; serves observation recall; behavior change documented |
| 4 | UUID tie-break | **ALREADY DONE** | `id DESC` shipped in M1 code; M2 adds a tie-break assertion test only |
| 5 | Hand-rolled client vs pinned SDK | **DEFER** | M2 adds no LLM client code; decide later |
| 6 | `tui.New` signature drift | **FOLD IN** | Cosmetic; M2 edits app.go anyway — fix doc comment |
