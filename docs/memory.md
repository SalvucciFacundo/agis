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
| `observations` | `id` (UUID), `topic_key`, `type`, `content`, `importance`, `created_at`, `source_ref` — reserved for M2; no writer yet in M1 |
| `memory_fts` | standalone FTS5 virtual table — see below |

The M2 learning-loop tables (`skills`, `user_model`, `session_events`) are described in `spec.md` but do not exist yet.

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
- `Search` returns matches ordered by FTS rank across both doc types (`internal/memory/fts.go:33`).
- User queries are wrapped as FTS5 phrases (`ftsQuery`), so punctuation and operator characters are matched literally instead of being interpreted as query syntax. Note: this means M1 search is a single phrase; multi-word free-form query syntax is a known follow-up.

## Embedded migrations

- SQL files are embedded with `//go:embed migrations/*.sql` and versioned by their numeric prefix (`0001_init.sql` → version 1).
- On open, the applier reads `PRAGMA user_version`, runs each newer file in its own transaction, and bumps `user_version` — no external migration tool, consistent with the single-static-binary goal.
- Migrations are idempotent: a database already at version N skips everything below N.
- Non-transactional pragmas (`journal_mode`, `foreign_keys`) are applied separately in `configureDB` because they are no-ops inside a transaction.

## Same-transaction FTS sync

`AppendMessage` inserts the message row, inserts its `memory_fts` row, and updates the conversation's `updated_at` + `message_count` **in one transaction** (`internal/memory/sqlite.go:105`). A reader never observes a message without its search row, a stale count, or a timestamp that is behind — and a failure rolls the whole append back (verified by `TestAppendMessage_FailureRollsBackFTS`).

There are no hidden triggers; the FTS row is written explicitly. Consequences:

- `Search` sees a message immediately after `AppendMessage` commits.
- **Follow-up:** FTS rows are not yet deleted when their base row is deleted (e.g. cascade-deleting a conversation orphans its `memory_fts` rows). This is a documented M1 follow-up.

## M2 vision: the learning loop (designed, not implemented)

The Repository is the substrate the M2 loop will build on:

- **Curator + nudges** — the agent periodically decides what to persist as observations.
- **Topic-key observations** — each observation carries a stable `topic_key` (e.g. `user/preferences/coffee`) and an importance score; same-topic writes update the existing row instead of duplicating.
- **Session summarization** — at session end the LLM compresses the session into `conversations.summary`.
- **User model** — observations about the user aggregate into `user_model` rows with confidence.
- **Skills as procedural memory** — after complex tasks the agent writes a skill (plain-text Markdown with frontmatter) into a skills directory; the skill hub indexes by name, trigger, and description.

Semantic search is a deliberate non-goal: the Repository port keeps the option to bolt on embeddings (e.g. `sqlite-vec`) later without touching domain logic.
