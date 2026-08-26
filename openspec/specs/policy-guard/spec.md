# Policy Guard Spec

## Purpose

Policy Guard is the single enforcement point for every tool call. It owns $AGIS_HOME/policy.yaml, evaluates subjects against postures, rules and session grants, and records every decision in the audit log. The brain never mutates policy directly.

## Requirements

policy-guard (NEW)

### Requirement: Policy store
System MUST manage `$AGIS_HOME/policy.yaml` (categories: commands, files, network, backends; rules per backend with action allow|deny). `agis policy init` MUST create safe defaults with every backend at posture `sandbox`. A corrupt or unreadable policy file MUST yield deny-all decisions plus a surfaced error — the system MUST NOT fall back to permissive defaults (fail closed).

#### Scenario: Init creates safe defaults
- GIVEN no policy.yaml
- WHEN `agis policy init` runs
- THEN the file exists with all backends at sandbox

#### Scenario: Corrupt policy fails closed
- GIVEN a syntactically broken policy.yaml
- WHEN any tool decision is evaluated
- THEN the decision is deny and the error is surfaced

### Requirement: Decision flow
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

### Requirement: Approval scopes
When the guard returns ask, the resolved scope MUST behave as: once runs this time only; session skips further asks until session close; always persists an allow rule to policy.yaml immediately; deny records refusal. Session grants MUST be held in memory only and cleared on CloseSession. Only always writes persistent policy.

#### Scenario: Session grant suppresses repeat ask
- GIVEN a session grant for `git status`
- WHEN `git status` is evaluated again
- THEN the decision is allow without asking

#### Scenario: Always persists across restarts
- GIVEN an always grant
- THEN policy.yaml contains the rule after process restart

### Requirement: CLI management
`agis policy` MUST implement init, set `<category> <pattern> <allow|deny>`, rm, show, tier `<backend> <posture>`, and test `<command>` (dry-run printing the decision without executing). Setting tier full via CLI MUST be refused with guidance that full is session-only through the TUI.

#### Scenario: Test previews decision
- GIVEN an allow rule for `git`
- WHEN `agis policy test git status` runs
- THEN it prints allow and executes nothing

#### Scenario: Tier full refused via CLI
- WHEN `agis policy tier local full` runs
- THEN the CLI refuses and explains the session-only path

### Requirement: Audit log
Every decision (allow/deny/ask resolution), grant, revocation, and tier change MUST be appended to the audit log with timestamp, backend, category, subject, decision, and scope. The log MUST be readable for the `/permisos` panel.

#### Scenario: Ask resolution audited
- GIVEN an ask resolved as session
- THEN the audit log holds one row with scope session
