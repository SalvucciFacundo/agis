package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// doc_type discriminators in memory_fts.
const (
	docTypeMessage     = "message"
	docTypeObservation = "observation"
)

// insertFTSRow adds one row to the standalone memory_fts table. It MUST be
// called inside the same transaction as the corresponding base-table write
// (message or observation) so the index can never drift from its source rows.
func insertFTSRow(ctx context.Context, tx *sql.Tx, docType, docID, content string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO memory_fts (doc_type, doc_id, content) VALUES (?, ?, ?)`,
		docType, docID, content)
	if err != nil {
		return err
	}
	return nil
}

// deleteFTSRow removes the memory_fts row for one base row. It MUST be called
// inside the same transaction as the base-table write it mirrors (e.g. an
// observation upsert that replaces content), so a replaced row's stale content
// can never haunt search.
func deleteFTSRow(ctx context.Context, tx *sql.Tx, docType, docID string) error {
	_, err := tx.ExecContext(ctx,
		`DELETE FROM memory_fts WHERE doc_type = ? AND doc_id = ?`,
		docType, docID)
	if err != nil {
		return err
	}
	return nil
}

// searchMatches runs a full-text query over memory_fts, returning up to limit
// results across every doc_type (message and observation), best matches first.
func (r *Repository) searchMatches(ctx context.Context, query string, limit int) ([]core.SearchResult, error) {
	q := `SELECT doc_type, doc_id, content
	      FROM memory_fts
	      WHERE memory_fts MATCH ?
	      ORDER BY rank`
	args := []any{ftsQuery(query)}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("searching: %w", err)
	}
	defer rows.Close()

	results := []core.SearchResult{}
	for rows.Next() {
		var res core.SearchResult
		if err := rows.Scan(&res.DocType, &res.DocID, &res.Content); err != nil {
			return nil, fmt.Errorf("scanning search result: %w", err)
		}
		results = append(results, res)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating search results: %w", err)
	}
	return results, nil
}

// ftsQuery rewrites a free-text query as an FTS5 conjunction of quoted
// phrases: every whitespace-separated token is wrapped in quotes (escaping any
// embedded quote as "") and joined with AND. This makes multi-word search
// require every term to match, which suits observation recall where several
// distinct words commonly co-occur.
func ftsQuery(query string) string {
	tokens := strings.Fields(query)
	quoted := make([]string, len(tokens))
	for i, t := range tokens {
		quoted[i] = `"` + strings.ReplaceAll(t, `"`, `""`) + `"`
	}
	return strings.Join(quoted, " AND ")
}
