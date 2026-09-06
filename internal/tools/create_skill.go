package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/skills"
)

// CreateSkillRunner implements the core.ToolRunner interface for create_skill.
type CreateSkillRunner struct {
	skillsDir string
	hub       core.SkillHub
}

// NewCreateSkillRunner creates a new CreateSkillRunner.
func NewCreateSkillRunner(skillsDir string, hub core.SkillHub) *CreateSkillRunner {
	return &CreateSkillRunner{
		skillsDir: skillsDir,
		hub:       hub,
	}
}

// Name returns the tool name.
func (c *CreateSkillRunner) Name() string {
	return "create_skill"
}

// Description returns the tool description.
func (c *CreateSkillRunner) Description() string {
	return "Create or update a reusable procedural skill adhering to the agentskills.io standard."
}

// Backend returns the tool execution backend.
func (c *CreateSkillRunner) Backend() string {
	return "internal"
}

// CreateSkillInput represents the input structure for create_skill.
type CreateSkillInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Trigger     string `json:"trigger,omitempty"`
	Body        string `json:"body"`
	Overwrite   bool   `json:"overwrite"`
	License     string `json:"license,omitempty"`
	Author      string `json:"author,omitempty"`
}

// CreateSkillOutput represents the output structure for create_skill.
type CreateSkillOutput struct {
	Status string `json:"status"`
	Name   string `json:"name"`
	Path   string `json:"path"`
}

type skillFileFrontMatter struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Trigger     string            `yaml:"trigger,omitempty"`
	License     string            `yaml:"license,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
}

// Run executes the create_skill tool.
func (c *CreateSkillRunner) Run(ctx context.Context, input string) (string, error) {
	var in CreateSkillInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", fmt.Errorf("invalid json input: %w", err)
	}

	name := strings.TrimSpace(in.Name)
	if err := skills.ValidateSkillName(name); err != nil {
		return "", err
	}

	desc := strings.TrimSpace(in.Description)
	if desc == "" {
		return "", errors.New("missing description: skill description is required")
	}
	if len(desc) > 500 {
		return "", fmt.Errorf("skill description too long (%d chars, max 500)", len(desc))
	}

	body := strings.TrimSpace(in.Body)
	if body == "" {
		return "", errors.New("empty body: skill markdown body is required")
	}
	if err := skills.ValidateSkillSections(body); err != nil {
		return "", err
	}

	skillDir := filepath.Join(c.skillsDir, name)
	targetPath := filepath.Join(skillDir, "SKILL.md")
	flatPath := filepath.Join(c.skillsDir, name+".md")

	if !in.Overwrite {
		if _, err := os.Stat(targetPath); err == nil {
			return "", fmt.Errorf("skill %q already exists; set overwrite: true to replace", name)
		}
		if _, err := os.Stat(flatPath); err == nil {
			return "", fmt.Errorf("skill %q already exists; set overwrite: true to replace", name)
		}
	}

	license := strings.TrimSpace(in.License)
	if license == "" {
		license = "Apache-2.0"
	}

	author := strings.TrimSpace(in.Author)
	if author == "" {
		author = "agent"
	}

	fm := skillFileFrontMatter{
		Name:        name,
		Description: desc,
		Trigger:     strings.TrimSpace(in.Trigger),
		License:     license,
		Metadata: map[string]string{
			"author":  author,
			"version": "1.0",
		},
	}

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("marshaling frontmatter: %w", err)
	}

	var fileContent strings.Builder
	fileContent.WriteString("---\n")
	fileContent.Write(fmBytes)
	fileContent.WriteString("---\n\n")
	fileContent.WriteString(body)
	fileContent.WriteString("\n")

	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return "", fmt.Errorf("creating skill directory %q: %w", skillDir, err)
	}

	tmpFile := filepath.Join(skillDir, "SKILL.tmp")
	if err := os.WriteFile(tmpFile, []byte(fileContent.String()), 0o600); err != nil {
		return "", fmt.Errorf("writing temporary skill file: %w", err)
	}

	if err := os.Rename(tmpFile, targetPath); err != nil {
		_ = os.Remove(tmpFile)
		return "", fmt.Errorf("renaming skill into place: %w", err)
	}

	if c.hub != nil {
		if err := c.hub.Reload(ctx, c.skillsDir); err != nil {
			return "", fmt.Errorf("reloading skill hub: %w", err)
		}
	}

	out := CreateSkillOutput{
		Status: "created",
		Name:   name,
		Path:   targetPath,
	}

	data, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("marshaling output: %w", err)
	}

	return string(data), nil
}
