# SDD Archive Report: m7-hybrid-search

## Milestone 7 — Hybrid Search (RRF + Vector BLOBs)

- **Change Name:** `m7-hybrid-search`
- **Archived Date:** 2026-08-29
- **Final Status:** COMPLETED & MERGED
- **Store Mode:** Hybrid (OpenSpec + Engram)
- **TDD Mode:** Strict TDD (100% verified across 16 packages)

---

## 1. Executive Summary

Milestone 7 implements **Hybrid Search** in AGIS, integrating Dense Vector Semantic Search with existing BM25 FTS5 lexical keyword search via Reciprocal Rank Fusion (RRF).

Key components delivered:
1. **Core Embedder Port (`internal/core/port_embedder.go`)**: Domain interface `core.Embedder` declaring `Embed`, `EmbedBatch`, and `Dimension`.
2. **Embedding Adapters (`internal/adapters/llm/`)**: Ollama adapter (`/api/embed` with `/api/embeddings` fallback, `nomic-embed-text`) and OpenAI adapter (`/v1/embeddings`, `text-embedding-3-small`, sub-batch chunking).
3. **Pure Go Vector Math & BLOB Serialization (`internal/memory/vector.go`)**: IEEE 754 LittleEndian float32 encoding to SQLite BLOBs and in-memory cosine similarity calculation with zero-magnitude / unit-vector guards.
4. **Reciprocal Rank Fusion Engine (`internal/memory/rrf.go`)**: Mathematical rank fusion algorithm ($k = 60$) merging lexical and semantic result sets with deduplication on `(doc_type, doc_id)` and deterministic tie-breaking.
5. **Hybrid Search & Resilient Fallback (`internal/memory/sqlite.go`, `hybrid.go`)**: Seamless integration into `Repository.Search` with automatic 100% BM25 FTS5 fallback on provider failure or disabled config.
6. **Asynchronous Vector Indexer**: Background worker queue generating observation embeddings without blocking database write transactions.
7. **Database Migration `0006_embeddings.sql`**: Added `embeddings` table and advanced `user_version` to 6.

---

## 2. Pull Request Delivery Sequence (Stacked to Main)

| PR | Title | Commits | Lines Changed | Status |
|---|---|---|---|---|
| **#25** | `feat(embeddings): M7 PR1 — embedder port, vector math, config and migration 0006` | `d63218f` | +1,098 / -4 | Merged |
| **#26** | `feat(embeddings): M7 PR2 — ollama and openai embedding adapters with batch chunking` | `77755e1` | +1,060 / -27 | Merged |
| **#27** | `feat(memory): M7 PR3 — reciprocal rank fusion hybrid search and async embedding indexer` | `6469e77` | +1,061 / -74 | Merged |

---

## 3. Capabilities Synced to `openspec/specs/`

- `embeddings/spec.md` (NEW): `core.Embedder` port, Ollama adapter, OpenAI adapter, sub-batch chunking.
- `repository-memory/spec.md` (MODIFIED): IEEE 754 float32 vector BLOBs, cosine similarity, RRF hybrid search, fallback, migration 0006.
- `config-loader/spec.md` (MODIFIED): `embeddings` configuration block schema and defaults.

---

## 4. Verification Evidence

- `go test -race -count=1 ./...` PASSED across all 16 packages:
  `cmd/agis`, `internal/adapters/llm`, `internal/adapters/tui`, `internal/config`, `internal/core`, `internal/cron`, `internal/gateway`, `internal/memory`, `internal/persona`, `internal/plugins`, `internal/policy`, `internal/scan`, `internal/session`, `internal/skills`, `internal/tools`, `internal/webhook`.
- 0 data races detected.
- 0 goroutine leaks confirmed via `go.uber.org/goleak`.
- Static binary compilation verified (`go build -o /dev/null ./cmd/agis`).
