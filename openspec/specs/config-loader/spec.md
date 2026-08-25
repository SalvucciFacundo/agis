# Config Loader Spec

## Purpose

Load and validate AGIS configuration from a YAML file with safe defaults and a documented precedence order.

## Requirements

### Requirement: Load configuration from YAML
The system MUST load `~/.agis/config.yaml` with mode `0600`, warning if looser. Precedence MUST be `-config` flag > `AGIS_HOME` > default path. M1 fields MUST include `llm.provider`, `llm.model`, `llm.api_key`, and `db.path`.

#### Scenario: Config loads with defaults
- GIVEN `~/.agis/config.yaml` is missing
- WHEN application starts
- THEN it uses built-in defaults.


config-loader (MODIFIED)

### Requirement: Agent and skills blocks
Config MUST support `agent.personalities` (map of custom presets), `agent.evolution_enabled` (bool, default true, explicit false survives), `skills.enabled` (bool, default true, explicit false survives), `skills.dir` (string, default `$AGIS_HOME/skills`). Absent keys MUST keep defaults.

#### Scenario: Custom personality
- GIVEN `agent.personalities: {mentor: "..."}`
- THEN `/personality mentor` resolves

#### Scenario: Disabled skills
- GIVEN `skills.enabled: false`
- THEN no skill loading, matching, or close-time extraction occurs
