# Skills

Skills are AGIS's procedural memory: reusable, plain-Markdown instructions the agent can apply to future requests (spec §4). The implementation lives in `internal/skills`.

## Skill files

Drop `.md` files into `$AGIS_HOME/skills` (`~/.agis/skills` by default). Each file is agentskills.io-style Markdown with a YAML frontmatter header:

```markdown
---
name: release-checklist
description: Steps to ship a clean release
trigger: release          # optional keyword that aids matching
---

1. Run the full test suite.
2. Tag the merge commit.
3. Watch CI before announcing.
```

- `name` and `description` are **required**; `trigger` is optional.
- Invalid files are skipped with a logged warning — startup never fails over one bad skill.
- File contents pass through the [injection scanner](security.md) before entering any prompt; flagged lines are dropped.

## Matching and usage

On every turn the hub matches your input against each skill's `name`, `trigger`, and `description` with AND-term semantics (common stop words ignored). Up to three matches are injected as a system message ahead of recall, and each injected skill's usage counter is bumped.

## Agent-created skills

When a session closes, AGIS makes one bounded LLM call asking whether the conversation produced a reusable procedure. If it did, the skill is saved with `source: agent`, a `skill` session event is recorded, and the in-memory index refreshes. Malformed answers are logged and skipped; extraction failures never block quitting.

## Registry

The hub regenerates `$AGIS_HOME/.atl/skill-registry.md` after loading and creating skills — a human-readable index of name, trigger, source, usage count, and description. Write failures are warnings only.

## Configuration

```yaml
skills:
  enabled: true       # false disables loading, matching, and creation entirely
  dir: ~/.agis/skills # where skill files live
```

See [configuration.md](configuration.md) for precedence rules.
