# Specification: Headless Browser Automation & Gaia/Hermes TUI Interface

## Scope & Purpose
Defines requirements, interfaces, and behaviors for Phase 12 Browser Automation tools (`browser_navigate`, `browser_action`, `browser_screenshot`, `browser_close`) and the expanded Gaia/Hermes interactive TUI (`internal/adapters/tui`).

---

## 1. Browser Automation Tools (`internal/tools/browser`)

### `AGIS-BRW-001`: Browser Configuration Schema
- **Requirement**: Struct `BrowserConfig` in `internal/config` MUST support:
  - `enabled: bool` (default: false)
  - `headless: bool` (default: true)
  - `executable_path: string` (optional override; auto-detects chromium, google-chrome, brave, or flatpak brave)
  - `timeout: time.Duration` (default: 30s)
  - `viewport_width: int` (default: 1280)
  - `viewport_height: int` (default: 720)
  - `user_data_dir: string` (optional path)

### `AGIS-BRW-002`: Browser Driver & Tool Runners
- **Requirement**: Package `internal/tools/browser` MUST provide `Driver` interface and `CLIDriver` implementation.
- **Requirement**: `NewBrowserRunners(cfg, driver)` MUST declare:
  - `browser_navigate`: accepts `{"url": "https://..."}`, renders JavaScript, extracts title, and converts DOM to Markdown using AST parser.
  - `browser_action`: accepts `{"action": "click"|"type"|"evaluate"|"scroll", "selector": "...", "text": "...", "script": "..."}`.
  - `browser_screenshot`: accepts `{"url": "...", "path": "..."}`, saves a PNG screenshot to disk, and returns the filepath.
  - `browser_close`: closes active browser session and releases resources.
- **Requirement**: If browser executable is not found or fails to start, `BrowserRunner` MUST return a clear descriptive error without crashing.

---

## 2. TUI Interface Expansion (`internal/adapters/tui`)

### `AGIS-TUI-003`: Slash Command `/help`
- **Requirement**: The TUI MUST handle `/help` and `/?` locally by displaying categorized help for all supported slash commands:
  - Session management: `/new`, `/reset`, `/save`, `/list`, `/restore`, `/compress`, `/snapshot`, `/rename`
  - Agent & Identity: `/personality`, `/persona`
  - Subsystems: `/permisos`, `/profile`, `/skills`, `/tools`, `/mcp`, `/doctor`, `/browser`

### `AGIS-TUI-004`: Slash Command `/profile`
- **Requirement**: The TUI MUST handle `/profile`:
  - `/profile` (or `/profile show`): displays active profile name and home directory.
  - `/profile list`: displays all discovered profiles with active marker `*`.
  - `/profile use <name>` (or `/profile switch <name>`): switches the active profile pointer and updates the prompt.

### `AGIS-TUI-005`: Slash Command `/skills`
- **Requirement**: The TUI MUST handle `/skills`:
  - `/skills` (or `/skills list`): lists installed skill names and descriptions.
  - `/skills show <name>`: displays formatted description and triggers of the specified skill.

### `AGIS-TUI-006`: Slash Command `/tools`
- **Requirement**: The TUI MUST handle `/tools`:
  - Lists all currently registered and active tools (local, browser, web, mcp, docker, ssh).

### `AGIS-TUI-007`: Slash Command `/mcp`
- **Requirement**: The TUI MUST handle `/mcp`:
  - Lists configured MCP servers and connected active count.

### `AGIS-TUI-008`: Slash Command `/doctor`
- **Requirement**: The TUI MUST handle `/doctor`:
  - Runs environment and health diagnostic checks (profile, model, mcp, tools, browser engine, DB).

### `AGIS-TUI-009`: Slash Command `/browser`
- **Requirement**: The TUI MUST handle `/browser`:
  - `/browser` (or `/browser status`): displays browser engine availability, detected executable path, and configuration.
  - `/browser open <url>`: executes a test headless navigation to `<url>` and prints title and snippet.

### `AGIS-TUI-010`: Active Profile Prompt Indicator
- **Requirement**: The TUI text input prompt MUST display the active profile, e.g. `[default] ❯ ` or `[work] ❯ `, updating dynamically when `/profile use` is executed.

### `AGIS-TUI-011`: Gaia/Hermes Status Header & Role Badges
- **Requirement**: The TUI MUST render a top status header containing:
  - Active profile name: `[profile: <name>]`
  - Active model name: `[model: <name>]`
  - Context window usage: `[ctx: <pct>% / <limit>k]`
  - Connected MCP servers count: `[mcp: <count>]`
- **Requirement**: Conversation messages MUST feature Gaia/Hermes styled role indicators:
  - User: `❯ you: `
  - Assistant: `✦ assistant: `
  - Error: `✖ error: `
- **Requirement**: Input area MUST have a clean container with shortcut hints footer (`[Ctrl+C: quit] · [Enter: send] · [/help: commands]`).
