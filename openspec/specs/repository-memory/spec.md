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


repository-memory (MODIFIED)

### Requirement: Extended port
MUST add `SaveObservations`, `Observations`, `UpdateConversationSummary`, `UpsertUserModel`. Upsert on UNIQUE `topic_key`, preserve `created_at`, bump `updated_at`. FTS delete+insert same-tx.
(Previously: 5 methods, no observation writes.)

#### Scenario: Upsert
- GIVEN topic_key=X, created_at=T1 → re-save: created_at=T1, updated_at>T1

#### Scenario: FTS delete-sync
- GIVEN "coffee" indexed → upsert "tea" → "coffee" returns nothing

#### Scenario: Batch atomicity
- GIVEN 3 obs, 2nd invalid → zero persisted

### Requirement: Multi-word AND search
`Search` MUST split on whitespace, quote each term, join AND.
(Previously: M1 exact-phrase wrap — behavior change.)

#### Scenario: AND semantics
- GIVEN msg1="coffee", msg2="preference" → Search("coffee preference") returns zero

#### Scenario: Both terms match
- GIVEN msg="coffee preference noted" → returned

### Requirement: Migration 0002
MUST: (1) ADD updated_at+backfill, (2) UNIQUE topic_key index, (3) CREATE user_model, (4) CREATE session_events CHECK kind IN('nudge','summary','skill'). Idempotent via user_version.

#### Scenario: v1→v2
- GIVEN user_version=1 → 0002 applies, version=2

#### Scenario: Idempotent
- GIVEN user_version=2 → no SQL, version=2

---
