# Embeddings Spec

## Purpose

Define the dense vector embedding subsystem in AGIS, providing an abstract `core.Embedder` port with Ollama and OpenAI adapters for computing vector representations of text strings and batches.

## Requirements

### Requirement AGIS-M7-EMB-001: Embedder Port Interface
The core domain MUST define an `Embedder` interface in `internal/core/port_embedder.go` to abstract dense vector embedding generation:
- `Embed(ctx context.Context, text string) ([]float32, error)`: Computes the dense float32 vector embedding for a single text input string.
- `EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)`: Computes embeddings for a slice of text inputs in a single logical batch request.
- `Dimension() int`: Returns the fixed vector dimension associated with the configured model (e.g. 768 for `nomic-embed-text`, 1536 for `text-embedding-3-small`), or 0 if dynamically detected.

Rules:
1. When an empty slice is passed to `EmbedBatch`, the method MUST return an empty slice of vectors (`[][]float32{}`) with `nil` error without issuing network requests.
2. Methods MUST respect `ctx` cancellation and timeouts, returning context errors immediately upon deadline expiry.
3. If the upstream provider returns an HTTP error status or malformed payload, methods MUST return a wrapped, descriptive error without panicking.

#### Scenario: Embed single text string
- GIVEN a valid and configured `Embedder` instance
- WHEN `Embed(ctx, "architecture patterns in Go")` is called
- THEN a non-empty `[]float32` vector of matching dimension is returned with `nil` error

#### Scenario: Embed empty batch returns empty slice
- GIVEN an active `Embedder` instance
- WHEN `EmbedBatch(ctx, []string{})` is called
- THEN an empty slice `[][]float32{}` is returned immediately with `nil` error and no network calls are dispatched

#### Scenario: Context cancellation aborts embedding request
- GIVEN an embedding request with an already-canceled context
- WHEN `Embed(ctx, "test query")` is invoked
- THEN `ctx.Err()` is returned immediately and no remote processing occurs

---

### Requirement AGIS-M7-EMB-002: Ollama Embedding Adapter
The system MUST provide an Ollama embedding adapter in `internal/adapters/llm/` implementing `core.Embedder`:
1. The adapter MUST issue HTTP POST requests to the Ollama embedding API endpoint (`/api/embed` with fallback support for `/api/embeddings`).
2. The default model MUST be `"nomic-embed-text"` when not explicitly overridden in configuration.
3. The adapter MUST parse JSON responses containing single or batch vector arrays and convert them into `[]float32` / `[][]float32`.
4. Network timeouts and connection errors (such as Ollama daemon offline) MUST be returned as descriptive errors.

#### Scenario: Generate single embedding via Ollama
- GIVEN a running Ollama daemon and adapter configured with model `"nomic-embed-text"`
- WHEN `Embed(ctx, "hello world")` is executed
- THEN an HTTP POST request is sent to `/api/embed` and a 768-dimensional float32 vector is returned

#### Scenario: Generate batch embeddings via Ollama
- GIVEN an Ollama embedding adapter
- WHEN `EmbedBatch(ctx, []string{"alpha", "beta", "gamma"})` is called with 3 strings
- THEN the adapter transmits the batch payload and returns 3 float32 vectors corresponding to each input in order

#### Scenario: Ollama endpoint unreachable returns error
- GIVEN an Ollama daemon that is stopped or unreachable at the configured host/port
- WHEN `Embed(ctx, "query")` is called
- THEN the adapter returns a connection error wrapping the network failure without crashing

---

### Requirement AGIS-M7-EMB-003: OpenAI Embedding Adapter
The system MUST provide an OpenAI embedding adapter in `internal/adapters/llm/` implementing `core.Embedder`:
1. The adapter MUST issue HTTP POST requests to `https://api.openai.com/v1/embeddings` (or custom configured base URL) with the `Authorization: Bearer <api_key>` header.
2. The default model MUST be `"text-embedding-3-small"` (1536 dimensions) when not specified in configuration.
3. If a batch request exceeds the configured `batch_size` (default 100), the adapter MUST chunk the input slice into consecutive sub-batches of at most `batch_size`, request embeddings for each chunk, and reconstruct the final result slice preserving input order.
4. HTTP response status codes 401/403 (invalid API key) or 429 (rate limit exceeded) MUST be surfaced as explicit typed or wrapped errors.

#### Scenario: Generate embeddings via OpenAI endpoint
- GIVEN a valid OpenAI API key and model `"text-embedding-3-small"`
- WHEN `Embed(ctx, "semantic memory indexing")` is called
- THEN a 1536-dimensional float32 vector is returned

#### Scenario: Batch chunking for large inputs
- GIVEN an OpenAI embedding adapter configured with `batch_size: 100`
- WHEN `EmbedBatch(ctx, texts)` is called with 250 text items
- THEN the adapter executes 3 sub-requests (100, 100, and 50 items) and returns all 250 vectors in their original order

#### Scenario: OpenAI API returns authentication failure
- GIVEN an invalid or revoked OpenAI API key
- WHEN `Embed(ctx, "test")` is executed
- THEN the adapter returns an unauthorized error and no vector is produced
