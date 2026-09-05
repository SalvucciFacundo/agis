package config_test

import (
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestMaskSecrets_LLMFallbacksAndAPIKeys(t *testing.T) {
	orig := &config.Config{
		LLM: config.LLMConfig{
			Provider: "openai",
			Model:    "gpt-4o",
			APIKey:   "sk-primary-key",
			APIKeys:  []string{"sk-primary-1", "sk-primary-2"},
			Fallbacks: []config.LLMFallbackConfig{
				{
					Provider: "openrouter",
					Model:    "anthropic/claude-3.5-sonnet",
					APIKey:   "sk-or-fallback",
					APIKeys:  []string{"sk-or-1", "sk-or-2"},
					BaseURL:  "https://openrouter.ai/api/v1",
				},
				{
					Provider: "ollama",
					Model:    "llama3.2",
					BaseURL:  "http://localhost:11434/v1",
				},
			},
		},
	}

	masked := config.MaskSecrets(orig)

	// Verify original is untouched
	if orig.LLM.APIKey != "sk-primary-key" {
		t.Errorf("Original LLM.APIKey mutated: %q", orig.LLM.APIKey)
	}
	if len(orig.LLM.APIKeys) != 2 || orig.LLM.APIKeys[0] != "sk-primary-1" {
		t.Errorf("Original LLM.APIKeys mutated: %+v", orig.LLM.APIKeys)
	}
	if len(orig.LLM.Fallbacks) != 2 || orig.LLM.Fallbacks[0].APIKey != "sk-or-fallback" {
		t.Errorf("Original LLM.Fallbacks mutated: %+v", orig.LLM.Fallbacks)
	}
	if len(orig.LLM.Fallbacks[0].APIKeys) != 2 || orig.LLM.Fallbacks[0].APIKeys[0] != "sk-or-1" {
		t.Errorf("Original LLM.Fallbacks[0].APIKeys mutated: %+v", orig.LLM.Fallbacks[0].APIKeys)
	}

	// Verify masked copy has "[MASKED]"
	if masked.LLM.APIKey != "[MASKED]" {
		t.Errorf("Masked LLM.APIKey = %q, want '[MASKED]'", masked.LLM.APIKey)
	}
	for i, k := range masked.LLM.APIKeys {
		if k != "[MASKED]" {
			t.Errorf("Masked LLM.APIKeys[%d] = %q, want '[MASKED]'", i, k)
		}
	}
	if masked.LLM.Fallbacks[0].APIKey != "[MASKED]" {
		t.Errorf("Masked Fallbacks[0].APIKey = %q, want '[MASKED]'", masked.LLM.Fallbacks[0].APIKey)
	}
	for i, k := range masked.LLM.Fallbacks[0].APIKeys {
		if k != "[MASKED]" {
			t.Errorf("Masked Fallbacks[0].APIKeys[%d] = %q, want '[MASKED]'", i, k)
		}
	}
	// Fallback without keys remains unchanged
	if masked.LLM.Fallbacks[1].APIKey != "" {
		t.Errorf("Masked Fallbacks[1].APIKey = %q, want ''", masked.LLM.Fallbacks[1].APIKey)
	}
}

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

func TestMaskSecrets_ServerAPIKey(t *testing.T) {
	orig := &config.Config{
		Server: config.ServerConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    8080,
			APIKey:  "sk-server-secret-key-12345",
		},
	}

	masked := config.MaskSecrets(orig)

	if orig.Server.APIKey != "sk-server-secret-key-12345" {
		t.Errorf("Original Server.APIKey mutated: %q", orig.Server.APIKey)
	}
	if masked.Server.APIKey != "[MASKED]" {
		t.Errorf("Masked Server.APIKey = %q, want '[MASKED]'", masked.Server.APIKey)
	}
	if masked.Server.Host != "127.0.0.1" || masked.Server.Port != 8080 {
		t.Errorf("Non-secret Server fields altered: %+v", masked.Server)
	}
}
