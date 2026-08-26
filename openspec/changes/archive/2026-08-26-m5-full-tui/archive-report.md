# Archive Report: m5-full-tui

**Archived:** 2026-08-26
**Delivered as:** 3 stacked PRs merged to main — PR #18 (repository+manager) → #19 (TUI slash commands + wiring) → #20 (docs + polish).
**Baseline at start:** main `8d1f57f` (M4 archived). **Final:** main `d5e6580` (pre-archive).

## Shipped

- Session Manager (`internal/session`): active session id owned independent of surface, 7 operations (`NewSession`, `Save`, `List` ordered `updated_at DESC, id DESC`, `Restore`, `Rename` with injection scan, `Compress` early summarizer, `Snapshot` point-in-time copy with `messages_json`), share with TUI/gateway/cron.
- Repository extensions: `ListConversations`/`GetConversation`/`RenameConversation` (bumps `updated_at`, scanned title, empty rejected), `CreateSnapshot`/`ListSnapshots` via `snapshots` table (`internal/memory/migrations/0005_snapshots.sql`).
- Brain delegation: `SetActiveConversation(id)` and `ensureConversation` prefers manager id when set, falling back to `LatestConversation`.
- TUI: 7 slash branches in `runCommand` (`/new`/`/reset`, `/save`, `/list` inline, `/restore <id>` reloads history, `/compress` gated, `/snapshot`, `/rename <title>`), all gated `!streaming && !closing`, feedback via `commandFeedbackPrefix`, session list view, interrupt-and-redirect reuse.

## Verification

Independent bounded reviews (gentle-ai review-integration/v2):

- PR1 lineage `review-7df24a75bee32bf0`: **approved**, zero findings.
- PR2 lineage `review-0c860483df5cc7ee`: **approved**, zero findings.
- PR3 lineage `review-f57254b7f99fbf00`: **approved**, zero findings.

All pre-pr gates allow. Final verification per slice on clean tree: `go build ./...`, `go vet ./...`, `go test ./...` green under `goleak` (11 packages). `ListConversations` ordering shared constant ensures `LatestConversation` == `List` top.

## Spec sync

NEW capability: `openspec/specs/session-manager/spec.md`. MODIFIED capabilities appended: `repository-memory`, `brain-loop`, `minimal-tui` (and `config-loader` unchanged).

## Amendment note

No spec amendments beyond implementation-clarified `snapshots` table as point-in-time JSON store, consistent with delta spec.
