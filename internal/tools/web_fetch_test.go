package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/tools/web/fetch"
)

func TestWebFetchRunner_Contract(t *testing.T) {
	fetcher := fetch.NewFetcher(fetch.FetchOptions{})
	runner := NewWebFetchRunner(fetcher, 2097152)

	if runner.Backend() != "web" {
		t.Errorf("Backend() = %q, want %q", runner.Backend(), "web")
	}
	if runner.Name() != "web_fetch" {
		t.Errorf("Name() = %q, want %q", runner.Name(), "web_fetch")
	}
	if !strings.Contains(runner.Description(), "Fetch a web page") {
		t.Errorf("Description() = %q, want containing 'Fetch a web page'", runner.Description())
	}
}

func TestWebFetchRunner_Run_ValidURLMarkdown(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Test Page</title><script>var x = 1;</script></head>
<body>
<nav><a href="/">Home</a></nav>
<h1>Main Title</h1>
<p>Hello <strong>World</strong> from test server.</p>
</body>
</html>`))
	}))
	defer ts.Close()

	fetcher := fetch.NewFetcher(fetch.FetchOptions{
		Timeout:         5 * time.Second,
		MaxBytes:        1024 * 1024,
		AllowPrivateIPs: true,
	})
	runner := NewWebFetchRunner(fetcher, 1024*1024)

	ctx := context.Background()
	input := `{"url": "` + ts.URL + `"}`
	out, err := runner.Run(ctx, input)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !strings.Contains(out, "# Main Title") {
		t.Errorf("markdown output missing '# Main Title':\n%s", out)
	}
	if !strings.Contains(out, "Hello **World** from test server.") {
		t.Errorf("markdown output missing bold text:\n%s", out)
	}
	if strings.Contains(out, "var x = 1") {
		t.Errorf("markdown output should not contain script contents")
	}
}

func TestWebFetchRunner_Run_RawMode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status": "ok", "version": "1.0.0"}`))
	}))
	defer ts.Close()

	fetcher := fetch.NewFetcher(fetch.FetchOptions{
		Timeout:         5 * time.Second,
		MaxBytes:        1024 * 1024,
		AllowPrivateIPs: true,
	})
	runner := NewWebFetchRunner(fetcher, 1024*1024)

	ctx := context.Background()
	input := `{"url": "` + ts.URL + `", "raw": true}`
	out, err := runner.Run(ctx, input)
	if err != nil {
		t.Fatalf("Run() raw mode error = %v", err)
	}

	if strings.TrimSpace(out) != `{"status": "ok", "version": "1.0.0"}` {
		t.Errorf("raw output = %q, want json string", out)
	}
}

func TestWebFetchRunner_Run_PlainTextInput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<h1>Simple</h1>`))
	}))
	defer ts.Close()

	fetcher := fetch.NewFetcher(fetch.FetchOptions{
		Timeout:         5 * time.Second,
		MaxBytes:        1024 * 1024,
		AllowPrivateIPs: true,
	})
	runner := NewWebFetchRunner(fetcher, 1024*1024)

	out, err := runner.Run(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Run() with raw URL string error = %v", err)
	}
	if !strings.Contains(out, "# Simple") {
		t.Errorf("output missing '# Simple': %s", out)
	}
}

func TestWebFetchRunner_Run_InvalidURLScheme(t *testing.T) {
	fetcher := fetch.NewFetcher(fetch.FetchOptions{})
	runner := NewWebFetchRunner(fetcher, 2097152)

	cases := []struct {
		name  string
		input string
	}{
		{"file scheme", `{"url": "file:///etc/passwd"}`},
		{"ftp scheme", `{"url": "ftp://ftp.example.com/file"}`},
		{"empty url", `{"url": ""}`},
		{"invalid url", `{"url": "not-a-url"}`},
		{"empty string", `""`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runner.Run(context.Background(), tc.input)
			if err == nil {
				t.Fatalf("Run(%q) expected error for invalid URL, got nil", tc.input)
			}
		})
	}
}

func TestWebFetchRunner_Run_MaxBytesClamping(t *testing.T) {
	args, err := parseFetchArgs(`{"url": "https://example.com", "max_bytes": 20000000}`, 2097152)
	if err != nil {
		t.Fatalf("parseFetchArgs error: %v", err)
	}
	// Max allowed clamped to 10MB (10485760)
	if args.MaxBytes > 10485760 {
		t.Errorf("args.MaxBytes = %d, want <= 10485760", args.MaxBytes)
	}
}
