# Tasks: M2 — Learning Loop

## Review Workload Forecast

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

| Unit (PR) | Focused test | Runtime harness | Rollback boundary |
|-----------|--------------|-----------------|-------------------|
| 1 Substrate (PR1) | `go test ./internal/memory/... ./internal/core/...` | N/A — t.TempDir() DBs | Revert port+0002 (unused → M1 intact) |
| 2 Loop (PR2) | `go test ./internal/memory/... ./internal/core/...` | N/A — fake providers, no LLM | Revert brain.go+memory (TUI quits → M1 intact) |
| 3 TUI+cfg+wiring (PR3) | `go test ./...` + `go build ./cmd/agis/...` | `AGIS_HOME=$(mktemp -d) go run ./cmd/agis` — CtrlC closes; ×2 force | Revert app.go/config.go/main.go |

## Phase 1: Memory Substrate (PR1) — REPO-001..003

- [x] T1.1 Mig 0002 — `0002_learning.sql`: updated_at+backfill, UNIQUE topic_key, user_model, session_events CHECK. AC: v1→v2, v2 no-op.
- [x] T1.2 Types — `core/types.go`: `Observation`(+UpdatedAt), `UserModel`.
- [x] T1.3 Port — `core/port_repository.go`: SaveObservations, Observations, UpdateConversationSummary, UpsertUserModel, RecordSessionEvent (5th: CUR-002/BRN-002 need event rows). Deps: T1.2
- [x] T1.4 FTS — `memory/fts.go`: deleteFTSRow; AND-join ftsQuery. AC: `"coffee" AND "preference"` (fts_test.go). Deps: T1.3
- [x] T1.5 Repo — `memory/sqlite.go`: 5 port methods; upsert keeps created_at, bumps updated_at, clamp[1,5] dflt 3, FTS delete+insert same-tx, atomic batch. AC: REPO-001 scenarios (sqlite/migrations_test.go). Deps: T1.1,T1.3,T1.4
- [x] T1.6 Docs — `docs/memory.md`: AND-search note. Deps: T1.5

## Phase 2: Learning Loop (PR2) — CUR-001..003, SUM-001..002, USR-001, BRN-001..002

- [x] T2.1 Ports — `core/brain.go`: `Nudger`, `SessionCloser` (import-cycle fix). Deps: T1.2
- [x] T2.2 Curator — `memory/curator.go`: 1 Chat → fence-strip JSON, importance dflt 3, fail→log+skip. AC: CUR-001. Deps: T2.1
- [x] T2.3 Summarizer — `memory/summarizer.go`: 1 Chat → {summary, obs[]} → Update+Save; non-fatal. AC: SUM-001. Deps: T2.1
- [x] T2.4 Usermodel — `memory/usermodel.go`: pure AggregateUserModel — `user/` only, key=full topic_key, clamp(imp/5) first, 0.7/0.3 update. AC: USR-001. Deps: T1.2
- [x] T2.5 Recall — Step: Observations(recallLimit=10) → system prompt. AC: BRN-001 — fake provider sees obs. Deps: T2.1
- [x] T2.6 Nudge — Step: count%nudgeEvery==0 → Nudge+event('nudge'); nil curator → skip. AC: CUR-002/003. Deps: T2.2,T2.5
- [x] T2.7 CloseSession — ensure→msgs 200→Close→Aggregate→Upsert→event('summary'); deadline, non-fatal, nil→no-op. AC: BRN-002 order; drain+goleak. Deps: T2.3,T2.4,T2.6

## Phase 3: TUI + Config + Wiring (PR3) — TUI-001

- [x] T3.1 Config — `config/config.go`: MemoryConfig defaults true/10/10/30s.
- [x] T3.2 Wiring — `cmd/agis/main.go`: nil curator/summarizer when !LearningEnabled; Brain opts; tui.New(+timeout). Deps: T2.7,T3.1
- [x] T3.3 TUI close — `tui/app.go`: cancel+closing; idle → close→quit; streaming → cancel→drain→close→quit; 2nd CtrlC force. AC: TUI-001 tests + goleak. Deps: T2.7
- [x] T3.4 Docs — `docs/configuration.md` memory block. Deps: T3.1
- [x] T3.5 Full suite — `go test ./...`, `go build ./cmd/agis/...`; 47 M1 + new green. Deps: all

## Work-Unit Commit Plan (code+tests+docs; each PR green)

- PR1: types → port → mig → upsert+FTS → AND search → events → docs
- PR2: ports → curator → summarizer → usermodel → recall → nudge → close
- PR3: config → TUI close → wiring → docs

## Dependency Ordering & Design Gaps

PR1: T1.2∥1.1→1.3→1.4→1.5→1.6; PR2: 2.1→{2.2–2.4}→2.5→2.6→2.7; PR3: 3.1→3.2→3.3,3.4,3.5. Sequential (PR2←PR1, PR3←PR2).

Gaps: (1) RecordSessionEvent = 5th port method (REPO-001's four aren't exhaustive; CUR-002/BRN-002 need event rows). (2) Brain uses core interfaces, not `*memory.Curator` (import cycle).
