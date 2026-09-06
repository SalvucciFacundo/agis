package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/tools"
)

type dummyRunner struct {
	name        string
	description string
	backend     string
}

func (d *dummyRunner) Name() string        { return d.name }
func (d *dummyRunner) Description() string { return d.description }
func (d *dummyRunner) Backend() string     { return d.backend }
func (d *dummyRunner) Run(_ context.Context, _ string) (string, error) {
	return "ok", nil
}

func sampleInventory() []core.ToolRunner {
	return []core.ToolRunner{
		&dummyRunner{
			name:        "web_search",
			description: "Search the web using DuckDuckGo, Brave, Tavily, or SearXNG.",
			backend:     "web",
		},
		&dummyRunner{
			name:        "web_fetch",
			description: "Extract readable text content and markdown from a webpage URL.",
			backend:     "web",
		},
		&dummyRunner{
			name:        "shell-local",
			description: "Run a shell command on the local backend.",
			backend:     "local",
		},
		&dummyRunner{
			name:        "mcp_github_create_issue",
			description: "Create a new issue on a GitHub repository.",
			backend:     "mcp",
		},
		&dummyRunner{
			name:        "mcp_slack_post_message",
			description: "Post a message to a Slack channel.",
			backend:     "mcp",
		},
		&dummyRunner{
			name:        "delegate_task",
			description: "Delegate a subtask to an isolated worker subagent.",
			backend:     "subagent",
		},
	}
}

func TestToolSearchRunner_Contract(t *testing.T) {
	runner := tools.NewToolSearchRunner(sampleInventory())

	if runner.Name() != "tool_search" {
		t.Errorf("Name() = %q, want 'tool_search'", runner.Name())
	}
	if runner.Backend() != "internal" {
		t.Errorf("Backend() = %q, want 'internal'", runner.Backend())
	}
	if !strings.Contains(runner.Description(), "Search for available tools") {
		t.Errorf("Description() = %q, want description mentioning searching tools", runner.Description())
	}
}

func TestToolSearchRunner_Search(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		errSubstr   string
		wantMatches []string
	}{
		{
			name:        "search by keyword matches name",
			input:       `{"query": "github"}`,
			wantMatches: []string{"mcp_github_create_issue"},
		},
		{
			name:        "search by keyword matches description",
			input:       `{"query": "markdown"}`,
			wantMatches: []string{"web_fetch"},
		},
		{
			name:        "search by category web",
			input:       `{"category": "web"}`,
			wantMatches: []string{"web_search", "web_fetch"},
		},
		{
			name:        "search by category mcp",
			input:       `{"category": "mcp"}`,
			wantMatches: []string{"mcp_github_create_issue", "mcp_slack_post_message"},
		},
		{
			name:        "search by query and category filter",
			input:       `{"query": "slack", "category": "mcp"}`,
			wantMatches: []string{"mcp_slack_post_message"},
		},
		{
			name:        "search with no matches returns empty json array",
			input:       `{"query": "nonexistent_tool_query"}`,
			wantMatches: []string{},
		},
		{
			name:      "empty query and category returns error",
			input:     `{}`,
			wantErr:   true,
			errSubstr: "at least one search parameter",
		},
		{
			name:      "invalid json input returns error",
			input:     `not json`,
			wantErr:   true,
			errSubstr: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := tools.NewToolSearchRunner(sampleInventory())
			out, err := runner.Run(context.Background(), tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Run(%s) expected error, got nil", tt.input)
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Run(%s) unexpected error = %v", tt.input, err)
			}

			var results []tools.ToolMetadata
			if err := json.Unmarshal([]byte(out), &results); err != nil {
				t.Fatalf("failed to unmarshal output %q: %v", out, err)
			}

			if len(results) != len(tt.wantMatches) {
				t.Fatalf("got %d matches, want %d (results: %+v)", len(results), len(tt.wantMatches), results)
			}

			for i, wantName := range tt.wantMatches {
				if results[i].Name != wantName {
					t.Errorf("results[%d].Name = %q, want %q", i, results[i].Name, wantName)
				}
			}
		})
	}
}
