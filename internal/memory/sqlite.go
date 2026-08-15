package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/google/uuid"
)

// defaultTitle matches the conversations.title DEFAULT in 0001_init.sql.
const defaultTitle = "New session"

// timeLayout is a fixed-width RFC3339 layout. Fixed-width fractional seconds
// (nine digits, trailing zeros kept) are required so that lexicographic
// ordering of the TEXT timestamp columns equals chronological ordering; the
// trimmed form of RFC3339Nano does not sort correctly.
const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

// Repository is the SQLite + FTS5 implementation of core.Repository.
type Repository struct {
	db *sql.DB
}

var _ core.Repository = (*Repository)(nil)

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// NewRepository opens (or creates) the SQLite database at path, applies the
// embedded migrations, and returns a ready Repository. The caller owns the
// returned Repository and must call Close when done.
func NewRepository(ctx context.Context, path string) (*Repository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite %s: %w", path, err)
	}
	// SQLite serializes writers; M1 has a single write surface. One pooled
	// connection keeps WAL reads and writes on the same connection.
	db.SetMaxOpenConns(1)

	if err := configureDB(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := applyMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Repository{db: db}, nil
}

// CreateConversation persists a new conversation and returns it. An empty
// title falls back to the default.
func (r *Repository) CreateConversation(ctx context.Context, title string) (*core.Conversation, error) {
	if title == "" {
		title = defaultTitle
	}
	now := time.Now().UTC()
	conv := &core.Conversation{
		ID:        uuid.NewString(),
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
		Summary:   "",
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO conversations (id, title, created_at, updated_at, summary, message_count)
		 VALUES (?, ?, ?, ?, ?, 0)`,
		conv.ID, conv.Title, formatTime(conv.CreatedAt), formatTime(conv.UpdatedAt), conv.Summary)
	if err != nil {
		return nil, fmt.Errorf("creating conversation: %w", err)
	}
	return conv, nil
}

// LatestConversation returns the most recently updated conversation, or
// core.ErrNotFound when none exists.
func (r *Repository) LatestConversation(ctx context.Context) (*core.Conversation, error) {
	conv, err := scanConversation(r.db.QueryRowContext(ctx,
		`SELECT id, title, created_at, updated_at, summary, message_count
		 FROM conversations
		 ORDER BY updated_at DESC, id DESC
		 LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, core.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading latest conversation: %w", err)
	}
	return conv, nil
}

// AppendMessage persists a message, syncs its FTS5 row, and bumps the
// conversation's updated_at and message_count — all in one transaction, so a
// reader never observes a message without its search row or a stale count.
func (r *Repository) AppendMessage(ctx context.Context, convID string, msg core.Message) error {
	now := time.Now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning append transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO messages (conversation_id, role, content, created_at)
		 VALUES (?, ?, ?, ?)`,
		convID, string(msg.Role), msg.Content, formatTime(now))
	if err != nil {
		return fmt.Errorf("inserting message: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("reading message id: %w", err)
	}

	if err := insertFTSRow(ctx, tx, docTypeMessage, strconv.FormatInt(id, 10), msg.Content); err != nil {
		return fmt.Errorf("indexing message: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE conversations
		 SET updated_at = ?, message_count = message_count + 1
		 WHERE id = ?`,
		formatTime(now), convID); err != nil {
		return fmt.Errorf("updating conversation: %w", err)
	}

	return tx.Commit()
}

// Messages returns the conversation's messages in chronological order. A
// positive limit returns the most recent `limit` messages (the tail); a
// non-positive limit returns every message.
func (r *Repository) Messages(ctx context.Context, convID string, limit int) ([]core.Message, error) {
	const cols = `id, conversation_id, role, content, created_at`

	query := `SELECT ` + cols + ` FROM messages WHERE conversation_id = ? ORDER BY id ASC`
	args := []any{convID}

	if limit > 0 {
		query = `SELECT ` + cols + ` FROM (
			SELECT ` + cols + ` FROM messages WHERE conversation_id = ? ORDER BY id DESC LIMIT ?
		) ORDER BY id ASC`
		args = []any{convID, limit}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("loading messages: %w", err)
	}
	defer rows.Close()

	msgs := []core.Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating messages: %w", err)
	}
	return msgs, nil
}

// Search returns full-text matches across messages and observations, best
// matches first. A non-positive limit is unbounded.
func (r *Repository) Search(ctx context.Context, query string, limit int) ([]core.SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return []core.SearchResult{}, nil
	}
	return r.searchMatches(ctx, query, limit)
}

// Close releases the underlying database handle.
func (r *Repository) Close() error {
	return r.db.Close()
}

// scanConversation maps a single conversations row into a core.Conversation.
func scanConversation(s rowScanner) (*core.Conversation, error) {
	var (
		conv                 core.Conversation
		createdAt, updatedAt string
	)
	if err := s.Scan(&conv.ID, &conv.Title, &createdAt, &updatedAt, &conv.Summary, &conv.MessageCount); err != nil {
		return nil, err
	}
	var err error
	if conv.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	if conv.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return nil, err
	}
	return &conv, nil
}

// scanMessage maps a single messages row into a core.Message.
func scanMessage(s rowScanner) (core.Message, error) {
	var (
		m         core.Message
		role      string
		createdAt string
	)
	if err := s.Scan(&m.ID, &m.ConversationID, &role, &m.Content, &createdAt); err != nil {
		return core.Message{}, err
	}
	m.Role = core.Role(role)
	t, err := parseTime(createdAt)
	if err != nil {
		return core.Message{}, err
	}
	m.CreatedAt = t
	return m, nil
}

// formatTime serializes t as a fixed-width UTC timestamp.
func formatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

// parseTime deserializes a fixed-width UTC timestamp.
func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing timestamp %q: %w", s, err)
	}
	return t, nil
}
