# Architecture

AGIS is a hexagonal (ports & adapters) Go application, mirroring the structure proven in GAIA. The domain lives in `internal/core` and never depends on an adapter; every implementation plugs in behind a port.

## Package layout (M1)

| Package | Role | Depends on |
|---|---|---|
| `cmd/agis` | Entrypoint: flag parsing, config load, wiring, TUI launch | everything |
| `internal/core` | Domain: types, `Provider` port, `Repository` port, `Brain` loop | nothing internal |
| `internal/config` | YAML loader, defaults, precedence, 0600 check | `gopkg.in/yaml.v3` |
| `internal/memory` | `Repository` adapter on SQLite + FTS5, embedded migrations | `core` |
| `internal/adapters/llm` | `Provider` adapters: OpenAI, Ollama, shared client | `core`, `config` |
| `internal/adapters/tui` | Bubbletea TUI: viewport, input, spinner, streaming | `core` |

Six packages in M1. The layers from `spec.md` (`gateway`, `cron`, `skills`, `policy`, `persona`, …) exist only as designed directories; nothing beyond the six above is shipped yet.

## Ports

Two ports define the domain boundary:

**`core.Provider`** (`internal/core/port_llm.go`) — the LLM port.

```go
type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    Stream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
    Models() []ModelInfo
}
```

**`core.Repository`** (`internal/core/port_repository.go`) — the persistence port.

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

Both ports are interfaces over domain types defined in `internal/core/types.go` — `Message`, `Conversation`, `ChatRequest`, `ChatResponse`, `ModelInfo`, `SearchResult`. Adapters implement the interfaces and import `core`; `core` never imports an adapter.

## Data flow

```
TUI (Bubbletea) ──input──▶ Brain.Step ──▶ Provider.Stream ──▶ OpenAI / Ollama
      ▲                          │  │
      │   streamed tokens        │  ▼
      └──────────────────────────┘  Repository (SQLite + FTS5)
```

1. The user presses Enter in the TUI; `submit()` (`internal/adapters/tui/app.go:188`) clears the input and starts a goroutine that calls `Brain.Step`.
2. `Brain.Step` (`internal/core/brain.go:47`) ensures a conversation exists, persists the user message, loads the last 50 messages as context, and calls `Provider.Stream`.
3. Provider tokens flow back through the Brain's `Sink` (`core.WithSink`) into a buffered channel the TUI drains and paints in real time.
4. When the stream ends, `Step` persists the assistant message. A provider error leaves the user message persisted but writes no assistant reply.

The stream channel is buffered (64) so a slow update loop back-pressures the provider instead of dropping tokens (`cmd/agis/main.go:53`).

## StreamEvent contract

`Provider.Stream` returns `(<-chan StreamEvent, error)`:

- `StreamEvent{Text}` — one content delta. Text and Err are mutually exclusive.
- `StreamEvent{Err}` — a terminal mid-stream failure.
- The provider **must** close the channel, including after a terminal error event.
- An immediate failure (bad request, non-200) is returned as the error result, not on the channel.

`Brain.Step` honors the contract: on a mid-stream error it drains the channel to its close before returning, so a blocked provider goroutine never leaks (`internal/core/brain.go:69`).

The shared OpenAI-compatible client (`internal/adapters/llm/client.go`) implements the wire protocol; `NewOpenAI` points it at `https://api.openai.com/v1` and `NewOllama` at `http://localhost:11434/v1`. `llm.NewProvider` selects the Ollama adapter when `provider` is `ollama`; any other value selects the OpenAI-compatible client — which is how alternative endpoints plug in without code changes.

## Dependency direction

- `core` imports no internal package — it is the center of the hexagon.
- `config`, `memory`, and `adapters/*` import `core` (and, where needed, `config`).
- `cmd/agis` is the only composition root; it constructs the repository, provider, brain, and TUI and wires them together.
- The TUI's only dependency on `core` is `*core.Brain` and `core.Repository` — the surface never reaches into storage or the provider directly.
