# Archive Report: m2-learning-loop

**Archived:** 2026-08-24
**Delivered as:** 4 stacked PRs merged to main — PR #5 (memory substrate) → #6 (PR2a curator/summarizer/user-model) → #7 (PR2b recall/nudge/CloseSession) → #8 (TUI close hook, memory config, wiring).
**Baseline at start:** main `211d00f` (M1 archived). **Final:** main `4be154e`.

## Shipped

- Repository substrate (M2-REPO-001..003): extended port (`SaveObservations`, `Observations`, `UpdateConversationSummary`, `UpsertUserModel`, `RecordSessionEvent`), multi-word AND FTS search, migration 0002 (`user_model`, `session_events`, `topic_key` UNIQUE, `updated_at` backfill).
- Curator (CUR-001..003): one `Provider.Chat` per nudge returning a JSON array of observations; fence-stripping parse; importance defaults to 3 via clamp; malformed responses log-and-skip, never fail the turn; nudge cadence every N assistant messages with session events.
- Session summarizer (SUM-001..002): one combined close call returning `{summary, observations[]}`; persists summary without bumping conversation order, saves observations, aggregates the user model; non-fatal on parse failure.
- User model (USR-001): pure `AggregateUserModel` — `user/*` keys only, confidence clamp(importance/5) then 0.7/0.3 blending, deterministic insertion order.
- Brain loop (BRN-001..002): top-N recall injected as a system message each turn; `CloseSession` resolves the latest conversation (ErrNotFound → no-op), loads up to 200 messages, hands them to the closer, records a summary event; fully non-fatal so shutdown always proceeds.
- TUI (TUI-001): idle CtrlC/Esc shows a closing status and runs a bounded CloseSession before quitting; streaming first press cancels and drains the partial reply, second press force-quits without closing; submits rejected during the closing sequence. Suite runs under goleak.
- Config: `memory` block (`learning_enabled`, `recall_limit`, `nudge_every`, `close_timeout`) with explicit-false/zero preservation and duration-string decoding.

## Verification

Independent bounded reviews (gentle-ai review-integration/v2, reliability lens):

- PR2a lineage `review-9848878c67ab3777`: **approved**, 3 informational findings (nil-baseline aggregation semantics, float-literal test brittleness, cadence option asymmetry — the latter closed by PR2b's guard).
- PR2b lineage `review-c8dc1c735254e53e`: **approved**, 2 informational suggestions (recall-load failure aborts the turn; recall block unbounded in bytes).
- PR3 lineage `review-46fb9f36c27bcb4d`: **approved, zero findings** after an abandoned earlier iteration whose quit-sequence state-machine hole (submit accepted while closing) was fixed and re-reviewed.

All pre-pr gates allow. Final verification per slice: `go build ./...`, `go vet ./...`, `go test ./...` green.

## Spec sync

NEW capabilities: `openspec/specs/{memory-curator,session-summarizer,user-model}/spec.md`. MODIFIED capabilities appended: `repository-memory`, `brain-loop`, `minimal-tui`.
