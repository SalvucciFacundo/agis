package memory

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/google/uuid"
)

// Option configures a Repository instance.
type Option func(*Repository)

// WithEmbedder configures the vector embedding port for hybrid search and indexing.
func WithEmbedder(embedder core.Embedder) Option {
	return func(r *Repository) {
		r.embedder = embedder
	}
}

// WithLogger configures a structured logger for the repository.
func WithLogger(logger *slog.Logger) Option {
	return func(r *Repository) {
		if logger != nil {
			r.logger = logger
		}
	}
}

type embeddingJob struct {
	docType string
	docID   string
	content string
}

type embeddingBatchMsg struct {
	jobs []embeddingJob
	done chan struct{}
}

func (r *Repository) embeddingWorker() {
	defer r.wg.Done()
	for {
		select {
		case <-r.ctx.Done():
			return
		case msg, ok := <-r.embedChan:
			if !ok {
				return
			}
			if len(msg.jobs) > 0 {
				r.processEmbeddingBatch(msg.jobs)
			}
			if msg.done != nil {
				close(msg.done)
			}
		}
	}
}

func (r *Repository) processEmbeddingBatch(batch []embeddingJob) {
	if len(batch) == 0 || r.embedder == nil {
		return
	}

	texts := make([]string, len(batch))
	for i, item := range batch {
		texts[i] = item.content
	}

	ctx, cancel := context.WithTimeout(r.ctx, 30*time.Second)
	vectors, err := r.embedder.EmbedBatch(ctx, texts)
	cancel()

	if err != nil {
		r.logger.Warn("memory: background embedding generation failed", "error", err, "count", len(batch))
		return
	}

	if len(vectors) != len(batch) {
		r.logger.Warn("memory: embedder returned vector count mismatch", "want", len(batch), "got", len(vectors))
		return
	}

	now := formatTime(time.Now().UTC())
	for i, item := range batch {
		vec := vectors[i]
		if len(vec) == 0 {
			continue
		}
		blob := EncodeVector(vec)
		id := uuid.NewString()

		_, err := r.db.ExecContext(r.ctx,
			`INSERT INTO embeddings (id, doc_type, doc_id, dimension, vector, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(doc_type, doc_id) DO UPDATE SET
			   dimension = excluded.dimension,
			   vector = excluded.vector,
			   updated_at = excluded.updated_at`,
			id, item.docType, item.docID, len(vec), blob, now, now)
		if err != nil {
			r.logger.Warn("memory: storing embedding BLOB failed", "doc_type", item.docType, "doc_id", item.docID, "error", err)
		}
	}
}

// FlushEmbeddings waits for currently queued background embedding jobs to complete.
func (r *Repository) FlushEmbeddings(ctx context.Context) {
	if r.embedder == nil {
		return
	}
	r.embedMu.Lock()
	if r.closed {
		r.embedMu.Unlock()
		return
	}
	done := make(chan struct{})
	select {
	case r.embedChan <- embeddingBatchMsg{done: done}:
		r.embedMu.Unlock()
		select {
		case <-done:
		case <-ctx.Done():
		case <-r.ctx.Done():
		}
	default:
		r.embedMu.Unlock()
	}
}

type scoredDoc struct {
	docType string
	docID   string
	score   float32
}

func (r *Repository) searchVectors(ctx context.Context, queryVec []float32, limit int) ([]core.SearchResult, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT doc_type, doc_id, dimension, vector FROM embeddings`)
	if err != nil {
		return nil, fmt.Errorf("querying embeddings: %w", err)
	}
	defer rows.Close()

	var candidates []scoredDoc
	qDim := len(queryVec)

	for rows.Next() {
		var (
			docType string
			docID   string
			dim     int
			blob    []byte
		)
		if err := rows.Scan(&docType, &docID, &dim, &blob); err != nil {
			return nil, fmt.Errorf("scanning embedding: %w", err)
		}
		if dim != qDim && len(blob)/4 != qDim {
			continue
		}
		vec, err := DecodeVector(blob)
		if err != nil || len(vec) != qDim {
			continue
		}
		sim := CosineSimilarity(queryVec, vec)
		if sim > 0 {
			candidates = append(candidates, scoredDoc{
				docType: docType,
				docID:   docID,
				score:   sim,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating embeddings: %w", err)
	}

	if len(candidates) == 0 {
		return []core.SearchResult{}, nil
	}

	// Sort candidates by cosine similarity descending, tiebreak on docID ascending
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].docID < candidates[j].docID
	})

	fetchCount := len(candidates)
	if limit > 0 && limit < fetchCount {
		fetchCount = limit
	}

	results := make([]core.SearchResult, 0, fetchCount)
	for i := 0; i < fetchCount; i++ {
		cand := candidates[i]
		content, err := r.lookupContent(ctx, cand.docType, cand.docID)
		if err != nil {
			continue
		}
		results = append(results, core.SearchResult{
			DocType: cand.docType,
			DocID:   cand.docID,
			Content: content,
		})
	}

	return results, nil
}

func (r *Repository) lookupContent(ctx context.Context, docType, docID string) (string, error) {
	switch docType {
	case docTypeObservation:
		var content string
		err := r.db.QueryRowContext(ctx, `SELECT content FROM observations WHERE id = ?`, docID).Scan(&content)
		if err == nil {
			return content, nil
		}
	case docTypeMessage:
		var content string
		err := r.db.QueryRowContext(ctx, `SELECT content FROM messages WHERE id = ?`, docID).Scan(&content)
		if err == nil {
			return content, nil
		}
	}

	// Fallback to memory_fts table if not found in primary tables
	var content string
	err := r.db.QueryRowContext(ctx, `SELECT content FROM memory_fts WHERE doc_type = ? AND doc_id = ? LIMIT 1`, docType, docID).Scan(&content)
	if err != nil {
		return "", fmt.Errorf("looking up content for %s/%s: %w", docType, docID, err)
	}
	return content, nil
}
