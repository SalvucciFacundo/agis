package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultTavilyEndpoint = "https://api.tavily.com/search"

// TavilyOption configures a TavilySearcher instance.
type TavilyOption func(*TavilySearcher)

// TavilySearcher executes searches using the Tavily API.
type TavilySearcher struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

// WithTavilyEndpoint overrides the target Tavily Search API URL (useful for testing).
func WithTavilyEndpoint(endpoint string) TavilyOption {
	return func(s *TavilySearcher) {
		s.endpoint = endpoint
	}
}

// WithTavilyClient overrides the HTTP client for Tavily Search.
func WithTavilyClient(client *http.Client) TavilyOption {
	return func(s *TavilySearcher) {
		if client != nil {
			s.client = client
		}
	}
}

// NewTavilySearcher creates a new TavilySearcher with the provided API key.
func NewTavilySearcher(apiKey string, opts ...TavilyOption) *TavilySearcher {
	s := &TavilySearcher{
		apiKey:   apiKey,
		endpoint: defaultTavilyEndpoint,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Name returns the provider name.
func (t *TavilySearcher) Name() string {
	return "tavily"
}

type tavilyRequestPayload struct {
	APIKey     string `json:"api_key"`
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

type tavilyAPIResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
	Detail string `json:"detail,omitempty"`
}

// Search executes a query against the Tavily API.
func (t *TavilySearcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	q, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	if t.apiKey == "" {
		return nil, fmt.Errorf("tavily search API key is not configured")
	}

	maxResults := clampMaxResults(opts.MaxResults)
	payload := tavilyRequestPayload{
		APIKey:     t.apiKey,
		Query:      q,
		MaxResults: maxResults,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling tavily request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("creating tavily search request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing tavily search request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading tavily search response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp tavilyAPIResponse
		_ = json.Unmarshal(body, &errResp)
		msg := errResp.Detail
		if msg == "" {
			msg = string(body)
		}
		return nil, fmt.Errorf("tavily search failed with status %d: %s", resp.StatusCode, msg)
	}

	var data tavilyAPIResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parsing tavily search JSON response: %w", err)
	}

	results := make([]SearchResult, 0, len(data.Results))
	for _, r := range data.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}

	return results, nil
}
