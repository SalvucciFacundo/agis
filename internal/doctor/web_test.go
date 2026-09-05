package doctor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestDoctor_CheckWebTools_Disabled(t *testing.T) {
	cfg := &config.Config{
		Tools: config.ToolsConfig{
			Enabled: true,
			Web: config.WebConfig{
				Enabled: false,
			},
		},
	}
	doc := New(cfg)
	res := doc.checkWebTools(context.Background())

	if res.Status != StatusPass {
		t.Errorf("expected StatusPass when disabled, got %s", res.Status)
	}
	if !strings.Contains(res.Message, "disabled") {
		t.Errorf("expected message to mention disabled, got %q", res.Message)
	}
	if res.Name != "web_tools" {
		t.Errorf("Name = %q, want %q", res.Name, "web_tools")
	}
}

func TestDoctor_CheckWebTools_DuckDuckGo(t *testing.T) {
	cfg := &config.Config{
		Tools: config.ToolsConfig{
			Enabled: true,
			Web: config.WebConfig{
				Enabled:         true,
				DefaultProvider: "duckduckgo",
				FetchTimeout:    15 * time.Second,
				MaxFetchBytes:   2097152,
			},
		},
	}
	doc := New(cfg)
	res := doc.checkWebTools(context.Background())

	if res.Status != StatusPass {
		t.Errorf("expected StatusPass for duckduckgo, got %s (message: %s)", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "duckduckgo") {
		t.Errorf("expected message to mention duckduckgo, got %q", res.Message)
	}
	if len(res.Details) == 0 {
		t.Errorf("expected details to be populated, got empty")
	}
}

func TestDoctor_CheckWebTools_Brave(t *testing.T) {
	// 1. Missing API Key -> StatusWarn
	cfgMissing := &config.Config{
		Tools: config.ToolsConfig{
			Enabled: true,
			Web: config.WebConfig{
				Enabled:         true,
				DefaultProvider: "brave",
			},
		},
	}
	docMissing := New(cfgMissing)
	resMissing := docMissing.checkWebTools(context.Background())

	if resMissing.Status != StatusWarn {
		t.Errorf("expected StatusWarn when Brave API key missing, got %s", resMissing.Status)
	}
	if !strings.Contains(resMissing.Message, "API key") {
		t.Errorf("expected warning about API key, got %q", resMissing.Message)
	}

	// 2. Configured API Key -> StatusPass
	cfgValid := &config.Config{
		Tools: config.ToolsConfig{
			Enabled: true,
			Web: config.WebConfig{
				Enabled:         true,
				DefaultProvider: "brave",
				Providers: config.WebProviders{
					BraveAPIKey: "BSA_mock_key_12345",
				},
			},
		},
	}
	docValid := New(cfgValid)
	resValid := docValid.checkWebTools(context.Background())

	if resValid.Status != StatusPass {
		t.Errorf("expected StatusPass when Brave API key present, got %s", resValid.Status)
	}
	if !strings.Contains(resValid.Message, "brave") {
		t.Errorf("expected message to mention brave, got %q", resValid.Message)
	}
}

func TestDoctor_CheckWebTools_Tavily(t *testing.T) {
	// 1. Missing API key -> StatusWarn
	cfgMissing := &config.Config{
		Tools: config.ToolsConfig{
			Enabled: true,
			Web: config.WebConfig{
				Enabled:         true,
				DefaultProvider: "tavily",
			},
		},
	}
	docMissing := New(cfgMissing)
	resMissing := docMissing.checkWebTools(context.Background())

	if resMissing.Status != StatusWarn {
		t.Errorf("expected StatusWarn when Tavily API key missing, got %s", resMissing.Status)
	}

	// 2. Configured API key -> StatusPass
	cfgValid := &config.Config{
		Tools: config.ToolsConfig{
			Enabled: true,
			Web: config.WebConfig{
				Enabled:         true,
				DefaultProvider: "tavily",
				Providers: config.WebProviders{
					TavilyAPIKey: "tvly-mock_key",
				},
			},
		},
	}
	docValid := New(cfgValid)
	resValid := docValid.checkWebTools(context.Background())

	if resValid.Status != StatusPass {
		t.Errorf("expected StatusPass when Tavily API key present, got %s", resValid.Status)
	}
}

func TestDoctor_CheckWebTools_SearXNG(t *testing.T) {
	cfg := &config.Config{
		Tools: config.ToolsConfig{
			Enabled: true,
			Web: config.WebConfig{
				Enabled:         true,
				DefaultProvider: "searxng",
				Providers: config.WebProviders{
					SearxngURL: "http://localhost:8080",
				},
			},
		},
	}
	doc := New(cfg)
	res := doc.checkWebTools(context.Background())

	if res.Status != StatusPass {
		t.Errorf("expected StatusPass for SearXNG, got %s", res.Status)
	}
}
