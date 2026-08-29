# Exploration: m6-ecosystem for AGIS

This exploration investigates the requirements for Milestone 6 (m6-ecosystem), focusing on expanding AGIS into a multi-interface, event-driven agent.

## Core Architectural Findings
The `internal/core/Brain` is well-encapsulated, using a `Sink` pattern for streaming output and supporting context-switching via `SetActiveConversation` and `SetOverlay`. This design is naturally extensible for the gateway and cron requirements.

### 1. Gateway
- **Design**: Introduce `internal/gateway/`.
- **Interface**: Define `Adapter` interface:
  ```go
  type Adapter interface {
      Start(ctx context.Context) error
      Stop() error
      Send(msg string) error
  }
  ```
- **Concurrency**: Each gateway adapter (Telegram, Discord) should run in its own goroutine managed by a gateway supervisor in `internal/gateway/`.
- **Session Mapping**: Use `internal/session/manager.go` to look up or create sessions based on user IDs. The gateway will call `Brain.SetActiveConversation` before `Brain.Step`.

### 2. Cron Scheduler
- **Design**: Introduce `internal/cron/`.
- **Implementation**: Background goroutine using a cron library (like `robfig/cron`).
- **Integration**:
  - `cron` jobs will be defined in `config.yaml`.
  - On trigger, the scheduler will instantiate or look up the appropriate session, set the Brain context, and call `Brain.Step` with the configured prompt.

### 3. Plugin Manager
- **Design**: Extend `internal/skills/` or add `internal/plugins/`.
- **Lifecycle**: The existing `internal/skills/loader.go` provides a foundation. Plugins can be treated as skill-providers.

### 4. Webhook Listener
- **Design**: Introduce `internal/webhook/`.
- **Implementation**: Standard `net/http` server.
- **Security**: Mandatory HMAC signature verification.

## Implementation Path & Trade-offs
- **Graceful Shutdown**: All new components (Gateway, Cron, Webhook) must respect `context.Context` cancellation.
- **Configuration**: The existing `internal/config/` (using `internal/config/config.go`) needs updates to support the new `gateway`, `cron`, `plugins`, and `webhook` configuration blocks.
- **CLI**: `cmd/agis` needs subcommands (`start`, `cron`) to wire these new components.

## Key Learnings
1.  The `Brain`'s `SetActiveConversation` mechanism is key to supporting multi-user sessions across disparate gateways.
2.  `internal/adapters/` is the correct place for gateway implementations, mirroring the existing LLM and TUI adapters.
3.  `internal/config/config.go` should be extended to support polymorphic configurations for adapters and jobs.
4.  Standard library `net/http` and context-based cancellation are sufficient for these additions; no complex framework is required.
