package plugins_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/plugins"
)

func TestParseManifest_Valid(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		validate func(t *testing.T, m *plugins.Manifest)
	}{
		{
			name: "minimal manifest",
			json: `{
				"name": "weather",
				"version": "1.0.0"
			}`,
			validate: func(t *testing.T, m *plugins.Manifest) {
				if m.Name != "weather" {
					t.Errorf("Name = %q, want weather", m.Name)
				}
				if m.Version != "1.0.0" {
					t.Errorf("Version = %q, want 1.0.0", m.Version)
				}
			},
		},
		{
			name: "full manifest with tools and skills",
			json: `{
				"name": "github-tools_v2",
				"version": "2.1.0-alpha",
				"description": "GitHub integration tools",
				"entrypoint": "bin/github-helper",
				"tools": [
					{
						"name": "search_repos",
						"description": "Search repositories on GitHub",
						"parameters": {
							"type": "object",
							"properties": {
								"query": {"type": "string"}
							}
						}
					}
				],
				"skills": ["search-skill.md", "pr-skill.md"],
				"permissions": ["network", "commands"]
			}`,
			validate: func(t *testing.T, m *plugins.Manifest) {
				if m.Name != "github-tools_v2" {
					t.Errorf("Name = %q, want github-tools_v2", m.Name)
				}
				if m.Version != "2.1.0-alpha" {
					t.Errorf("Version = %q, want 2.1.0-alpha", m.Version)
				}
				if m.Description != "GitHub integration tools" {
					t.Errorf("Description = %q", m.Description)
				}
				if m.Entrypoint != "bin/github-helper" {
					t.Errorf("Entrypoint = %q", m.Entrypoint)
				}
				if len(m.Tools) != 1 || m.Tools[0].Name != "search_repos" {
					t.Errorf("Tools = %+v", m.Tools)
				}
				if len(m.Skills) != 2 || m.Skills[0] != "search-skill.md" {
					t.Errorf("Skills = %+v", m.Skills)
				}
				if len(m.Permissions) != 2 || m.Permissions[0] != "network" {
					t.Errorf("Permissions = %+v", m.Permissions)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := plugins.ParseManifest([]byte(tt.json))
			if err != nil {
				t.Fatalf("ParseManifest() unexpected error: %v", err)
			}
			if tt.validate != nil {
				tt.validate(t, m)
			}
		})
	}
}

func TestParseManifest_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		errContains string
	}{
		{
			name:        "invalid JSON syntax",
			json:        `{name: weather}`,
			errContains: "parsing",
		},
		{
			name:        "missing name",
			json:        `{"version": "1.0.0"}`,
			errContains: "name is required",
		},
		{
			name:        "empty name",
			json:        `{"name": "", "version": "1.0.0"}`,
			errContains: "name is required",
		},
		{
			name:        "invalid name with uppercase",
			json:        `{"name": "Weather", "version": "1.0.0"}`,
			errContains: "invalid plugin name",
		},
		{
			name:        "invalid name with spaces",
			json:        `{"name": "my plugin", "version": "1.0.0"}`,
			errContains: "invalid plugin name",
		},
		{
			name:        "invalid name with special symbols",
			json:        `{"name": "plugin@v1", "version": "1.0.0"}`,
			errContains: "invalid plugin name",
		},
		{
			name:        "missing version",
			json:        `{"name": "weather"}`,
			errContains: "version is required",
		},
		{
			name:        "empty version",
			json:        `{"name": "weather", "version": ""}`,
			errContains: "version is required",
		},
		{
			name: "tool with empty name",
			json: `{
				"name": "weather",
				"version": "1.0.0",
				"tools": [{"name": ""}]
			}`,
			errContains: "tool name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := plugins.ParseManifest([]byte(tt.json))
			if err == nil {
				t.Fatalf("ParseManifest() expected error containing %q, got nil", tt.errContains)
			}
			if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestParseManifestFile(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "plugin.json")

	content := `{"name": "test-plugin", "version": "1.0.0", "description": "A test plugin"}`
	if err := os.WriteFile(manifestPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m, err := plugins.ParseManifestFile(manifestPath)
	if err != nil {
		t.Fatalf("ParseManifestFile() error = %v", err)
	}
	if m.Name != "test-plugin" {
		t.Errorf("Name = %q, want test-plugin", m.Name)
	}

	// Missing file test
	_, err = plugins.ParseManifestFile(filepath.Join(dir, "nonexistent.json"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || filepath.Base(s) == substr || len(substr) > 0 && searchSubstr(s, substr))
}

func searchSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
