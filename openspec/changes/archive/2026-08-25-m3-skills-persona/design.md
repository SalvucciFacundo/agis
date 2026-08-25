# Design: m3-skills-persona

## Technical Approach

Two new adapter-side packages (`internal/skills`, `internal/persona`) behind two small consumer-side ports in `core`, mirroring the M2 Nudger/SessionCloser pattern that keeps the dependency rule intact. Persistence reuses the M2 upsert substrate; close-time skill creation reuses the curator's LLM→fenced-JSON→log-and-skip pipeline; new config blocks reuse the pre-filled-defaults overlay semantics.

## Architecture Decisions

| # | Decision | Alternatives | Rationale |
|---|----------|--------------|-----------|
| D1 | Core defines `SkillHub` + identity enters as plain text via `WithIdentity`; runtime overlay via `Brain.SetOverlay` | Import packages into core; pass full persona manager | No import cycle (same fix as M2 ports); TUI is single-goroutine across turns, so a plain field is race-free given Enter is blocked while streaming/closing |
| D2 | Context assembly: `identityMsg` → `skillsMsg` → `recallMsg` prepended to tail in that order, empty layers omitted | One merged system blob | Each layer independently testable; mirrors existing recall test assertions |
| D3 | Frontmatter: decode YAML frontmatter into a typed struct, require `name`+`description`, everything else optional-but-typed; invalid files skipped with warning | Strict KnownFields; regex parsing | Matches curator validation style; loud-but-non-fatal per SKL-001 |
| D4 | Matching: AND-term over haystack `name+trigger+description`, top-3, case-insensitive | FTS query against memory_fts; embeddings | In-memory, zero schema coupling; FTS row would conflate conversations with skill docs |
| D5 | Close-time creator: ONE `Chat` call after summarizer, fenced JSON `{name,description,trigger,content}` or literal `null`; malformed→warn+skip; success persists `source="agent"` + `RecordSessionEvent("skill", name)` | Reuse summarizer combined call; cadence nudge | Keeps SUM-001 contract untouched; bounded by same close timeout; kill switch `skills.enabled` |
| D6 | Evolution = derived layer: top-5 `user_model` rows by confidence rendered as a guidance block; `/persona freeze` hides layer; `/persona reset` DELETES `user_model` rows (derived cache — rebuildable from observations); `/persona status` reports mode+counts | Rewrite SOUL.md; marker columns | Honors PER-004 never-mutate rule; reset is cheap and reversible via re-aggregation |
| D7 | SOUL.md seeded from `//go:embed` default at 0600; loaded once at startup into `persona.Identity`; injection scan drops flagged lines per fixed lowercase substring list | Per-turn file read; NLP detection | Identity belongs to instance; scan is deterministic and testable |
| D8 | Registry regenerated atomically (tmp+rename) after load/creation; failures warn only | Append-only journal | GAIA-style human-readable index; idempotent |
| D9 | TUI dispatcher: input starting `/` routes exact-match first token to handler returning viewport feedback lines (prefixed `· `); never persisted, never sent | Bubbletea key prefix mode; autocomplete | Minimal surface ahead of M5; trivially testable through existing drive helpers |

## Data Flow

    main.go
      ├─ persona.Load(AGIS_HOME) ─→ soul text (+scan)
      ├─ skills.Load(dir) ─→ hub index ─→ registry sync ($AGIS_HOME/.atl/)
      └─ NewBrain(repo, provider,
             WithIdentity(soul), WithSkills(hub),
             WithNudger(curator), WithSessionCloser(summarizer),
             WithSkillCreator(creator))

    Step(): [identity][skills][recall] + tail ──→ provider
       │                          ↑ hub.Match(input) → RecordSkillUsage
    CloseSession(): summarizer.Close → creator.Extract → SaveSkill(agent)

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/memory/migrations/0003_skills.sql` | Create | `skills` table, UNIQUE name, source CHECK |
| `internal/core/skills.go` | Create | `SkillRef`, `SkillHub` port, options |
| `internal/core/brain.go` | Modify | context slots, SetOverlay, close hook order |
| `internal/skills/loader.go` | Create | frontmatter load/validate/scan-import |
| `internal/skills/hub.go` | Create | index, AND-match, usage recording |
| `internal/skills/creator.go` | Create | close-time LLM extraction |
| `internal/skills/registry.go` | Create | atomic registry writer |
| `internal/persona/{soul,overlay,evolution}.go` | Create | lifecycle, presets, derived layer, scan |
| `internal/adapters/tui/app.go` | Modify | slash dispatch + feedback lines |
| `cmd/agis/main.go` | Modify | wiring |
| `internal/config/config.go` | Modify | `agent` + `skills` blocks |
| Repository port + sqlite impl + fakes | Modify | skill methods |

## Interfaces / Contracts

```go
// core (consumer side)
type SkillRef struct { Name, Description, Trigger string }
type SkillHub interface {
    Match(ctx context.Context, input string, limit int) ([]SkillRef, error)
    RecordUse(ctx context.Context, name string) error
}
type SkillCreator interface {
    Extract(ctx context.Context, convID string, msgs []Message) (string, error)
}

// repository port additions
SaveSkill(ctx, Skill) error                       // upsert by name
ListSkills(ctx) ([]Skill, error)                  // last_used DESC, name ASC
RecordSkillUsage(ctx, name string) error          // usage_count++, last_used=now

// persona
type Overlay struct { Name, Text string }
func (m *Model) applyPersonality(name string) error  // TUI side
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | loader validation, matcher AND-semantics, scan patterns, registry writer, evolution assembly, config overlay | table-driven, temp dirs, yaml fixtures |
| Unit | migration 0003 idempotency, port upsert/usage | real SQLite (modernc) like M2 tests |
| Integration | close flow: summarizer → creator → SaveSkill + event; context slot ordering with fakes | fake provider capturing requests (M2 style) |
| E2E-ish | TUI slash commands through drive helpers; goleak held | existing app_test harness |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. Skills/SOUL are parsed text entering prompts; the injection-scan requirement (PER-002) covers that surface with RED tests in the unit layer.

## Migration / Rollout

Migration 0003 additive-only (`user_version` 2→3); v2 binaries ignore the table. Feature switches `skills.enabled` / `agent.evolution_enabled` neutralize behavior without code revert. PRs revert cleanly via `git revert -m 1`.

## Open Questions

- None blocking. Preset texts (built-in personalities) are copy, finalized during implementation review.
