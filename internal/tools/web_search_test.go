package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/tools/web/search"
)

type mockSearcher struct {
	name       string
	results    []search.SearchResult
	err        error
	lastQuery  string
	lastOpts   search.SearchOptions
	calledWith []string
}

func (m *mockSearcher) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func (m *mockSearcher) Search(ctx context.Context, query string, opts search.SearchOptions) ([]search.SearchResult, error) {
	m.lastQuery = query
	m.lastOpts = opts
	m.calledWith = append(m.calledWith, query)
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func TestWebSearchRunner_Contract(t *testing.T) {
	ms := &mockSearcher{name: "duckduckgo"}
	runner := NewWebSearchRunner(ms, "duckduckgo", config.WebProviders{})

	if runner.Backend() != "web" {
		t.Errorf("Backend() = %q, want %q", runner.Backend(), "web")
	}
	if runner.Name() != "web_search" {
		t.Errorf("Name() = %q, want %q", runner.Name(), "web_search")
	}
	if !strings.Contains(runner.Description(), "Search the web") {
		t.Errorf("Description() = %q, want containing 'Search the web'", runner.Description())
	}
}

func TestWebSearchRunner_Run_JSONInput(t *testing.T) {
	ms := &mockSearcher{
		results: []search.SearchResult{
			{Title: "Go 1.26 Release Notes", URL: "https://go.dev/doc/go1.26", Snippet: "Go 1.26 introduces new features."},
			{Title: "Go Blog", URL: "https://go.dev/blog", Snippet: "Latest Go announcements."},
		},
	}
	runner := NewWebSearchRunner(ms, "duckduckgo", config.WebProviders{})

	ctx := context.Background()
	input := `{"query": "golang 1.26", "max_results": 2}`
	out, err := runner.Run(ctx, input)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if ms.lastQuery != "golang 1.26" {
		t.Errorf("lastQuery = %q, want %q", ms.lastQuery, "golang 1.26")
	}
	if ms.lastOpts.MaxResults != 2 {
		t.Errorf("lastOpts.MaxResults = %d, want %d", ms.lastOpts.MaxResults, 2)
	}

	var parsed []search.SearchResult
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("unmarshaling Run() output error = %v, output = %s", err, out)
	}
	if len(parsed) != 2 {
		t.Fatalf("len(parsed) = %d, want 2", len(parsed))
	}
	if parsed[0].Title != "Go 1.26 Release Notes" {
		t.Errorf("parsed[0].Title = %q", parsed[0].Title)
	}
}

func TestWebSearchRunner_Run_PlainTextInput(t *testing.T) {
	ms := &mockSearcher{
		results: []search.SearchResult{
			{Title: "Example", URL: "https://example.com", Snippet: "Example domain."},
		},
	}
	runner := NewWebSearchRunner(ms, "duckduckgo", config.WebProviders{})

	ctx := context.Background()
	out, err := runner.Run(ctx, "pure go html parser")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if ms.lastQuery != "pure go html parser" {
		t.Errorf("lastQuery = %q, want %q", ms.lastQuery, "pure go html parser")
	}
	if ms.lastOpts.MaxResults != 5 { // default
		t.Errorf("lastOpts.MaxResults = %d, want 5 (default)", ms.lastOpts.MaxResults)
	}
	if !strings.Contains(out, "https://example.com") {
		t.Errorf("output missing expected URL: %s", out)
	}
}

func TestWebSearchRunner_Run_EmptyQueryValidation(t *testing.T) {
	ms := &mockSearcher{}
	runner := NewWebSearchRunner(ms, "duckduckgo", config.WebProviders{})

	cases := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"whitespace only", "   \t\n  "},
		{"json empty query", `{"query": ""}`},
		{"json whitespace query", `{"query": "   "}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runner.Run(context.Background(), tc.input)
			if err == nil {
				t.Fatalf("Run(%q) expected error for empty query, got nil", tc.input)
			}
		})
	}
}

func TestWebSearchRunner_Run_MaxResultsClamping(t *testing.T) {
	ms := &mockSearcher{
		results: []search.SearchResult{{Title: "T", URL: "U", Snippet: "S"}},
	}
	runner := NewWebSearchRunner(ms, "duckduckgo", config.WebProviders{})

	// Negative / 0 clamped to default 5
	_, _ = runner.Run(context.Background(), `{"query": "test", "max_results": 0}`)
	if ms.lastOpts.MaxResults != 5 {
		t.Errorf("max_results 0 should default to 5, got %d", ms.lastOpts.MaxResults)
	}

	// Exceeding 20 clamped to 20
	_, _ = runner.Run(context.Background(), `{"query": "test", "max_results": 50}`)
	if ms.lastOpts.MaxResults != 20 {
		t.Errorf("max_results 50 should clamp to 20, got %d", ms.lastOpts.MaxResults)
	}
}

func TestWebSearchRunner_Run_ProviderOverride(t *testing.T) {
	providers := config.WebProviders{
		BraveAPIKey: "BSA_test_key",
	}
	defaultMS := &mockSearcher{name: "duckduckgo"}
	runner := NewWebSearchRunner(defaultMS, "duckduckgo", providers)

	// When provider override is specified
	input := `{"query": "test query", "provider": "brave", "max_results": 3}`
	runner.providerFactory = func(name string, cfg config.WebConfig) (search.Searcher, error) {
		if name != "brave" {
			t.Errorf("factory called with provider %q, want brave", name)
		}
		return &mockSearcher{
			name: "brave",
			results: []search.SearchResult{
				{Title: "Brave Result", URL: "https://brave.com", Snippet: "Brave snippet"},
			},
		}, nil
	}

	out, err := runner.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run() with provider override error = %v", err)
	}
	if !strings.Contains(out, "Brave Result") {
		t.Errorf("output does not contain Brave Result: %s", out)
	}
}

func TestWebSearchRunner_Run_ZeroResults(t *testing.T) {
	ms := &mockSearcher{results: []search.SearchResult{}}
	runner := NewWebSearchRunner(ms, "duckduckgo", config.WebProviders{})

	out, err := runner.Run(context.Background(), `{"query": "nonexistent query xyz123"}`)
	if err != nil {
		t.Fatalf("Run() unexpected error = %v", err)
	}
	if out != "[]" && !strings.Contains(strings.ToLower(out), "no results") {
		t.Errorf("Run() with zero results should return [] or no results message, got %q", out)
	}
}

func TestWebSearchRunner_Run_SearcherError(t *testing.T) {
	ms := &mockSearcher{err: errors.New("upstream timeout")}
	runner := NewWebSearchRunner(ms, "duckduckgo", config.WebProviders{})

	_, err := runner.Run(context.Background(), `{"query": "failing query"}`)
	if err == nil {
		t.Fatalf("Run() expected error from upstream, got nil")
	}
	if !strings.Contains(err.Error(), "upstream timeout") {
		t.Errorf("error %q should contain 'upstream timeout'", err.Error())
	}
}
