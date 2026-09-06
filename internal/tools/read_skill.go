package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// ReadSkillRunner implements the core.ToolRunner interface for the read_skill tool.
type ReadSkillRunner struct {
	hub core.SkillHub
}

// NewReadSkillRunner creates a new ReadSkillRunner.
func NewReadSkillRunner(hub core.SkillHub) *ReadSkillRunner {
	return &ReadSkillRunner{hub: hub}
}

// Name returns the tool name.
func (r *ReadSkillRunner) Name() string {
	return "read_skill"
}

// Description returns the tool description.
func (r *ReadSkillRunner) Description() string {
	return "Fetch the full procedural instructions, rules, and examples of a specified skill by name."
}

// Backend returns the tool execution backend.
func (r *ReadSkillRunner) Backend() string {
	return "internal"
}

// ReadSkillInput represents the input structure for read_skill.
type ReadSkillInput struct {
	Name string `json:"name"`
}

// ReadSkillOutput represents the output structure for read_skill.
type ReadSkillOutput struct {
	Status      string `json:"status"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Trigger     string `json:"trigger,omitempty"`
	Content     string `json:"content"`
}

// Run executes the read_skill tool.
func (r *ReadSkillRunner) Run(ctx context.Context, input string) (string, error) {
	var in ReadSkillInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", fmt.Errorf("invalid json input: %w", err)
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		return "", errors.New("skill name is required")
	}

	if r.hub == nil {
		return "", errors.New("skill hub not available")
	}

	skill, found := r.hub.GetSkill(name)
	if !found || skill == nil {
		return "", fmt.Errorf("skill %q not found", name)
	}

	r.hub.RecordUse(ctx, skill.Name)

	out := ReadSkillOutput{
		Status:      "found",
		Name:        skill.Name,
		Description: skill.Description,
		Trigger:     skill.Trigger,
		Content:     skill.Content,
	}

	data, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("marshaling output: %w", err)
	}

	return string(data), nil
}
