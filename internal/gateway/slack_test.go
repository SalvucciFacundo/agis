package gateway_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/gateway"
)

func computeSlackSignature(secret string, timestamp int64, body []byte) string {
	sigBase := fmt.Sprintf("v0:%d:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sigBase))
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func TestSlackAdapter_Contract(t *testing.T) {
	adapter := gateway.NewSlackAdapter(config.SlackConfig{})
	if adapter.Name() != "slack" {
		t.Errorf("Name() = %q, want 'slack'", adapter.Name())
	}
}

func TestSlackAdapter_URLVerification(t *testing.T) {
	signingSecret := "test-signing-secret"
	adapter := gateway.NewSlackAdapter(config.SlackConfig{
		SigningSecret: signingSecret,
		AllowedUsers:  []string{"U123"},
	})

	body := []byte(`{"type":"url_verification","challenge":"test_challenge_abc_123"}`)
	timestamp := time.Now().Unix()
	sig := computeSlackSignature(signingSecret, timestamp, body)

	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-Request-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-Slack-Signature", sig)

	rec := httptest.NewRecorder()
	adapter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Challenge != "test_challenge_abc_123" {
		t.Errorf("challenge = %q, want 'test_challenge_abc_123'", resp.Challenge)
	}
}

func TestSlackAdapter_SignatureVerification(t *testing.T) {
	signingSecret := "valid-secret"
	adapter := gateway.NewSlackAdapter(config.SlackConfig{
		SigningSecret: signingSecret,
	})

	body := []byte(`{"type":"url_verification","challenge":"test"}`)
	timestamp := time.Now().Unix()

	tests := []struct {
		name       string
		timestamp  string
		sig        string
		wantStatus int
	}{
		{
			name:       "valid signature",
			timestamp:  fmt.Sprintf("%d", timestamp),
			sig:        computeSlackSignature(signingSecret, timestamp, body),
			wantStatus: http.StatusOK,
		},
		{
			name:       "forged signature",
			timestamp:  fmt.Sprintf("%d", timestamp),
			sig:        computeSlackSignature("wrong-secret", timestamp, body),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "missing signature header",
			timestamp:  fmt.Sprintf("%d", timestamp),
			sig:        "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "expired timestamp (>300s old)",
			timestamp:  fmt.Sprintf("%d", timestamp-400),
			sig:        computeSlackSignature(signingSecret, timestamp-400, body),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "future timestamp (>300s ahead)",
			timestamp:  fmt.Sprintf("%d", timestamp+400),
			sig:        computeSlackSignature(signingSecret, timestamp+400, body),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid non-numeric timestamp",
			timestamp:  "not-a-number",
			sig:        "v0=abc",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if tt.timestamp != "" {
				req.Header.Set("X-Slack-Request-Timestamp", tt.timestamp)
			}
			if tt.sig != "" {
				req.Header.Set("X-Slack-Signature", tt.sig)
			}

			rec := httptest.NewRecorder()
			adapter.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestSlackAdapter_EventCallback_AuthorizedUser(t *testing.T) {
	signingSecret := "slack-secret-123"
	var receivedEvent gateway.MessageEvent
	var handlerCalled bool
	var mu sync.Mutex

	adapter := gateway.NewSlackAdapter(
		config.SlackConfig{
			SigningSecret: signingSecret,
			AllowedUsers:  []string{"U12345"},
		},
		gateway.WithSlackHandler(func(_ context.Context, ev gateway.MessageEvent) error {
			mu.Lock()
			defer mu.Unlock()
			receivedEvent = ev
			handlerCalled = true
			return nil
		}),
	)

	body := []byte(`{
		"type": "event_callback",
		"event": {
			"type": "message",
			"user": "U12345",
			"channel": "C67890",
			"text": "Summarize release notes",
			"ts": "1700000000.000100"
		}
	}`)
	timestamp := time.Now().Unix()
	sig := computeSlackSignature(signingSecret, timestamp, body)

	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-Request-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-Slack-Signature", sig)

	rec := httptest.NewRecorder()
	adapter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	mu.Lock()
	defer mu.Unlock()
	if !handlerCalled {
		t.Fatal("handler was not called for authorized user")
	}
	if receivedEvent.Adapter != "slack" {
		t.Errorf("Adapter = %q, want 'slack'", receivedEvent.Adapter)
	}
	if receivedEvent.UserID != "U12345" {
		t.Errorf("UserID = %q, want 'U12345'", receivedEvent.UserID)
	}
	if receivedEvent.ChatID != "C67890" {
		t.Errorf("ChatID = %q, want 'C67890'", receivedEvent.ChatID)
	}
	if receivedEvent.Content != "Summarize release notes" {
		t.Errorf("Content = %q, want 'Summarize release notes'", receivedEvent.Content)
	}
}

func TestSlackAdapter_EventCallback_UnauthorizedAndBotFiltering(t *testing.T) {
	signingSecret := "slack-secret-123"
	var handlerCalled bool

	adapter := gateway.NewSlackAdapter(
		config.SlackConfig{
			SigningSecret: signingSecret,
			AllowedUsers:  []string{"U12345"},
		},
		gateway.WithSlackHandler(func(_ context.Context, _ gateway.MessageEvent) error {
			handlerCalled = true
			return nil
		}),
	)

	tests := []struct {
		name string
		body string
	}{
		{
			name: "unauthorized user",
			body: `{
				"type": "event_callback",
				"event": {
					"type": "message",
					"user": "U99999",
					"channel": "C67890",
					"text": "Unauthorized message"
				}
			}`,
		},
		{
			name: "bot message with bot_id",
			body: `{
				"type": "event_callback",
				"event": {
					"type": "message",
					"user": "U12345",
					"bot_id": "B12345",
					"channel": "C67890",
					"text": "Bot message"
				}
			}`,
		},
		{
			name: "bot message with subtype",
			body: `{
				"type": "event_callback",
				"event": {
					"type": "message",
					"user": "U12345",
					"subtype": "bot_message",
					"channel": "C67890",
					"text": "Bot subtype message"
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled = false
			timestamp := time.Now().Unix()
			bodyBytes := []byte(tt.body)
			sig := computeSlackSignature(signingSecret, timestamp, bodyBytes)

			req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Slack-Request-Timestamp", fmt.Sprintf("%d", timestamp))
			req.Header.Set("X-Slack-Signature", sig)

			rec := httptest.NewRecorder()
			adapter.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if handlerCalled {
				t.Errorf("handler should NOT have been called for %s", tt.name)
			}
		})
	}
}

func TestSlackAdapter_Send_ChunkingAndMockAPI(t *testing.T) {
	var postedMessages []struct {
		Channel string `json:"channel"`
		Text    string `json:"text"`
	}
	var authHeaders []string
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/chat.postMessage" {
			http.NotFound(w, r)
			return
		}

		authHeaders = append(authHeaders, r.Header.Get("Authorization"))

		var payload struct {
			Channel string `json:"channel"`
			Text    string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		postedMessages = append(postedMessages, payload)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()

	botToken := "xoxb-mock-token-12345"
	adapter := gateway.NewSlackAdapter(
		config.SlackConfig{
			BotToken: botToken,
		},
		gateway.WithSlackBaseURL(server.URL),
	)

	// 1. Send single message under 4000 runes
	shortMsg := "Hello Slack"
	if err := adapter.Send(context.Background(), "C67890", shortMsg); err != nil {
		t.Fatalf("Send(shortMsg) error = %v", err)
	}

	mu.Lock()
	if len(postedMessages) != 1 {
		t.Fatalf("postedMessages count = %d, want 1", len(postedMessages))
	}
	if postedMessages[0].Channel != "C67890" || postedMessages[0].Text != shortMsg {
		t.Errorf("posted message = %+v, want channel C67890 text %q", postedMessages[0], shortMsg)
	}
	if authHeaders[0] != "Bearer "+botToken {
		t.Errorf("Authorization header = %q, want 'Bearer %s'", authHeaders[0], botToken)
	}
	mu.Unlock()

	// 2. Send long message exceeding 4000 runes (7500 runes -> 4000 + 3500)
	longMsg := strings.Repeat("A", 4000) + strings.Repeat("B", 3500)
	if err := adapter.Send(context.Background(), "C67890", longMsg); err != nil {
		t.Fatalf("Send(longMsg) error = %v", err)
	}

	mu.Lock()
	if len(postedMessages) != 3 {
		t.Fatalf("postedMessages count = %d, want 3 (1 previous + 2 chunks)", len(postedMessages))
	}
	if len([]rune(postedMessages[1].Text)) != 4000 {
		t.Errorf("chunk 1 length = %d runes, want 4000", len([]rune(postedMessages[1].Text)))
	}
	if len([]rune(postedMessages[2].Text)) != 3500 {
		t.Errorf("chunk 2 length = %d runes, want 3500", len([]rune(postedMessages[2].Text)))
	}
	mu.Unlock()
}

func TestSlackAdapter_Lifecycle(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Pick a free local port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on port: %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	adapter := gateway.NewSlackAdapter(config.SlackConfig{
		Enabled:    true,
		ListenAddr: addr,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Double start should return error
	if err := adapter.Start(ctx); err == nil {
		t.Error("Start() on running adapter expected error, got nil")
	}

	// Stop adapter
	if err := adapter.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Double stop should be a safe no-op
	if err := adapter.Stop(); err != nil {
		t.Errorf("Stop() second time error = %v", err)
	}

	// Send after stop should return ErrAdapterClosed
	if err := adapter.Send(context.Background(), "C123", "msg"); err != gateway.ErrAdapterClosed {
		t.Errorf("Send() after Stop = %v, want ErrAdapterClosed", err)
	}
}
