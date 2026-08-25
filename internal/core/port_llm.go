package core

import "context"

// Provider is the LLM port. Adapters implement it for OpenAI, Ollama, and
// future backends behind a shared OpenAI-compatible client.
type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	Stream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
	Models() []ModelInfo
}

// StreamEvent is one event of a streaming response. Exactly one of Text,
// ToolCall, or Err is meaningful per event. A provider MUST close the channel
// after emitting a terminal Err event. ToolCall events are additive in M4:
// streams without tools emit only Text/Err exactly as before.
type StreamEvent struct {
	Text     string
	ToolCall *ToolCall
	Err      error
}
