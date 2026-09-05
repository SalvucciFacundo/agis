package search_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/tools/web/search"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestSearch_EmptyQueryValidation(t *testing.T) {
	searchers := []struct {
		name string
		s    search.Searcher
	}{
		{"brave", search.NewBraveSearcher("test-key")},
		{"tavily", search.NewTavilySearcher("test-key")},
		{"searxng", search.NewSearXNGSearcher("http://localhost:8080")},
		{"duckduckgo", search.NewDuckDuckGoSearcher()},
	}

	invalidQueries := []string{"", "   ", "\t\n  "}

	for _, tc := range searchers {
		for _, q := range invalidQueries {
			t.Run(tc.name+"_query_"+q, func(t *testing.T) {
				_, err := tc.s.Search(context.Background(), q, search.SearchOptions{MaxResults: 5})
				if err == nil {
					t.Fatalf("expected error for empty query on %s, got nil", tc.name)
				}
				if !strings.Contains(strings.ToLower(err.Error()), "query") {
					t.Errorf("expected error message to mention 'query', got: %v", err)
				}
			})
		}
	}
}

func TestBraveSearcher(t *testing.T) {
	t.Run("missing API key returns error", func(t *testing.T) {
		s := search.NewBraveSearcher("")
		_, err := s.Search(context.Background(), "golang", search.SearchOptions{MaxResults: 5})
		if err == nil {
			t.Fatal("expected error for missing API key, got nil")
		}
	})

	t.Run("successful search query and result parsing", func(t *testing.T) {
		var receivedToken, receivedQuery, receivedCount string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedToken = r.Header.Get("X-Subscription-Token")
			receivedQuery = r.URL.Query().Get("q")
			receivedCount = r.URL.Query().Get("count")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"web": {
					"results": [
						{
							"title": "Go 1.26 Release Notes",
							"url": "https://go.dev/doc/go1.26",
							"description": "Go 1.26 introduces new runtime features."
						},
						{
							"title": "Go Packages",
							"url": "https://pkg.go.dev",
							"description": "Find Go packages and documentation."
						}
					]
				}
			}`))
		}))
		defer server.Close()

		s := search.NewBraveSearcher("test-brave-key", search.WithBraveEndpoint(server.URL))
		results, err := s.Search(context.Background(), "golang 1.26", search.SearchOptions{MaxResults: 2})
		if err != nil {
			t.Fatalf("Search unexpected error: %v", err)
		}

		if receivedToken != "test-brave-key" {
			t.Errorf("receivedToken = %q, want 'test-brave-key'", receivedToken)
		}
		if receivedQuery != "golang 1.26" {
			t.Errorf("receivedQuery = %q, want 'golang 1.26'", receivedQuery)
		}
		if receivedCount != "2" {
			t.Errorf("receivedCount = %q, want '2'", receivedCount)
		}

		if len(results) != 2 {
			t.Fatalf("len(results) = %d, want 2", len(results))
		}
		if results[0].Title != "Go 1.26 Release Notes" || results[0].URL != "https://go.dev/doc/go1.26" {
			t.Errorf("unexpected results[0]: %+v", results[0])
		}
		if results[0].Snippet != "Go 1.26 introduces new runtime features." {
			t.Errorf("unexpected snippet: %q", results[0].Snippet)
		}
	})

	t.Run("api returns 401 unauthorized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message": "Invalid subscription token"}`))
		}))
		defer server.Close()

		s := search.NewBraveSearcher("bad-token", search.WithBraveEndpoint(server.URL))
		_, err := s.Search(context.Background(), "golang", search.SearchOptions{MaxResults: 5})
		if err == nil {
			t.Fatal("expected error on 401, got nil")
		}
	})
}

func TestTavilySearcher(t *testing.T) {
	t.Run("missing API key returns error", func(t *testing.T) {
		s := search.NewTavilySearcher("")
		_, err := s.Search(context.Background(), "sqlite", search.SearchOptions{MaxResults: 5})
		if err == nil {
			t.Fatal("expected error for missing API key, got nil")
		}
	})

	t.Run("successful search POST request and parsing", func(t *testing.T) {
		var reqBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &reqBody)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"results": [
					{
						"title": "SQLite FTS5",
						"url": "https://sqlite.org/fts5.html",
						"content": "Full-text search engine extension."
					}
				]
			}`))
		}))
		defer server.Close()

		s := search.NewTavilySearcher("tvly-key-123", search.WithTavilyEndpoint(server.URL))
		results, err := s.Search(context.Background(), "sqlite fts5", search.SearchOptions{MaxResults: 3})
		if err != nil {
			t.Fatalf("Search error: %v", err)
		}

		if reqBody["api_key"] != "tvly-key-123" {
			t.Errorf("reqBody[api_key] = %v, want 'tvly-key-123'", reqBody["api_key"])
		}
		if reqBody["query"] != "sqlite fts5" {
			t.Errorf("reqBody[query] = %v, want 'sqlite fts5'", reqBody["query"])
		}

		if len(results) != 1 {
			t.Fatalf("len(results) = %d, want 1", len(results))
		}
		if results[0].Title != "SQLite FTS5" || results[0].Snippet != "Full-text search engine extension." {
			t.Errorf("unexpected results[0]: %+v", results[0])
		}
	})
}

func TestSearXNGSearcher(t *testing.T) {
	t.Run("successful search request and parsing", func(t *testing.T) {
		var receivedQ, receivedFormat string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedQ = r.URL.Query().Get("q")
			receivedFormat = r.URL.Query().Get("format")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"results": [
					{
						"title": "AGIS Autonomous Agent",
						"url": "https://github.com/SalvucciFacundo/agis",
						"content": "Personal AI agent architecture."
					}
				]
			}`))
		}))
		defer server.Close()

		s := search.NewSearXNGSearcher(server.URL)
		results, err := s.Search(context.Background(), "agis ai", search.SearchOptions{MaxResults: 5})
		if err != nil {
			t.Fatalf("Search error: %v", err)
		}

		if receivedQ != "agis ai" {
			t.Errorf("receivedQ = %q, want 'agis ai'", receivedQ)
		}
		if receivedFormat != "json" {
			t.Errorf("receivedFormat = %q, want 'json'", receivedFormat)
		}

		if len(results) != 1 {
			t.Fatalf("len(results) = %d, want 1", len(results))
		}
		if results[0].Title != "AGIS Autonomous Agent" || results[0].URL != "https://github.com/SalvucciFacundo/agis" {
			t.Errorf("unexpected results[0]: %+v", results[0])
		}
	})
}

func TestDuckDuckGoSearcher(t *testing.T) {
	t.Run("parses html results correctly", func(t *testing.T) {
		htmlContent := `<!DOCTYPE html>
<html>
<body>
  <div class="results">
    <div class="result results_links results_links_deep web-result">
      <h2 class="result__title">
        <a class="result__a" href="https://example.com/item1">First Search Result</a>
      </h2>
      <a class="result__snippet">This is the first snippet describing item 1.</a>
    </div>
    <div class="result results_links results_links_deep web-result">
      <h2 class="result__title">
        <a class="result__a" href="https://example.com/item2">Second Search Result</a>
      </h2>
      <a class="result__snippet">This is the second snippet describing item 2.</a>
    </div>
  </div>
</body>
</html>`

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(htmlContent))
		}))
		defer server.Close()

		s := search.NewDuckDuckGoSearcher(search.WithDuckDuckGoEndpoint(server.URL))
		results, err := s.Search(context.Background(), "test query", search.SearchOptions{MaxResults: 5})
		if err != nil {
			t.Fatalf("Search unexpected error: %v", err)
		}

		if len(results) != 2 {
			t.Fatalf("len(results) = %d, want 2", len(results))
		}
		if results[0].Title != "First Search Result" || results[0].URL != "https://example.com/item1" {
			t.Errorf("unexpected results[0]: %+v", results[0])
		}
		if results[0].Snippet != "This is the first snippet describing item 1." {
			t.Errorf("unexpected snippet[0]: %q", results[0].Snippet)
		}
		if results[1].Title != "Second Search Result" || results[1].URL != "https://example.com/item2" {
			t.Errorf("unexpected results[1]: %+v", results[1])
		}
	})

	t.Run("handles rate limit 429", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`Rate limit exceeded`))
		}))
		defer server.Close()

		s := search.NewDuckDuckGoSearcher(search.WithDuckDuckGoEndpoint(server.URL))
		_, err := s.Search(context.Background(), "test query", search.SearchOptions{MaxResults: 5})
		if err == nil {
			t.Fatal("expected error on 429 rate limit, got nil")
		}
	})

	t.Run("parses uddg encoded links and lite table elements", func(t *testing.T) {
		htmlContent := `<!DOCTYPE html>
<html>
<body>
  <table>
    <tr class="result-default">
      <td>
        <a class="result-link" href="/l/?uddg=https%3A%2F%2Fgolang.org%2Fpkg&rut=123">Go Standard Library</a>
      </td>
      <td class="result-snippet">
        Package documentation for the Go standard library.
      </td>
    </tr>
  </table>
</body>
</html>`

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(htmlContent))
		}))
		defer server.Close()

		s := search.NewDuckDuckGoSearcher(search.WithDuckDuckGoEndpoint(server.URL))
		results, err := s.Search(context.Background(), "golang docs", search.SearchOptions{MaxResults: 5})
		if err != nil {
			t.Fatalf("Search unexpected error: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("len(results) = %d, want 1", len(results))
		}
		if results[0].URL != "https://golang.org/pkg" {
			t.Errorf("results[0].URL = %q, want 'https://golang.org/pkg'", results[0].URL)
		}
		if results[0].Title != "Go Standard Library" {
			t.Errorf("results[0].Title = %q, want 'Go Standard Library'", results[0].Title)
		}
		if results[0].Snippet != "Package documentation for the Go standard library." {
			t.Errorf("results[0].Snippet = %q", results[0].Snippet)
		}
	})

	t.Run("empty html returns zero results without error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html><body><div class="result--no-result">No results</div></body></html>`))
		}))
		defer server.Close()

		s := search.NewDuckDuckGoSearcher(search.WithDuckDuckGoEndpoint(server.URL))
		results, err := s.Search(context.Background(), "no results query", search.SearchOptions{MaxResults: 5})
		if err != nil {
			t.Fatalf("Search unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("len(results) = %d, want 0", len(results))
		}
	})
}

func TestNewSearcher_Factory(t *testing.T) {
	cfg := config.WebConfig{
		DefaultProvider: "duckduckgo",
		Providers: config.WebProviders{
			Brave:   config.ProviderConfig{APIKey: "brave-key"},
			Tavily:  config.ProviderConfig{APIKey: "tavily-key"},
			Searxng: config.ProviderConfig{BaseURL: "http://localhost:8080"},
		},
	}

	tests := []struct {
		name         string
		providerName string
		wantName     string
		wantErr      bool
	}{
		{
			name:         "empty provider defaults to duckduckgo",
			providerName: "",
			wantName:     "duckduckgo",
		},
		{
			name:         "brave provider",
			providerName: "brave",
			wantName:     "brave",
		},
		{
			name:         "tavily provider",
			providerName: "tavily",
			wantName:     "tavily",
		},
		{
			name:         "searxng provider",
			providerName: "searxng",
			wantName:     "searxng",
		},
		{
			name:         "duckduckgo provider explicit",
			providerName: "duckduckgo",
			wantName:     "duckduckgo",
		},
		{
			name:         "unsupported provider returns error",
			providerName: "google_custom_search",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := search.NewSearcher(tt.providerName, cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tt.providerName)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.providerName, err)
			}
			if s.Name() != tt.wantName {
				t.Errorf("s.Name() = %q, want %q", s.Name(), tt.wantName)
			}
		})
	}
}

func TestSearch_TimeoutCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results": []}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	s := search.NewSearXNGSearcher(server.URL)
	_, err := s.Search(ctx, "timeout test", search.SearchOptions{MaxResults: 5})
	if err == nil {
		t.Fatal("expected deadline exceeded error, got nil")
	}
}
