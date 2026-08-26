# M4 — Tools, backends & permissions (delta spec)

Delta spec for `m4-tools-permissions`: Policy Guard, tool backends, streaming tool-calls, approvals, audit.

## policy-guard (NEW)

### AGIS-M4-POL-001: Policy store
System MUST manage `$AGIS_HOME/policy.yaml` (categories: commands, files, network, backends; rules per backend with action allow|deny). `agis policy init` MUST create safe defaults with every backend at posture `sandbox`. A corrupt or unreadable policy file MUST yield deny-all decisions plus a surfaced error — the system MUST NOT fall back to permissive defaults (fail closed).

#### Scenario: Init creates safe defaults
- GIVEN no policy.yaml
- WHEN `agis policy init` runs
- THEN the file exists with all backends at sandbox

#### Scenario: Corrupt policy fails closed
- GIVEN a syntactically broken policy.yaml
- WHEN any tool decision is evaluated
- THEN the decision is deny and the error is surfaced

### AGIS-M4-POL-002: Decision flow
The guard MUST evaluate in order: baseline posture → category rules → explicit overrides, returning allow, deny, or ask. Sandbox MUST deny everything except read-only operations on allowlisted paths, regardless of other rules. Standard MUST allow allowlisted subjects, ask for unlisted ones, and honor deny rules before allow rules. Full posture MUST behave as blanket allow for the current session only. Every decision MUST be recorded in the audit log.

#### Scenario: Sandbox denies by default
- GIVEN local backend at sandbox and no matching rule
- WHEN any shell command is evaluated
- THEN the decision is deny

#### Scenario: Standard asks outside allowlist
- GIVEN standard posture and command `curl` not listed
- WHEN evaluated
- THEN the decision is ask

#### Scenario: Deny beats allow
- GIVEN conflicting rules for one subject
- THEN the decision is deny

### AGIS-M4-POL-003: Approval scopes
When the guard returns ask, the resolved scope MUST behave as: once runs this time only; session skips further asks until session close; always persists an allow rule to policy.yaml immediately; deny records refusal. Session grants MUST be held in memory only and cleared on CloseSession. Only always writes persistent policy.

#### Scenario: Session grant suppresses repeat ask
- GIVEN a session grant for `git status`
- WHEN `git status` is evaluated again
- THEN the decision is allow without asking

#### Scenario: Always persists across restarts
- GIVEN an always grant
- THEN policy.yaml contains the rule after process restart

### AGIS-M4-POL-004: CLI management
`agis policy` MUST implement init, set `<category> <pattern> <allow|deny>`, rm, show, tier `<backend> <posture>`, and test `<command>` (dry-run printing the decision without executing). Setting tier full via CLI MUST be refused with guidance that full is session-only through the TUI.

#### Scenario: Test previews decision
- GIVEN an allow rule for `git`
- WHEN `agis policy test git status` runs
- THEN it prints allow and executes nothing

#### Scenario: Tier full refused via CLI
- WHEN `agis policy tier local full` runs
- THEN the CLI refuses and explains the session-only path

### AGIS-M4-POL-005: Audit log
Every decision (allow/deny/ask resolution), grant, revocation, and tier change MUST be appended to the audit log with timestamp, backend, category, subject, decision, and scope. The log MUST be readable for the `/permisos` panel.

#### Scenario: Ask resolution audited
- GIVEN an ask resolved as session
- THEN the audit log holds one row with scope session

## tools-backends (NEW)

### AGIS-M4-TLS-001: Tool port and registry
A Tool MUST expose Name, Description, and Execute(ctx, args, guard). The registry MUST register enabled backends' tools at startup; a backend whose binary is unavailable MUST be skipped with a warning when enabled, never fatal.

#### Scenario: Docker missing degrades
- GIVEN docker.enabled true and no docker binary
- THEN startup continues without docker tools and warns

### AGIS-M4-TLS-002: Local backend
The local backend MUST execute shell commands and filesystem reads/writes on the host. In sandbox posture it MUST refuse destructive classes (writes outside allowlist, network commands, package removal) even when patterns would otherwise match.

#### Scenario: Sandbox blocks network command
- GIVEN sandbox posture
- WHEN `curl example.com` executes locally
- THEN it is denied

### AGIS-M4-TLS-003: Docker backend
The docker backend MUST run each command inside an ephemeral container from a configured image, requiring the docker binary. Container teardown MUST occur after execution even on failure.

#### Scenario: Ephemeral execution
- WHEN a command runs on the docker backend
- THEN no container survives the call

### AGIS-M4-TLS-004: SSH backend
The ssh backend MUST execute commands on a configured host (host, user, key path) per call, surfacing connection failures as tool errors.

#### Scenario: Connection failure surfaces
- GIVEN an unreachable host
- THEN Execute returns an error and the audit logs a failed attempt

## tool-calling (NEW)

### AGIS-M4-TOL-001: Wire format
ChatRequest MUST carry the advertised tools (name, description). StreamEvent MUST carry an optional ToolCall (name, arguments). Zero-value events MUST remain plain text tokens: existing flows MUST be unaffected.

#### Scenario: Plain reply unchanged
- GIVEN no tools advertised
- THEN streamed events are text-only as before

### AGIS-M4-TOL-002: Bounded tool loop
On a ToolCall event, Step MUST evaluate the call through the guard: allow executes on the backend and feeds the output back as a RoleTool message; deny informs the model the action was blocked; ask presents the interactive approval in the TUI (auto-deny surfaces later). The loop MUST stop after 8 rounds, informing the model, and every round MUST be audited.

#### Scenario: Approved command feeds back
- GIVEN an ask approved as once for `git status`
- THEN the command runs, its output reaches the model, and the reply continues

#### Scenario: Runaway loop capped
- GIVEN a model requesting tools for 8 consecutive rounds
- THEN round 9 does not execute and the model is told to answer directly

## brain-loop (MODIFIED)

### AGIS-M4-BRN-004: Tool calls execute under guard
Step MUST route every model-initiated tool request through the PolicyGuard port before any execution; the model's own output MUST be incapable of mutating policy state.

#### Scenario: Model cannot self-grant
- GIVEN a model reply attempting to change policy
- THEN no policy mutation occurs and the attempt is inert data

## llm-provider-port (MODIFIED)

### AGIS-M4-LLM-001: Additive tool-call parsing
Provider adapters MUST parse tool_calls from responses defensively: known shapes populate StreamEvent.ToolCall, unknown or malformed shapes degrade to plain text events without breaking the stream contract (channel always closes).

#### Scenario: Malformed tool call degrades
- GIVEN a provider emitting an unrecognized tool_calls shape
- THEN the stream continues as text and closes normally

## minimal-tui (MODIFIED)

### AGIS-M4-TUI-002: Interactive approval
When the guard returns ask during a turn, the TUI MUST show the exact action with four choices — allow once, allow for session, always allow, deny — mapped to fixed keys, with deny as the safe default on interrupt.

#### Scenario: Interrupting a prompt denies
- GIVEN a visible approval prompt
- WHEN CtrlC pressed
- THEN the action is denied and audited

### AGIS-M4-TUI-003: Permissions panel
`/permisos` MUST open a panel listing rules grouped by category, offering allow/deny toggles, per-backend posture display, a decision preview for a typed command, audit-log view, and revocation of always grants.

#### Scenario: Revoke an always grant
- GIVEN an always rule visible in the panel
- WHEN revoked
- THEN policy.yaml loses the rule and the audit records the revocation

## config-loader (MODIFIED)

### AGIS-M4-CONF-002: Tools configuration
Config MUST support `tools.enabled` (default false — tools are opt-in), `tools.backends.docker` (enabled, image), and `tools.backends.ssh` (enabled, host, user, key_path). Absent keys MUST keep defaults; enabling a backend without its binary degrades with a warning.

#### Scenario: Tools off by default
- GIVEN no tools block in config
- THEN no tools are registered and the brain streams exactly as before
