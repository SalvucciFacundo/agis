# Implementation Tasks: config-cli

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~350 - 450 lines |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Medium

---

## Task Breakdown (Strict TDD: RED -> GREEN -> TRIANGULATE -> REFACTOR)

- [x] Task 1.1: Write unit tests (`internal/config/accessor_test.go`) covering `ResolvePath`, dot-notation `Get`, dot-notation `Set` with strict type validation (bool, int, duration, string, string slice), and `MaskSecrets`. <!-- sdd-owner: implementation -->
- [x] Task 1.2: Implement `ResolvePath`, `Get`, `Set`, and `MaskSecrets` in `internal/config/accessor.go` and `internal/config/mask.go` to pass all accessor unit tests. <!-- sdd-owner: implementation -->
- [x] Task 2.1: Write unit tests (`internal/config/save_test.go`) for atomic configuration file saving, parent directory creation (`0700`), and strict `0600` file permission enforcement. <!-- sdd-owner: implementation -->
- [x] Task 2.2: Implement `Save` in `internal/config/save.go` using temporary files, sync, `0600` permissions, and atomic rename. <!-- sdd-owner: implementation -->
- [x] Task 3.1: Write integration tests (`cmd/agis/config_test.go`) for CLI routing, subcommands (`show`, `get`, `set`, `path`), flags (`-config`, `-json`, `-reveal`), POSIX exit codes (0, 1, 2), and stream separation (stdout vs stderr). <!-- sdd-owner: implementation -->
- [x] Task 3.2: Implement `RunConfigCLI` and subcommand handlers in `cmd/agis/config.go`. <!-- sdd-owner: implementation -->
- [x] Task 4.1: Wire the `config` root subcommand into the main CLI router in `cmd/agis/main.go`. <!-- sdd-owner: implementation -->
- [x] Task 4.2: Update CLI user documentation in `docs/cli.md` and `README.md`. <!-- sdd-owner: implementation -->
- [x] Task 4.3: Perform end-to-end verification and run full test suites across all packages. <!-- sdd-owner: implementation -->
- [ ] Start or reuse bounded review. <!-- sdd-owner: parent -->
