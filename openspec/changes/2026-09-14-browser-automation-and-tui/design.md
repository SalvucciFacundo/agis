# Design: Browser Automation Headless & TUI Interface Expansion

## 1. Headless Browser Architecture (`internal/tools/browser`)

### Driver & Process Strategy
We design a modular browser runner:
```
+-------------------------------------------------------------+
|                 core.ToolRunner ("browser")                 |
|                                                             |
|   browser_navigate    browser_action    browser_screenshot  |
+-------------------------------------------------------------+
                              |
                              v
+-------------------------------------------------------------+
|                    BrowserDriver Interface                  |
|   +Navigate(ctx, url) (*PageResult, error)                  |
|   +Action(ctx, action, selector, text, script) error        |
|   +Screenshot(ctx, path) (string, error)                    |
|   +Close() error                                            |
+-------------------------------------------------------------+
             |                                    |
             v                                    v
+------------------------+            +-----------------------+
| ChromiumDriver (CDP)   |            | ExecDriver (CLI /     |
| Direct DevTools Pipe/WS|            | Headless dump / agent)|
+------------------------+            +-----------------------+
```

### Path Resolution
The runner auto-detects browser binaries in order:
1. `cfg.ExecutablePath` (if non-empty)
2. `google-chrome`, `google-chrome-stable`, `chromium`, `chromium-browser`, `brave`, `brave-browser`, `microsoft-edge`
3. Flatpak command `flatpak run com.brave.Browser` (detected on current host)
4. If no browser is found, returns an informative error guiding user to install chromium or specify `executable_path`.

### HTML to Text Sanitization
Reuses pure Go HTML parsing (from `internal/tools/web/fetch`) to convert rendered DOM into clean markdown/text without script/style tags.

---

## 2. TUI Interface Design (`internal/adapters/tui`)

### Slash Commands Architecture
In `internal/adapters/tui/app.go`:
```go
func (m *Model) runCommand(input string) (tea.Model, tea.Cmd) {
    fields := strings.Fields(input)
    switch fields[0] {
    case "/help", "/?":
        return m.cmdHelp()
    case "/profile":
        return m.cmdProfile(fields[1:])
    case "/skills", "/skill":
        return m.cmdSkills(fields[1:])
    case "/tools":
        return m.cmdTools()
    case "/mcp":
        return m.cmdMCP()
    case "/doctor":
        return m.cmdDoctor()
    case "/browser":
        return m.cmdBrowser(fields[1:])
    // existing commands:
    case "/personality": ...
    case "/persona": ...
    case "/permisos": ...
    case "/new", "/reset": ...
    ...
    }
}
```

### Profile Indicator & Input Box
- Maintain `m.activeProfile` in `tui.Model`.
- Set prompt indicator: `[<profile>] ❯ ` styled with Lipgloss cyan/magenta.
- Bordered input container with footer shortcuts: `[Ctrl+C: exit] · [Enter: send] · [/help: commands]`.
- When `/profile use <name>` is executed, update `m.activeProfile`, update `config.SetActiveProfile(name)`, and update prompt.

---

## 3. Gaia/Hermes Status Header & Visual Design

### Header Bar Metrics
Rendered at top of viewport on each view refresh:
```
[profile: coder] · [model: claude-3-7-sonnet] · [ctx: 14% / 200k] · [mcp: 3]
----------------------------------------------------------------------------
```
- **Active Profile**: `m.activeProfile` (default: "default")
- **Model Name**: `m.modelName` (from config or active LLM client)
- **Context Percentage**:
  - Model context window map (e.g. `gpt-4o`: 128k, `claude-3-5/7-sonnet`: 200k, `deepseek-chat`: 64k, `o1/o3`: 200k, default: 128k).
  - Used tokens estimated from session messages history: `(estimatedTokens / windowLimit) * 100`.
- **Connected MCP Count**: `len(m.mcpServers)` or active clients count from `mcp.Manager`.

### Role Badges
- User: `lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true).Render("❯ You")`
- Assistant: `lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("✦ AGIS")`
- Tool: `lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("⚙ [tool: " + name + "]")`
- Error: `lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render("✖ [error]")`
