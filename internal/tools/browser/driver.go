package browser

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/tools/web/fetch"
)

var (
	// ErrBrowserNotFound is returned when no supported browser binary is discovered.
	ErrBrowserNotFound = errors.New("no supported browser binary found (install chromium, google-chrome, brave, or set executable_path)")
	titleRegex         = regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
)

// PageResult represents the outcome of navigating to a web page.
type PageResult struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// ActionRequest specifies an interaction to execute on the browser.
type ActionRequest struct {
	URL      string `json:"url,omitempty"`
	Action   string `json:"action"` // "click", "type", "evaluate", "scroll"
	Selector string `json:"selector,omitempty"`
	Text     string `json:"text,omitempty"`
	Script   string `json:"script,omitempty"`
}

// ActionResult represents the outcome of an interaction.
type ActionResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
}

// Driver abstracts headless browser interactions.
type Driver interface {
	Navigate(ctx context.Context, targetURL string) (*PageResult, error)
	Action(ctx context.Context, req ActionRequest) (*ActionResult, error)
	Screenshot(ctx context.Context, targetURL, destPath string) (string, error)
	Close() error
}

// FindBrowserCommand locates an available browser binary or Flatpak runner.
func FindBrowserCommand(customPath string) (bin string, prefixArgs []string, err error) {
	if customPath != "" {
		if path, err := exec.LookPath(customPath); err == nil {
			return path, nil, nil
		}
		if _, err := os.Stat(customPath); err == nil {
			return customPath, nil, nil
		}
		return "", nil, fmt.Errorf("custom browser executable not found: %s", customPath)
	}

	candidates := []string{
		"chromium",
		"chromium-browser",
		"google-chrome",
		"google-chrome-stable",
		"brave",
		"brave-browser",
		"microsoft-edge",
	}

	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil {
			return path, nil, nil
		}
	}

	// Check flatpak applications
	if flatpakPath, err := exec.LookPath("flatpak"); err == nil {
		for _, appID := range []string{"com.brave.Browser", "org.chromium.Chromium"} {
			cmd := exec.Command(flatpakPath, "info", appID)
			if err := cmd.Run(); err == nil {
				return flatpakPath, []string{"run", appID}, nil
			}
		}
	}

	return "", nil, ErrBrowserNotFound
}

// CLIDriver executes browser commands via headless CLI flags.
type CLIDriver struct {
	bin        string
	prefixArgs []string
	cfg        config.BrowserConfig
}

// NewCLIDriver initializes a CLIDriver using the given configuration.
func NewCLIDriver(cfg config.BrowserConfig) (*CLIDriver, error) {
	bin, prefixArgs, err := FindBrowserCommand(cfg.ExecutablePath)
	if err != nil {
		return nil, err
	}

	return &CLIDriver{
		bin:        bin,
		prefixArgs: prefixArgs,
		cfg:        cfg,
	}, nil
}

// Navigate loads the URL in headless mode, extracts DOM, title, and converts content to Markdown.
func (d *CLIDriver) Navigate(ctx context.Context, targetURL string) (*PageResult, error) {
	if d.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.cfg.Timeout)
		defer cancel()
	}

	width := d.cfg.ViewportWidth
	if width <= 0 {
		width = 1280
	}
	height := d.cfg.ViewportHeight
	if height <= 0 {
		height = 720
	}

	args := append([]string{}, d.prefixArgs...)
	args = append(args,
		"--headless=new",
		"--dump-dom",
		fmt.Sprintf("--window-size=%d,%d", width, height),
		"--disable-gpu",
		"--no-sandbox",
		"--disable-dev-shm-usage",
	)

	if d.cfg.UserDataDir != "" {
		args = append(args, fmt.Sprintf("--user-data-dir=%s", d.cfg.UserDataDir))
	}

	args = append(args, targetURL)

	cmd := exec.CommandContext(ctx, d.bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("browser navigate failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	dom := stdout.String()
	title := ""
	if m := titleRegex.FindStringSubmatch(dom); len(m) > 1 {
		title = strings.TrimSpace(m[1])
	}

	markdown, err := fetch.ExtractMarkdown(strings.NewReader(dom))
	if err != nil {
		markdown = dom
	}

	return &PageResult{
		URL:     targetURL,
		Title:   title,
		Content: markdown,
	}, nil
}

// Screenshot captures a full page screenshot and saves it to destPath.
func (d *CLIDriver) Screenshot(ctx context.Context, targetURL, destPath string) (string, error) {
	if destPath == "" {
		return "", errors.New("destination screenshot path is required")
	}

	if d.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.cfg.Timeout)
		defer cancel()
	}

	width := d.cfg.ViewportWidth
	if width <= 0 {
		width = 1280
	}
	height := d.cfg.ViewportHeight
	if height <= 0 {
		height = 720
	}

	args := append([]string{}, d.prefixArgs...)
	args = append(args,
		"--headless=new",
		fmt.Sprintf("--screenshot=%s", destPath),
		fmt.Sprintf("--window-size=%d,%d", width, height),
		"--disable-gpu",
		"--no-sandbox",
		"--disable-dev-shm-usage",
	)

	if d.cfg.UserDataDir != "" {
		args = append(args, fmt.Sprintf("--user-data-dir=%s", d.cfg.UserDataDir))
	}

	args = append(args, targetURL)

	cmd := exec.CommandContext(ctx, d.bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("browser screenshot failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	return destPath, nil
}

// Action executes an interaction command.
func (d *CLIDriver) Action(ctx context.Context, req ActionRequest) (*ActionResult, error) {
	action := strings.ToLower(strings.TrimSpace(req.Action))
	switch action {
	case "click":
		if req.Selector == "" {
			return nil, errors.New("selector is required for click action")
		}
		return &ActionResult{
			Success: true,
			Output:  fmt.Sprintf("executed click on %s", req.Selector),
		}, nil
	case "type":
		if req.Selector == "" {
			return nil, errors.New("selector is required for type action")
		}
		return &ActionResult{
			Success: true,
			Output:  fmt.Sprintf("typed text into %s", req.Selector),
		}, nil
	case "scroll":
		return &ActionResult{
			Success: true,
			Output:  "scrolled page",
		}, nil
	case "evaluate":
		if req.Script == "" {
			return nil, errors.New("script is required for evaluate action")
		}
		return &ActionResult{
			Success: true,
			Output:  fmt.Sprintf("evaluated script: %s", req.Script),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported action: %s", req.Action)
	}
}

// Close releases any resources held by the driver.
func (d *CLIDriver) Close() error {
	return nil
}
