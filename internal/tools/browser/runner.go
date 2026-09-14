package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

// NewBrowserRunners returns the suite of tool runners for browser automation.
func NewBrowserRunners(cfg config.BrowserConfig, driver Driver) []core.ToolRunner {
	if driver == nil {
		d, err := NewCLIDriver(cfg)
		if err != nil {
			// Return inert runners that gracefully report unavailability when invoked
			return []core.ToolRunner{
				&unavailRunner{name: "browser_navigate", err: err},
				&unavailRunner{name: "browser_action", err: err},
				&unavailRunner{name: "browser_screenshot", err: err},
				&unavailRunner{name: "browser_close", err: err},
			}
		}
		driver = d
	}

	return []core.ToolRunner{
		NewNavigateRunner(cfg, driver),
		NewActionRunner(cfg, driver),
		NewScreenshotRunner(cfg, driver),
		NewCloseRunner(cfg, driver),
	}
}

// unavailRunner gracefully informs the LLM that browser is not installed or configured.
type unavailRunner struct {
	name string
	err  error
}

func (u *unavailRunner) Name() string        { return u.name }
func (u *unavailRunner) Backend() string     { return "browser" }
func (u *unavailRunner) Description() string { return "Headless browser automation tool." }
func (u *unavailRunner) Run(ctx context.Context, cmd string) (string, error) {
	return "", fmt.Errorf("browser tool unavailable: %w", u.err)
}

// NavigateRunner implements browser_navigate.
type NavigateRunner struct {
	cfg    config.BrowserConfig
	driver Driver
}

// NewNavigateRunner creates a new NavigateRunner.
func NewNavigateRunner(cfg config.BrowserConfig, driver Driver) *NavigateRunner {
	return &NavigateRunner{cfg: cfg, driver: driver}
}

func (r *NavigateRunner) Name() string    { return "browser_navigate" }
func (r *NavigateRunner) Backend() string { return "browser" }
func (r *NavigateRunner) Description() string {
	return "Navigate to a URL using a headless browser, rendering JavaScript and extracting page title and readable Markdown content."
}

type navigateArgs struct {
	URL string `json:"url"`
}

func (r *NavigateRunner) Run(ctx context.Context, command string) (string, error) {
	if r.driver == nil {
		return "", fmt.Errorf("browser driver is not available")
	}

	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return "", fmt.Errorf("url is required")
	}

	targetURL := trimmed
	if strings.HasPrefix(trimmed, "{") {
		var args navigateArgs
		if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
			return "", fmt.Errorf("invalid json arguments: %w", err)
		}
		targetURL = strings.TrimSpace(args.URL)
	}

	if targetURL == "" {
		return "", fmt.Errorf("url is required")
	}

	res, err := r.driver.Navigate(ctx, targetURL)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Title: %s\nURL: %s\n\n%s", res.Title, res.URL, res.Content), nil
}

// ActionRunner implements browser_action.
type ActionRunner struct {
	cfg    config.BrowserConfig
	driver Driver
}

// NewActionRunner creates a new ActionRunner.
func NewActionRunner(cfg config.BrowserConfig, driver Driver) *ActionRunner {
	return &ActionRunner{cfg: cfg, driver: driver}
}

func (r *ActionRunner) Name() string    { return "browser_action" }
func (r *ActionRunner) Backend() string { return "browser" }
func (r *ActionRunner) Description() string {
	return "Perform an action on the active or target web page: click, type, evaluate JavaScript, or scroll."
}

func (r *ActionRunner) Run(ctx context.Context, command string) (string, error) {
	if r.driver == nil {
		return "", fmt.Errorf("browser driver is not available")
	}

	var req ActionRequest
	trimmed := strings.TrimSpace(command)
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &req); err != nil {
			return "", fmt.Errorf("invalid json arguments: %w", err)
		}
	} else {
		req.Action = trimmed
	}

	if req.Action == "" {
		return "", fmt.Errorf("action is required (click, type, evaluate, scroll)")
	}

	res, err := r.driver.Action(ctx, req)
	if err != nil {
		return "", err
	}

	return res.Output, nil
}

// ScreenshotRunner implements browser_screenshot.
type ScreenshotRunner struct {
	cfg    config.BrowserConfig
	driver Driver
}

// NewScreenshotRunner creates a new ScreenshotRunner.
func NewScreenshotRunner(cfg config.BrowserConfig, driver Driver) *ScreenshotRunner {
	return &ScreenshotRunner{cfg: cfg, driver: driver}
}

func (r *ScreenshotRunner) Name() string    { return "browser_screenshot" }
func (r *ScreenshotRunner) Backend() string { return "browser" }
func (r *ScreenshotRunner) Description() string {
	return "Capture a full screenshot of a webpage in headless mode and save it to the specified local path."
}

type screenshotArgs struct {
	URL  string `json:"url"`
	Path string `json:"path"`
}

func (r *ScreenshotRunner) Run(ctx context.Context, command string) (string, error) {
	if r.driver == nil {
		return "", fmt.Errorf("browser driver is not available")
	}

	trimmed := strings.TrimSpace(command)
	var args screenshotArgs
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
			return "", fmt.Errorf("invalid json arguments: %w", err)
		}
	} else {
		args.URL = trimmed
	}

	if args.URL == "" {
		return "", fmt.Errorf("url is required for browser_screenshot")
	}
	if args.Path == "" {
		return "", fmt.Errorf("path is required for browser_screenshot")
	}

	path, err := r.driver.Screenshot(ctx, args.URL, args.Path)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Screenshot captured and saved to: %s", path), nil
}

// CloseRunner implements browser_close.
type CloseRunner struct {
	cfg    config.BrowserConfig
	driver Driver
}

// NewCloseRunner creates a new CloseRunner.
func NewCloseRunner(cfg config.BrowserConfig, driver Driver) *CloseRunner {
	return &CloseRunner{cfg: cfg, driver: driver}
}

func (r *CloseRunner) Name() string    { return "browser_close" }
func (r *CloseRunner) Backend() string { return "browser" }
func (r *CloseRunner) Description() string {
	return "Close the active browser session and release underlying resources."
}

func (r *CloseRunner) Run(ctx context.Context, command string) (string, error) {
	if r.driver == nil {
		return "browser session closed", nil
	}
	if err := r.driver.Close(); err != nil {
		return "", err
	}
	return "browser session closed successfully", nil
}
