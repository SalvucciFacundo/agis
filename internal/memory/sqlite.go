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

// clampImportance normalizes an observation importance score to [1,5], with 0
// (the "absent/unset" value from a curator) defaulting to 3.
func clampImportance(v int) int {
	switch {
	case v == 0:
		return 3
	case v < 1:
		return 1
	case v > 5:
		return 5
	default:
		return v
	}
}

// SaveObservations persists a batch of observations, upserting on the unique
// topic_key. A re-saved topic keeps its original id and created_at and only
// bumps updated_at; a new topic gets a fresh UUID. Importance is clamped to
// [1,5] with 0 defaulting to 3. Each upsert also deletes and re-inserts the
// observation's FTS row in the same transaction, so replaced content can never
// haunt search. The whole batch is atomic: one bad row rolls back everything.
func (r *Repository) SaveObservations(ctx context.Context, convID string, obs []core.Observation) error {
	if len(obs) == 0 {
		return nil
	}

	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning observation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, o := range obs {
		if err := r.upsertObservation(ctx, tx, convID, o, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing observations: %w", err)
	}
	return nil
}

// upsertObservation writes one observation inside tx, upserting on topic_key.
func (r *Repository) upsertObservation(ctx context.Context, tx *sql.Tx, convID string, o core.Observation, now time.Time) error {
	if strings.TrimSpace(o.TopicKey) == "" {
		return fmt.Errorf("observation has empty topic_key")
	}

	importance := clampImportance(o.Importance)
	nowStr := formatTime(now)

	var id string
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM observations WHERE topic_key = ?`, o.TopicKey).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		id = uuid.NewString()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO observations (id, topic_key, type, content, importance, created_at, updated_at, source_ref)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, o.TopicKey, o.Type, o.Content, importance, nowStr, nowStr, convID); err != nil {
			return fmt.Errorf("inserting observation %q: %w", o.TopicKey, err)
		}
	case err != nil:
		return fmt.Errorf("looking up observation %q: %w", o.TopicKey, err)
	default:
		// Re-save: preserve id and created_at, bump updated_at.
		if _, err := tx.ExecContext(ctx,
			`UPDATE observations
			 SET type = ?, content = ?, importance = ?, source_ref = ?, updated_at = ?
			 WHERE id = ?`,
			o.Type, o.Content, importance, convID, nowStr, id); err != nil {
			return fmt.Errorf("updating observation %q: %w", o.TopicKey, err)
		}
	}

	// Replace the FTS row so stale content cannot survive an upsert.
	if err := deleteFTSRow(ctx, tx, docTypeObservation, id); err != nil {
		return fmt.Errorf("deleting observation FTS row: %w", err)
	}
	if err := insertFTSRow(ctx, tx, docTypeObservation, id, o.Content); err != nil {
		return fmt.Errorf("indexing observation: %w", err)
	}
	return nil
}

// Observations returns the most recently updated observations, newest first. A
// non-positive limit is unbounded. This is the recall read path.
func (r *Repository) Observations(ctx context.Context, limit int) ([]core.Observation, error) {
	query := `SELECT id, topic_key, type, content, importance, created_at, updated_at, source_ref
	          FROM observations
	          ORDER BY updated_at DESC, id DESC`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("loading observations: %w", err)
	}
	defer rows.Close()

	obs := []core.Observation{}
	for rows.Next() {
		o, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		obs = append(obs, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating observations: %w", err)
	}
	return obs, nil
}

// UpdateConversationSummary writes a conversation's summary WITHOUT touching
// its updated_at, so a summary write never changes LatestConversation order.
func (r *Repository) UpdateConversationSummary(ctx context.Context, convID, summary string) error {
	if _, err := r.db.ExecContext(ctx,
		`UPDATE conversations SET summary = ? WHERE id = ?`, summary, convID); err != nil {
		return fmt.Errorf("updating conversation summary: %w", err)
	}
	return nil
}

// UpsertUserModel persists user-model rows, upserting on the unique key. A
// re-seen key keeps its id and bumps updated_at; a new key gets a fresh UUID.
func (r *Repository) UpsertUserModel(ctx context.Context, rows []core.UserModel) error {
	if len(rows) == 0 {
		return nil
	}

	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning user model transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, u := range rows {
		if err := r.upsertUserModel(ctx, tx, u, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing user model: %w", err)
	}
	return nil
}

// upsertUserModel writes one user-model row inside tx, upserting on key.
func (r *Repository) upsertUserModel(ctx context.Context, tx *sql.Tx, u core.UserModel, now time.Time) error {
	if strings.TrimSpace(u.Key) == "" {
		return fmt.Errorf("user model row has empty key")
	}

	nowStr := formatTime(now)

	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM user_model WHERE key = ?`, u.Key).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		id = uuid.NewString()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO user_model (id, key, value, confidence, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			id, u.Key, u.Value, u.Confidence, nowStr); err != nil {
			return fmt.Errorf("inserting user model %q: %w", u.Key, err)
		}
	case err != nil:
		return fmt.Errorf("looking up user model %q: %w", u.Key, err)
	default:
		if _, err := tx.ExecContext(ctx,
			`UPDATE user_model SET value = ?, confidence = ?, updated_at = ? WHERE id = ?`,
			u.Value, u.Confidence, nowStr, id); err != nil {
			return fmt.Errorf("updating user model %q: %w", u.Key, err)
		}
	}
	return nil
}

// UserModelRows returns up to limit user-model rows ordered by confidence
// descending, then key ascending. A non-positive limit is unbounded.
func (r *Repository) UserModelRows(ctx context.Context, limit int) ([]core.UserModel, error) {
	q := `SELECT id, key, value, confidence, updated_at FROM user_model`
	args := []any{}
	if limit > 0 {
		q += ` ORDER BY confidence DESC, key ASC LIMIT ?`
		args = append(args, limit)
	} else {
		q += ` ORDER BY confidence DESC, key ASC`
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing user model: %w", err)
	}
	defer rows.Close()

	var out []core.UserModel
	for rows.Next() {
		var u core.UserModel
		var updatedStr string
		if err := rows.Scan(&u.ID, &u.Key, &u.Value, &u.Confidence, &updatedStr); err != nil {
			return nil, fmt.Errorf("scanning user model: %w", err)
		}
		updated, err := parseTime(updatedStr)
		if err != nil {
			return nil, fmt.Errorf("parsing user model updated_at: %w", err)
		}
		u.UpdatedAt = updated
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user model: %w", err)
	}
	return out, nil
}

// ClearUserModel deletes every user-model row. The rows are derived data, so
// clearing them resets persona evolution without touching observations.
func (r *Repository) ClearUserModel(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM user_model`); err != nil {
		return fmt.Errorf("clearing user model: %w", err)
	}
	return nil
}

// sessionEventKinds are the allowed session_events.kind values, mirroring the
// CHECK constraint in 0002_learning.sql.
var sessionEventKinds = map[string]bool{
	"nudge":   true,
	"summary": true,
	"skill":   true,
}

// RecordSessionEvent appends one observability row about learning-loop
// activity (a nudge, a summary, or later a skill write).
func (r *Repository) RecordSessionEvent(ctx context.Context, sessionID, kind, payload string) error {
	if !sessionEventKinds[kind] {
		return fmt.Errorf("invalid session event kind %q", kind)
	}
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO session_events (session_id, kind, payload, created_at)
		 VALUES (?, ?, ?, ?)`,
		sessionID, kind, payload, formatTime(time.Now().UTC())); err != nil {
		return fmt.Errorf("recording session event %q: %w", kind, err)
	}
	return nil
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

// scanObservation maps a single observations row into a core.Observation.
func scanObservation(s rowScanner) (core.Observation, error) {
	var (
		o                    core.Observation
		createdAt, updatedAt string
	)
	if err := s.Scan(&o.ID, &o.TopicKey, &o.Type, &o.Content, &o.Importance, &createdAt, &updatedAt, &o.SourceRef); err != nil {
		return core.Observation{}, err
	}
	var err error
	if o.CreatedAt, err = parseTime(createdAt); err != nil {
		return core.Observation{}, err
	}
	if o.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return core.Observation{}, err
	}
	return o, nil
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

// skillSources are the allowed skills.source values, mirroring the CHECK
// constraint in 0003_skills.sql.
var skillSources = map[string]bool{
	core.SourceImported: true,
	core.SourceAgent:    true,
}

// SaveSkill persists one skill, upserting on the unique name. A re-saved name
// keeps its id, created_at, and usage counters and only refreshes description,
// trigger, content, and source; a new name gets a fresh UUID and starts with
// zero usage.
func (r *Repository) SaveSkill(ctx context.Context, skill core.Skill) error {
	if strings.TrimSpace(skill.Name) == "" {
		return fmt.Errorf("skill has empty name")
	}
	if strings.TrimSpace(skill.Content) == "" {
		return fmt.Errorf("skill %q has empty content", skill.Name)
	}
	if !skillSources[skill.Source] {
		return fmt.Errorf("invalid skill source %q", skill.Source)
	}

	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning skill transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM skills WHERE name = ?`, skill.Name).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		id = uuid.NewString()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO skills (id, name, description, "trigger", content, source, usage_count, last_used, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, 0, '', ?)`,
			id, skill.Name, skill.Description, skill.Trigger, skill.Content,
			skill.Source, formatTime(now)); err != nil {
			return fmt.Errorf("inserting skill %q: %w", skill.Name, err)
		}
	case err != nil:
		return fmt.Errorf("looking up skill %q: %w", skill.Name, err)
	default:
		if _, err := tx.ExecContext(ctx,
			`UPDATE skills SET description = ?, "trigger" = ?, content = ?, source = ? WHERE id = ?`,
			skill.Description, skill.Trigger, skill.Content, skill.Source, id); err != nil {
			return fmt.Errorf("updating skill %q: %w", skill.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing skill %q: %w", skill.Name, err)
	}
	return nil
}

// ListSkills returns every known skill ordered by last_used descending
// (never-used entries last), then by name ascending as a stable tiebreak.
func (r *Repository) ListSkills(ctx context.Context) ([]core.Skill, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, description, "trigger", content, source, usage_count, last_used, created_at
		 FROM skills
		 ORDER BY last_used DESC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing skills: %w", err)
	}
	defer rows.Close()

	var out []core.Skill
	for rows.Next() {
		s, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating skills: %w", err)
	}
	return out, nil
}

// RecordSkillUsage increments the usage counter and stamps last_used for the
// named skill. An unknown name returns an error wrapping ErrNotFound.
func (r *Repository) RecordSkillUsage(ctx context.Context, name string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE skills SET usage_count = usage_count + 1, last_used = ?
		 WHERE name = ?`, formatTime(time.Now().UTC()), name)
	if err != nil {
		return fmt.Errorf("recording skill usage %q: %w", name, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("recording skill usage %q: %w", name, core.ErrNotFound)
	}
	return nil
}

// scanner is the subset of *sql.Row/*sql.Rows both skill readers use.
type scanner interface {
	Scan(dest ...any) error
}

// scanSkill reads one skills row into a domain Skill.
func scanSkill(sc scanner) (core.Skill, error) {
	var (
		s            core.Skill
		lastUsedStr  string
		createdAtStr string
	)
	if err := sc.Scan(&s.ID, &s.Name, &s.Description, &s.Trigger, &s.Content,
		&s.Source, &s.UsageCount, &lastUsedStr, &createdAtStr); err != nil {
		return core.Skill{}, fmt.Errorf("scanning skill: %w", err)
	}
	if lastUsedStr != "" {
		t, err := parseTime(lastUsedStr)
		if err != nil {
			return core.Skill{}, fmt.Errorf("parsing skill last_used: %w", err)
		}
		s.LastUsed = t
	}
	created, err := parseTime(createdAtStr)
	if err != nil {
		return core.Skill{}, fmt.Errorf("parsing skill created_at: %w", err)
	}
	s.CreatedAt = created
	return s, nil
}
