# Archive Report: m1-skeleton (M1 — Thinking agent with memory)

- **Change**: `m1-skeleton` · **Repo**: `github.com/SalvucciFacundo/agis` · **Archived**: `2026-08-15`
- **Archived to**: `openspec/changes/archive/2026-08-15-m1-skeleton/`
- **Execution mode**: `auto` · **Artifact store**: `hybrid` (openspec files + Engram)
- **Verified HEAD**: `a35f351` (all 4 PRs merged to main)

## Final State

This is the terminal record of the M1 cycle. It reflects the state of the change AT CLOSE.

| Fact | Value | Authority |
|---|---|---|
| Task completion | 20/20 implementation tasks complete | Persisted `tasks.md` (all `[x]`) + native status `taskProgress{20,20,0,allComplete:true}` |
| Verification verdict | **PASS** — 9/9 requirements, 11/11 scenarios | `verify-report.md` (`gentle-ai.verify-result/v1`, `verdict: pass`, blockers 0, critical 0) |
| Tests | 47 tests passing, 5 packages, `go test -count=1 ./...` exit 0 | `verify-report.md` proof suite |
| Delivery | 4 stacked PRs merged to main, HEAD `a35f351` | git history (merge commits #1–#4) |
| Native review gate | `reviewGate` structurally absent — no native RDD review artifacts exist for this candidate (empty `review*` arrays, no `reviews/` dir, no `review/` topics read) | native status `sdd-status` |
| Review coverage | 4 lineages = the 4 merged GitHub PRs, each reviewed and merged to main | git history + task work-unit plan (PR1–PR4) |

### Source-artifact observation IDs read

Source artifacts were read from the filesystem (openspec paths), not from Engram topics, so no Engram observation IDs are applicable for the source read. Observation IDs recorded here are for traceability only:

- Read from filesystem: `openspec/changes/archive/2026-08-15-m1-skeleton/{proposal,spec,design,tasks,verify-report,exploration}.md`
- Archive report persisted to Engram: topic `sdd/m1-skeleton/archive-report`

## Specs Synced

The M1 delta spec (flat `spec.md`, 5 capabilities) was synced into the persistent spec structure `openspec/specs/` per the openspec convention. Because no `openspec/specs/` main spec existed before M1, each capability became a new main spec file:

| Domain | Action | Requirements |
|--------|--------|--------------|
| `openspec/specs/config-loader/spec.md` | Created | 1 (CONF-001) |
| `openspec/specs/repository-memory/spec.md` | Created | 4 (REPO-001..004) |
| `openspec/specs/llm-provider-port/spec.md` | Created | 2 (LLM-001..002) |
| `openspec/specs/brain-loop/spec.md` | Created | 1 (BRAIN-001) |
| `openspec/specs/minimal-tui/spec.md` | Created | 1 (TUI-001) |
| **Total** | 5 files created | 9 requirements, 11 scenarios |

### Main `spec.md` (source-of-truth) updated

- **§2 LLM provider port**: `Stream` signature amended from `(<-chan Token, error)` to `(<-chan StreamEvent, error)` with `StreamEvent{Text, Err}` — now REAL (was a planned amendment). Documented the shared OpenAI-compatible client serving OpenAI + Ollama in M1.
- **§3 Memory system**: `observation_fts` replaced with `memory_fts` standalone FTS5 table (`doc_type`, `doc_id`, `content`, tokenizer `unicode61 remove_diacritics 1`) with same-transaction sync — now REAL. Added embedded-migrations note.
- **Milestones**: M1 marked **DONE** with shipping summary (date, change, verification, delivery, deferred follow-ups).

## Archive Move — Mechanical Copy Readback

Change folder moved `openspec/changes/m1-skeleton/` → `openspec/changes/archive/2026-08-15-m1-skeleton/` via `git mv` (fallback `mv`). Recursive snapshot taken before move; `diff -r` against post-move tree:

```
diff exit status: 0   (empty output — no differences)
```

Verbatim `diff -r` output was empty, which is the only passing evidence of byte-identity. Five tracked files renamed `R100` (100% similarity). Archive contains all artifacts: `proposal.md`, `spec.md`, `design.md`, `tasks.md`, `verify-report.md`, `exploration.md`. Archived `tasks.md` has **zero** unchecked implementation tasks. Active `openspec/changes/` no longer contains this change.

## Deferred Follow-ups (→ M2)

Recorded from `verify-report.md` risks and the M1 close (intentional, out of scope for M1):

1. **FTS delete sync** — message/observation deletes currently do not remove their `memory_fts` rows (explicit-sync insert only; M1 has no delete path).
2. **Stream cancel/abandon leak** — a `Brain.Step` whose consumer abandons the stream mid-way could leave the provider goroutine running until the stream closes; needs cancel-path coverage in M2.
3. **Multi-word phrase search** — user queries are FTS5-escaped; multi-word phrase semantics across a single field to be revisited with observations in M2.
4. **UUID tie-break** — `LatestConversation` ordering keyed on `updated_at`; a tie needs a deterministic `id` tie-break.
5. **Hand-rolled client vs pinned SDK** — M1 hand-rolls the OpenAI-compatible HTTP+SSE client and uses stdlib `testing` instead of the proposal's `go-openai v1.42.0` / `testify v1.11.1` pins. SUGGESTION-level deviation, behaviorally covered by httptest SSE tests; decision deferred (keep hand-rolled or adopt the SDK).
6. **`tui.New` signature drift** — design shows `tui.New(brain, repo)`; implementation is `tui.New(brain, repo, stream)` (injected token channel). Cosmetic doc drift only.

## Scope Creep

None. No `internal/{skills,tools,policy,persona,gateway,mcp,cron,plugins,webhook}` or `pkg/`. Grep for M2+ concepts returned no matches. `internal/` contains only `adapters`, `config`, `core`, `memory`.

## Intentional-with-Warnings

None — archive proceeded without user-approved partial-archive or stale-checkbox reconciliation. All tasks were complete in the persisted artifact; no CRITICAL findings in `verify-report`.

## Next

M2 — Learning loop (curator + nudges, session summarizer, topic-key observations, user model).
