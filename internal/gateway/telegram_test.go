package gateway_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/SalvucciFacundo/agis/internal/gateway"
)

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		limit     int
		wantCount int
	}{
		{
			name:      "empty string",
			input:     "",
			limit:     4096,
			wantCount: 0,
		},
		{
			name:      "short string under limit",
			input:     "Hello world",
			limit:     4096,
			wantCount: 1,
		},
		{
			name:      "exact limit",
			input:     strings.Repeat("a", 4096),
			limit:     4096,
			wantCount: 1,
		},
		{
			name:      "exceeds limit 5000 chars split into 2 chunks",
			input:     strings.Repeat("a", 5000),
			limit:     4096,
			wantCount: 2,
		},
		{
			name:      "multibyte unicode strings split without splitting runes",
			input:     strings.Repeat("🚀", 3000), // each rocket is 1 rune (4 bytes)
			limit:     2000,
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := gateway.SplitMessage(tt.input, tt.limit)
			if len(chunks) != tt.wantCount {
				t.Fatalf("SplitMessage() count = %d, want %d", len(chunks), tt.wantCount)
			}
			for i, ch := range chunks {
				if len([]rune(ch)) > tt.limit {
					t.Errorf("chunk %d rune length %d exceeds limit %d", i, len([]rune(ch)), tt.limit)
				}
			}
			if tt.input != "" && strings.Join(chunks, "") != tt.input {
				t.Errorf("joined chunks do not match input")
			}
		})
	}
}

func TestTelegramAdapter_LifecycleAndPolling(t *testing.T) {
	defer goleak.VerifyNone(t)

	var updateCount atomic.Int32
	var lastOffset atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "getUpdates") {
			if updateCount.Add(1) == 1 {
				// First poll: return 2 updates (one authorized, one unauthorized)
				resp := map[string]any{
					"ok": true,
					"result": []map[string]any{
						{
							"update_id": 100,
							"message": map[string]any{
								"message_id": 1,
								"from":       map[string]any{"id": 12345, "username": "alice"},
								"chat":       map[string]any{"id": 12345},
								"text":       "Hello from authorized user",
								"date":       time.Now().Unix(),
							},
						},
						{
							"update_id": 101,
							"message": map[string]any{
								"message_id": 2,
								"from":       map[string]any{"id": 99999, "username": "intruder"},
								"chat":       map[string]any{"id": 99999},
								"text":       "Hello from unauthorized user",
								"date":       time.Now().Unix(),
							},
						},
					},
				}
				lastOffset.Store(102)
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			// Subsequent polls: return empty
			resp := map[string]any{
				"ok":     true,
				"result": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var receivedMu sync.Mutex
	var received []gateway.MessageEvent

	handler := func(ctx context.Context, ev gateway.MessageEvent) error {
		receivedMu.Lock()
		defer receivedMu.Unlock()
		received = append(received, ev)
		return nil
	}

	adapter := gateway.NewTelegramAdapter(gateway.TelegramConfig{
		Enabled:   true,
		Token:     "test-token",
		Allowlist: []string{"12345"},
	},
		gateway.WithTelegramBaseURL(server.URL),
		gateway.WithTelegramHandler(handler),
		gateway.WithTelegramPollInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for handler to receive message
	deadline := time.Now().Add(1 * time.Second)
	for {
		receivedMu.Lock()
		count := len(received)
		receivedMu.Unlock()
		if count >= 1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := adapter.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	receivedMu.Lock()
	defer receivedMu.Unlock()
	if len(received) != 1 {
		t.Fatalf("received %d messages, want exactly 1 (authorized only)", len(received))
	}
	if received[0].UserID != "12345" || received[0].Content != "Hello from authorized user" {
		t.Errorf("received event = %+v, unexpected payload", received[0])
	}
}

func TestTelegramAdapter_Send_Chunking(t *testing.T) {
	var sentMu sync.Mutex
	var sentTexts []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "sendMessage") {
			var payload struct {
				ChatID string `json:"chat_id"`
				Text   string `json:"text"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			sentMu.Lock()
			sentTexts = append(sentTexts, payload.Text)
			sentMu.Unlock()
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter := gateway.NewTelegramAdapter(gateway.TelegramConfig{
		Enabled:   true,
		Token:     "test-token",
		Allowlist: []string{"12345"},
	}, gateway.WithTelegramBaseURL(server.URL))

	ctx := context.Background()

	// Send message of 5000 chars (exceeds 4096)
	longMsg := strings.Repeat("a", 5000)
	if err := adapter.Send(ctx, "12345", longMsg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	sentMu.Lock()
	defer sentMu.Unlock()
	if len(sentTexts) != 2 {
		t.Fatalf("sent %d chunks, want 2", len(sentTexts))
	}
	if len([]rune(sentTexts[0])) != 4096 || len([]rune(sentTexts[1])) != 904 {
		t.Errorf("chunk sizes = %d, %d, want 4096, 904", len([]rune(sentTexts[0])), len([]rune(sentTexts[1])))
	}
}
