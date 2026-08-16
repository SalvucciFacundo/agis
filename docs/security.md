# Security

> **DESIGNED, PARTIALLY IMPLEMENTED.** The threat model, ten defenses, and four invariants below are the security design from `spec.md` §9. In M1 the **only** live security control is the config-file 0600 check; everything else — Policy Guard, sandbox tiers, approval prompts, untrusted-input tagging, audit log — is designed for M4+ and is not yet enforced.

## Why security is designed in first

A general-purpose agent that executes tools on the host is a high-value target: one prompt-injected skill, one malicious webpage read into context, and the model can be steered into running arbitrary commands. Security is therefore designed from the first milestone, not bolted on — but tool execution itself is not until M4, so most controls are still ahead of their enforcement points.

## Threat model

| Threat | Description |
|---|---|
| **Prompt injection** | Malicious instructions inside tool output, web content, skills, or observations. Assumed active on every untrusted input. |
| **Tool abuse** | The model executing destructive or exfiltrating commands (`rm`, `curl` to an attacker, reading secrets). |
| **Data exposure** | Secrets, keys, personal files leaking into prompts, tool output, memory, or logs. |
| **Gateway abuse** | Unauthorized users reaching the agent through a messaging platform (DM pairing, allowed-user allowlist). |
| **Memory poisoning** | Attackers writing malicious observations/skills that later steer the agent (injection via the memory layer). |

## The ten defenses

1. **Policy Guard is the single enforcement point for every tool call** — never bypassed by an adapter; every invocation from any surface goes through it.
2. **Tool allowlist/denylist** — patterns approved per backend; dangerous operations (`rm -rf`, raw network exfiltration, credential reads) denied by default or requiring interactive approval.
3. **Sandbox tiers per backend** — `sandbox` (no destructive ops, scrubbed output), `standard` (allowlist + approval), `full` (user-confirmed blanket trust for a session). Default tier is `sandbox`; escalation is explicit and per-session.
4. **Interactive approval in the TUI** — the exact command is shown and approved before execution above the tier; gateway surfaces default to deny above `sandbox`.
5. **Untrusted input tagging** — tool output, fetched content, and observations are tagged as data, not instructions; injection attempts in tool output are surfaced to the user.
6. **Backend isolation** — local = policy-filtered host, Docker = container boundary, SSH = remote boundary; high-risk workloads default to Docker/SSH, never bare local.
7. **Secrets management** — API keys stored with restrictive permissions (0600), never logged, never in observations, masked when echoed in tool output.
8. **Memory hygiene** — observations and skills are scanned for embedded instruction-like content on import; the user can audit/delete any memory entry.
9. **Gateway security** — DM pairing, allowed-user allowlist, gateway sessions inherit the default `sandbox` tier.
10. **Audit log** — every tool call, approval, and policy decision (who/what/when/outcome) recorded in SQLite for review.

## Security invariants

- No adapter, skill, or surface may call a tool without passing Policy Guard.
- No secret value ever enters the prompt, memory, or logs.
- The default tier is always the most restrictive; trust is granted explicitly, per session, never inferred.
- **Deny is the default answer** for anything the policy does not explicitly allow.

## Live vs designed controls

| Control | M1 status |
|---|---|
| Config file 0600 check | **live** (`internal/config/config.go:141`) |
| API key never logged | **live** by construction — the key is only read into config |
| Policy Guard / allowlists / tiers | designed (M4) |
| Interactive approval prompts | designed (M4) |
| Untrusted input tagging | designed (M4) |
| Backend isolation (local/docker/ssh) | designed (M4) |
| Memory hygiene scanning | designed (M3 imports) |
| Gateway pairing & allowlist | designed (M6) |
| Audit log | designed (M4) |

## Design notes

- **Untrusted input tagging** (#5) works by labeling tool output, fetched content, and observations as data in the prompt, with explicit instructions that they are not commands; suspected injection attempts are surfaced to the user rather than silently ignored.
- **Deny-by-default is the load-bearing invariant.** Every other control is a mechanism to relax it explicitly and temporarily. Because it has no enforcement point before M4, there is currently no code path that executes anything — the absence of tool execution is itself the M1 safeguard.
- **Secrets never cross the prompt/memory/log boundary** — this is why `api_key` is deliberately excluded from the config-defaulting logic (`internal/config/config.go:101`): an empty key is a valid state, and the value flows only from config to the HTTP `Authorization` header.

## Implemented today (M1)

Only defense #7's permission check is live:

- The config loader warns when `~/.agis/config.yaml` is looser than `0600` (`internal/config/config.go:141`). It warns, it does not refuse.
- The config file is the only place an API key is read; it is never logged by the app.

Everything else requires tool execution and the Policy Guard, both M4 scope. Until then, the M1 agent only chats — it runs no tools, so there is no execution surface to abuse.
