# TUI Commands & Hotkeys Reference

This document provides a comprehensive reference for all interactive slash commands, keyboard shortcuts, and control panels available in the AGIS terminal user interface (TUI).

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
