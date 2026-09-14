# TUI Commands & Hotkeys Reference

This document provides a comprehensive reference for all interactive slash commands, keyboard shortcuts, and control panels available in the AGIS terminal user interface (TUI).

---

## 🖥️ Gaia / Hermes Interface & Status Header

AGIS features a real-time status header at the top of the terminal:

```text
[profile: default] · [model: claude-3-7-sonnet] · [ctx: 14% / 200k] · [mcp: 2]
──────────────────────────────────────────────────────────────────────────────
```

- **`profile`**: Displays the active agent profile identity. The prompt input matches: `[<profile>] ❯ `.
- **`model`**: Displays the active primary LLM model.
- **`ctx`**: Displays the real-time context window usage percentage based on conversation history tokens versus model limit.
- **`mcp`**: Displays the count of active connected Model Context Protocol servers.

### Role Badges
- **User**: `❯ you: <message>`
- **Assistant**: `✦ assistant: <reply>`
- **Error**: `✖ error: <message>`

---

## 🛠️ Subsystem Slash Commands

| Command | Description | Example |
|---|---|---|
| `/help` or `/?` | Displays the complete categorized manual of all available commands. | `/help` |
| `/profile` or `/profile show` | Displays active profile name and configuration directories. | `/profile show` |
| `/profile list` | Lists all discovered agent profiles with an active indicator `*`. | `/profile list` |
| `/profile use <name>` | Switches active profile and updates prompt indicator immediately. | `/profile use coder` |
| `/skills` or `/skills list` | Lists all loaded skills and their descriptions. | `/skills` |
| `/skills show <name>` | Displays details, description, and triggers for a specific skill. | `/skills show go-testing` |
| `/tools` | Lists all active registered tools (local, browser, web, mcp, docker, ssh). | `/tools` |
| `/mcp` | Displays connected MCP server count and operational status. | `/mcp` |
| `/doctor` | Runs system diagnostics (profile, model, tools, browser engine, DB). | `/doctor` |
| `/browser` or `/browser status` | Displays detected browser binary, headless mode, and viewport settings. | `/browser status` |
| `/browser open <url>` | Executes test headless navigation and displays page title and content snippet. | `/browser open https://example.com` |

---

## 💬 Session Management Slash Commands

Commands are entered directly into the TUI input box starting with a forward slash (`/`). Slash commands execute immediately and do not consume LLM tokens.

| Command | Description | Example |
|---|---|---|
| `/new` or `/reset` | Creates a brand-new clean conversation session and resets turn state. Past conversations remain saved in SQLite. | `/new` |
| `/save` | Saves the current active conversation to SQLite and confirms the session ID. | `/save` |
| `/list` | Displays the most recent saved conversations with their session IDs, creation timestamps, and message counts. | `/list` |
| `/restore <id>` | Switches context and restores a previous conversation session by its ID. Reloads conversation history into view. | `/restore conv_8f2b1a` |
| `/rename <title>` | Renames the current conversation title in the database. Injection patterns are automatically stripped. | `/rename Project Refactor` |
| `/snapshot` | Takes an immutable point-in-time snapshot copy of the active conversation and stores it in the `snapshots` table. | `/snapshot` |
| `/compress` | Forces early context compaction by triggering the session summarizer on the current conversation history. | `/compress` |

---

## 🎭 Persona & Identity Slash Commands

| Command | Description | Example |
|---|---|---|
| `/personality <preset>` | Applies a temporary personality overlay for the current session. Built-in presets: `concise`, `teacher`, `technical`, `creative`, or any custom preset in `config.yaml`. | `/personality teacher` |
| `/personality none` | Clears the active personality overlay and returns to the default `SOUL.md` voice. | `/personality none` |
| `/persona status` | Displays the current identity status, loaded `SOUL.md` details, active personality overlay, and evolution state. | `/persona status` |
| `/persona freeze` | Freezes the dynamic persona layer, preventing automated user-model guided evolution. | `/persona freeze` |
| `/persona reset` | Resets the dynamic evolution layer to the base `SOUL.md` defaults. | `/persona reset` |

---

## 🛡️ Security & Permissions: `/permisos` Panel

Typing `/permisos` opens the interactive security policy control panel:

```text
┌────────────────────────────────────────────────────────┐
│               AGIS POLICY & PERMISSIONS                │
│                                                        │
│ Baseline Tier: [ SANDBOX ] (standard / full available) │
│                                                        │
│ Active Rules (3):                                      │
│   [ALLOW] git pull (local)                             │
│   [ALLOW] docker:alpine:3 (docker)                     │
│   [DENY]  rm -rf / (all)                               │
│                                                        │
│ Recent Decision Audit Tail:                            │
│   14:23:01 | git status  | ALLOW | Tier Policy         │
│   14:23:05 | rm -rf data | DENY  | Explicit Deny Rule  │
│                                                        │
│ Keys: [Space] Toggle Rule  [r] Revoke  [q/Esc] Close   │
└────────────────────────────────────────────────────────┘
```

### Controls inside `/permisos`:
- `↑` / `↓`: Navigate rules list.
- `Space`: Toggle rule between `ALLOW` and `DENY`.
- `r`: Revoke/delete the selected persistent rule.
- `q` or `Esc`: Close the `/permisos` panel and return to chat.

---

## ⚡ Real-Time Tool Approval Prompts

When a tool execution requires human approval (under `sandbox` or `standard` tier with no prior `always` rule), AGIS pauses execution and prompts in-line:

```text
⚠️ Tool Execution Requested:
   Backend: local
   Command: git push origin main

   [a] Allow Once
   [s] Allow for Session
   [l] Always Allow (persists in policy.yaml)
   [n] Deny (or press Ctrl+C)
```

- Pressing `a`: Executes the command once; future identical commands will prompt again.
- Pressing `s`: Grants temporary in-memory permission for the rest of the current TUI session.
- Pressing `l`: Adds an immutable rule to `$AGIS_HOME/policy.yaml` so the command never prompts again.
- Pressing `n` or `Ctrl+C`: Rejects the tool call with `DecisionDeny`. The denial is logged in the audit trail and returned to the model as `"Action blocked by policy"`.

---

## ⌨️ Keyboard Shortcuts & Navigation

| Key | Action | Description |
|---|---|---|
| `Enter` | Submit | Sends the message in the input box to `Brain.Step` for processing. |
| `Ctrl + C` (while streaming) | Cancel Turn | Cancels the active token stream, drains the partial response, and frees the input box. |
| `Ctrl + C` (idle) | Quit Sequence | Triggers graceful session closure (`CloseSession`), writes summaries/observations, and exits. |
| `Ctrl + C` × 2 | Force Quit | Immediately terminates the application without waiting for session summarization. |
| `Esc` | Dismiss / Unfocus | Closes modal panels (`/permisos`), cancels in-flight prompts, or unfocuses input. |
| `Page Up` / `Page Down` | Scroll Viewport | Scrolls conversation history up and down. |
| `Mouse Wheel` | Scroll Viewport | Native mouse wheel scrolling in supported terminals. |
| `Ctrl + L` | Redraw | Redraws the terminal viewport cleanly if corrupted by external output. |
