# Proposal: Subagent Memory Learning (subagent-memory-learning)

## Intent
The goal of this change is to enable native knowledge distillation for subagents into persistent memory. By capturing durable learnings, decisions, facts, and discoveries produced during a child subagent's execution and persisting them directly to the parent's memory, we allow the parent `core.Brain` to retain valuable insights from ephemeral subagent tasks across turns and future sessions without bloating its immediate context window. 

## Scope
1. **Knowledge Distillation Mechanism**: 
   - Parse subagent synthesized outputs for structured markdown sections like `## Key Learnings` or `## Discoveries` (passive distillation).
   - Provide an optional, lightweight active extraction mechanism (LLM curation call) for tasks producing substantial multi-turn findings.
2. **Direct Memory Persistence**:
   - Utilize `parent.SaveObservations` to persist the extracted knowledge as `core.Observation` entities in the SQLite backend (`agis.db`).
   - Observations will use stable keys (e.g., `topic_key: subagent/<topic>`), categorized by type (`discovery`, `decision`, `pattern`, `bugfix`), with an assigned importance score (1-5).
   - Benefit from existing automated FTS indexing (`memory_fts`) and async vector embedding triggers for Hybrid Search (RRF).
3. **Configuration & Guardrails**:
   - Introduce `LearningEnabled bool` (default `true`) and `MaxObservations int` (default `3`, cap per delegation) to `SubagentsConfig` in `internal/config/config.go` to prevent memory flooding.
   - Enforce rate/count limits per task execution.
   - Record memory persistence events in the `PolicyGuard` audit trail using `core.AuditEntry` (e.g. `Backend: "subagent"`, `Category: "learning"`).
4. **Observability**:
   - Extend the `checkSubagents` diagnostic probe in `internal/doctor/subagents.go` to verify and report the active learning configuration and status.

## Affected Areas
- `internal/subagents/engine.go` (Distillation logic and hook into completion of `delegate_task`).
- `internal/config/config.go` (`SubagentsConfig` structure).
- `internal/core/` (If any new constants like `CategoryLearning` are needed for the audit trail).
- `internal/doctor/subagents.go` (`checkSubagents` diagnostic probe).
- `internal/tools/subagent.go` (Integration with distillation during tool execution and synthesis).

## Risks
- **Memory Flooding**: Subagents could generate a high volume of low-value observations. Mitigated by strict `MaxObservations` clamping, default limits (3 per task), and importance scoring.
- **Latency Overheads**: Subagent completion could be delayed if active extraction is heavily used. Mitigated by keeping active extraction optional and utilizing fast, small-model calls or falling back to regex-based passive distillation. Background vector embeddings already mitigate database blocking.
- **Context Pollution**: Poorly categorized observations might degrade Hybrid Search (RRF) precision. Mitigated by namespaced `topic_key` strategies (e.g. `subagent/<topic>`).

## Rollback
- Disable `learning_enabled` in `SubagentsConfig` to immediately halt new knowledge distillation.
- For deep rollbacks, revert the modifications in `internal/subagents/` and `internal/config/config.go`. Existing persisted memory will gracefully degrade as standard observations in the FTS/Vector indexes.

## Success Criteria
- [ ] Subagents successfully extract `## Key Learnings` from their synthesized output.
- [ ] Extracted items are correctly written to SQLite via `parent.SaveObservations` and become searchable in subsequent RRF queries.
- [ ] Configuration correctly gates the subsystem (`learning_enabled`), and limits enforce `max_observations` (default 3) per task.
- [ ] Memory persistence triggers an `AuditEntry` log for traceability.
- [ ] The `doctor` probe explicitly reports the subagent learning capability state.