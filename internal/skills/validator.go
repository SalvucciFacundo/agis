package skills

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/SalvucciFacundo/agis/internal/core"
)

var (
	skillNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,40}$`)

	reWhenToUse     = regexp.MustCompile(`(?im)^##\s+(when\s+to\s+use|description\s*&\s*intent)\b`)
	reCriticalRules = regexp.MustCompile(`(?im)^##\s+(critical\s+rules|rules)\b`)
	reWorkflow      = regexp.MustCompile(`(?im)^##\s+(workflow|steps|procedure)\b`)
	reExamples      = regexp.MustCompile(`(?im)^##\s+(examples|reference)\b`)
)

// ValidateSkillName checks if a skill name conforms to the agentskills.io standard:
// 1 to 40 alphanumeric, underscore, or hyphen characters.
func ValidateSkillName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("skill name is required")
	}
	if !skillNameRegex.MatchString(name) {
		return fmt.Errorf("invalid skill name %q: must match regex ^[a-zA-Z0-9_-]{1,40}$", name)
	}
	return nil
}

// ValidateSkillSections checks that the markdown body contains all required level-2 sections.
func ValidateSkillSections(body string) error {
	if !reWhenToUse.MatchString(body) {
		return errors.New("missing required section: '## When to Use' (or '## Description & Intent')")
	}
	if !reCriticalRules.MatchString(body) {
		return errors.New("missing required section: '## Critical Rules' (or '## Rules')")
	}
	if !reWorkflow.MatchString(body) {
		return errors.New("missing required section: '## Workflow' (or '## Steps' / '## Procedure')")
	}
	if !reExamples.MatchString(body) {
		return errors.New("missing required section: '## Examples' (or '## Reference')")
	}
	return nil
}

// ValidateSkillContent validates a raw skill Markdown string with YAML frontmatter.
func ValidateSkillContent(raw string) error {
	const fence = "---"

	rest := strings.TrimLeft(raw, "\ufeff\n ")
	if !strings.HasPrefix(rest, fence+"\n") && rest != fence {
		return errors.New("missing frontmatter: skill file must start with ---")
	}
	body := strings.TrimPrefix(rest, fence+"\n")

	end := strings.Index(body, "\n"+fence)
	if end < 0 {
		return errors.New("unclosed frontmatter: missing closing ---")
	}

	var fm frontMatter
	if err := yaml.Unmarshal([]byte(body[:end]), &fm); err != nil {
		return fmt.Errorf("parsing frontmatter: %w", err)
	}

	if err := ValidateSkillName(fm.Name); err != nil {
		return err
	}

	desc := strings.TrimSpace(fm.Description)
	if desc == "" {
		return errors.New("missing description: skill description is required")
	}
	if len(desc) > 500 {
		return fmt.Errorf("skill description too long (%d chars, max 500)", len(desc))
	}

	content := strings.TrimSpace(body[end+len(fence)+1:])
	if content == "" {
		return errors.New("empty content body")
	}

	if err := ValidateSkillSections(content); err != nil {
		return err
	}

	return nil
}

// ValidateSkill validates a core.Skill struct.
func ValidateSkill(skill core.Skill) error {
	if err := ValidateSkillName(skill.Name); err != nil {
		return err
	}

	desc := strings.TrimSpace(skill.Description)
	if desc == "" {
		return errors.New("missing description: skill description is required")
	}
	if len(desc) > 500 {
		return fmt.Errorf("skill description too long (%d chars, max 500)", len(desc))
	}

	if err := ValidateSkillSections(skill.Content); err != nil {
		return err
	}

	return nil
}
