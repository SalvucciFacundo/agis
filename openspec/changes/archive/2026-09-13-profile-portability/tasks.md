# Tasks: Profile Portability & Migration (profile-portability)

## Review Workload Forecast
- Estimated changed lines: ~450 additions, ~20 deletions
- 400-line budget risk: Low/Medium
- Chained PRs recommended: No (Single cohesive architectural unit)
- Delivery strategy: Single PR stacked to main
- TDD mode: Strict TDD (RED -> GREEN -> REFACTOR)

---

## Work Units

### Work Unit 1: Backup Engine & Manifest (`internal/backup`)
- [x] Define manifest schema models and hashing utilities in `internal/backup/manifest.go`.
- [x] Implement `Create` and `Inspect` in `internal/backup/backup.go`.
- [x] Write unit tests in `internal/backup/backup_test.go` covering archive generation, WAL file inclusion, and manifest accuracy.
- [x] Verify test pass: `go test -race ./internal/backup/...`.

### Work Unit 2: Restore Engine & Safety Guards (`internal/backup`)
- [x] Implement `Restore` in `internal/backup/restore.go` with checksum verification, path traversal prevention, and atomic staging.
- [x] Implement overwrite detection and `-force` handling.
- [x] Write unit tests covering roundtrip restore, corrupted archive rejection, hash mismatch rollback, and file permissions.
- [x] Verify test pass: `go test -race ./internal/backup/...`.

### Work Unit 3: CLI Subcommands & Profile Aliases (`cmd/agis`)
- [x] Implement `RunBackupCLI` and `RunRestoreCLI` in `cmd/agis/backup.go`.
- [x] Wire top-level `agis backup` and `agis restore` into `cmd/agis/main.go`.
- [x] Wire aliases `agis profile backup` and `agis profile restore` in `cmd/agis/profile.go`.
- [x] Write CLI integration tests in `cmd/agis/backup_test.go`.
- [x] Verify test pass: `go test -race ./cmd/agis/...`.

### Work Unit 4: Verification, Documentation & Roadmap
- [x] Update `docs/development/hermes-parity-roadmap.md` (mark Fase 10 as ✅ DONE).
- [x] Update `docs/cli.md`.
- [x] Run full regression suite: `go test -race ./...` and `go vet ./...`.
- [x] Commit with conventional commits and push.
