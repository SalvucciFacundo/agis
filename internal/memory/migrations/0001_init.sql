PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE conversations (
    id            TEXT PRIMARY KEY,
    title         TEXT NOT NULL DEFAULT 'New session',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    summary       TEXT NOT NULL DEFAULT '',
    message_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK (role IN ('user','assistant','system','tool')),
    content         TEXT NOT NULL,
    created_at      TEXT NOT NULL
);
CREATE INDEX idx_messages_conv ON messages(conversation_id, id);

CREATE TABLE observations (
    id          TEXT PRIMARY KEY,
    topic_key   TEXT NOT NULL,
    type        TEXT NOT NULL DEFAULT 'note',
    content     TEXT NOT NULL,
    importance  INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    source_ref  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_observations_topic ON observations(topic_key);

CREATE VIRTUAL TABLE memory_fts USING fts5(
    doc_type UNINDEXED,
    doc_id   UNINDEXED,
    content,
    tokenize = 'unicode61 remove_diacritics 1'
);
