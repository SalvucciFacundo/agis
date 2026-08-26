# Design: m5-full-tui

## Technical Approach

Introduce `internal/session` as the domain owner of the active session id, wrapping the `Repository` port already shipped in M1. The manager holds `activeID string` in memory (re-derived from `LatestConversation` on start), and exposes the 7 slash operations as thin methods that the TUI calls. Brain gains `SetActiveConversation(id)` so that `/new` and `/restore` switch without touching turn logic. Snapshot is a new `snapshots` table storing `messages_json` as a JSON array; listing and rename are direct SQL. TUI commands are 8 new branches in `runCommand` (≤10 lines each, `commandFeedbackPrefix` feedback) reusing the existing re-entrancy gate and sub-model pattern from `/permisos`.

## Architecture Decisions

| # | Decision | Alternatives | Rationale |
|---|----------|--------------|-----------|
| D1 | `internal/session` owns active id, not `Brain` | Fat Brain with `NewSession()` methods | Keeps `brain.go` (530 lines, 7 responsibilities) from growing; matches hexagon — manager is surface-agnostic and testable in isolation, TUI/gateway/cron share it |
| D2 | Snapshot table `snapshots(id, conversation_id, title, summary, messages_json, created_at)` via `0005_snapshots.sql` | File copy of DB, external JSON file | Transactional, queryable, fits `//go:embed` + `PRAGMA user_version` pattern; no FTS indexing needed |
| D3 | `Brain.SetActiveConversation(id)` setter, `ensureConversation` prefers manager id when set | Manager owns full `ensureConversation` | Minimal Brain change (one setter + one branch); existing `ensure → CreateConversation` fallback stays for startup |
| D4 | List ordering constant `ORDER BY updated_at DESC, id DESC` shared between `LatestConversation` and `ListConversations` | Separate queries | Guarantees "latest" == list top; avoids divergence bug (risk noted in exploration) |
| D5 | Title scanned via `internal/scan.Lines` before `RenameConversation` and before `View` rendering | Raw persist | Reuses SOUL.md/skill injection defense; `scan` is already the shared pattern list |
| D6 | Compress reuses `CloseSession`'s summarizer path without closing: manager calls `Summarizer.Close` early on the active conversation's tail | Duplicate summarizer logic | Single summarizer contract, non-fatal, bounded by `closeMessageLimit` |
| D7 | TUI list view reuses `Panel` sub-model pattern vs inline | Inline string join in `app.go` | Sub-model keeps `app.go` from growing past 650 lines; panel already handles tab/j/k/space/q and is testable via `drive` helpers |

## Data Flow

```
User: /new
  TUI runCommand → session.Manager.New(ctx) → repo.CreateConversation(title=New session)
                 → brain.SetActiveConversation(id) → feedback "· new session <id>"

User: hello
  TUI submit → brain.Step → session.Manager.Ensure? → repo.AppendMessage(activeID) → provider → persist reply
  (manager's activeID is the source of truth; ensureConversation reads it)

User: /compress
  TUI (gated !streaming) → manager.Compress(ctx) → repo.Messages(activeID, 200) → SessionCloser.Close
  (same summarizer as CloseSession but without skill creator / summary event)

/restore <id> → manager.Restore(ctx,id) validates GetConversation → SetActive → TUI loadHistory
/snapshot → INSERT INTO snapshots SELECT ... + messages_json
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/session/manager.go` | Create | Manager with activeID, 7 operations, scan on rename |
| `internal/core/port_repository.go` | Modify | Add `ListConversations`, `GetConversation`, `RenameConversation`, `CreateSnapshot`, `ListSnapshots` |
| `internal/memory/migrations/0005_snapshots.sql` | New | `snapshots` table + index, user_version gate |
| `internal/memory/sqlite.go` | Modify | Implement new port methods, shared ordering constant |
| `internal/core/brain.go` | Modify | Add `SetActiveConversation` + branch in `ensureConversation` |
| `internal/adapters/tui/app.go` | Modify | 7 new slash branches, list/panel wiring, gates |
| `internal/adapters/tui/session_panel.go` | New (optional) | Session list sub-model if panel grows beyond inline |
| `cmd/agis/main.go` | Modify | Wire Session Manager, pass to TUI via `WithSessionManager` |
| `docs/sessions.md` | Modify | Flip header to Implemented, document commands |
| `openspec/specs/session-manager/spec.md` | New (via sync) | NEW capability from delta |

## Interfaces / Contracts

```go
// core (consumer side) — additive to Repository port
ListConversations(ctx context.Context, limit, offset int) ([]Conversation, error)
GetConversation(ctx context.Context, id string) (*Conversation, error)
RenameConversation(ctx context.Context, id, title string) error
CreateSnapshot(ctx context.Context, convID string) (*Snapshot, error)
ListSnapshots(ctx context.Context, convID string) ([]Snapshot, error)

type Snapshot struct {
    ID             string
    ConversationID string
    Title          string
    Summary        string
    MessagesJSON   string // JSON array of Message
    CreatedAt      time.Time
}

// internal/session
type Manager struct { repo core.Repository; activeID string; logger *slog.Logger }
func New(repo core.Repository) *Manager
func (m *Manager) ActiveID() string
func (m *Manager) SetActive(id string)
func (m *Manager) NewSession(ctx context.Context) (*core.Conversation, error)
func (m *Manager) List(ctx context.Context, limit int) ([]core.Conversation, error)
func (m *Manager) Restore(ctx context.Context, id string) error
func (m *Manager) Rename(ctx context.Context, id, title string) error
func (m *Manager) Compress(ctx context.Context) error
func (m *Manager) Snapshot(ctx context.Context) (*core.Snapshot, error)
func (m *Manager) Save(ctx context.Context) error // explicit persist (no-op beyond CloseSession hook, feedback only)

// Brain
func (b *Brain) SetActiveConversation(id string)
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Manager: New/SetActive/List ordering, Rename scan, Snapshot insert, Restore switches id, Compress gated | Table-driven with fake Repository (in-memory map) |
| Unit | SQLite: ListConversations ordering, Rename bumps updated_at, Snapshot JSON round-trip, migration 0005 idempotency | Real SQLite via `openTestRepo` (modernc) |
| Unit | TUI slash commands via `drive` helpers: /new creates and switches, /list renders, /restore loads, gated while streaming | `newTestModel` with fake repo + manager |
| Integration | Full turn after /restore appends to restored id, not latest | Scripted provider + real repo |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. Session titles are the only user-supplied display string and are scanned via `internal/scan` (RED test in unit layer).

## Migration / Rollout

Migration 0005 additive-only (`user_version` 4→5); v4 binaries ignore `snapshots` table. No destructive schema. Stacked PRs; each revertible. No feature flag needed: slash commands inert until typed.

## Open Questions

- Snapshot retention policy: no GC in M5, documented as future prune — confirm owner is okay with unbounded growth for now.
