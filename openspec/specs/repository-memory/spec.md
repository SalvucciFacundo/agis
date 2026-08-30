# Repository Memory Spec

## Purpose

Persist conversations, messages, and observations in SQLite with FTS5 full-text search, managed through a single `Repository` port.

## Requirements

### Requirement: Repository port with M1 subset
`Repository` port MUST expose `CreateConversation`, `LatestConversation`, `AppendMessage`, `Messages(convID, limit)`, `Search(query, limit)`, and `Close`. `AppendMessage` MUST update `conversations.updated_at` and `message_count` transactionally.

#### Scenario: Persist and retrieve messages
- GIVEN a new repository
- WHEN a conversation is created and messages are appended
- THEN `Messages` returns them in order and `LatestConversation` returns the conversation.

### Requirement: SQLite schema
Schema MUST contain `conversations`, `messages`, and `observations` tables. Message roles MUST be one of `user`, `assistant`, `system`, `tool`.

#### Scenario: Schema created by migrations
- GIVEN an empty database
- WHEN migrations apply
- THEN the three tables exist and foreign keys are enforced.

### Requirement: Single FTS5 table with doc_type discriminator
System MUST use a standalone `memory_fts` FTS5 table (`doc_type`, `doc_id`, `content`) with tokenizer `unicode61 remove_diacritics 1`. `Search` MUST match both `message` and `observation` doc types.
(Previously: spec §3 described `observation_fts` over observations and messages.)

#### Scenario: Accent-insensitive search
- GIVEN a persisted message "configuración"
- WHEN `Search` is called with "configuracion"
- THEN the message is returned.

### Requirement: Embedded migrations
Migrations MUST be embedded with `//go:embed migrations/*.sql`. The applier MUST read `PRAGMA user_version`, execute newer files in a transaction, and update `PRAGMA user_version`.
(Previously: spec §3 did not prescribe a migration mechanism.)

#### Scenario: Migrations are idempotent
- GIVEN a database at version 0
- WHEN the repository opens
- THEN `0001_init.sql` applies and `PRAGMA user_version` becomes 1.


repository-memory (MODIFIED)

### Requirement: Extended port
MUST add `SaveObservations`, `Observations`, `UpdateConversationSummary`, `UpsertUserModel`. Upsert on UNIQUE `topic_key`, preserve `created_at`, bump `updated_at`. FTS delete+insert same-tx.
(Previously: 5 methods, no observation writes.)

#### Scenario: Upsert
- GIVEN topic_key=X, created_at=T1 → re-save: created_at=T1, updated_at>T1

#### Scenario: FTS delete-sync
- GIVEN "coffee" indexed → upsert "tea" → "coffee" returns nothing

#### Scenario: Batch atomicity
- GIVEN 3 obs, 2nd invalid → zero persisted

### Requirement: Multi-word AND search
`Search` MUST split on whitespace, quote each term, join AND.
(Previously: M1 exact-phrase wrap — behavior change.)

#### Scenario: AND semantics
- GIVEN msg1="coffee", msg2="preference" → Search("coffee preference") returns zero

#### Scenario: Both terms match
- GIVEN msg="coffee preference noted" → returned

### Requirement: Migration 0002
MUST: (1) ADD updated_at+backfill, (2) UNIQUE topic_key index, (3) CREATE user_model, (4) CREATE session_events CHECK kind IN('nudge','summary','skill'). Idempotent via user_version.

#### Scenario: v1→v2
- GIVEN user_version=1 → 0002 applies, version=2

#### Scenario: Idempotent
- GIVEN user_version=2 → no SQL, version=2

---


repository-memory (MODIFIED)

### Requirement: Skills persistence
Repository port MUST add `SaveSkill` (upsert by unique name, preserving `created_at`), `ListSkills`, and `RecordSkillUsage` (increment `usage_count`, set `last_used`). `ListSkills` MUST order by `last_used` DESC then name.

#### Scenario: Upsert by name
- GIVEN skill "deploy-notes" exists
- WHEN saved again with new content
- THEN one row remains with updated content

#### Scenario: Usage bump
- WHEN RecordSkillUsage runs twice
- THEN usage_count increased by 2

### Requirement: Migration 0003
Migration 0003 MUST create the `skills` table (`id`, UNIQUE `name`, `description`, `trigger`, `content`, `source` CHECK IN(`imported`,`agent`), `usage_count` DEFAULT 0, `last_used`, `created_at`) gated idempotently by `user_version`.

#### Scenario: v2 to v3
- GIVEN user_version=2
- THEN 0003 applies once, version becomes 3


## ADDED Requirements

### Requirement: List and get conversations
The Repository MUST expose `ListConversations(ctx, limit, offset) ([]Conversation, error)` ordered `updated_at DESC, id DESC` and `GetConversation(ctx, id) (*Conversation, error)`.

#### Scenario: List ordering matches latest
- GIVEN LatestConversation returns id X
- THEN ListConversations(1,0)[0].ID == X

### Requirement: Rename conversation
`RenameConversation(ctx, id, title)` MUST update `conversations.title` and bump `updated_at` (so renamed session becomes latest). Title MUST be scanned for injection before write. Empty title MUST error.

#### Scenario: Rename bumps ordering
- GIVEN two conversations A (older), B (latest)
- WHEN A is renamed
- THEN A becomes latest

### Requirement: Snapshots table
Migration 0005 MUST create `snapshots` (`id TEXT PRIMARY KEY`, `conversation_id TEXT NOT NULL`, `title TEXT`, `summary TEXT`, `messages_json TEXT NOT NULL`, `created_at TEXT NOT NULL`) with index on `conversation_id`, gated by `user_version`.

#### Scenario: v4→v5
- GIVEN user_version=4
- THEN 0005 applies once, version becomes 5

---

repository-memory (MODIFIED)

### Requirement AGIS-M7-REPO-001: Binary Float32 Vector BLOB Encoding
The system MUST encode and decode `[]float32` vector arrays to and from raw SQLite `BLOB` byte slices using standard IEEE 754 32-bit floating point binary representation (`encoding/binary.LittleEndian` with `math.Float32bits` / `math.Float32frombits`):
1. The serialized binary BLOB length MUST equal exactly `len(vector) * 4` bytes.
2. An empty or nil vector MUST serialize to an empty byte slice (`[]byte{}`), and an empty BLOB MUST deserialize to an empty `[]float32{}`.
3. Deserializing a BLOB whose length is not a positive multiple of 4 MUST return an error (`ErrMalformedVector`) and MUST NOT panic.

#### Scenario: Roundtrip serialization and deserialization of float32 vector
- GIVEN a vector `[]float32{1.0, -0.5, 3.14159, 0.0}`
- WHEN encoded to binary BLOB and subsequently decoded back
- THEN the decoded vector is identical in length and float values to the original vector

### Requirement AGIS-M7-REPO-002: Cosine Similarity and In-Memory Vector Search
The system MUST compute cosine similarity between two float32 vectors in pure Go without CGO dependencies:
1. Formula: $\text{similarity} = \frac{\mathbf{u} \cdot \mathbf{v}}{\|\mathbf{u}\|_2 \|\mathbf{v}\|_2}$.
2. If either vector has zero magnitude ($\|\mathbf{u}\|_2 = 0$) or if vector lengths differ ($\text{len}(\mathbf{u}) \neq \text{len}(\mathbf{v})$), the function MUST return `0.0`.
3. Identical non-zero vectors MUST yield `1.0` (within $10^{-5}$ float epsilon). Orthogonal vectors MUST yield `0.0`.

#### Scenario: Compute similarity between identical and orthogonal vectors
- GIVEN vector `A = [1.0, 0.0]` and vector `B = [0.0, 1.0]`
- WHEN `CosineSimilarity(A, A)` and `CosineSimilarity(A, B)` are evaluated
- THEN `CosineSimilarity(A, A)` equals `1.0` and `CosineSimilarity(A, B)` equals `0.0`

### Requirement AGIS-M7-REPO-003: Reciprocal Rank Fusion (RRF) Hybrid Search
When hybrid search is active, `Repository.Search(ctx, query, limit)` MUST merge results from BM25 FTS5 keyword search and dense vector semantic search using Reciprocal Rank Fusion:
1. Formula: $\text{RRF}(d) = \sum_{m \in \{\text{fts}, \text{vec}\}} \frac{1}{60 + \text{rank}_m(d)}$, where $\text{rank}_m(d) \ge 1$.
2. Results matching both FTS5 and vector similarity MUST be deduplicated by `(doc_type, doc_id)`, combining their reciprocal rank scores.
3. Results MUST be sorted by final RRF score descending. Deterministic tie-breaking MUST sort by `doc_id` ascending.
4. The final returned slice MUST be truncated to at most `limit` items.

#### Scenario: Query matches both lexical and semantic candidates
- GIVEN a database with observations
- WHEN `Search(ctx, "morning coffee routine", 5)` is executed
- THEN results returned reflect blended RRF scores combining lexical BM25 and vector cosine ranks

### Requirement AGIS-M7-REPO-004: Graceful Degradation & Asynchronous Indexing
1. If embeddings are disabled (`enabled: false`), if the `Embedder` is `nil`, or if the embedder encounters network errors, `Repository.Search` MUST log a warning and fallback to 100% BM25 FTS5 search results without returning an error.
2. Saving observations via `SaveObservations` MUST queue vector generation asynchronously in the background so database write transactions are not blocked by embedding API latency.

#### Scenario: Embedding provider offline falls back to FTS5
- GIVEN an embedding provider that is unreachable
- WHEN `Search(ctx, "database schema", 5)` is invoked
- THEN search executes successfully returning FTS5 keyword results and logs a fallback warning

### Requirement AGIS-M7-REPO-005: Database Migration 0006_embeddings.sql
Migration 0006 MUST create the `embeddings` table:
```sql
CREATE TABLE IF NOT EXISTS embeddings (
    id         TEXT PRIMARY KEY,
    doc_type   TEXT NOT NULL,
    doc_id     TEXT NOT NULL,
    dimension  INTEGER NOT NULL,
    vector     BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(doc_type, doc_id)
);
CREATE INDEX IF NOT EXISTS idx_embeddings_doc ON embeddings(doc_type, doc_id);
PRAGMA user_version = 6;
```

#### Scenario: Migration advances schema to version 6
- GIVEN a database at `user_version = 5`
- WHEN `NewRepository` initializes
- THEN migration 0006 applies idempotently and `PRAGMA user_version` becomes 6


repository-memory (MODIFIED)

### Requirement AGIS-M9-REPO-001: Attachments Storage & Migration 0007_attachments.sql
The persistence layer MUST store message attachments in an `attachments` table linked to `messages(id)`:
```sql
CREATE TABLE IF NOT EXISTS attachments (
    id         TEXT PRIMARY KEY,
    message_id TEXT NOT NULL,
    type       TEXT NOT NULL,
    mime_type  TEXT NOT NULL,
    data       BLOB,
    url        TEXT NOT NULL DEFAULT '',
    name       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    FOREIGN KEY(message_id) REFERENCES messages(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_attachments_msg ON attachments(message_id);
PRAGMA user_version = 7;
```

1. `Repository.AppendMessage` MUST persist message attachments in the same transaction.
2. `Repository.Messages` MUST query and populate the `Attachments` slice for each message.
3. Deleting a conversation or message MUST cascade delete associated attachment rows.

#### Scenario: Attachments saved and loaded transactionally
- GIVEN a message with an image attachment
- WHEN `AppendMessage` and `Messages` are executed
- THEN the returned message includes the exact binary attachment payload


