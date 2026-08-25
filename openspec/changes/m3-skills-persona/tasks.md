# Tasks: m3-skills-persona

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1700–1900 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 substrate → PR2 skills pkg → PR3 persona+config → PR4 integration+wiring |
| Delivery strategy | auto-chain (owner precedent M1/M2: stacked-to-main) |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

| Unit (PR) | Focused test | Runtime harness | Rollback boundary |
|-----------|--------------|-----------------|-------------------|
| PR1 | `go test ./internal/memory/...` | N/A — library slice; binary smoke deferred to PR4 harness | migration+port revert independently |
| PR2 | `go test ./internal/skills/...` | N/A — library slice | new package, no consumers yet |
| PR3 | `go test ./internal/persona/... ./internal/config/...` | N/A — library slice | new package + additive config |
| PR4 | `go test ./...` | `./bin/agis`: identity prompt, `/personality`, `/persona status`, quit-close creates skill | `skills.enabled=false` + `agent.evolution_enabled=false` neutralize without revert |

## Phase 1: Substrate (PR1)

- [x] T1.1 Create `internal/core/skill.go`: `Skill` domain type (`ID`,`Name`,`Description`,`Trigger`,`Content`,`Source`,`UsageCount`,`LastUsed`,`CreatedAt`), source constants imported/agent. AC: REPO-001 types
- [x] T1.2 Write `internal/memory/migrations/0003_skills.sql` (UNIQUE name, source CHECK, usage_count DEFAULT 0); register in embedded migrations. Test: v2→v3 applies once, re-run no-op. AC: REPO-002
- [x] T1.3 Extend `core.Repository` + SQLite impl: `SaveSkill` (upsert by name, preserve created_at), `ListSkills` (last_used DESC, name ASC), `RecordSkillUsage`. Tests: upsert, usage bump ×2, ordering. AC: REPO-001
- [x] T1.4 Update test doubles (`memory/fakes_test.go`, `tui/app_test.go` fakeRepo) to implement new methods

## Phase 2: Skills package (PR2)

- [x] T2.1 Create `internal/scan/scan.go`: fixed lowercase substring pattern list; `ScanLines(text) (clean string, dropped int)`; RED tests first (injection patterns dropped, benign intact). AC: PER-002 shared engine
- [x] T2.2 Create `internal/skills/loader.go`: YAML frontmatter decode (`name`,`description` required; `trigger` optional), invalid files skipped+logged. Tests: valid loads, missing-name skipped, empty dir OK. AC: SKL-001
- [x] T2.3 Create `internal/skills/hub.go`: index by name/trigger/description; `Match` AND-term case-insensitive top-N (default 3); `RecordUse` via repo. Tests: trigger match, no-match, limit. AC: SKL-002/003
- [x] T2.4 Create `internal/skills/creator.go`: one bounded Chat call post-summarizer returning fenced JSON `{name,description,trigger,content}` or null; malformed log-and-skip; persists source=agent + records `skill` event; honors enabled flag. Tests: captured/malformed/disabled/error-nonfatal (curator test style). AC: SKL-004, BRN-003
- [x] T2.5 Create `internal/skills/registry.go`: atomic tmp+rename writer listing indexed skills; failure warns only. Tests: reflects state, unwritable path non-fatal. AC: SKL-005
- [x] T2.6 Hub startup sync: loaded file skills upsert to repo as source=imported. Test: import persists. AC: SKL-003

## Phase 3: Persona + config (PR3)

- [x] T3.1 Config: `agent.personalities` map, `agent.evolution_enabled` (default true, false survives), `skills.enabled`/`skills.dir` (defaults true / `$AGIS_HOME/skills`). Tests: defaults, partial overlay, explicit off survives. AC: CONF-001
- [x] T3.2 Create `internal/persona/soul.go`: `//go:embed` default SOUL template; seed-if-missing at 0600, never overwrite, fallback on empty/unreadable, read only from `$AGIS_HOME`; run through `scan.ScanLines`. Tests: seeds/preserves/falls-back/drops-injected. AC: PER-001/002
- [x] T3.3 Create `internal/persona/overlay.go`: built-in presets (concise, teacher, technical, creative) + custom from config map; resolve(name) error on unknown; none/default/neutral clears. Tests: preset/custom/unknown/clear. AC: PER-003
- [x] T3.4 Create `internal/persona/evolution.go`: assemble layer from top-5 `user_model` rows by confidence; `Freeze()` excludes; `Reset(ctx)` deletes user_model rows via repo; `Status()`. Tests: participates/frozen/reset/status. AC: PER-004

## Phase 4: Integration (PR4)

- [x] T4.1 Core: `SkillHub` consumer-side port, `WithIdentity`, `WithSkills`, `WithSkillCreator`, `SetOverlay`; compose identity text = SOUL + overlay + evolution at Step start; prepend system slots identity→skills(matched, RecordUse)→recall, omit empties. Tests: slot order full stack + bare minimum. AC: BRN-001
- [x] T4.2 Wire creator into `CloseSession` after summarizer, same timeout, non-fatal, kill switch respected. Tests: created skill + event; extractor error continues. AC: BRN-003/SKL-004
- [x] T4.3 TUI slash dispatcher in `app.go`: exact-match first token; `/personality <name|none>` calls overlay resolver; `/persona freeze|reset|status` drives evolution; feedback lines prefixed `· `, never persisted/sent; unknown → error line. Tests through drive helpers. AC: TUI-001
- [x] T4.4 Wire `cmd/agis/main.go`: load persona + hub + registry sync at startup, options into Brain/TUI. Manual harness: seeded SOUL on first run, `/persona status`, quit-close extraction
- [x] T4.5 Docs: create `docs/skills.md` + `docs/persona.md`; update `docs/configuration.md` (agent/skills blocks), README roadmap table (M2 DONE, M3 DONE), roadmap.md M3 section
- [x] T4.6 Full suite green: `go build ./...`, `go vet ./...`, `go test ./...` under goleak

## Dependency Ordering

T1.x → T2.x → {T3.x ∥ nothing} → T4.x. Sequential slices; PR2/PR3 depend on PR1 port; PR4 depends on all. Threat matrix: no applicable rows (design §Threat Matrix N/A); PER-002 RED tests carried in T2.1/T3.2.
