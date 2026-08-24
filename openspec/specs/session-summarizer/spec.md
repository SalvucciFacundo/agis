# Session Summarizer Spec

## Purpose

Compress a finished session into a stored summary plus final observations with a single bounded LLM call at close time.

## Requirements

session-summarizer (NEW)

### Requirement: Combined close call
At close, ONE `Chat` MUST return `{summary, observations[]}`. `UpdateConversationSummary` MUST NOT bump `conversations.updated_at`.

#### Scenario: One call produces both
- WHEN CloseSession → `UpdateConversationSummary` + `SaveObservations` called

#### Scenario: Ordering preserved
- WHEN UpdateConversationSummary → `updated_at` unchanged

### Requirement: Close timeout
Close MUST be synchronous, bounded by `close_timeout` (default 30s). Errors non-fatal.

#### Scenario: Timeout
- GIVEN provider hangs → ctx cancels at 30s, logged, exit continues

---
