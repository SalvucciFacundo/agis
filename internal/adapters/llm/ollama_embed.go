package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

const (
	defaultOllamaEmbedBaseURL = "http://localhost:11434"
	defaultOllamaEmbedModel   = "nomic-embed-text"
	defaultOllamaDimension    = 768
)

// OllamaEmbedder is an adapter implementing core.Embedder for the Ollama embeddings API.
type OllamaEmbedder struct {
	baseURL    string
	model      string
	dimension  int
	httpClient *http.Client
}

var _ core.Embedder = (*OllamaEmbedder)(nil)

// NewOllamaEmbedder creates a new OllamaEmbedder configured from cfg and optional options.
func NewOllamaEmbedder(cfg config.EmbeddingsConfig, opts ...embedderOption) *OllamaEmbedder {
	model := cfg.Model
	if model == "" {
		model = defaultOllamaEmbedModel
	}
	dim := cfg.Dimensions
	if dim <= 0 {
		dim = defaultOllamaDimension
	}
	options := &embedderOptions{
		baseURL:    defaultOllamaEmbedBaseURL,
		httpClient: &http.Client{},
	}
	for _, opt := range opts {
		opt(options)
	}

	return &OllamaEmbedder{
		baseURL:    strings.TrimRight(options.baseURL, "/"),
		model:      model,
		dimension:  dim,
		httpClient: options.httpClient,
	}
}

// Dimension returns the embedding vector dimension.
func (o *OllamaEmbedder) Dimension() int {
	return o.dimension
}

// Embed generates an embedding vector for a single text input.
func (o *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	vecs, err := o.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, errors.New("ollama embed: empty embeddings response")
	}
	return vecs[0], nil
}

// EmbedBatch generates embeddings for a slice of text inputs.
// If texts is empty, it returns an empty slice immediately with no network calls.
func (o *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Try the modern /api/embed endpoint first
	vecs, notFound, err := o.doEmbed(ctx, texts)
	if err == nil {
		return vecs, nil
	}
	if notFound {
		// Fallback to legacy /api/embeddings for older Ollama instances
		return o.doEmbeddingsFallback(ctx, texts)
	}
	return nil, err
}

type ollamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type ollamaEmbedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
	Error      string      `json:"error,omitempty"`
}

func (o *OllamaEmbedder) doEmbed(ctx context.Context, texts []string) ([][]float32, bool, error) {
	reqBody, err := json.Marshal(ollamaEmbedRequest{
		Model: o.model,
		Input: texts,
	})
	if err != nil {
		return nil, false, fmt.Errorf("ollama embed: encoding request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/embed", bytes.NewReader(reqBody))
	if err != nil {
		return nil, false, fmt.Errorf("ollama embed: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, false, fmt.Errorf("ollama embed: sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, true, errors.New("endpoint not found")
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, false, fmt.Errorf("ollama embed failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var out ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, false, fmt.Errorf("ollama embed: decoding response: %w", err)
	}
	if out.Error != "" {
		return nil, false, fmt.Errorf("ollama embed: %s", out.Error)
	}
	return out.Embeddings, false, nil
}

type ollamaLegacyRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaLegacyResponse struct {
	Embedding []float32 `json:"embedding"`
	Error     string    `json:"error,omitempty"`
}

func (o *OllamaEmbedder) doEmbeddingsFallback(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, 0, len(texts))
	for _, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		reqBody, err := json.Marshal(ollamaLegacyRequest{
			Model:  o.model,
			Prompt: text,
		})
		if err != nil {
			return nil, fmt.Errorf("ollama embeddings fallback: encoding request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/embeddings", bytes.NewReader(reqBody))
		if err != nil {
			return nil, fmt.Errorf("ollama embeddings fallback: building request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := o.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("ollama embeddings fallback: sending request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			return nil, fmt.Errorf("ollama embeddings fallback failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}

		var out ollamaLegacyResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("ollama embeddings fallback: decoding response: %w", err)
		}
		resp.Body.Close()

		if out.Error != "" {
			return nil, fmt.Errorf("ollama embeddings fallback: %s", out.Error)
		}
		results = append(results, out.Embedding)
	}
	return results, nil
}
