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

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

const (
	anthropicVersion     = "2023-06-01"
	anthropicDefaultMaxTokens = 4096
	providerAnthropic    = "anthropic"
)

// Anthropic implements the core.Provider interface for Anthropic Messages API.
type Anthropic struct {
	client     *http.Client
	baseURL    string
	pool       *CredentialPool
	model      string
}

var _ core.Provider = (*Anthropic)(nil)

// NewAnthropic returns an Anthropic-backed Provider configured from cfg.
func NewAnthropic(cfg config.LLMConfig) *Anthropic {
	baseURL := ResolveBaseURL(providerAnthropic, cfg.BaseURL)
	return &Anthropic{
		client:     &http.Client{},
		baseURL:    baseURL,
		pool:       NewCredentialPool(cfg.APIKey, cfg.APIKeys),
		model:      cfg.Model,
	}
}

// Models returns the static model list for Anthropic.
func (a *Anthropic) Models() []core.ModelInfo {
	return staticModels(a.model, providerAnthropic)
}

// Chat executes a non-streaming turn using Anthropic Messages API (/v1/messages).
func (a *Anthropic) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	resp, err := a.doMessages(ctx, req, false)
	if err != nil {
		return core.ChatResponse{}, err
	}
	defer resp.Body.Close()

	var out anthropicMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return core.ChatResponse{}, fmt.Errorf("decoding anthropic response: %w", err)
	}
	if out.Error != nil {
		return core.ChatResponse{}, out.Error
	}

	var sb strings.Builder
	for _, block := range out.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}

	return core.ChatResponse{Content: sb.String()}, nil
}

// Stream executes a streaming turn using Anthropic Messages SSE stream.
func (a *Anthropic) Stream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	resp, err := a.doMessages(ctx, req, true)
	if err != nil {
		return nil, err
	}

	ch := make(chan core.StreamEvent)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		stop := context.AfterFunc(ctx, func() { resp.Body.Close() })
		defer stop()

		a.streamEvents(resp.Body, ch)
	}()

	return ch, nil
}

func (a *Anthropic) doMessages(ctx context.Context, req core.ChatRequest, stream bool) (*http.Response, error) {
	req = ensureModel(req, a.model)
	payload, err := a.buildPayload(req, stream)
	if err != nil {
		return nil, fmt.Errorf("building anthropic payload: %w", err)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding anthropic request: %w", err)
	}

	maxAttempts := a.pool.Len()
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
			a.baseURL+"/v1/messages",
			bytes.NewReader(body),
		)
		if err != nil {
			return nil, fmt.Errorf("building anthropic http request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("anthropic-version", anthropicVersion)

		keyUsed := a.pool.CurrentKey()
		if keyUsed != "" {
			httpReq.Header.Set("x-api-key", keyUsed)
		}

		resp, err := a.client.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("posting anthropic messages: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			if _, ok := a.pool.RotateKey(keyUsed); ok && attempt < maxAttempts-1 {
				resp.Body.Close()
				continue
			}
		}

		if resp.StatusCode != http.StatusOK {
			defer resp.Body.Close()
			return nil, a.httpError(resp)
		}
		return resp, nil
	}

	return nil, errors.New("anthropic: all API keys exhausted")
}

func (a *Anthropic) buildPayload(req core.ChatRequest, stream bool) (anthropicMessageRequest, error) {
	var systemParts []string
	messages := make([]anthropicMessage, 0, len(req.Messages))

	for _, m := range req.Messages {
		if m.Role == core.RoleSystem {
			if m.Content != "" {
				systemParts = append(systemParts, m.Content)
			}
			continue
		}

		role := "user"
		if m.Role == core.RoleAssistant {
			role = "assistant"
		}

		var contentBlocks []any

		// Check attachments for vision
		for _, att := range m.Attachments {
			if att.Type == "image" && isAllowedVisionMIME(att.MimeType) {
				if len(att.Data) > 0 {
					contentBlocks = append(contentBlocks, map[string]any{
						"type": "image",
						"source": map[string]any{
							"type":       "base64",
							"media_type": att.MimeType,
							"data":       base64.StdEncoding.EncodeToString(att.Data),
						},
					})
				}
			}
		}

		if m.Content != "" {
			if len(contentBlocks) > 0 {
				contentBlocks = append(contentBlocks, map[string]any{
					"type": "text",
					"text": m.Content,
				})
			}
		}

		for _, tc := range m.ToolCalls {
			var input map[string]any
			if tc.Arguments != "" {
				_ = json.Unmarshal([]byte(tc.Arguments), &input)
			}
			if input == nil {
				input = map[string]any{}
			}
			contentBlocks = append(contentBlocks, map[string]any{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Name,
				"input": input,
			})
		}

		if m.ToolCallID != "" {
			role = "user"
			contentBlocks = append(contentBlocks, map[string]any{
				"type":         "tool_result",
				"tool_use_id":  m.ToolCallID,
				"content":      m.Content,
			})
		}

		var finalContent any
		if len(contentBlocks) == 0 {
			finalContent = m.Content
		} else if len(contentBlocks) == 1 && m.Content != "" && len(m.Attachments) == 0 && len(m.ToolCalls) == 0 && m.ToolCallID == "" {
			finalContent = m.Content
		} else {
			finalContent = contentBlocks
		}

		messages = append(messages, anthropicMessage{
			Role:    role,
			Content: finalContent,
		})
	}

	payload := anthropicMessageRequest{
		Model:     req.Model,
		MaxTokens: anthropicDefaultMaxTokens,
		Messages:  messages,
		Stream:    stream,
	}

	if len(systemParts) > 0 {
		payload.System = strings.Join(systemParts, "\n\n")
	}

	if len(req.Tools) > 0 {
		payload.Tools = make([]anthropicTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			payload.Tools = append(payload.Tools, anthropicTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			})
		}
	}

	return payload, nil
}

func (a *Anthropic) streamEvents(body io.Reader, ch chan<- core.StreamEvent) {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	type activeTool struct {
		id   string
		name string
		args strings.Builder
	}
	activeTools := map[int]*activeTool{}

	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}

		var event struct {
			Type         string `json:"type"`
			Index        int    `json:"index"`
			ContentBlock struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
				Text string `json:"text"`
			} `json:"content_block"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
			Error *anthropicAPIError `json:"error"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			ch <- core.StreamEvent{Err: fmt.Errorf("decoding anthropic stream event: %w", err)}
			return
		}

		if event.Error != nil {
			ch <- core.StreamEvent{Err: event.Error}
			return
		}

		switch event.Type {
		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				activeTools[event.Index] = &activeTool{
					id:   event.ContentBlock.ID,
					name: event.ContentBlock.Name,
				}
			}
		case "content_block_delta":
			if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
				ch <- core.StreamEvent{Text: event.Delta.Text}
			} else if event.Delta.Type == "input_json_delta" && event.Delta.PartialJSON != "" {
				if tool, ok := activeTools[event.Index]; ok {
					tool.args.WriteString(event.Delta.PartialJSON)
				}
			}
		case "content_block_stop":
			if tool, ok := activeTools[event.Index]; ok {
				ch <- core.StreamEvent{
					ToolCall: &core.ToolCall{
						ID:        tool.id,
						Name:      tool.name,
						Arguments: tool.args.String(),
					},
				}
				delete(activeTools, event.Index)
			}
		case "message_stop":
			return
		}
	}

	if err := sc.Err(); err != nil {
		ch <- core.StreamEvent{Err: fmt.Errorf("reading anthropic stream: %w", err)}
	}
}

func (a *Anthropic) httpError(resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("anthropic request failed with status %d", resp.StatusCode)
	}

	var env struct {
		Error *anthropicAPIError `json:"error"`
	}
	if json.Unmarshal(body, &env) == nil && env.Error != nil {
		return env.Error
	}

	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("anthropic: %s (status %d)", msg, resp.StatusCode)
}

type anthropicMessageRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
	Stream    bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicMessageResponse struct {
	ID      string                  `json:"id"`
	Type    string                  `json:"type"`
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
	Error   *anthropicAPIError      `json:"error"`
}

type anthropicContentBlock struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type anthropicAPIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e *anthropicAPIError) Error() string {
	if e.Type != "" && e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Type, e.Message)
	}
	if e.Message != "" {
		return e.Message
	}
	return "anthropic error"
}
