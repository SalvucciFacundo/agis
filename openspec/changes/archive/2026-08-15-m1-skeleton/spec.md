# M1 — Thinking agent with memory (delta spec)

Delta spec for `m1-skeleton`: first AGIS milestone — skeleton with brain loop, LLM port, SQLite+FTS5, config, TUI.

## Capability: config-loader

### Requirement AGIS-M1-CONF-001: Load configuration from YAML
The system MUST load `~/.agis/config.yaml` with mode `0600`, warning if looser. Precedence MUST be `-config` flag > `AGIS_HOME` > default path. M1 fields MUST include `llm.provider`, `llm.model`, `llm.api_key`, and `db.path`.

#### Scenario: Config loads with defaults
- GIVEN `~/.agis/config.yaml` is missing
- WHEN application starts
- THEN it uses built-in defaults.

## Capability: repository-memory

### Requirement AGIS-M1-REPO-001: Repository port with M1 subset
`Repository` port MUST expose `CreateConversation`, `LatestConversation`, `AppendMessage`, `Messages(convID, limit)`, `Search(query, limit)`, and `Close`. `AppendMessage` MUST update `conversations.updated_at` and `message_count` transactionally.

#### Scenario: Persist and retrieve messages
- GIVEN a new repository
- WHEN a conversation is created and messages are appended
- THEN `Messages` returns them in order and `LatestConversation` returns the conversation.

### Requirement AGIS-M1-REPO-002: SQLite schema
Schema MUST contain `conversations`, `messages`, and `observations` tables. Message roles MUST be one of `user`, `assistant`, `system`, `tool`.

#### Scenario: Schema created by migrations
- GIVEN an empty database
- WHEN migrations apply
- THEN the three tables exist and foreign keys are enforced.

### Requirement AGIS-M1-REPO-003: Single FTS5 table with doc_type discriminator
System MUST use a standalone `memory_fts` FTS5 table (`doc_type`, `doc_id`, `content`) with tokenizer `unicode61 remove_diacritics 1`. `Search` MUST match both `message` and `observation` doc types.
(Previously: spec §3 described `observation_fts` over observations and messages.)

#### Scenario: Accent-insensitive search
- GIVEN a persisted message "configuración"
- WHEN `Search` is called with "configuracion"
- THEN the message is returned.

### Requirement AGIS-M1-REPO-004: Embedded migrations
Migrations MUST be embedded with `//go:embed migrations/*.sql`. The applier MUST read `PRAGMA user_version`, execute newer files in a transaction, and update `PRAGMA user_version`.
(Previously: spec §3 did not prescribe a migration mechanism.)

#### Scenario: Migrations are idempotent
- GIVEN a database at version 0
- WHEN the repository opens
- THEN `0001_init.sql` applies and `PRAGMA user_version` becomes 1.

## Capability: llm-provider-port

### Requirement AGIS-M1-LLM-001: Provider port and adapters
`Provider` port MUST expose `Chat`, `Stream`, and `Models`. `Stream` MUST return `(<-chan StreamEvent, error)` with `StreamEvent{Text, Err}`. M1 adapters MUST be OpenAI and Ollama via a shared OpenAI-compatible client.
(Previously: spec §2 defined `Stream(ctx, ChatRequest) (<-chan Token, error)`.)

#### Scenario: Stream emits text events
- GIVEN a fake provider streams "hello" then "world"
- WHEN `Stream` is called
- THEN the channel yields both tokens and closes.

#### Scenario: Stream surfaces mid-stream errors
- GIVEN a provider fails mid-stream
- WHEN `Stream` is called
- THEN the channel yields a `StreamEvent{Err: ...}` and closes.

### Requirement AGIS-M1-LLM-002: Static Models list
`Models()` MUST return the static model from `llm.model` in M1. Live enumeration is deferred to M4.

#### Scenario: Models returns configured entry
- GIVEN `llm.provider: openai` and `llm.model: gpt-4o-mini`
- WHEN `Models()` is called
- THEN it returns one `ModelInfo` with the configured values.

## Capability: brain-loop

### Requirement AGIS-M1-BRAIN-001: Brain.Step loop
`Brain.Step(ctx, input)` MUST persist the user message, load the tail, call `Provider.Stream`, forward tokens to a sink, and persist the assistant message. Tool calls are logged and ignored in M1.

#### Scenario: Step streams and persists
- GIVEN a fake provider streams "Hi"
- WHEN `Step` is called with "Hello"
- THEN both messages are persisted and the sink receives "Hi".

#### Scenario: Step handles provider errors
- GIVEN `Stream` returns an error
- WHEN `Step` is called
- THEN the error is returned, the user message is persisted, and no assistant message is written.

## Capability: minimal-tui

### Requirement AGIS-M1-TUI-001: Minimal Bubbletea TUI
TUI MUST render viewport, text input, and spinner. Enter sends input to `Brain.Step`, streams tokens into the viewport, and restores the latest conversation on startup.

#### Scenario: Send a message
- GIVEN the app launches
- WHEN the user types "Hello" and presses Enter
- THEN the message is persisted and streamed response appears.
