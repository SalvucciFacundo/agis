# Specification: Browser Automation Headless & TUI Interface Expansion

## Scope & Purpose
Defines requirements and behaviors for Phase 12 Browser Automation tools and the expanded TUI slash command subsystem.

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

### `AGIS-BRW-002`: Browser Runner & Tools
- **Requirement**: Package `internal/tools/browser` MUST provide `BrowserRunner` implementing `core.ToolRunner` with backend identifier `"browser"`.
- **Requirement**: `BrowserRunner` MUST declare:
  - `browser_navigate`: accepts `{"url": "https://..."}`, returns document title, final URL, and text/markdown content.
  - `browser_action`: accepts `{"action": "click"|"type"|"evaluate"|"scroll", "selector": "...", "text": "...", "script": "..."}`.
  - `browser_screenshot`: accepts `{"path": "..."}`, saves a PNG screenshot to disk, and returns the filepath.
  - `browser_close`: closes active browser session.
- **Requirement**: If browser executable is not found or fails to start, `BrowserRunner` MUST return clear descriptive error without crashing.

---

## 2. TUI Interface Expansion (`internal/adapters/tui`)

### `AGIS-TUI-003`: Slash Command `/help`
- **Requirement**: The TUI MUST handle `/help` and `/?` locally by displaying categorized help for all supported slash commands:
  - Session management: `/new`, `/reset`, `/save`, `/list`, `/restore`, `/compress`, `/snapshot`, `/rename`
  - Agent & Identity: `/personality`, `/persona`
  - Subsystems: `/permisos`, `/profile`, `/skills`, `/tools`, `/mcp`, `/doctor`, `/browser`

### `AGIS-TUI-004`: Slash Command `/profile`
- **Requirement**: The TUI MUST handle `/profile`:
  - `/profile` (or `/profile show`): displays active profile name, configuration directory, and database path.
  - `/profile list`: displays all discovered profiles with active marker `*`.
  - `/profile use <name>` (or `/profile switch <name>`): switches the active profile pointer and updates the prompt.

### `AGIS-TUI-005`: Slash Command `/skills`
- **Requirement**: The TUI MUST handle `/skills`:
  - `/skills` (or `/skills list`): lists installed skill names and trigger words.
  - `/skills show <name>`: displays formatted description and rules of the specified skill.

### `AGIS-TUI-006`: Slash Command `/tools`
- **Requirement**: The TUI MUST handle `/tools`:
  - Lists all currently registered and active tools (local, docker, ssh, web_search, web_fetch, browser_*, etc.).

### `AGIS-TUI-007`: Slash Command `/mcp`
- **Requirement**: The TUI MUST handle `/mcp`:
  - Lists configured MCP servers, standby status, and registered remote tools.

### `AGIS-TUI-008`: Slash Command `/doctor`
- **Requirement**: The TUI MUST handle `/doctor`:
  - Runs environment and health diagnostic checks and renders the tabular check results directly in the viewport.

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
  - User: `❯ You`
  - Assistant: `✦ AGIS`
  - Tool calls: `⚙ [tool: <name>]`
  - Errors: `✖ [error]`
- **Requirement**: Input area MUST have a clean bordered container with shortcut hints footer (`Ctrl+C: exit`, `Enter: send`, `/help: commands`).
