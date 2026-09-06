package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateSkillRunner_Metadata(t *testing.T) {
	hub := newFakeSkillHub()
	runner := NewCreateSkillRunner(t.TempDir(), hub)

	if runner.Name() != "create_skill" {
		t.Errorf("Name() = %q, want 'create_skill'", runner.Name())
	}
	if runner.Backend() != "internal" {
		t.Errorf("Backend() = %q, want 'internal'", runner.Backend())
	}
	if runner.Description() == "" {
		t.Errorf("Description() is empty")
	}
}

func TestCreateSkillRunner_Success(t *testing.T) {
	skillsDir := t.TempDir()
	hub := newFakeSkillHub()
	runner := NewCreateSkillRunner(skillsDir, hub)

	validInput := `{
		"name": "docker-build",
		"description": "Build and tag container images",
		"trigger": "docker build",
		"body": "## When to Use\nBuilding containers.\n\n## Critical Rules\n- No root.\n\n## Workflow\n1. Run build.\n\n## Examples\ndocker build -t app .",
		"overwrite": false,
		"license": "MIT",
		"author": "kuno"
	}`

	out, err := runner.Run(context.Background(), validInput)
	if err != nil {
		t.Fatalf("Run() unexpected error = %v", err)
	}

	var parsed struct {
		Status string `json:"status"`
		Name   string `json:"name"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("Unmarshal() error = %v, raw output: %s", err, out)
	}

	if parsed.Status != "created" {
		t.Errorf("status = %q, want 'created'", parsed.Status)
	}
	if parsed.Name != "docker-build" {
		t.Errorf("name = %q, want 'docker-build'", parsed.Name)
	}

	expectedPath := filepath.Join(skillsDir, "docker-build", "SKILL.md")
	if parsed.Path != expectedPath {
		t.Errorf("path = %q, want %q", parsed.Path, expectedPath)
	}

	// Verify file content on disk
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", expectedPath, err)
	}

	fileStr := string(content)
	if !strings.Contains(fileStr, "name: docker-build") {
		t.Errorf("file missing name: %s", fileStr)
	}
	if !strings.Contains(fileStr, "license: MIT") {
		t.Errorf("file missing license: %s", fileStr)
	}
	if !strings.Contains(fileStr, "author: kuno") {
		t.Errorf("file missing author: %s", fileStr)
	}
	if !strings.Contains(fileStr, "## When to Use") {
		t.Errorf("file missing When to Use section: %s", fileStr)
	}

	// Check permissions: mode 0600
	info, err := os.Stat(expectedPath)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", expectedPath, err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file permissions = %04o, want 0600", perm)
	}

	// Verify hub reload was called
	if hub.reloads != 1 {
		t.Errorf("hub.reloads = %d, want 1", hub.reloads)
	}
}

func TestCreateSkillRunner_ValidationErrors(t *testing.T) {
	skillsDir := t.TempDir()
	hub := newFakeSkillHub()
	runner := NewCreateSkillRunner(skillsDir, hub)

	tests := []struct {
		name        string
		input       string
		errContains string
	}{
		{
			name:        "invalid JSON",
			input:       "invalid-json",
			errContains: "invalid",
		},
		{
			name:        "empty name",
			input:       `{"name": "", "description": "desc", "body": "## When to Use\nx\n## Critical Rules\nx\n## Workflow\nx\n## Examples\nx"}`,
			errContains: "name",
		},
		{
			name:        "invalid name chars",
			input:       `{"name": "bad name with space", "description": "desc", "body": "## When to Use\nx\n## Critical Rules\nx\n## Workflow\nx\n## Examples\nx"}`,
			errContains: "name",
		},
		{
			name:        "missing description",
			input:       `{"name": "valid-name", "description": "", "body": "## When to Use\nx\n## Critical Rules\nx\n## Workflow\nx\n## Examples\nx"}`,
			errContains: "description",
		},
		{
			name:        "missing sections in body",
			input:       `{"name": "valid-name", "description": "valid desc", "body": "## Only Workflow\n1. do"}`,
			errContains: "When to Use",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runner.Run(context.Background(), tt.input)
			if err == nil {
				t.Fatalf("Run() expected error, got nil")
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errContains)) {
				t.Errorf("Run() error = %v, want substring %q", err, tt.errContains)
			}
		})
	}
}

func TestCreateSkillRunner_OverwriteProtection(t *testing.T) {
	skillsDir := t.TempDir()
	hub := newFakeSkillHub()
	runner := NewCreateSkillRunner(skillsDir, hub)

	input := `{
		"name": "existing-skill",
		"description": "desc",
		"body": "## When to Use\nx\n## Critical Rules\nx\n## Workflow\nx\n## Examples\nx",
		"overwrite": false
	}`

	// First creation succeeds
	_, err := runner.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}

	// Second creation with overwrite: false fails
	_, err = runner.Run(context.Background(), input)
	if err == nil {
		t.Fatalf("second Run() with overwrite=false expected error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
		t.Errorf("err = %v, want 'already exists'", err)
	}

	// Third creation with overwrite: true succeeds
	overwriteInput := `{
		"name": "existing-skill",
		"description": "updated desc",
		"body": "## When to Use\nx\n## Critical Rules\nx\n## Workflow\nx\n## Examples\nx",
		"overwrite": true
	}`
	_, err = runner.Run(context.Background(), overwriteInput)
	if err != nil {
		t.Fatalf("Run() with overwrite=true error = %v", err)
	}
}
