# Tool Calling Spec

## Purpose

Streaming tool-call wire format and bounded brain loop. ChatRequest advertises tools, StreamEvent carries optional ToolCall, and Brain.Step loops evaluate-execute-feed-back up to a hard cap.

## Requirements

tool-calling (NEW)

### Requirement: Wire format
ChatRequest MUST carry the advertised tools (name, description). StreamEvent MUST carry an optional ToolCall (name, arguments). Zero-value events MUST remain plain text tokens: existing flows MUST be unaffected.

#### Scenario: Plain reply unchanged
- GIVEN no tools advertised
- THEN streamed events are text-only as before

### Requirement: Bounded tool loop
On a ToolCall event, Step MUST evaluate the call through the guard: allow executes on the backend and feeds the output back as a RoleTool message; deny informs the model the action was blocked; ask presents the interactive approval in the TUI (auto-deny surfaces later). The loop MUST stop after 8 rounds, informing the model, and every round MUST be audited.

#### Scenario: Approved command feeds back
- GIVEN an ask approved as once for `git status`
- THEN the command runs, its output reaches the model, and the reply continues

#### Scenario: Runaway loop capped
- GIVEN a model requesting tools for 8 consecutive rounds
- THEN round 9 does not execute and the model is told to answer directly
