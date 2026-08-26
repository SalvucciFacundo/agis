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


brain-loop (MODIFIED)

### Requirement: Context assembly slots
Step MUST assemble system messages in this order: composed identity (SOUL + active overlay + evolution layer), matched skills (when any), recall observations (when any). Empty layers MUST be omitted. Identity is loaded once at startup; overlay changes apply from the next turn.

#### Scenario: Full stack
- GIVEN identity, one matched skill, and recall observations
- THEN three system messages precede the conversation tail in that order

#### Scenario: Bare minimum
- GIVEN no skills and no observations
- THEN only the identity system message precedes the tail

### Requirement: Close-time extraction hook
CloseSession MUST run skill extraction after the summarizer when enabled, bounded by the same close timeout. Extraction failures MUST log-and-continue; successful creations MUST record a `skill` session event.

#### Scenario: Extractor error non-fatal
- GIVEN the extraction LLM call fails
- THEN close completes and quit proceeds


brain-loop (MODIFIED)

### Requirement: Tool calls execute under guard
Step MUST route every model-initiated tool request through the PolicyGuard port before any execution; the model's own output MUST be incapable of mutating policy state.

#### Scenario: Model cannot self-grant
- GIVEN a model reply attempting to change policy
- THEN no policy mutation occurs and the attempt is inert data
