package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
)

// SearchOptions provides tuning parameters for search queries.
type SearchOptions struct {
	MaxResults int
	Timeout    time.Duration
}

// SearchResult represents a normalized item returned by a search provider.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// Searcher is the provider abstraction for executing web search queries.
type Searcher interface {
	Name() string
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
}

func validateQuery(query string) (string, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return "", fmt.Errorf("search query cannot be empty")
	}
	return trimmed, nil
}

func clampMaxResults(maxResults int) int {
	if maxResults <= 0 {
		return 5
	}
	if maxResults > 20 {
		return 20
	}
	return maxResults
}

// NewSearcher instantiates a Searcher according to the provider name and configuration.
func NewSearcher(provider string, cfg config.WebConfig) (Searcher, error) {
	prov := strings.ToLower(strings.TrimSpace(provider))
	if prov == "" {
		prov = strings.ToLower(strings.TrimSpace(cfg.DefaultProvider))
	}
	if prov == "" {
		prov = "duckduckgo"
	}

	switch prov {
	case "duckduckgo", "ddg":
		return NewDuckDuckGoSearcher(), nil
	case "brave":
		key := cfg.Providers.GetBraveAPIKey()
		return NewBraveSearcher(key), nil
	case "tavily":
		key := cfg.Providers.GetTavilyAPIKey()
		return NewTavilySearcher(key), nil
	case "searxng":
		baseURL := cfg.Providers.GetSearxngURL()
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}
		return NewSearXNGSearcher(baseURL), nil
	default:
		return nil, fmt.Errorf("unsupported search provider: %q", provider)
	}
}
