package core

import "context"

// Embedder abstracts dense vector embedding generation for text inputs.
// Implementations exist for Ollama and OpenAI providers in internal/adapters/llm.
type Embedder interface {
	// Embed computes the dense float32 vector embedding for a single text input string.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch computes embeddings for a slice of text inputs in a single logical batch request.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the fixed vector dimension associated with the configured model
	// (e.g. 768 for nomic-embed-text, 1536 for text-embedding-3-small).
	Dimension() int
}
