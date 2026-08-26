# Tasks: m5-full-tui

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1200-1500 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 repository+manager → PR2 TUI slash commands + wiring → PR3 docs + polish |
| Delivery strategy | auto-chain (owner precedent M1-M4: stacked-to-main) |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

| Unit (PR) | Focused test | Runtime harness | Rollback boundary |
|-----------|--------------|-----------------|-------------------|
| PR1 | `go test ./internal/session/ ./internal/memory/` | N/A — library slice; run `go test` only | migration 0005 additive; port additive; new package |
| PR2 | `go test ./internal/adapters/tui/` | manual: /new, /list, /restore, /rename, /compress, /snapshot, /save in TUI | TUI slash branches additive; manager not wired until PR2 |
| PR3 | `go test ./...` | `go build ./...` + manual docs check | docs only |

## Phase 1: Repository + Manager substrate (PR1)

- [x] T1.1 Create `internal/memory/migrations/0005_snapshots.sql` (`snapshots` table: id PK, conversation_id FK, title, summary, messages_json, created_at; index on conversation_id) + bump `user_version` gate. Tests: v4→v5 applies, re-run no-op.
- [x] T1.2 Extend `internal/core/port_repository.go`: `ListConversations(limit, offset)`, `GetConversation(id)`, `RenameConversation(id, title)`, `CreateSnapshot`, `ListSnapshots` — all ordered `updated_at DESC, id DESC` where applicable. Scan title via `scan.Lines` before write.
- [x] T1.3 Implement `internal/memory/sqlite.go` for new methods: shared ordering constant, `Rename` bumps `updated_at` via `UPDATE ... SET title=?, updated_at=?`, snapshot inserts JSON array of `Messages` for the conversation.
- [x] T1.4 Create `internal/session/manager.go`: struct `Manager{repo, activeID, logger}`, methods `NewSession`, `List`, `Get`, `Restore`, `Rename`, `Compress` (early summarizer reusing `SessionCloser`), `Snapshot`, `Save`, `ActiveID`/`SetActive`. `NewSession` also clears `activeID` switch. Tests with fake repo.

## Phase 2: TUI slash commands + wiring (PR2)

- [x] T2.1 Add `Brain.SetActiveConversation(id)` and branch in `ensureConversation` to prefer manager id. Test: restore switches active id.
- [x] T2.2 Wire Session Manager in `cmd/agis/main.go` via `WithSessionManager` / `WithSession` option, pass to TUI.
- [x] T2.3 Implement `internal/adapters/tui/app.go` 7 slash branches in `runCommand`: `/new`/`/reset` → `manager.NewSession` + `brain.SetActive` + feedback; `/save` → `manager.Save`; `/list` → render `List` ids/titles; `/restore <id>` → `Restore` + `loadHistory`; `/compress` → `Compress` gated `!streaming && !closing`; `/snapshot` → `CreateSnapshot` + feedback; `/rename <title>` → `Rename` with scan. Each gated while streaming/closing.
- [x] T2.4 Add session list sub-view or inline rendering for `/list` (reuse panel pattern if needed). Tests via `drive` helpers: after `/new` next turn uses new id; `/list` shows titles; gated while streaming ignores.
- [x] T2.5 Tests: TUI slash commands through `newTestModel` with fake manager/repo; interrupt-and-redirect reuse existing cancel/drain.

## Phase 3: Docs + verification (PR3)

- [x] T3.1 Update `docs/sessions.md` header to Implemented, document the 7 commands with the lifecycle diagram.
- [x] T3.2 Update `docs/configuration.md` if session-related config added (none expected), `README.md` roadmap M5 DONE, `docs/roadmap.md` M5 section to ✅ DONE with shipped bullets.
- [x] T3.3 Full suite green: `go build ./...`, `go vet ./...`, `go test ./...` under `goleak`, `golangci-lint` if configured.

## Dependency Ordering

T1.x -> T2.x -> T3.x. Manager must exist before TUI can call it; repository methods must exist before manager.

## Threat Matrix

N/A — panel already handled scan for titles; no new executable boundary.
