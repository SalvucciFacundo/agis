# Apply Progress — M7: Hybrid Search

## Execution Summary

- **Change:** `m7-hybrid-search`
- **Delivery Strategy:** `auto-chain` (Stacked PRs to main)
- **Current Slice:** PR 2 — Embedding Adapters (Ollama & OpenAI)
- **Status:** Complete (Tasks 2.1–2.3 completed; PR 1 + PR 2 merged)

---

## Workload & PR Slices Breakdown

```
[PR 1: Core Port, Vector Math, Config, Migration 0006] (Completed)
    ↓
[PR 2: Embedding Adapters (Ollama & OpenAI)] 📍 (Current - Completed)
    ↓
[PR 3: Reciprocal Rank Fusion, Hybrid Search, Async Indexer, CLI & Docs] (Next)
```

- **Authored Lines (PR 1):** ~460 lines
- **Authored Lines (PR 2):** ~1030 lines (including unit/mock tests with `httptest.Server` and `goleak`)
- **400-Line Budget Status:** Structured into independent reviewable PR slices.

---

## Tasks Completed (PR 1 & PR 2)

| Task | Description | Status |
|---|---|---|
| **1.1** | Config extensions: `EmbeddingsConfig` in `internal/config/config.go` with safe defaults (`enabled: false`, `provider: "ollama"`, `model: "nomic-embed-text"`, `dimensions: 768`, `batch_size: 100`). | ✅ Completed |
| **1.2** | Migration 0006: `internal/memory/migrations/0006_embeddings.sql` with `embeddings` table, unique index on `(doc_type, doc_id)`, `PRAGMA user_version = 6`. | ✅ Completed |
| **1.3** | Embedder port: `internal/core/port_embedder.go` with `Embedder` interface (`Embed`, `EmbedBatch`, `Dimension`). | ✅ Completed |
| **1.4** | Vector encoding & Cosine similarity: `internal/memory/vector.go` with `EncodeVector`, `DecodeVector`, `CosineSimilarity`. | ✅ Completed |
| **1.5** | Unit tests for vector encoding, error rejection on non-multiple-of-4 BLOBs, cosine math edge cases, and migration idempotency. | ✅ Completed |
| **2.1** | Ollama adapter: `internal/adapters/llm/ollama_embed.go` implementing `core.Embedder` targeting `/api/embed` with fallback to `/api/embeddings`, context cancellation, and zero-alloc empty batch handling. | ✅ Completed |
| **2.2** | OpenAI adapter: `internal/adapters/llm/openai_embed.go` implementing `core.Embedder` targeting `/v1/embeddings` with sub-batch chunking up to `batch_size`, index reassembly, and typed HTTP error handling. | ✅ Completed |
| **2.3** | Unit & mock tests in `internal/adapters/llm/embed_test.go` verifying single embedding, batch embedding, sub-batch chunking, context cancellation, empty batches, legacy fallback, and error envelopes with `goleak`. | ✅ Completed |

---

## TDD Cycle Evidence

| Unit | RED Test | RED Output | GREEN Implementation | GREEN Verification |
|---|---|---|---|---|
| **Config** | `TestLoad_EmbeddingsDefaultsAndExplicit` | `cfg.Embeddings undefined` | `EmbeddingsConfig` struct + defaults & resolution logic in `config.go` | `go test -race ./internal/config` (PASS) |
| **Migration** | `TestMigration_V5ToV6`, updated `TestMigrations` to v6 | `table "embeddings" missing`, `user_version = 5, want 6` | `0006_embeddings.sql` DDL + unique index | `go test -race ./internal/memory -run 'TestMigration.*'` (PASS) |
| **Core Port** | Port declaration | Interface contract defined | `internal/core/port_embedder.go` | `go test ./internal/core/...` (PASS) |
| **Vector Math** | `TestEncodeDecodeVector_Roundtrip`, `TestDecodeVector_MalformedLength`, `TestCosineSimilarity` | `undefined: EncodeVector`, `undefined: DecodeVector`, `undefined: CosineSimilarity` | `EncodeVector`, `DecodeVector`, `CosineSimilarity` in `vector.go` | `go test -race ./internal/memory -run 'Test(EncodeDecodeVector\|DecodeVector\|CosineSimilarity).*'` (PASS) |
| **Ollama Adapter** | `TestOllamaEmbedder_EmbedSingle`, `TestOllamaEmbedder_EmbedBatch`, `TestOllamaEmbedder_FallbackToLegacyEndpoint` | `undefined: NewOllamaEmbedder` | `OllamaEmbedder` in `ollama_embed.go` supporting `/api/embed` and `/api/embeddings` fallback | `go test -race ./internal/adapters/llm -run 'TestOllamaEmbedder.*'` (PASS) |
| **OpenAI Adapter** | `TestOpenAIEmbedder_EmbedSingle`, `TestOpenAIEmbedder_EmbedBatch_ChunkingAndOrdering`, `TestOpenAIEmbedder_AuthErrors` | `undefined: NewOpenAIEmbedder` | `OpenAIEmbedder` in `openai_embed.go` with sub-batch chunking and 401/403/429 status mapping | `go test -race ./internal/adapters/llm -run 'TestOpenAIEmbedder.*'` (PASS) |
| **Embedder Factory** | `TestNewEmbedder_Factory` | `undefined: NewEmbedder` | `NewEmbedder` factory in `embed.go` | `go test -race ./internal/adapters/llm -run 'TestNewEmbedder.*'` (PASS) |

---

## Files Changed / Created

- **Created (PR 2):**
  - `internal/adapters/llm/embed.go`: Factory function `NewEmbedder` and option helpers.
  - `internal/adapters/llm/ollama_embed.go`: Ollama `core.Embedder` implementation (`/api/embed` + `/api/embeddings` fallback).
  - `internal/adapters/llm/openai_embed.go`: OpenAI `core.Embedder` implementation (`/v1/embeddings` + chunking & error mapping).
  - `internal/adapters/llm/embed_test.go`: Comprehensive table-driven tests for single/batch embeddings, sub-batch chunking, legacy fallback, cancellation, and goroutine leak validation via `goleak`.
- **Created (PR 1):**
  - `internal/core/port_embedder.go`: Embedder domain interface definition.
  - `internal/memory/migrations/0006_embeddings.sql`: DDL schema for float32 BLOB vector storage with unique `(doc_type, doc_id)` constraint.
  - `internal/memory/vector.go`: Binary serialization (`EncodeVector`, `DecodeVector`) and cosine similarity math (`CosineSimilarity`).
  - `internal/memory/vector_test.go`: Table-driven tests for float32 binary roundtrip, malformed input rejection, and cosine similarity calculations.
- **Modified:**
  - `openspec/changes/m7-hybrid-search/tasks.md`: Marked tasks 2.1, 2.2, and 2.3 as completed.
  - `openspec/changes/m7-hybrid-search/apply-progress.md`: Merged cumulative PR 1 and PR 2 apply progress.

---

## Verification Evidence

- `go test -race ./internal/adapters/llm`: PASS (All Ollama, OpenAI, and factory tests passing with race detection)
- `go test -race ./...`: PASS (All repository unit & integration tests passing across all packages)

---

## Remaining Tasks (PR 3)

```markdown
### PR 3: Reciprocal Rank Fusion (RRF), Hybrid Search, Async Indexer, CLI & Docs
- [ ] 3.1 RRF implementation: Implement ReciprocalRankFusion helper in internal/memory/rrf.go with k = 60 and deduplication by (doc_type, doc_id) (RED → GREEN).
- [ ] 3.2 Hybrid repository search: Extend Search in internal/memory/sqlite.go to compute embeddings, query vectors, calculate cosine similarities, and merge with FTS5 BM25 via RRF.
- [ ] 3.3 Graceful fallback: Verify and implement seamless fallback to 100% BM25 FTS5 when embeddings are disabled, offline, or fail.
- [ ] 3.4 Async embedding worker: Implement non-blocking background embedding generation for observations on SaveObservations.
- [ ] 3.5 Integration tests in internal/memory/hybrid_test.go & cmd/agis/ with race detector (go test -race ./...) and goroutine leak validation (goleak).
- [ ] 3.6 Documentation updates: docs/roadmap.md, docs/architecture.md, docs/configuration.md, README.md.
```
