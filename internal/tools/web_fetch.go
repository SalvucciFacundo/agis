package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/tools/web/fetch"
)

// WebFetchRunner fetches web content and converts it to Markdown or returns raw response.
type WebFetchRunner struct {
	fetcher         *fetch.Fetcher
	defaultMaxBytes int64
}

// NewWebFetchRunner creates an initialized WebFetchRunner.
func NewWebFetchRunner(fetcher *fetch.Fetcher, defaultMaxBytes int64) *WebFetchRunner {
	if defaultMaxBytes <= 0 {
		defaultMaxBytes = fetch.DefaultMaxFetchBytes
	}
	return &WebFetchRunner{
		fetcher:         fetcher,
		defaultMaxBytes: defaultMaxBytes,
	}
}

// Backend implements core.ToolRunner.
func (r *WebFetchRunner) Backend() string {
	return "web"
}

// Name implements core.ToolRunner.
func (r *WebFetchRunner) Name() string {
	return "web_fetch"
}

// Description implements core.ToolRunner.
func (r *WebFetchRunner) Description() string {
	return "Fetch a web page by URL and extract its main readable text content converted to clean Markdown. Strips navigation, scripts, styles, and boilerplate."
}

type fetchArgs struct {
	URL      string `json:"url"`
	MaxBytes int64  `json:"max_bytes"`
	Raw      bool   `json:"raw"`
}

func parseFetchArgs(input string, defaultMaxBytes int64) (fetchArgs, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return fetchArgs{}, fmt.Errorf("url is required")
	}

	var args fetchArgs
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
			return fetchArgs{}, fmt.Errorf("invalid json arguments: %w", err)
		}
	} else {
		args.URL = trimmed
	}

	args.URL = strings.TrimSpace(args.URL)
	if args.URL == "" {
		return fetchArgs{}, fmt.Errorf("url is required")
	}

	if args.MaxBytes <= 0 {
		args.MaxBytes = defaultMaxBytes
	} else if args.MaxBytes > fetch.MaxAllowedFetchBytes {
		args.MaxBytes = fetch.MaxAllowedFetchBytes
	}

	return args, nil
}

// Run fetches the target URL and returns its Markdown or raw content.
func (r *WebFetchRunner) Run(ctx context.Context, command string) (string, error) {
	args, err := parseFetchArgs(command, r.defaultMaxBytes)
	if err != nil {
		return "", err
	}

	if r.fetcher == nil {
		return "", fmt.Errorf("fetcher is not configured")
	}

	return r.fetcher.Fetch(ctx, args.URL, args.Raw)
}
