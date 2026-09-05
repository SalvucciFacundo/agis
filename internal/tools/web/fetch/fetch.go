package fetch

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultFetchTimeout is the default duration before an HTTP fetch times out.
	DefaultFetchTimeout = 15 * time.Second

	// DefaultMaxFetchBytes is the default byte cap (2MB) for fetched responses.
	DefaultMaxFetchBytes = int64(2 * 1024 * 1024)

	// MaxAllowedFetchBytes is the maximum allowed byte cap (10MB) for fetched responses.
	MaxAllowedFetchBytes = int64(10 * 1024 * 1024)

	// DefaultUserAgent is the HTTP User-Agent header value sent by AGIS.
	DefaultUserAgent = "AGIS/1.0 (+https://github.com/SalvucciFacundo/agis)"

	// MaxRedirects is the maximum number of HTTP redirects followed before failing.
	MaxRedirects = 10
)

// FetchOptions contains configuration for HTTP fetching.
type FetchOptions struct {
	Timeout         time.Duration
	MaxBytes        int64
	UserAgent       string
	AllowPrivateIPs bool
}

// Fetcher encapsulates HTTP fetching, size guards, and content extraction.
type Fetcher struct {
	client *http.Client
	opts   FetchOptions
}

// NewFetcher creates an initialized Fetcher with safe defaults and SSRF protection.
func NewFetcher(opts FetchOptions) *Fetcher {
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultFetchTimeout
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = DefaultMaxFetchBytes
	} else if opts.MaxBytes > MaxAllowedFetchBytes {
		opts.MaxBytes = MaxAllowedFetchBytes
	}
	if opts.UserAgent == "" {
		opts.UserAgent = DefaultUserAgent
	}

	transport := NewSafeTransport(opts.AllowPrivateIPs, opts.Timeout)

	client := &http.Client{
		Transport: transport,
		Timeout:   opts.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", MaxRedirects)
			}
			// Validate redirect destination
			if err := ValidateURL(req.URL.String(), opts.AllowPrivateIPs); err != nil {
				return fmt.Errorf("redirect blocked: %w", err)
			}
			return nil
		},
	}

	return &Fetcher{
		client: client,
		opts:   opts,
	}
}

// Fetch retrieves a web page and returns its content. When raw is false, HTML is converted to Markdown.
func (f *Fetcher) Fetch(ctx context.Context, targetURL string, raw bool) (string, error) {
	if err := ValidateURL(targetURL, f.opts.AllowPrivateIPs); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", f.opts.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json,text/plain;q=0.9,*/*;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed for %q: %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("http %d %s fetching %q", resp.StatusCode, http.StatusText(resp.StatusCode), targetURL)
	}

	// Validate Content-Type
	contentType := resp.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType == "" {
		mediaType = "text/html" // Default assumption
	}

	if isBinaryMediaType(mediaType) {
		return "", fmt.Errorf("unsupported binary content type %q; cannot render as text", mediaType)
	}

	// Guard size with LimitReader
	limitReader := io.LimitReader(resp.Body, f.opts.MaxBytes)

	if raw {
		bodyBytes, err := io.ReadAll(limitReader)
		if err != nil {
			return "", fmt.Errorf("failed to read response: %w", err)
		}
		return string(bodyBytes), nil
	}

	// If HTML, extract markdown
	if strings.Contains(mediaType, "html") || mediaType == "application/xhtml+xml" {
		md, err := ExtractMarkdown(limitReader)
		if err != nil {
			return "", fmt.Errorf("failed to extract markdown from html: %w", err)
		}
		return md, nil
	}

	// Plain text / JSON / CSV
	bodyBytes, err := io.ReadAll(limitReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	return string(bodyBytes), nil
}

// FetchMarkdown retrieves a web page and extracts its content as Markdown.
func (f *Fetcher) FetchMarkdown(ctx context.Context, targetURL string) (string, error) {
	return f.Fetch(ctx, targetURL, false)
}

func isBinaryMediaType(mediaType string) bool {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if strings.HasPrefix(mediaType, "image/") ||
		strings.HasPrefix(mediaType, "video/") ||
		strings.HasPrefix(mediaType, "audio/") {
		return true
	}

	switch mediaType {
	case "application/octet-stream",
		"application/zip",
		"application/x-gzip",
		"application/x-tar",
		"application/x-bzip2",
		"application/pdf",
		"application/x-shockwave-flash",
		"application/wasm":
		return true
	}

	return false
}
