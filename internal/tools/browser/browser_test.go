package browser

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
)

type mockDriver struct {
	navFunc        func(ctx context.Context, targetURL string) (*PageResult, error)
	actionFunc     func(ctx context.Context, req ActionRequest) (*ActionResult, error)
	screenshotFunc func(ctx context.Context, targetURL, destPath string) (string, error)
	closeFunc      func() error
}

func (m *mockDriver) Navigate(ctx context.Context, targetURL string) (*PageResult, error) {
	if m.navFunc != nil {
		return m.navFunc(ctx, targetURL)
	}
	return &PageResult{
		URL:     targetURL,
		Title:   "Mock Title",
		Content: "# Mock Content\n\nHello world",
	}, nil
}

func (m *mockDriver) Action(ctx context.Context, req ActionRequest) (*ActionResult, error) {
	if m.actionFunc != nil {
		return m.actionFunc(ctx, req)
	}
	return &ActionResult{
		Success: true,
		Output:  "action completed: " + req.Action,
	}, nil
}

func (m *mockDriver) Screenshot(ctx context.Context, targetURL, destPath string) (string, error) {
	if m.screenshotFunc != nil {
		return m.screenshotFunc(ctx, targetURL, destPath)
	}
	return destPath, nil
}

func (m *mockDriver) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestBrowserNavigateRunner(t *testing.T) {
	mock := &mockDriver{
		navFunc: func(ctx context.Context, targetURL string) (*PageResult, error) {
			if targetURL == "https://fail.com" {
				return nil, errors.New("connection refused")
			}
			return &PageResult{
				URL:     targetURL,
				Title:   "Example Page",
				Content: "Sample page text",
			}, nil
		},
	}

	cfg := config.BrowserConfig{
		Enabled: true,
		Timeout: 5 * time.Second,
	}

	runner := NewNavigateRunner(cfg, mock)
	if runner.Name() != "browser_navigate" {
		t.Errorf("Name() = %q, want 'browser_navigate'", runner.Name())
	}
	if runner.Backend() != "browser" {
		t.Errorf("Backend() = %q, want 'browser'", runner.Backend())
	}

	// Test successful navigation
	ctx := context.Background()
	out, err := runner.Run(ctx, `{"url": "https://example.com"}`)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out, "Example Page") {
		t.Errorf("expected output to contain title, got: %s", out)
	}
	if !strings.Contains(out, "Sample page text") {
		t.Errorf("expected output to contain content, got: %s", out)
	}

	// Test missing url
	_, err = runner.Run(ctx, `{}`)
	if err == nil {
		t.Errorf("expected error for missing url, got nil")
	}

	// Test navigation failure
	_, err = runner.Run(ctx, `https://fail.com`)
	if err == nil {
		t.Errorf("expected error for failed navigation, got nil")
	}
}

func TestBrowserActionRunner(t *testing.T) {
	mock := &mockDriver{
		actionFunc: func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
			if req.Action == "invalid" {
				return nil, errors.New("unsupported action")
			}
			return &ActionResult{
				Success: true,
				Output:  "clicked button",
			}, nil
		},
	}

	cfg := config.BrowserConfig{Enabled: true}
	runner := NewActionRunner(cfg, mock)
	if runner.Name() != "browser_action" {
		t.Errorf("Name() = %q, want 'browser_action'", runner.Name())
	}

	ctx := context.Background()
	out, err := runner.Run(ctx, `{"action": "click", "selector": "#submit-btn"}`)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out, "clicked button") {
		t.Errorf("unexpected output: %s", out)
	}

	// Test invalid action
	_, err = runner.Run(ctx, `{"action": "invalid"}`)
	if err == nil {
		t.Errorf("expected error for invalid action, got nil")
	}
}

func TestBrowserScreenshotRunner(t *testing.T) {
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "screenshot.png")

	mock := &mockDriver{
		screenshotFunc: func(ctx context.Context, targetURL, path string) (string, error) {
			if targetURL == "" {
				return "", errors.New("url required")
			}
			return destPath, nil
		},
	}

	cfg := config.BrowserConfig{Enabled: true}
	runner := NewScreenshotRunner(cfg, mock)
	if runner.Name() != "browser_screenshot" {
		t.Errorf("Name() = %q, want 'browser_screenshot'", runner.Name())
	}

	ctx := context.Background()
	out, err := runner.Run(ctx, `{"url": "https://example.com", "path": "`+destPath+`"}`)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out, destPath) {
		t.Errorf("expected output to contain path %q, got: %s", destPath, out)
	}

	// Test missing url
	_, err = runner.Run(ctx, `{"path": "foo.png"}`)
	if err == nil {
		t.Errorf("expected error for missing url, got nil")
	}
}

func TestBrowserCloseRunner(t *testing.T) {
	closed := false
	mock := &mockDriver{
		closeFunc: func() error {
			closed = true
			return nil
		},
	}

	cfg := config.BrowserConfig{Enabled: true}
	runner := NewCloseRunner(cfg, mock)
	if runner.Name() != "browser_close" {
		t.Errorf("Name() = %q, want 'browser_close'", runner.Name())
	}

	out, err := runner.Run(context.Background(), "")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !closed {
		t.Errorf("expected driver.Close() to be called")
	}
	if !strings.Contains(out, "closed") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestBrowserLocator(t *testing.T) {
	tempDir := t.TempDir()
	dummyBin := filepath.Join(tempDir, "my-browser")
	if err := os.WriteFile(dummyBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd, args, err := FindBrowserCommand(dummyBin)
	if err != nil {
		t.Fatalf("FindBrowserCommand(%q) error = %v", dummyBin, err)
	}
	if cmd != dummyBin {
		t.Errorf("cmd = %q, want %q", cmd, dummyBin)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}
