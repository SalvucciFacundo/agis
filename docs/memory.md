# Memory

AGIS persists everything in a single SQLite database behind the `core.Repository` port. The implementation lives in `internal/memory`.

## Storage: SQLite via modernc.org/sqlite

- **Pure Go, no cgo** — `modernc.org/sqlite` compiles into the single static binary; no platform libraries or runtime services are required.
- **Single file** — the whole store is one file (`~/.agis/agis.db` by default).
- **WAL mode** with `busy_timeout = 5000` and `foreign_keys = ON`, applied as connection pragmas at open (`internal/memory/migrations.go:29`). SQLite serializes writers, so M1 pins the pool to one connection.
- **Fixed-width UTC timestamps** — stored as nine-digit RFC3339 strings so lexicographic order equals chronological order (`internal/memory/sqlite.go:23`).

## Schema

Applied by the embedded migration `0001_init.sql`:

| Table | Purpose |
|---|---|
| `conversations` | `id` (UUID), `title`, `created_at`, `updated_at`, `summary`, `message_count` |
| `messages` | `id` (autoinc), `conversation_id` → conversations (cascade delete), `role`, `content`, `created_at`. Role is `CHECK`ed to `user\|assistant\|system\|tool`. |
| `observations` | `id` (UUID), `topic_key` (unique), `type`, `content`, `importance`, `created_at`, `updated_at`, `source_ref` — topic-key upsert with FTS delete-sync |
| `user_model` | `id` (UUID), `key` (unique), `value`, `confidence`, `updated_at` — aggregated user facts keyed by the source observation's `topic_key` |
| `session_events` | `id` (autoinc), `session_id`, `kind` (`nudge`\|`summary`\|`skill`), `payload`, `created_at` — observability for learning-loop activity |
| `memory_fts` | standalone FTS5 virtual table — see below |

## Full-text search: one FTS5 table, `doc_type` discriminator

`memory_fts` is a **standalone FTS5 table**, not external-content mode:

```sql
CREATE VIRTUAL TABLE memory_fts USING fts5(
    doc_type UNINDEXED,
    doc_id   UNINDEXED,
    content,
    tokenize = 'unicode61 remove_diacritics 1'
);
```

- `doc_type` discriminates `message` vs `observation` rows. External-content mode was rejected because FTS5 binds to a single base table; a standalone table indexes both kinds under one index.
- `unicode61 remove_diacritics 1` makes search accent-insensitive — searching `configuracion` matches a stored `configuración` (verified by test).
- `Search` returns matches ordered by FTS rank across both doc types (`internal/memory/fts.go`).
- Multi-word queries are AND-joined per token (`ftsQuery`): each whitespace-separated token is quoted (embedded quotes escaped as `""`) and joined with `AND`, so every term must match. A query of `coffee preference` becomes `"coffee" AND "preference"`. This changed M1's whole-query exact-phrase behavior — a deliberate shift to serve observation recall, where several distinct words commonly co-occur.

## Embedded migrations

- SQL files are embedded with `//go:embed migrations/*.sql` and versioned by their numeric prefix (`0001_init.sql` → version 1).
- On open, the applier reads `PRAGMA user_version`, runs each newer file in its own transaction, and bumps `user_version` — no external migration tool, consistent with the single-static-binary goal.
- Migrations are idempotent: a database already at version N skips everything below N.
- Non-transactional pragmas (`journal_mode`, `foreign_keys`) are applied separately in `configureDB` because they are no-ops inside a transaction.

## Same-transaction FTS sync

`AppendMessage` inserts the message row, inserts its `memory_fts` row, and updates the conversation's `updated_at` + `message_count` **in one transaction** (`internal/memory/sqlite.go:105`). A reader never observes a message without its search row, a stale count, or a timestamp that is behind — and a failure rolls the whole append back (verified by `TestAppendMessage_FailureRollsBackFTS`).

There are no hidden triggers; the FTS row is written explicitly. Consequences:

- `Search` sees a message immediately after `AppendMessage` commits.
- Observation upserts also delete-then-insert their FTS row in the same transaction (`deleteFTSRow`), so replaced content never haunts search.
- **Remaining follow-up:** message-side FTS rows are still not deleted when their base row is deleted (e.g. cascade-deleting a conversation orphans its `memory_fts` rows); no conversation-delete path exists yet.

## Observation write path

`SaveObservations` persists a batch atomically, upserting on the unique `topic_key`:

- A re-saved topic keeps its `id` and `created_at` and only bumps `updated_at`; a new topic gets a fresh UUID.
- Importance is clamped to `[1,5]`, with `0` (the "unset" curator value) defaulting to `3`.
- `source_ref` records the producing conversation.
- Each upsert deletes then re-inserts its `memory_fts` row in the same transaction, and one bad row rolls back the whole batch.

`Observations` is the recall read path, returning the most recently updated observations newest first. `UpdateConversationSummary` writes a summary without bumping `conversations.updated_at`, keeping `LatestConversation` ordering stable. `UpsertUserModel` upserts on the unique `key`, and `RecordSessionEvent` appends a `session_events` row for learning-loop observability.

## M2 learning loop (in progress)

The repository substrate is in place; the loop components land across the remaining M2 PRs:

- **Curator + nudges** (PR2) — the agent periodically decides what to persist as observations.
- **Session summarization** (PR2) — at session end the LLM compresses the session into `conversations.summary`.
- **User model** (PR2) — observations about the user aggregate into `user_model` rows with confidence.
- **Recall + close hook** (PR2/PR3) — top-N observations injected into `Brain.Step`; TUI quit triggers `CloseSession`.
- **Skills as procedural memory** (M3) — after complex tasks the agent writes a skill into a skills directory.

Semantic search is a deliberate non-goal: the Repository port keeps the option to bolt on embeddings (e.g. `sqlite-vec`) later without touching domain logic.
