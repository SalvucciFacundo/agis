# Proposal: Profile Portability & Migration (profile-portability)

## Intent
The goal of this change is to enable seamless packaging, export, import, and migration of AGIS agent profiles across machines and environments. Currently, AGIS profiles isolate state under `$AGIS_HOME/profiles/<name>/`, but there is no atomic or standardized mechanism to bundle an agent's identity (`SOUL.md`), configuration (`config.yaml`), policy permissions (`policy.yaml`), custom skills (`skills/`), installed plugins (`plugins/`), and persistent memory database (`agis.db`) into a portable, verified archive.

Introducing `agis backup` and `agis restore` solves this by producing self-contained `.tar.gz` bundles with cryptographic checksum verification (`manifest.json`), SQLite WAL-safe consistency, and overwrite safeguards.

## Scope
1. **Core Backup & Restore Engine (`internal/backup`)**:
   - `Create(profileName string, w io.Writer, opts CreateOptions) (*Manifest, error)`
   - `Restore(r io.Reader, targetProfile string, opts RestoreOptions) (*RestoreReport, error)`
   - `Inspect(r io.Reader) (*Manifest, error)`
2. **Database Integrity in WAL Mode**:
   - Safe SQLite snapshotting ensuring that pending transactions and un-checkpointed WAL frames in `agis.db-wal` are cleanly included or flushed into the archived snapshot.
3. **Archive Manifest Contract (`manifest.json`)**:
   - Header containing manifest version, AGIS version, source profile name, creation timestamp, and SHA-256 file checksums.
4. **Safety & Overwrite Protection**:
   - Abort on restore if target profile directory already exists, unless explicitly authorized via `-force` (`-f`).
   - Enforce strict POSIX permissions (0600 for sensitive configurations and database files, 0700 for directories).
5. **CLI Interfaces**:
   - Top-level commands: `agis backup [profile] [-o file.tar.gz]` and `agis restore <tarball> [-profile name] [-force]`.
   - Command aliases in `agis profile backup` and `agis profile restore`.

## Affected Areas
- `internal/backup/` (new package: `manifest.go`, `backup.go`, `restore.go`, `backup_test.go`)
- `cmd/agis/backup.go` (new file for backup/restore subcommands)
- `cmd/agis/backup_test.go` (integration tests)
- `cmd/agis/main.go` (subcommand routing)
- `cmd/agis/profile.go` (subcommand aliases)
- `docs/development/hermes-parity-roadmap.md`, `docs/cli.md`

## Risks & Mitigations
- **SQLite Locking / Inconsistency**: Copying an active database could yield torn pages or missing WAL commits. Mitigated by checking for WAL/SHM files and copying all database components or utilizing checkpointing before archiving.
- **Accidental Overwrite on Restore**: Restoring an archive into an existing profile could wipe active user conversations. Mitigated by mandatory overwrite guards requiring `-force` / `-f`.
- **Path Traversal in Tar Archives**: Malicious tarballs with `../../` entries could write outside target profile directories. Mitigated by strict path sanitization (`filepath.Clean` and prefix checks) on extraction.

## Success Criteria
- [ ] `agis backup` creates a compressed `.tar.gz` bundle containing manifest, config, db, soul, policy, and skills.
- [ ] `agis restore` reconstructs the profile with matching SHA-256 checksums and proper permissions (0600/0700).
- [ ] Database restored from an active WAL state opens successfully with `sqlite3` without corruption.
- [ ] Overwrite of existing profiles is rejected unless `-force` is set.
- [ ] 100% test coverage under `go test -race ./...` with zero leaks.
