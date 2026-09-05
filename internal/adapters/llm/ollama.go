package llm

import (
	"context"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

// Ollama is the Ollama adapter for the core.Provider port. Ollama serves an
// OpenAI-compatible protocol locally, so this adapter is the shared client
// pointed at http://localhost:11434/v1.
type Ollama struct {
	client *Client
	model  string
}

var _ core.Provider = (*Ollama)(nil)

// NewOllama returns an Ollama-backed Provider configured from cfg.
func NewOllama(cfg config.LLMConfig) *Ollama {
	baseURL := ollamaBaseURL
	if cfg.BaseURL != "" {
		baseURL = cfg.BaseURL
	}
	return &Ollama{
		client: NewClientWithPool(baseURL, NewCredentialPool(cfg.APIKey, cfg.APIKeys)),
		model:  cfg.Model,
	}
}

// Chat implements core.Provider.
func (o *Ollama) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	return o.client.Chat(ctx, ensureModel(req, o.model))
}

// Stream implements core.Provider.
func (o *Ollama) Stream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	return o.client.Stream(ctx, ensureModel(req, o.model))
}

// Models returns the static M1 model list.
func (o *Ollama) Models() []core.ModelInfo {
	return staticModels(o.model, providerOllama)
}
