# Sessions

A session is a bounded conversation: it has an identity, a lifespan, and at close it produces a summary and candidate observations. The Session Manager is designed in `spec.md` §7; M1 ships the persistence substrate it will run on.

## What exists today (M1)

M1 implements the storage layer, not the session manager. What you can do now:

- **Persist conversations and messages** — `conversations` and `messages` tables, written transactionally by `Brain.Step` and `Repository.AppendMessage`.
- **Latest conversation** — `LatestConversation` returns the most recently updated conversation (`ORDER BY updated_at DESC, id DESC`), creating one titled `New session` when none exists.
- **Restore on TUI startup** — the TUI's `Init` loads the latest conversation and renders its full message history (`internal/adapters/tui/app.go:241`). Launch AGIS and your last session is there.
- **Single-writer model** — there is exactly one active conversation; `Brain.Step` reuses the latest one (`ensureConversation`, `internal/core/brain.go:93`).

Messages accumulate in one conversation across runs. The tail sent to the provider is bounded at 50 messages (`tailLimit`, `internal/core/brain.go:13`).

## Session lifecycle (designed, not implemented)

The full manager arrives with M5. Designed behavior:

1. **Start** — a session is created; cross-session recall loads relevant observations (FTS5) and a context digest.
2. **Run** — messages and tool activity persist incrementally, so a crash loses nothing.
3. **Close** — on `/new`, `/save`, app exit, or timeout: the curator evaluates pending observations, the summarizer compresses the session into `conversations.summary`, and session-scoped permission grants are discarded.
4. **Restore** — `/restore <id>` reloads the summary plus the tail of messages; the full history stays queryable via FTS5.

## Slash commands (M5)

| Command | Purpose | Milestone |
|---|---|---|
| `/new` or `/reset` | start a fresh session, closing the current one | M5 |
| `/save` | persist the current session explicitly | M5 |
| `/list` | browse recent sessions (id, title, created_at) | M5 |
| `/restore <id>` | load a session from summary + last messages | M5 |
| `/compress` | run the session summarizer early, freeing context | M5 |
| `/snapshot` | capture a point-in-time snapshot of session state | M5 |
| `/rename <title>` | title the session for later discovery | M5 |

None of these commands exist in the M1 TUI. The TUI's only key handling is Enter (submit), Ctrl+C / Esc (quit), and window-resize.

## Today's single-conversation behavior

M1 has no `/new`: there is exactly one active conversation, and every `Brain.Step` continues it. The observable consequences:

- Restarting AGIS resumes the same conversation — there is no way to start clean yet.
- Every conversation is titled `New session` (`defaultTitle` in `internal/memory/sqlite.go:17`); `summary` is always `""` because the summarizer is M2.
- The full history is always reloaded into the TUI on startup; context sent to the provider is bounded at the 50-message tail.
- Conversation ordering is `updated_at DESC, id DESC` — the newest write wins; UUID ordering is the tie-break for identical timestamps.

## Crash-safety today

Messages persist incrementally inside `Brain.Step`'s transaction: the user message is committed before the provider is called, so a crash mid-stream never loses your input. The assistant reply commits only when the stream completes. On restart the TUI restores whatever was committed.

## Where the Session Manager will sit

The M5 manager is a domain component (per the hexagon in `spec.md` §Architecture) that will own session lifecycle **independent of the surface** — the TUI, the future gateway, and cron all attach to sessions. It will build directly on the M1 `Repository` methods (`CreateConversation`, `LatestConversation`, `AppendMessage`, `Messages`, `Search`), which is why M1 scoped the port exactly to those five calls.

## Session-scoped permission grants (M4)

The `session` approval scope from the permission system will live here: grants held in memory for the active session, expiring at close, never persisted. This depends on M4's Policy Guard — see [docs/permissions.md](docs/permissions.md). It is not implemented.

## Summary

M1 delivers the durable half of sessions — conversations and messages persist, the latest is restored on startup, writes are crash-safe — and leaves the lifecycle half (commands, summarize, restore-by-id, session-scoped grants) to M4/M5.
