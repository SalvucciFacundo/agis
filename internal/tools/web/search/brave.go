package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultBraveEndpoint = "https://api.search.brave.com/res/v1/web/search"

// BraveOption configures a BraveSearcher instance.
type BraveOption func(*BraveSearcher)

// BraveSearcher executes searches using the Brave Search API.
type BraveSearcher struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

// WithBraveEndpoint overrides the target Brave Search API URL (useful for testing).
func WithBraveEndpoint(endpoint string) BraveOption {
	return func(s *BraveSearcher) {
		s.endpoint = endpoint
	}
}

// WithBraveClient overrides the HTTP client for Brave Search.
func WithBraveClient(client *http.Client) BraveOption {
	return func(s *BraveSearcher) {
		if client != nil {
			s.client = client
		}
	}
}

// NewBraveSearcher creates a new BraveSearcher with the provided API key.
func NewBraveSearcher(apiKey string, opts ...BraveOption) *BraveSearcher {
	s := &BraveSearcher{
		apiKey:   apiKey,
		endpoint: defaultBraveEndpoint,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Name returns the provider name.
func (b *BraveSearcher) Name() string {
	return "brave"
}

type braveAPIResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
	Message string `json:"message,omitempty"`
}

// Search executes a query against the Brave Search API.
func (b *BraveSearcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	q, err := validateQuery(query)
	if err != nil {
		return nil, err
	}
	if b.apiKey == "" {
		return nil, fmt.Errorf("brave search API key is not configured")
	}

	maxResults := clampMaxResults(opts.MaxResults)
	reqURL, err := url.Parse(b.endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing brave endpoint %q: %w", b.endpoint, err)
	}

	values := reqURL.Query()
	values.Set("q", q)
	values.Set("count", strconv.Itoa(maxResults))
	reqURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating brave search request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", b.apiKey)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing brave search request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading brave search response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp braveAPIResponse
		_ = json.Unmarshal(body, &errResp)
		msg := errResp.Message
		if msg == "" {
			msg = string(body)
		}
		return nil, fmt.Errorf("brave search failed with status %d: %s", resp.StatusCode, msg)
	}

	var data braveAPIResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parsing brave search JSON response: %w", err)
	}

	results := make([]SearchResult, 0, len(data.Web.Results))
	for _, r := range data.Web.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Description,
		})
	}

	return results, nil
}
