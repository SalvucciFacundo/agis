package skills

import (
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestValidateSkillContent_Valid(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "full standard agentskills.io skill",
			content: `---
name: docker-build
description: Build and tag container images
trigger: docker build
license: Apache-2.0
metadata:
  author: agis-agent
  version: "1.0"
---

## When to Use
Use this skill whenever building docker images.

## Critical Rules
- Never use latest tag in production.
- Always run as non-root user.

## Workflow
1. Run docker build.
2. Tag with git SHA.

## Examples
docker build -t myapp:v1 .
`,
		},
		{
			name: "alternate header variations",
			content: `---
name: git_commit-test
description: Testing git workflow
---

## Description & Intent
Intended for committing changes.

## Rules
Follow conventional commits.

## Steps
1. Stage changes.
2. Commit.

## Reference
See git commit docs.
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSkillContent(tt.content)
			if err != nil {
				t.Fatalf("ValidateSkillContent() unexpected error = %v", err)
			}
		})
	}
}

func TestValidateSkillContent_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		errContains string
	}{
		{
			name:        "missing frontmatter",
			content:     "Just markdown body without frontmatter",
			errContains: "missing frontmatter",
		},
		{
			name: "missing name",
			content: `---
description: No name provided
---

## When to Use
x

## Critical Rules
x

## Workflow
x

## Examples
x
`,
			errContains: "name",
		},
		{
			name: "invalid name with spaces",
			content: `---
name: invalid name with spaces
description: bad name
---

## When to Use
x

## Critical Rules
x

## Workflow
x

## Examples
x
`,
			errContains: "invalid skill name",
		},
		{
			name: "name too long (over 40 chars)",
			content: `---
name: a_very_long_skill_name_that_exceeds_forty_characters_limit
description: long name
---

## When to Use
x

## Critical Rules
x

## Workflow
x

## Examples
x
`,
			errContains: "invalid skill name",
		},
		{
			name: "missing description",
			content: `---
name: good-name
description: ""
---

## When to Use
x

## Critical Rules
x

## Workflow
x

## Examples
x
`,
			errContains: "description",
		},
		{
			name: "description too long",
			content: `---
name: good-name
description: ` + strings.Repeat("a", 501) + `
---

## When to Use
x

## Critical Rules
x

## Workflow
x

## Examples
x
`,
			errContains: "description",
		},
		{
			name: "missing when to use section",
			content: `---
name: my-skill
description: valid description
---

## Critical Rules
- Rule 1

## Workflow
1. Step 1

## Examples
Example 1
`,
			errContains: "When to Use",
		},
		{
			name: "missing critical rules section",
			content: `---
name: my-skill
description: valid description
---

## When to Use
Whenever.

## Workflow
1. Step 1

## Examples
Example 1
`,
			errContains: "Critical Rules",
		},
		{
			name: "missing workflow section",
			content: `---
name: my-skill
description: valid description
---

## When to Use
Whenever.

## Critical Rules
- Rule 1

## Examples
Example 1
`,
			errContains: "Workflow",
		},
		{
			name: "missing examples section",
			content: `---
name: my-skill
description: valid description
---

## When to Use
Whenever.

## Critical Rules
- Rule 1

## Workflow
1. Step 1
`,
			errContains: "Examples",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSkillContent(tt.content)
			if err == nil {
				t.Fatalf("ValidateSkillContent() expected error containing %q, got nil", tt.errContains)
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errContains)) {
				t.Fatalf("ValidateSkillContent() error = %v, want substring %q", err, tt.errContains)
			}
		})
	}
}

func TestValidateSkill_Struct(t *testing.T) {
	validSkill := core.Skill{
		Name:        "k8s-deploy",
		Description: "Deploy to Kubernetes cluster",
		Content: `## When to Use
Deploying manifests.

## Critical Rules
- Verify namespace.

## Workflow
1. Apply manifests.

## Examples
kubectl apply -f .
`,
	}

	if err := ValidateSkill(validSkill); err != nil {
		t.Fatalf("ValidateSkill(validSkill) unexpected error = %v", err)
	}

	invalidSkill := core.Skill{
		Name:        "invalid skill name!",
		Description: "Desc",
		Content:     validSkill.Content,
	}
	if err := ValidateSkill(invalidSkill); err == nil {
		t.Fatalf("ValidateSkill(invalidSkill) expected error, got nil")
	}
}
