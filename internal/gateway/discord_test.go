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

func TestDiscordAdapter_Send_Chunking(t *testing.T) {
	var sentMu sync.Mutex
	var sentContents []string
	var authHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		if strings.Contains(r.URL.Path, "/messages") {
			var payload struct {
				Content string `json:"content"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			sentMu.Lock()
			sentContents = append(sentContents, payload.Content)
			sentMu.Unlock()
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "msg-123"})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter := gateway.NewDiscordAdapter(gateway.DiscordConfig{
		Enabled:   true,
		Token:     "my-discord-bot-token",
		Allowlist: []string{"user-123"},
	}, gateway.WithDiscordBaseURL(server.URL))

	ctx := context.Background()

	// 3000 chars exceeds 2000 limit -> 2 chunks (2000 + 1000)
	longMsg := strings.Repeat("d", 3000)
	if err := adapter.Send(ctx, "channel-789", longMsg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if authHeader != "Bot my-discord-bot-token" {
		t.Errorf("Authorization header = %q, want %q", authHeader, "Bot my-discord-bot-token")
	}

	sentMu.Lock()
	defer sentMu.Unlock()
	if len(sentContents) != 2 {
		t.Fatalf("sent %d chunks, want 2", len(sentContents))
	}
	if len([]rune(sentContents[0])) != 2000 || len([]rune(sentContents[1])) != 1000 {
		t.Errorf("chunk sizes = %d, %d, want 2000, 1000", len([]rune(sentContents[0])), len([]rune(sentContents[1])))
	}
}

func TestDiscordAdapter_LifecycleAndIngest(t *testing.T) {
	defer goleak.VerifyNone(t)

	var receivedMu sync.Mutex
	var received []gateway.MessageEvent

	handler := func(ctx context.Context, ev gateway.MessageEvent) error {
		receivedMu.Lock()
		defer receivedMu.Unlock()
		received = append(received, ev)
		return nil
	}

	var pollCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/messages") && r.Method == http.MethodGet {
			if pollCount.Add(1) == 1 {
				// Return 2 messages: 1 from authorized user, 1 from unauthorized user, 1 from bot
				resp := []map[string]any{
					{
						"id":         "msg-1",
						"channel_id": "chan-1",
						"content":    "Authorized discord hello",
						"author": map[string]any{
							"id":       "user-1",
							"username": "bob",
							"bot":      false,
						},
						"timestamp": "2026-03-30T12:00:00Z",
					},
					{
						"id":         "msg-2",
						"channel_id": "chan-1",
						"content":    "Unauthorized discord hello",
						"author": map[string]any{
							"id":       "user-unknown",
							"username": "mallory",
							"bot":      false,
						},
						"timestamp": "2026-03-30T12:00:01Z",
					},
					{
						"id":         "msg-3",
						"channel_id": "chan-1",
						"content":    "Bot loop message",
						"author": map[string]any{
							"id":       "user-1",
							"username": "bob",
							"bot":      true,
						},
						"timestamp": "2026-03-30T12:00:02Z",
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			_ = json.NewEncoder(w).Encode([]any{})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter := gateway.NewDiscordAdapter(gateway.DiscordConfig{
		Enabled:   true,
		Token:     "test-token",
		Allowlist: []string{"user-1"},
	},
		gateway.WithDiscordBaseURL(server.URL),
		gateway.WithDiscordHandler(handler),
		gateway.WithDiscordPollChannels([]string{"chan-1"}),
		gateway.WithDiscordPollInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

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
		t.Fatalf("received %d messages, want 1 (authorized non-bot only)", len(received))
	}
	if received[0].UserID != "user-1" || received[0].Content != "Authorized discord hello" || received[0].ChatID != "chan-1" {
		t.Errorf("received event = %+v, unexpected payload", received[0])
	}
}
