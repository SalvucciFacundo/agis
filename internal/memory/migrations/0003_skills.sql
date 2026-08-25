-- 0003_skills.sql — M3 procedural-memory schema.
--
-- Additive only: creates the skills table keyed by unique name with an
-- imported/agent source discriminator and usage-tracking columns. The column
-- named "trigger" is quoted because TRIGGER is a reserved word in SQLite.
-- user_version gates idempotency.

CREATE TABLE skills (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    "trigger"   TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL,
    source      TEXT NOT NULL CHECK (source IN ('imported', 'agent')),
    usage_count INTEGER NOT NULL DEFAULT 0,
    last_used   TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);
