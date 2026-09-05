package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestClient_AuthorizationHeaderInjection(t *testing.T) {
	tests := []struct {
		name       string
		primaryKey string
		extraKeys  []string
		wantAuth   string
	}{
		{
			name:       "non-empty primary key",
			primaryKey: "sk-primary-123",
			extraKeys:  nil,
			wantAuth:   "Bearer sk-primary-123",
		},
		{
			name:       "empty primary key and empty extra keys",
			primaryKey: "",
			extraKeys:  nil,
			wantAuth:   "",
		},
		{
			name:       "empty primary key but non-empty extra key",
			primaryKey: "",
			extraKeys:  []string{"sk-extra-456"},
			wantAuth:   "Bearer sk-extra-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var recordedAuth string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				recordedAuth = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
			}))
			t.Cleanup(server.Close)

			pool := NewCredentialPool(tt.primaryKey, tt.extraKeys)
			client := NewClientWithPool(server.URL, pool)

			resp, err := client.Chat(context.Background(), core.ChatRequest{Model: "gpt-4o"})
			if err != nil {
				t.Fatalf("Chat() unexpected error: %v", err)
			}
			if resp.Content != "ok" {
				t.Fatalf("Chat() got %q, want %q", resp.Content, "ok")
			}
			if recordedAuth != tt.wantAuth {
				t.Errorf("Authorization header = %q, want %q", recordedAuth, tt.wantAuth)
			}
		})
	}
}

func TestClient_Chat_RateLimit_AutoRotationAndRetry(t *testing.T) {
	t.Run("rotates on 429 and succeeds with next key", func(t *testing.T) {
		var receivedAuths []string
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			mu.Lock()
			receivedAuths = append(receivedAuths, auth)
			mu.Unlock()

			if auth == "Bearer sk-key-1" {
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, `{"error":{"message":"Rate limit reached","type":"requests"}}`)
				return
			}
			if auth == "Bearer sk-key-2" {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"choices":[{"message":{"content":"success with key 2"}}]}`)
				return
			}

			w.WriteHeader(http.StatusUnauthorized)
		}))
		t.Cleanup(server.Close)

		pool := NewCredentialPool("sk-key-1", []string{"sk-key-2"})
		client := NewClientWithPool(server.URL, pool)

		resp, err := client.Chat(context.Background(), core.ChatRequest{Model: "gpt-4o"})
		if err != nil {
			t.Fatalf("Chat() should succeed on rotated key, got error: %v", err)
		}
		if resp.Content != "success with key 2" {
			t.Errorf("Chat() content = %q, want %q", resp.Content, "success with key 2")
		}

		mu.Lock()
		defer mu.Unlock()
		if len(receivedAuths) != 2 {
			t.Fatalf("expected 2 requests, got %d: %v", len(receivedAuths), receivedAuths)
		}
		if receivedAuths[0] != "Bearer sk-key-1" || receivedAuths[1] != "Bearer sk-key-2" {
			t.Errorf("auth sequence = %v, want [Bearer sk-key-1, Bearer sk-key-2]", receivedAuths)
		}
	})

	t.Run("fails when all keys in pool return 429", func(t *testing.T) {
		var callCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&callCount, 1)
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"message":"Rate limit reached","type":"requests"}}`)
		}))
		t.Cleanup(server.Close)

		pool := NewCredentialPool("sk-key-1", []string{"sk-key-2", "sk-key-3"})
		client := NewClientWithPool(server.URL, pool)

		_, err := client.Chat(context.Background(), core.ChatRequest{Model: "gpt-4o"})
		if err == nil {
			t.Fatal("Chat() expected error when all keys fail with 429, got nil")
		}

		if atomic.LoadInt32(&callCount) != 3 {
			t.Errorf("callCount = %d, want 3", callCount)
		}
	})

	t.Run("concurrent 429 requests rotate keys cleanly", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "Bearer sk-key-1" {
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, `{"error":{"message":"Rate limit reached","type":"requests"}}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
		}))
		t.Cleanup(server.Close)

		pool := NewCredentialPool("sk-key-1", []string{"sk-key-2", "sk-key-3"})
		client := NewClientWithPool(server.URL, pool)

		const workers = 10
		var wg sync.WaitGroup
		errCh := make(chan error, workers)

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := client.Chat(context.Background(), core.ChatRequest{Model: "gpt-4o"})
				if err != nil {
					errCh <- err
				}
			}()
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			t.Errorf("concurrent Chat() failed: %v", err)
		}
	})
}

func TestClient_Stream_RateLimit_AutoRotationAndRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "Bearer sk-key-1" {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"message":"Rate limit reached","type":"requests"}}`)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"stream ok"}}]}`)
		flusher.Flush()
		fmt.Fprintf(w, "data: %s\n\n", `[DONE]`)
		flusher.Flush()
	}))
	t.Cleanup(server.Close)

	pool := NewCredentialPool("sk-key-1", []string{"sk-key-2"})
	client := NewClientWithPool(server.URL, pool)

	ch, err := client.Stream(context.Background(), core.ChatRequest{Model: "gpt-4o"})
	if err != nil {
		t.Fatalf("Stream() unexpected error: %v", err)
	}

	var events []core.StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) == 0 {
		t.Fatal("expected stream events, got none")
	}
	if events[0].Text != "stream ok" {
		t.Errorf("Stream() got event text %q, want %q", events[0].Text, "stream ok")
	}
}
