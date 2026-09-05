// Package llm provides OpenAI-compatible LLM adapters (OpenAI and Ollama)
// behind the core.Provider port.
//
// Both backends speak the OpenAI /chat/completions protocol and differ only in
// their base URL: a single shared Client implements the wire protocol, and the
// OpenAI and Ollama adapters configure it with their respective endpoints.
// Streaming responses are delivered as server-sent events mapped onto
// core.StreamEvent. Timeouts and cancellation are the caller's responsibility
// and are expressed through the context argument, which is respected on every
// request.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

const (
	// openAIBaseURL is the OpenAI Chat Completions endpoint base.
	openAIBaseURL = "https://api.openai.com/v1"
	// ollamaBaseURL is the local Ollama endpoint. Ollama serves the OpenAI
	// protocol at /v1.
	ollamaBaseURL = "http://localhost:11434/v1"

	// providerOpenAI and providerOllama are the adapter identifiers reported
	// by Models and matched by NewProvider.
	providerOpenAI = "openai"
	providerOllama = "ollama"
)

// Client is a shared OpenAI-compatible HTTP client. It POSTs to the
// /chat/completions endpoint and maps the JSON and SSE responses onto the
// core domain types.
type Client struct {
	baseURL    string
	pool       *CredentialPool
	httpClient *http.Client
}

// NewClient returns an OpenAI-compatible client targeting baseURL. apiKey may
// be empty for local backends such as Ollama. Optional apiKeys provide additional
// fallback credentials in the CredentialPool.
func NewClient(baseURL, apiKey string, apiKeys ...string) *Client {
	return NewClientWithPool(baseURL, NewCredentialPool(apiKey, apiKeys))
}

// NewClientWithPool returns an OpenAI-compatible client with a dedicated CredentialPool.
func NewClientWithPool(baseURL string, pool *CredentialPool) *Client {
	if pool == nil {
		pool = NewCredentialPool("", nil)
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		pool:       pool,
		httpClient: &http.Client{},
	}
}

// KeyPool returns the client's CredentialPool.
func (c *Client) KeyPool() *CredentialPool {
	return c.pool
}

// Chat performs a non-streaming completion and returns the full reply.
func (c *Client) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	resp, err := c.doChat(ctx, req, false)
	if err != nil {
		return core.ChatResponse{}, err
	}
	defer resp.Body.Close()

	var out chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return core.ChatResponse{}, fmt.Errorf("decoding chat completion: %w", err)
	}
	if out.Error != nil {
		return core.ChatResponse{}, out.Error
	}
	if len(out.Choices) == 0 {
		return core.ChatResponse{}, errors.New("chat completion returned no choices")
	}
	return core.ChatResponse{Content: out.Choices[0].Message.Content}, nil
}

// Stream performs a streaming completion. It returns a receive-only channel of
// StreamEvent values that is closed when the stream ends. An immediate failure
// (bad request, non-200 response) is returned as the error result; a failure
// after the stream starts is delivered as a terminal StreamEvent with Err set.
func (c *Client) Stream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	resp, err := c.doChat(ctx, req, true)
	if err != nil {
		return nil, err
	}

	ch := make(chan core.StreamEvent)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		// Closing the response body unblocks a blocked Read when the caller
		// cancels the context mid-stream.
		stop := context.AfterFunc(ctx, func() { resp.Body.Close() })
		defer stop()

		c.streamEvents(resp.Body, ch)
	}()
	return ch, nil
}

// doChat posts req to the chat-completions endpoint and returns the response
// after validating its status.
func (c *Client) doChat(ctx context.Context, req core.ChatRequest, stream bool) (*http.Response, error) {
	payload := chatCompletionRequest{
		Model:    req.Model,
		Messages: make([]messagePayload, 0, len(req.Messages)),
		Stream:   stream,
	}
	if len(req.Tools) > 0 {
		payload.Tools = make([]toolDefPayload, 0, len(req.Tools))
		for _, t := range req.Tools {
			var td toolDefPayload
			td.Type = "function"
			td.Function.Name = t.Name
			td.Function.Description = t.Description
			payload.Tools = append(payload.Tools, td)
		}
	}
	for _, m := range req.Messages {
		mp := messagePayload{Role: string(m.Role), Content: formatContent(m)}
		for _, tc := range m.ToolCalls {
			mp.ToolCalls = append(mp.ToolCalls, toolCallPayload{
				ID:       tc.ID,
				Type:     "function",
				Function: funcSpec{Name: tc.Name, Arguments: tc.Arguments},
			})
		}
		mp.ToolCallID = m.ToolCallID
		payload.Messages = append(payload.Messages, mp)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding chat request: %w", err)
	}

	maxAttempts := c.pool.Len()
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		httpReq, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			c.baseURL+"/chat/completions",
			bytes.NewReader(body),
		)
		if err != nil {
			return nil, fmt.Errorf("building chat request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		keyUsed := c.pool.CurrentKey()
		if keyUsed != "" {
			httpReq.Header.Set("Authorization", "Bearer "+keyUsed)
		}

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("posting chat completion: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			if _, ok := c.pool.RotateKey(keyUsed); ok && attempt < maxAttempts-1 {
				resp.Body.Close()
				continue
			}
		}

		if resp.StatusCode != http.StatusOK {
			defer resp.Body.Close()
			return nil, httpStatusError(resp)
		}
		return resp, nil
	}

	return nil, errors.New("chat completion: all API keys exhausted")
}

// streamEvents reads SSE data lines from body and emits one StreamEvent per
// content delta. It returns (closing the channel) on [DONE], a terminal error
// event, a malformed chunk, or EOF.
func (c *Client) streamEvents(body io.Reader, ch chan<- core.StreamEvent) {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// Accumulator for streamed tool-call fragments keyed by their index.
	type accToolCall struct {
		id, name string
		args     strings.Builder
	}
	var accOrder []int
	acc := map[int]*accToolCall{}
	flushed := false

	flushToolCalls := func() {
		if flushed {
			return
		}
		flushed = true
		for _, idx := range accOrder {
			a := acc[idx]
			ch <- core.StreamEvent{ToolCall: &core.ToolCall{
				ID:        a.id,
				Name:      a.name,
				Arguments: a.args.String(),
			}}
		}
	}

	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			flushToolCalls()
			return
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			ch <- core.StreamEvent{Err: fmt.Errorf("decoding stream chunk: %w", err)}
			return
		}
		if chunk.Error != nil {
			ch <- core.StreamEvent{Err: chunk.Error}
			return
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if text := choice.Delta.Content; text != "" {
			ch <- core.StreamEvent{Text: text}
		}
		for _, tc := range choice.Delta.ToolCalls {
			a := acc[tc.Index]
			if a == nil {
				a = &accToolCall{}
				acc[tc.Index] = a
				accOrder = append(accOrder, tc.Index)
			}
			if tc.ID != "" {
				a.id = tc.ID
			}
			if tc.Function.Name != "" {
				a.name += tc.Function.Name
			}
			a.args.WriteString(tc.Function.Arguments)
		}
		if choice.FinishReason == "tool_calls" {
			flushToolCalls()
		}
	}
	if err := sc.Err(); err != nil {
		ch <- core.StreamEvent{Err: fmt.Errorf("reading stream: %w", err)}
	}
}

// chatCompletionRequest is the OpenAI-compatible request body.
type chatCompletionRequest struct {
	Model    string           `json:"model"`
	Messages []messagePayload `json:"messages"`
	Stream   bool             `json:"stream,omitempty"`
	Tools    []toolDefPayload `json:"tools,omitempty"`
}

// toolDefPayload advertises one callable function to the provider.
type toolDefPayload struct {
	Type     string `json:"type"` // always "function"
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"function"`
}

// messagePayload is one message in a chat-completion request. ToolCalls and
// ToolCallID are additive M4 fields for the assistant-request / tool-result
// protocol halves. Content can be a string or a slice of contentPart for vision.
type messagePayload struct {
	Role       string            `json:"role"`
	Content    any               `json:"content"`
	ToolCalls  []toolCallPayload `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

// contentPart represents a single part in a multipart vision message.
type contentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *imageURLPart `json:"image_url,omitempty"`
}

// imageURLPart holds the URL (data: URL or remote link) for a vision image part.
type imageURLPart struct {
	URL string `json:"url"`
}

// isAllowedVisionMIME checks if a given MIME type is supported by vision models.
func isAllowedVisionMIME(mime string) bool {
	switch mime {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

// formatContent converts a core.Message into either a plain string content or
// a multipart content array if valid image attachments are present.
func formatContent(m core.Message) any {
	var imageParts []contentPart
	for _, att := range m.Attachments {
		if att.Type != "image" || !isAllowedVisionMIME(att.MimeType) {
			continue
		}
		var url string
		if len(att.Data) > 0 {
			url = fmt.Sprintf("data:%s;base64,%s", att.MimeType, base64.StdEncoding.EncodeToString(att.Data))
		} else if att.URL != "" {
			url = att.URL
		}
		if url != "" {
			imageParts = append(imageParts, contentPart{
				Type:     "image_url",
				ImageURL: &imageURLPart{URL: url},
			})
		}
	}

	if len(imageParts) == 0 {
		return m.Content
	}

	parts := make([]contentPart, 0, 1+len(imageParts))
	if m.Content != "" {
		parts = append(parts, contentPart{
			Type: "text",
			Text: m.Content,
		})
	}
	parts = append(parts, imageParts...)
	return parts
}

// toolCallPayload is one tool invocation on the wire.
type toolCallPayload struct {
	ID       string   `json:"id,omitempty"`
	Type     string   `json:"type,omitempty"` // "function"
	Function funcSpec `json:"function"`
}

// funcSpec is the callable part of a tool call.
type funcSpec struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// chatCompletionResponse is the non-streaming response body.
type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *apiError `json:"error"`
}

// streamChunk is a single SSE data event.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string           `json:"content"`
			ToolCalls []streamToolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *apiError `json:"error"`
}

// streamToolCall is one incremental tool-call fragment keyed by Index:
// arguments arrive split across several chunks and must be concatenated in
// order before emission (design D5).
type streamToolCall struct {
	Index    int      `json:"index"`
	ID       string   `json:"id,omitempty"`
	Type     string   `json:"type,omitempty"`
	Function funcSpec `json:"function"`
}

// apiError is the OpenAI-compatible error envelope.
type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (e *apiError) Error() string {
	switch {
	case e.Message != "" && e.Type != "":
		return e.Type + ": " + e.Message
	case e.Message != "":
		return e.Message
	case e.Type != "":
		return e.Type
	default:
		return "api error"
	}
}

// httpStatusError converts a non-200 response into an error, preferring the
// structured error envelope when present.
func httpStatusError(resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("chat completion failed with status %d", resp.StatusCode)
	}
	var env struct {
		Error *apiError `json:"error"`
	}
	if json.Unmarshal(body, &env) == nil && env.Error != nil {
		return env.Error
	}
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("chat completion: %s (status %d)", msg, resp.StatusCode)
}
