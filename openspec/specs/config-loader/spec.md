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


config-loader (MODIFIED)

### Requirement: Tools configuration
Config MUST support `tools.enabled` (default false — tools are opt-in), `tools.backends.docker` (enabled, image), and `tools.backends.ssh` (enabled, host, user, key_path). Absent keys MUST keep defaults; enabling a backend without its binary degrades with a warning.

#### Scenario: Tools off by default
- GIVEN no tools block in config
- THEN no tools are registered and the brain streams exactly as before


config-loader (MODIFIED)

### Requirement AGIS-M6-CONF-003: Ecosystem Configuration Schema (Gateway, Cron, Plugins, Webhook)
The configuration loader in `internal/config/config.go` MUST support the following optional root configuration blocks:

```yaml
gateway:
  enabled: false
  telegram:
    enabled: false
    token: ""
    allowlist: []
  discord:
    enabled: false
    token: ""
    allowlist: []

cron:
  enabled: false
  jobs:
    - name: "daily-health"
      schedule: "@every 1h"
      prompt: "Check system health"
      session_id: "cron-health"
      target:
        adapter: "telegram"
        recipient: "123456"

plugins:
  enabled: false
  dir: "~/.agis/plugins"

webhook:
  enabled: false
  host: "127.0.0.1"
  port: 8080
  path: "/webhook"
  secret: ""
  default_session_id: "webhook-events"
  target:
    adapter: "telegram"
    recipient: "123456"
```

- All ecosystem blocks MUST be disabled by default (`enabled: false`).
- Missing fields MUST inherit documented safe defaults (`host: "127.0.0.1"`, `port: 8080`, `path: "/webhook"`, `plugins.dir: "$AGIS_HOME/plugins"`).
- Unmarshaling must handle environment variable expansions or missing files without panicking.

#### Scenario: Default configuration disables ecosystem blocks
- GIVEN an empty or minimal `config.yaml`
- WHEN `config.Load()` is invoked
- THEN `cfg.Gateway.Enabled`, `cfg.Cron.Enabled`, `cfg.Plugins.Enabled`, and `cfg.Webhook.Enabled` are all `false`

#### Scenario: Full ecosystem configuration parsed
- GIVEN a `config.yaml` containing complete `gateway`, `cron`, `plugins`, and `webhook` blocks
- WHEN `config.Load()` is invoked
- THEN all struct fields, job lists, allowlists, and secrets are populated accurately


config-loader (MODIFIED)

### Requirement AGIS-M7-CONF-001: Embeddings Configuration Schema
The configuration loader in `internal/config/config.go` MUST support the following optional root configuration block:

```yaml
embeddings:
  enabled: false
  provider: "ollama"           # "ollama" | "openai"
  model: "nomic-embed-text"    # default: nomic-embed-text for ollama, text-embedding-3-small for openai
  dimensions: 768              # default: 768 for ollama, 1536 for openai
  batch_size: 100              # maximum sub-batch chunk size (capped at 2048)
```

- `embeddings.enabled` MUST default to `false` (opt-in).
- Missing model/dimension fields MUST inherit provider-specific defaults.
- Absent `embeddings` block MUST preserve backward compatibility.

#### Scenario: Default configuration disables embeddings
- GIVEN an empty or default `config.yaml`
- WHEN `config.Load()` is executed
- THEN `cfg.Embeddings.Enabled` is `false` and defaults are set

#### Scenario: Explicit embeddings configuration loaded
- GIVEN `config.yaml` with `embeddings.provider: openai` and `embeddings.model: text-embedding-3-small`
- WHEN `config.Load()` is executed
- THEN `cfg.Embeddings.Dimensions` defaults to `1536` and `BatchSize` to `100`


