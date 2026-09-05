# Archive Report: update-cli

## Change Overview
- **Name**: `update-cli`
- **Archived Date**: 2026-09-01
- **Status**: Completed & Archived
- **Mode**: Automatic (auto)
- **Artifact Store**: Hybrid (`openspec/` + Engram)

## Summary of Accomplishments
1. **Version Package (`internal/version`)**:
   - Implemented SemVer parsing, build variable injection (`Version`, `Commit`, `BuildDate`), and comparison functions (`Compare`, `IsNewer`).
   - Covered SemVer comparison matrix, `v` prefix stripping, and `dev` build handling in `internal/version/version_test.go`.
2. **Updater Core Subsystem (`internal/updater`)**:
   - Implemented state backup (`backup.go`) archiving `$AGIS_HOME` components (`agis.db`, `config.yaml`, `policy.yaml`, `SOUL.md`, `skills/`, `plugins/`) into timestamped `.tar.gz` files inside `$AGIS_HOME/backups/`.
   - Implemented SHA-256 checksum verification (`verify.go`) parsing `checksums.txt`.
   - Built GitHub Releases client (`client.go`) with GoReleaser asset discovery across OS/architectures.
   - Built cross-platform atomic binary replacer (`apply.go`) supporting Unix atomic rename and Windows `.old` renaming dance.
3. **CLI Subcommand Router**:
   - Implemented `cmd/agis/update.go` and wired it into `cmd/agis/main.go`.
   - Supported flags: `-check`, `-backup`, `-version <tag>`, `-force`, `-config`.
   - Maintained strict POSIX stream separation (`stdout` for outcomes, `stderr` for logs) and exit code discipline (0, 1, 2).
4. **Verification & Quality**:
   - 100% test pass rate across unit and integration tests under `-race` (`go test -race -count=1 ./...`).
   - 9/9 RFC 2119 requirement specifications verified.
   - Master capability specs synchronized in `openspec/specs/cli/spec.md`.

## Artifact Inventory
- `proposal.md`
- `spec.md`
- `design.md`
- `tasks.md`
- `apply-progress.md`
- `verify-report.md`
- `archive-report.md`
