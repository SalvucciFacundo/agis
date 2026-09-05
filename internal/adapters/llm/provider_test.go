package llm

import (
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestNewProviderForTask(t *testing.T) {
	baseCfg := config.LLMConfig{
		Provider: "openai",
		Model:    "gpt-4o",
		APIKey:   "sk-test-base",
		APIKeys:  []string{"sk-test-backup"},
	}

	tests := []struct {
		name         string
		taskProvider string
		taskModel    string
		wantProvider string
		wantModel    string
	}{
		{
			name:         "empty overrides inherit base config",
			taskProvider: "",
			taskModel:    "",
			wantProvider: "openai",
			wantModel:    "gpt-4o",
		},
		{
			name:         "override model only",
			taskProvider: "",
			taskModel:    "gpt-4o-mini",
			wantProvider: "openai",
			wantModel:    "gpt-4o-mini",
		},
		{
			name:         "override provider only",
			taskProvider: "ollama",
			taskModel:    "",
			wantProvider: "ollama",
			wantModel:    "gpt-4o",
		},
		{
			name:         "override both provider and model",
			taskProvider: "ollama",
			taskModel:    "llama3.2:1b",
			wantProvider: "ollama",
			wantModel:    "llama3.2:1b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProviderForTask(baseCfg, tt.taskProvider, tt.taskModel)
			if p == nil {
				t.Fatal("NewProviderForTask returned nil")
			}

			models := p.Models()
			if len(models) == 0 {
				t.Fatal("Models() returned empty list")
			}

			if models[0].Provider != tt.wantProvider {
				t.Errorf("Provider = %q, want %q", models[0].Provider, tt.wantProvider)
			}
			if models[0].ID != tt.wantModel {
				t.Errorf("Model ID = %q, want %q", models[0].ID, tt.wantModel)
			}
		})
	}
}

func TestNewResilientProvider(t *testing.T) {
	t.Run("single provider without fallbacks", func(t *testing.T) {
		cfg := config.LLMConfig{
			Provider: "openai",
			Model:    "gpt-4o",
			APIKey:   "sk-test",
		}
		p := NewResilientProvider(cfg)
		if p == nil {
			t.Fatal("NewResilientProvider returned nil")
		}

		models := p.Models()
		if len(models) != 1 {
			t.Fatalf("expected 1 model, got %d", len(models))
		}
		if models[0].ID != "gpt-4o" || models[0].Provider != "openai" {
			t.Errorf("got model %+v, want gpt-4o / openai", models[0])
		}
	})

	t.Run("composite provider with fallbacks", func(t *testing.T) {
		cfg := config.LLMConfig{
			Provider: "openai",
			Model:    "gpt-4o",
			APIKey:   "sk-test",
			Fallbacks: []config.LLMFallbackConfig{
				{
					Provider: "ollama",
					Model:    "llama3.2",
				},
				{
					Provider: "openai",
					Model:    "gpt-4o-mini",
					APIKey:   "sk-mini",
				},
			},
		}

		p := NewResilientProvider(cfg)
		if p == nil {
			t.Fatal("NewResilientProvider returned nil")
		}

		fallback, ok := p.(*FallbackProvider)
		if !ok {
			t.Fatalf("expected *FallbackProvider, got %T", p)
		}

		if len(fallback.providers) != 3 {
			t.Fatalf("expected 3 chain providers, got %d", len(fallback.providers))
		}

		models := p.Models()
		if len(models) != 3 {
			t.Fatalf("expected 3 models in combined list, got %d", len(models))
		}
	})
}
