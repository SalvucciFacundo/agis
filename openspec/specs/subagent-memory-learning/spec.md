# Delta for Subagents (subagent-memory-learning)

## ADDED Requirements

### Requirement SUB-DIST-001: Passive Knowledge Distillation Parser
The system MUST provide a passive knowledge distillation parser in `internal/subagents` (`ExtractKeyLearnings`) that extracts structured memory observations from subagent synthesized outputs.
- The parser MUST detect markdown sections headed by `## Key Learnings`, `## Discoveries`, `## Key Takeaways`, and `## Decisions` (case-insensitive match, allowing optional trailing colons).
- The parser MUST extract individual list entries formatted as unordered bullets (`- `, `* `) or ordered numbered items (`1. `, `2. `, `[0-9]+\. `).
- Extracted entries MUST be stripped of list markers and leading/trailing whitespace. Empty or whitespace-only items MUST be discarded.
- For each valid item, the parser MUST construct a `core.Observation` entity:
  - `TopicKey`: Namespaced as `subagent/<sanitized_task_slug>/<identifier>`, where `<sanitized_task_slug>` is an alphanumeric and hyphenated slug derived from the task description (max 32 characters), and `<identifier>` is a deterministic sequence index or content hash.
  - `Type`: Inferred from the section heading (e.g. `## Decisions` -> `"decision"`, `## Key Learnings` / `## Discoveries` / `## Key Takeaways` -> `"discovery"`, or fallback `"discovery"`).
  - `Content`: The raw item text.
  - `Importance`: Default `3`, clamped to `[1, 5]`.
  - `SourceRef`: The conversation ID of the execution session.
- The total count of extracted observations returned MUST NOT exceed `maxObs` (bounded by `[1, 5]`).

#### Scenario: Extract numbered items from Key Learnings section
- GIVEN a subagent output containing:
  ```markdown
  Task completed.
  ## Key Learnings
  1. The authentication service requires redis v7+.
  2. Cache keys must be prefixed with tenant ID.
  ```
- WHEN `ExtractKeyLearnings` is called with task `"Investigate auth cache"` and `maxObs: 3`
- THEN exactly 2 `core.Observation` instances are returned
- AND their `TopicKey` values begin with `subagent/investigate-auth-cache/`
- AND their `Type` is `"discovery"`
- AND their contents match the respective list items

#### Scenario: Extract bullet items from Decisions section
- GIVEN a subagent output containing:
  ```markdown
  ## Decisions
  - Selected SQLite WAL mode for higher concurrency.
  - Configured 60s busy timeout.
  ```
- WHEN `ExtractKeyLearnings` is called with task `"Tune database"` and `maxObs: 3`
- THEN 2 `core.Observation` instances are returned with `Type` set to `"decision"`

#### Scenario: Output without structured learning sections
- GIVEN a subagent output with standard prose and no recognized distillation headings
- WHEN `ExtractKeyLearnings` parses the output
- THEN an empty slice of observations (`[]core.Observation{}`) is returned without error

#### Scenario: Enforcing max observations ceiling
- GIVEN a subagent output containing 8 bullet points under `## Key Learnings`
- WHEN `ExtractKeyLearnings` is called with `maxObs: 3`
- THEN only the first 3 observations are returned

---

### Requirement SUB-DIST-002: Distillation Pipeline & Parent Memory Persistence
The subagent execution engine (`subagents.Engine.Spawn`) MUST execute a knowledge distillation and memory persistence pipeline upon successful completion of a child subagent execution.
- After the child `core.Brain.Step` produces a synthesized response and before returning to the caller:
  - If `cfg.LearningEnabled` is `true` and the parent repository (`e.parent`) is not `nil`:
    - The engine MUST invoke `ExtractKeyLearnings` with the trimmed task description, the synthesized response text, and `cfg.MaxObservations`.
    - If one or more observations are extracted:
      - The engine MUST call `e.parent.SaveObservations(childCtx, conv.ID, observations)` to atomically persist them into the parent SQLite memory store and trigger FTS/vector indexing.
      - The engine MUST record an audit entry for the learning event via `e.parent.AppendAudit` with `Backend: "subagent"`, `Category: "learning"`, `Subject: fmt.Sprintf("distilled %d observations from subagent task: %s", len(observations), task)`, and `Decision: "allow"`.
- If `cfg.LearningEnabled` is `false`, the engine MUST skip distillation and persistence entirely.
- If `SaveObservations` or `AppendAudit` returns an error, the engine MUST log a warning and MUST NOT fail the `Spawn` execution or discard the subagent's response (graceful degradation).

#### Scenario: Automatic persistence of distilled observations
- GIVEN an enabled learning configuration (`LearningEnabled: true`, `MaxObservations: 3`)
- AND a child subagent execution producing output with `## Key Learnings`
- WHEN `Engine.Spawn` completes the child execution loop
- THEN `e.parent.SaveObservations` is invoked with the extracted observations
- AND an `AuditEntry` with `Category: "learning"` and `Backend: "subagent"` is recorded
- AND the synthesized output is returned to the parent caller

#### Scenario: Learning disabled skips distillation
- GIVEN `LearningEnabled: false`
- AND a child subagent producing output with `## Key Learnings`
- WHEN `Engine.Spawn` completes
- THEN `e.parent.SaveObservations` is NOT called
- AND no learning audit entry is generated
- AND the synthesized output is returned normally

#### Scenario: Resilience against memory save errors
- GIVEN a parent repository whose `SaveObservations` returns an I/O error
- WHEN `Engine.Spawn` attempts to persist distilled observations
- THEN the error is logged as a warning
- AND `Spawn` succeeds returning the synthesized response without crashing or failing the delegation

---

## MODIFIED Requirements

### Requirement SUB-CFG-001: Subagents Configuration Schema & Defaults
The system configuration in `internal/config` MUST include a `SubagentsConfig` section with explicit defaults:
```go
type SubagentsConfig struct {
    Enabled         bool          `yaml:"enabled"`
    MaxConcurrent   int           `yaml:"max_concurrent"`
    MaxDepth        int           `yaml:"max_depth"`
    DefaultTimeout  time.Duration `yaml:"default_timeout"`
    MaxTurns        int           `yaml:"max_turns"`
    LearningEnabled bool          `yaml:"learning_enabled"`
    MaxObservations int           `yaml:"max_observations"`
}
```
Default values when unconfigured:
- `Enabled`: `true`
- `MaxConcurrent`: `3`
- `MaxDepth`: `1`
- `DefaultTimeout`: `60 * time.Second`
- `MaxTurns`: `8`
- `LearningEnabled`: `true`
- `MaxObservations`: `3`

(Previously: SubagentsConfig contained only Enabled, MaxConcurrent, MaxDepth, DefaultTimeout, and MaxTurns without learning and observation bounds.)

#### Scenario: Default configuration initialization
- GIVEN an empty configuration file
- WHEN `config.Load` parses the defaults
- THEN `Subagents.Enabled` is `true`, `MaxConcurrent` is `3`, `MaxDepth` is `1`, `DefaultTimeout` is `60s`, `MaxTurns` is `8`, `LearningEnabled` is `true`, and `MaxObservations` is `3`

---

### Requirement SUB-CFG-002: Hard Boundary Clamping
The configuration loader MUST validate and clamp subagent parameters to prevent dangerous configuration values:
- `MaxConcurrent`: If `<= 0`, reset to `1`. If `> 10`, clamp to `10`.
- `MaxDepth`: If `<= 0`, reset to `1`. If `> 2`, clamp to `2`.
- `MaxTurns`: If `<= 0`, reset to `8`. If `> 15`, clamp to `15`.
- `DefaultTimeout`: If `<= 0`, reset to `60s`. If `> 300s` (5 minutes), clamp to `300s`.
- `MaxObservations`: If `<= 0`, reset to `3`. If `> 5`, clamp to `5`.

(Previously: Clamping only covered MaxConcurrent, MaxDepth, MaxTurns, and DefaultTimeout.)

#### Scenario: Boundary clamping on invalid configuration
- GIVEN a config file specifying `max_depth: 10`, `max_concurrent: 0`, `max_turns: 100`, and `max_observations: 20`
- WHEN `config.Load` validates the config
- THEN `MaxDepth` is clamped to `2`, `MaxConcurrent` is reset to `1`, `MaxTurns` is clamped to `15`, and `MaxObservations` is clamped to `5`

#### Scenario: Boundary clamping on negative max_observations
- GIVEN a config file specifying `max_observations: -5`
- WHEN `config.Load` validates the config
- THEN `MaxObservations` is reset to default `3`

---

### Requirement SUB-DOC-001: Doctor Subagents Probe Verification
The diagnostic subsystem in `internal/doctor` MUST include a probe `checkSubagents(ctx context.Context) CheckResult`.
- When subagents are disabled (`enabled: false`), the probe MUST return `StatusPass` with message `"Subagents subsystem disabled"`.
- When subagents are enabled, the probe MUST return `StatusPass` with details including:
  - Enabled status
  - Configured `MaxConcurrent` and `MaxDepth`
  - `DefaultTimeout` and `MaxTurns`
  - `LearningEnabled` status (e.g. `"Learning enabled: true"`)
  - `MaxObservations` ceiling (e.g. `"Max observations per task: 3"`)
  - Availability of LLM provider client for child spawning
- If configuration has invalid or clamped values, the probe MUST detail the active clamped settings.

(Previously: Probe only reported concurrency, depth, timeout, and max turns without knowledge learning state.)

#### Scenario: Doctor probe output with subagents and learning enabled
- GIVEN a running AGIS instance with subagents enabled and default settings
- WHEN `doctor.Run` executes `checkSubagents`
- THEN the check returns `StatusPass` with details:
  - `"Subagents enabled"`
  - `"Max concurrency: 3"`
  - `"Max depth: 1 (hard limit: 2)"`
  - `"Default timeout: 1m0s"`
  - `"Max turns per task: 8"`
  - `"Learning enabled: true"`
  - `"Max observations per task: 3"`

#### Scenario: Doctor probe output with learning disabled
- GIVEN `subagents.enabled = true` and `subagents.learning_enabled = false`
- WHEN `doctor.Run` executes `checkSubagents`
- THEN the check returns `StatusPass` with detail `"Learning enabled: false"`

#### Scenario: Doctor probe output with subagents disabled
- GIVEN `subagents.enabled = false`
- WHEN `doctor.Run` executes `checkSubagents`
- THEN the check returns `StatusPass` with message `"Subagents subsystem disabled"`

---

### Requirement SUB-SEC-003: Audit Logging for Delegation & Learning Events
All subagent delegation requests and memory learning distillation events MUST be recorded in the policy audit log via `AuditEntry`.
- Every evaluation MUST log an `AuditEntry` containing timestamp, backend `"subagent"`, category `"execution"`, subject (task summary), and the resulting decision (`"allow"`, `"deny"`, or `"ask"`).
- Every successful knowledge distillation event MUST log an `AuditEntry` with backend `"subagent"`, category `"learning"`, subject detailing distilled observation count and task summary, and decision `"allow"`.
- In addition, subagent completion events SHOULD log execution metrics (duration, turn count, depth, success/failure status, observations count) using structured logging (`slog`).

(Previously: Audit logging only covered delegation request evaluation without knowledge distillation tracking.)

#### Scenario: Audit entry recorded on delegation evaluation
- GIVEN a subagent execution evaluated under standard policy
- WHEN evaluation occurs
- THEN an `AuditEntry` is appended to the audit store with `Backend: "subagent"`, `Category: "execution"`, and `Decision: "allow"` (or `"ask"`)

#### Scenario: Audit entry recorded on knowledge distillation
- GIVEN a subagent completing a task that yields 2 observations
- WHEN distillation persists the observations to parent repository
- THEN an `AuditEntry` is appended with `Backend: "subagent"`, `Category: "learning"`, `Subject: "distilled 2 observations from subagent task: ..."` and `Decision: "allow"`
