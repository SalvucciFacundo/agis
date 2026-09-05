package llm_test

import (
	"testing"

	"github.com/SalvucciFacundo/agis/internal/adapters/llm"
	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestProviderPresets_Resolution(t *testing.T) {
	tests := []struct {
		provider string
		wantURL  string
	}{
		{"openai", "https://api.openai.com/v1"},
		{"OpenAI", "https://api.openai.com/v1"},
		{"ollama", "http://localhost:11434/v1"},
		{"OLLAMA", "http://localhost:11434/v1"},
		{"openrouter", "https://openrouter.ai/api/v1"},
		{"gemini", "https://generativelanguage.googleapis.com/v1beta/openai"},
		{"deepseek", "https://api.deepseek.com/v1"},
		{"DeepSeek", "https://api.deepseek.com/v1"},
		{"groq", "https://api.groq.com/openai/v1"},
		{"mistral", "https://api.mistral.ai/v1"},
		{"xai", "https://api.x.ai/v1"},
		{"together", "https://api.together.xyz/v1"},
		{"cohere", "https://api.cohere.com/v2"},
		{"anthropic", "https://api.anthropic.com"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := llm.ResolveBaseURL(tt.provider, "")
			if got != tt.wantURL {
				t.Errorf("ResolveBaseURL(%q, \"\") = %q, want %q", tt.provider, got, tt.wantURL)
			}
		})
	}
}

func TestProviderPresets_ExplicitBaseURLOverridesDefault(t *testing.T) {
	customURL := "http://custom-proxy.internal/v1"
	got := llm.ResolveBaseURL("deepseek", customURL)
	if got != customURL {
		t.Errorf("ResolveBaseURL with explicit URL = %q, want %q", got, customURL)
	}
}

func TestNewProvider_AllPresetsInstantiate(t *testing.T) {
	providers := []string{
		"openai", "ollama", "openrouter", "gemini", "deepseek",
		"groq", "mistral", "xai", "together", "cohere", "anthropic",
	}

	for _, p := range providers {
		t.Run(p, func(t *testing.T) {
			cfg := config.LLMConfig{
				Provider: p,
				Model:    "test-model",
				APIKey:   "test-key",
			}
			prov := llm.NewProvider(cfg)
			if prov == nil {
				t.Fatalf("NewProvider(%q) returned nil", p)
			}
			models := prov.Models()
			if len(models) == 0 {
				t.Errorf("prov.Models() is empty for %q", p)
			}
		})
	}
}
