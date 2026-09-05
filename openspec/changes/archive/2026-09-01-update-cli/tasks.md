# SDD Tasks: update-cli

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~650-850 additions across new packages and command |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: `internal/version` & `internal/updater` core (client, backup, verification) -> PR 2: Atomic binary replacement, CLI routing (`RunUpdateCLI`), flags, and integration tests |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

---

## Tasks Breakdown (Strict TDD Sequencing)

### Task 1: `internal/version` Package & SemVer Utilities
- [x] **RED**: Create unit tests in `internal/version/version_test.go` covering SemVer comparison, `Compare()` edge cases (`v` prefixes, invalid versions), `IsNewer()`, and the `"dev"` build fallback condition. <!-- sdd-owner: implementation -->
- [x] **GREEN**: Implement `internal/version/version.go` defining build variables (`Version`, `Commit`, `BuildDate`), `Info` struct, `Get()`, `Compare()`, and `IsNewer()` to make tests pass. <!-- sdd-owner: implementation -->
- [x] **TRIANGULATE**: Add additional test cases in `internal/version/version_test.go` for pre-release tags, malformed strings, and equal versions. <!-- sdd-owner: implementation -->
- [x] **REFACTOR**: Ensure code clarity, adherence to Go idioms, and proper docstrings across `internal/version`. <!-- sdd-owner: implementation -->

### Task 2: `internal/updater` Core Backup & Checksum Verification
- [x] **RED**: Create unit tests in `internal/updater/backup_test.go` and `internal/updater/verify_test.go` verifying `$AGIS_HOME` archive creation (`.tar.gz`), missing optional file skipping, backup error aborts, and SHA-256 `checksums.txt` validation success/mismatch. <!-- sdd-owner: implementation -->
- [x] **GREEN**: Implement `internal/updater/backup.go` (tar.gz creation targeting `agis.db`, `config.yaml`, `policy.yaml`, `SOUL.md`, `skills/`, `plugins/`) and `internal/updater/verify.go` (parsing checksums and comparing SHA-256). <!-- sdd-owner: implementation -->
- [x] **TRIANGULATE**: Test backup creation with partial/empty `$AGIS_HOME` directories and corrupt checksum files. <!-- sdd-owner: implementation -->
- [x] **REFACTOR**: Clean up I/O error handling, ensure proper deferred cleanup, and add logging hooks. <!-- sdd-owner: implementation -->

### Task 3: `internal/updater` GitHub Release Client, Asset Matching & Atomic Replacement
- [x] **RED**: Create unit tests in `internal/updater/client_test.go` and `internal/updater/apply_test.go` using `httptest.Server` for GitHub API mocking (latest release, tag retrieval, asset download, platform asset matching for Linux/macOS/Windows) and tempdir file replacement testing (including atomic rename and failure cleanup). <!-- sdd-owner: implementation -->
- [x] **GREEN**: Implement `internal/updater/client.go` (functional options, GitHub API structs, release fetching, asset download) and `internal/updater/apply.go` (`ApplyBinary` with cross-platform atomic replacement and Windows `.old` handling). <!-- sdd-owner: implementation -->
- [x] **TRIANGULATE**: Test rate-limiting errors, HTTP timeout context cancellation, and missing asset error handling. <!-- sdd-owner: implementation -->
- [x] **REFACTOR**: Verify request timeouts, proper HTTP header usage (User-Agent, Accept), and clean resource management. <!-- sdd-owner: implementation -->

### Task 4: CLI Router (`cmd/agis/update.go`), Wiring & Documentation
- [x] **RED**: Create unit/integration tests in `cmd/agis/update_test.go` testing `RunUpdateCLI` with various flag combinations (`--check`, `--backup`, `--version`, `--force`, `--config`), stream isolation (`stdout` vs `stderr`), and correct POSIX exit codes (0, 1, 2). <!-- sdd-owner: implementation -->
- [x] **GREEN**: Implement `cmd/agis/update.go` providing `NewUpdateCmd()` and `RunUpdateCLI()`, wire the subcommand into `cmd/agis/root.go` or `cmd/agis/main.go`, and ensure output separation. <!-- sdd-owner: implementation -->
- [x] **TRIANGULATE**: Test invalid flags, unknown positional arguments (exit code 2), network failure simulation (exit code 1), and up-to-date checks (exit code 0). <!-- sdd-owner: implementation -->
- [x] **REFACTOR**: Review overall CLI integration, update help text, ensure strict POSIX compliance, and verify no side-effects on existing commands. <!-- sdd-owner: implementation -->
