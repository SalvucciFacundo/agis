-- 0007_attachments.sql — M9 multimodal attachment storage.
--
-- Additive only: stores media attachments (images/audio) linked to messages.
-- user_version gates idempotency.

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
