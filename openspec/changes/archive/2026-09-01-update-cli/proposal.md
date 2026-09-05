# SDD Proposal: update-cli

## 1. Intent
Implement a command-line interface `agis update` to support self-updating the AGIS binary. The update mechanism will check the latest release on GitHub, compare versions against the local binary, backup current data, download the new binary, verify its checksum, and replace the executable in-place across supported OS/architectures.

## 2. Scope
### In-Scope
- Subcommand `agis update` with flags:
  - `--check`: Only check for updates and exit.
  - `--backup`: Create a backup archive before updating.
  - `--version <tag>`: Update/downgrade to a specific version tag.
  - `--force`: Bypass version comparison and force the update.
- Version comparison using the `internal/version` package.
- Fetching release metadata from `https://api.github.com/repos/SalvucciFacundo/agis/releases/latest` (or specific tag endpoint).
- Checksum validation of the downloaded asset against `checksums.txt` provided in the GitHub release.
- Automated backup of `$AGIS_HOME` components (`agis.db`, `config.yaml`, `policy.yaml`, `SOUL.md`, `skills/`, `plugins/`) into a timestamped `.tar.gz` archive.
- Safe in-place binary replacement (handling OS-specific constraints like Windows file locks).
- Standard POSIX exit codes (0: success/up-to-date, 1: error, 2: usage error).
- Strict separation of streams (logs/progress to stderr, parsable output to stdout).

### Out-of-Scope
- Background auto-updates (cron/daemon).
- Updating external dependencies (e.g., system packages).
- Rollback functionality (backups are created, but restoring is manual for now).
- Delta updates (full binary download is required).

## 3. Affected Areas
- **CLI (`cmd/agis/update.go`)**: New subcommand registration via Cobra.
- **Update Logic (`internal/update/`)**: New package containing the core update mechanics, GitHub API interaction, downloading, checksum verification, and binary replacement.
- **Backup Logic (`internal/update/backup.go`)**: Tar.gz creation and $AGIS_HOME targeting.
- **Config / Version (`internal/version/`)**: Integration for current version checking.

## 4. Risks & Mitigations
- **Risk**: Binary replacement fails mid-write, leaving the system with a corrupted binary.
  - *Mitigation*: Download to a temporary file, verify checksums, and perform an atomic rename (or safe replacement on Windows) as the final step.
- **Risk**: GitHub API rate limits.
  - *Mitigation*: Use conditional requests if applicable, clearly report rate limit errors to the user, and advise providing a GITHUB_TOKEN if necessary (though unauthenticated should suffice for occasional checks).
- **Risk**: Windows file locking prevents replacing the running executable.
  - *Mitigation*: On Windows, rename the running executable to `.old` before moving the new binary into place (Windows allows renaming open files, but not deleting/overwriting).
- **Risk**: Backing up large databases or plugin directories consumes too much space/time.
  - *Mitigation*: Provide informative logging (stderr) during backup and only include critical user state.

## 5. Rollback Strategy
If the update process fails at any point *before* the final atomic swap, delete the temporary downloaded files and exit with code 1. The original binary remains untouched.
If the update succeeds but introduces issues, users can manually run `agis update --version <previous_tag>` to downgrade, or manually extract the created backup archive if data corruption occurred.

## 6. Success Criteria
- `agis update --check` accurately reports if a new version is available.
- `agis update` successfully downloads, verifies, and replaces the running binary on a supported platform (Linux/macOS/Windows).
- `agis update --backup` correctly creates a `.tar.gz` archive containing the specified `$AGIS_HOME` files.
- Appropriate exit codes are returned (0 for success/up-to-date, 1 for update failures, 2 for flag errors).
- All progress logs and diagnostic messages are routed to `stderr`, leaving `stdout` clean for scripting when necessary.
