package fetch_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/tools/web/fetch"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestFetcher_FetchMarkdown_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Test</title><script>console.log("bad")</script></head>
<body>
	<h1>Article Headline</h1>
	<p>Welcome to AGIS web fetcher. Read more <a href="https://example.com">here</a>.</p>
</body>
</html>`))
	}))
	defer server.Close()

	fetcher := fetch.NewFetcher(fetch.FetchOptions{
		Timeout:         5 * time.Second,
		MaxBytes:        2 * 1024 * 1024,
		UserAgent:       "AGIS/1.0-Test",
		AllowPrivateIPs: true, // required for httptest loopback server
	})

	ctx := context.Background()
	got, err := fetcher.FetchMarkdown(ctx, server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "# Article Headline\n\nWelcome to AGIS web fetcher. Read more [here](https://example.com)."
	if got != expected {
		t.Errorf("FetchMarkdown() =\n%q\nwant:\n%q", got, expected)
	}
}

func TestFetcher_Fetch_RawMode(t *testing.T) {
	rawJSON := `{"status":"ok","items":[1,2,3]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(rawJSON))
	}))
	defer server.Close()

	fetcher := fetch.NewFetcher(fetch.FetchOptions{
		Timeout:         5 * time.Second,
		AllowPrivateIPs: true,
	})

	ctx := context.Background()
	got, err := fetcher.Fetch(ctx, server.URL, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != rawJSON {
		t.Errorf("Fetch(raw=true) = %q, want %q", got, rawJSON)
	}
}

func TestFetcher_SizeLimitGuard(t *testing.T) {
	// Create server that serves a 500KB payload
	largeData := strings.Repeat("A", 500*1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(largeData))
	}))
	defer server.Close()

	// Configure fetcher with a 50KB limit
	maxBytes := int64(50 * 1024)
	fetcher := fetch.NewFetcher(fetch.FetchOptions{
		Timeout:         5 * time.Second,
		MaxBytes:        maxBytes,
		AllowPrivateIPs: true,
	})

	ctx := context.Background()
	got, err := fetcher.Fetch(ctx, server.URL, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if int64(len(got)) > maxBytes {
		t.Errorf("Fetch output length = %d bytes, exceeded limit of %d bytes", len(got), maxBytes)
	}
}

func TestFetcher_ContentTypeValidation(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        []byte
		expectError bool
	}{
		{
			name:        "html content type allowed",
			contentType: "text/html; charset=utf-8",
			body:        []byte("<h1>Hello</h1>"),
			expectError: false,
		},
		{
			name:        "json content type allowed",
			contentType: "application/json",
			body:        []byte(`{"key":"value"}`),
			expectError: false,
		},
		{
			name:        "plain text allowed",
			contentType: "text/plain",
			body:        []byte("plain text content"),
			expectError: false,
		},
		{
			name:        "zip binary rejected",
			contentType: "application/zip",
			body:        []byte("PK\x03\x04..."),
			expectError: true,
		},
		{
			name:        "png image rejected",
			contentType: "image/png",
			body:        []byte("\x89PNG\r\n\x1a\n..."),
			expectError: true,
		},
		{
			name:        "octet-stream binary rejected",
			contentType: "application/octet-stream",
			body:        []byte("\x00\x01\x02\x03"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				_, _ = w.Write(tt.body)
			}))
			defer server.Close()

			fetcher := fetch.NewFetcher(fetch.FetchOptions{
				Timeout:         5 * time.Second,
				AllowPrivateIPs: true,
			})

			ctx := context.Background()
			_, err := fetcher.Fetch(ctx, server.URL, false)
			if (err != nil) != tt.expectError {
				t.Errorf("Fetch() with Content-Type %q error = %v, want error: %v", tt.contentType, err, tt.expectError)
			}
		})
	}
}

func TestFetcher_UserAgentHeader(t *testing.T) {
	customUA := "AGIS-Custom-Agent/2.0"
	var receivedUA string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	fetcher := fetch.NewFetcher(fetch.FetchOptions{
		Timeout:         5 * time.Second,
		UserAgent:       customUA,
		AllowPrivateIPs: true,
	})

	ctx := context.Background()
	_, err := fetcher.Fetch(ctx, server.URL, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedUA != customUA {
		t.Errorf("received User-Agent = %q, want %q", receivedUA, customUA)
	}
}

func TestFetcher_TimeoutHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte("delayed response"))
	}))
	defer server.Close()

	fetcher := fetch.NewFetcher(fetch.FetchOptions{
		Timeout:         50 * time.Millisecond,
		AllowPrivateIPs: true,
	})

	ctx := context.Background()
	_, err := fetcher.Fetch(ctx, server.URL, false)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestFetcher_HTTPErrorStatuses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer server.Close()

	fetcher := fetch.NewFetcher(fetch.FetchOptions{
		Timeout:         5 * time.Second,
		AllowPrivateIPs: true,
	})

	ctx := context.Background()
	_, err := fetcher.Fetch(ctx, server.URL, false)
	if err == nil {
		t.Fatal("expected HTTP 404 error, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error to mention 404, got %q", err.Error())
	}
}

func TestFetcher_RedirectLimit(t *testing.T) {
	redirectCount := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCount++
		// Endless redirect loop
		http.Redirect(w, r, fmt.Sprintf("%s/redirect/%d", server.URL, redirectCount), http.StatusFound)
	}))
	defer server.Close()

	fetcher := fetch.NewFetcher(fetch.FetchOptions{
		Timeout:         5 * time.Second,
		AllowPrivateIPs: true,
	})

	ctx := context.Background()
	_, err := fetcher.Fetch(ctx, server.URL, false)
	if err == nil {
		t.Fatal("expected redirect limit error, got nil")
	}
}

func TestFetcher_SSRFBlocked(t *testing.T) {
	fetcher := fetch.NewFetcher(fetch.FetchOptions{
		Timeout:         5 * time.Second,
		AllowPrivateIPs: false, // Strict SSRF mode
	})

	ctx := context.Background()
	// Attempting to fetch 127.0.0.1 should be blocked before dial or at dial time
	_, err := fetcher.Fetch(ctx, "http://127.0.0.1:8080/admin", false)
	if err == nil {
		t.Fatal("expected SSRF blocked error, got nil")
	}
}
