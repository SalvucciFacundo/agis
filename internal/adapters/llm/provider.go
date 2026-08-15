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
