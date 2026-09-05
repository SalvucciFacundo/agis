# Proposal: session-cli

## Intent
Enable headless management and automation of the `agis` conversation lifecycle. Users and scripts currently cannot list, review, export, or delete sessions without launching the interactive Bubbletea TUI, making backups, pruning, and data portability operations cumbersome.

## Target Users and Situations
- **Users**: Developers, power users, and data scientists looking to extract, analyze, or clean up chat histories from the command line.
- **Automations**: Cron jobs pruning old conversations, backup scripts creating snapshots, or CI/CD pipelines exporting session outputs.

## Scope
- Add `agis session [list|show|delete|rename|export|snapshot]` subcommand utilizing the standard library `flag` package to maintain structural consistency with `agis doctor` and `agis policy`.
- **list**: Output recent sessions to `stdout`.
- **show**: Display conversation details and messages.
- **delete**: Permanently remove a session. Ensure it leverages `core.Repository.DeleteConversation(ctx, id)` to cascade deletion to messages, snapshots, and attachments.
- **rename**: Rename a session with prompt injection scanning (already in `Manager.Rename`).
- **export**: Extract session messages formatted as `json`, `markdown`, or `plaintext`.
- **snapshot**: Trigger a point-in-time snapshot of the specified session ID.
- Extend `internal/session.Manager` to support CLI-facing operations (specifically by adding `Show`, `Delete`, `Export`, and an ID-specific `SnapshotID` or similar).
- Implement standard POSIX exit codes and `-config` flag support in `cmd/agis/session.go` and ensure full testing coverage in `cmd/agis/session_test.go`.

## Non-Goals
- Introducing a new CLI framework (e.g. Cobra/Viper) for just this command; we stick to stdlib `flag` matching current patterns.
- Bulk processing commands (e.g. `--delete-all-before-date`). Users can script this by piping `list` to `delete`.
- TUI integration for the new export formats. This is CLI-only for now.

## Business Rules & Constraints
- **Cascade Deletion**: Deleting a session must wipe all associated records from the system without leaving orphans.
- **Security**: Renames must pass prompt injection scanning to prevent control character injection or prompt interference.
- **CLI Patterns**: Output data to `stdout`, errors/logs to `stderr`. Exit codes: 0 for success, 1 for general errors (e.g., ID not found), 2 for flag/usage errors.
- **Strict TDD**: Tests must be implemented according to the project's Strict TDD mode conventions.

## Current-State Gap
- `internal/session.Manager` currently assumes an active TUI context (`activeID`) for certain operations like `Snapshot`. It lacks ID-specific overrides necessary for offline CLI usage.
- `DeleteConversation(ctx, id)` exists in the repository but is not exposed to users via the Manager or CLI.

## Affected Areas
- `cmd/agis/session.go` (new)
- `cmd/agis/session_test.go` (new)
- `cmd/agis/main.go` (CLI command routing)
- `internal/session/manager.go` (new operations: `Show`, `Delete`, `Export`, `SnapshotID`)
- `internal/session/manager_test.go`

## Implications and Impact
- Reductions in SQLite DB bloat, as automated prune scripts become feasible.
- TUI users may experience a state where their currently open active conversation is deleted underneath them by an external CLI command.

## Edge Cases
- **Missing or Invalid ID**: Show, delete, rename, export, and snapshot on non-existent IDs must return a clean exit code 1 with an error message on stderr, instead of panicking.
- **Snapshot ID vs Active ID**: The current `Manager.Snapshot` assumes `m.activeID`. We need to accommodate targeting a specific ID directly from the CLI arguments.
- **Concurrent modification**: SQLite transactions will protect integrity, but if the active session is deleted externally, the TUI might need to gracefully handle `ErrNotFound`.

## Risks and Tradeoffs
- **Risk**: Modifying `internal/session.Manager` methods to take explicit IDs might diverge from its original "active session stateholder" design.
- **Tradeoff**: Implementing nested subcommands with stdlib `flag` is slightly more verbose and brittle than using a router like `cobra`, but maintains zero-dependency consistency with the rest of the `agis` ecosystem.

## Rollback
- Drop `cmd/agis/session.go`, remove routing from `cmd/agis/main.go`, and revert `internal/session.Manager` additions. No schema migrations are required, making rollback trivial.

## Success Criteria
- `agis session <cmd>` operations execute cleanly.
- `Delete` successfully cascades (verified via DB state).
- `Export` outputs correctly formatted JSON, Markdown, and Plaintext to `stdout`.
- Appropriate POSIX exit codes (0, 1, 2) are reliably returned on success, failure, and usage errors.

## Proposal Question Round
1. **Interactive TUI fallback**: Should deleting the currently active conversation via CLI attempt to gracefully handle the event for a running TUI process, or do we treat CLI operations as completely distinct from runtime TUI state?
2. **Export output scope**: Should the markdown and plaintext exports include the tool use/system prompts, or strictly the human/assistant message pairs?
3. **List formatting**: For the `list` command, do we want a tabular output, or basic newline-delimited (ID + title) output to make `awk/grep` scripting easier?
4. **Manager API update**: The existing `Snapshot` method relies on the TUI's `activeID`. Is it acceptable to add a new `SnapshotSession(ctx, id)` method specifically for the CLI, leaving the TUI's `Snapshot(ctx)` untouched?
