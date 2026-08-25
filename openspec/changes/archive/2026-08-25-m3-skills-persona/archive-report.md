# Archive Report: m3-skills-persona

**Archived:** 2026-08-25
**Delivered as:** 4 stacked PRs merged to main — PR #9 (skills substrate) → #10 (skill hub) → #11 (persona) → #12 (integration: brain slots, close-time creation, slash commands, wiring).
**Baseline at start:** main `e082161` (M2 archived). **Final:** main `82686d3`.

## Shipped

- Skills substrate (REPO-001/002): `core.Skill` type, migration 0003 (`skills` table with UNIQUE name, quoted `"trigger"` column, source CHECK), port methods `SaveSkill`/`ListSkills`/`RecordSkillUsage`, plus M3 additions `UserModelRows` and `ClearUserModel` (derived-data-only deletion).
- Skill hub (SKL-001..005): agentskills.io frontmatter loading with strict validation and skip-and-log; AND-term matching over name/trigger/description with stop-word filtering so natural-language inputs reach keywords; usage tracking; atomic `.atl/skill-registry.md` regeneration in `$AGIS_HOME`.
- Close-time creation (SKL-004, BRN-003): one bounded LLM call after the summarizer distills reusable procedures into agent-sourced skills; malformed answers log-and-skip; `skill` session events recorded; `skills.enabled` kill switch.
- Persona (PER-001..004): SOUL.md seeded from an embedded default (0600) on first run, user edits preserved, fallback on empty/unreadable; injection scanning shared via `internal/scan`; built-in + config personality overlays with clearing aliases and typed unknown-name errors; derived evolution layer from top-5 user-model rows with `/persona freeze|reset|status`. Evolution never rewrites SOUL.md.
- Brain integration (BRN-001): context assembly in fixed order — composed identity → matched skills → recall observations — empty layers omitted; consumer-side ports (`SkillHub`, `EvolutionLayer`, `SkillCreator`) keep core adapter-free.
- TUI (TUI-001): exact-match slash-command dispatcher for `/personality` and `/persona`; feedback rendered as prefixed viewport lines; commands never reach the provider nor persist as messages.
- Config (CONF-001): `agent.personalities`, `agent.evolution_enabled`, `skills.enabled`, `skills.dir` with explicit-false survival semantics.

## Verification

Independent bounded reviews (gentle-ai review-integration/v2, reliability lens):

- PR1 lineage `review-09b3e6b68f79e7ec`: **approved**, 1 informational suggestion (RowsAffected error path in RecordSkillUsage).
- PR2 lineage `review-c607a297ce981f99`: **approved**, zero findings. Stop-word filtering added mid-slice after tests proved raw AND semantics could not match natural-language input.
- PR3 lineage `review-03fad061ca0440f5`: **approved**, zero findings. An earlier iteration was abandoned after inspection caught seeding failures propagating a non-nil error; all failure paths now warn-and-fallback.
- PR4 lineage `review-160769e2bb52929f`: **approved**, zero findings.

All pre-pr gates allow. Final verification per slice on a clean tree: `go build ./...`, `go vet ./...`, `go test ./...` green under goleak. A staging miss that briefly broke main after the PR1 merge was fixed immediately (`d3d8014`) and led to a clean-tree-before-evidence process rule adopted for every later slice.

## Spec sync

NEW capabilities: `openspec/specs/{skill-hub,persona}/spec.md`. MODIFIED capabilities appended: `repository-memory`, `brain-loop`, `minimal-tui`, `config-loader`.

## Amendment note

SKL-002's matching requirement gained stop-word filtering during implementation (tests proved raw whitespace-split AND could never match phrased inputs like "how do I deploy this"). The synced spec text reflects the implemented stop-word-filtered AND semantics.
