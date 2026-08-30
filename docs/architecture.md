# Architecture

AGIS is a hexagonal (ports & adapters) Go application, mirroring the structure proven in GAIA. The domain lives in `internal/core` and never depends on an adapter; every implementation plugs in behind a port.

## Package Layout

| Package | Role | Depends on |
|---|---|---|
| `cmd/agis` | Entrypoint: flag parsing, subcommands routing (`gateway`, `cron`, `plugins`, `webhook`, `policy`, `mcp`), wiring, TUI launch | everything |
| `internal/core` | Domain: types, `Provider` port, `Repository` port, `Brain` loop, `ToolRunner` port, `Approver` port | nothing internal |
| `internal/config` | YAML loader, defaults, precedence, 0600 check, ecosystem blocks, embeddings config, mcp config | `gopkg.in/yaml.v3` |
| `internal/mcp` | Native MCP client: JSON-RPC 2.0 protocol, multiplexing, handshake, tool discovery/call, multi-server Manager | `config`, `core`, `internal/mcp/transport`, `golang.org/x/sync` |
| `internal/mcp/transport` | MCP stream transports: `stdio` (subprocess with process group cleanup) and `sse` (HTTP event stream) | `config` |
| `internal/tools` | Tool runners: Local, Docker, SSH, and MCP ToolRunner bridge (`MCPRunner`) | `core`, `config`, `internal/mcp` |
| `internal/memory` | `Repository` adapter on SQLite + FTS5, binary vector storage, RRF hybrid search, embedded migrations, summarizer, curator | `core` |
| `internal/adapters/llm` | `Provider` adapters (OpenAI, Ollama) & `Embedder` adapters (Ollama, OpenAI) | `core`, `config` |
| `internal/adapters/tui` | Bubbletea TUI: viewport, input, spinner, streaming, slash commands | `core`, `policy`, `session`, `persona` |
| `internal/gateway` | External chat platform adapters (Telegram, Discord), media ingestion helpers, Multiplexer, Auto-deny approver | `core`, `session` |
| `internal/cron` | Background job scheduler, cron parser, interval triggers, gateway notification sender | `core`, `config` |
| `internal/plugins` | External plugin discovery, `plugin.json` manifest parsing, tool/skill registration, state persistence | `core`, `skills` |
| `internal/webhook` | HTTP webhook ingestion server, HMAC-SHA256 signature verification, Brain event dispatch | `core`, `gateway` |
| `internal/policy` | Policy Guard: file-backed policy store, sandbox/standard/full postures, audit logging | `core` |
| `internal/session` | Session Manager: conversation lifecycle, switching, snapshots, renaming, history compression | `core` |
| `internal/skills` | Skill Hub: Markdown skill discovery, keyword matching, agent skill creation | `core` |
| `internal/persona` | SOUL.md durable identity, personality overlays, user-model guided evolution | `core` |

## Domain Ports

Core ports define the domain boundaries:

**`core.Provider`** (`internal/core/port_llm.go`) — the LLM port:
```go
type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    Stream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
    Models() []ModelInfo
}
```

**`core.Repository`** (`internal/core/port_repository.go`) — the persistence port:
```go
type Repository interface {
    CreateConversation(ctx context.Context, title string) (*Conversation, error)
    LatestConversation(ctx context.Context) (*Conversation, error)
    GetConversation(ctx context.Context, id string) (*Conversation, error)
    ListConversations(ctx context.Context) ([]Conversation, error)
    RenameConversation(ctx context.Context, id string, title string) error
    AppendMessage(ctx context.Context, convID string, msg Message) error
    Messages(ctx context.Context, convID string, limit int) ([]Message, error)
    Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
    SaveSkill(ctx context.Context, skill Skill) error
    ListSkills(ctx context.Context) ([]Skill, error)
    Close() error
}
```

**`core.ToolRunner`** (`internal/core/port_learning.go`) — the tool execution port:
```go
type ToolRunner interface {
    Name() string
    Description() string
    Run(ctx context.Context, command string) (string, error)
    Backend() string
}
```

**`core.Approver`** (`internal/core/guard.go`) — the policy decision callback:
```go
type Approver func(ctx context.Context, req GuardRequest) Scope
```

**`core.Embedder`** (`internal/core/port_embedder.go`) — the dense vector embedding port:
```go
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
}
```

**`core.Transcriber`** (`internal/core/port_transcriber.go`) — the audio transcription port:
```go
type Transcriber interface {
    Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error)
}
```

Adapters implement the interfaces and import `core`; `core` never imports an adapter.

## Data Flow & Ecosystem Routing

All external interfaces (TUI, Chat Gateway, Cron Scheduler, Webhooks) drive the exact same `core.Brain` loop and Repository:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          INTERACTION SURFACES                           │
│                                                                         │
│  ┌───────────────┐  ┌────────────────────┐  ┌──────────┐  ┌──────────┐  │
│  │ Bubbletea TUI │  │ Gateway Mux (TG/DC)│  │ Cron Eng │  │ Webhook  │  │
│  └───────┬───────┘  └─────────┬──────────┘  └────┬─────┘  └────┬─────┘  │
└──────────┼────────────────────┼──────────────────┼─────────────┼────────┘
           │                    │                  │             │
           ▼                    ▼                  ▼             ▼
      ┌───────────────────────────────────────────────────────────────┐
      │                           core.Brain                          │
      │  ┌─────────────────────────────────────────────────────────┐  │
      │  │ Step(ctx, input)                                        │  │
      │  │  1. Bind/Restore Conversation ID in Session Manager     │  │
      │  │  2. Append User Message to Repository                   │  │
      │  │  3. Inject SOUL.md, Persona, Skills & Long-Term Memory  │  │
      │  │  4. Stream Provider Turns with Tool Loop (up to 8 rds)  │  │
      │  │  5. Policy Guard & Approver (Sandbox auto-deny / TUI UI)│  │
      │  │  6. Execute Local / Docker / SSH / Plugin ToolRunners   │  │
      │  │  7. Append Assistant Message & Emit to Output Sink      │  │
      │  └─────────────────────────────────────────────────────────┘  │
      └───────────────────────────────┬───────────────────────────────┘
                                      │
              ┌───────────────────────┴───────────────────────┐
              ▼                                               ▼
   ┌───────────────────────┐                     ┌────────────────────────┐
   │     core.Provider     │                     │    core.Repository     │
   │  (OpenAI / Ollama)    │                     │    (SQLite + FTS5)     │
   └───────────────────────┘                     └────────────────────────┘
```

## Ecosystem Architecture

### 1. Gateway Multiplexer & Adapters (`internal/gateway`)
The Gateway subsystem coordinates external chat platforms concurrently:
- **`Adapter` interface**: provides `Name() string`, `Start(ctx) error`, `Stop() error`, and `Send(ctx, target, msg) error`.
- **Telegram Adapter**: uses Telegram Bot API polling to ingest updates and chunk outbound messages at 4096 characters.
- **Discord Adapter**: connects via Discord Gateway events and splits outbound messages at 2000 characters.
- **Multiplexer**: routes events to distinct session IDs (`gateway:<adapter>:<chatID>`), executes `core.Brain.Step`, and transmits replies back via the originating adapter.
- **Security**: static user ID allowlists (`IsAllowed`) enforce fail-closed access control; `AutoDenyApprover` auto-denies unapproved tool actions (`DecisionAsk`) in non-interactive chat environments.

### 2. Cron Scheduler Engine (`internal/cron`)
The Cron subsystem schedules background autonomous tasks:
- **Expression Parsing**: parses standard 5-field cron syntax (`"0 9 * * *"`, `"*/15 * * * *"`, step ranges, macros `@hourly`, `@daily`, `@weekly`, `@monthly`, `@annually`) and duration intervals (`"@every 1h"`).
- **Execution Loop**: wakes periodically to trigger due jobs, executing prompts via `core.Brain.Step` in isolated (`cron:<name>`) or bound sessions under sandbox policy.
- **Target Notification**: forwards completed job outputs to the configured `Sender` (`gateway.Multiplexer` adapter/recipient) or logs output.

### 3. Plugin Manager (`internal/plugins`)
The Plugin subsystem manages modular agent extensions:
- **Manifest Schema (`plugin.json`)**: validates name regex (`^[a-z0-9-_]+$`), semver version, entrypoints, tools, skills, and permissions.
- **Lifecycle Management**: dynamically discovers plugins in `$AGIS_HOME/plugins/`, tracks enabled/disabled status in `state.json`, and exposes `Load`, `List`, `Enable`, `Disable`, and `inspect` APIs.
- **Tool Bridge**: wraps plugin entrypoints as `PluginRunner` (`core.ToolRunner`) with backend identifier `plugin-<name>`.
- **Skill Hub Integration**: extracts markdown skills declared in manifests into AGIS Skill Hub.

### 4. Webhook Listener Server (`internal/webhook`)
The Webhook subsystem enables HTTP event ingestion:
- **HTTP Handler**: listens on configured host/port, routing POST requests at configured path (`/webhook` or `/events`). Non-POST requests return `405 Method Not Allowed`.
- **HMAC-SHA256 Verification**: verifies incoming signatures in `X-Hub-Signature-256` or `X-Signature` headers using constant-time comparison (`crypto/subtle.ConstantTimeCompare`). Missing or invalid signatures return `401 Unauthorized`.
- **Dispatch & Target Forwarding**: extracts JSON event types into session keys (`webhook:<event_type>`), triggers `core.Brain.Step`, and forwards responses to chat gateway targets.

### 5. Model Context Protocol (MCP) Client (`internal/mcp`)
The MCP subsystem provides native tool integration with MCP-compliant servers:
- **JSON-RPC 2.0 Client**: pure Go client implementing MCP specification (`2024-11-05`), atomic ID correlation, and lifecycle handshakes.
- **Transports (`internal/mcp/transport`)**: `stdio` subprocess with POSIX process group cleanup (`Setpgid`) and `sse` network event streaming over HTTP.
- **Multi-Server Manager**: coordinates server pools, concurrent initialization, server health tracking, and tool aggregation.
- **ToolRunner Bridge (`internal/tools`)**: wraps MCP tools as `MCPRunner` under backend `mcp:<server_name>`, enforcing `PolicyGuard` security postures.

## StreamEvent Contract

`Provider.Stream` returns `(<-chan StreamEvent, error)`:
- `StreamEvent{Text}` — one content delta. Text and Err are mutually exclusive.
- `StreamEvent{ToolCall}` — tool invocation requested by model.
- `StreamEvent{Err}` — a terminal mid-stream failure.
- The provider **must** close the channel, including after a terminal error event.
- An immediate failure (bad request, non-200) is returned as the error result, not on the channel.

`Brain.Step` honors the contract: on a mid-stream error it drains the channel to its close before returning, preventing goroutine leaks.

## Dependency Direction

- `core` imports no internal package — it is the center of the hexagon.
- Adapters, memory, config, gateway, cron, plugins, webhook, and policy import `core`.
- `cmd/agis` is the composition root; it routes subcommands, instantiates dependencies, and wires subsystems together.
- Surfaces (TUI, Gateway, Cron, Webhook) interact with storage and LLMs exclusively through domain interfaces.
