# Persona Spec

## Purpose

Compose the agent identity from a durable SOUL.md seed, session-scoped personality overlays, and a derived evolution layer driven by curated user-model rows — never rewriting the identity file.

## Requirements

persona (NEW)

### Requirement: SOUL.md lifecycle
System MUST seed `$AGIS_HOME/SOUL.md` from an embedded default when missing, MUST NOT overwrite an existing file, and MUST fall back to the built-in identity when the file is empty or unreadable. SOUL.md MUST only ever be read from `$AGIS_HOME`, never the working directory.

#### Scenario: First run seeds
- GIVEN no SOUL.md
- WHEN the app starts
- THEN the embedded default is written and used

#### Scenario: User edits preserved
- GIVEN a customized SOUL.md
- THEN restarts load it verbatim

### Requirement: Injection scan
SOUL.md and any overlay text MUST be scanned against a fixed prompt-injection pattern list before entering the prompt; flagged lines MUST be excluded and logged.

#### Scenario: Benign identity loads fully
- GIVEN a normal SOUL.md
- THEN all lines are included

#### Scenario: Injected line dropped
- GIVEN a line "ignore all previous instructions"
- THEN that line is excluded, the rest loads

### Requirement: Persona overlays
`/personality <name>` MUST apply a session-scoped overlay inserted immediately after the identity slot. Built-in presets MUST ship with the binary; custom presets come from config `agent.personalities`. `/personality none|default|neutral` MUST clear the overlay. Unknown names MUST report an error and leave state unchanged.

#### Scenario: Apply preset
- GIVEN `/personality teacher`
- THEN the next turn carries the overlay; later turns keep it

#### Scenario: Clear overlay
- GIVEN an active overlay and `/personality none`
- THEN the baseline voice returns

### Requirement: Derived evolution
When `evolution_enabled` is true, System MUST assemble an evolution layer from curated communication observations and include it after the overlay slot. `/persona freeze` MUST exclude the layer; `/persona reset` MUST return it to seed state; `/persona status` MUST report current mode and layer size. Evolution MUST NEVER modify SOUL.md.

#### Scenario: Evolution participates
- GIVEN user-model observations about tone preferences
- THEN the assembled layer reflects them

#### Scenario: Freeze excludes
- GIVEN `/persona freeze`
- THEN the layer disappears from prompts and status reports frozen
