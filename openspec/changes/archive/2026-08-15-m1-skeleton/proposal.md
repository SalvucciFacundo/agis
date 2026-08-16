# Proposal: M1 — Thinking agent with memory (Go skeleton)

## Intent

M1 is the first implementation milestone of AGIS. It delivers the smallest honest skeleton where the Brain loop actually talks to a real LLM (OpenAI or Ollama), persists sessions to SQLite+FTS5, restores the last session on restart, and runs in a minimal Bubbletea TUI. M1 first because every later milestone (learning loop, skills, tools, gateway) bolts onto the ports and layout this milestone establishes — getting the hexagonal seam right is the hardest thing to reverse.

## Scope

### In Scope
- Hexagonal Go skeleton (`cmd/agis`, `internal/core`, `internal/adapters/{llm,tui}`, `internal/memory`, `internal/config`)
- `core.Provider` port + OpenAI and Ollama adapters (one shared OpenAI-compatible client)
- `core.Repository` port + SQLite+FTS5 implementation with embedded migrations
- `Brain.Step` loop (one user message in → streamed response out; tool-call stub)
- Session persist/restore substrate (no slash commands — M5)
- Minimal Bubbletea TUI (viewport + textinput + streaming tokens)
- Tests: brain (fake provider), repository (t.TempDir), FTS5 (accent-insensitive), LLM adapters (httptest SSE), config

### Out of Scope
- Slash commands `/new /save /list /restore /compress` (M5)
- Curator, nudges, session summarizer, user model (M2)
- Skill hub, SOUL.md, persona overlays (M3)
- Tool port, Policy Guard, Docker/SSH backends (M4)
- Gateway, cron, MCP, plugins, webhooks (M6)
- Anthropic adapter (later milestone)
- Empty placeholder dirs (`internal/skills`, `internal/tools`, …) — created when their milestone lands

## Resolved Decisions

1. **Module path**: `github.com/SalvucciFacundo/agis` (confirmed).
2. **Stream signature**: amending spec §2 to `Stream(ctx, ChatRequest) (<-chan StreamEvent, error)` with `StreamEvent{Text string; Err error}`. A bare `<-chan Token` cannot surface mid-stream failures; the single-channel event pattern is the idiomatic Go fix and ripples cleanly into M2–M6 consumers.
3. **FTS5 architecture**: single standalone `memory_fts` table with `doc_type` discriminator (`message`|`observation`) + explicit sync in the same transaction as the base write. FTS5 external-content mode binds to ONE base table, so the spec's "over observations + messages" forces two tables+triggers OR this discriminator table. Explicit sync is testable and avoids hidden SQL (golang-database rule).
4. **FTS5 tokenizer**: `unicode61 remove_diacritics 1` — verified working with modernc.org/sqlite v1.56.0, accent-insensitive for Spanish+English.
5. **Embedded migrations**: `//go:embed migrations/*.sql` + `PRAGMA user_version` versioning. Deviates from golang-database's external-migration rule; justified by the single-binary / zero-services vision (embedded SQLite is not a shared production DB).
6. **Config layout**: `~/.agis/config.yaml` (0600) + `AGIS_HOME` env override + `-config` flag. M1 structure:
   ```yaml
   llm:
     provider: openai|ollama
     model: gpt-4o-mini
     api_key: ""   # or AGIS_OPENAI_API_KEY env
   db:
     path: ~/.agis/agis.db
   ```
   `internal/config` is added to spec layout (spec omitted it; M1 needs it).
7. **`Models()` in M1**: static list returned from config. Live provider fetch (Ollama `/api/tags`, OpenAI list-models) deferred to M4 with `agis model` subcommand.
8. **M1 session subset**: persistence substrate only — `CreateConversation`, `LatestConversation`, `AppendMessage`, `Messages(convID, limit)`, `Search`, `Close`. No slash commands, no summarizer, no session-scoped permission grants. The session IS a conversation in M1.
9. **Review-budget slicing**: full M1 exceeds 800 lines → chained PRs (boundaries below).

## Approach

GAIA-aligned minimal skeleton. Both ports real and tested. One shared OpenAI-compatible client (`sashabaranov/go-openai`) serving both adapters via BaseURL config. Manual constructor injection (no DI framework). `flag` stdlib for CLI (Cobra lands in M4).

## Directory Skeleton

| Directory | M1 Status | Rationale |
|---|---|---|
| `cmd/agis/` | REAL | entry point, wiring |
| `internal/config/` | REAL | spec-omitted but required in M1 |
| `internal/core/` | REAL | domain + ports + brain loop |
| `internal/adapters/llm/` | REAL | openai + ollama adapters |
| `internal/adapters/tui/` | REAL | minimal bubbletea app |
| `internal/memory/` | REAL | SQLite repo + migrations + FTS |
| `internal/{skills,tools,policy,persona,gateway,mcp,cron,plugins,webhook}/` | ABSENT | created at their milestone |
| `pkg/` | ABSENT | no shared packages needed yet |

## Chained PR Slicing (review budget 800 lines)

| PR | Scope | Approx. Δ-lines |
|---|---|---|
| 1 — skeleton | `go.mod`, `cmd/agis/main.go`, `internal/config`, `internal/core` (types, ports, brain with fakes), `Makefile`, `.gitignore`, `.golangci.yml` | ~650 |
| 2 — memory | `internal/memory` (sqlite repo, FTS5, embedded migrations, tests) | ~700 |
| 3 — LLM adapters | `internal/adapters/llm` (client, openai, ollama, httptest SSE tests) | ~450 |
| 4 — TUI + wiring | `internal/adapters/tui`, `cmd/agis/main.go` end-to-end wiring | ~500 |

PR 1 is the hard seam; PRs 2-4 attach to it. Each PR is reviewable in isolation.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `cmd/agis/main.go` | New | entry: flags, config, wiring, tea.Program |
| `internal/config/config.go` | New | YAML loader, defaults, `AGIS_HOME` |
| `internal/core/{types,port_llm,port_repository,brain,session}.go` | New | domain + ports + brain loop |
| `internal/adapters/llm/{client,openai,ollama}.go` | New | shared client + 2 adapters |
| `internal/adapters/tui/app.go` | New | minimal Bubbletea app |
| `internal/memory/{sqlite,fts}.go` + `migrations/0001_init.sql` | New | Repository impl + FTS5 + schema |

## Dependencies (pinned)

| Module | Version | Why |
|---|---|---|
| `modernc.org/sqlite` | v1.56.0 | pure-Go SQLite + FTS5 (smoke-tested) |
| `github.com/charmbracelet/bubbletea` | v1.3.10 | TUI runtime |
| `github.com/charmbracelet/bubbles` | v1.0.0 | textinput, viewport, spinner |
| `github.com/charmbracelet/lipgloss` | v1.1.0 | styling |
| `github.com/sashabaranov/go-openai` | v1.42.0 | shared OpenAI-compatible client |
| `gopkg.in/yaml.v3` | v3.0.1 | config file |
| `github.com/google/uuid` | v1.6.0 | IDs |
| `github.com/stretchr/testify` | v1.11.1 | test asserts (test-only) |
| `go.uber.org/goleak` | v1.3.0 | stream goroutine leak tests (test-only) |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Stream signature amendment ripples into M2–M6 | Med | Document in spec; all consumers use StreamEvent from day one |
| FTS5 single-table design revisited when observations land in M2 | Low | doc_type discriminator already reserves the observation row shape |
| go-openai version drift from upstream OpenAI API | Low | pin exact version; Ollama's compat layer is stable |
| Modernc.org/sqlite FTS5 breaks on upgrade | Low | pin v1.56.0; smoke test in CI |

## Rollback Plan

M1 is greenfield — rollback is `git reset --hard` to pre-M1 state (empty repo). Each PR is independently mergeable and revertable; no PR depends on runtime state from a previous one. If PR 2 (memory) has issues, PR 1 (skeleton) remains deployable with stub Repository.

## Success Criteria

- [ ] `go build ./cmd/agis/...` produces a single static binary
- [ ] `go test ./...` green across brain, memory, FTS5, LLM adapters, config
- [ ] `agis` starts, accepts input in TUI, streams a response from OpenAI OR Ollama, and restores the last session on restart
- [ ] FTS5 MATCH works accent-insensitive over persisted messages
- [ ] No cgo dependency; binary runs on a fresh Linux/amd64 with no runtime services

## Dependencies

- Go 1.26.x toolchain (detected)
- GitHub repo `github.com/SalvucciFacundo/agis` (must exist before PR 1 push)
- OpenAI API key OR local Ollama running for end-to-end verification
