# Archive Report: m4-tools-permissions

**Archived:** 2026-08-26
**Delivered as:** 5 stacked PRs merged to main — PR #13 (policy core+audit) → #14 (CLI) → #15 (wire+loop+local) → #16 (docker+ssh) → #17 (panel+docs).
**Baseline at start:** main `f60fe43` (M3 archived). **Final:** main `0787f8c`.

## Shipped

- Policy Guard (POL-001..005): fail-closed YAML store at `$AGIS_HOME/policy.yaml`, postures `sandbox`/`standard`/`full` (full session-only), decision flow `allow|deny|ask` with `deny` beating `allow`, scopes `once|session|always|deny` (`always` persists exact-subject allow rule, `session` in-memory cleared at close), audit log of every decision/grant/revocation/tier change. Trust boundary enforced type-level: brain sees only `PolicyGuard.Evaluate`, mutation via `PolicyAdmin`.
- `agis policy` CLI (POL-004): `init` (sandbox defaults, `--force` overwrite), `set`/`rm` (per-backend via `-b`, exact-or-prefix matcher no regex), `show`, `tier` (refuses `full`), `test` (dry-run preview). Wiring before flag parsing, exit codes 0/1.
- Tool-calling wire (TOL-001, LLM-001): additive `ChatRequest.Tools` + `StreamEvent.ToolCall` + `Message.ToolCalls/ToolCallID`; provider accumulates streamed `tool_calls` fragments per index, emits once at `finish_reason: tool_calls`, malformed degrades to text, channel always closes.
- Bounded brain loop (TOL-002, BRN-004): up to 8 rounds of evaluate→approve→execute→RoleTool feedback; cap audited with user notice, forced final answer streams live; disabled tools leave path byte-identical; model output inert w.r.t policy mutation.
- Backends (TLS-001..004): local shell (`sh -c`, 60s timeout), docker (`--rm` ephemeral, `alpine:3` default), ssh (strict host-key, optional key), all behind injectable `cmdExec` seam; registry `Select()` orders local→docker→ssh, skips missing binaries/incomplete settings with warnings, `tools.enabled=false` returns empty.
- TUI: interactive approval prompt (`[a]llow once [s]ession a[l]ways [n]o`, `CtrlC` denies, watcher re-armed), and `/permisos` panel (rules by category, postures, live preview via `Guard.Evaluate`, audit tail; `space` toggle `allow↔deny`, `r` revoke `always`, `q` close).
- Config (CONF-002): `tools.enabled` (default false, opt-in), `tools.docker`/`tools.ssh` blocks; explicit values survive.

## Verification

Independent bounded reviews (gentle-ai review-integration/v2):

- PR1 lineage `review-438b697e60ca0faa`: **approved**, zero findings.
- PR2 lineage `review-6ad2941f29c60059`: **approved**, 1 suggestion (tier backend string not validated).
- PR3 lineage `review-0ea8ce9c02f5e770` (HIGH, 4 lenses): **approved**, 3 advisories (stale policy window vs CLI edits mid-session; cap-branch spin vs non-compliant provider; brain.go multi-responsibility). Earlier iteration abandoned after second-ask hang + cap sink suppression fixed.
- PR4 lineage `review-186ffa919384e005` (HIGH, 4 lenses): **approved**, 1 suggestion (unused convID param).
- PR5 lineage `review-3387ee87cd446297`: **approved**, zero findings.

All pre-pr gates allow. Final verification per slice on clean tree: `go build ./...`, `go vet ./...`, `go test ./...` green under `goleak` (10 packages). Fail-closed and graceful degradation proven via RED tests written first.

## Spec sync

NEW capabilities: `openspec/specs/{policy-guard,tools-backends,tool-calling}/spec.md`. MODIFIED capabilities appended: `brain-loop`, `llm-provider-port`, `minimal-tui`, `config-loader` (and `repository-memory` via audit migration).

## Amendment note

No spec amendments beyond implementation-clarified matching semantics already carried from M3. Panel preview uses `Guard.Evaluate` live, matching the spec's preview requirement verbatim.
