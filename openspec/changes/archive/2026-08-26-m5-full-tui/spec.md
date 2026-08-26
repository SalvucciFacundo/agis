# Delta for session-manager

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

# Delta for repository-memory

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

# Delta for brain-loop

## MODIFIED Requirements

### Requirement: Active conversation tracking
The Brain MUST delegate active session tracking to the Session Manager via `SetActiveConversation(id)`. `ensureConversation` MUST prefer the manager's active id when set, falling back to `LatestConversation` when empty (startup). `Step` MUST continue appending to the active id.

#### Scenario: Restore switches active session
- GIVEN manager active id set to past conversation
- WHEN Step is invoked
- THEN messages append to that id, not to latest

---

# Delta for minimal-tui

## MODIFIED Requirements

### Requirement: Session slash commands
Input beginning with `/` that matches `/new`, `/reset`, `/save`, `/list`, `/restore`, `/compress`, `/snapshot`, `/rename` MUST dispatch locally and MUST NOT reach the provider nor persist as a message. Unknown slash MUST print an error line. All session commands MUST be gated with `streaming || closing` check.

#### Scenario: Unknown slash
- GIVEN `/unknown`
- THEN error line appears and no provider call occurs

#### Scenario: Commands gated while streaming
- GIVEN streaming true
- WHEN `/new` is invoked
- THEN it is ignored

### Requirement: Session feedback and views
`/list` MUST render id, title, created_at from `ListConversations`. `/restore` MUST load summary + tail into viewport. `/save` MUST trigger an explicit persist without quitting. Feedback lines MUST use `commandFeedbackPrefix`.

#### Scenario: Save feedback
- GIVEN `/save`
- THEN viewport shows `· saved` and no new conversation is created
