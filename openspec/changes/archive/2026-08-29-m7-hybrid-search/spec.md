# M7 — Hybrid Search (Delta Spec)

Delta specification for `m7-hybrid-search`: Embedder Port & Adapters (Ollama & OpenAI), Binary Vector BLOB Storage, Cosine Similarity, Reciprocal Rank Fusion (RRF), Graceful Fallback, Migration 0006, and Embeddings Configuration.

---

## embeddings (NEW)

### AGIS-M7-EMB-001: Embedder Port Interface
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

### AGIS-M7-EMB-002: Ollama Embedding Adapter
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

### AGIS-M7-EMB-003: OpenAI Embedding Adapter
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

---

## repository-memory (MODIFIED)

### AGIS-M7-REPO-001: Binary Float32 Vector BLOB Encoding
The system MUST encode and decode `[]float32` vector arrays to and from raw SQLite `BLOB` byte slices using standard IEEE 754 32-bit floating point binary representation (`encoding/binary.LittleEndian` with `math.Float32bits` / `math.Float32frombits`):
1. The serialized binary BLOB length MUST equal exactly `len(vector) * 4` bytes.
2. An empty or nil vector MUST serialize to an empty byte slice (`[]byte{}`), and an empty BLOB MUST deserialize to an empty `[]float32{}`.
3. Deserializing a BLOB whose length is not a positive multiple of 4 MUST return an error and MUST NOT panic or produce corrupted vectors.

#### Scenario: Roundtrip serialization and deserialization of float32 vector
- GIVEN a vector `[]float32{1.0, -0.5, 3.14159, 0.0}`
- WHEN encoded to binary BLOB and subsequently decoded back
- THEN the decoded vector is identical in length and float values to the original vector

#### Scenario: Malformed BLOB length safely rejected
- GIVEN a corrupted database BLOB containing 7 bytes (not a multiple of 4)
- WHEN decoded into `[]float32`
- THEN a malformed vector error is returned and no panic occurs

---

### AGIS-M7-REPO-002: Cosine Similarity and In-Memory Vector Search
The system MUST compute pure Go Cosine Similarity between float32 query vector $\vec{u}$ and stored vector $\vec{v}$:
$$\text{CosineSimilarity}(\vec{u}, \vec{v}) = \frac{\sum_{i=1}^n u_i \cdot v_i}{\sqrt{\sum_{i=1}^n u_i^2} \cdot \sqrt{\sum_{i=1}^n v_i^2}}$$

Rules:
1. If vector dimensions differ ($len(\vec{u}) \neq len(\vec{v})$), the similarity MUST evaluate to `0.0` or be excluded from ranking.
2. If either vector has zero magnitude ($\|\vec{u}\| = 0$ or $\|\vec{v}\| = 0$), the calculated similarity MUST be `0.0`.
3. In-memory vector search MUST scan candidate vectors from the `embeddings` table for matching document types, calculate cosine similarity against the query vector, and sort candidates in descending order of similarity score.

#### Scenario: Cosine similarity between identical vectors is 1.0
- GIVEN two identical non-zero vectors $\vec{u} = \vec{v} = [0.6, 0.8]$
- WHEN cosine similarity is computed
- THEN the result is `1.0` within floating point epsilon tolerance ($10^{-5}$)

#### Scenario: Cosine similarity between orthogonal vectors is 0.0
- GIVEN orthogonal vectors $\vec{u} = [1.0, 0.0]$ and $\vec{v} = [0.0, 1.0]$
- WHEN cosine similarity is computed
- THEN the result is `0.0`

#### Scenario: Vector search ranks semantically closer items first
- GIVEN query vector $\vec{q} = [1.0, 0.0]$, candidate A with $\vec{v}_A = [0.9, 0.1]$ and candidate B with $\vec{v}_B = [0.2, 0.8]$
- WHEN vector search ranking is computed
- THEN candidate A is ranked before candidate B

---

### AGIS-M7-REPO-003: Reciprocal Rank Fusion (RRF) Hybrid Search
The `Repository.Search(ctx context.Context, query string, limit int)` method MUST combine lexical keyword search results from `memory_fts` (BM25 rank) and semantic vector search results from `embeddings` (cosine similarity rank) using Reciprocal Rank Fusion (RRF):
$$\text{RRF\_Score}(d) = \sum_{m \in \{\text{fts}, \text{vec}\}} \frac{1}{k + \text{rank}_m(d)}$$
where $k = 60$, and $\text{rank}_m(d) \ge 1$ is the 1-based index of document $d$ in result set $m$.

Rules:
1. If a document $d$ appears in only one ranking list $m$, its score contribution for the absent list is $0$.
2. Result documents MUST be deduplicated by `(doc_type, doc_id)` and ordered by `RRF_Score` descending.
3. In case of identical `RRF_Score` ties, ordering MUST be deterministically broken by `doc_id` ascending.
4. The final returned slice MUST be capped to `limit` entries.
(Previously: `Search` only executed BM25 full-text matching against `memory_fts`.)

#### Scenario: Document present in both FTS and vector results ranks top
- GIVEN document D1 at rank 1 in FTS and rank 1 in Vector search, and document D2 at rank 2 in FTS only
- WHEN hybrid search executes with RRF ($k=60$)
- THEN D1 score is $\frac{1}{61} + \frac{1}{61} \approx 0.0328$, D2 score is $\frac{1}{62} \approx 0.0161$, and D1 is returned as the top result

#### Scenario: Semantic synonym missing from FTS query is retrieved
- GIVEN observation "automobile repair" in database and user query "car maintenance"
- WHEN hybrid search is executed
- THEN FTS5 yields 0 matches but vector search identifies high semantic similarity, allowing the observation to be returned in search results

#### Scenario: Deterministic tie-breaking on equal RRF score
- GIVEN documents "doc-b" and "doc-a" with identical RRF scores
- WHEN hybrid search sorts the merged results
- THEN "doc-a" appears before "doc-b" due to deterministic alphabetical tie-breaking on `doc_id`

---

### AGIS-M7-REPO-004: Graceful Degradation and Fallback
The search and memory subsystems MUST remain fully functional even when embeddings are disabled or unavailable:
1. If `embeddings.enabled` is `false`, or if `Embedder` is `nil`, `Repository.Search` MUST bypass vector search and return standard BM25 FTS5 results.
2. If `Embedder.Embed` encounters a runtime failure (network timeout, connection refused, provider rate limit), `Repository.Search` MUST log a warning and return BM25 FTS5 results without surfacing an error to the caller.
3. Embedding generation for new or updated observations MUST run asynchronously in a background goroutine or worker queue, ensuring that embedding latency or outages never fail or stall core conversation/observation persistence transactions.
4. When an observation is deleted or updated, its corresponding vector row in `embeddings` MUST be invalidated and re-indexed.

#### Scenario: Search falls back to BM25 FTS5 when embedding provider fails
- GIVEN an enabled embeddings configuration but an unreachable Ollama instance
- WHEN `Repository.Search(ctx, "database schema", 10)` is invoked
- THEN search does not return an error, logs a warning, and returns BM25 FTS5 keyword matches

#### Scenario: Observation persistence succeeds even if embedding generation fails
- GIVEN an observation being saved via `SaveObservations` while the embedding service is down
- WHEN the save transaction completes
- THEN the observation and its FTS row are persisted in SQLite, and the background embedding task handles the failure gracefully without reverting the transaction

---

### AGIS-M7-REPO-005: Database Migration 0006_embeddings.sql
The system MUST include migration `internal/memory/migrations/0006_embeddings.sql`:
1. The migration MUST create the `embeddings` table:
   ```sql
   CREATE TABLE IF NOT EXISTS embeddings (
       id         TEXT PRIMARY KEY,
       doc_type   TEXT NOT NULL,
       doc_id     TEXT NOT NULL,
       dimension  INTEGER NOT NULL,
       vector     BLOB NOT NULL,
       created_at TEXT NOT NULL,
       updated_at TEXT NOT NULL,
       UNIQUE(doc_type, doc_id)
   );

   CREATE INDEX IF NOT EXISTS idx_embeddings_doc ON embeddings(doc_type, doc_id);
   ```
2. The migration MUST increment `PRAGMA user_version` from 5 to 6.
3. The migration MUST be additive, fully idempotent, and safe for existing databases.

#### Scenario: Database schema upgrades from v5 to v6
- GIVEN a database at `user_version` 5
- WHEN `Repository` is opened and migrations run
- THEN table `embeddings` and index `idx_embeddings_doc` are created and `user_version` becomes 6

#### Scenario: Migration is idempotent on v6 database
- GIVEN a database already at `user_version` 6
- WHEN migrations are executed
- THEN no DDL statements are re-executed and `user_version` remains 6

---

## config-loader (MODIFIED)

### AGIS-M7-CONF-001: Embeddings Configuration Schema
The configuration loader in `internal/config/config.go` MUST support the optional `embeddings` block:

```yaml
embeddings:
  enabled: false
  provider: "ollama"
  model: "nomic-embed-text"
  dimensions: 768
  batch_size: 100
```

Rules:
1. `embeddings.enabled` MUST default to `false` (opt-in feature).
2. `embeddings.provider` MUST default to `"ollama"`.
3. If `embeddings.model` is empty, it MUST resolve to `"nomic-embed-text"` when provider is `"ollama"` and `"text-embedding-3-small"` when provider is `"openai"`.
4. `embeddings.dimensions` MUST default to `0` (auto-detected when 0).
5. `embeddings.batch_size` MUST default to `100` with a safety maximum cap of `2048`.
6. Omission of the `embeddings` block in `config.yaml` MUST retain default values without error.
(Previously: `Config` struct did not contain an `embeddings` section.)

#### Scenario: Default configuration disables embeddings
- GIVEN an empty or standard `config.yaml` without an `embeddings` block
- WHEN `config.Load()` is called
- THEN `cfg.Embeddings.Enabled` is `false` and defaults are populated

#### Scenario: Provider-specific default model resolution
- GIVEN a `config.yaml` with `embeddings.enabled: true` and `embeddings.provider: "openai"` without specifying `model`
- WHEN `config.Load()` is called
- THEN `cfg.Embeddings.Model` is initialized to `"text-embedding-3-small"`

#### Scenario: Custom embeddings settings loaded accurately
- GIVEN a `config.yaml` specifying `embeddings: {enabled: true, provider: "ollama", model: "all-minilm", dimensions: 384, batch_size: 50}`
- WHEN `config.Load()` is called
- THEN all custom values are parsed accurately into `cfg.Embeddings`
