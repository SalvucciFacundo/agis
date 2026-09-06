package skills

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/scan"
)

// frontMatter is the YAML header of an agentskills.io-compatible skill file.
type frontMatter struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Trigger     string            `yaml:"trigger"`
	License     string            `yaml:"license"`
	Metadata    map[string]string `yaml:"metadata"`
}

// LoadDir loads every valid skill file from dir and returns them as imported
// skills. Supports both flat layout (dir/<name>.md) and nested layout
// (dir/<name>/SKILL.md or dir/<name>/<name>.md).
// Files with missing name/description or unparsable frontmatter are
// skipped with a logged warning, never fatal (spec SKL-001, SKL-IDX-003). Skill contents
// pass through the injection scanner before returning; dropped lines are
// logged too. A missing directory returns an empty result: no skills is a
// normal state, not an error.
func LoadDir(dir string, logger *slog.Logger) ([]core.Skill, error) {
	if logger == nil {
		logger = slog.Default()
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []core.Skill
	for _, e := range entries {
		if e.IsDir() {
			// Check nested layout: <dir>/SKILL.md, or <dir>/<dirname>.md
			subDirPath := filepath.Join(dir, e.Name())
			targetPath := filepath.Join(subDirPath, "SKILL.md")
			if _, err := os.Stat(targetPath); err != nil {
				// Fallback to <subdirname>.md
				targetPath = filepath.Join(subDirPath, e.Name()+".md")
				if _, err := os.Stat(targetPath); err != nil {
					continue
				}
			}

			skill, ok := loadSkillFile(targetPath, logger)
			if ok {
				out = append(out, skill)
			}
			continue
		}

		if !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		skill, ok := loadSkillFile(path, logger)
		if ok {
			out = append(out, skill)
		}
	}
	return out, nil
}

func loadSkillFile(path string, logger *slog.Logger) (core.Skill, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		logger.Warn("skills: skipping unreadable file", "path", path, "error", err)
		return core.Skill{}, false
	}

	skill, err := parseFile(string(data))
	if err != nil {
		logger.Warn("skills: skipping invalid skill file", "path", path, "error", err)
		return core.Skill{}, false
	}
	skill.Source = core.SourceImported

	content, dropped := scan.Lines(skill.Content)
	if dropped > 0 {
		logger.Warn("skills: dropped injected lines", "path", path, "count", dropped)
	}
	skill.Content = content

	return skill, true
}

// parseFile splits a Markdown skill file into its YAML frontmatter and body,
// validates the required fields, and builds the domain skill.
func parseFile(data string) (core.Skill, error) {
	const fence = "---"

	rest := strings.TrimLeft(data, "\ufeff\n ")
	if !strings.HasPrefix(rest, fence+"\n") && rest != fence {
		return core.Skill{}, errInvalid{"missing frontmatter"}
	}
	body := strings.TrimPrefix(rest, fence+"\n")

	end := strings.Index(body, "\n"+fence)
	if end < 0 {
		return core.Skill{}, errInvalid{"unclosed frontmatter"}
	}

	var fm frontMatter
	if err := yaml.Unmarshal([]byte(body[:end]), &fm); err != nil {
		return core.Skill{}, errInvalid{"parsing frontmatter: " + err.Error()}
	}
	if strings.TrimSpace(fm.Name) == "" {
		return core.Skill{}, errInvalid{"missing name"}
	}
	if strings.TrimSpace(fm.Description) == "" {
		return core.Skill{}, errInvalid{"missing description"}
	}

	content := strings.TrimSpace(body[end+len(fence)+1:])
	if content == "" {
		return core.Skill{}, errInvalid{"empty content body"}
	}

	return core.Skill{
		Name:        strings.TrimSpace(fm.Name),
		Description: strings.TrimSpace(fm.Description),
		Trigger:     strings.TrimSpace(fm.Trigger),
		Content:     content,
	}, nil
}

// errInvalid marks a skill file as structurally invalid.
type errInvalid struct{ msg string }

func (e errInvalid) Error() string { return "invalid skill file: " + e.msg }
