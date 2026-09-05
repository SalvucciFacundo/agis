package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SearXNGOption configures a SearXNGSearcher instance.
type SearXNGOption func(*SearXNGSearcher)

// SearXNGSearcher executes searches using a SearXNG instance.
type SearXNGSearcher struct {
	baseURL string
	client  *http.Client
}

// WithSearXNGClient overrides the HTTP client for SearXNG Search.
func WithSearXNGClient(client *http.Client) SearXNGOption {
	return func(s *SearXNGSearcher) {
		if client != nil {
			s.client = client
		}
	}
}

// NewSearXNGSearcher creates a new SearXNGSearcher targeting baseURL.
func NewSearXNGSearcher(baseURL string, opts ...SearXNGOption) *SearXNGSearcher {
	trimmed := strings.TrimRight(baseURL, "/")
	if trimmed == "" {
		trimmed = "http://localhost:8080"
	}
	s := &SearXNGSearcher{
		baseURL: trimmed,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Name returns the provider name.
func (s *SearXNGSearcher) Name() string {
	return "searxng"
}

type searxngAPIResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}

// Search executes a query against SearXNG.
func (s *SearXNGSearcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	q, err := validateQuery(query)
	if err != nil {
		return nil, err
	}

	endpoint := s.baseURL
	if !strings.HasSuffix(endpoint, "/search") {
		endpoint += "/search"
	}

	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing searxng endpoint %q: %w", endpoint, err)
	}

	values := reqURL.Query()
	values.Set("q", q)
	values.Set("format", "json")
	reqURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating searxng request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing searxng search request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading searxng search response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("searxng search failed with status %d: %s", resp.StatusCode, string(body))
	}

	var data searxngAPIResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parsing searxng search JSON response: %w", err)
	}

	maxResults := clampMaxResults(opts.MaxResults)
	results := make([]SearchResult, 0, len(data.Results))
	for i, r := range data.Results {
		if i >= maxResults {
			break
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}

	return results, nil
}
