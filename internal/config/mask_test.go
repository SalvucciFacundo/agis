package config_test

import (
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestMaskSecrets_WebProviders(t *testing.T) {
	orig, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}

	orig.Tools.Web.Enabled = true
	orig.Tools.Web.Providers.Brave.APIKey = "bsa-secret-1"
	orig.Tools.Web.Providers.BraveAPIKey = "bsa-secret-2"
	orig.Tools.Web.Providers.Tavily.APIKey = "tvly-secret-1"
	orig.Tools.Web.Providers.TavilyAPIKey = "tvly-secret-2"
	orig.Tools.Web.Providers.Searxng.BaseURL = "http://searxng.local"
	orig.Tools.Web.DefaultProvider = "brave"

	masked := config.MaskSecrets(orig)

	// Verify original is untouched
	if orig.Tools.Web.Providers.Brave.APIKey != "bsa-secret-1" {
		t.Errorf("Original Brave.APIKey mutated: %q", orig.Tools.Web.Providers.Brave.APIKey)
	}
	if orig.Tools.Web.Providers.BraveAPIKey != "bsa-secret-2" {
		t.Errorf("Original BraveAPIKey mutated: %q", orig.Tools.Web.Providers.BraveAPIKey)
	}
	if orig.Tools.Web.Providers.Tavily.APIKey != "tvly-secret-1" {
		t.Errorf("Original Tavily.APIKey mutated: %q", orig.Tools.Web.Providers.Tavily.APIKey)
	}
	if orig.Tools.Web.Providers.TavilyAPIKey != "tvly-secret-2" {
		t.Errorf("Original TavilyAPIKey mutated: %q", orig.Tools.Web.Providers.TavilyAPIKey)
	}

	// Verify masked copy has "[MASKED]"
	if masked.Tools.Web.Providers.Brave.APIKey != "[MASKED]" {
		t.Errorf("Masked Brave.APIKey = %q, want '[MASKED]'", masked.Tools.Web.Providers.Brave.APIKey)
	}
	if masked.Tools.Web.Providers.BraveAPIKey != "[MASKED]" {
		t.Errorf("Masked BraveAPIKey = %q, want '[MASKED]'", masked.Tools.Web.Providers.BraveAPIKey)
	}
	if masked.Tools.Web.Providers.Tavily.APIKey != "[MASKED]" {
		t.Errorf("Masked Tavily.APIKey = %q, want '[MASKED]'", masked.Tools.Web.Providers.Tavily.APIKey)
	}
	if masked.Tools.Web.Providers.TavilyAPIKey != "[MASKED]" {
		t.Errorf("Masked TavilyAPIKey = %q, want '[MASKED]'", masked.Tools.Web.Providers.TavilyAPIKey)
	}

	// Verify non-secret fields remain preserved
	if masked.Tools.Web.Providers.Searxng.BaseURL != "http://searxng.local" {
		t.Errorf("Masked Searxng.BaseURL = %q, want 'http://searxng.local'", masked.Tools.Web.Providers.Searxng.BaseURL)
	}
	if masked.Tools.Web.DefaultProvider != "brave" {
		t.Errorf("Masked DefaultProvider = %q, want 'brave'", masked.Tools.Web.DefaultProvider)
	}
}
