package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// newTestOpenAI returns an OpenAI adapter whose client targets baseURL.
func newTestOpenAI(baseURL string) *OpenAI {
	return &OpenAI{
		client: NewClient(baseURL, "test-key"),
		model:  "gpt-4o-mini",
	}
}

// newTestOllama returns an Ollama adapter whose client targets baseURL.
func newTestOllama(baseURL string) *Ollama {
	return &Ollama{
		client: NewClient(baseURL, ""),
		model:  "llama3.2",
	}
}

// newSSEServer streams the given data events as an OpenAI-compatible SSE
// stream, flushing after each event.
func newSSEServer(t *testing.T, events ...string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		for _, ev := range events {
			fmt.Fprintf(w, "data: %s\n\n", ev)
			flusher.Flush()
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// recordedRequest holds the parts of a chat request that tests assert on.
type recordedRequest struct {
	path     string
	auth     string
	model    string
	messages []messagePayload
}

// newRecordingServer serves a single JSON body and records the request it
// receives into a buffered channel.
func newRecordingServer(t *testing.T, body string) (*httptest.Server, <-chan recordedRequest) {
	t.Helper()
	rec := make(chan recordedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatCompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		rec <- recordedRequest{
			path:     r.URL.Path,
			auth:     r.Header.Get("Authorization"),
			model:    req.Model,
			messages: req.Messages,
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	return server, rec
}

func TestStream_TokenOrder(t *testing.T) {
	server := newSSEServer(t,
		`{"choices":[{"delta":{"content":"Hel"}}]}`,
		`{"choices":[{"delta":{"content":"lo"}}]}`,
		`[DONE]`,
	)
	provider := newTestOpenAI(server.URL)

	ch, err := provider.Stream(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var got strings.Builder
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("Stream() event error = %v, want none", ev.Err)
		}
		got.WriteString(ev.Text)
	}
	if got.String() != "Hello" {
		t.Errorf("streamed text = %q, want %q", got.String(), "Hello")
	}
}

func TestStream_MidStreamError(t *testing.T) {
	server := newSSEServer(t,
		`{"choices":[{"delta":{"content":"Hel"}}]}`,
		`{"error":{"message":"mid-stream boom","type":"server_error"}}`,
	)
	provider := newTestOpenAI(server.URL)

	ch, err := provider.Stream(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	events := collect(ch)
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (text then error)", len(events))
	}
	if events[0].Text != "Hel" {
		t.Errorf("events[0].Text = %q, want %q", events[0].Text, "Hel")
	}
	if events[1].Err == nil {
		t.Fatal("events[1].Err = nil, want mid-stream error")
	}
	if !strings.Contains(events[1].Err.Error(), "mid-stream boom") {
		t.Errorf("events[1].Err = %v, want to contain %q", events[1].Err, "mid-stream boom")
	}
}

func TestStream_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"invalid api key"}}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := newTestOpenAI(server.URL)
	_, err := provider.Stream(context.Background(), core.ChatRequest{})
	if err == nil {
		t.Fatal("Stream() error = nil, want non-200 error")
	}
	if !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("Stream() error = %v, want to contain %q", err, "invalid api key")
	}
}

func TestChat(t *testing.T) {
	server, rec := newRecordingServer(t, `{"choices":[{"message":{"content":"hello"}}]}`)
	provider := newTestOpenAI(server.URL)

	resp, err := provider.Chat(context.Background(), core.ChatRequest{
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.Content != "hello" {
		t.Errorf("Chat() content = %q, want %q", resp.Content, "hello")
	}

	got := <-rec
	if got.path != "/chat/completions" {
		t.Errorf("request path = %q, want /chat/completions", got.path)
	}
	if got.model != "gpt-4o-mini" {
		t.Errorf("request model = %q, want %q", got.model, "gpt-4o-mini")
	}
	if len(got.messages) != 1 || got.messages[0].Role != "user" || got.messages[0].Content != "hi" {
		t.Errorf("request messages = %+v, want single user/hi", got.messages)
	}
	if got.auth != "Bearer test-key" {
		t.Errorf("Authorization = %q, want %q", got.auth, "Bearer test-key")
	}
}

func TestOllama_Chat(t *testing.T) {
	server, rec := newRecordingServer(t, `{"choices":[{"message":{"content":"hi back"}}]}`)
	provider := newTestOllama(server.URL)

	resp, err := provider.Chat(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.Content != "hi back" {
		t.Errorf("Chat() content = %q, want %q", resp.Content, "hi back")
	}

	got := <-rec
	if got.model != "llama3.2" {
		t.Errorf("request model = %q, want %q", got.model, "llama3.2")
	}
	if got.auth != "" {
		t.Errorf("Authorization = %q, want empty (local backend)", got.auth)
	}
}

func TestModels(t *testing.T) {
	tests := []struct {
		name     string
		provider core.Provider
		wantID   string
		wantProv string
	}{
		{
			name:     "openai",
			provider: NewOpenAI(config.LLMConfig{Model: "gpt-4o-mini"}),
			wantID:   "gpt-4o-mini",
			wantProv: "openai",
		},
		{
			name:     "ollama",
			provider: NewOllama(config.LLMConfig{Model: "llama3.2"}),
			wantID:   "llama3.2",
			wantProv: "ollama",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			models := tt.provider.Models()
			if len(models) != 1 {
				t.Fatalf("Models() = %d entries, want 1", len(models))
			}
			if models[0].ID != tt.wantID {
				t.Errorf("Models()[0].ID = %q, want %q", models[0].ID, tt.wantID)
			}
			if models[0].Provider != tt.wantProv {
				t.Errorf("Models()[0].Provider = %q, want %q", models[0].Provider, tt.wantProv)
			}
		})
	}
}

func TestNewProvider_SelectsAdapter(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     string
	}{
		{name: "openai", provider: "openai", want: "*llm.OpenAI"},
		{name: "ollama", provider: "ollama", want: "*llm.Ollama"},
		{name: "unknown defaults to openai", provider: "custom", want: "*llm.OpenAI"},
		{name: "empty defaults to openai", provider: "", want: "*llm.OpenAI"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider(config.LLMConfig{Provider: tt.provider, Model: "m"})
			if got := fmt.Sprintf("%T", p); got != tt.want {
				t.Errorf("NewProvider(%q) type = %s, want %s", tt.provider, got, tt.want)
			}
		})
	}
}

func TestAdapterBaseURLs(t *testing.T) {
	openAI := NewOpenAI(config.LLMConfig{})
	if openAI.client.baseURL != openAIBaseURL {
		t.Errorf("OpenAI baseURL = %q, want %q", openAI.client.baseURL, openAIBaseURL)
	}
	ollama := NewOllama(config.LLMConfig{})
	if ollama.client.baseURL != ollamaBaseURL {
		t.Errorf("Ollama baseURL = %q, want %q", ollama.client.baseURL, ollamaBaseURL)
	}
}

// collect drains a stream channel, returning every event in order.
func collect(ch <-chan core.StreamEvent) []core.StreamEvent {
	var events []core.StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}
