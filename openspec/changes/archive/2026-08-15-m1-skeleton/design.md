# Design: M1 — Thinking agent with memory (Go skeleton)

## Technical Approach

Build a GAIA-aligned hexagonal skeleton for AGIS. `internal/core` owns the domain types, the `Provider` and `Repository` ports, and the `Brain` loop. Adapters live under `internal/adapters` and `internal/memory`. Wiring is manual constructor injection in `cmd/agis/main.go`. All M1 code is real and tested; no placeholder directories.

## Architecture Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Stream contract | `StreamEvent{Text, Err}` on a receive-only channel | A bare token channel cannot carry mid-stream failures; this amendment is used by every downstream consumer from M1 onward. |
| FTS5 layout | Single `memory_fts(doc_type, doc_id, content)` table | FTS5 external-content mode references only one base table, so the spec's "over messages + observations" needs either two tables+triggers or this discriminator. Explicit same-transaction inserts avoid hidden SQL. |
| Migrations | `//go:embed migrations/*.sql` + `PRAGMA user_version` | Keeps AGIS a single static binary with zero external services; justified because SQLite is embedded and single-writer, not a shared production DB. |
| DI | Manual constructor injection | Matches GAIA and keeps M1 simple; no framework needed for two ports. |
| CLI flags | stdlib `flag` | Only `-config` is needed in M1; Cobra lands with subcommands in M4. |

## Directory Skeleton

```
cmd/agis/main.go
internal/config/config.go
internal/core/types.go
internal/core/port_llm.go
internal/core/port_repository.go
internal/core/brain.go
internal/adapters/llm/client.go
internal/adapters/llm/openai.go
internal/adapters/llm/ollama.go
internal/adapters/tui/app.go
internal/memory/sqlite.go
internal/memory/fts.go
internal/memory/migrations/0001_init.sql
```

No `internal/{skills,tools,policy,persona,gateway,mcp,cron,plugins,webhook}` or `pkg/` in M1.

## Data Flow

```
TUI ──Enter──► Brain.Step
                │
                ├──► Repository.AppendMessage(user)
                ├──► Repository.Messages(tail)
                ├──► Provider.Stream ──► TUI viewport
                └──► Repository.AppendMessage(assistant)
```

## Interfaces / Contracts

`internal/core/port_repository.go`:

```go
type Repository interface {
    CreateConversation(ctx context.Context, title string) (*Conversation, error)
    LatestConversation(ctx context.Context) (*Conversation, error)
    AppendMessage(ctx context.Context, convID string, msg Message) error
    Messages(ctx context.Context, convID string, limit int) ([]Message, error)
    Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
    Close() error
}
```

`internal/core/port_llm.go`:

```go
type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    Stream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
    Models() []ModelInfo
}

type StreamEvent struct {
    Text string
    Err  error
}
```

`internal/core/types.go`:

```go
type Role string
const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleSystem    Role = "system"
    RoleTool      Role = "tool"
)

type Message struct {
    ID             int64
    ConversationID string
    Role           Role
    Content        string
    CreatedAt      time.Time
}

type Conversation struct {
    ID           string
    Title        string
    CreatedAt    time.Time
    UpdatedAt    time.Time
    Summary      string
    MessageCount int
}

type ChatRequest struct {
    Model    string
    Messages []Message
}

type ChatResponse struct {
    Content string
}

type ModelInfo struct {
    ID       string
    Provider string
}

type SearchResult struct {
    DocType string
    DocID   string
    Content string
}
```

`internal/config/config.go`:

```go
type Config struct {
    LLM LLMConfig `yaml:"llm"`
    DB  DBConfig  `yaml:"db"`
}

type LLMConfig struct {
    Provider string `yaml:"provider"`
    Model    string `yaml:"model"`
    APIKey   string `yaml:"api_key"`
}

type DBConfig struct {
    Path string `yaml:"path"`
}
```

Load precedence: `-config` flag > `AGIS_HOME` env > default `~/.agis/config.yaml`. Default values: provider `ollama`, model `llama3.2`, db.path `~/.agis/agis.db`. Warn (stderr) if mode is not `0600`.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `cmd/agis/main.go` | Create | Flags, config load, wiring, `tea.Program` |
| `internal/config/config.go` | Create | YAML loader, defaults, permission check |
| `internal/core/*.go` | Create | Domain types, ports, brain loop |
| `internal/adapters/llm/*.go` | Create | Shared client + OpenAI/Ollama adapters |
| `internal/adapters/tui/app.go` | Create | Bubbletea model/update/view |
| `internal/memory/*.go` | Create | SQLite Repository, FTS sync, migrations applier |
| `internal/memory/migrations/0001_init.sql` | Create | Schema + FTS5 table |
| `go.mod` / `go.sum` | Create | Pinned dependencies |
| `Makefile`, `.gitignore`, `.golangci.yml` | Create | Project essentials |

## Schema DDL

`internal/memory/migrations/0001_init.sql`:

```sql
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
```

`AppendMessage` inserts the message and a `memory_fts` row with `doc_type='message'` in the same transaction, then updates `conversations.updated_at` and `message_count`. Observations are schema-only in M1; future `SaveObservation` will use `doc_type='observation'`.

## Wiring / Composition Root

`cmd/agis/main.go`:

1. Parse `-config`.
2. `cfg, err := config.Load(*configPath)`.
3. `repo, err := memory.NewRepository(ctx, cfg.DB.Path)` — runs embedded migrations.
4. `provider := llm.NewProvider(cfg.LLM)` — selects OpenAI or Ollama by `cfg.LLM.Provider`.
5. `brain := core.NewBrain(cfg, repo, provider)`.
6. `app := tui.New(brain, repo)`.
7. `tea.NewProgram(app).Run()`.

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Config loader | Missing file, defaults, override precedence, permission warning (`t.TempDir`) |
| Unit | Repository | In-memory/temp-file SQLite; CRUD, ordering, `message_count`, cascade delete |
| Unit | FTS5 | Accent-insensitive `configuración`/`configuracion`, doc_type filtering, same-tx sync |
| Unit | LLM adapters | `httptest` SSE server; token order and `StreamEvent.Err` |
| Unit | Brain | Fake `Provider` + temp repository; persisted user message, assistant streaming, error cases |
| Integration | TUI wiring | `tea.NewProgram` smoke test with fake provider (optional in M1) |

Use `goleak.VerifyTestMain` to catch stream goroutine leaks.

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary in M1.

## Migration / Rollout

No migration required. M1 is greenfield; embedded migrations create the schema on first run.

## Risks and Tradeoffs

- **StreamEvent amendment**: changes the spec's `<-chan Token` to `<-chan StreamEvent`. This is the smallest fix that lets M2–M6 consumers handle streaming errors without a second error path.
- **FTS5 discriminator table**: deviates from a literal "FTS over messages + observations" reading, but avoids triggers and keeps writes explicit and testable.
- **Embedded migrations**: deviates from the `golang-database` external-tool rule, justified by the single-binary, zero-external-services goal.

## Open Questions

None.
