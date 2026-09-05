# Apply Progress: config-cli

## Executive Summary
Implemented the `agis config` CLI subcommand router (`show`, `get`, `set`, `path`) and the underlying `internal/config` management extension (`ResolvePath`, `Get`, `Set`, `MaskSecrets`, `Save`) following Strict TDD (RED -> GREEN -> REFACTOR). All unit and integration test suites pass cleanly with race detection (`go test -race ./...`).

---

## Completed Tasks
- [x] Task 1.1: Write unit tests (`internal/config/accessor_test.go`) covering `ResolvePath`, dot-notation `Get`, dot-notation `Set` with strict type validation (bool, int, duration, string, string slice), and `MaskSecrets`.
- [x] Task 1.2: Implement `ResolvePath`, `Get`, `Set`, and `MaskSecrets` in `internal/config/accessor.go` and `internal/config/mask.go` to pass all accessor unit tests.
- [x] Task 2.1: Write unit tests (`internal/config/save_test.go`) for atomic configuration file saving, parent directory creation (`0700`), and strict `0600` file permission enforcement.
- [x] Task 2.2: Implement `Save` in `internal/config/save.go` using temporary files, sync, `0600` permissions, and atomic rename.
- [x] Task 3.1: Write integration tests (`cmd/agis/config_test.go`) for CLI routing, subcommands (`show`, `get`, `set`, `path`), flags (`-config`, `-json`, `-reveal`), POSIX exit codes (0, 1, 2), and stream separation (stdout vs stderr).
- [x] Task 3.2: Implement `RunConfigCLI` and subcommand handlers in `cmd/agis/config.go`.
- [x] Task 4.1: Wire the `config` root subcommand into the main CLI router in `cmd/agis/main.go`.
- [x] Task 4.2: Update CLI user documentation in `docs/cli.md` and `README.md`.
- [x] Task 4.3: Perform end-to-end verification and run full test suites across all packages.

---

## TDD Cycle Evidence

| Phase | Target | Test Evidence / Command | Outcome |
|---|---|---|---|
| RED | Task 1: Accessor & Mask | `go test ./internal/config` | Failed with undefined `ResolvePath`, `Get`, `Set`, `MaskSecrets` |
| GREEN | Task 1: Accessor & Mask | `go test -v -race ./internal/config` | Passed all 25 subtests in `internal/config` |
| RED | Task 2: Atomic Save | `go test ./internal/config` | Failed with undefined `config.Save` |
| GREEN | Task 2: Atomic Save | `go test -v -race ./internal/config` | Passed `TestSave` verifying `0600` file mode, `0700` dir mode, and atomic rename |
| RED | Task 3: Config CLI Router | `go test ./cmd/agis` | Failed with undefined `RunConfigCLI` |
| GREEN | Task 3: Config CLI Router | `go test -v -race ./cmd/agis -run TestRunConfigCLI` | Passed all 20 subtests for show/get/set/path, -json, -reveal, exit codes, streams |
| REFACTOR | Task 4: Main & Docs Wiring | `go test -race ./...` & `go vet ./...` | 100% test pass rate across all 21 packages with zero vet warnings |

---

## Files Changed
- `internal/config/config.go`: Exported `ResolvePath(flagPath string) string`.
- `internal/config/config_test.go`: Updated test calls to `ResolvePath`.
- `internal/config/accessor.go`: Implemented reflection-based case-insensitive dot-notation `Get` and `Set` with strict type validation (`bool`, `int`, `time.Duration`, `string`, `[]string`).
- `internal/config/mask.go`: Implemented `MaskSecrets` with YAML deep-cloning and credential masking (`llm.api_key`, `gateway.telegram.token`, `gateway.discord.token`, `webhook.secret`).
- `internal/config/accessor_test.go`: Unit tests for `ResolvePath`, `Get`, `Set`, and `MaskSecrets`.
- `internal/config/save.go`: Implemented atomic `Save` with `0700` parent dir creation, temp file sync, and strict `0600` permissions.
- `internal/config/save_test.go`: Unit tests for `Save` permission enforcement and atomicity.
- `cmd/agis/config.go`: Implemented `RunConfigCLI` routing for `show`, `get`, `set`, and `path`.
- `cmd/agis/config_test.go`: Integration tests for `RunConfigCLI` flags, exit codes, and stream separation.
- `cmd/agis/main.go`: Registered `case "config": os.Exit(RunConfigCLI(...))` in the root CLI router.
- `docs/cli.md`: Added section 11 documenting `agis config` subcommands, flags, and examples.
- `README.md`: Added `agis config` to the CLI subcommands overview.
- `openspec/changes/config-cli/tasks.md`: Marked all implementation tasks complete.

---

## Test Commands Run
- `go test -race ./internal/config`
- `go test -v -race ./cmd/agis -run TestRunConfigCLI`
- `go test -race ./...`
- `go vet ./...`
- `go test -count=1 ./...`

---

## Deviations from Design
None. All components, signatures, exit codes, and stream disciplines strictly match `design.md` and `spec.md`.

---

## Remaining Tasks (Parent Lifecycle)
- [ ] Start or reuse bounded review. <!-- sdd-owner: parent -->
