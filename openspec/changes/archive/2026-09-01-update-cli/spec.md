# Specification: AGIS Update CLI and In-Place Self-Updater

## Purpose

Define the command-line interface and underlying updater and version packages for `agis update`, providing self-updating capabilities for the AGIS binary across supported operating systems and architectures with release inspection, checksum verification, automated backup, and atomic executable replacement.

---

## Requirements

### CLI Subcommand (`cmd/agis`)

#### Requirement CLI-UPD-001: Subcommand Registration and Flag Parsing
The `cmd/agis` CLI entry point MUST provide an `agis update` subcommand routed via `RunUpdateCLI(args []string, stdout, stderr io.Writer) int`.
- The command MUST accept the following flags:
  - `--check` (or `-check`): Query GitHub release metadata, compare against local binary version, report availability, and exit without downloading or modifying the local executable.
  - `--backup` (or `-backup`): Archive `$AGIS_HOME` state files (`agis.db`, `config.yaml`, `policy.yaml`, `SOUL.md`, `skills/`, `plugins/`) into a `.tar.gz` file inside `$AGIS_HOME/backups/` before performing any binary update.
  - `--version <tag>` (or `-version <tag>`): Target a specific release tag (e.g., `v0.4.0`) rather than the latest release, allowing both explicit upgrades and downgrades.
  - `--force` (or `-force`): Bypass version equality or downgrade checks, forcing a re-download, verification, and replacement even if the local binary matches or exceeds the target version.
  - `--config <path>` (or `-config <path>`): Specify custom configuration path (to resolve `$AGIS_HOME` if set).
  - `--help` / `-h`: Print help and usage information to `stdout` and exit with code 0.
- When invoked with invalid flags or unexpected positional arguments, the command MUST write a descriptive error to `stderr`, print usage, and exit with code 2.

##### Scenario: Display update help
- GIVEN the user executes `agis update --help` or `agis update -h`
- WHEN the command is evaluated
- THEN the system prints the available update flags and descriptions to `stdout` and exits with code 0

##### Scenario: Unrecognized flag returns usage error
- GIVEN the user executes `agis update --invalid-option`
- WHEN the command flags are parsed
- THEN the system prints a flag error message to `stderr` and exits with code 2

---

#### Requirement CLI-UPD-002: Version Check (`--check`)
When `--check` is specified, `agis update` MUST check release availability without downloading or replacing the binary.
- The command MUST query GitHub releases for the target version (latest release or specified `--version <tag>`).
- It MUST compare the target version against the currently running binary version obtained from `internal/version`.
- If an update is available:
  - It MUST print an informational message to `stdout` (e.g., `"Update available: v0.3.0 -> v0.4.0"`).
  - It MUST exit with code 0.
- If the current binary is already up-to-date:
  - It MUST print a message to `stdout` (e.g., `"agis is up to date (v0.3.0)"`).
  - It MUST exit with code 0.
- If the local binary is a development build (`dev`), it MUST report the latest release version and indicate that an update is available unless `--version` targets `"dev"`.

##### Scenario: Check reports available update
- GIVEN the current binary version is `v0.3.0` and the latest GitHub release is `v0.4.0`
- WHEN the user executes `agis update --check`
- THEN the system prints `"Update available: v0.3.0 -> v0.4.0"` to `stdout`, makes no filesystem modifications, and exits with code 0

##### Scenario: Check reports binary is up to date
- GIVEN the current binary version is `v0.4.0` and the latest GitHub release is `v0.4.0`
- WHEN the user executes `agis update --check`
- THEN the system prints `"agis is already up to date (v0.4.0)"` to `stdout` and exits with code 0

---

#### Requirement CLI-UPD-003: In-Place Binary Update Flow
When `agis update` is invoked without `--check`:
- The system MUST determine the target version (latest release or explicitly specified `--version <tag>`).
- If the binary is already up-to-date and `--force` is not set, the command MUST print `"agis is already up to date (<version>). Use --force to re-install."` to `stdout` and exit with code 0.
- If an update is required or `--force` is set:
  - If `--backup` is set, it MUST execute the backup procedure before downloading.
  - The system MUST download the platform asset matching `runtime.GOOS` and `runtime.GOARCH` and the corresponding `checksums.txt` from the GitHub release.
  - The system MUST compute the SHA-256 hash of the downloaded asset and verify it against `checksums.txt`. If the checksum does not match, the update MUST be aborted immediately.
  - The system MUST extract/prepare the new executable and replace the currently running binary in-place atomically.
  - On successful replacement, it MUST print `"Successfully updated agis to <version>"` to `stdout` and exit with code 0.

##### Scenario: Standard update to latest release
- GIVEN the current binary version is `v0.3.0` and the latest release is `v0.4.0`
- WHEN the user executes `agis update`
- THEN the system downloads the `v0.4.0` binary for the current OS/architecture, validates its checksum against `checksums.txt`, atomically replaces the current binary, prints `"Successfully updated agis to v0.4.0"` to `stdout`, and exits with code 0

##### Scenario: Attempted update when already on latest version without force
- GIVEN the current binary version is `v0.4.0` and the latest release is `v0.4.0`
- WHEN the user executes `agis update`
- THEN the system prints `"agis is already up to date (v0.4.0). Use --force to re-install."` to `stdout`, downloads no files, and exits with code 0

---

#### Requirement CLI-UPD-004: State and Configuration Backup (`--backup`)
When the `--backup` flag is provided:
- The system MUST locate `$AGIS_HOME` (defaulting to `~/.agis`).
- It MUST create a directory `$AGIS_HOME/backups/` if it does not already exist.
- It MUST create a timestamped gzip-compressed tarball with the naming pattern:
  `agis-backup-<YYYYMMDD-HHMMSS>.tar.gz` (or ISO8601-derived timestamp).
- The archive MUST include any existing critical state files:
  - `agis.db` (SQLite database)
  - `config.yaml`
  - `policy.yaml`
  - `SOUL.md`
  - `skills/` directory (recursively)
  - `plugins/` directory (recursively)
- Missing optional files or directories MUST be skipped without failing the backup.
- If no files can be found or backup creation fails due to I/O error, the update MUST abort with an error on `stderr` and exit with code 1 before modifying the binary.
- On successful backup, a diagnostic message MUST be written to `stderr` indicating the created backup archive path.

##### Scenario: Backup created before update
- GIVEN `$AGIS_HOME` containing `config.yaml`, `agis.db`, and `SOUL.md`
- WHEN the user executes `agis update --backup`
- THEN a new archive `$AGIS_HOME/backups/agis-backup-*.tar.gz` containing `config.yaml`, `agis.db`, and `SOUL.md` is created before the binary is updated, and the command proceeds to update the binary

##### Scenario: Backup failure aborts update
- GIVEN `$AGIS_HOME/backups/` is not writable
- WHEN the user executes `agis update --backup`
- THEN the system writes a backup error to `stderr`, aborts without downloading or replacing the binary, and exits with code 1

---

#### Requirement CLI-UPD-005: Version Pinning and Force Re-install (`--version`, `--force`)
- When `--version <tag>` is provided:
  - The system MUST fetch the release corresponding to `<tag>` (normalizing leading `v` prefixes if necessary).
  - If `<tag>` does not exist on GitHub, the system MUST print an error to `stderr` (e.g., `"release tag 'v9.9.9' not found"`) and exit with code 1.
  - The system MUST allow downgrading to an older version when `--version` is explicitly passed.
- When `--force` is provided:
  - The system MUST perform the download, verification, and replacement even if the target version matches the current version or is older.

##### Scenario: Explicit downgrade using `--version`
- GIVEN the current binary version is `v0.4.0` and release `v0.3.0` exists
- WHEN the user executes `agis update --version v0.3.0`
- THEN the system downloads and installs `v0.3.0`, prints `"Successfully updated agis to v0.3.0"` to `stdout`, and exits with code 0

##### Scenario: Force re-install same version
- GIVEN the current binary version is `v0.4.0` and the latest release is `v0.4.0`
- WHEN the user executes `agis update --force`
- THEN the system downloads `v0.4.0`, verifies checksum, replaces the binary, prints `"Successfully updated agis to v0.4.0"` to `stdout`, and exits with code 0

##### Scenario: Non-existent release tag
- GIVEN no release exists for tag `v99.0.0`
- WHEN the user executes `agis update --version v99.0.0`
- THEN the system writes `"release tag 'v99.0.0' not found"` to `stderr` and exits with code 1

---

#### Requirement CLI-UPD-006: POSIX Exit Codes and Stream Separation
The `agis update` command MUST strictly adhere to POSIX stream separation and exit code standards:
- Exit Codes:
  - `0`: Successful execution (update applied, binary up to date, or check completed).
  - `1`: General operational failure (network error, release not found, checksum mismatch, backup failure, write/permission error).
  - `2`: Command line usage error (invalid flags, unexpected positional arguments).
- Stream Separation:
  - `stdout`: Reserved strictly for primary human/machine readable status and outcomes (`"Update available..."`, `"agis is up to date..."`, `"Successfully updated..."`).
  - `stderr`: Reserved strictly for error messages, usage warnings, diagnostic notices, and progress logs.

##### Scenario: Network failure writes error to stderr and exits with code 1
- GIVEN no network connectivity or unreachable GitHub API
- WHEN the user runs `agis update`
- THEN the error details are written to `stderr`, nothing is written to `stdout`, and the command exits with code 1

##### Scenario: Invalid command argument returns code 2
- GIVEN the user runs `agis update extra-argument`
- WHEN argument parsing occurs
- THEN usage instructions are written to `stderr` and the command exits with code 2

---

### Package Contracts (`internal/version` & `internal/updater`)

#### Requirement UPD-PKG-001: Version Contract (`internal/version`)
The `internal/version` package MUST provide build version information and semantic version comparison utilities:
- Variables & Accessors:
  - `Version`: string (embedded via `-ldflags "-X ...Version=..."`, default: `"dev"`).
  - `Commit`: string (embedded via `-ldflags "-X ...Commit=..."`, default: `"none"`).
  - `BuildDate`: string (embedded via `-ldflags "-X ...BuildDate=..."`, default: `"unknown"`).
  - `Get()` or `Current()` returning formatted version info struct or string.
- Semantic Versioning Functions:
  - `Compare(v1, v2 string) (int, error)`: Compares two semantic version strings (ignoring leading `v`). Returns `-1` if `v1 < v2`, `0` if `v1 == v2`, and `1` if `v1 > v2`.
  - `IsNewer(target, current string) bool`: Returns `true` if `target` is semantically newer than `current`. If `current == "dev"`, `IsNewer` returns `true` for any valid release tag.

##### Scenario: Semver comparison
- GIVEN versions `"v0.3.0"` and `"v0.4.0"`
- WHEN `version.Compare("v0.3.0", "v0.4.0")` is called
- THEN it returns `-1` and a `nil` error

##### Scenario: Dev build comparison
- GIVEN current version `"dev"` and target release `"v0.1.0"`
- WHEN `version.IsNewer("v0.1.0", "dev")` is called
- THEN it returns `true`

---

#### Requirement UPD-PKG-002: GitHub Release Client and Asset Discovery (`internal/updater`)
The `internal/updater` package MUST interact with GitHub Releases API and verify asset integrity:
- Types & Interfaces:
  - `Release`: struct containing `TagName`, `Name`, `PublishedAt`, `Assets []Asset`, `HTMLURL`.
  - `Asset`: struct containing `Name`, `Size`, `BrowserDownloadURL`.
  - `Client`: struct or interface configured with `Repository` (e.g. `SalvucciFacundo/agis`), optional `HTTPClient`, and optional `BaseURL` (to facilitate unit testing via `httptest.Server`).
- Methods:
  - `FetchLatestRelease(ctx context.Context) (*Release, error)`: Retrieves the latest release from `https://api.github.com/repos/{owner}/{repo}/releases/latest`.
  - `FetchReleaseByTag(ctx context.Context, tag string) (*Release, error)`: Retrieves release metadata from `https://api.github.com/repos/{owner}/{repo}/releases/tags/{tag}`.
  - `FindAssetForPlatform(release *Release, goos, goarch string) (*Asset, error)`: Matches the asset name for the current OS/architecture (e.g. `agis_linux_amd64.tar.gz`, `agis_darwin_arm64.tar.gz`, `agis_windows_amd64.zip` or binary equivalents).
  - `DownloadAsset(ctx context.Context, asset *Asset) ([]byte, error)`: Downloads asset bytes with context timeout support.
  - `VerifyChecksum(assetBytes []byte, assetName string, checksumsContent string) error`: Parses `checksums.txt` (or `SHA256SUMS`), extracts the expected SHA-256 for `assetName`, computes the SHA-256 of `assetBytes`, and returns an error if they do not match.

##### Scenario: Asset resolution for Linux amd64
- GIVEN a release with assets `["agis_linux_amd64.tar.gz", "agis_darwin_arm64.tar.gz", "checksums.txt"]`
- WHEN `FindAssetForPlatform` is invoked with `goos="linux"` and `goarch="amd64"`
- THEN it returns the `Asset` corresponding to `"agis_linux_amd64.tar.gz"`

##### Scenario: Checksum verification mismatch fails
- GIVEN asset bytes with SHA-256 hash `"aaa..."` and `checksums.txt` containing `"bbb...  agis_linux_amd64.tar.gz"`
- WHEN `VerifyChecksum(assetBytes, "agis_linux_amd64.tar.gz", checksumsContent)` is called
- THEN it returns an `ErrChecksumMismatch` error and aborts

---

#### Requirement UPD-PKG-003: Backup and Atomic Binary Replacement (`internal/updater`)
The `internal/updater` package MUST provide safe atomic replacement of the executable and backup capabilities:
- Backup Function:
  - `CreateBackup(agisHome string, destDir string) (string, error)`: Reads state files from `agisHome` and writes a compressed tarball (`.tar.gz`) into `destDir`.
- Replacement Function:
  - `ApplyBinary(newBinaryData []byte, targetPath string) error`:
    1. Writes `newBinaryData` to a temporary file in the same directory as `targetPath` (e.g. `targetPath + ".tmp"`).
    2. Sets executable permissions (`0755` on Unix).
    3. Performs atomic replacement:
       - On Unix (Linux/macOS): `os.Rename(tempFile, targetPath)`.
       - On Windows: Renames running `targetPath` to `targetPath + ".old"` before moving `tempFile` to `targetPath`, avoiding executable file locking collisions.
    4. Cleans up temporary artifacts on failure.

##### Scenario: Atomic replacement on Unix
- GIVEN an existing executable at `/usr/local/bin/agis`
- WHEN `ApplyBinary(newBytes, "/usr/local/bin/agis")` is executed on Linux
- THEN a temporary file is written and renamed over `/usr/local/bin/agis` with `0755` permissions

##### Scenario: Failure cleanup
- GIVEN a write failure during binary replacement
- WHEN `ApplyBinary` fails
- THEN temporary files are removed and the original binary remains intact
