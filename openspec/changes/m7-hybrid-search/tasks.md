## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 750-950 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Core, Config, Migration, Vector) → PR 2 (Adapters) → PR 3 (RRF, Hybrid Search, Async, CLI, Tests) |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

---

## Task Breakdown

### PR 1: Core Embedder Port, Vector Math, Config & Migration 0006

- [x] 1.1 Config extensions: Add `EmbeddingsConfig` to `internal/config/config.go` with defaults (`enabled: false`, `provider: "ollama"`, `model: "nomic-embed-text"`, `dimensions: 768`, `batch_size: 100`). <!-- sdd-owner: implementation -->
- [x] 1.2 Migration 0006: Create `internal/memory/migrations/0006_embeddings.sql` with `embeddings` table, unique index on `(doc_type, doc_id)`, and `PRAGMA user_version = 6`. <!-- sdd-owner: implementation -->
- [x] 1.3 Embedder port: Create `internal/core/port_embedder.go` with `Embedder` interface (`Embed`, `EmbedBatch`, `Dimension`). <!-- sdd-owner: implementation -->
- [x] 1.4 Vector encoding & Cosine similarity: Implement `internal/memory/vector.go` with `EncodeVector`, `DecodeVector`, and `CosineSimilarity` (RED → GREEN). <!-- sdd-owner: implementation -->
- [x] 1.5 Unit tests for vector encoding, error rejection for non-multiple-of-4 BLOBs, and cosine math edge cases in `internal/memory/vector_test.go`. <!-- sdd-owner: implementation -->

### PR 2: Embedding Adapters (Ollama & OpenAI)

- [ ] 2.1 Ollama adapter: Implement `internal/adapters/llm/ollama_embed.go` targeting `/api/embed` with fallback support and timeout handling (RED → GREEN). <!-- sdd-owner: implementation -->
- [ ] 2.2 OpenAI adapter: Implement `internal/adapters/llm/openai_embed.go` targeting `/v1/embeddings` with sub-batch chunking up to `batch_size` (RED → GREEN). <!-- sdd-owner: implementation -->
- [ ] 2.3 Unit & mock tests in `internal/adapters/llm/embed_test.go` verifying single embedding, batch embedding, context cancellation, and error handling. <!-- sdd-owner: implementation -->

### PR 3: Reciprocal Rank Fusion (RRF), Hybrid Search, Async Indexer, CLI & Docs

- [ ] 3.1 RRF implementation: Implement `ReciprocalRankFusion` helper in `internal/memory/rrf.go` with $k = 60$ and deduplication by `(doc_type, doc_id)` (RED → GREEN). <!-- sdd-owner: implementation -->
- [ ] 3.2 Hybrid repository search: Extend `Search` in `internal/memory/sqlite.go` to compute embeddings, query vectors, calculate cosine similarities, and merge with FTS5 BM25 via RRF. <!-- sdd-owner: implementation -->
- [ ] 3.3 Graceful fallback: Verify and implement seamless fallback to 100% BM25 FTS5 when embeddings are disabled, offline, or fail. <!-- sdd-owner: implementation -->
- [ ] 3.4 Async embedding worker: Implement non-blocking background embedding generation for observations on `SaveObservations`. <!-- sdd-owner: implementation -->
- [ ] 3.5 Integration tests in `internal/memory/hybrid_test.go` & `cmd/agis/` with race detector (`go test -race ./...`) and goroutine leak validation (`goleak`). <!-- sdd-owner: implementation -->
- [ ] 3.6 Documentation updates: `docs/roadmap.md`, `docs/architecture.md`, `docs/configuration.md`, `README.md`. <!-- sdd-owner: implementation -->
