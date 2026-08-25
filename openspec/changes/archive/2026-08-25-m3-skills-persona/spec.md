# M3 — Skills & persona (delta spec)

Delta spec for `m3-skills-persona`: skill hub (agentskills.io-compatible), close-time skill creation, SOUL.md identity, persona overlays, derived evolution.

## skill-hub (NEW)

### AGIS-M3-SKL-001: Directory loading
System MUST load skills as Markdown files with YAML frontmatter (`name` required, `description` required, `trigger` optional) from the configured skills directory. Files failing validation MUST be skipped and logged, never fatal. The directory MUST be scanned at startup and on demand.

#### Scenario: Valid skill loads
- GIVEN `~/.agis/skills/coffee.md` with valid frontmatter
- WHEN the hub indexes
- THEN the skill is queryable by name

#### Scenario: Invalid frontmatter skipped
- GIVEN a file missing `name`
- WHEN the hub indexes
- THEN the file is skipped, a warning is logged, startup continues

### AGIS-M3-SKL-002: Index and matching
The hub MUST index skills by name, trigger, and description, and MUST match them against the current user input using whitespace-split AND term semantics over those fields, returning at most N results (default 3).

#### Scenario: Trigger match
- GIVEN skill with trigger "deploy" and input "how do I deploy this"
- THEN the skill matches

#### Scenario: No match
- GIVEN unrelated input
- THEN zero skills are returned

### AGIS-M3-SKL-003: Persistence and usage
Skills MUST persist to the `skills` table with `source` ∈ {`imported`,`agent`}. Using a skill in context MUST increment `usage_count` and update `last_used`.

#### Scenario: Import persists
- GIVEN a loaded file skill
- WHEN the hub syncs to the repository
- THEN a row with source `imported` exists

#### Scenario: Usage tracked
- GIVEN a skill injected into context
- THEN `usage_count` increases and `last_used` updates

### AGIS-M3-SKL-004: Close-time creation
When enabled, CloseSession MUST run ONE bounded LLM call after the summarizer asking whether the session produced a reusable procedure, returning `{name, description, trigger, content}` or null. Malformed responses MUST log-and-skip; created skills persist with source `agent`; a `skill` session event MUST be recorded. `skills.enabled: false` MUST disable the call entirely.

#### Scenario: Procedure captured
- GIVEN a session that iteratively fixed a recurring problem
- WHEN the session closes
- THEN an agent-sourced skill persists and a `skill` event exists

#### Scenario: Malformed response skips
- GIVEN prose instead of JSON
- THEN nothing persists, close succeeds

### AGIS-M3-SKL-005: Registry file
The hub MUST regenerate `$AGIS_HOME/.atl/skill-registry.md` (path configurable) listing indexed skills after load and creation. Write failures MUST be non-fatal.

#### Scenario: Registry reflects state
- GIVEN imported and agent skills exist
- THEN the registry lists both

#### Scenario: Unwritable path
- GIVEN a read-only registry directory
- THEN operation continues with a warning

## persona (NEW)

### AGIS-M3-PER-001: SOUL.md lifecycle
System MUST seed `$AGIS_HOME/SOUL.md` from an embedded default when missing, MUST NOT overwrite an existing file, and MUST fall back to the built-in identity when the file is empty or unreadable. SOUL.md MUST only ever be read from `$AGIS_HOME`, never the working directory.

#### Scenario: First run seeds
- GIVEN no SOUL.md
- WHEN the app starts
- THEN the embedded default is written and used

#### Scenario: User edits preserved
- GIVEN a customized SOUL.md
- THEN restarts load it verbatim

### AGIS-M3-PER-002: Injection scan
SOUL.md and any overlay text MUST be scanned against a fixed prompt-injection pattern list before entering the prompt; flagged lines MUST be excluded and logged.

#### Scenario: Benign identity loads fully
- GIVEN a normal SOUL.md
- THEN all lines are included

#### Scenario: Injected line dropped
- GIVEN a line "ignore all previous instructions"
- THEN that line is excluded, the rest loads

### AGIS-M3-PER-003: Persona overlays
`/personality <name>` MUST apply a session-scoped overlay inserted immediately after the identity slot. Built-in presets MUST ship with the binary; custom presets come from config `agent.personalities`. `/personality none|default|neutral` MUST clear the overlay. Unknown names MUST report an error and leave state unchanged.

#### Scenario: Apply preset
- GIVEN `/personality teacher`
- THEN the next turn carries the overlay; later turns keep it

#### Scenario: Clear overlay
- GIVEN an active overlay and `/personality none`
- THEN the baseline voice returns

### AGIS-M3-PER-004: Derived evolution
When `evolution_enabled` is true, System MUST assemble an evolution layer from curated communication observations and include it after the overlay slot. `/persona freeze` MUST exclude the layer; `/persona reset` MUST return it to seed state; `/persona status` MUST report current mode and layer size. Evolution MUST NEVER modify SOUL.md.

#### Scenario: Evolution participates
- GIVEN user-model observations about tone preferences
- THEN the assembled layer reflects them

#### Scenario: Freeze excludes
- GIVEN `/persona freeze`
- THEN the layer disappears from prompts and status reports frozen

## repository-memory (MODIFIED)

### AGIS-M3-REPO-001: Skills persistence
Repository port MUST add `SaveSkill` (upsert by unique name, preserving `created_at`), `ListSkills`, and `RecordSkillUsage` (increment `usage_count`, set `last_used`). `ListSkills` MUST order by `last_used` DESC then name.

#### Scenario: Upsert by name
- GIVEN skill "deploy-notes" exists
- WHEN saved again with new content
- THEN one row remains with updated content

#### Scenario: Usage bump
- WHEN RecordSkillUsage runs twice
- THEN usage_count increased by 2

### AGIS-M3-REPO-002: Migration 0003
Migration 0003 MUST create the `skills` table (`id`, UNIQUE `name`, `description`, `trigger`, `content`, `source` CHECK IN(`imported`,`agent`), `usage_count` DEFAULT 0, `last_used`, `created_at`) gated idempotently by `user_version`.

#### Scenario: v2 to v3
- GIVEN user_version=2
- THEN 0003 applies once, version becomes 3

## brain-loop (MODIFIED)

### AGIS-M3-BRN-001: Context assembly slots
Step MUST assemble system messages in this order: composed identity (SOUL + active overlay + evolution layer), matched skills (when any), recall observations (when any). Empty layers MUST be omitted. Identity is loaded once at startup; overlay changes apply from the next turn.

#### Scenario: Full stack
- GIVEN identity, one matched skill, and recall observations
- THEN three system messages precede the conversation tail in that order

#### Scenario: Bare minimum
- GIVEN no skills and no observations
- THEN only the identity system message precedes the tail

### AGIS-M3-BRN-003: Close-time extraction hook
CloseSession MUST run skill extraction after the summarizer when enabled, bounded by the same close timeout. Extraction failures MUST log-and-continue; successful creations MUST record a `skill` session event.

#### Scenario: Extractor error non-fatal
- GIVEN the extraction LLM call fails
- THEN close completes and quit proceeds

## minimal-tui (MODIFIED)

### AGIS-M3-TUI-001: Slash-command dispatch
Input beginning with `/` MUST dispatch exact-match commands locally and MUST NOT reach the provider or persist as a message. Required commands: `/personality <name|none>`, `/persona freeze|reset|status`. Unknown commands MUST print an error line without changing state.

#### Scenario: Command handled locally
- GIVEN `/persona status`
- THEN status renders in the viewport and no provider call occurs

#### Scenario: Unknown command
- GIVEN `/foo`
- THEN an error line appears; conversation unchanged

## config-loader (MODIFIED)

### AGIS-M3-CONF-001: Agent and skills blocks
Config MUST support `agent.personalities` (map of custom presets), `agent.evolution_enabled` (bool, default true, explicit false survives), `skills.enabled` (bool, default true, explicit false survives), `skills.dir` (string, default `$AGIS_HOME/skills`). Absent keys MUST keep defaults.

#### Scenario: Custom personality
- GIVEN `agent.personalities: {mentor: "..."}`
- THEN `/personality mentor` resolves

#### Scenario: Disabled skills
- GIVEN `skills.enabled: false`
- THEN no skill loading, matching, or close-time extraction occurs
