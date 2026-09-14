# Apply Progress: Profile Portability & Migration (profile-portability)

## Status: COMPLETE

- **Change:** `2026-09-13-profile-portability`
- **Target Specification:** `openspec/specs/profile-portability/spec.md`
- **Execution Date:** 2026-09-13
- **TDD Mode:** Strict TDD (RED -> GREEN -> REFACTOR)

---

## Completed Tasks

### Work Unit 1: Backup Engine & Manifest (`internal/backup`)
- [x] Defined `Manifest`, `FileInfo` models and `HashFile` SHA-256 calculation in `internal/backup/manifest.go`.
- [x] Implemented `Create()` archiving and `Inspect()` manifest reading in `internal/backup/backup.go`.
- [x] Handled SQLite WAL/SHM file bundling to preserve transaction consistency.
- [x] Wrote unit tests in `internal/backup/backup_test.go` verifying archive structure and integrity.
- [x] All unit tests pass: `go test -v -race ./internal/backup/...`.

### Work Unit 2: Restore Engine & Safety Guards (`internal/backup`)
- [x] Implemented `Restore()` in `internal/backup/restore.go` with two-phase staging (`.restore-tmp-*`).
- [x] Added path traversal validation rejecting `..` and absolute paths.
- [x] Added checksum verification against `manifest.json` before commit.
- [x] Added overwrite protection requiring `Force: true` (`-force`).
- [x] Enforced strict POSIX permissions (`0600` for files, `0700` for directories).
- [x] Wrote unit tests verifying restore roundtrip, tamper rejection, overwrite guard, and permissions.
- [x] All unit tests pass: `go test -v -race ./internal/backup/...`.

### Work Unit 3: CLI Subcommands & Profile Aliases (`cmd/agis`)
- [x] Implemented `RunBackupCLI` and `RunRestoreCLI` in `cmd/agis/backup.go`.
- [x] Connected `agis backup` and `agis restore` in `cmd/agis/main.go`.
- [x] Added `agis profile backup` and `agis profile restore` aliases in `cmd/agis/profile.go`.
- [x] Updated profile CLI usage string.
- [x] Wrote CLI integration tests in `cmd/agis/backup_test.go`.
- [x] All tests pass: `go test -v -race ./cmd/agis/...`.

### Work Unit 4: Documentation & Roadmap
- [x] Updated `docs/development/hermes-parity-roadmap.md` marking Fase 10 as Shipped.
- [x] Updated `docs/cli.md` with full documentation for `agis backup`, `agis restore`, and profile aliases.
- [x] Verified zero regressions across entire suite: `go test -race ./...` and `go vet ./...`.
