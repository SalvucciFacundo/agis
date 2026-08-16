# AGIS — Autonomous Go Intelligent System

A general-purpose autonomous agent written in Go. Inspired by the functional scope of Hermes Agent (Nous Research) and the Go architecture of GAIA, engineered for resource efficiency: a single static binary, zero external services at runtime, local-first persistence.

## Vision

AGIS is a self-improving general assistant that runs anywhere — from a $5 VPS to a laptop — using minimal resources. It carries a persistent, curated memory across sessions, learns skills from experience, builds a model of its user, and executes tools on local and remote backends under a strict permission policy. It is not tied to a specific provider: the same binary talks to OpenAI, Anthropic, OpenRouter, or local models via Ollama.

## Goals

- **General purpose**: a personal assistant for everyday tasks, not a coding-only agent (unlike GAIA).
- **Resource efficiency**: single Go binary, SQLite persistence, no mandatory external services, low idle cost.
- **Closed learning loop**: agent-curated memory, session summarization, skill creation, user modeling — the pattern proven by Hermes and already validated in GAIA.
- **Provider-agnostic**: multi-provider LLM support behind one common port.
- **Multi-surface**: TUI first; messaging gateway (Telegram, Discord, Slack, WhatsApp, Signal, Email) as a later surface with the same core.
- **Safe by default**: explicit permission policy for tool execution; sandboxed local and remote backends.

## Non-Goals (v1)

- No coding-agent specialization (no SDD, no code review protocol — that is GAIA's domain).
- No GPU training, no fine-tuning.
- No managed memory service; no vector database server. Semantic search, if ever needed, is a bolt-on behind the Repository port.
- No web UI in v1.

## Architecture

Hexagonal (ports & adapters), mirroring the structure that proved sound in GAIA.

```
┌──────────────────────────────────────────────────────┐
│                      Surfaces                        │
│   TUI (Bubbletea)   Gateway*   MCP Server*   Cron*  │
└───────────────────────┬──────────────────────────────┘
                        │
┌───────────────────────▼──────────────────────────────┐
│                     Agent Core                       │
│   Brain Loop    Memory Curator    Skill Hub          │
│   User Model    Session Manager   Planner            │
└─────────┬──────────────────┬─────────────────────────┘
          │                  │
┌─────────▼─────────┐  ┌─────▼─────────────────────────┐
│    LLM Port       │  │       Tool Port               │
│  OpenAI adapter   │  │  Local (shell, fs) adapter    │
│  Anthropic        │  │  Docker adapter               │
│  OpenRouter       │  │  SSH adapter                  │
│  Ollama           │  │  Policy / approval guard      │
└───────────────────┘  └───────────────────────────────┘
          │
┌─────────▼────────────────────────────────────────────┐
│                 Memory (SQLite + FTS5)               │
│  conversations  observations  skills  user_model     │
└──────────────────────────────────────────────────────┘
```

(*) Gateway, MCP Server and Cron are defined in the spec; the TUI is the v1 surface.

## Repository layout

```
cmd/agis/            — entry points (tui, doctor, model, config)
internal/core        — domain logic, ports, kernel, brain loop
internal/adapters    — TUI (Bubbletea), LLM clients, backends
internal/memory      — memory store, curator, summarizer, user model
internal/skills      — skill hub, agentskills.io compatible loader
internal/tools       — tool registry, policy guard
internal/policy      — permission system: policy model, CLI, TUI panel, audit
internal/persona     — SOUL.md loader, persona overlays, evolution
internal/gateway     — messaging gateway (Telegram, Discord, Slack, WhatsApp, Signal, Email)
internal/mcp         — MCP server (future)
internal/cron        — scheduled automations
internal/plugins     — plugin manager (future)
internal/webhook     — webhook listener (future)
pkg/                 — shared packages
```

## Core components

### 1. Brain loop

The central loop of the agent, modeled after Hermes/GAIA:

1. Receive input (user message or scheduled task).
2. Load relevant context: current conversation, related observations (FTS5 search + topic-key lookup), user model, matching skills.
3. Build the prompt with context + tool availability.
4. Call the LLM; stream the response to the TUI.
5. If the model requests a tool: route to the Tool Port through the Policy Guard, execute, feed the result back.
6. At session end: curator evaluates what to persist, nudge prompts for memory writes, summarizer compresses the session.

### 2. LLM provider port

```
type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    Stream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
    Models() []ModelInfo
}

type StreamEvent struct {
    Text string
    Err  error
}
```

`Stream` returns a `StreamEvent` channel (amended from a bare `<-chan Token` in M1) so mid-stream failures surface on the channel instead of requiring a second error path. The shared OpenAI-compatible client serves both the OpenAI and Ollama adapters in M1; Anthropic and OpenRouter adapters follow later. Provider and model selected via `agis model` / config; no code changes to switch.

### 3. Memory system

The full learning loop in v1, following the GAIA pattern with a generalized schema.

#### Storage

- SQLite via `modernc.org/sqlite` (pure Go, no cgo, single file).
- FTS5 for full-text search over conversations and observations.
- A single `Repository` port; the schema is generic, not coding-oriented.
- Migrations are embedded in the binary (`//go:embed migrations/*.sql`) and applied via `PRAGMA user_version` — the single-static-binary, zero-external-services goal keeps the schema self-contained (deviation from an external migration tool, deliberate for a single-writer embedded DB).

#### Schema

```
conversations        — id, title, created_at, updated_at, summary, message_count
messages             — id, conversation_id, role, content, created_at
observations         — id, topic_key, type, content, importance, created_at, source_ref
memory_fts           — standalone FTS5 table (doc_type, doc_id, content) over messages + observations; tokenizer unicode61 remove_diacritics 1
skills               — id, name, description, trigger, content, source, usage_count, last_used
user_model           — id, key, value, confidence, updated_at
session_events       — id, session_id, kind, payload, created_at   (nudges, summaries, skill creations)
```

`memory_fts` is a standalone FTS5 table with a `doc_type` discriminator (`message` | `observation`) rather than FTS5 external-content mode, which binds to a single base table. Full-text rows are synced in the same transaction as the base write (no hidden triggers).

#### The loop

1. **Agent-curated memory**: the agent decides what to persist as observations, prompted by periodic nudges. Each observation carries a `topic_key` (stable key for evolving topics, e.g. `user/preferences/coffee`) and an importance score. Same-topic observations update the existing row instead of duplicating — the upsert model used by Engram.
2. **Cross-session recall**: at conversation start, FTS5 search over past sessions + LLM summarization produces a context digest. `topic_key` lookup retrieves evolving facts.
3. **Session summarization**: at session end, the LLM compresses the session into a summary stored on the conversation and a set of candidate observations.
4. **Skills as procedural memory**: after complex tasks, the agent creates a skill (procedural memory). Skills self-improve during use (usage_count, refinement).
5. **User model**: lightweight dialectic user modeling (inspired by Honcho): observations about the user are periodically aggregated into `user_model` rows with confidence, updated as new evidence arrives.

#### Why not a memory framework

Frameworks (Mem0, Zep, Letta) add servers, vector stores, and network dependencies — the opposite of the efficiency goal. Hermes itself uses SQLite + FTS5 + LLM summarization. The Repository port keeps the option to bolt on embeddings (e.g. `sqlite-vec`) later without touching domain logic.

### 4. Skills system

- Skills are procedural memory: plain-text instructions (Markdown with frontmatter) loaded from a skills directory, compatible with the agentskills.io open standard.
- The Skill Hub indexes by name + trigger + description, matching the agentskills.io metadata conventions.
- Skills are created by the agent after complex tasks, and can be imported by the user.
- Skill registry persisted to `.atl/skill-registry.md` (same convention as GAIA).

### 5. Tool execution & backends

```
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, args Args, policy PolicyGuard) (Result, error)
}
```

Backends in v1:

- **Local**: shell commands and filesystem operations on the host. Policy guard approves/denies per pattern.
- **Docker**: isolated container backend for higher-risk execution.
- **SSH**: remote backend for VPS/cloud execution, mirroring Hermes's architecture.

Every tool call passes through the Policy Guard: allowlist patterns, interactive approval in the TUI when needed, and a sandbox tier (`sandbox` / `standard` / `full`) configured per backend.

### 6. TUI (v1 surface)

Bubbletea-based terminal UI, modeled on GAIA's TUI: multiline editing, streaming output, slash-command autocomplete, session list/restore, interrupt-and-redirect, tool execution feedback with approval prompts.

### 7. Session management

A session is a bounded conversation: it has an identity, a lifespan, and produces its own summary and observations at close. The Session Manager owns session lifecycle, independent of the surface (TUI, gateway, cron all attach to sessions).

#### Commands (TUI slash commands, GAIA/Hermes convention)

- `/new` or `/reset` — start a fresh session (closing the current one).
- `/save` — persist the current session explicitly.
- `/list` — browse recent sessions (id, title, created_at).
- `/restore <id>` — load a previous session and continue from its summary + last messages.
- `/compress` — run the session summarizer early, freeing context.
- `/snapshot` — capture a point-in-time snapshot of the session state.
- `/rename <title>` — title the session for later discovery.

#### Lifecycle

1. **Start**: session created; cross-session recall loads relevant observations and a context digest.
2. **Run**: messages and tool activity are persisted incrementally to SQLite (crash-safe).
3. **Close** (`/new`, `/save`, app exit, timeout): the curator evaluates pending observations, the summarizer compresses the session into `conversations.summary`, and session-scoped permission grants (`session` scope) are discarded.
4. **Restore**: `/restore` reloads the summary + tail of messages; the full history stays queryable via FTS5.

#### Session-scoped permission grants

The `session` approval scope from the Permission system lives here: grants are held in memory for the active session, expire at close, and are never persisted.

### 8. Identity & persona system

The agent has a durable identity and per-session persona overlays. Modeled on Hermes SOUL.md + `/personality`, improved with the GAIA "seed + evolution" model: a persona is a starting point that evolves through experience, not a fixed cage.

#### SOUL.md (durable identity)

- `~/.agis/SOUL.md` is the agent's primary identity, occupying slot #1 of the system prompt — the same convention as Hermes.
- Loaded only from the AGIS home directory, never from the working directory: the identity belongs to the agent instance, not to whatever project you launch it from.
- Contains durable voice and personality guidance (tone, directness, style, how to handle disagreement), not task or project instructions (those belong in AGENTS.md).
- Seeded automatically on first run; user files are never overwritten; empty/unreadable files fall back to a built-in default identity.
- Scanned for prompt-injection patterns before inclusion, like any context-bearing file.

#### Persona overlays (`/personality`)

- Session-level system-prompt overlays, exactly like Hermes: `SOUL.md` is the baseline voice, `/personality <name>` is a temporary mode switch for a specific conversation (teacher, concise, technical, creative, custom).
- Built-in presets ship with the binary; custom presets live in `~/.agis/config.yaml` under `agent.personalities`.
- `/personality none` (or `default`/`neutral`) clears the overlay and returns to the SOUL.md baseline.

#### Seed + evolution (GAIA model)

- Each persona is a **seed, not a cage**: it starts from a base personality and evolves through the learning loop as the agent accumulates observations about how this user responds best.
- `evolution_enabled: false` freezes a persona into SOUL.md-traditional mode (static identity, no learning).
- Commands: `/persona freeze`, `/persona reset`, `/persona status`.
- The learning loop tracks communication patterns, feedback style, and user reactions to drive the evolution.

### 9. Gateway (architecture-ready, later milestone)

Single gateway process that exposes the **same Agent Core** to Telegram, Discord, Slack, WhatsApp, Signal, and Email — the full Hermes platform set. Each platform is a separate adapter behind one Gateway port. Talking to AGIS from the TUI or from Telegram is the same agent, the same memory, and the same sessions — the surfaces are interchangeable front-ends, never parallel agent paths. Defined now so the core never depends on the surface; implemented in a later milestone.

### 10. Cron

Scheduled automations with natural-language scheduling, delivering results to the active surface (TUI notifications first, gateway later).

### 11. Permission system

The user-facing layer over Policy Guard (which is the enforcement point). Modeled on GAIA's `policy init` + `/permisos` panel, generalized for a general-purpose agent. Fully configurable by the user without editing YAML by hand.

Two orthogonal axes define a decision: the **baseline posture** (how restrictive the backend is by default) and the **approval scope** (what a granted approval means over time). This is the model proven by Hermes and Claude Code.

#### Policy model

- **Policy file** (`~/.agis/policy.yaml`, managed via CLI, never hand-edited as the primary path): rules organized by category.
- **Rule categories**:
  - `commands` — shell patterns allowlisted/denylisted per backend (`git`, `ls`, `curl`, `rm`, etc.).
  - `files` — readable/writable paths per backend (e.g. `~/Documents` read, `~/.ssh` denied).
  - `network` — host/domain allow/deny for fetch and curl.
  - `backends` — which backend is available for what (`local`, `docker`, `ssh`).

#### Baseline posture (per backend)

- `sandbox`: deny by default; read-only ops on allowlisted paths; no destructive or network-exfiltration commands; output scrubbed. Default for all backends.
- `standard`: allowlist applies; anything outside the allowlist goes to `ask` (approval prompt).
- `full`: user-confirmed blanket trust for the current session only; never persisted as a permanent state.

#### Approval scope (when a decision is `ask`)

When the guard returns `ask`, the user is shown the exact action and chooses its scope:

- **Allow once** (`permitir`): the action runs this time only; the next occurrence asks again.
- **Allow for session** (`sesión`): the action runs without asking for the rest of the current session; expires at session end and is never persisted.
- **Always allow** (`siempre`): persists as an allowlist rule in `policy.yaml`, revocable anytime via CLI or `/permisos`; recorded in the audit log.
- **Deny** (`no`): blocks the action and records the decision.

`always` is the only scope that writes persistent policy; `once` and `session` live in the in-memory session state. Gateway surfaces never offer `once`/`session` interactive prompts: they auto-deny above `sandbox` unless a persistent `always` rule exists.

#### CLI

```
agis policy init             # create policy.yaml with safe defaults (tier: sandbox)
agis policy set <category> <pattern> <allow|deny>
agis policy rm  <category> <pattern>
agis policy show             # print effective policy
agis policy tier <backend> <sandbox|standard|full>
agis policy test <command>   # dry-run: would this be allowed/denied/asked?
```

#### TUI panel

- `/permisos` panel (same convention as GAIA): browse rules by category, toggle allow/deny, change posture per backend, preview the effective decision for a command, review the audit log, and revoke `always` grants.

#### Decision flow

1. Tool call arrives at Policy Guard from any surface.
2. Guard evaluates: baseline posture → category rules → allowlist/denylist → explicit user overrides.
3. Result: `allow` (execute), `deny` (block, log), or `ask` (approval prompt with scope choices in TUI; auto-deny in gateway surfaces).
4. A granted `ask` resolves by scope: once (run now), session (remember in-memory until session end), or always (persist as allowlist rule).
5. Every decision and every grant is recorded in the audit log.

#### Trust boundaries

- Rule changes require the user (CLI/TUI), never the model. The model cannot grant itself permissions.
- Posture escalation (`sandbox` → `standard` → `full`) requires explicit user confirmation, is per-session, and expires at session end.
- `always` grants are the only persistent approvals; they are visible, revocable, and auditable.
- Policy file is user-owned, plain text, and versionable.

## Configuration

- YAML config file (`~/.agis/config.yaml`): providers, model, memory paths, backends, policy tiers.
- `agis config set/get`, `agis model` to switch provider/model at runtime.

## Security

A general-purpose agent that executes tools on the host is a high-value target: one prompt-injected skill, one malicious webpage read into context, and the model can be steered into running arbitrary commands. Security is therefore designed in from the first milestone, not bolted on.

### Threat model

- **Prompt injection**: malicious instructions arriving inside tool output, web content, skills, or observations. Assumed to be an active threat on every untrusted input.
- **Tool abuse**: the model executing destructive or exfiltrating commands (rm, curl to attacker, reading secrets).
- **Data exposure**: secrets, keys, personal files leaking into prompts, tool output, memory, or logs.
- **Gateway abuse**: unauthorized users reaching the agent through a messaging platform (DM pairing, allowed-user allowlist).
- **Memory poisoning**: attacker writing malicious observations/skills that later steer the agent (prompt injection via the memory layer).

### Defenses

1. **Policy Guard is the single enforcement point for every tool call** — never bypassed by an adapter. Every tool invocation, from any surface (TUI, gateway, cron), goes through it.
2. **Tool allowlist/denylist**: patterns approved per backend. Dangerous operations (rm -rf, raw network exfiltration, reading credential files) are denied by default or require explicit interactive approval.
3. **Sandbox tiers per backend**: `sandbox` (no destructive ops, output scrubbed), `standard` (allowlist + approval prompts), `full` (user-confirmed blanket trust for a session). Default tier is `sandbox`; escalation is explicit and per-session.
4. **Interactive approval in TUI**: when the model requests an action above its tier, the TUI shows the exact command and asks before executing. Gateway surfaces use a stricter default (deny-by-default above `sandbox`).
5. **Untrusted input tagging**: tool output, fetched content, and observations are tagged as untrusted; the prompt tells the model they are data, not instructions. Injection attempts in tool output are surfaced to the user.
6. **Backend isolation**: local = policy-filtered host, Docker = container boundary, SSH = remote boundary. High-risk workloads default to Docker/SSH, never bare local.
7. **Secrets management**: API keys stored in the config file with restrictive permissions (0600), never logged, never included in memory observations, masked in tool output when echoed.
8. **Memory hygiene**: observations and skills are scanned for embedded instruction-like content on import; user can audit/delete any memory entry.
9. **Gateway security**: DM pairing (user must pair a platform identity to their local profile), allowed-user allowlist, and gateway sessions inherit the default `sandbox` tier.
10. **Audit log**: every tool call, approval, and policy decision is recorded (who/what/when/outcome) in SQLite for review.

### Security invariants

- No adapter, skill, or surface may call a tool without passing Policy Guard.
- No secret value ever enters the prompt, memory, or logs.
- The default tier is always the most restrictive one; trust is granted explicitly, per session, never inferred.
- Deny is the default answer for anything the policy does not explicitly allow.

## Milestones

- **M1 — Thinking agent with memory**: ~~Go skeleton, hexagonal layout, Brain loop, multi-provider LLM port (OpenAI + Ollama first), SQLite + FTS5 storage, session persist/restore, minimal TUI.~~ **DONE** — M1 shipped (change `m1-skeleton`, archived `2026-08-15`): hexagonal skeleton, `Brain.Step` loop, OpenAI + Ollama `Provider` adapters over a shared OpenAI-compatible client, SQLite + FTS5 `memory_fts` Repository with embedded migrations, config loader, minimal Bubbletea TUI. Verified 9/9 requirements, 11/11 scenarios, 47 tests green; delivered as 4 stacked PRs. Follow-ups deferred to M2: FTS delete sync, stream cancel/abandon leak, multi-word phrase search, UUID tie-break, hand-rolled client vs pinned SDK, `tui.New` signature drift.
- **M2 — Learning loop**: curator + nudges, session summarization, topic-key observations, user model.
- **M3 — Skills & persona**: skill hub, agentskills.io loading, skill creation from experience, registry; SOUL.md loader, persona overlays (`/personality`), seed + evolution.
- **M4 — Tools, backends & permissions**: local tools with Policy Guard, Docker backend, SSH backend, `agis policy` CLI, `/permisos` TUI panel, interactive approval in TUI.
- **M5 — Full TUI**: streaming, slash commands, session browse, interrupt-and-redirect.
- **M6 — Gateway + cron + ecosystem**: Telegram/Discord gateway first (WhatsApp, Signal, Slack, Email adapters follow), scheduled automations, plugin manager, webhook listener.

## Relationship to existing projects

- **GAIA** (github.com/SalvucciFacundo/gaia): architectural DNA. Hexagonal layout, Bubbletea TUI, SQLite persistence, skill registry. AGIS is NOT a fork of GAIA: it is a general-purpose agent with its own codebase, memory DB, and no coding-specific machinery.
- **Hermes** (NousResearch/hermes-agent): functional reference. Learning loop, skills, multi-provider, multi-backend. AGIS reimplements that scope in Go with a fraction of the resource footprint.
