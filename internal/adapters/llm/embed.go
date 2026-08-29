package llm

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

type embedderOption func(*embedderOptions)

type embedderOptions struct {
	baseURL    string
	httpClient *http.Client
}

func withBaseURL(url string) embedderOption {
	return func(o *embedderOptions) {
		o.baseURL = url
	}
}

func withHTTPClient(client *http.Client) embedderOption {
	return func(o *embedderOptions) {
		o.httpClient = client
	}
}

// NewEmbedder constructs a core.Embedder matching cfg.Provider.
func NewEmbedder(cfg config.EmbeddingsConfig, apiKey string) (core.Embedder, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	case "", providerOllama:
		return NewOllamaEmbedder(cfg), nil
	case providerOpenAI:
		return NewOpenAIEmbedder(cfg, apiKey), nil
	default:
		return nil, fmt.Errorf("unsupported embeddings provider: %s", cfg.Provider)
	}
}
