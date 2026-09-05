package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/tools/web/search"
)

// WebSearchRunner executes web search queries across supported search providers.
type WebSearchRunner struct {
	searcher        search.Searcher
	defaultProvider string
	providers       config.WebProviders
	providerFactory func(provider string, cfg config.WebConfig) (search.Searcher, error)
}

// NewWebSearchRunner creates an initialized WebSearchRunner.
func NewWebSearchRunner(searcher search.Searcher, defaultProvider string, providers config.WebProviders) *WebSearchRunner {
	if defaultProvider == "" {
		defaultProvider = "duckduckgo"
	}
	return &WebSearchRunner{
		searcher:        searcher,
		defaultProvider: defaultProvider,
		providers:       providers,
		providerFactory: search.NewSearcher,
	}
}

// Backend implements core.ToolRunner.
func (r *WebSearchRunner) Backend() string {
	return "web"
}

// Name implements core.ToolRunner.
func (r *WebSearchRunner) Name() string {
	return "web_search"
}

// Description implements core.ToolRunner.
func (r *WebSearchRunner) Description() string {
	return "Search the web for real-time information, documentation, news, and technical references. Returns top search results containing title, URL, and snippet."
}

type searchArgs struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
	Provider   string `json:"provider"`
}

func parseSearchArgs(input string) (searchArgs, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return searchArgs{}, fmt.Errorf("search query is required")
	}

	var args searchArgs
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
			return searchArgs{}, fmt.Errorf("invalid json arguments: %w", err)
		}
	} else {
		args.Query = trimmed
	}

	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return searchArgs{}, fmt.Errorf("search query is required")
	}

	if args.MaxResults <= 0 {
		args.MaxResults = 5
	} else if args.MaxResults > 20 {
		args.MaxResults = 20
	}

	return args, nil
}

// Run executes a web search query with optional JSON parameters.
func (r *WebSearchRunner) Run(ctx context.Context, command string) (string, error) {
	args, err := parseSearchArgs(command)
	if err != nil {
		return "", err
	}

	searcher := r.searcher
	if args.Provider != "" && strings.ToLower(args.Provider) != strings.ToLower(r.defaultProvider) {
		cfg := config.WebConfig{
			DefaultProvider: args.Provider,
			Providers:       r.providers,
		}
		override, err := r.providerFactory(args.Provider, cfg)
		if err != nil {
			return "", fmt.Errorf("search provider %q error: %w", args.Provider, err)
		}
		searcher = override
	}

	if searcher == nil {
		return "", fmt.Errorf("no searcher available for provider %q", r.defaultProvider)
	}

	opts := search.SearchOptions{
		MaxResults: args.MaxResults,
	}

	results, err := searcher.Search(ctx, args.Query, opts)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return "[]", nil
	}

	outBytes, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("failed to format search results: %w", err)
	}

	return string(outBytes), nil
}
