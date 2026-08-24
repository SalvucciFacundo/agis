# Exploration: M2 — Learning loop (curator + nudges, session summarization, topic-key observations, user model)

**Change name**: `m2-learning-loop`
**Milestone**: M2 — Learning loop (spec.md Milestones).
**Scope authority**: `spec.md` §3 Memory system (the loop, schema), §7 Session management (close: curator + summarizer), Milestones.
**Baseline**: HEAD `main` (`211d00f`), M1 complete/verified/archived. Schema at `user_version = 1` (`0001_init.sql`). Repository port has 5 data methods; Brain.Step is a single-turn loop; TUI quits on CtrlC/Esc with **no close hook**.
**Verified live**: `LatestConversation` already orders `updated_at DESC, id DESC` (UUID tie-break shipped in M1 at `a35f351` — archive listed it as deferred but code has it; M2 only needs a test). `memory_fts` doc_type discriminator already reserves `observation`. FTS sync is insert-only (no delete path — M1 follow-up confirmed open in code).
**Store mode**: hybrid (openspec file + Engram topic `sdd/m2-learning-loop/explore`).

---

## Current State

M1 delivered the persistence substrate, nothing of the learning loop:

- **Schema (v1)**: `conversations` (has `summary TEXT NOT NULL DEFAULT ''`, filled by M2), `messages`, `observations` (**schema-only, no writer, no `updated_at` column**), `memory_fts` (doc_type `message`|`observation`, explicit same-transaction sync, insert-only).
- **Repository port** (`core.Repository`): `CreateConversation`, `LatestConversation`, `AppendMessage`, `Messages`, `Search`, `Close`. **No observation writes, no summary update, no user model.**
- **Brain** (`Brain.Step`): persist user msg → load 50-message tail → `Provider.Stream` → persist assistant msg. **No recall, no memory injection, no close lifecycle.**
- **TUI**: viewport + input + spinner. CtrlC/Esc → `tea.Quit` immediately (no close sequence). Single active conversation (no `/new` — M5).
- **Provider port**: `Chat` (unused by Brain today; used by adapters/tests) and `Stream` — the curator/summarizer will use **`Chat`** (non-streaming, structured reply).
- **Tests**: 47 green, 5 packages, hand-written fakes (`fakeProvider`, `fakeRepo` in `internal/core/brain_test.go`), `goleak.VerifyTestMain`.

## Scope Boundary: IN vs OUT

### IN (M2)
| Piece | What it means |
|---|---|
| **Curator** | LLM decides what to persist as observations; structured JSON reply (`topic_key`, `type`, `content`, `importance`); agent-curated per spec §3.1 |
| **Nudges** | Periodic mid-session persist prompts (message-count triggered; default every 10 assistant messages) — the "nudge" is the system prompt, the trigger cadence is count-based |
| **Session summarizer** | At close, ONE LLM `Chat` call returns `{summary, observations[]}`; summary written to `conversations.summary` (spec §3.3: summary + candidate observations in one shot) |
| **Topic-key observations (real writes)** | `SaveObservations` with upsert semantics (same `topic_key` updates existing row), importance clamp, `source_ref = convID`, FTS sync with **delete-on-replace** |
| **User model (minimal)** | `user_model` table + aggregation of `user/*` observations into rows with confidence; write-path only in M2 |
| **Session close hook** | TUI quit triggers `Brain.CloseSession` (summarize + curate + aggregate) before exit |
| **Minimal recall** | Top-N observations injected into the Step system prompt — closes the "loop" (write-only memory is not a loop); cheap, ~30 lines |
| **M1 follow-ups absorbed** | FTS delete sync (required by upsert), stream cancel/abandon leak coverage (quit path exercises it), multi-word phrase search (observation recall needs it), UUID tie-break test |
| **Config** | `memory: {learning_enabled, nudge_every, recall_limit, close_timeout}` |

### OUT (deferred)
| Piece | Milestone |
|---|---|
| Skills hub, skill creation, agentskills.io, registry | M3 |
| SOUL.md, persona overlays, evolution | M3 |
| `skills` table | M3 (do NOT create in 0002) |
| Tool port, Policy Guard, backends | M4 |
| Slash commands `/new /save /list /restore /compress /snapshot /rename`, session manager UI | M5 |
| Gateway, cron, MCP, plugins, webhooks | M6 |
| Honcho-style dialectic (propositions/antitheses/syntheses, multi-layer user model) | later milestone; M2 = single row per key |
| Anthropic adapter, hand-rolled client vs pinned SDK decision | later (M2 adds no LLM client code) |
| LLM-generated cross-session context *digest* (FTS5 search + summarize at start) | M5 with `/restore`; M2 does top-N injection only |

## Affected Areas

- `internal/core/types.go` — add `Observation`, `UserModel` domain types (with `UpdatedAt`)
- `internal/core/port_repository.go` — extend port: `SaveObservations`, `Observations`, `UpdateConversationSummary`, `UpsertUserModel` (batch, keyed on `key`)
- `internal/core/brain.go` — recall injection in `Step`; new `CloseSession(ctx)` orchestrating summarizer+curator+aggregation; stream-cancel handling
- `internal/memory/sqlite.go` — implement new repo methods; observation upsert + FTS delete/insert in one tx
- `internal/memory/fts.go` — `deleteFTSRow` helper; multi-word AND query build (split+quote each term)
- `internal/memory/curator.go` **NEW** — curator + nudge prompt/parse (spec layout: `internal/memory` = "memory store, curator, summarizer, user model")
- `internal/memory/summarizer.go` **NEW** — close-prompt (summary + candidate observations)
- `internal/memory/usermodel.go` **NEW** — pure aggregation function + upsert
- `internal/memory/migrations/0002_learning.sql` **NEW** — `observations.updated_at` ALTER, unique `topic_key` index, `user_model`, `session_events`
- `internal/adapters/tui/app.go` — CtrlC/Esc close sequence (status line, then quit; cancel stream when streaming)
- `internal/config/config.go` — `MemoryConfig` block with defaults
- `*_test.go` alongside each; `internal/core/brain_test.go` fakes extended for new port methods

## 1. Curator Design

**Question**: when does the agent decide to persist an observation?

Options:
- **(a) LLM-driven** — brain asks the provider "what should be remembered?" with a structured-output prompt (Hermes/GAIA pattern, spec-literal: "the agent decides what to persist", "agent-curated memory").
- **(b) Heuristic rules** — importance score from message content (keywords/regex). Deterministic, zero LLM cost, but cannot generalize to a general-purpose assistant (the user may discuss anything).
- **(c) Hybrid** — heuristic pre-filter gates the LLM. Saves tokens, but adds a brittle layer and risks dropping nuance; the LLM is already in the loop at close anyway.

**Recommendation: (a) LLM-driven, single shared prompt, two trigger points.** Rationale: spec §3 says the agent curates; heuristics (b) cannot decide *what is worth remembering* for arbitrary domains; (c) buys little for its complexity in M2 because the close call is unavoidable. The cost control is not a heuristic gate — it is bounded transcript input (tail cap) and a config kill-switch.

### Nudges

- **Trigger cadence is message-count, not timer** (no ticker, no mid-turn interruption, deterministic in tests): after every `nudge_every` (default 10) assistant messages, the next `Step` first runs the curator prompt over the *last N messages* and persists results. The system message is the "nudge"; a `session_events` row (`kind='nudge'`) records it for observability.
- **Nudge prompt shape** (system): *"You are the memory curator for AGIS. From the recent conversation, decide what is worth remembering long-term. Return ONLY a JSON array of observations: [{topic_key, type, content, importance(1-5)}]. Same topic_key updates existing memory. If nothing is worth remembering, return []."* + bounded transcript (last `nudge_every` messages).
- **Parsing**: fenced JSON block, stripped and `encoding/json`-parsed. No new dependencies, no JSON-mode dependency on the provider (Ollama ignores `response_format`). Lenient: on parse failure, log + skip (never fail the turn).

### Close-time curation

The close call combines summary + observations in ONE LLM call (spec §3.3) — halves cost vs two calls:

> system: "You are AGIS's session summarizer and memory curator. Compress this conversation into a concise summary (~150-200 words): what was discussed, decisions, facts about the user, open tasks, in the conversation's language. Also return candidate long-term observations. Respond ONLY with JSON: {"summary": "...", "observations": [{topic_key, type, content, importance(1-5)}]}."

## 2. Observation Write Path (Repository)

New port methods (batch, per golang-database "batch operations"; all FTS sync in the same tx — no drift):

```go
SaveObservations(ctx context.Context, convID string, obs []Observation) error
Observations(ctx context.Context, limit int) ([]Observation, error)   // recall + curator context
UpdateConversationSummary(ctx context.Context, convID, summary string) error
UpsertUserModel(ctx context.Context, rows []UserModel) error
```

**Upsert semantics** (per spec: "same-topic observations update the existing row — the Engram model"):
- Unique index on `observations(topic_key)` (new in 0002). Inside `SaveObservations`' transaction, per observation: `SELECT id, content FROM observations WHERE topic_key=?` → if found: `UPDATE ... SET content/type/importance/source_ref, updated_at=now` (preserve `created_at`), then **`DELETE FROM memory_fts WHERE doc_type='observation' AND doc_id=?`** then insert the new FTS row (same doc_id); if not found: `INSERT` with new UUID + FTS insert.
- **This is where M1 follow-up #1 (FTS delete sync) is absorbed** — a replaced observation MUST drop its old FTS row or stale content haunts search. `deleteFTSRow` helper in `fts.go`, used by the upsert path. Message-side FTS delete stays dormant (no delete path exists in M2 — conversations are never deleted).
- **Importance**: integer 1–5, clamped to [1,5], default 3 when absent/zero from the LLM.
- **`source_ref`** = conversation ID (auditability: which session produced this memory).
- **Batch atomicity**: one bad row rolls back the whole batch (single tx).

**Multi-word phrase search** (follow-up #3): replace whole-query phrase wrapping with per-term quoting joined by `AND` — `"coffee preference"` → `"coffee" AND "preference"`. This is a behavior change vs M1's exact-phrase match; flag in proposal (recommended: AND-join; exact phrase can return later via a quoted variant). Observation recall makes multi-word queries the common case.

## 3. Session Summarization Hook

- **Trigger in M2**: TUI quit (CtrlC/Esc) is the ONLY session close (no `/new` until M5). The TUI's close path calls `Brain.CloseSession(ctx)` and shows a status line ("closing session…") while it runs, then `tea.Quit`.
- **`Brain.CloseSession` sequence** (domain-owned, surface-agnostic per spec §7):
  1. Load full message list (bounded — cap transcript at last ~200 messages for cost).
  2. One `Provider.Chat` call → parse `{summary, observations[]}`.
  3. `UpdateConversationSummary`; `SaveObservations`; aggregate `user/*` observations into `user_model`; record `session_events` (`kind='summary'` + `kind='nudge'` if nudges fired).
  4. Errors are **non-fatal**: log + continue to quit. Data is already persisted incrementally (crash-safe); a failed summary never blocks exit.
- **Blocking vs async**: **synchronous with a 30s context timeout** (config `memory.close_timeout`). Rationale: a background goroutine risks being killed at process exit (no flush guarantee); synchronous is simple, observable, and the timeout bounds the wait. This is a proposal decision.
- **Quit while streaming**: CtrlC during an in-flight `Step` first cancels the Step context (drains the provider stream — exercises follow-up #2), then runs the close sequence. Implement as: CtrlC → if streaming, cancel + wait for `streamDoneMsg`; second CtrlC quits immediately without closing.
- **`UpdateConversationSummary`** must NOT bump `conversations.updated_at` (summary is metadata, not activity) — keeps `LatestConversation` ordering stable.

## 4. User Model (minimal M2 subset)

Spec §3.5: "observations about the user are periodically aggregated into `user_model` rows with confidence, updated as new evidence arrives." Honcho-style dialectic (propositions/antitheses/syntheses) is the long-term vision — **M2 subset: one row per key, confidence from evidence strength, updated by upsert.**

- **Schema** (0002): `user_model (id TEXT PK, key TEXT NOT NULL UNIQUE, value TEXT NOT NULL, confidence REAL NOT NULL DEFAULT 0, updated_at TEXT NOT NULL)` — `UNIQUE(key)` is the upsert target.
- **Convention**: an observation participates iff `topic_key` has prefix `user/` (e.g. `user/preferences/coffee`). Aggregation is a **pure function** `AggregateUserModel(obs []Observation) []UserModel` (fully unit-testable):
  - `key = observation.topic_key` (full string — traceable back to the observation; prefix filter works either way)
  - `value = observation.content`
  - `confidence`: first write `= clamp(importance/5, 0, 1)`; on update `= clamp(0.7*old + 0.3*new, 0, 1)` (recency-weighted blend, bounded)
- **When it runs**: inside `CloseSession` after `SaveObservations` (and on the nudge path) — read back the session's `user/*` observations, aggregate, `UpsertUserModel`.
- **Read path**: M2 is write-path only — recall injects observations (which already include `user/*` rows), so `user_model` injection is redundant; it becomes valuable in M4+ when tools make decisions. Flag as decision.

## 5. M1 Follow-ups: Absorb vs Defer

| # | Follow-up | Verdict | Note |
|---|---|---|---|
| 1 | FTS delete sync | **IN** | Required by observation upsert; `deleteFTSRow` in fts.go; regression test: replaced observation's old content no longer matches |
| 2 | Stream cancel/abandon leak | **IN** | TUI quit-during-stream exercises it; add ctx-cancel drain test + goleak |
| 3 | Multi-word phrase search | **IN** | AND-join per-term; serves observation recall; behavior change to flag |
| 4 | UUID tie-break | **ALREADY DONE** | `id DESC` shipped in M1 code (`sqlite.go:91`); archive listed as deferred inaccurately — M2 adds a tie-break assertion test only |
| 5 | Hand-rolled client vs pinned SDK | **DEFER** | M2 adds no LLM client code; decide later |
| 6 | `tui.New` signature drift | **FOLD IN** | Cosmetic; M2 edits app.go anyway — fix doc comment |

## 6. Migration — `0002_learning.sql`

- `ALTER TABLE observations ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''` then backfill `UPDATE observations SET updated_at = created_at WHERE updated_at = ''` (SQLite can't default to another column).
- `DROP INDEX idx_observations_topic;` + `CREATE UNIQUE INDEX idx_observations_topic_key ON observations(topic_key);` — enforces the upsert model (1 evolving fact per topic).
- `CREATE TABLE user_model (...)` as §4.
- `CREATE TABLE session_events (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL, kind TEXT NOT NULL CHECK (kind IN ('nudge','summary','skill')), payload TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL);` — `session_id` = conversation UUID (session IS a conversation in M2). Written on nudges + summaries; `skill` reserved for M3. **Recommended but optional** — it is the only observability into learning-loop LLM activity/cost; if the proposal wants zero unused surface, defer the whole table (kind CHECK cannot be altered without rebuild, so include `skill` now if creating it).
- **No `skills` table** — that is M3.

## 7. Testing Strategy

All pure Go, no integration tag (M1 pattern: `t.TempDir()` DBs, hand-written fakes, goleak):

1. **Curator** (`internal/memory/curator_test.go`): fake `Provider.Chat` returns canned JSON (fenced + plain) → parsed observations correct; absent importance → 3; malformed JSON → error, zero writes; `[]` → no rows; prompt contains bounded transcript.
2. **Summarizer** (`internal/memory/summarizer_test.go`): fake returns `{summary, observations}` → `UpdateConversationSummary` + `SaveObservations` called with parsed values; provider error → close fails gracefully (no partial summary).
3. **Observation writes** (`internal/memory/sqlite_test.go` +): `SaveObservations` creates rows; **topic-key upsert**: same key twice → 1 row, content=latest, `created_at` preserved, `updated_at` bumped; importance clamp; batch atomicity (one bad row → no writes).
4. **FTS observations** (`internal/memory/fts_test.go` +): `Search` matches saved observation (`doc_type='observation'`); **delete-sync regression**: after upsert replaces content, old content no longer matches; multi-word AND query.
5. **User model** (`internal/memory/usermodel_test.go`): pure `AggregateUserModel` — `user/*` included, others excluded, confidence `=importance/5` on first write, blend `0.7/0.3` on update, bounds respected.
6. **Brain.CloseSession** (`internal/core/brain_test.go` +): order of operations (summary → observations → user_model) with fake repo asserting calls; **stream-abandon**: slow fake provider + canceled ctx → provider goroutine terminates (goleak); recall: Step prepends top-N observations to the provider request.
7. **TUI quit** (`internal/adapters/tui/app_test.go` +): CtrlC idle → close cmd runs → Quit; CtrlC streaming → cancel, no hang (fake instant provider).

## 8. Config

`internal/config/config.go` gains a `MemoryConfig` block (yaml `memory:`):

```yaml
memory:
  learning_enabled: true   # kill switch: false = M1 behavior, no LLM calls at close
  nudge_every: 10          # messages between curator nudges; 0 = disable nudges
  recall_limit: 10         # top-N observations injected into Step context
  close_timeout: 30s       # bound on the close-sequence LLM call
```

## Approaches (summary)

1. **LLM-curated loop with combined close call** (recommended): curator + summarizer = one shared structured prompt; close call returns `{summary, observations}` in one `Chat`; message-count nudges; unique-topic_key upsert + FTS delete-sync; minimal top-N recall; pure user-model aggregation; synchronous non-fatal close with timeout. Effort: **Medium-High**.
2. **Spec-minimal, no recall**: same curator/summarizer/user-model, but no observation injection into Step (write-only memory until M5) and no session_events. Pros: ~150 fewer lines, strictly the milestone bullet list. Cons: milestone is named "learning **loop**" — without a read path the loop is open; recall is ~30 lines and proves the observations are usable. Effort: Medium.
3. **Heuristic + hybrid curator**: keyword importance pre-filter, LLM only on flagged messages. Pros: lower LLM cost. Cons: brittle for a general-purpose agent, spec-divergent ("agent-curated"), more code. Effort: Medium-High, lower value.

## Recommendation

**Approach 1.** M2 delivers a genuine closed learning loop: message-count nudges + one combined close call (summary + candidate observations) → topic-key upsert writes with FTS delete-sync → minimal top-N recall into Step context → user-model aggregation of `user/*` observations. Follow-ups 1–3 absorbed, #4 tested, #5 deferred, #6 folded into the TUI edit. 0002 migration adds `observations.updated_at`, the unique topic_key index, `user_model`, and (recommended) `session_events`. Everything is unit-testable with the existing fake pattern; the close path is non-blocking-by-timeout and never crashes an exit.

## Risks & Open Decisions (for the proposal phase)

1. **Close blocks TUI quit** — recommended synchronous + 30s timeout + non-fatal errors; alternative: fire-and-forget goroutine (risks losing the write at exit). DECISION.
2. **LLM cost per session** — 1 close call (combined) + `ceil(messages/10)` nudges; bounded by transcript caps and `learning_enabled` kill switch. Confirm defaults.
3. **topic_key uniqueness** — unique index enforces 1 row per topic (spec-literal "same-topic updates existing row"); if multiple types per topic are ever needed it is a migration. DECISION.
4. **Multi-word search behavior change** — M1 exact-phrase → AND-join. Confirm the change is acceptable.
5. **Recall scope** — top-N injection (recommended) vs defer to M5. DECISION.
6. **session_events** — create + write (recommended) vs defer whole table. DECISION.
7. **Component placement** — curator/summarizer/usermodel live in `internal/memory` per spec layout (memory package imports core for ports/types — direction is fine); strict-hexagonal would put them in `internal/core`. Spec-authoritative: `internal/memory`. Flag in design.
8. **Structured output robustness** — fenced-JSON lenient parse; Ollama may return prose — on parse failure log + skip persist, never fail the turn. Confirm skip-not-fail.
9. **User model key** = full `topic_key` (traceable) vs stripped `user/` prefix. Recommend full.
10. **Review budget (800 lines)** — M2 clearly exceeds it (repo methods + FTS + 3 domain components + migration + TUI close + config + tests). `sdd-tasks` must forecast chained PRs, e.g. PR1 memory+fts+migration, PR2 curator+summarizer+usermodel+brain, PR3 TUI close+config.

## Ready for Proposal

**Yes** — all 9 questions answered concretely against real code; every follow-up disposition verified in the tree; migration shape and test plan defined. Proposal must resolve decisions 1, 3, 5, 6 (and confirm 2/4/8 defaults) and confirm the change name `m2-learning-loop`.

---

## Key Learnings

1. The UUID tie-break follow-up was already shipped in M1 code (`ORDER BY updated_at DESC, id DESC`) — the archive report over-declared it as deferred; M2 only needs a test.
2. The spec's close-time curation is one combined LLM call (`{summary, observations[]}`), so summarizer and curator share a single structured prompt and halve LLM cost.
3. Observation topic-key upsert forces the M1 FTS delete-sync follow-up into M2: a replaced row must delete its old `memory_fts` row inside the same transaction.
4. The `observations` table needs an `updated_at` column (absent in 0001) before upsert semantics are possible; SQLite cannot default it to `created_at`.
5. With no `/new` until M5, the TUI CtrlC/Esc quit path is the only session-close hook in M2 — close must be synchronous-but-time-bounded and non-fatal.
6. `memory_fts` doc_type `observation` and the phrase-wrapped `ftsQuery` were designed for M2; multi-word AND-join is a small behavior change to flag in proposal.
