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

// Message is a single turn in a Conversation. An assistant message carrying a
// tool request sets ToolCalls; the execution result answers with Role "tool"
// and ToolCallID matching the request ID.
type Message struct {
	ID             int64
	ConversationID string
	Role           Role
	Content        string
	CreatedAt      time.Time
	ToolCalls      []ToolCall
	ToolCallID     string
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

// ToolCall is one model-requested tool invocation.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // raw JSON arguments as emitted by the provider
}

// ToolDef advertises one callable tool to the provider.
type ToolDef struct {
	Name        string
	Description string
}

// ChatRequest is the input to a Provider. Tools advertises callable tools;
// when empty the provider behaves exactly as before M4.
type ChatRequest struct {
	Model    string
	Messages []Message
	Tools    []ToolDef
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

// Observation is a single long-term memory fact. It is keyed by TopicKey:
// re-saving an observation with the same TopicKey updates the existing row
// instead of duplicating it. SourceRef records which conversation produced it.
type Observation struct {
	ID         string
	TopicKey   string
	Type       string
	Content    string
	Importance int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	SourceRef  string
}

// UserModel is one aggregated fact about the user, keyed by Key (the source
// observation's full topic_key). Confidence is a float in [0,1].
type UserModel struct {
	ID         string
	Key        string
	Value      string
	Confidence float64
	UpdatedAt  time.Time
}

// Snapshot is a point-in-time copy of a conversation and its messages.
type Snapshot struct {
	ID             string
	ConversationID string
	Title          string
	Summary        string
	MessagesJSON   string
	CreatedAt      time.Time
}
