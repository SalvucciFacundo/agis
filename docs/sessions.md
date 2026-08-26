# Sessions

A session is a bounded conversation: it has an identity, a lifespan, and at close it produces a summary and candidate observations. The Session Manager is implemented in `internal/session` and owns the lifecycle independent of surface (TUI today, gateway and cron in M6).

## What exists today (M5)

M5 implements the full session manager. What you can do now:

- **Persist conversations and messages** — `conversations` and `messages` tables, written transactionally by `Brain.Step` and `Repository.AppendMessage`.
- **Latest conversation** — `LatestConversation` returns the most recently updated conversation (`ORDER BY updated_at DESC, id DESC`), creating one titled `New session` when none exists.
- **Restore on TUI startup** — the TUI's `Init` loads the latest conversation and renders its full message history (`internal/adapters/tui/app.go:241`). Launch AGIS and your last session is there.
- **Session lifecycle via slash commands** — `/new`, `/save`, `/list`, `/restore`, `/compress`, `/snapshot`, `/rename` (see table below). Active session id is tracked by `internal/session.Manager` and `Brain.SetActiveConversation`, so every `Step` continues the chosen session.
- **Title and snapshot persistence** — `ListConversations`, `GetConversation`, `RenameConversation` (with injection scan), `CreateSnapshot`/`ListSnapshots` via `snapshots` table (`internal/memory/migrations/0005_snapshots.sql`).

Messages accumulate in the active conversation across runs. The tail sent to the provider is bounded at 50 messages (`tailLimit`, `internal/core/brain.go:13`).

## Session lifecycle (implemented in M5)

1. **Start** — `Manager.NewSession` creates a conversation; `Brain.SetActiveConversation` makes it the target for subsequent `Step` calls. Cross-session recall loads relevant observations (FTS5) and a context digest.
2. **Run** — messages and tool activity persist incrementally, so a crash loses nothing.
3. **Close** — on `/new`, `/save`, app exit, or timeout: the curator evaluates pending observations, the summarizer compresses the session into `conversations.summary`, and session-scoped permission grants are discarded via `ClearSessionGrants`.
4. **Restore** — `/restore <id>` validates via `GetConversation`, switches `Manager` and `Brain` active ids, then reloads `Messages` into the viewport.

## Slash commands (M5 — implemented)

| Command | Purpose |
|---|---|
| `/new` or `/reset` | start a fresh session, closing the current one |
| `/save` | persist the current session explicitly (feedback `· saved`) |
| `/list` | browse recent sessions (id, title, created_at) via `ListConversations` |
| `/restore <id>` | load a session from summary + tail and continue it |
| `/compress` | run the session summarizer early, freeing context (gated `!streaming && !closing`) |
| `/snapshot` | capture a point-in-time snapshot (`snapshots` row with `messages_json`) |
| `/rename <title>` | title the session for later discovery (scanned via `internal/scan`) |

All session slash commands are gated while `streaming || closing` and never reach the provider nor persist as messages.

## Today's behavior (post-M5)

- Restarting AGIS resumes the latest conversation unless you used `/new` or `/restore` to switch. There is now a way to start clean.
- Every conversation is titled `New session` by default (`defaultTitle` in `internal/memory/sqlite.go:17`); `/rename` changes it and bumps `updated_at` so the renamed session becomes latest.
- `summary` is populated by the M2 summarizer on close or by `/compress` early; `ListConversations` ordering is `updated_at DESC, id DESC` — shared constant ensures `LatestConversation` == `List` top.
- Snapshots are point-in-time copies in `snapshots`; full history for a conversation stays queryable via `Messages` and FTS5.

## Crash-safety

Messages persist incrementally inside `Brain.Step`'s transaction: the user message is committed before the provider is called, so a crash mid-stream never loses your input. The assistant reply commits only when the stream completes. Snapshots are independent inserts and do not affect the active session.

## Where the Session Manager sits

The M5 manager is a domain component (`internal/session/manager.go`) that owns the active session id and the 7 operations, wrapping the `Repository` port. It builds directly on the M1 `Repository` methods plus the 5 new ones added in this milestone. The TUI (`internal/adapters/tui/app.go`) is a thin surface: each slash branch is ≤10 lines plus a feedback line, matching `cmdPersonality`/`cmdPersona` style, and `cmd/agis/main.go` wires the manager via `WithSessionManager` and seeds it from `LatestConversation` on startup.

## Session-scoped permission grants (M4)

The `session` approval scope from the permission system lives here: grants held in memory for the active session, expiring at close via `ClearSessionGrants`, never persisted. See [docs/permissions.md](docs/permissions.md).

## Summary

M5 delivers both halves of sessions — the durable half (conversations, messages, snapshots) and the lifecycle half (7 slash commands, active-id switching, early summarization, title management). The manager is surface-agnostic and ready for gateway and cron in M6.
