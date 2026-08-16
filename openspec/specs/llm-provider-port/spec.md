# LLM Provider Port Spec

## Purpose

Abstract LLM access behind a single `Provider` port so the core never depends on a specific vendor. M1 ships OpenAI and Ollama adapters over one shared OpenAI-compatible client.

## Requirements

### Requirement: Provider port and adapters
`Provider` port MUST expose `Chat`, `Stream`, and `Models`. `Stream` MUST return `(<-chan StreamEvent, error)` with `StreamEvent{Text, Err}`. M1 adapters MUST be OpenAI and Ollama via a shared OpenAI-compatible client.
(Previously: spec §2 defined `Stream(ctx, ChatRequest) (<-chan Token, error)`.)

#### Scenario: Stream emits text events
- GIVEN a fake provider streams "hello" then "world"
- WHEN `Stream` is called
- THEN the channel yields both tokens and closes.

#### Scenario: Stream surfaces mid-stream errors
- GIVEN a provider fails mid-stream
- WHEN `Stream` is called
- THEN the channel yields a `StreamEvent{Err: ...}` and closes.

### Requirement: Static Models list
`Models()` MUST return the static model from `llm.model` in M1. Live enumeration is deferred to M4.

#### Scenario: Models returns configured entry
- GIVEN `llm.provider: openai` and `llm.model: gpt-4o-mini`
- WHEN `Models()` is called
- THEN it returns one `ModelInfo` with the configured values.
