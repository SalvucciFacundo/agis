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
| `embeddings` | `id` (UUID), `doc_type`, `doc_id`, `dimension`, `vector` (BLOB), `created_at`, `updated_at`, UNIQUE(`doc_type`, `doc_id`) — see Hybrid Search below |

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

## Hybrid Search (Lexical BM25 + Semantic Dense Vectors)

AGIS combines full-text keyword search (BM25 via FTS5) with semantic dense vector search (via `core.Embedder`) using Reciprocal Rank Fusion (RRF).

### 1. Pure Go Binary Vector BLOB Storage
Dense float32 vectors are serialized to and from binary SQLite `BLOB`s without external extensions (such as `sqlite-vss` or `sqlite-vec`), preserving zero-dependency single-binary portability:
- Stored as IEEE 754 32-bit floats in LittleEndian format (`len(vector) * 4` bytes).
- Managed in the associative `embeddings` table partitioned by `(doc_type, doc_id)`.

### 2. Pure Go Cosine Similarity
Vector similarity is computed in pure Go:
$$\text{CosineSimilarity}(\vec{u}, \vec{v}) = \frac{\sum_{i=1}^n u_i \cdot v_i}{\sqrt{\sum_{i=1}^n u_i^2} \cdot \sqrt{\sum_{i=1}^n v_i^2}}$$
- Non-matching dimensions evaluate to `0.0`.
- Zero-magnitude vectors evaluate to `0.0`.

### 3. Reciprocal Rank Fusion (RRF)
Search results from BM25 FTS5 and vector similarity rankings are fused using RRF ($k = 60$):
$$\text{RRF\_Score}(d) = \sum_{m \in \{\text{fts}, \text{vec}\}} \frac{1}{60 + \text{rank}_m(d)}$$
where $\text{rank}_m(d) \ge 1$ is the 1-based index in ranking list $m$.
- Documents appearing in only one ranking list contribute $0$ for the missing list.
- Results are deduplicated by `(doc_type, doc_id)` and ordered by `RRF_Score` descending.
- Ties in `RRF_Score` are broken deterministically by `doc_id` ascending.

### 4. Asynchronous Background Embedding Pipeline
When observations are saved in `SaveObservations`:
- The SQLite base table and FTS index updates commit immediately in a synchronous transaction.
- Stale vector embeddings are invalidated.
- Embedding generation tasks are queued to a background worker pool, decoupling LLM provider network latency from agent memory transactions.

### 5. Resilient Graceful Degradation & Fallback
- If `embeddings.enabled` is `false` or `Embedder` is `nil`, `Repository.Search` executes standard BM25 FTS5 keyword matching.
- If the embedding provider fails (e.g. Ollama daemon offline, OpenAI rate limit, network timeout), `Repository.Search` logs a warning and returns BM25 FTS5 results without failing or returning an error to the caller.
