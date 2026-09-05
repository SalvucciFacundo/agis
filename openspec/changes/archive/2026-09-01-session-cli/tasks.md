# Tasks: session-cli

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~650-850 additions across core, session manager, CLI router, and test suites |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: Repository delete verification & Session Manager stateless extensions (`internal/session`) <br>PR 2: CLI Subcommand router, subcommands (`cmd/agis/session.go`), unit/integration tests, and main routing (`cmd/agis/main.go`) |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

---

## Work Unit Breakdown

### Task 1: Repository port & SQLite implementation verification (`DeleteConversation` unit test with cascade verification)
- [x] Create or extend unit tests in `internal/db` (or repository test suite) to verify `DeleteConversation(ctx, id)` cascades correctly to messages, snapshots, and attachments without leaving orphans. <!-- sdd-owner: implementation -->
- [x] Verify foreign key cascade behavior under SQLite when deleting a conversation row. <!-- sdd-owner: implementation -->
- [x] Run test suite for Task 1 using the project test runner and verify green status. <!-- sdd-owner: implementation -->

### Task 2: `internal/session.Manager` extensions (`Show`, `Delete`, `Export`, `SnapshotSession`) and unit tests
- [x] Implement `ExportFormat` enum and type definitions in `internal/session/manager.go`. <!-- sdd-owner: implementation -->
- [x] Add `Show(ctx context.Context, id string) (*core.Conversation, []core.Message, error)` method to `session.Manager`. <!-- sdd-owner: implementation -->
- [x] Add `Delete(ctx context.Context, id string) error` method to `session.Manager` ensuring `activeID` reset when matching. <!-- sdd-owner: implementation -->
- [x] Add `SnapshotSession(ctx context.Context, id string) (*core.Snapshot, error)` and update `Snapshot(ctx)` delegation. <!-- sdd-owner: implementation -->
- [x] Add `Export(ctx context.Context, id string, format ExportFormat) ([]byte, error)` supporting `json`, `markdown`, and `txt` serializers. <!-- sdd-owner: implementation -->
- [x] Write comprehensive unit tests in `internal/session/manager_test.go` covering all new methods, error states (`core.ErrNotFound`), and serialization formats. <!-- sdd-owner: implementation -->
- [x] Run `go test ./internal/session/...` and verify green status under Strict TDD rules. <!-- sdd-owner: implementation -->

### Task 3: `cmd/agis/session.go` subcommand router implementation and integration tests in `cmd/agis/session_test.go`
- [x] Create `cmd/agis/session.go` implementing `HandleSession` and subcommand handlers (`list`, `show`, `delete`, `rename`, `export`, `snapshot`) using stdlib `flag`. <!-- sdd-owner: implementation -->
- [x] Implement strict POSIX stream separation (`stdout` for data/success notices, `stderr` for errors/usage/warnings) and exit code mapping (0 for success, 1 for domain/runtime errors, 2 for flag/usage errors). <!-- sdd-owner: implementation -->
- [x] Implement `-yes` / `-y` guard for non-interactive `delete` operations and prompt handling for interactive mode. <!-- sdd-owner: implementation -->
- [x] Implement prompt injection sanitization (`scan.Lines`) for `rename`. <!-- sdd-owner: implementation -->
- [x] Create comprehensive integration tests in `cmd/agis/session_test.go` asserting stream isolation, exit codes, JSON outputs, and flag combinations. <!-- sdd-owner: implementation -->
- [x] Run `go test ./cmd/agis/...` and verify green status. <!-- sdd-owner: implementation -->

### Task 4: CLI router wiring in `cmd/agis/main.go`, documentation update, and end-to-end verification
- [x] Wire `session` subcommand into `cmd/agis/main.go` alongside `doctor` and `policy`. <!-- sdd-owner: implementation -->
- [x] Update `docs/cli.md` and `README.md` to document the new `agis session` commands and flags. <!-- sdd-owner: implementation -->
- [x] Run full project test suite (`go test ./...`) and perform manual end-to-end smoke test verifying clean exit codes and stream separation. <!-- sdd-owner: implementation -->
- [x] Start or reuse bounded review. <!-- sdd-owner: parent -->
