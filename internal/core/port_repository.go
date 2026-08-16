package core

import (
	"context"
	"errors"
)

// ErrNotFound is returned by Repository when the requested resource does not
// exist (e.g. there is no latest conversation yet).
var ErrNotFound = errors.New("not found")

// Repository is the persistence port. The SQLite+FTS5 adapter implements it
// in internal/memory.
type Repository interface {
	CreateConversation(ctx context.Context, title string) (*Conversation, error)
	LatestConversation(ctx context.Context) (*Conversation, error)
	AppendMessage(ctx context.Context, convID string, msg Message) error
	Messages(ctx context.Context, convID string, limit int) ([]Message, error)
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)

	// SaveObservations persists a batch of observations, upserting on the
	// unique topic_key: a re-saved topic keeps its id and created_at and only
	// bumps updated_at; a new topic gets a fresh id. Importance is clamped to
	// [1,5] with 0 defaulting to 3. FTS rows are deleted and re-inserted in the
	// same transaction. The batch is atomic: one bad row rolls back all.
	SaveObservations(ctx context.Context, convID string, obs []Observation) error

	// Observations returns the most recently updated observations, newest
	// first. A non-positive limit is unbounded.
	Observations(ctx context.Context, limit int) ([]Observation, error)

	// UpdateConversationSummary writes a conversation's summary WITHOUT bumping
	// its updated_at, so a summary write never changes LatestConversation order.
	UpdateConversationSummary(ctx context.Context, convID, summary string) error

	// UpsertUserModel persists user-model rows, upserting on the unique key and
	// bumping updated_at on re-save.
	UpsertUserModel(ctx context.Context, rows []UserModel) error

	// RecordSessionEvent appends one observability row about learning-loop
	// activity. kind is one of "nudge", "summary", or "skill"; sessionID is the
	// conversation UUID the event belongs to.
	RecordSessionEvent(ctx context.Context, sessionID, kind, payload string) error

	Close() error
}
