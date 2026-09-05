# Tasks: Setup Wizard & Multi-Profile Management Subsystem (`setup-profiles`)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1450–1850 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 → PR 4 |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

```text
Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High
```

---

## Work Units

### Work Unit 1: Profile Resolution Engine & Management API
- [x] Create `internal/config/profile.go` defining `ProfilePaths`, `ProfileInfo`, validation regex `^[a-zA-Z0-9_-]+$`, and precedence resolver (`--profile` flag, `AGIS_PROFILE` env, `.active_profile` file, default root). <!-- sdd-owner: implementation -->
- [x] Create `internal/config/profile_manager.go` implementing `ProfileManager` operations: `List`, `Create` (with `-clone` support and fresh database isolation), `Show`, `Switch` (`use`), and `Delete` (with active profile safeguards and `-force`). <!-- sdd-owner: implementation -->
- [x] Implement strict POSIX file mode enforcement (`0700` for profile directories, `0600` for `.active_profile` pointer and profile configs). <!-- sdd-owner: implementation -->
- [x] Write comprehensive unit tests in `internal/config/profile_test.go` covering name validation, precedence rules, directory scaffolding, cloning, and deletion guards. <!-- sdd-owner: implementation -->

### Work Unit 2: Setup Wizard Engine & Connectivity Probe
- [x] Create `internal/setup/probe.go` implementing bounded 5-second `context.WithTimeout` connectivity probes for `ollama`, `openai`, `openrouter`, and `anthropic`. <!-- sdd-owner: implementation -->
- [x] Create `internal/setup/wizard.go` implementing interactive prompt loop (with masked password reader for API keys) and headless non-interactive automation mode (`-non-interactive`, `-force`). <!-- sdd-owner: implementation -->
- [x] Implement atomic file writer with parent directory creation (`0700`), temporary file flush (`Sync()`), `chmod 0600`, and atomic rename for configuration persistence. <!-- sdd-owner: implementation -->
- [x] Write unit tests in `internal/setup/wizard_test.go` and `internal/setup/probe_test.go` using mock TTY streams and `httptest.Server` stubs. <!-- sdd-owner: implementation -->

### Work Unit 3: CLI Subcommands & Global Integration
- [x] Create `cmd/agis/setup.go` implementing `agis setup` and `agis init` entry points with proper flag parsing (`-provider`, `-model`, `-api-key`, `-base-url`, `-non-interactive`, `-force`, `-config`, `-profile`) and POSIX exit codes (`0`, `1`, `2`). <!-- sdd-owner: implementation -->
- [x] Create `cmd/agis/profile.go` implementing the `agis profile` subcommand router (`list`, `create`, `show`, `use`/`switch`, `delete`) with JSON formatting support (`-json`) and strict stream separation (`stdout` for data, `stderr` for diagnostics). <!-- sdd-owner: implementation -->
- [x] Update root command in `cmd/agis/main.go` to intercept global `--profile` / `-profile` flags early and configure active profile context before loading configuration or connecting to SQLite. <!-- sdd-owner: implementation -->
- [x] Write CLI integration tests in `cmd/agis/setup_test.go` and `cmd/agis/profile_test.go`. <!-- sdd-owner: implementation -->

### Work Unit 4: Doctor Health Diagnostics & Documentation
- [x] Create `internal/doctor/profile.go` implementing doctor health probes for active profile validation and configuration file permission checks (`0600` vs loose permissions). <!-- sdd-owner: implementation -->
- [x] Integrate profile and configuration permission checks into `internal/doctor/doctor.go` main check suite. <!-- sdd-owner: implementation -->
- [x] Update user documentation in `docs/cli.md`, `docs/configuration.md`, and `README.md` covering `agis setup`, `agis init`, and `agis profile` management. <!-- sdd-owner: implementation -->
- [x] Run full test suite, verify strict POSIX permissions (`0600`/`0700`), and perform end-to-end verification. <!-- sdd-owner: implementation -->

---

## Key Learnings
1. Multi-profile isolation requires path resolution to be strictly centralized rather than reading `AGIS_HOME` directly in individual modules.
2. Atomic file persistence with temporary files and explicit `chmod 0600` before rename prevents race conditions and credential leaks.
3. Bounded connectivity probes with 5-second timeouts prevent setup hangs when provider endpoints are unreachable.
4. Early flag interception in Cobra/CLI root commands ensures all subcommands inherit the active profile without duplicate parsing.
