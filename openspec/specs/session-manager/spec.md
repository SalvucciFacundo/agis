# Session Manager Spec

## Purpose

Session lifecycle owned independent of surface. Active session id, 7 slash operations, snapshot point-in-time copies, and ordering guarantees.

## ADDED Requirements

### Requirement: Session lifecycle ownership
The Session Manager MUST own the active session id and expose Start/New, Save, Close, Restore, List, Rename, Compress, Snapshot. It MUST be surface-agnostic: TUI, gateway and cron attach to the same manager. The active id MUST survive only in memory and be re-derived from `LatestConversation` on restart.

#### Scenario: New session starts clean
- GIVEN an active conversation with messages
- WHEN `/new` is invoked
- THEN a new conversation is created and the next turn uses the new id

#### Scenario: Restore continues session
- GIVEN a past conversation id
- WHEN `/restore <id>` is invoked
- THEN the active id switches and subsequent turns append to that conversation

### Requirement: List and rename
The Manager MUST list recent sessions ordered `updated_at DESC, id DESC` and MUST rename via scanned title (injection patterns dropped). Empty title MUST be rejected.

#### Scenario: List shows recent sessions
- GIVEN 3 conversations created at T1 < T2 < T3
- WHEN `/list` is invoked
- THEN ids appear T3, T2, T1 with title and created_at

#### Scenario: Rename persists
- GIVEN `/rename My Research`
- THEN `GetConversation` returns title `My Research` and `/list` shows it

### Requirement: Snapshot point-in-time copy
Snapshot MUST insert a row in `snapshots` (`id`, `conversation_id`, `title`, `summary`, `messages_json`, `created_at`) without changing the active session. Snapshots MUST NOT be indexed by FTS.

#### Scenario: Snapshot does not switch session
- GIVEN active id A
- WHEN `/snapshot` is invoked
- THEN active id remains A and a snapshot row exists

### Requirement: Compress early summarization
Compress MUST run the summarizer path early (same as `CloseSession`'s summarizer step) without closing the session. It MUST be gated while `streaming || closing`.

#### Scenario: Compress gated while streaming
- GIVEN streaming is true
- WHEN `/compress` is invoked
- THEN it is ignored and no summarization runs

---
