# Minimal TUI Spec

## Purpose

Provide the v1 surface: a Bubbletea terminal app that renders a viewport, text input, and spinner, and streams the agent's response into view.

## Requirements

### Requirement: Minimal Bubbletea TUI
TUI MUST render viewport, text input, and spinner. Enter sends input to `Brain.Step`, streams tokens into the viewport, and restores the latest conversation on startup.

#### Scenario: Send a message
- GIVEN the app launches
- WHEN the user types "Hello" and presses Enter
- THEN the message is persisted and streamed response appears.


minimal-tui (MODIFIED)

### Requirement: Close hook
CtrlC/Esc MUST call `CloseSession` with status line. Synchronous, bounded by `close_timeout`. Streaming: 1st CtrlC cancels stream; 2nd quits immediately.
(Previously: quit immediately, no close hook.)

#### Scenario: Idle quit
- CtrlC → "closing…" → CloseSession → quit

#### Scenario: Streaming cancel
- CtrlC → cancel stream → drain → close

#### Scenario: Force quit
- CtrlC×2 → immediate quit, no close


minimal-tui (MODIFIED)

### Requirement: Slash-command dispatch
Input beginning with `/` MUST dispatch exact-match commands locally and MUST NOT reach the provider or persist as a message. Required commands: `/personality <name|none>`, `/persona freeze|reset|status`. Unknown commands MUST print an error line without changing state.

#### Scenario: Command handled locally
- GIVEN `/persona status`
- THEN status renders in the viewport and no provider call occurs

#### Scenario: Unknown command
- GIVEN `/foo`
- THEN an error line appears; conversation unchanged
