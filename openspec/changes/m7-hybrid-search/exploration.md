# Exploration: Hybrid Search (Dense Vector + BM25 FTS5)

## Objective
Implement Hybrid Search in AGIS using Reciprocal Rank Fusion (RRF) to combine Dense Vector Embeddings with existing BM25 FTS5 full-text search.

## 1. Existing Search Architecture
The current search implementation resides in `internal/memory/fts.go`, utilizing a standalone SQLite FTS5 table `memory_fts`. The `Repository.Search` method queries this table directly with an FTS5 `MATCH` query.

- **Strengths**: Lightweight, no external dependencies, excellent for keyword matching.
- **Weaknesses**: Semantic context is lost; keyword-only search fails on synonyms or conceptual relationships.

## 2. Proposed Architecture: Embedding Provider Port
We will introduce a new port `Embedder` to abstract embedding generation.

- **Port definition**: `internal/core/port_llm.go` (or `port_embedder.go`).
  - `Embed(ctx, text) ([]float32, error)`
  - `EmbedBatch(ctx, []string) ([][]float32, error)`
- **Adapters**:
  - `internal/adapters/llm/ollama.go`: Use `/api/embed` endpoint.
  - `internal/adapters/llm/openai.go`: Use `/v1/embeddings` endpoint.
- **Integration**:
  - `config.go` needs a new section `embeddings` to configure provider/model/dimensions.

## 3. Storage & Indexing in SQLite
SQLite lacks native vector support without extensions, so we will implement:
- **Schema**: Add `embeddings` table via a new migration.
  - `doc_type TEXT`, `doc_id TEXT` (composite unique index for lookup).
  - `vector BLOB` (compact binary format: IEEE 754 float32 array).
  - `dimension INTEGER`.
- **Cos Similarity**: Implement pure Go cosine similarity on `BLOB` data. With IEEE 754 conversion, this is extremely fast.

## 4. Hybrid Rank Fusion (RRF)
To combine rankings:
1. Fetch `K` results from FTS5 (rank-ordered).
2. Fetch `K` results from Vector Search (similarity-ordered).
3. Compute `Score(d) = sum(1 / (k + rank_i(d)))` for each document `d` in the union.
4. Sort by `Score(d)`.

## 5. Architectural Decisions & Tradeoffs
1. **Decision**: Asynchronous vs Synchronous Embedding generation.
   - *Choice*: Asynchronous background job for observations (via `SaveObservations` callback).
   - *Reason*: Keep interaction latency low; users don't need instant search indexing.
2. **Decision**: Vector Storage Format.
   - *Choice*: `[]float32` as raw binary BLOB in SQLite.
   - *Reason*: Avoids dependency on heavy CGO extensions like `sqlite-vss`.
3. **Decision**: Embedding Provider abstraction.
   - *Choice*: New `Embedder` interface.
   - *Reason*: Consistent with existing `Provider` (LLM) pattern.
4. **Decision**: RRF constant `k`.
   - *Choice*: `k = 60`.
   - *Reason*: Standard in information retrieval literature for RRF.
5. **Decision**: Fallback strategy.
   - *Choice*: If `Embedder` returns error/disabled, search defaults to pure FTS5.
   - *Reason*: Robustness; search should never fail completely due to embedding service downtime.

## 6. Open Questions
- Should we force-refresh embeddings on observation update? (Likely yes, via `deleteFTSRow` + `insertFTSRow` analogue).
- Are there specific embedding models that outperform others for AGIS's conversational data? (Suggest `nomic-embed-text` as reliable default).
