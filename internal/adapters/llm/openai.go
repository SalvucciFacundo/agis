package llm

import (
	"context"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

// OpenAI is the OpenAI adapter for the core.Provider port. It speaks the
// OpenAI /chat/completions protocol against the public OpenAI endpoint.
type OpenAI struct {
	client *Client
	model  string
}

var _ core.Provider = (*OpenAI)(nil)

// NewOpenAI returns an OpenAI-backed Provider configured from cfg.
func NewOpenAI(cfg config.LLMConfig) *OpenAI {
	baseURL := ResolveBaseURL(cfg.Provider, cfg.BaseURL)
	return &OpenAI{
		client: NewClientWithPool(baseURL, NewCredentialPool(cfg.APIKey, cfg.APIKeys)),
		model:  cfg.Model,
	}
}

// Chat implements core.Provider.
func (o *OpenAI) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	return o.client.Chat(ctx, ensureModel(req, o.model))
}

// Stream implements core.Provider.
func (o *OpenAI) Stream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	return o.client.Stream(ctx, ensureModel(req, o.model))
}

// Models returns the static M1 model list.
func (o *OpenAI) Models() []core.ModelInfo {
	return staticModels(o.model, providerOpenAI)
}
