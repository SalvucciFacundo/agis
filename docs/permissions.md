# Permissions

> **Implemented in M4.** Policy Guard is the single enforcement point for every tool call. `policy.yaml` lives at `$AGIS_HOME/policy.yaml`, managed via `agis policy` CLI and the `/permisos` TUI panel.

The permission system is the user-facing layer over the Policy Guard, which is the single enforcement point for every tool call. Modeled on GAIA's `policy init` + `/permisos` panel, generalized for a general-purpose agent — and fully configurable without hand-editing YAML.

## Two orthogonal axes

Every decision combines a **baseline posture** (how restrictive the backend is by default) and an **approval scope** (what a granted approval means over time) — the model proven by Hermes and Claude Code.

**Baseline posture (per backend):**

| Posture | Meaning |
|---|---|
| `sandbox` | Deny by default. Read-only ops on allowlisted paths; no destructive or network-exfiltration commands; output scrubbed. **Default for all backends.** |
| `standard` | Allowlist applies; anything outside it goes to `ask` (approval prompt). |
| `full` | User-confirmed blanket trust for the current session only; never persisted as a permanent state. |

**Approval scope (when a decision is `ask`):**

| Scope | Meaning |
|---|---|
| `once` | Runs this time only; the next occurrence asks again. |
| `session` | Runs without asking for the rest of the current session; expires at close, never persisted. |
| `always` | Persists as an allowlist rule in `policy.yaml`; revocable anytime; recorded in the audit log. |
| `deny` | Blocks the action and records the decision. |

`always` is the only scope that writes persistent policy. Gateway surfaces never offer `once`/`session` prompts: they auto-deny above `sandbox` unless a persistent `always` rule exists.

## Policy file

`~/.agis/policy.yaml`, managed via CLI (never hand-edited as the primary path). Rules organized by category:

- `commands` — shell patterns allowlisted/denylisted per backend (`git`, `ls`, `curl`, `rm`, …)
- `files` — readable/writable paths per backend (e.g. `~/Documents` read, `~/.ssh` denied)
- `network` — host/domain allow/deny for fetch and curl
- `backends` — which backend is available for what (`local`, `docker`, `ssh`)

## CLI

```
agis policy init             # create policy.yaml with safe defaults (tier: sandbox)
agis policy set <category> <pattern> <allow|deny>
agis policy rm  <category> <pattern>
agis policy show             # print effective policy
agis policy tier <backend> <sandbox|standard|full>
agis policy test <command>   # dry-run: would this be allowed/denied/asked?
```

## TUI panel

`/permisos` (GAIA convention): browse rules by category, toggle allow/deny, change posture per backend, preview the effective decision for a command, review the audit log, and revoke `always` grants.

## Decision flow

1. A tool call arrives at Policy Guard from any surface.
2. Guard evaluates: baseline posture → category rules → allowlist/denylist → explicit user overrides.
3. Result: `allow` (execute), `deny` (block, log), or `ask` (approval prompt with scope choices in TUI; auto-deny in gateway surfaces).
4. A granted `ask` resolves by scope: once, session, or always.
5. Every decision and grant is recorded in the audit log.

## Trust boundaries

- Rule changes require the user (CLI/TUI), never the model. The model cannot grant itself permissions.
- Posture escalation (`sandbox` → `standard` → `full`) requires explicit user confirmation, is per-session, and expires at session end.
- `always` grants are the only persistent approvals; they are visible, revocable, and auditable.
- The policy file is user-owned, plain text, and versionable.

Backends (`local` shell/fs, `docker`, `ssh`) are implemented in `internal/tools` (`local.go`, `backends.go`, `registry.go`); the `Tool` port lives in `internal/core/port_learning.go:18`.
