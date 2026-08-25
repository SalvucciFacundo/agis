-- 0004_audit.sql — M4 security audit log.
--
-- Additive only: one row per policy decision, grant, revocation, or tier
-- change. scope is empty except for ask resolutions. user_version gates
-- idempotency.

CREATE TABLE audit_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    ts         TEXT NOT NULL,
    backend    TEXT NOT NULL,
    category   TEXT NOT NULL DEFAULT '',
    subject    TEXT NOT NULL DEFAULT '',
    decision   TEXT NOT NULL,
    scope      TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_audit_log_ts ON audit_log(ts);
