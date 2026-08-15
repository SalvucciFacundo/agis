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
	Close() error
}
