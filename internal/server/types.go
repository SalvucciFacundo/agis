package server

// ErrorResponse is the OpenAI-compatible error payload.
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// APIError details the reason for request failure.
type APIError struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    string  `json:"code,omitempty"`
}

// HealthResponse represents the payload returned by /healthz and /v1/health.
type HealthResponse struct {
	Status         string `json:"status"`
	Version        string `json:"version"`
	Profile        string `json:"profile"`
	ActiveProvider string `json:"active_provider"`
	ActiveModel    string `json:"active_model"`
}

// ModelListResponse represents the payload returned by GET /v1/models.
type ModelListResponse struct {
	Object string      `json:"object"`
	Data   []ModelItem `json:"data"`
}

// ModelItem represents a single model descriptor in ModelListResponse.
type ModelItem struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ChatCompletionRequest is the OpenAI-compatible chat completion request schema.
type ChatCompletionRequest struct {
	Model       string                  `json:"model,omitempty"`
	Messages    []ChatCompletionMessage `json:"messages"`
	Stream      bool                    `json:"stream,omitempty"`
	Temperature *float32                `json:"temperature,omitempty"`
	User        string                  `json:"user,omitempty"`
}

// ChatCompletionMessage is one message in a ChatCompletionRequest.
type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// ChatCompletionResponse is the non-streaming chat completion response.
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   *Usage                 `json:"usage,omitempty"`
}

// ChatCompletionChoice is one choice candidate in ChatCompletionResponse.
type ChatCompletionChoice struct {
	Index        int           `json:"index"`
	Message      ChoiceMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// ChoiceMessage is the assistant response message.
type ChoiceMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Usage reports token counts.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionChunk represents an SSE chunk during streaming.
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
}

// ChunkChoice represents a single incremental choice delta in SSE streaming.
type ChunkChoice struct {
	Index        int        `json:"index"`
	Delta        ChunkDelta `json:"delta"`
	FinishReason *string    `json:"finish_reason"`
}

// ChunkDelta is the incremental content token.
type ChunkDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}
