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


minimal-tui (MODIFIED)

### Requirement: Interactive approval
When the guard returns ask during a turn, the TUI MUST show the exact action with four choices — allow once, allow for session, always allow, deny — mapped to fixed keys, with deny as the safe default on interrupt.

#### Scenario: Interrupting a prompt denies
- GIVEN a visible approval prompt
- WHEN CtrlC pressed
- THEN the action is denied and audited

### Requirement: Permissions panel
`/permisos` MUST open a panel listing rules grouped by category, offering allow/deny toggles, per-backend posture display, a decision preview for a typed command, audit-log view, and revocation of always grants.

#### Scenario: Revoke an always grant
- GIVEN an always rule visible in the panel
- WHEN revoked
- THEN policy.yaml loses the rule and the audit records the revocation


## MODIFIED Requirements

### Requirement: Session slash commands
Input beginning with `/` that matches `/new`, `/reset`, `/save`, `/list`, `/restore`, `/compress`, `/snapshot`, `/rename` MUST dispatch locally and MUST NOT reach the provider nor persist as a message. Unknown slash MUST print an error line. All session commands MUST be gated with `streaming || closing` check.

#### Scenario: Unknown slash
- GIVEN `/unknown`
- THEN error line appears and no provider call occurs

#### Scenario: Commands gated while streaming
- GIVEN streaming true
- WHEN `/new` is invoked
- THEN it is ignored

### Requirement: Session feedback and views
`/list` MUST render id, title, created_at from `ListConversations`. `/restore` MUST load summary + tail into viewport. `/save` MUST trigger an explicit persist without quitting. Feedback lines MUST use `commandFeedbackPrefix`.

#### Scenario: Save feedback
- GIVEN `/save`
- THEN viewport shows `· saved` and no new conversation is created
