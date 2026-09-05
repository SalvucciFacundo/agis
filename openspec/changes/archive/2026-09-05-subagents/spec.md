# Specification: Native Subagent Delegation (subagents)

## Purpose

Define the functional requirements, architectural boundaries, data contracts, security policies, and diagnostic probes for native subagent delegation (`delegate_task`) in AGIS (`agis`). This capability enables the primary `core.Brain` to spawn isolated, ephemeral child agent instances that execute focused multi-turn tasks within bounded execution loops, preventing parent context contamination and token bloat while strictly enforcing recursion limits, concurrency controls, timeout propagation, and `PolicyGuard` security invariants.

---

## Tool Contract & Wire Format

### Requirement SUB-TOOL-001: `delegate_task` Tool Contract & Metadata
The system MUST provide a `delegate_task` tool implementing the `core.ToolRunner` interface with backend identifier `"subagent"`.
- **Tool Name**: `"delegate_task"`
- **Backend**: `"subagent"`
- **Category**: `core.CategoryExecution` (`"execution"`)
- **Description**: `"Delegate a focused task to an isolated, bounded subagent instance. The subagent runs its own execution loop and returns a synthesized summary of the result."`
- **Input Parameters (JSON Schema)**:
  - `task` (string, required): Specific, clear description of the task for the child agent. MUST NOT be empty or whitespace-only.
  - `context` (string, optional): Contextual background, prior tool outputs, or constraints needed by the child agent.
  - `max_turns` (integer, optional): Maximum execution turn count for the child agent. Default is `8`. MUST NOT exceed the configured hard limit (clamped to `[1, 15]`).
- **Output Format**:
  - The tool MUST return a synthesized string representing the child agent's final text response.
  - Intermediate scratchpad reasoning, child tool call loops, and intermediate role messages MUST remain contained in the child's ephemeral state and MUST NOT be emitted directly to the parent conversation history.

#### Scenario: Subagent tool registration
- GIVEN a configured AGIS runtime with subagents enabled
- WHEN `core.Brain` initializes tool definitions
- THEN `delegate_task` is advertised in the tool definitions with backend `"subagent"` and parameter schema requiring `task`

#### Scenario: Successful execution of delegated task
- GIVEN a parent agent invoking `delegate_task` with `{"task": "Analyze error logs and extract unique stack traces", "context": "Log snippet attached"}`
- WHEN the tool executes
- THEN a child subagent is spawned with the provided task and context
- AND the child runs its bounded loop to completion
- AND returns the synthesized textual summary to the parent agent

---

### Requirement SUB-TOOL-002: Parameter Validation & Guardrails
The `delegate_task` runner MUST validate all input parameters before initiating subagent spawning.
- If `task` is missing, empty, or consists solely of whitespace, the tool MUST return a parameter validation error.
- If `max_turns` is provided as `<= 0`, the tool MUST use the default turn count (`8`).
- If `max_turns` exceeds the configured ceiling (`MaxTurns`, hard limit `15`), the runner MUST clamp `max_turns` to the ceiling.
- If the subagent subsystem is globally disabled via configuration (`enabled: false`), the tool MUST immediately return an error stating that subagent delegation is disabled.

#### Scenario: Rejection of empty task
- GIVEN an invocation of `delegate_task` with arguments `{"task": "   "}`
- WHEN input validation runs
- THEN the tool returns an error stating `"task parameter is required and cannot be empty"` without spawning a child agent

#### Scenario: Clamping excessive max_turns
- GIVEN an invocation of `delegate_task` with arguments `{"task": "Long search", "max_turns": 50}`
- WHEN the tool parses input parameters
- THEN `max_turns` is clamped to the maximum allowed limit (`15`) before launching the child agent

#### Scenario: Disabled subsystem rejection
- GIVEN configuration setting `subagents.enabled = false`
- WHEN `delegate_task` is called
- THEN the tool returns an error `"subagent delegation is disabled by configuration"`

---

### Requirement SUB-TOOL-003: Output Synthesis & Error Handling
The tool execution MUST handle child agent failures gracefully without crashing the parent process.
- When a child agent completes normally, the final assistant message MUST be returned as the tool string output.
- If a child agent hits its turn limit (`max_turns`) without producing a definitive conclusion, the tool MUST return the partial synthesis from the last completed assistant step appended with a warning indicating turn limit exhaustion.
- If a child agent encounters an unrecoverable LLM provider error, the tool MUST return an error message describing the provider failure.
- If the subagent execution is cancelled via context, the tool MUST immediately terminate the child loop and return a cancellation error.

#### Scenario: Graceful handling of turn exhaustion
- GIVEN a child agent that reaches its `max_turns` limit of `8` before completing the full task
- WHEN the child turn loop finishes round 8
- THEN the runner captures the last assistant response
- AND appends `"\n[subagent reached maximum turn limit (8)]"` to the returned string

#### Scenario: LLM provider failure in child
- GIVEN a child agent whose provider returns an authentication or network failure
- WHEN the child step fails
- THEN `delegate_task` returns `"subagent execution failed: provider error: <details>"` to the parent brain

---

## Subagent Execution Engine

### Requirement SUB-ENG-001: Ephemeral Session & State Lifecycle
The subagent execution engine MUST isolate child execution state from the parent session.
- Each delegated task MUST execute in an ephemeral session environment.
- The child agent MUST NOT overwrite or alter the parent conversation's active messages or repository state.
- Ephemeral child conversation records MUST either reside in an in-memory repository or use a temporary unique conversation ID (`subagent-<uuid>`) that is automatically cleaned up upon task completion.
- Any temporary resources allocated during subagent execution MUST be freed when the child runner exits.

#### Scenario: State isolation between parent and child
- GIVEN a parent session with conversation ID `"conv-parent-123"`
- WHEN `delegate_task` spawns a child subagent
- THEN the child operates under a distinct ephemeral conversation ID
- AND messages added during the child's execution do not appear in `"conv-parent-123"`

#### Scenario: Resource cleanup after execution
- GIVEN an ephemeral conversation created for a child subagent
- WHEN the child completes or errors out
- THEN temporary conversation records and in-memory structures are pruned

---

### Requirement SUB-ENG-002: Child Brain Instantiation & Tool Inheritance Filtering
The execution engine MUST instantiate a child `core.Brain` with inherited capabilities and strict security filtering.
- The child brain MUST inherit the parent's `core.Provider` (LLM client) and `core.PolicyGuard`.
- The child brain MUST inherit registered tool runners from the parent registry (e.g., local tools, web tools, MCP tools) EXCEPT `delegate_task` when recursion depth limit has been reached.
- The system prompt for the child agent MUST explicitly establish its role as a focused subagent whose primary objective is to execute the specified task and return a clear, concise synthesis.

#### Scenario: Filtering delegate_task at max depth
- GIVEN a child agent running at recursion depth equal to `MaxDepth` (e.g., depth 1)
- WHEN the child's tool registry is constructed
- THEN `delegate_task` is excluded from the child's available tools
- AND the child cannot invoke further subagents

#### Scenario: Child tool availability
- GIVEN parent registry with tools `["local", "web_search", "web_fetch", "delegate_task"]`
- WHEN spawning a depth-1 child subagent
- THEN the child receives tools `["local", "web_search", "web_fetch"]`

---

### Requirement SUB-ENG-003: Concurrency Limiting & Semaphore Control
The execution engine MUST enforce a global concurrency limit on active subagents to prevent resource and rate-limit exhaustion.
- The engine MUST maintain a semaphore pool whose capacity is configured by `SubagentsConfig.MaxConcurrent` (default `3`).
- When `delegate_task` runs, it MUST acquire a slot from the semaphore before spawning the child agent.
- If all slots are busy, the call MUST block waiting for an available slot until the context expires or a slot is released.
- The semaphore slot MUST be released immediately when the child execution finishes (via `defer`).

#### Scenario: Concurrency limit enforcement
- GIVEN `MaxConcurrent` configured to `2` and 2 subagents currently running
- WHEN a 3rd `delegate_task` call arrives
- THEN the 3rd call blocks until one of the 2 active subagents finishes
- AND acquires the slot immediately upon release to proceed

#### Scenario: Semaphore release on panic or error
- GIVEN an active child subagent that returns an error or encounters a panic
- WHEN the execution unwinds
- THEN the semaphore slot is guaranteed to be released back to the pool

---

### Requirement SUB-ENG-004: Context Timeout & Cancellation Propagation
The execution engine MUST enforce strict execution deadlines and bidirectional cancellation.
- The execution engine MUST derive a child context with a timeout configured by `SubagentsConfig.DefaultTimeout` (default `60s`).
- If the parent `context.Context` has a shorter remaining deadline, the child MUST inherit the tighter deadline.
- If the parent context is cancelled (e.g., user interrupts execution via Ctrl+C / TUI cancel), the child context MUST be cancelled immediately, aborting all child goroutines and provider requests.

#### Scenario: Default timeout triggered
- GIVEN a child subagent running a task that takes longer than `DefaultTimeout` (60s)
- WHEN the 60-second deadline expires
- THEN the child context cancels
- AND `delegate_task` returns a timeout error to the parent

#### Scenario: Parent cancellation propagation
- GIVEN an active child subagent running a multi-step task
- WHEN the parent context is cancelled by user interruption
- THEN the child execution stops immediately without orphan goroutine leaks

---

### Requirement SUB-ENG-005: Recursion Depth Control & Clamping
The engine MUST track and enforce recursion depth across delegated calls.
- The execution context MUST track the current delegation depth (root = `0`, child = `1`, grandchild = `2`).
- The configured `MaxDepth` MUST NOT exceed the hard maximum limit of `2` (default `1`).
- If a subagent attempts to invoke `delegate_task` when the current depth is `>= MaxDepth`, the execution MUST be rejected with a recursion limit exceeded error.

#### Scenario: Rejection when recursion depth exceeded
- GIVEN `MaxDepth` set to `1` and a running child agent at depth `1`
- WHEN the child attempts to execute `delegate_task`
- THEN the call fails with `"recursion depth limit (1) exceeded"`

#### Scenario: Hard clamp on MaxDepth configuration
- GIVEN configuration specifying `subagents.max_depth: 5`
- WHEN configuration is loaded
- THEN `MaxDepth` is clamped to the hard maximum of `2`

---

## PolicyGuard & Security

### Requirement SUB-SEC-001: Execution Policy Category & Guard Evaluation
The system MUST integrate subagent delegation into the `PolicyGuard` framework.
- `internal/core` MUST define constant `CategoryExecution = "execution"`.
- Every `delegate_task` execution MUST be evaluated through `PolicyGuard.Evaluate` before spawning a child.
- The `GuardRequest` for subagent delegation MUST have:
  - `Backend`: `"subagent"`
  - `Category`: `core.CategoryExecution`
  - `Subject`: The task description string (or a truncated prefix up to 256 characters for pattern matching).

#### Scenario: Policy evaluation before delegation
- GIVEN a parent brain processing a `delegate_task` tool call with task `"Inspect repository structure"`
- WHEN `executeTool` is evaluated
- THEN a `GuardRequest{Backend: "subagent", Category: "execution", Subject: "Inspect repository structure"}` is passed to `PolicyGuard.Evaluate`

---

### Requirement SUB-SEC-002: Posture-Based Authorization
The `PolicyGuard` evaluate logic MUST enforce posture-based rules for the `"subagent"` backend:
- Under `PostureSandbox` (`"sandbox"`): Delegation MUST be denied by default unless an explicit `allow` rule matches in `policy.yaml`.
- Under `PostureStandard` (`"standard"`): If an `allow` rule matches, delegation is allowed; otherwise, it MUST return `DecisionAsk` (prompting user approval in interactive modes or returning ask).
- Under `PostureFull` (`"full"`): Delegation MUST be allowed by default.
- An explicit `deny` rule for category `"execution"` and backend `"subagent"` MUST always take precedence and deny execution.

#### Scenario: Sandbox posture blocks subagents by default
- GIVEN policy tier for `"subagent"` is `sandbox` with no explicit rules
- WHEN a subagent delegation request is evaluated
- THEN `PolicyGuard` returns `DecisionDeny`

#### Scenario: Standard posture triggers ask
- GIVEN policy tier for `"subagent"` is `standard` with no matching allow rule
- WHEN a subagent delegation request is evaluated
- THEN `PolicyGuard` returns `DecisionAsk`

#### Scenario: Full posture allows subagents
- GIVEN policy tier for `"subagent"` is `full`
- WHEN a subagent delegation request is evaluated
- THEN `PolicyGuard` returns `DecisionAllow`

---

### Requirement SUB-SEC-003: Audit Logging for Delegation Events
All subagent delegation requests MUST be recorded in the policy audit log via `AuditEntry`.
- Every evaluation MUST log an `AuditEntry` containing timestamp, backend `"subagent"`, category `"execution"`, subject (task summary), and the resulting decision (`"allow"`, `"deny"`, or `"ask"`).
- In addition, subagent completion events SHOULD log execution metrics (duration, turn count, depth, success/failure status) using structured logging (`slog`).

#### Scenario: Audit entry recorded on delegation
- GIVEN a subagent execution evaluated under standard policy
- WHEN evaluation occurs
- THEN an `AuditEntry` is appended to the audit store with `Backend: "subagent"`, `Category: "execution"`, and `Decision: "allow"` (or `"ask"`)

---

## Subsystem Configuration

### Requirement SUB-CFG-001: Subagents Configuration Schema & Defaults
The system configuration in `internal/config` MUST include a `SubagentsConfig` section with explicit defaults:
```go
type SubagentsConfig struct {
    Enabled        bool          `yaml:"enabled"`
    MaxConcurrent  int           `yaml:"max_concurrent"`
    MaxDepth       int           `yaml:"max_depth"`
    DefaultTimeout time.Duration `yaml:"default_timeout"`
    MaxTurns       int           `yaml:"max_turns"`
}
```
Default values when unconfigured:
- `Enabled`: `true`
- `MaxConcurrent`: `3`
- `MaxDepth`: `1`
- `DefaultTimeout`: `60 * time.Second`
- `MaxTurns`: `8`

#### Scenario: Default configuration initialization
- GIVEN an empty configuration file
- WHEN `config.Load` parses the defaults
- THEN `Subagents.Enabled` is `true`, `MaxConcurrent` is `3`, `MaxDepth` is `1`, `DefaultTimeout` is `60s`, and `MaxTurns` is `8`

---

### Requirement SUB-CFG-002: Hard Boundary Clamping
The configuration loader MUST validate and clamp subagent parameters to prevent dangerous configuration values:
- `MaxConcurrent`: If `<= 0`, reset to `1`. If `> 10`, clamp to `10`.
- `MaxDepth`: If `<= 0`, reset to `1`. If `> 2`, clamp to `2`.
- `MaxTurns`: If `<= 0`, reset to `8`. If `> 15`, clamp to `15`.
- `DefaultTimeout`: If `<= 0`, reset to `60s`. If `> 300s` (5 minutes), clamp to `300s`.

#### Scenario: Boundary clamping on invalid configuration
- GIVEN a config file specifying `max_depth: 10`, `max_concurrent: 0`, and `max_turns: 100`
- WHEN `config.Load` validates the config
- THEN `MaxDepth` is clamped to `2`, `MaxConcurrent` is reset to `1`, and `MaxTurns` is clamped to `15`

---

## Diagnostic Health Probe

### Requirement SUB-DOC-001: Doctor Subagents Probe Verification
The diagnostic subsystem in `internal/doctor` MUST include a probe `checkSubagents(ctx context.Context) CheckResult`.
- When subagents are disabled (`enabled: false`), the probe MUST return `StatusPass` with message `"Subagents subsystem disabled"`.
- When subagents are enabled, the probe MUST return `StatusPass` with details including:
  - Enabled status
  - Configured `MaxConcurrent` and `MaxDepth`
  - `DefaultTimeout` and `MaxTurns`
  - Availability of LLM provider client for child spawning
- If configuration has invalid or clamped values, the probe MUST detail the active clamped settings.

#### Scenario: Doctor probe output with subagents enabled
- GIVEN a running AGIS instance with subagents enabled and default settings
- WHEN `doctor.Run` executes `checkSubagents`
- THEN the check returns `StatusPass` with details:
  - `"Subagents enabled"`
  - `"Max concurrency: 3"`
  - `"Max depth: 1 (hard limit: 2)"`
  - `"Default timeout: 1m0s"`
  - `"Max turns per task: 8"`

#### Scenario: Doctor probe output with subagents disabled
- GIVEN `subagents.enabled = false`
- WHEN `doctor.Run` executes `checkSubagents`
- THEN the check returns `StatusPass` with message `"Subagents subsystem disabled"`
