// Package core holds the AGIS domain model, its ports, and the Brain loop.
//
// The core never imports adapters: repository and provider implementations
// live under internal/memory and internal/adapters and depend on this
// package, not the other way around.
package core

import "time"

// Role identifies the author of a Message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Message is a single turn in a Conversation.
type Message struct {
	ID             int64
	ConversationID string
	Role           Role
	Content        string
	CreatedAt      time.Time
}

// Conversation is a persisted session.
type Conversation struct {
	ID           string
	Title        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Summary      string
	MessageCount int
}

// ChatRequest is the input to a Provider.
type ChatRequest struct {
	Model    string
	Messages []Message
}

// ChatResponse is the complete (non-streaming) Provider reply.
type ChatResponse struct {
	Content string
}

// ModelInfo describes one available model.
type ModelInfo struct {
	ID       string
	Provider string
}

// SearchResult is a single full-text match over persisted messages or
// observations.
type SearchResult struct {
	DocType string
	DocID   string
	Content string
}
