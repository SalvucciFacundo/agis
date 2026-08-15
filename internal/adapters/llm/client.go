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
	apiKey     string
	httpClient *http.Client
}

// NewClient returns an OpenAI-compatible client targeting baseURL. apiKey may
// be empty for local backends such as Ollama. The caller owns the returned
// client; it holds no background goroutines until a request is made.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
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
	for _, m := range req.Messages {
		payload.Messages = append(payload.Messages, messagePayload{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding chat request: %w", err)
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
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("posting chat completion: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, httpStatusError(resp)
	}
	return resp, nil
}

// streamEvents reads SSE data lines from body and emits one StreamEvent per
// content delta. It returns (closing the channel) on [DONE], a terminal error
// event, a malformed chunk, or EOF.
func (c *Client) streamEvents(body io.Reader, ch chan<- core.StreamEvent) {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

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
		if len(chunk.Choices) > 0 {
			if text := chunk.Choices[0].Delta.Content; text != "" {
				ch <- core.StreamEvent{Text: text}
			}
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
}

// messagePayload is one message in a chat-completion request.
type messagePayload struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Error *apiError `json:"error"`
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
