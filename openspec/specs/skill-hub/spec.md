# Skill Hub Spec

## Purpose

Load agentskills.io-compatible skill files, index and match them against user input, track usage, create skills from session experience, and persist a human-readable registry.

## Requirements

skill-hub (NEW)

### Requirement: Directory loading
System MUST load skills as Markdown files with YAML frontmatter (`name` required, `description` required, `trigger` optional) from the configured skills directory. Files failing validation MUST be skipped and logged, never fatal. The directory MUST be scanned at startup and on demand.

#### Scenario: Valid skill loads
- GIVEN `~/.agis/skills/coffee.md` with valid frontmatter
- WHEN the hub indexes
- THEN the skill is queryable by name

#### Scenario: Invalid frontmatter skipped
- GIVEN a file missing `name`
- WHEN the hub indexes
- THEN the file is skipped, a warning is logged, startup continues

### Requirement: Index and matching
The hub MUST index skills by name, trigger, and description, and MUST match them against the current user input using whitespace-split AND term semantics over those fields, returning at most N results (default 3).

#### Scenario: Trigger match
- GIVEN skill with trigger "deploy" and input "how do I deploy this"
- THEN the skill matches

#### Scenario: No match
- GIVEN unrelated input
- THEN zero skills are returned

### Requirement: Persistence and usage
Skills MUST persist to the `skills` table with `source` ∈ {`imported`,`agent`}. Using a skill in context MUST increment `usage_count` and update `last_used`.

#### Scenario: Import persists
- GIVEN a loaded file skill
- WHEN the hub syncs to the repository
- THEN a row with source `imported` exists

#### Scenario: Usage tracked
- GIVEN a skill injected into context
- THEN `usage_count` increases and `last_used` updates

### Requirement: Close-time creation
When enabled, CloseSession MUST run ONE bounded LLM call after the summarizer asking whether the session produced a reusable procedure, returning `{name, description, trigger, content}` or null. Malformed responses MUST log-and-skip; created skills persist with source `agent`; a `skill` session event MUST be recorded. `skills.enabled: false` MUST disable the call entirely.

#### Scenario: Procedure captured
- GIVEN a session that iteratively fixed a recurring problem
- WHEN the session closes
- THEN an agent-sourced skill persists and a `skill` event exists

#### Scenario: Malformed response skips
- GIVEN prose instead of JSON
- THEN nothing persists, close succeeds

### Requirement: Registry file
The hub MUST regenerate `$AGIS_HOME/.atl/skill-registry.md` (path configurable) listing indexed skills after load and creation. Write failures MUST be non-fatal.

#### Scenario: Registry reflects state
- GIVEN imported and agent skills exist
- THEN the registry lists both

#### Scenario: Unwritable path
- GIVEN a read-only registry directory
- THEN operation continues with a warning
