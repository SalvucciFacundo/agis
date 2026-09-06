# Skills

Skills are AGIS's procedural memory: reusable, plain-Markdown instructions the agent can apply to future requests. The implementation lives in `internal/skills`.

---

## 1. Skill Format (`agentskills.io` Standard)

Skills in AGIS adhere to the [agentskills.io](https://agentskills.io) specification. They can be stored in either flat or nested directory layouts inside `$AGIS_HOME/skills` (`~/.agis/skills` by default):

- **Flat layout**: `$AGIS_HOME/skills/<name>.md`
- **Nested layout**: `$AGIS_HOME/skills/<name>/SKILL.md` (or `$AGIS_HOME/skills/<name>/<name>.md`)

### Standard Structure

Every skill definition consists of a YAML frontmatter header followed by required Markdown Level-2 (`##`) sections:

```markdown
---
name: release-checklist
description: Step-by-step procedure to ship a clean production release
trigger: release, deploy
license: Apache-2.0
metadata:
  author: dev-team
  version: "1.0"
---

## When to Use
Describe when this skill should be selected and applied by the agent.

## Critical Rules
1. Run the full test suite with race detector enabled before tagging.
2. Never commit unreviewed code directly to production branches.

## Workflow
1. Run `go test -race ./...`.
2. Generate changelog and bump version tags.
3. Push tags and monitor CI pipeline.

## Examples
Example command: `git tag -a v1.2.0 -m "Release v1.2.0"`
```

### Frontmatter Fields
- `name` (**required**): 1 to 40 characters matching `^[a-zA-Z0-9_-]{1,40}$`.
- `description` (**required**): Short summary of the skill's purpose (up to 500 characters).
- `trigger` (*optional*): Comma-separated keywords to enhance search matching.
- `license` (*optional*): Open-source license identifier (default: `Apache-2.0`).
- `metadata` (*optional*): Map with author, version, or tags.

### Required Markdown Sections
The Markdown body must contain these Level-2 headings (or standard synonyms):
- `## When to Use` (or `## Description & Intent`)
- `## Critical Rules` (or `## Rules`)
- `## Workflow` (or `## Steps`, `## Procedure`)
- `## Examples` (or `## Reference`)

---

## 2. Lazy Loading & Prompt Optimization

By default, AGIS enables **Lazy Loading** (`skills.lazy_loading: true`). 

Instead of injecting the entire Markdown body of all matched skills into every turn's system prompt (which consumes significant context tokens), AGIS injects a compact Markdown index table:

```markdown
| Skill | Trigger / Description | Scope / Version |
| --- | --- | --- |
| `git-rebase` | Interactive git rebase workflow | imported (v1.0) |
| `docker-build` | Multi-stage container build | imported (v1.0) |

To read complete procedural instructions, call read_skill(name).
```

When the LLM decides it needs full procedural instructions, it calls the native `read_skill` tool to retrieve the complete rules and workflows on demand.

To revert to full-body injection, set `skills.lazy_loading: false` in `config.yaml`.

---

## 3. Native Model Tools

When `tools.enabled: true` and `skills.enabled: true`, AGIS automatically provides two internal model tools to the LLM:

### `read_skill`
Fetches full procedural instructions and examples on demand:
- **Input**: `{"name": "release-checklist"}`
- **Output**: Full Markdown body, frontmatter attributes, and usage tracking.

### `create_skill`
Allows autonomous agents to distill and save new skills conforming to the `agentskills.io` standard:
- **Input**:
  ```json
  {
    "name": "terraform-plan",
    "description": "Validate infrastructure changes with Terraform",
    "trigger": "terraform, infra",
    "body": "## When to Use\n...\n## Critical Rules\n...\n## Workflow\n...\n## Examples\n...",
    "overwrite": false
  }
  ```
- **Output**: Atomic write to `$AGIS_HOME/skills/<name>/SKILL.md` (`0600` permissions), hub reload, and registry synchronization.

---

## 4. CLI Subcommand Suite (`agis skill`)

Manage skills locally via the command line:

```bash
# List all installed skills in a tabular view
agis skill list

# List skills in structured JSON format
agis skill list -json

# Scaffold a new standard skill with boilerplate sections
agis skill create k8s-deploy -desc "Deploy services to Kubernetes" -trigger "k8s,deploy"

# Display skill details and formatted instructions
agis skill show k8s-deploy

# View raw file contents including frontmatter
agis skill show k8s-deploy -raw

# Delete an installed skill (with confirmation prompt)
agis skill delete k8s-deploy

# Force delete without interactive confirmation
agis skill delete k8s-deploy -yes
```

---

## 5. Agent Skill Extraction & Discovery

- **Session Learning**: When a session closes, AGIS evaluates whether the conversation produced a novel reusable procedure. If so, it creates an agent-authored skill.
- **Security Scanner**: File contents pass through the injection scanner before entering prompt context; flagged malicious lines are dropped.
- **Registry**: AGIS maintains a human-readable index at `$AGIS_HOME/.atl/skill-registry.md`.

---

## 6. Diagnostics & Health Probe

Run `agis doctor` to validate skills health:
```bash
agis doctor
```
The `skills` probe inspects all flat and nested files, validates schema and required sections, reports loaded skill counts, and flags malformed skills with actionable remediation steps.
