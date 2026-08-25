# Proposal: M3 — Skills & persona

## Intent

AGIS remembers (M2) but has no procedural memory and no durable identity. Every session starts from scratch on HOW to do things, and the agent's voice is whatever the model defaults to. M3 adds the skill hub (agentskills.io-compatible procedural memory, including agent-created skills) and the persona system (SOUL.md identity + session overlays + evolution from learning-loop observations), closing spec §4 and §8.

## Scope

### In Scope

- Migration 0003: `skills` table (id, name, description, trigger, content, source, usage_count, last_used).
- Repository port extension: save/list/get skills, record usage.
- `internal/skills`: agentskills.io frontmatter loader (strict validation), hub index + trigger/description matcher, `.atl/skill-registry.md` writer (default `$AGIS_HOME/.atl/`), close-time skill extraction via LLM (kill-switchable).
- `internal/persona`: SOUL.md seed-on-first-run / load / fallback / injection scan (fixed pattern list); `/personality` overlays (built-in presets + config `agent.personalities`); evolution as a derived overlay from observations (`evolution_enabled: false` freezes); never rewrites SOUL.md.
- Brain integration: SOUL.md as system slot #1 (before recall); matched skills injected as context; skill extraction wired into CloseSession.
- TUI: minimal exact-match slash-command dispatcher (`/personality`, `/persona freeze|reset|status`).
- Config: `agent.personalities`, `evolution_enabled`, `skills.enabled/dir`; docs; README roadmap-table fix (M2 still says "planned").

### Out of Scope

- Skill editing UI, autocomplete, skill marketplace/sync (M5+ surface work).
- Embeddings/vector matching for skills (FTS-style matching only).
- Rewriting SOUL.md from evolution (explicitly rejected — evolution is derived state).
- Gateway/cron surfaces.

## Capabilities

### New Capabilities
- `skill-hub`: agentskills.io loading, index/matching, registry persistence, close-time creation, usage tracking.
- `persona`: SOUL.md identity lifecycle, persona overlays, derived evolution layer.

### Modified Capabilities
- `repository-memory`: port gains skill methods; migration 0003 adds `skills`.
- `brain-loop`: identity slot #1, matched-skills context injection, close-time skill extraction step.
- `minimal-tui`: slash-command dispatcher with `/personality` and `/persona` commands.
- `config-loader`: new `agent` block (personalities, evolution_enabled) and `skills` block.

## Approach

Reuse proven M2 patterns end to end: LLM→fenced-JSON→validate→persist with log-and-skip (curator template) for close-time skill creation; pre-filled-defaults config overlay for new blocks; consumer-side ports in core to keep the hexagonal dependency rule; options-based Brain/TUI construction. Skills match by FTS over name/trigger/description against the current user input; identity is loaded once at startup, overlays are per-session state owned by the Model.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/memory/` | Modified | Port methods + migration 0003 |
| `internal/skills/` | New | Loader, hub, creator, registry |
| `internal/persona/` | New | SOUL.md, overlays, evolution |
| `internal/core/brain.go` | Modified | Context assembly slots, close hook extension |
| `internal/adapters/tui/app.go` | Modified | Slash-command dispatch |
| `cmd/agis/main.go`, `internal/config/` | Modified | Wiring + new blocks |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Injection patterns in imported skills/SOUL.md | Medium | Fixed scan pattern list before inclusion; strict frontmatter validation |
| Close latency grows (second LLM call) | Medium | Same bounded timeout; `skills.enabled` kill switch |
| Registry collides with tooling caches | Low | Default `$AGIS_HOME/.atl/`, configurable |
| Evolution quality | Medium | Derived overlay reads curated observations only; freeze switch |

## Rollback Plan

Chained stacked PRs; each merges green and reverts cleanly (`git revert -m 1`). Migration 0003 is additive-only (new table); schema v2 binaries ignore it. Feature switches (`skills.enabled`, `evolution_enabled`) neutralize behavior without revert.

## Dependencies

- None external. gopkg.in/yaml.v3 covers frontmatter parsing.

## Success Criteria

- [ ] A skill dropped into the skills dir loads, indexes, matches relevant input, and appears in the registry file.
- [ ] A session whose conversation produced a reusable procedure ends with an agent-created skill persisted (when enabled).
- [ ] First run seeds `$AGIS_HOME/SOUL.md`; deleting it reseeds; custom text survives upgrades.
- [ ] `/personality concise` changes the assistant voice next turn; `/persona status` reports state; freeze disables evolution.
- [ ] Full suite green under goleak; all slice reviews approved.

## Proposal question round

Round held with the owner: close-time skill extraction confirmed (vs manual/import-only). Remaining assumptions taken from spec §4/§8 verbatim: registry at instance-scoped `.atl/`, evolution never mutates SOUL.md, overlays are session-scoped only.
