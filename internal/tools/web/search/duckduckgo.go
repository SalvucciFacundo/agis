package search

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const defaultDuckDuckGoEndpoint = "https://html.duckduckgo.com/html/"
const defaultBrowserUserAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0"

// DuckDuckGoOption configures a DuckDuckGoSearcher instance.
type DuckDuckGoOption func(*DuckDuckGoSearcher)

// DuckDuckGoSearcher executes searches using the DuckDuckGo HTML endpoint without requiring API keys.
type DuckDuckGoSearcher struct {
	endpoint  string
	userAgent string
	client    *http.Client
}

// WithDuckDuckGoEndpoint overrides the target DuckDuckGo URL (useful for testing).
func WithDuckDuckGoEndpoint(endpoint string) DuckDuckGoOption {
	return func(s *DuckDuckGoSearcher) {
		s.endpoint = endpoint
	}
}

// WithDuckDuckGoUserAgent overrides the HTTP User-Agent header for DuckDuckGo.
func WithDuckDuckGoUserAgent(ua string) DuckDuckGoOption {
	return func(s *DuckDuckGoSearcher) {
		if ua != "" {
			s.userAgent = ua
		}
	}
}

// WithDuckDuckGoClient overrides the HTTP client for DuckDuckGo.
func WithDuckDuckGoClient(client *http.Client) DuckDuckGoOption {
	return func(s *DuckDuckGoSearcher) {
		if client != nil {
			s.client = client
		}
	}
}

// NewDuckDuckGoSearcher creates a new DuckDuckGoSearcher.
func NewDuckDuckGoSearcher(opts ...DuckDuckGoOption) *DuckDuckGoSearcher {
	s := &DuckDuckGoSearcher{
		endpoint:  defaultDuckDuckGoEndpoint,
		userAgent: defaultBrowserUserAgent,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Name returns the provider name.
func (d *DuckDuckGoSearcher) Name() string {
	return "duckduckgo"
}

// Search executes an HTML search query against DuckDuckGo.
func (d *DuckDuckGoSearcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	q, err := validateQuery(query)
	if err != nil {
		return nil, err
	}

	reqURL, err := url.Parse(d.endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing duckduckgo endpoint %q: %w", d.endpoint, err)
	}

	values := reqURL.Query()
	values.Set("q", q)
	reqURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating duckduckgo request: %w", err)
	}

	req.Header.Set("User-Agent", d.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing duckduckgo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusAccepted {
		return nil, fmt.Errorf("duckduckgo rate limit exceeded (status %d)", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("duckduckgo search failed with status %d: %s", resp.StatusCode, string(body))
	}

	doc, err := html.Parse(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("parsing duckduckgo html: %w", err)
	}

	maxResults := clampMaxResults(opts.MaxResults)
	results := parseDuckDuckGoHTML(doc, maxResults)
	return results, nil
}

func parseDuckDuckGoHTML(doc *html.Node, maxResults int) []SearchResult {
	results := make([]SearchResult, 0, maxResults)

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if len(results) >= maxResults {
			return
		}

		if n.Type == html.ElementNode {
			class := getAttr(n, "class")
			if isDDGResultContainer(class) {
				res := extractSingleDDGResult(n)
				if res.URL != "" && res.Title != "" {
					results = append(results, res)
					return // Do not recurse into children of an extracted result
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(doc)
	return results
}

func isDDGResultContainer(class string) bool {
	if strings.Contains(class, "results_links_more") || strings.Contains(class, "result--no-result") {
		return false
	}
	if strings.Contains(class, "web-result") || strings.Contains(class, "result__body") || strings.Contains(class, "result-default") {
		return true
	}
	for _, f := range strings.Fields(class) {
		if f == "result" {
			return true
		}
	}
	return false
}

func extractSingleDDGResult(resultNode *html.Node) SearchResult {
	var res SearchResult

	var scan func(*html.Node)
	scan = func(n *html.Node) {
		if n.Type == html.ElementNode {
			class := getAttr(n, "class")
			if (strings.Contains(class, "result__a") || strings.Contains(class, "result-link")) && res.Title == "" {
				res.Title = strings.TrimSpace(extractText(n))
				rawHref := getAttr(n, "href")
				res.URL = cleanDDGURL(rawHref)
			}
			if (strings.Contains(class, "result__snippet") || strings.Contains(class, "result-snippet")) && res.Snippet == "" {
				res.Snippet = strings.TrimSpace(extractText(n))
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			scan(c)
		}
	}

	scan(resultNode)
	return res
}

func cleanDDGURL(rawHref string) string {
	if rawHref == "" {
		return ""
	}
	if strings.Contains(rawHref, "uddg=") {
		u, err := url.Parse(rawHref)
		if err == nil {
			if target := u.Query().Get("uddg"); target != "" {
				return target
			}
		}
	}
	return rawHref
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func extractText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(curr *html.Node) {
		if curr.Type == html.TextNode {
			sb.WriteString(curr.Data)
		}
		for c := curr.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}
