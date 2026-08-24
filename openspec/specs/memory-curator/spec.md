# Memory Curator Spec

## Purpose

Curate durable observations from conversation turns with one LLM call per nudge, persisting them for later recall.

## Requirements

memory-curator (NEW)

### Requirement: LLM-driven curation
System MUST use ONE `Provider.Chat` call per nudge/close returning JSON `{topic_key, type, content, importance}`. Parse failures MUST log+skip, never fail the turn. Fence stripping MUST precede parsing.

#### Scenario: Valid JSON
- GIVEN provider returns `[{topic_key:"a",importance:4}]`
- WHEN parsed → `SaveObservations` called, importance=4

#### Scenario: Malformed response
- GIVEN provider returns prose
- WHEN parsed → error logged, zero rows, turn continues

#### Scenario: Missing importance
- GIVEN JSON without importance → defaults to 3

### Requirement: Nudge cadence
MUST trigger curator every `nudge_every` assistant msgs (default 10). Each nudge writes `session_events(kind='nudge')`. `nudge_every:0` disables.

#### Scenario: Boundary trigger
- GIVEN 10 assistant msgs, `nudge_every=10`
- WHEN 11th Step begins → curator runs, nudge row written

### Requirement: Kill switch
`memory.learning_enabled:false` MUST suppress all curator/summarizer Chat calls.

#### Scenario: Disabled → zero LLM calls, immediate exit

---
