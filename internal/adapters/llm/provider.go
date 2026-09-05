package llm

import (
	"strings"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

// NewProvider returns the Provider adapter selected by cfg.Provider. The value
// "ollama" selects the local Ollama adapter; every other value — including
// "openai" and the empty string — selects the OpenAI adapter, which any
// OpenAI-compatible endpoint can stand in for.
func NewProvider(cfg config.LLMConfig) core.Provider {
	if strings.EqualFold(cfg.Provider, providerOllama) {
		return NewOllama(cfg)
	}
	return NewOpenAI(cfg)
}

// NewResilientProvider returns a core.Provider with fallback chains and credential pools configured.
// If cfg.Fallbacks contains entries, it constructs a FallbackProvider with the primary provider
// at index 0 and fallback providers in order. If no fallbacks are configured, it returns the primary provider.
func NewResilientProvider(cfg config.LLMConfig) core.Provider {
	primary := NewProvider(cfg)
	if len(cfg.Fallbacks) == 0 {
		return primary
	}

	fallbacks := make([]core.Provider, 0, len(cfg.Fallbacks))
	for _, fb := range cfg.Fallbacks {
		fbCfg := config.LLMConfig{
			Provider: fb.Provider,
			Model:    fb.Model,
			APIKey:   fb.APIKey,
			APIKeys:  fb.APIKeys,
			BaseURL:  fb.BaseURL,
		}
		fallbacks = append(fallbacks, NewProvider(fbCfg))
	}
	return NewFallbackProvider(primary, fallbacks...)
}

// NewProviderForTask returns a core.Provider for a specific auxiliary task (e.g. Memory, Vision, Audio).
// If taskProvider or taskModel are empty, they default to baseCfg.Provider and baseCfg.Model.
func NewProviderForTask(baseCfg config.LLMConfig, taskProvider, taskModel string) core.Provider {
	cfg := baseCfg
	if taskProvider != "" {
		cfg.Provider = taskProvider
	}
	if taskModel != "" {
		cfg.Model = taskModel
	}
	return NewResilientProvider(cfg)
}

// staticModels returns the M1 static model list: exactly one ModelInfo for the
// configured provider and model. Live enumeration is deferred to M4.
func staticModels(model, provider string) []core.ModelInfo {
	return []core.ModelInfo{
		{ID: model, Provider: provider},
	}
}

// ensureModel fills req.Model from model when the caller left it empty, which
// is what Brain.Step does. The configured model therefore always reaches the
// backend even when the caller only supplies messages.
func ensureModel(req core.ChatRequest, model string) core.ChatRequest {
	if req.Model == "" {
		req.Model = model
	}
	return req
}
