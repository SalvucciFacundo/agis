# Apply Progress — M7: Hybrid Search

## Execution Summary

- **Change:** `m7-hybrid-search`
- **Delivery Strategy:** `auto-chain` (Stacked PRs to main)
- **Current Slice:** PR 1 — Core Embedder Port, Vector Math, Config & Migration 0006
- **Status:** Complete (Tasks 1.1–1.5)

---

## Workload & PR Slices Breakdown

```
[PR 1: Core Port, Vector Math, Config, Migration 0006] 📍 (Current - Completed)
    ↓
[PR 2: Embedding Adapters (Ollama & OpenAI)] (Next)
    ↓
[PR 3: Reciprocal Rank Fusion, Hybrid Search, Async Indexer, CLI & Docs]
```

- **Authored Lines (PR 1):** ~460 lines (including comprehensive tests)
- **400-Line Budget Status:** Cleanly sliced within independent PR 1 boundaries

---

## Tasks Completed (PR 1)

| Task | Description | Status |
|---|---|---|
| **1.1** | Config extensions: `EmbeddingsConfig` in `internal/config/config.go` with safe defaults (`enabled: false`, `provider: "ollama"`, `model: "nomic-embed-text"`, `dimensions: 768`, `batch_size: 100`). | ✅ Completed |
| **1.2** | Migration 0006: `internal/memory/migrations/0006_embeddings.sql` with `embeddings` table, unique index on `(doc_type, doc_id)`, `PRAGMA user_version = 6`. | ✅ Completed |
| **1.3** | Embedder port: `internal/core/port_embedder.go` with `Embedder` interface (`Embed`, `EmbedBatch`, `Dimension`). | ✅ Completed |
| **1.4** | Vector encoding & Cosine similarity: `internal/memory/vector.go` with `EncodeVector`, `DecodeVector`, `CosineSimilarity`. | ✅ Completed |
| **1.5** | Unit tests for vector encoding, error rejection on non-multiple-of-4 BLOBs, cosine math edge cases, and migration idempotency. | ✅ Completed |

---

## TDD Cycle Evidence

| Unit | RED Test | RED Output | GREEN Implementation | GREEN Verification |
|---|---|---|---|---|
| **Config** | `TestLoad_EmbeddingsDefaultsAndExplicit` | `cfg.Embeddings undefined` | `EmbeddingsConfig` struct + defaults & resolution logic in `config.go` | `go test -race ./internal/config` (PASS) |
| **Migration** | `TestMigration_V5ToV6`, updated `TestMigrations` to v6 | `table "embeddings" missing`, `user_version = 5, want 6` | `0006_embeddings.sql` DDL + unique index | `go test -race ./internal/memory -run 'TestMigration.*'` (PASS) |
| **Core Port** | Port declaration | Interface contract defined | `internal/core/port_embedder.go` | `go test ./internal/core/...` (PASS) |
| **Vector Math** | `TestEncodeDecodeVector_Roundtrip`, `TestDecodeVector_MalformedLength`, `TestCosineSimilarity` | `undefined: EncodeVector`, `undefined: DecodeVector`, `undefined: CosineSimilarity` | `EncodeVector`, `DecodeVector`, `CosineSimilarity` in `vector.go` | `go test -race ./internal/memory -run 'Test(EncodeDecodeVector\|DecodeVector\|CosineSimilarity).*'` (PASS) |

---

## Files Changed / Created

- **Created:**
  - `internal/core/port_embedder.go`: Embedder domain interface definition.
  - `internal/memory/migrations/0006_embeddings.sql`: DDL schema for float32 BLOB vector storage with unique `(doc_type, doc_id)` constraint.
  - `internal/memory/vector.go`: Binary serialization (`EncodeVector`, `DecodeVector`) and cosine similarity math (`CosineSimilarity`).
  - `internal/memory/vector_test.go`: Table-driven tests for float32 binary roundtrip, malformed input rejection, and cosine similarity calculations.
  - `openspec/changes/m7-hybrid-search/apply-progress.md`: Apply progress log.
- **Modified:**
  - `internal/config/config.go`: Added `EmbeddingsConfig` to root `Config` with safe defaults and provider model/dimension resolution.
  - `internal/config/config_test.go`: Added unit tests for defaults, explicit config overlay, provider model defaults, and batch size capping.
  - `internal/memory/migrations_test.go`: Updated latest schema version to 6 and added v5-to-v6 upgrade test with uniqueness checks.
  - `openspec/changes/m7-hybrid-search/tasks.md`: Marked tasks 1.1 through 1.5 as completed.

---

## Verification Evidence

- `go test -race ./internal/config`: PASS
- `go test -race ./internal/core`: PASS
- `go test -race ./internal/memory`: PASS
- `go test ./...`: PASS (All repository test suites passing without regressions)

---

## Remaining Tasks (PR 2 & PR 3)

```markdown
### PR 2: Embedding Adapters (Ollama & OpenAI)
- [ ] 2.1 Ollama adapter: Implement internal/adapters/llm/ollama_embed.go targeting /api/embed with fallback support and timeout handling (RED → GREEN).
- [ ] 2.2 OpenAI adapter: Implement internal/adapters/llm/openai_embed.go targeting /v1/embeddings with sub-batch chunking up to batch_size (RED → GREEN).
- [ ] 2.3 Unit & mock tests in internal/adapters/llm/embed_test.go verifying single embedding, batch embedding, context cancellation, and error handling.

### PR 3: Reciprocal Rank Fusion (RRF), Hybrid Search, Async Indexer, CLI & Docs
- [ ] 3.1 RRF implementation: Implement ReciprocalRankFusion helper in internal/memory/rrf.go with k = 60 and deduplication by (doc_type, doc_id) (RED → GREEN).
- [ ] 3.2 Hybrid repository search: Extend Search in internal/memory/sqlite.go to compute embeddings, query vectors, calculate cosine similarities, and merge with FTS5 BM25 via RRF.
- [ ] 3.3 Graceful fallback: Verify and implement seamless fallback to 100% BM25 FTS5 when embeddings are disabled, offline, or fail.
- [ ] 3.4 Async embedding worker: Implement non-blocking background embedding generation for observations on SaveObservations.
- [ ] 3.5 Integration tests in internal/memory/hybrid_test.go & cmd/agis/ with race detector (go test -race ./...) and goroutine leak validation (goleak).
- [ ] 3.6 Documentation updates: docs/roadmap.md, docs/architecture.md, docs/configuration.md, README.md.
```
