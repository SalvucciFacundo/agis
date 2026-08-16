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
