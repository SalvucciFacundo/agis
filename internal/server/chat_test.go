package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/server"
)

type mockProvider struct {
	mu        sync.Mutex
	events    []core.StreamEvent
	streamErr error
	delay     time.Duration
}

func (m *mockProvider) Chat(context.Context, core.ChatRequest) (core.ChatResponse, error) {
	return core.ChatResponse{}, errors.New("chat not implemented")
}

func (m *mockProvider) Stream(ctx context.Context, _ core.ChatRequest) (<-chan core.StreamEvent, error) {
	m.mu.Lock()
	err := m.streamErr
	events := m.events
	delay := m.delay
	m.mu.Unlock()

	if err != nil {
		return nil, err
	}

	ch := make(chan core.StreamEvent)
	go func() {
		defer close(ch)
		for _, ev := range events {
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return
				}
			}
			select {
			case ch <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (m *mockProvider) Models() []core.ModelInfo {
	return nil
}

type mockRepo struct {
	mu       sync.Mutex
	convs    map[string]*core.Conversation
	messages map[string][]core.Message
	latest   *core.Conversation
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		convs:    make(map[string]*core.Conversation),
		messages: make(map[string][]core.Message),
	}
}

func (r *mockRepo) CreateConversation(_ context.Context, title string) (*core.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := "conv-" + title
	if title == "" {
		id = "conv-default"
	}
	conv := &core.Conversation{ID: id, Title: title}
	r.convs[conv.ID] = conv
	r.latest = conv
	return conv, nil
}

func (r *mockRepo) LatestConversation(_ context.Context) (*core.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latest == nil {
		return nil, core.ErrNotFound
	}
	return r.latest, nil
}

func (r *mockRepo) GetConversation(_ context.Context, id string) (*core.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	conv, ok := r.convs[id]
	if !ok {
		return nil, core.ErrNotFound
	}
	return conv, nil
}

func (r *mockRepo) AppendMessage(_ context.Context, convID string, msg core.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages[convID] = append(r.messages[convID], msg)
	return nil
}

func (r *mockRepo) Messages(_ context.Context, convID string, _ int) ([]core.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.messages[convID], nil
}

func (r *mockRepo) Search(_ context.Context, _ string, _ int) ([]core.SearchResult, error) {
	return nil, nil
}

func (r *mockRepo) SaveObservations(_ context.Context, _ string, _ []core.Observation) error {
	return nil
}

func (r *mockRepo) Observations(_ context.Context, _ int) ([]core.Observation, error) {
	return nil, nil
}

func (r *mockRepo) UpdateConversationSummary(_ context.Context, _, _ string) error {
	return nil
}

func (r *mockRepo) UpsertUserModel(_ context.Context, _ []core.UserModel) error {
	return nil
}

func (r *mockRepo) UserModelRows(_ context.Context, _ int) ([]core.UserModel, error) {
	return nil, nil
}

func (r *mockRepo) ClearUserModel(_ context.Context) error {
	return nil
}

func (r *mockRepo) RecordSessionEvent(_ context.Context, _, _, _ string) error {
	return nil
}

func (r *mockRepo) SaveSkill(_ context.Context, _ core.Skill) error {
	return nil
}

func (r *mockRepo) ListSkills(_ context.Context) ([]core.Skill, error) {
	return nil, nil
}

func (r *mockRepo) RecordSkillUsage(_ context.Context, _ string) error {
	return nil
}

func (r *mockRepo) AppendAudit(_ context.Context, _ core.AuditEntry) error {
	return nil
}

func (r *mockRepo) AuditTail(_ context.Context, _ int) ([]core.AuditEntry, error) {
	return nil, nil
}

func (r *mockRepo) ListConversations(_ context.Context, _, _ int) ([]core.Conversation, error) {
	return nil, nil
}

func (r *mockRepo) RenameConversation(_ context.Context, _, _ string) error {
	return nil
}

func (r *mockRepo) DeleteConversation(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.convs, id)
	return nil
}

func (r *mockRepo) CreateSnapshot(_ context.Context, _ string) (*core.Snapshot, error) {
	return nil, nil
}

func (r *mockRepo) ListSnapshots(_ context.Context, _ string) ([]core.Snapshot, error) {
	return nil, nil
}

func (r *mockRepo) Snapshots(_ context.Context, _ string) ([]core.Snapshot, error) {
	return nil, nil
}


func (r *mockRepo) RestoreSnapshot(_ context.Context, _ string) (*core.Snapshot, error) {
	return nil, nil
}

func (r *mockRepo) Close() error {
	return nil
}


func TestChatCompletions_NonStreaming(t *testing.T) {
	repo := newMockRepo()
	prov := &mockProvider{
		events: []core.StreamEvent{
			{Text: "Hello"},
			{Text: " from "},
			{Text: "AGIS!"},
		},
	}
	brain := core.NewBrain(repo, prov)

	opts := server.Options{
		Host:     "127.0.0.1",
		Port:     8080,
		Brain:    brain,
		Provider: "ollama",
		Model:    "llama3.2",
	}
	srv := server.New(opts)

	reqBody := server.ChatCompletionRequest{
		Model: "llama3.2",
		Messages: []server.ChatCompletionMessage{
			{Role: "user", Content: "Hi there"},
		},
		User: "session-user-42",
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", "custom-session-uuid")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp server.ChatCompletionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Object != "chat.completion" {
		t.Errorf("resp.Object = %q, want 'chat.completion'", resp.Object)
	}
	if !strings.HasPrefix(resp.ID, "chatcmpl-") {
		t.Errorf("resp.ID = %q, want prefix 'chatcmpl-'", resp.ID)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("got %d choices, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hello from AGIS!" {
		t.Errorf("choice content = %q, want 'Hello from AGIS!'", resp.Choices[0].Message.Content)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("choice finish_reason = %q, want 'stop'", resp.Choices[0].FinishReason)
	}

	// Verify conversation session persistence
	repo.mu.Lock()
	msgs := repo.messages["conv-custom-session-uuid"]
	repo.mu.Unlock()

	if len(msgs) != 2 {
		t.Fatalf("persisted %d messages, want 2", len(msgs))
	}
	if msgs[0].Content != "Hi there" {
		t.Errorf("user message = %q, want 'Hi there'", msgs[0].Content)
	}
	if msgs[1].Content != "Hello from AGIS!" {
		t.Errorf("assistant message = %q, want 'Hello from AGIS!'", msgs[1].Content)
	}
}

func TestChatCompletions_MultimodalContent(t *testing.T) {
	repo := newMockRepo()
	prov := &mockProvider{
		events: []core.StreamEvent{
			{Text: "I see an image with text."},
		},
	}
	brain := core.NewBrain(repo, prov)

	opts := server.Options{
		Brain:    brain,
		Provider: "openai",
		Model:    "gpt-4o",
	}
	srv := server.New(opts)

	reqBody := server.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []server.ChatCompletionMessage{
			{
				Role: "user",
				Content: []any{
					map[string]any{"type": "text", "text": "Describe this photo:"},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/photo.jpg"}},
				},
			},
		},
		User: "user-session-123",
	}
	payload, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp server.ChatCompletionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Choices[0].Message.Content != "I see an image with text." {
		t.Errorf("content = %q", resp.Choices[0].Message.Content)
	}

	// Verify conversation session resolution from req.User
	repo.mu.Lock()
	msgs := repo.messages["conv-user-session-123"]
	repo.mu.Unlock()

	if len(msgs) != 2 {
		t.Fatalf("persisted %d messages, want 2", len(msgs))
	}
	if len(msgs[0].Attachments) != 1 {
		t.Fatalf("got %d attachments, want 1", len(msgs[0].Attachments))
	}
	if msgs[0].Attachments[0].URL != "https://example.com/photo.jpg" {
		t.Errorf("attachment URL = %q", msgs[0].Attachments[0].URL)
	}
}

func TestChatCompletions_MissingBrain(t *testing.T) {
	opts := server.Options{
		Brain: nil,
	}
	srv := server.New(opts)

	reqBody := server.ChatCompletionRequest{
		Messages: []server.ChatCompletionMessage{
			{Role: "user", Content: "Hello"},
		},
	}
	payload, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
}

func TestChatCompletions_ProviderError(t *testing.T) {
	repo := newMockRepo()
	prov := &mockProvider{
		streamErr: errors.New("upstream provider failure"),
	}
	brain := core.NewBrain(repo, prov)

	opts := server.Options{
		Brain: brain,
	}
	srv := server.New(opts)

	reqBody := server.ChatCompletionRequest{
		Messages: []server.ChatCompletionMessage{
			{Role: "user", Content: "Hello"},
		},
	}
	payload, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
}

func TestChatCompletions_StreamingSSE(t *testing.T) {
	repo := newMockRepo()
	prov := &mockProvider{
		events: []core.StreamEvent{
			{Text: "Token1 "},
			{Text: "Token2"},
		},
	}
	brain := core.NewBrain(repo, prov)

	opts := server.Options{
		Host:     "127.0.0.1",
		Port:     8080,
		Brain:    brain,
		Provider: "ollama",
		Model:    "llama3.2",
	}
	srv := server.New(opts)

	reqBody := server.ChatCompletionRequest{
		Model: "llama3.2",
		Messages: []server.ChatCompletionMessage{
			{Role: "user", Content: "Stream me"},
		},
		Stream: true,
	}
	payload, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("Content-Type = %q, want containing 'text/event-stream'", contentType)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "data: ") {
		t.Fatalf("body missing SSE data prefix: %s", body)
	}
	if !strings.HasSuffix(strings.TrimSpace(body), "data: [DONE]") {
		t.Errorf("body missing terminal 'data: [DONE]': %s", body)
	}

	// Verify chunks parsing
	lines := strings.Split(body, "\n")
	var tokenAccum strings.Builder
	var sawStopReason bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			continue
		}

		var chunk server.ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			t.Fatalf("failed to unmarshal chunk %q: %v", data, err)
		}
		if chunk.Object != "chat.completion.chunk" {
			t.Errorf("chunk.Object = %q, want 'chat.completion.chunk'", chunk.Object)
		}
		if len(chunk.Choices) > 0 {
			tokenAccum.WriteString(chunk.Choices[0].Delta.Content)
			if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason == "stop" {
				sawStopReason = true
			}
		}
	}

	if tokenAccum.String() != "Token1 Token2" {
		t.Errorf("streamed text = %q, want 'Token1 Token2'", tokenAccum.String())
	}
	if !sawStopReason {
		t.Error("expected finish_reason 'stop' chunk before [DONE]")
	}
}

func TestChatCompletions_ErrorHandling(t *testing.T) {
	repo := newMockRepo()
	prov := &mockProvider{}
	brain := core.NewBrain(repo, prov)

	opts := server.Options{
		Brain: brain,
	}
	srv := server.New(opts)

	tests := []struct {
		name       string
		method     string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "method not allowed GET",
			method:     http.MethodGet,
			body:       "",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "invalid json body",
			method:     http.MethodPost,
			body:       `{"messages": invalid}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request_error",
		},
		{
			name:       "empty messages list",
			method:     http.MethodPost,
			body:       `{"messages": []}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "missing_required_parameter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/v1/chat/completions", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			srv.Handler().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestChatCompletions_ContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	repo := newMockRepo()
	prov := &mockProvider{
		events: []core.StreamEvent{
			{Text: "chunk 1"},
			{Text: "chunk 2"},
		},
		delay: 50 * time.Millisecond,
	}
	brain := core.NewBrain(repo, prov)

	opts := server.Options{
		Brain: brain,
	}
	srv := server.New(opts)

	reqBody := server.ChatCompletionRequest{
		Messages: []server.ChatCompletionMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}
	payload, _ := json.Marshal(reqBody)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Cancel context after a small delay
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	srv.Handler().ServeHTTP(rec, req)
	// Give goroutines time to exit before goleak verification
	time.Sleep(100 * time.Millisecond)
}
