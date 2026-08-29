# SDD Proposal: m7-hybrid-search

## Objective
Implement a Hybrid Search system in AGIS incorporating Reciprocal Rank Fusion (RRF). This feature bridges the gap between semantic context (using Dense Vector Embeddings) and existing keyword matching (BM25 FTS5).

## 1. Capabilities Contract

| Capability | Status | Description |
|---|---|---|
| `embeddings` | **NEW** | Introduces a `core.Embedder` port and provides adapters for `ollama` and `openai` to compute vector representations of text. |
| `repository-memory` | **MODIFIED** | Implements pure Go vector storage in SQLite using raw `[]byte` mapped to IEEE 754 float32. Augments the `Search` method to calculate cosine similarity and apply RRF. Includes migration `0006_embeddings.sql`. |
| `config-loader` | **MODIFIED** | Extends the `Config` schema with a new `embeddings` block supporting `enabled`, `provider`, `model`, `dimensions`, and `batch_size`. |

## 2. Detailed Architectural Decisions

* **D1: `Embedder` Interface (`internal/core/port_embedder.go`)**:
  We introduce an interface with at least `Embed` and `EmbedBatch` methods to abstract away how vectors are requested. This follows the existing `core.Provider` abstraction pattern.
* **D2: Binary IEEE 754 float32 Encoding in SQLite**:
  To avoid heavy CGO dependency on `sqlite-vss`, embeddings will be stored as binary BLOBs using `encoding/binary`. Go will decode these into `[]float32` and compute Cosine Similarity entirely in memory.
* **D3: Reciprocal Rank Fusion (RRF) with `k = 60`**:
  Search results from both FTS5 and the new Vector space will be mapped to normalized ranks. The RRF formula applied will be: `Score(d) = sum(1 / (k + rank_i(d)))`, utilizing the industry-standard `k = 60`.
* **D4: Dynamic and Graceful Fallback**:
  The core system guarantees resiliency. If the embedding provider is disabled via config, unreachable, or returns an error during search, the query will silently and gracefully fallback to BM25 FTS5-only results.
* **D5: Asynchronous Batch Generation**:
  Since vector generation induces latency, embeddings for observations and long-term messages will be queued and processed in non-blocking background batches. This guarantees the conversational learning loop response times are unaffected.
* **D6: Vector Invalidation and Sync**:
  Vector rows are bound to their originating document IDs. When an observation is upserted or deleted, its corresponding BLOB is immediately deleted and asynchronously re-generated.
* **D7: Default Model Selections**:
  To minimize friction, the defaults are `nomic-embed-text` for the Ollama adapter and `text-embedding-3-small` for the OpenAI adapter.
* **D8: Chained PR Strategy & Review Workload Forecast**:
  The total footprint will be roughly 600-800 lines of code. It will be partitioned to ensure optimal code review workflows:
  * **PR 1**: `core.Embedder` port, `Config` updates, and `0006_embeddings.sql` migration.
  * **PR 2**: Embedding adapters (`ollama.go`, `openai.go`) and BLOB encode/decode utilities.
  * **PR 3**: The unified RRF Search and Repository integrations along with tests.

## 3. Security & Guardrails
* **Deny-by-default Malformed Vectors**: Invalid BLOB sizes or corrupted DB payloads correctly fallback to ignoring the vector row rather than panicking.
* **Bounded Contexts**: Embedding requests have strict configurable timeouts and bounded batch sizes (e.g. capped to 100 documents) to defend against excessive context sizes that might stall provider queues.

## 4. Compatibility & Rollback
* **Backward Compatibility**: Existing 1.0 databases apply `0006_embeddings.sql` additively. If a user does not configure embeddings, the system behaves precisely like version 1.0 (pure FTS5).
* **Rollback**: To rollback, a user simply sets `embeddings.enabled: false` in the AGIS config YAML. No data is lost, and FTS5 instantly takes over exclusively.
