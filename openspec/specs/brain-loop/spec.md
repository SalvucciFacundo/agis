# Brain Loop Spec

## Purpose

Drive the central agent loop: persist input, load context tail, stream an LLM response, and persist the assistant reply.

## Requirements

### Requirement: Brain.Step loop
`Brain.Step(ctx, input)` MUST persist the user message, load the tail, call `Provider.Stream`, forward tokens to a sink, and persist the assistant message. Tool calls are logged and ignored in M1.

#### Scenario: Step streams and persists
- GIVEN a fake provider streams "Hi"
- WHEN `Step` is called with "Hello"
- THEN both messages are persisted and the sink receives "Hi".

#### Scenario: Step handles provider errors
- GIVEN `Stream` returns an error
- WHEN `Step` is called
- THEN the error is returned, the user message is persisted, and no assistant message is written.


brain-loop (MODIFIED)

### Requirement: Recall injection
`Step` MUST load top-N observations (`recall_limit`, default 10) into system prompt.
(Previously: no recall — write-only memory.)

#### Scenario: Observations prepended
- GIVEN 5 obs → Step includes them in provider request

### Requirement: CloseSession
MUST: (1) load msgs, (2) Chat→summary+obs, (3) UpdateConversationSummary, (4) SaveObservations, (5) UpsertUserModel. Respects ctx deadline.

#### Scenario: Order
- GIVEN fakes → calls: summary→obs→user_model

#### Scenario: Stream cancel
- GIVEN streaming Step, ctx canceled → stream drains, CloseSession runs

---
