# Exploration: M5 — Full TUI

**Change name**: `m5-full-tui` · **Baseline**: HEAD main `8d1f57f` (M4 archived, schema v4).

## Scope: IN

Per `spec.md` §7 (session management), `spec.md` §6 (TUI v1 surface), `docs/sessions.md` and `docs/roadmap.md` M5:

- Session lifecycle independent of surface: Start / Run / Close / Restore, owned by a domain Session Manager (TUI, gateway, cron attach to same sessions).
- Slash commands: `/new` or `/reset`, `/save`, `/list`, `/restore <id>`, `/compress`, `/snapshot`, `/rename <title>` — all interactive in the Bubbletea TUI.
- Session browse + interrupt-and-redirect (already partially via streaming cancel/drain).
- Title and summary handling through `conversations` table (`title` default `New session`, `summary` populated by M2 summarizer).

## Current State

- **Storage substrate ships** (`docs/sessions.md:5`): `conversations` + `messages` tables, `Brain.Step` appends user/assistant messages transactionally, `LatestConversation` returns `ORDER BY updated_at DESC, id DESC`, `Init` restores full history on startup. Crash-safe: user message commits before provider call.
- **Single-writer model** (`internal/core/brain.go:521`): `ensureConversation` reuses `LatestConversation` or creates `New session`. Every `Step` continues the same conversation; there is no way to start clean. `MessageCount`, `title`, `summary` exist in the table but only `title="New session"` is ever written (`internal/memory/sqlite.go:17`).
- **Tail bounded at 50** (`brain.go:15`) sent to provider; full history always reloaded into viewport on startup.
- **Close path exists** (`brain.go:404`): `CloseSession` is non-fatal, runs `SessionCloser` (summarizer), skill creator, records `summary` event, and is already wired to `TUI handleQuit` via `closeSession()` with `defaultCloseTimeout`. Session-scoped permission grants are cleared there via `ClearSessionGrants` (already T4 TODO, now done). What is missing is **explicit** close triggers (`/new`, `/save`, timeout) and **restore by id**.
- **TUI today** (`internal/adapters/tui/app.go:352`): slash dispatch is `runCommand` handling `/personality`, `/persona`, `/permisos` only. Keys are `Enter` (submit), `CtrlC/Esc` (quit/cancel). No list view, no session picker, no rename/compress/snapshot. Spinner + viewport + input only.
- **Repository port today** (`internal/core/port_repository.go`): `CreateConversation`, `LatestConversation`, `AppendMessage`, `Messages`, `Search`, plus M2/M3 additions. **Missing** for M5: `ListConversations(limit, offset)`, `GetConversation(id)`, `RenameConversation`, `Search` already covers FTS but sessions browse needs listing.
- `docs/sessions.md:25` table lists all 7 commands as M5 with no implementation; `spec.md` §Milestones marks M5 as planned.

## Affected Areas

- `internal/core/types.go` — `Conversation` already has `Title`, `Summary`, `MessageCount`; no change, but ensure `Snapshot` representation is decided.
- `internal/core/port_repository.go` — add `ListConversations`, `GetConversation`, `RenameConversation` (and `Snapshot` storage if table needed).
- `internal/memory/sqlite.go` + `internal/memory/migrations/0005_sessions.sql` (maybe 0005) — implement listing, rename, snapshot table if chosen; `CreateConversation` already takes title.
- `internal/session/` (NEW) or `internal/core/session.go` — Session Manager domain component owning active session id, lifecycle Start/Save/Close/Restore/Compress/Snapshot/Rename/List.
- `internal/core/brain.go` — currently owns `ensureConversation` and `CloseSession`; needs to delegate active session tracking to Session Manager or expose `SetActiveConversation` so `/restore` and `/new` can switch.
- `internal/adapters/tui/app.go` — extend `runCommand` with 7 new branches, add list/snapshot views, interrupt-and-redirect (reuse existing streaming cancel/drain).
- `docs/sessions.md`, `docs/configuration.md` if needed, `README.md` roadmap.

## Approaches

1. **Thin Session Manager in `internal/session` (dedicated package)** — owns `activeID string`, wraps `Repository` for all lifecycle calls, exposes `New(ctx) (*Conversation, error)`, `Save(ctx) error` (explicit CloseSession trigger), `List(ctx, limit)`, `Restore(ctx, id)`, `Rename(ctx, id, title)`, `Compress(ctx)` (early summarizer), `Snapshot(ctx)` (copy conversation + messages with snapshot marker). Brain keeps `ensureConversation` but manager sets active id before `Step`.
   - Pros: Matches hexagon in `spec.md` §Architecture (manager independent of surface, TUI/gateway/cron share it); testable in isolation; keeps `brain.go` focused on turn logic.
   - Cons: One more package; small indirection.
   - Effort: Medium

2. **Fat Brain: add methods directly to `Brain`** — `Brain.NewSession()`, `Brain.RenameSession()`, etc., keeping active id inside `Brain`.
   - Pros: Fewest files, no new package.
   - Cons: Brain already 530 lines with 7 responsibilities (identity, skills, recall, tools, nudging, closing); violates single-responsibility and mixes turn vs lifecycle concerns; harder to share with future gateway.
   - Effort: Medium, but accumulates debt.

3. **Snapshot as file vs table** — Snapshot could be a filesystem copy of the DB or a new `snapshots` table with `conversation_id, snapshot_id, data JSON`.
   - Pros table: queryable, fits existing migration pattern, FTS still works.
   - Cons file: external to DB, not transactional.
   - Effort: Low/Medium — table is straightforward.

**Snapshot choice within Approach 1:** New `snapshots` table (`id`, `conversation_id`, `title`, `summary`, `created_at`, `messages_json`) via `0005_snapshots.sql`. `Snapshot` inserts one row; `List` and `Restore` remain `conversations`-centric, snapshots are auxiliary.

## Recommendation

**Approach 1 + snapshots table.** Create `internal/session` as the domain owner; TUI and future surfaces call it. Brain exposes `SetActiveConversation(id string)` (or manager owns the id and `ensureConversation` reads from it) so `/restore` and `/new` switch without touching turn logic. Add `snapshots` table. This satisfies "Session Manager will sit" (`docs/sessions.md:52`), keeps `brain.go` from growing further, and makes the 7 commands thin TUI wrappers (each ≤10 lines + a feedback line, matching `cmdPersonality`/`cmdPersona` pattern).

## Risks

- Prompt-injection via `/rename` title — title is displayed in list view and stored verbatim; apply `scan.Lines` before persisting/displaying, consistent with SOUL.md/skill handling.
- `/compress` early summarizer re-entrancy: `CloseSession` is not re-entrant while streaming; gate `/compress` when `streaming || closing` like `/personality` already does for personality changes.
- `/restore` while streaming — same gate; require `!streaming`.
- `ListConversations` ordering must match `LatestConversation` (`updated_at DESC, id DESC`) or "latest" diverges from list top.
- Snapshot table grows unbounded — not mitigated in M5, documented as future prune (`/snapshot` is point-in-time, no GC yet).

## Ready for Proposal

Yes — pending one product decision: confirm the `internal/session` package as the owner (vs fat Brain) and the `snapshots` table approach.
