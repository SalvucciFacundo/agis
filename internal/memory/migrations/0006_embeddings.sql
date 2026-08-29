-- 0006_embeddings.sql — M7 hybrid search embeddings storage.
--
-- Additive only: stores dense float32 binary BLOB vectors mapped to (doc_type, doc_id).
-- user_version gates idempotency.

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
