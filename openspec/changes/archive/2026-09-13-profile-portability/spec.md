# Specification: Profile Portability & Migration (profile-portability)

## Purpose
Define the technical specifications for exporting, verifying, and importing AGIS agent profiles, including archive formatting, manifest schema, database consistency, path sanitization, and CLI commands.

---

## Requirements

### Requirement BKP-001: Archive Structure & Manifest Schema
1. The archive MUST be a gzip-compressed POSIX tarball (`.tar.gz`).
2. The root of the archive MUST contain `manifest.json` formatted as:
   ```json
   {
     "version": 1,
     "agis_version": "v1.4.0",
     "source_profile": "coder",
     "created_at": "2026-09-13T21:30:00Z",
     "files": {
       "config.yaml": { "size": 1420, "sha256": "<hex>" },
       "agis.db": { "size": 1048576, "sha256": "<hex>" }
     }
   }
   ```
3. Relative paths inside the tarball MUST NOT start with a leading slash `/` or contain directory traversal markers (`..`).

### Requirement BKP-002: Profile Snapshotting & Content Inclusions
1. When creating a backup of a profile directory:
   - `config.yaml` MUST be included if present.
   - `agis.db` MUST be included if present.
   - If SQLite WAL files exist (`agis.db-wal`, `agis.db-shm`), they MUST be captured to preserve transaction consistency.
   - `SOUL.md` MUST be included if present.
   - `policy.yaml` MUST be included if present.
   - `skills/` directory tree MUST be included recursively if present.
   - `plugins/` directory tree MUST be included recursively if present.
   - Cache directories (e.g. `cache/`) SHOULD be omitted to keep tarballs compact.

### Requirement BKP-003: Integrity Verification & Path Traversal Guards
1. During `Restore()`:
   - The engine MUST parse and validate `manifest.json`.
   - Every extracted file MUST have its SHA-256 hash verified against the manifest. If any hash mismatches, extraction MUST abort and rollback.
   - Every tar header path MUST be validated using `filepath.Clean` to ensure it resides strictly within the target profile directory. Any entry attempting path traversal outside the destination MUST return a security error.

### Requirement BKP-004: Overwrite Protection & Atomic Restore
1. If the target profile directory already exists and contains files:
   - `Restore()` MUST return an error unless `opts.Force` is `true`.
2. Restoring MUST extract to a temporary directory in the target filesystem before final commit to ensure all-or-nothing atomicity.
3. Extracted files MUST be assigned secure POSIX file modes:
   - Directories: `0700`
   - Files (`config.yaml`, `agis.db`, `policy.yaml`): `0600`
   - Public markdown (`SOUL.md`, skills): `0644` or `0600`.

### Requirement BKP-005: CLI Commands & Usability
1. `agis backup [profile]` command:
   - If profile argument is omitted, defaults to the active profile name.
   - Flag `-o <filename>` allows specifying output archive path. Default name: `agis-<profile>-<YYYYMMDD-HHMMSS>.tar.gz`.
   - Returns exit code 0 on success, reporting archive path, file count, and uncompressed size.
2. `agis restore <archive.tar.gz>` command:
   - Flag `-profile <name>` specifies target profile name. If omitted, defaults to `source_profile` from `manifest.json`.
   - Flag `-force` (`-f`) enables overwriting an existing profile.
   - Returns exit code 0 on success, reporting restored profile name and location.
3. `agis profile backup` and `agis profile restore` MUST be wired as transparent aliases in `cmd/agis/profile.go`.
