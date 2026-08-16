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
