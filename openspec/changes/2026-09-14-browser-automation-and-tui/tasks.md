# Tasks: Browser Automation Headless & TUI Interface Expansion

## Review Workload Forecast
- Estimated changed lines: ~500 additions, ~30 deletions
- 400-line budget risk: Medium
- Chained PRs recommended: No (Single cohesive architectural unit)
- Delivery strategy: Single PR stacked to main
- TDD mode: Strict TDD (RED -> GREEN -> REFACTOR)

---

## Work Units

### Work Unit 1: Browser Configuration Schema (`internal/config`)
- [x] Add `BrowserConfig` struct to `internal/config/config.go`.
- [x] Add `Browser` field to `ToolsConfig`.
- [x] Implement default population and validation logic in `applyDefaults`.
- [x] Add unit tests in `internal/config/config_test.go`.
- [x] Verify test pass: `go test -race ./internal/config/...`.

### Work Unit 2: Headless Browser Runner & Tools (`internal/tools/browser`)
- [x] Create `internal/tools/browser/driver.go` (interfaces and browser detection).
- [x] Create `internal/tools/browser/runner.go` (`browser_navigate`, `browser_action`, `browser_screenshot`, `browser_close`).
- [x] Write comprehensive unit tests in `internal/tools/browser/browser_test.go` with mock drivers and real process checks.
- [x] Wire browser runner in `internal/tools/registry.go`.
- [x] Verify test pass: `go test -race ./internal/tools/...`.

### Work Unit 3: TUI Redesign & Slash Commands Expansion (`internal/adapters/tui`)
- [x] Implement Gaia/Hermes status header: `[profile: <name>] · [model: <name>] · [ctx: X% / Yk] · [mcp: N]`.
- [x] Add context window percentage estimation based on session token count and model context limits.
- [x] Implement role badges: `❯ You`, `✦ AGIS`, `⚙ [tool: <name>]`, `✖ [error]`.
- [x] Implement bordered input container with prompt `[<profile>] ❯ ` and shortcut footer.
- [x] Implement `/help` with categorized command summary.
- [x] Implement `/profile` (`list`, `use`, `show`).
- [x] Implement `/skills` (`list`, `show`).
- [x] Implement `/tools` (lists registered tools).
- [x] Implement `/mcp` (lists MCP servers & tools).
- [x] Implement `/doctor` (runs diagnostics inside TUI).
- [x] Implement `/browser` (status & test navigation).
- [x] Write unit tests in `internal/adapters/tui/commands_test.go` and `app_test.go`.
- [x] Verify test pass: `go test -race ./internal/adapters/tui/...`.

### Work Unit 4: Verification, Documentation & Roadmap
- [x] Update `docs/development/hermes-parity-roadmap.md` (mark Fase 12 as ✅ DONE).
- [x] Update `docs/tui-commands.md`.
- [x] Run full regression suite: `go test -race ./...` and `go vet ./...`.
- [x] Commit with conventional commits and push.
