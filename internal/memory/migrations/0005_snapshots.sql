-- 0005_snapshots.sql — M5 session snapshots.
--
-- Additive only: point-in-time copies of a conversation. user_version gates
-- idempotency.

CREATE TABLE snapshots (
    id              TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    title           TEXT NOT NULL DEFAULT '',
    summary         TEXT NOT NULL DEFAULT '',
    messages_json   TEXT NOT NULL,
    created_at      TEXT NOT NULL
);

CREATE INDEX idx_snapshots_conversation_id ON snapshots(conversation_id);
