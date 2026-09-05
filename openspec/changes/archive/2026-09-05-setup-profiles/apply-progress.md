# Apply Progress: Setup Wizard & Multi-Profile Management Subsystem (`setup-profiles`)

## Status & Overview
- **Change:** `setup-profiles`
- **Batch:** Batch 2 (Work Unit 3 + Work Unit 4) - ALL WORK UNITS COMPLETE
- **Status:** Complete (Ready for verify)
- **Mode:** Strict TDD

---

## Completed Tasks

### Work Unit 1: Profile Resolution Engine & Management API
- [x] Create `internal/config/profile.go` defining `ProfilePaths`, `ProfileInfo`, validation regex `^[a-zA-Z0-9_-]+$`, and precedence resolver (`--profile` flag, `AGIS_PROFILE` env, `.active_profile` file, default root).
- [x] Create `internal/config/profile_manager.go` implementing `ProfileManager` operations: `List`, `Create` (with `-clone` support and fresh database isolation), `Show`, `Switch` (`use`), and `Delete` (with active profile safeguards and `-force`).
- [x] Implement strict POSIX file mode enforcement (`0700` for profile directories, `0600` for `.active_profile` pointer and profile configs).
- [x] Write comprehensive unit tests in `internal/config/profile_test.go` covering name validation, precedence rules, directory scaffolding, cloning, and deletion guards.

### Work Unit 2: Setup Wizard Engine & Connectivity Probe
- [x] Create `internal/setup/probe.go` implementing bounded 5-second `context.WithTimeout` connectivity probes for `ollama`, `openai`, `openrouter`, and `anthropic`.
- [x] Create `internal/setup/wizard.go` implementing interactive prompt loop (with masked password reader for API keys) and headless non-interactive automation mode (`-non-interactive`, `-force`).
- [x] Implement atomic file writer with parent directory creation (`0700`), temporary file flush (`Sync()`), `chmod 0600`, and atomic rename for configuration persistence.
- [x] Write unit tests in `internal/setup/wizard_test.go` and `internal/setup/probe_test.go` using mock TTY streams and `httptest.Server` stubs.

### Work Unit 3: CLI Subcommands & Global Integration
- [x] Create `cmd/agis/setup.go` implementing `agis setup` and `agis init` entry points with proper flag parsing (`-provider`, `-model`, `-api-key`, `-base-url`, `-non-interactive`, `-force`, `-config`, `-profile`) and POSIX exit codes (`0`, `1`, `2`).
- [x] Create `cmd/agis/profile.go` implementing the `agis profile` subcommand router (`list`, `create`, `show`, `use`/`switch`, `delete`) with JSON formatting support (`-json`) and strict stream separation (`stdout` for data, `stderr` for diagnostics).
- [x] Update root command in `cmd/agis/main.go` to intercept global `--profile` / `-profile` flags early and configure active profile context before loading configuration or connecting to SQLite.
- [x] Write CLI integration tests in `cmd/agis/setup_test.go` and `cmd/agis/profile_test.go`.

### Work Unit 4: Doctor Health Diagnostics & Documentation
- [x] Create `internal/doctor/profile.go` implementing doctor health probes for active profile validation and configuration file permission checks (`0600` vs loose permissions).
- [x] Integrate profile and configuration permission checks into `internal/doctor/doctor.go` main check suite.
- [x] Update user documentation in `docs/cli.md`, `docs/configuration.md`, and `README.md` covering `agis setup`, `agis init`, and `agis profile` management.
- [x] Run full test suite, verify strict POSIX permissions (`0600`/`0700`), and perform end-to-end verification.

---

## TDD Cycle Evidence

| Phase | Work Unit | Test File | Target Code | Outcome |
|---|---|---|---|---|
| RED | Unit 1 | `internal/config/profile_test.go` | `internal/config/profile.go`, `internal/config/profile_manager.go` | Tests failed compilation as expected (undefined types/methods). |
| GREEN | Unit 1 | `internal/config/profile_test.go` | `internal/config/profile.go`, `internal/config/profile_manager.go`, `internal/config/config.go` | All unit tests passed with `-race -count=1`. |
| RED | Unit 2 | `internal/setup/probe_test.go`, `internal/setup/wizard_test.go` | `internal/setup/probe.go`, `internal/setup/wizard.go` | Tests failed compilation (no non-test Go files). |
| GREEN | Unit 2 | `internal/setup/probe_test.go`, `internal/setup/wizard_test.go` | `internal/setup/probe.go`, `internal/setup/wizard.go` | All probe & wizard tests passed with `-race -count=1`. |
| RED | Unit 3 | `cmd/agis/setup_test.go`, `cmd/agis/profile_test.go` | `cmd/agis/setup.go`, `cmd/agis/profile.go`, `cmd/agis/main.go` | Tests failed compilation with undefined `RunSetupCLI` and `RunProfileCLI`. |
| GREEN | Unit 3 | `cmd/agis/setup_test.go`, `cmd/agis/profile_test.go` | `cmd/agis/setup.go`, `cmd/agis/profile.go`, `cmd/agis/main.go` | CLI subcommand & integration tests passed with `-race -count=1`. |
| RED | Unit 4 | `internal/doctor/profile_test.go` | `internal/doctor/profile.go`, `internal/doctor/doctor.go` | Tests failed asserting missing `profile` check in report. |
| GREEN | Unit 4 | `internal/doctor/profile_test.go`, `internal/doctor/doctor_test.go` | `internal/doctor/profile.go`, `internal/doctor/doctor.go` | Doctor profile diagnostics tests passed with `-race -count=1`. |
| REFACTOR | Units 1-4 | Entire project | `docs/cli.md`, `docs/configuration.md`, `README.md` | Full suite `go test -race -count=1 ./...` PASS (25 pkgs), `go vet ./...` clean. |

---

## Files Changed / Created

### Code & Tests:
- `internal/config/profile.go` (New: types, validation, resolution functions, package helpers)
- `internal/config/profile_manager.go` (New: filesystem-backed `DefaultProfileManager` implementing `ProfileManager`)
- `internal/config/profile_test.go` (New: comprehensive unit tests for profile resolution, manager lifecycle, clone isolation, permissions)
- `internal/config/config.go` (Updated: path resolution functions mapped to active profile)
- `internal/setup/probe.go` (New: bounded 5s connectivity probes for Ollama, OpenAI, OpenRouter, Anthropic)
- `internal/setup/wizard.go` (New: interactive prompt loop & headless non-interactive automation setup wizard)
- `internal/setup/probe_test.go` (New: mock `httptest.Server` unit tests for probe endpoints, auth failures, timeouts)
- `internal/setup/wizard_test.go` (New: mock I/O stream tests for non-interactive and interactive flows, file modes)
- `cmd/agis/setup.go` (New: `RunSetupCLI` supporting flags, aliases, help, and exit codes)
- `cmd/agis/profile.go` (New: `RunProfileCLI` supporting list, create, show, use/switch, delete)
- `cmd/agis/setup_test.go` (New: CLI integration tests for setup command)
- `cmd/agis/profile_test.go` (New: CLI integration tests for profile subcommands)
- `cmd/agis/main.go` (Updated: global `--profile` early interception and setup/profile routing)
- `internal/doctor/profile.go` (New: `checkProfile` health probe)
- `internal/doctor/profile_test.go` (New: tests for active profile and permissions probe)
- `internal/doctor/doctor.go` (Updated: registered `checkProfile` check)
- `internal/doctor/doctor_test.go` (Updated: added `profile` check assertion)

### Documentation & Tracking:
- `docs/cli.md` (Updated: documented `agis setup`, `agis init`, `agis profile`, and global `--profile`)
- `docs/configuration.md` (Updated: added Multi-Profile Configuration & State Isolation section)
- `README.md` (Updated: added setup and profile CLI command references)
- `openspec/changes/setup-profiles/tasks.md` (Updated: checked off all 4 work units)
- `openspec/changes/setup-profiles/apply-progress.md` (Updated: cumulative progress record)

---

## Test Commands Run
- `go test -race -count=1 ./cmd/agis/...` -> PASS (3.546s)
- `go test -race -count=1 ./internal/doctor/...` -> PASS (1.181s)
- `go test -race -count=1 ./internal/config/...` -> PASS (1.090s)
- `go test -race -count=1 ./internal/setup/...` -> PASS (1.132s)
- `go test -race -count=1 ./...` -> PASS (all 25 packages green)
- `go vet ./...` -> PASS (zero warnings across entire project)

---

## Deviations from Design
None. All components adhere strictly to the design and specification contracts.

---

## Remaining Tasks
None. All implementation tasks across Work Units 1, 2, 3, and 4 are complete.
