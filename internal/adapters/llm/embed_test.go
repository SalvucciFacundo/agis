package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"go.uber.org/goleak"
)

func TestOllamaEmbedder_EmbedSingle(t *testing.T) {
	defer goleak.VerifyNone(t)

	mockVector := []float32{0.1, 0.2, 0.3}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		if req.Model != "nomic-embed-text" {
			t.Errorf("got model %q, want %q", req.Model, "nomic-embed-text")
		}
		if len(req.Input) != 1 || req.Input[0] != "hello world" {
			t.Errorf("got input %+v, want [\"hello world\"]", req.Input)
		}

		resp := struct {
			Model      string      `json:"model"`
			Embeddings [][]float32 `json:"embeddings"`
		}{
			Model:      "nomic-embed-text",
			Embeddings: [][]float32{mockVector},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(config.EmbeddingsConfig{
		Model:      "nomic-embed-text",
		Dimensions: 768,
	}, withBaseURL(server.URL))

	vec, err := embedder.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed() unexpected error: %v", err)
	}
	if len(vec) != len(mockVector) {
		t.Fatalf("got vector len %d, want %d", len(vec), len(mockVector))
	}
	for i := range mockVector {
		if vec[i] != mockVector[i] {
			t.Errorf("vec[%d] = %f, want %f", i, vec[i], mockVector[i])
		}
	}
	if embedder.Dimension() != 768 {
		t.Errorf("Dimension() = %d, want 768", embedder.Dimension())
	}
}

func TestOllamaEmbedder_EmbedBatch(t *testing.T) {
	defer goleak.VerifyNone(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Model      string      `json:"model"`
			Embeddings [][]float32 `json:"embeddings"`
		}{
			Model: "nomic-embed-text",
			Embeddings: [][]float32{
				{0.1, 0.2},
				{0.3, 0.4},
				{0.5, 0.6},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(config.EmbeddingsConfig{
		Model:      "nomic-embed-text",
		Dimensions: 768,
	}, withBaseURL(server.URL))

	vecs, err := embedder.EmbedBatch(context.Background(), []string{"alpha", "beta", "gamma"})
	if err != nil {
		t.Fatalf("EmbedBatch() unexpected error: %v", err)
	}
	if len(vecs) != 3 {
		t.Fatalf("got %d vectors, want 3", len(vecs))
	}
	if vecs[0][0] != 0.1 || vecs[1][0] != 0.3 || vecs[2][0] != 0.5 {
		t.Errorf("unexpected vectors values: %+v", vecs)
	}
}

func TestOllamaEmbedder_ExplicitModelAndDimensions(t *testing.T) {
	defer goleak.VerifyNone(t)

	embedder := NewOllamaEmbedder(config.EmbeddingsConfig{
		Model:      "all-minilm",
		Dimensions: 384,
	})
	if embedder.Dimension() != 384 {
		t.Errorf("Dimension() = %d, want 384", embedder.Dimension())
	}
	if embedder.model != "all-minilm" {
		t.Errorf("model = %q, want all-minilm", embedder.model)
	}
}

func TestOllamaEmbedder_EmptyBatch(t *testing.T) {
	defer goleak.VerifyNone(t)

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(config.EmbeddingsConfig{}, withBaseURL(server.URL))

	vecs, err := embedder.EmbedBatch(context.Background(), []string{})
	if err != nil {
		t.Fatalf("EmbedBatch() unexpected error: %v", err)
	}
	if len(vecs) != 0 {
		t.Errorf("got %d vectors, want 0", len(vecs))
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Errorf("expected 0 HTTP calls for empty batch, got %d", calls)
	}
}

func TestOllamaEmbedder_ContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(config.EmbeddingsConfig{}, withBaseURL(server.URL))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := embedder.Embed(ctx, "test")
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
}

func TestOllamaEmbedder_FallbackToLegacyEndpoint(t *testing.T) {
	defer goleak.VerifyNone(t)

	var embedCalls int32
	var embeddingsCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/embed":
			atomic.AddInt32(&embedCalls, 1)
			http.NotFound(w, r)
		case "/api/embeddings":
			atomic.AddInt32(&embeddingsCalls, 1)
			var req struct {
				Model  string `json:"model"`
				Prompt string `json:"prompt"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			resp := struct {
				Embedding []float32 `json:"embedding"`
			}{
				Embedding: []float32{0.9, 0.8},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(config.EmbeddingsConfig{
		Model:      "nomic-embed-text",
		Dimensions: 768,
	}, withBaseURL(server.URL))

	vecs, err := embedder.EmbedBatch(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("EmbedBatch fallback unexpected error: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("got %d vectors, want 2", len(vecs))
	}
	if atomic.LoadInt32(&embedCalls) != 1 {
		t.Errorf("expected 1 /api/embed call, got %d", embedCalls)
	}
	if atomic.LoadInt32(&embeddingsCalls) != 2 {
		t.Errorf("expected 2 /api/embeddings fallback calls, got %d", embeddingsCalls)
	}
}

func TestOllamaEmbedder_ServerErrorAndPayloadError(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("500 internal server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}))
		defer server.Close()

		embedder := NewOllamaEmbedder(config.EmbeddingsConfig{}, withBaseURL(server.URL))

		_, err := embedder.Embed(context.Background(), "test")
		if err == nil {
			t.Fatal("expected error on 500 response, got nil")
		}
	})

	t.Run("200 ok with error field in json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"error": "model not found"}`))
		}))
		defer server.Close()

		embedder := NewOllamaEmbedder(config.EmbeddingsConfig{}, withBaseURL(server.URL))

		_, err := embedder.Embed(context.Background(), "test")
		if err == nil {
			t.Fatal("expected error when json contains error field, got nil")
		}
		if !strings.Contains(err.Error(), "model not found") {
			t.Errorf("err = %q, want substring 'model not found'", err.Error())
		}
	})
}

func TestOpenAIEmbedder_EmbedSingle(t *testing.T) {
	defer goleak.VerifyNone(t)

	mockVector := []float32{0.42, 0.84}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer sk-testkey" {
			t.Errorf("unexpected Authorization header: %q", auth)
		}
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding body: %v", err)
		}
		if req.Model != "text-embedding-3-small" {
			t.Errorf("unexpected model: %s", req.Model)
		}

		resp := struct {
			Object string `json:"object"`
			Data   []struct {
				Object    string    `json:"object"`
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			} `json:"data"`
		}{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			}{
				{Object: "embedding", Index: 0, Embedding: mockVector},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewOpenAIEmbedder(config.EmbeddingsConfig{
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
		BatchSize:  100,
	}, "sk-testkey", withBaseURL(server.URL+"/v1"))

	vec, err := embedder.Embed(context.Background(), "semantic search")
	if err != nil {
		t.Fatalf("Embed() unexpected error: %v", err)
	}
	if len(vec) != len(mockVector) {
		t.Fatalf("got vector len %d, want %d", len(vec), len(mockVector))
	}
	if vec[0] != mockVector[0] || vec[1] != mockVector[1] {
		t.Errorf("vector values mismatch: %+v", vec)
	}
	if embedder.Dimension() != 1536 {
		t.Errorf("Dimension() = %d, want 1536", embedder.Dimension())
	}
}

func TestOpenAIEmbedder_EmbedBatch_ChunkingAndOrdering(t *testing.T) {
	defer goleak.VerifyNone(t)

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		type item struct {
			Object    string    `json:"object"`
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		}
		var data []item
		// Return items in reverse order to verify proper index reassembly
		for i := len(req.Input) - 1; i >= 0; i-- {
			data = append(data, item{
				Object:    "embedding",
				Index:     i,
				Embedding: []float32{float32(i), float32(len(req.Input))},
			})
		}

		resp := struct {
			Object string `json:"object"`
			Data   []item `json:"data"`
		}{
			Object: "list",
			Data:   data,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 250 inputs with batch size 100 => 3 requests (100, 100, 50)
	embedder := NewOpenAIEmbedder(config.EmbeddingsConfig{
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
		BatchSize:  100,
	}, "sk-key", withBaseURL(server.URL+"/v1"))

	inputs := make([]string, 250)
	for i := range inputs {
		inputs[i] = fmt.Sprintf("text item %d", i)
	}

	vecs, err := embedder.EmbedBatch(context.Background(), inputs)
	if err != nil {
		t.Fatalf("EmbedBatch() unexpected error: %v", err)
	}
	if len(vecs) != 250 {
		t.Fatalf("got %d vectors, want 250", len(vecs))
	}
	if atomic.LoadInt32(&requestCount) != 3 {
		t.Errorf("got %d requests, want 3 chunks", requestCount)
	}

	// Verify order was preserved across chunks
	for i := 0; i < 250; i++ {
		chunkIndex := i % 100
		if i >= 200 {
			chunkIndex = i - 200
		}
		if vecs[i][0] != float32(chunkIndex) {
			t.Errorf("vecs[%d][0] = %f, want %f", i, vecs[i][0], float32(chunkIndex))
		}
	}
}

func TestOpenAIEmbedder_EmptyBatch(t *testing.T) {
	defer goleak.VerifyNone(t)

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	}))
	defer server.Close()

	embedder := NewOpenAIEmbedder(config.EmbeddingsConfig{}, "key", withBaseURL(server.URL+"/v1"))

	vecs, err := embedder.EmbedBatch(context.Background(), []string{})
	if err != nil {
		t.Fatalf("EmbedBatch() unexpected error: %v", err)
	}
	if len(vecs) != 0 {
		t.Errorf("got %d vectors, want 0", len(vecs))
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Errorf("expected 0 calls, got %d", calls)
	}
}

func TestOpenAIEmbedder_AuthErrors(t *testing.T) {
	defer goleak.VerifyNone(t)

	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErrSub string
	}{
		{
			name:       "401 unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"error": {"message": "Invalid API key provided", "type": "invalid_request_error"}}`,
			wantErrSub: "unauthorized",
		},
		{
			name:       "403 forbidden",
			statusCode: http.StatusForbidden,
			body:       `{"error": {"message": "Country or region not supported", "type": "forbidden"}}`,
			wantErrSub: "forbidden",
		},
		{
			name:       "429 rate limit",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error": {"message": "Rate limit reached for requests", "type": "requests"}}`,
			wantErrSub: "rate limit",
		},
		{
			name:       "500 internal server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"error": {"message": "Internal error", "type": "server_error"}}`,
			wantErrSub: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			embedder := NewOpenAIEmbedder(config.EmbeddingsConfig{}, "bad-key", withBaseURL(server.URL+"/v1"))
			_, err := embedder.Embed(context.Background(), "test")
			if err == nil {
				t.Fatalf("expected error for status %d, got nil", tt.statusCode)
			}
			if !stringsContainsFold(err.Error(), tt.wantErrSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.wantErrSub)
			}
		})
	}
}

func TestNewEmbedder_Factory(t *testing.T) {
	tests := []struct {
		name        string
		cfg         config.EmbeddingsConfig
		apiKey      string
		wantType    string
		wantDim     int
		expectError bool
	}{
		{
			name: "ollama default",
			cfg: config.EmbeddingsConfig{
				Provider: "ollama",
			},
			wantType: "*llm.OllamaEmbedder",
			wantDim:  768,
		},
		{
			name: "ollama case-insensitive",
			cfg: config.EmbeddingsConfig{
				Provider: "OLLAMA",
			},
			wantType: "*llm.OllamaEmbedder",
			wantDim:  768,
		},
		{
			name: "openai default",
			cfg: config.EmbeddingsConfig{
				Provider: "openai",
			},
			apiKey:   "sk-test",
			wantType: "*llm.OpenAIEmbedder",
			wantDim:  1536,
		},
		{
			name: "openai case-insensitive",
			cfg: config.EmbeddingsConfig{
				Provider: "OpenAI",
			},
			apiKey:   "sk-test",
			wantType: "*llm.OpenAIEmbedder",
			wantDim:  1536,
		},
		{
			name: "empty provider defaults to ollama",
			cfg: config.EmbeddingsConfig{
				Provider: "",
			},
			wantType: "*llm.OllamaEmbedder",
			wantDim:  768,
		},
		{
			name: "unsupported provider",
			cfg: config.EmbeddingsConfig{
				Provider: "unknown-provider",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			embedder, err := NewEmbedder(tt.cfg, tt.apiKey)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error for provider %q, got nil", tt.cfg.Provider)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotType := fmt.Sprintf("%T", embedder); gotType != tt.wantType {
				t.Errorf("got type %q, want %q", gotType, tt.wantType)
			}
			if embedder.Dimension() != tt.wantDim {
				t.Errorf("Dimension() = %d, want %d", embedder.Dimension(), tt.wantDim)
			}
			var _ core.Embedder = embedder
		})
	}
}

func stringsContainsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
