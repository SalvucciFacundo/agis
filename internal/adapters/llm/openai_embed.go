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
	defaultOpenAIEmbedBaseURL = "https://api.openai.com/v1"
	defaultOpenAIEmbedModel   = "text-embedding-3-small"
	defaultOpenAIDimension    = 1536
	defaultOpenAIBatchSize    = 100
)

// OpenAIEmbedder is an adapter implementing core.Embedder for the OpenAI embeddings API.
type OpenAIEmbedder struct {
	baseURL    string
	apiKey     string
	model      string
	dimension  int
	batchSize  int
	httpClient *http.Client
}

var _ core.Embedder = (*OpenAIEmbedder)(nil)

// NewOpenAIEmbedder creates a new OpenAIEmbedder configured from cfg, apiKey, and optional options.
func NewOpenAIEmbedder(cfg config.EmbeddingsConfig, apiKey string, opts ...embedderOption) *OpenAIEmbedder {
	model := cfg.Model
	if model == "" {
		model = defaultOpenAIEmbedModel
	}
	dim := cfg.Dimensions
	if dim <= 0 {
		dim = defaultOpenAIDimension
	}
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultOpenAIBatchSize
	} else if batchSize > 2048 {
		batchSize = 2048
	}

	options := &embedderOptions{
		baseURL:    defaultOpenAIEmbedBaseURL,
		httpClient: &http.Client{},
	}
	for _, opt := range opts {
		opt(options)
	}

	return &OpenAIEmbedder{
		baseURL:    strings.TrimRight(options.baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		dimension:  dim,
		batchSize:  batchSize,
		httpClient: options.httpClient,
	}
}

// Dimension returns the embedding vector dimension.
func (o *OpenAIEmbedder) Dimension() int {
	return o.dimension
}

// Embed generates an embedding vector for a single text input.
func (o *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	vecs, err := o.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, errors.New("openai embeddings: empty embeddings response")
	}
	return vecs[0], nil
}

// EmbedBatch generates embeddings for a slice of text inputs.
// If texts is empty, it returns an empty slice immediately with no network calls.
// If len(texts) > batchSize, it breaks the input into chunks of batchSize and reassembles them in order.
func (o *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	chunkSize := o.batchSize
	if chunkSize <= 0 {
		chunkSize = defaultOpenAIBatchSize
	}

	allEmbeddings := make([][]float32, 0, len(texts))
	for i := 0; i < len(texts); i += chunkSize {
		end := i + chunkSize
		if end > len(texts) {
			end = len(texts)
		}
		chunk := texts[i:end]

		chunkVecs, err := o.doEmbedChunk(ctx, chunk)
		if err != nil {
			return nil, err
		}
		allEmbeddings = append(allEmbeddings, chunkVecs...)
	}

	return allEmbeddings, nil
}

type openAIEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbedItem struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

type openAIEmbedResponse struct {
	Object string            `json:"object"`
	Data   []openAIEmbedItem `json:"data"`
	Error  *apiError         `json:"error"`
}

func (o *OpenAIEmbedder) doEmbedChunk(ctx context.Context, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	reqBody, err := json.Marshal(openAIEmbedRequest{
		Model: o.model,
		Input: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("openai embeddings: encoding request: %w", err)
	}

	url := o.baseURL + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("openai embeddings: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai embeddings: sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, openAIStatusError(resp)
	}

	var out openAIEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("openai embeddings: decoding response: %w", err)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("openai embeddings: %w", out.Error)
	}

	result := make([][]float32, len(texts))
	for _, item := range out.Data {
		if item.Index >= 0 && item.Index < len(result) {
			result[item.Index] = item.Embedding
		}
	}
	// Fallback to sequential placement if indices were missing or not populated
	for idx, item := range out.Data {
		if idx < len(result) && result[idx] == nil {
			result[idx] = item.Embedding
		}
	}

	return result, nil
}

func openAIStatusError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var env struct {
		Error *apiError `json:"error"`
	}
	_ = json.Unmarshal(body, &env)

	errMsg := strings.TrimSpace(string(body))
	if env.Error != nil && env.Error.Message != "" {
		errMsg = env.Error.Message
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("openai embeddings: unauthorized (invalid api key): %s (status 401)", errMsg)
	case http.StatusForbidden:
		return fmt.Errorf("openai embeddings: forbidden: %s (status 403)", errMsg)
	case http.StatusTooManyRequests:
		return fmt.Errorf("openai embeddings: rate limit exceeded: %s (status 429)", errMsg)
	default:
		return fmt.Errorf("openai embeddings failed with status %d: %s", resp.StatusCode, errMsg)
	}
}
