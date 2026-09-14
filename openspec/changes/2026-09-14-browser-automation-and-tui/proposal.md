# Change Proposal: Browser Automation Headless & TUI Interface Expansion

## 1. Problem Statement & Motivation
AGIS currently features fast static web retrieval via `web_search` and `web_fetch`. However, modern Single Page Applications (SPAs) and dynamic sites rely on client-side JavaScript rendering, user interactions (clicks, form inputs, scrolling), and screenshots. Phase 12 of the Hermes parity roadmap specifies headless browser automation to unlock full web interaction for the agent.

Simultaneously, the interactive Terminal User Interface (TUI) in `internal/adapters/tui` lacks slash command handlers and visual indicators for recently shipped subsystems:
- There is no `/help` command (typing `/help` yields `unknown command: /help`).
- Multi-profile management (`/profile list`, `/profile use`, `/profile show`), skills inspection (`/skills`), active tools enumeration (`/tools`), MCP server status (`/mcp`), system diagnostics (`/doctor`), and browser inspection (`/browser`) are only accessible via external CLI, leaving the interactive TUI disconnected from these capabilities.
- The input prompt does not reflect the currently active profile context (e.g. `[default] > `).

## 2. Proposed Solution
1. **Headless Browser Automation (`internal/tools/browser`)**:
   - Create browser automation engine implementing `core.ToolRunner` for:
     - `browser_navigate`: Loads URL in headless Chromium/Brave/Chrome via DevTools Protocol (CDP) or browser process, awaits DOM/network-idle, and extracts document title and content.
     - `browser_action`: Supports interactive actions (`click`, `type`, `evaluate`, `scroll`).
     - `browser_screenshot`: Captures viewport or full-page PNG screenshots.
     - `browser_close`: Closes active page/session.
   - Configure via `config.ToolsConfig.Browser` (`tools.browser` in `config.yaml`).
2. **TUI Interface Expansion (`internal/adapters/tui`)**:
   - Implement comprehensive `/help` slash command.
   - Implement `/profile` commands (`list`, `use`, `show`) allowing seamless profile switching from within the TUI session.
   - Implement `/skills` commands (`list`, `show`) to inspect loaded skill triggers and instructions.
   - Implement `/tools` command to list active runners and registered tools.
   - Implement `/mcp` command to inspect MCP servers, standby status, and tools.
   - Implement `/doctor` command to run in-session health checks.
   - Implement `/browser` command for browser engine diagnostics and manual page inspection.
   - Update TUI prompt/header to display the active profile indicator (e.g. `[default] > `).

## 3. Risks & Tradeoffs
- **Browser Dependencies**: Not all environments have Chromium installed. We detect available binaries (`chromium`, `google-chrome`, `brave`, flatpak) and gracefully degrade if unavailable, keeping tools inert without crashing.
- **TUI State Integrity**: Switching profiles via `/profile use` updates disk state; in-flight sessions remain safe because conversations are scoped by ID.
