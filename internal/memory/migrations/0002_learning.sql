-- 0002_learning.sql — M2 learning-loop schema.
--
-- Additive only: no destructive schema changes. Adds the observation upsert
-- substrate (updated_at + unique topic_key), the user_model table, and the
-- session_events observability table. user_version gates idempotency.

ALTER TABLE observations ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';

-- SQLite cannot default one column to another, so backfill updated_at from
-- created_at after the ADD COLUMN.
UPDATE observations SET updated_at = created_at WHERE updated_at = '';

DROP INDEX idx_observations_topic;
CREATE UNIQUE INDEX idx_observations_topic_key ON observations(topic_key);

CREATE TABLE user_model (
    id         TEXT PRIMARY KEY,
    key        TEXT NOT NULL UNIQUE,
    value      TEXT NOT NULL,
    confidence REAL NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
);

CREATE TABLE session_events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    kind       TEXT NOT NULL CHECK (kind IN ('nudge','summary','skill')),
    payload    TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
