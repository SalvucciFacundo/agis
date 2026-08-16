# Design: M2 — Learning Loop

## Technical Approach

Close the M1 write-only memory loop with three new domain components (`Curator`, `Summarizer`, `UserModel`) under `internal/memory`, four new `core.Repository` port methods, minimal top-N recall injection in `Brain.Step`, and a synchronous non-fatal TUI close hook. One combined `Provider.Chat` call per nudge/close returns structured JSON; parse failures log+skip. Migration 0002 adds `updated_at`, unique `topic_key`, `user_model`, and `session_events`.

## Architecture Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Port extension | Extend `core.Repository` with 4 methods | Spec REPO-001 mandates port-level methods; keeps hexagonal direction |
| Curator placement | `internal/memory/curator.go` | Spec layout: "memory store, curator, summarizer, user model" |
| Close call shape | ONE `Chat` returning `{summary, observations[]}` | Halves LLM cost vs two calls; spec SUM-001 |
| Nudge trigger | Message-count (not timer) | Deterministic in tests; no mid-turn interruption |
| FTS delete-sync | Same-tx DELETE before INSERT on upsert | Prevents stale content haunting search |
| Search semantics | AND-join per whitespace token | Observation recall makes multi-word the common case |
| Close blocking | Synchronous + `close_timeout` (30s) | Fire-and-forget risks lost writes at exit |
| user_model.key | Full `topic_key` | Traceable back to source observation |
| session_events.kind | Include `'skill'` in CHECK | SQLite CHECK cannot ALTER; reserves M3 slot |

## Data Flow

```
Brain.Step:
  user input → AppendMessage → load tail → Recall(top-N obs) →
  prepend obs to system prompt → Provider.Stream → AppendMessage →
  if assistant_count % nudge_every == 0 → Curator.Nudge → SaveObservations

Brain.CloseSession:
  load msgs (cap 200) → Provider.Chat(combined prompt) →
  parse {summary, observations[]} →
  UpdateConversationSummary → SaveObservations → UpsertUserModel →
  session_event(kind='summary')

TUI CtrlC:
  if streaming → cancel ctx → drain streamDoneMsg →
  "closing session…" → Brain.CloseSession(30s ctx) → tea.Quit
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/memory/migrations/0002_learning.sql` | Create | DDL: updated_at+backfill, unique topic_key, user_model, session_events |
| `internal/core/types.go` | Modify | Add `Observation` (with UpdatedAt), `UserModel` domain types |
| `internal/core/port_repository.go` | Modify | Add `SaveObservations`, `Observations`, `UpdateConversationSummary`, `UpsertUserModel` |
| `internal/memory/sqlite.go` | Modify | Implement 4 new repo methods; observation upsert with FTS delete-sync in one tx |
| `internal/memory/fts.go` | Modify | Add `deleteFTSRow` helper; change `ftsQuery` to AND-join per-token |
| `internal/memory/curator.go` | Create | `Curator` type, `Nudge(ctx, msgs)` method, fence-stripping JSON parse |
| `internal/memory/summarizer.go` | Create | `Summarizer` type, `Close(ctx, msgs)` returning `{summary, observations[]}` |
| `internal/memory/usermodel.go` | Create | Pure `AggregateUserModel(obs)` function |
| `internal/core/brain.go` | Modify | Recall injection in `Step`, `CloseSession(ctx)`, assistant-msg counter, nudge trigger |
| `internal/adapters/tui/app.go` | Modify | CtrlC/Esc close sequence: cancel stream → drain → CloseSession → quit |
| `internal/config/config.go` | Modify | `MemoryConfig` block: `learning_enabled`, `nudge_every`, `recall_limit`, `close_timeout` |
| `cmd/agis/main.go` | Modify | Wire `Curator`, `Summarizer`, pass config to Brain |
| `*_test.go` alongside each | Modify/Create | Tests per requirement (see Testing Strategy) |

## Interfaces / Contracts

### Migration 0002 DDL
```sql
ALTER TABLE observations ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';
UPDATE observations SET updated_at = created_at WHERE updated_at = '';
DROP INDEX idx_observations_topic;
CREATE UNIQUE INDEX idx_observations_topic_key ON observations(topic_key);

CREATE TABLE user_model (
    id TEXT PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    confidence REAL NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
);

CREATE TABLE session_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('nudge','summary','skill')),
    payload TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
```
Idempotency: `applyMigrations` skips when `user_version >= 2`.

### Domain Types
```go
// core/types.go
type Observation struct {
    ID          string
    TopicKey    string
    Type        string
    Content     string
    Importance  int
    CreatedAt   time.Time
    UpdatedAt   time.Time
    SourceRef   string
}

type UserModel struct {
    ID         string
    Key        string
    Value      string
    Confidence float64
    UpdatedAt  time.Time
}
```

### Repository Port Extension
```go
// core/port_repository.go — add to existing Repository interface
SaveObservations(ctx context.Context, convID string, obs []Observation) error
  // Upsert on UNIQUE(topic_key). Preserve created_at, bump updated_at.
  // FTS delete+insert same-tx. Batch atomic: one bad row → rollback all.
  // Importance clamped [1,5], default 3 when zero.

Observations(ctx context.Context, limit int) ([]Observation, error)
  // Recall: ORDER BY updated_at DESC LIMIT N.

UpdateConversationSummary(ctx context.Context, convID, summary string) error
  // MUST NOT bump conversations.updated_at.

UpsertUserModel(ctx context.Context, rows []UserModel) error
  // Upsert on UNIQUE(key). Bump updated_at.
```

### FTS Delete-Sync Helper
```go
// fts.go
func deleteFTSRow(ctx context.Context, tx *sql.Tx, docType, docID string) error

// ftsQuery change: split on whitespace, quote each, join AND
func ftsQuery(query string) string {
    tokens := strings.Fields(query)
    quoted := make([]string, len(tokens))
    for i, t := range tokens {
        quoted[i] = `"` + strings.ReplaceAll(t, `"`, `""`) + `"`
    }
    return strings.Join(quoted, " AND ")
}
```

### Curator
```go
// memory/curator.go
type Curator struct {
    provider core.Provider
    repo     core.Repository
    logger   *slog.Logger
}

func NewCurator(provider core.Provider, repo core.Repository, logger *slog.Logger) *Curator

// Nudge runs the curator prompt over msgs, parses JSON, calls SaveObservations.
// Parse failure → log+return nil (never error). Returns parsed observations.
func (c *Curator) Nudge(ctx context.Context, convID string, msgs []core.Message) ([]core.Observation, error)

// stripFences removes ```json ... ``` wrappers before JSON parse.
// importance defaults to 3 when absent/zero.
```

### Summarizer
```go
// memory/summarizer.go
type Summarizer struct {
    provider core.Provider
    repo     core.Repository
    logger   *slog.Logger
}

func NewSummarizer(provider core.Provider, repo core.Repository, logger *slog.Logger) *Summarizer

// Close runs ONE Chat call returning {summary, observations[]}.
// Calls UpdateConversationSummary + SaveObservations. Non-fatal on error.
func (s *Summarizer) Close(ctx context.Context, convID string, msgs []core.Message) error
```

### User Model Aggregation
```go
// memory/usermodel.go

// AggregateUserModel is a pure function. Only observations with topic_key
// prefix "user/" participate. key = full topic_key. First write:
// confidence = clamp(importance/5, 0, 1). Update: clamp(0.7*old + 0.3*new, 0, 1).
func AggregateUserModel(existing []core.UserModel, obs []core.Observation) []core.UserModel
```

### Brain Amendments
```go
// core/brain.go — add fields
type Brain struct {
    repo         Repository
    provider     Provider
    sink         Sink
    curator      *memory.Curator      // nil when learning_enabled=false
    summarizer   *memory.Summarizer   // nil when learning_enabled=false
    recallLimit  int                  // default 10
    nudgeEvery   int                  // default 10; 0 disables
    assistantCount int                // counter for nudge cadence
}

// Step amendments:
// 1. After loading tail, call repo.Observations(ctx, recallLimit)
// 2. Prepend observations to system prompt (format: "Relevant memories:\n- ...")
// 3. After AppendMessage(assistant), increment assistantCount
// 4. If nudgeEvery > 0 && assistantCount % nudgeEvery == 0 → curator.Nudge
// 5. Record session_event(kind='nudge') on nudge

// CloseSession orchestrates close:
func (b *Brain) CloseSession(ctx context.Context) error {
    // 1. ensureConversation
    // 2. load msgs (cap 200)
    // 3. summarizer.Close(ctx, convID, msgs) — runs Chat, writes summary+obs
    // 4. Aggregate user/* observations → UpsertUserModel
    // Errors non-fatal: log + continue
}
```

### TUI Close Hook
```go
// adapters/tui/app.go — Model amendments
type Model struct {
    // ... existing fields
    cancel     context.CancelFunc  // cancel func for Step ctx
    closing    bool                // true after first CtrlC during close
}

// CtrlC/Esc handler:
// if streaming → m.cancel() (cancels Step ctx), wait for streamDoneMsg
// if streamDoneMsg received → m.closing=true, show "closing session…"
//   → run Brain.CloseSession(30s ctx) → tea.Quit
// if m.closing && second CtrlC → tea.Quit immediately (force quit)
// if idle → m.closing=true → CloseSession → tea.Quit

// submit() uses context.WithCancel so CtrlC can cancel the in-flight Step
```

### Config
```go
// config/config.go
type MemoryConfig struct {
    LearningEnabled bool          `yaml:"learning_enabled"` // default true
    NudgeEvery      int           `yaml:"nudge_every"`      // default 10
    RecallLimit     int           `yaml:"recall_limit"`     // default 10
    CloseTimeout    time.Duration `yaml:"close_timeout"`    // default 30s
}

// Config struct adds: Memory MemoryConfig `yaml:"memory"`
// applyDefaults sets Memory defaults
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Curator parse | Fake Provider returns fenced JSON → parsed obs correct; malformed → zero writes; missing importance → 3 |
| Unit | Summarizer | Fake Provider returns `{summary, obs[]}` → `UpdateConversationSummary` + `SaveObservations` called; provider error → graceful |
| Unit | User model | `AggregateUserModel` pure: `user/*` included, others excluded, confidence math |
| Unit | FTS delete-sync | Upsert obs with same topic_key → old content no longer matches in Search |
| Unit | AND search | `ftsQuery("coffee preference")` → `"coffee" AND "preference"`; Search returns only rows matching both |
| Unit | Migration 0002 | Apply to v1 → v2; apply again → no-op (idempotent via user_version) |
| Integration | Observation upsert | Same topic_key twice → 1 row, created_at preserved, updated_at bumped; batch atomicity |
| Integration | Brain recall | Step prepends top-N observations to provider request (assert via fake provider capturing ChatRequest) |
| Integration | Brain.CloseSession | Order: summary → obs → user_model; stream cancel + drain (goleak) |
| E2E | TUI close | CtrlC idle → close cmd → quit; CtrlC streaming → cancel → drain → close; CtrlC×2 → force quit |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary.

## Migration / Rollout

Migration 0002 is additive (no destructive schema changes). `user_version` gating ensures idempotency. `memory.learning_enabled: false` reverts to M1 behavior (no LLM calls at close) for cost-sensitive deployments. No feature flags needed beyond the config kill switch.

## Open Questions

- [ ] None — all decisions resolved in proposal.

## Wiring (main.go)

```go
// cmd/agis/main.go amendments
curator := memory.NewCurator(provider, repo, logger)
summarizer := memory.NewSummarizer(provider, repo, logger)

brain := core.NewBrain(repo, provider,
    core.WithSink(func(text string) { stream <- text }),
    core.WithCurator(curator),           // new option
    core.WithSummarizer(summarizer),     // new option
    core.WithRecallLimit(cfg.Memory.RecallLimit),
    core.WithNudgeEvery(cfg.Memory.NudgeEvery),
)

app := tui.New(brain, repo, stream, cfg.Memory.CloseTimeout)
```

## Risks / Tradeoffs

| Risk | Mitigation |
|------|------------|
| Close timeout (30s) insufficient for slow providers | Config-tunable `close_timeout`; errors non-fatal (log + continue to quit) |
| LLM returns malformed JSON | Lenient parse: fence strip + `encoding/json`; on failure log + skip, never fail turn |
| Multi-word search behavior change (exact-phrase → AND) | Document in changelog; AND-join serves observation recall better |
| Nudge fires mid-turn | Nudge runs AFTER Step's provider call completes (not before); boundary check: `assistantCount % nudgeEvery == 0` |
| Stream cancel + close ordering | CtrlC cancels Step ctx → drain streamDoneMsg → CloseSession runs; second CtrlC force-quits |
| goleak on stream cancel | Provider goroutine must exit on ctx cancel; test with goleak.VerifyTestMain |
