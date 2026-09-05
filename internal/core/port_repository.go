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

	// UserModelRows returns up to limit user-model rows ordered by confidence
	// descending, then key ascending. A non-positive limit is unbounded.
	UserModelRows(ctx context.Context, limit int) ([]UserModel, error)

	// ClearUserModel deletes every user-model row. The rows are derived data —
	// rebuildable from observations via AggregateUserModel — so clearing them
	// resets persona evolution without touching long-term memory.
	ClearUserModel(ctx context.Context) error

	// RecordSessionEvent appends one observability row about learning-loop
	// activity. kind is one of "nudge", "summary", or "skill"; sessionID is the
	// conversation UUID the event belongs to.
	RecordSessionEvent(ctx context.Context, sessionID, kind, payload string) error

	// SaveSkill persists one skill, upserting on the unique name: a re-saved
	// name keeps its id, created_at, and usage counters and only refreshes
	// description, trigger, content, and source; a new name gets a fresh id.
	SaveSkill(ctx context.Context, skill Skill) error

	// ListSkills returns every known skill ordered by last_used descending
	// (never-used entries last), then by name ascending as a stable tiebreak.
	ListSkills(ctx context.Context) ([]Skill, error)

	// RecordSkillUsage increments the usage counter and stamps last_used for
	// the named skill. An unknown name returns an error wrapping ErrNotFound.
	RecordSkillUsage(ctx context.Context, name string) error

	// AppendAudit records one security-relevant event (policy decision, grant,
	// revocation, tier change). Audit failures never block decisions; the
	// guard logs them.
	AppendAudit(ctx context.Context, entry AuditEntry) error

	// AuditTail returns up to n audit entries, newest first.
	AuditTail(ctx context.Context, n int) ([]AuditEntry, error)

	// ListConversations returns conversations ordered updated_at DESC, id DESC.
	ListConversations(ctx context.Context, limit, offset int) ([]Conversation, error)

	// GetConversation returns one conversation by id.
	GetConversation(ctx context.Context, id string) (*Conversation, error)

	// RenameConversation updates a conversation's title and bumps updated_at.
	RenameConversation(ctx context.Context, id, title string) error

	// DeleteConversation permanently deletes a conversation and cascades to all
	// linked messages, snapshots, and attachments.
	DeleteConversation(ctx context.Context, id string) error

	// CreateSnapshot captures a point-in-time copy of a conversation.
	CreateSnapshot(ctx context.Context, convID string) (*Snapshot, error)

	// ListSnapshots returns snapshots for a conversation, newest first.
	ListSnapshots(ctx context.Context, convID string) ([]Snapshot, error)

	Close() error
}
