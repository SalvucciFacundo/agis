# Archive Report: Profile Portability & Migration (profile-portability)

## Metadata
- **Change Name:** `2026-09-13-profile-portability`
- **Archived Date:** 2026-09-13
- **Author/Team:** Senior Architect / AGIS Core Team
- **Target Spec:** `openspec/specs/profile-portability/spec.md`

## Summary of Delivery
Implemented Phase 10: Profile Portability for the AGIS agent system.
- Package `internal/backup` provides single-pass `.tar.gz` archiving and atomic restoration of agent profiles.
- Integrated SQLite WAL mode consistency snapshots (`agis.db`, `agis.db-wal`, `agis.db-shm`).
- Implemented cryptographic manifest generation and validation with SHA-256 hashes for all archived files.
- Built-in path traversal sanitization and strict POSIX permission hardening (`0600` files, `0700` directories).
- CLI subcommands `agis backup` and `agis restore` with aliases `agis profile backup` and `agis profile restore`.
- Full documentation in `docs/cli.md` and updated Hermes parity roadmap `docs/development/hermes-parity-roadmap.md`.
- 100% test pass under `go test -race ./...` and zero issues under `go vet ./...`.
