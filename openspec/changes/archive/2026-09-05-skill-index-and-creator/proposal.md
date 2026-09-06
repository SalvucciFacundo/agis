# Skill Index and Creator Proposal

## Intent
Implement a Skill Index with lazy loading to conserve prompt tokens, add native autonomous skill creation/reading tools (`read_skill`, `create_skill`), and establish the `agis skill` CLI for local management following the `agentskills.io` specification.

## Scope
1. **Lazy Loading Skills / `read_skill` tool**:
   - Introduce `SkillsConfig.LazyLoading` toggle in `internal/config/config.go` (default: `true`).
   - Modify `internal/core/brain.go` / `port_learning.go` (`skillsSystemMessage`) to emit a Markdown index of matched skills (Table format: `Skill`, `Trigger / Description`, `Scope`) when `LazyLoading` is true, falling back to full bodies if false.
   - Implement `internal/tools/read_skill.go` implementing `ToolRunner`, allowing the agent to fetch the exact full body of a requested skill.
   - Expand `core.SkillHub` interface to expose `GetSkill(name string) (*Skill, bool)` to support `read_skill`.

2. **Native Skill Creator / `create_skill` tool & `agis skill` CLI**:
   - Implement `internal/tools/create_skill.go` implementing `ToolRunner` to allow autonomous skill writing.
   - Expand `core.SkillHub` interface to expose `Refresh(ctx, dir)` (or similar) to atomically re-index after creation.
   - Enforce the `agentskills.io` standard: `create_skill` strictly validates the required YAML frontmatter (`name`, `description`, `trigger`, `license`, `metadata: { author, version }`), sanitizes names (`^[a-zA-Z0-9_-]+$`), and checks for required standard sections.
   - Update `internal/skills/loader.go`'s `frontMatter` struct to capture all required `agentskills.io` standard fields.
   - Add a new `cmd/agis/skill.go` Cobra tree: `agis skill list`, `agis skill create`, `agis skill show`, `agis skill delete`.

3. **Configuration & Diagnostics**:
   - Modify `internal/doctor/doctor.go` (`checkSkills` probe) to validate the strict syntax and metadata completeness of skills.

## Non-goals & Boundaries
- Creating a remote skill repository/sharing registry.
- Migrating existing skills to a new data store (they remain local `.md` files).
- Auto-fixing heavily malformed skills in `create_skill` beyond basic field defaults (it should reject and ask the agent to correct if completely invalid).

## Risks & Edge Cases
1. **Tool Blindness**: If `lazy_loading` is enabled, the agent must actually use the `read_skill` tool. If it hallucinates the skill contents based solely on the description, implementation quality drops. 
   - *Mitigation*: The system prompt for lazy loading will strictly instruct the agent: "Use the `read_skill(name)` tool to read the full body of a skill."
2. **Namespace Collisions**: Agent creates a skill that overwrites a user's manual file.
   - *Mitigation*: The `create_skill` tool should probably either fail if the file exists and is not an agent-managed file, or strictly overwrite. First slice allows overwrite for simplicity.
3. **Invalid Agent Output**: The LLM omits sections like `metadata.author`.
   - *Mitigation*: `create_skill` validates inputs, rejecting with a detailed error telling the agent what is missing so it can retry with the correct structure.

## Rollback Plan
- The `skills.lazy_loading: false` toggle will immediately revert the context window to full body injection.
- Revert the `agis skill` commands and tool runners via Git.

## Success Criteria
- The agent prompt successfully truncates long skills into a compact table index.
- The agent natively discovers and uses `read_skill(name)` to access the full instructions.
- The agent natively constructs new skills via `create_skill`, passing frontmatter validation, which are immediately accessible in the Hub.
- The `agis skill list`, `show`, `delete`, and `create` CLI commands function as expected locally.

## Proposal Question Round
*(For user review/adjustment)*
1. **Business problem & product outcome:** We default `lazy_loading` to `true`. Have we ensured the model is proactive enough to call `read_skill` instead of hallucinating content from just the description?
2. **Edge cases & scope:** The `create_skill` tool will write to `skills/<name>/SKILL.md`. What should happen if the agent tries to overwrite a user-managed skill? Should we allow overwriting any skill, or fail if the file already exists?
3. **Impact:** The `agentskills.io` standard expects strict `metadata: { author, version }`. If the LLM drifts from the exact syntax, should the `create_skill` tool aggressively auto-correct missing fields to maintain velocity, or fail and prompt the agent to fix it?
4. **Implications:** Do we need an `agis skill edit` CLI command to easily open skills in `$EDITOR`, or is that out of scope for the first slice?