# Apply Progress: session-cli

## Summary
Successfully implemented the `session-cli` change under Strict TDD rules. All four implementation work units are complete, fully tested with unit and integration suites, and wired into `cmd/agis/main.go` and documentation.

## TDD Cycle Evidence

| Work Unit | Phase | Test Target / Evidence | Outcome |
|---|---|---|---|
| Task 1 | RED | `internal/core/brain_test.go`, `internal/persona/persona_test.go`, `internal/skills/hub_test.go`, `internal/adapters/tui/app_test.go` | Detected missing `DeleteConversation` across test doubles |
| Task 1 | GREEN | Added `DeleteConversation` to all test doubles; added `TestDeleteConversation_Cascade` and `TestDeleteConversation_NotFound` in `internal/memory/sqlite_test.go` | PASS (`go test -race ./internal/memory/...`) |
| Task 2 | RED | Added unit tests in `internal/session/manager_test.go` for `Show`, `Delete`, `SnapshotSession`, and `Export` (JSON, Markdown, TXT, plaintext, invalid format) | Compilation failed as methods and types were not yet declared |
| Task 2 | GREEN | Implemented `ExportFormat` enum, `Show`, `Delete`, `SnapshotSession`, `Snapshot` refactoring, and `Export` serializers in `internal/session/manager.go` | PASS (`go test -race ./internal/session/...`) |
| Task 2 | REFACTOR | Triangulated with `TestManager_Export_RichMessagesAndAttachments` checking summaries, attachments, and tool roles | PASS (`go test -race ./internal/session/...`) |
| Task 3 | RED | Added integration tests in `cmd/agis/session_test.go` covering CLI routing, POSIX stream isolation, flag combinations, non-interactive vs interactive `-yes` guards, and exit codes | Compilation failed as `RunSessionCLI` was not yet declared |
| Task 3 | GREEN | Implemented `cmd/agis/session.go` (`RunSessionCLI`, `RunSessionCLIWithIn`, subcommands `list`, `show`, `delete`, `rename`, `export`, `snapshot`) | PASS (`go test -race ./cmd/agis/...`) |
| Task 4 | GREEN | Wired `session` in `cmd/agis/main.go`, updated `docs/cli.md` and `README.md`, verified full test suite | PASS (`go test -race -count=1 ./...` and `go vet ./...`) |

## Files Changed
- `internal/core/brain_test.go` (added `DeleteConversation` to `fakeRepo`)
- `internal/persona/persona_test.go` (added `DeleteConversation` to `fakeEvolutionRepo`)
- `internal/skills/hub_test.go` (added `DeleteConversation` to `fakeSkillRepo`)
- `internal/adapters/tui/app_test.go` (added `DeleteConversation` to `fakeRepo`)
- `internal/memory/sqlite_test.go` (cascade deletion and not-found unit tests)
- `internal/session/manager.go` (`ExportFormat`, `Show`, `Delete`, `SnapshotSession`, `Export`)
- `internal/session/manager_test.go` (unit tests for new Manager methods)
- `cmd/agis/session.go` (CLI subcommand router and handlers)
- `cmd/agis/session_test.go` (integration test suite for session CLI)
- `cmd/agis/main.go` (wired `session` subcommand)
- `docs/cli.md` (CLI reference documentation for `agis session`)
- `README.md` (overview documentation)
- `openspec/changes/session-cli/tasks.md` (persisted task checkboxes)

## Test Commands Run
- `go test -race ./internal/memory/...` (PASS)
- `go test -race ./internal/session/...` (PASS)
- `go test -race ./cmd/agis/...` (PASS)
- `go test -race -count=1 ./...` (PASS)
- `go vet ./...` (PASS)

## Remaining Deferred Tasks
- `[ ] Start or reuse bounded review. <!-- sdd-owner: parent -->`
