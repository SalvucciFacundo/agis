# Proposal: m5-full-tui

## Intent

M5 closes the session half of AGIS. M1 shipped the durable half (conversations persist, latest restores on startup, crash-safe writes) and M2 wired the learning close path, but the user still cannot start a clean session, list past work, restore by id, or manage sessions. This change delivers the 7 slash commands and the domain Session Manager so the lifecycle is owned once and shared by TUI, future gateway and cron — exactly as `spec.md` §7 and `docs/sessions.md` design it.

## Scope

### In Scope
- Domain Session Manager in `internal/session` owning active session id and lifecycle: Start/New, Save, List, Restore, Rename, Compress (early summarization), Snapshot (point-in-time copy), Close. Independent of surface.
- Repository extensions: `ListConversations(limit, offset)`, `GetConversation(id)`, `RenameConversation(id, title)`, `snapshots` table via `0005_snapshots.sql`.
- TUI slash commands: `/new`/`/reset`, `/save`, `/list`, `/restore <id>`, `/compress`, `/snapshot`, `/rename <title>` — all interactive, with feedback lines and re-entrancy gates (`!streaming && !closing`).
- Session browse view (reuse `/permisos` panel pattern as sub-model or inline list), interrupt-and-redirect reuse of existing streaming cancel/drain.
- Title handling with `scan.Lines` before persist/display; `defaultTitle` and ordering `updated_at DESC, id DESC` preserved.
- Docs: `docs/sessions.md` header flip to Implemented, `README.md` + `docs/roadmap.md` M5 DONE, spec sync.

### Out of Scope
- Gateway surfaces (Telegram/Discord/etc. attaching to sessions) — M6.
- Cron scheduled automations — M6.
- FTS search over snapshots (snapshots are point-in-time, not indexed).
- Snapshot GC/pruning (documented as future).
- Rich multiline editing beyond existing `textinput` (kept as-is).

## Capabilities

### New Capabilities
- `session-manager`: lifecycle, active id, Browse/List/Restore/Rename/Compress/Snapshot/Save, crash-safe incremental persistence contract.

### Modified Capabilities
- `repository-memory`: add listing, get-by-id, rename, snapshots table.
- `brain-loop`: delegate active conversation tracking to Session Manager via `SetActiveConversation`; keep `ensureConversation` as fallback.
- `minimal-tui`: 7 new slash branches, session list view, re-entrancy gates, feedback lines.

## Approach

Reuse proven M3/M4 patterns: consumer-side port in `core` (`SessionStore` with the 4 new methods, `SnapshotStore`) keeps `internal/session` hexagonal — `core` defines the contract, `memory` implements it, `session` consumes it. TUI commands are thin wrappers (≤10 lines each, feedback via `commandFeedbackPrefix`) matching `cmdPersonality`/`cmdPersona` style, with no provider reach and no persistence as messages. Snapshot is a table (`snapshots` with `messages_json` TEXT) rather than a file, so it stays transactional and queryable. Title injection scanned via `internal/scan`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/session/manager.go` | New | Domain manager owning active id + 7 operations |
| `internal/core/port_repository.go` | Modified | Add `ListConversations`, `GetConversation`, `RenameConversation`, snapshot methods |
| `internal/memory/sqlite.go` | Modified | Implement new port methods, `ORDER BY updated_at DESC, id DESC` consistent |
| `internal/memory/migrations/0005_snapshots.sql` | New | `snapshots` table + index |
| `internal/adapters/tui/app.go` | Modified | 7 new `runCommand` branches, list view, gates |
| `internal/adapters/tui/panel.go` | Modified | Reuse sub-model pattern if session list needs overlay |
| `cmd/agis/main.go` | Modified | Wire Session Manager, pass to TUI via `WithSession` |
| `docs/sessions.md`, `README.md`, `docs/roadmap.md` | Modified | Flip headers, mark M5 DONE |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Title injection via `/rename` | Medium | `scan.Lines` before save/display (same as SOUL.md) |
| Re-entrancy: `/compress` or `/restore` while streaming | Medium | Gate `if m.streaming \|\| m.closing { return }` — reuse existing guard |
| List ordering diverges from `LatestConversation` | Low | Single SQL fragment `ORDER BY updated_at DESC, id DESC` shared as constant |
| Snapshot table unbounded growth | Low | Document future prune; no GC in M5 — not blocking |

## Rollback Plan

Stacked PR. Each PR is additive and revertible via `git revert -m 1`. Migration `0005` is additive-only (new table); `user_version` gates idempotency. No destructive schema changes. Feature flag not needed: slash commands are inert until typed; manager not wired until PR2/3.

## Dependencies

- M1 `Repository` 5-method substrate (done), M2 summarizer (done), `internal/scan` (done).

## Success Criteria

- [ ] `/new` starts a fresh conversation and next turn uses it; `/reset` aliases `/new`
- [ ] `/list` shows recent sessions (id, title, created_at) from `ListConversations`
- [ ] `/restore <id>` loads summary + tail and continues that session
- [ ] `/rename <title>` persists and appears in `/list`
- [ ] `/compress` runs summarizer early and frees context (re-uses `CloseSession` path)
- [ ] `/snapshot` inserts a row in `snapshots` with `messages_json` and does not affect active session
- [ ] `/save` triggers an explicit `CloseSession`-style persist without quitting
- [ ] Titles with injection patterns are scrubbed before display
- [ ] Full suite green: `go build ./...`, `go vet ./...`, `go test ./...` under `goleak`

## Proposal question round

Approach 1 (dedicated `internal/session` + `snapshots` table) confirmed by owner in exploration. No open product gaps.
