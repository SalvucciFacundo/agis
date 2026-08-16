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
