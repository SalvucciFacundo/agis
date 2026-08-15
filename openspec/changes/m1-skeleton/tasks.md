# Tasks: M1 — Thinking agent with memory (Go skeleton)

## Review Workload Forecast

Full M1 ≈ 2300 lines → 4 slices, each < 800-line budget.

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

## Work Units (test cmd · harness · rollback)

- PR1 skeleton+config+core+brain: `go test ./internal/config/... ./internal/core/...` · harness N/A (binary from PR4): `go build` · rollback: revert PR1 files
- PR2 memory+FTS+migrations: `go test ./internal/memory/...` · harness: TestMigrations (t.TempDir) · rollback: revert internal/memory/
- PR3 LLM adapters: `go test ./internal/adapters/llm/...` · harness: httptest SSE · rollback: revert adapters/llm/
- PR4 TUI+wiring: `go test ./...` · harness: go run ./cmd/agis (fake provider) · rollback: revert tui/ + main.go

## Tasks (→ acceptance; D deps)

### Phase 1 — Foundation

- [x] 1.1 Init go.mod `github.com/SalvucciFacundo/agis` (go 1.26) + deps + go.sum → go build ./...
- [x] 1.2 Add Makefile, .gitignore, .golangci.yml → make lint runs. D: 1.1
- [x] 1.3 internal/config/config.go — Load(): defaults (ollama/llama3.2/~/.agis/agis.db), precedence flag>AGIS_HOME>default, 0600-check → CONF-001. D: 1.1
- [x] 1.4 internal/config/config_test.go — missing file, precedence, perm warn → CONF-001. D: 1.3
- [x] 1.5 internal/core/types.go — Role consts, Message, Conversation, ChatRequest/Response, ModelInfo, SearchResult → compiles. D: 1.1
- [x] 1.6 internal/core/port_repository.go + port_llm.go — Repository (6 m), Provider (Chat/Stream/Models), StreamEvent{Text,Err} → var _ checks. D: 1.5
- [x] 1.7 internal/core/brain.go — Step: persist user, tail, Stream→sink, persist assistant; tools ignored → BRAIN-001. D: 1.6
- [x] 1.8 internal/core/brain_test.go — fake provider: persist+sink "Hi"; error → no assistant → BRAIN-001+goleak. D: 1.7

### Phase 2 — Memory

- [x] 2.1 migrations/0001_init.sql — 3 tables + memory_fts (unicode61 remove_diacritics 1), WAL, FK ON, role CHECK → REPO-002. D: 1.6
- [x] 2.2 internal/memory/migrations.go — //go:embed, PRAGMA user_version, transactional apply → REPO-004. D: 2.1
- [x] 2.3 internal/memory/sqlite.go — NewRepository, Create/LatestConversation, AppendMessage (tx+FTS+count), Messages, Search, Close → REPO-001. D: 2.2
- [x] 2.4 internal/memory/fts.go — FTS sync; Search matches message+observation, accent-insensitive → REPO-003. D: 2.3
- [x] 2.5 internal/memory/*_test.go — schema, CRUD order, cascade, count, FTS accent+doc_type → go test green. D: 2.3, 2.4

### Phase 3 — LLM adapters

- [x] 3.1 internal/adapters/llm/client.go — shared OpenAI-compatible client (BaseURL) → reused by both. D: 1.6
- [x] 3.2 openai.go/ollama.go — Provider impls; NewProvider selects by llm.provider → LLM-001. D: 3.1
- [x] 3.3 Models() static from cfg → LLM-002. D: 3.2
- [x] 3.4 *_test.go — httptest SSE: token order, mid-stream Err, Models → LLM-001+goleak. D: 3.2

### Phase 4 — TUI + wiring

- [ ] 4.1 internal/adapters/tui/app.go — viewport+textinput+spinner, Enter→Step, stream to viewport, restore latest → TUI-001. D: 1.7, 2.3
- [ ] 4.2 cmd/agis/main.go — -config, Load, repo, provider, brain, tui, tea.NewProgram → go build static. D: 4.1
- [ ] 4.3 app_test.go — tea smoke w/ fake provider, goleak.VerifyTestMain → TUI-001 + go test ./... D: 4.2

## Dependency graph

1.1→1.2,1.3→1.4 · 1.5→1.6→1.7→1.8 · 1.6→2.1→2.2→2.3→2.4,2.5 · 1.6→3.1→3.2→3.3,3.4 · 1.7,2.3→4.1→4.2→4.3.

## Commit plan (work units)

Tests+docs+code per commit: PR1=1.1–1.8 feat(skeleton) · PR2=2.1–2.5 feat(memory) · PR3=3.1–3.4 feat(llm) · PR4=4.1–4.3 feat(tui); merge main in order, per-PR revert.
