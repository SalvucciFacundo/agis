package webhook_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"github.com/SalvucciFacundo/agis/internal/webhook"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func computeHMACSHA256(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

type mockBrain struct {
	mu           sync.Mutex
	activeConvID string
	stepCalls    []string
	stepErr      error
	repo         core.Repository
	replyText    string
}

func (b *mockBrain) SetActiveConversation(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.activeConvID = id
}

func (b *mockBrain) Step(ctx context.Context, input string) error {
	b.mu.Lock()
	b.stepCalls = append(b.stepCalls, input)
	convID := b.activeConvID
	err := b.stepErr
	b.mu.Unlock()

	if err != nil {
		return err
	}

	if b.repo != nil && convID != "" {
		_ = b.repo.AppendMessage(ctx, convID, core.Message{Role: core.RoleUser, Content: input})
		reply := b.replyText
		if reply == "" {
			reply = "Processed alert successfully"
		}
		_ = b.repo.AppendMessage(ctx, convID, core.Message{Role: core.RoleAssistant, Content: reply})
	}
	return nil
}

type fakeSender struct {
	mu       sync.Mutex
	sentMsgs []sentMessage
}

type sentMessage struct {
	Adapter   string
	Recipient string
	Msg       string
}

func (s *fakeSender) Send(ctx context.Context, adapter, recipient, msg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sentMsgs = append(s.sentMsgs, sentMessage{Adapter: adapter, Recipient: recipient, Msg: msg})
	return nil
}

func TestVerifySignature(t *testing.T) {
	secret := "my-secret-key"
	payload := []byte(`{"event":"test","status":"ok"}`)
	validSig := computeHMACSHA256(secret, payload)

	tests := []struct {
		name      string
		secret    string
		body      []byte
		headerSig string
		wantValid bool
	}{
		{
			name:      "valid signature with sha256= prefix",
			secret:    secret,
			body:      payload,
			headerSig: "sha256=" + validSig,
			wantValid: true,
		},
		{
			name:      "valid signature without prefix",
			secret:    secret,
			body:      payload,
			headerSig: validSig,
			wantValid: true,
		},
		{
			name:      "tampered body",
			secret:    secret,
			body:      []byte(`{"event":"tampered"}`),
			headerSig: "sha256=" + validSig,
			wantValid: false,
		},
		{
			name:      "wrong signature",
			secret:    secret,
			body:      payload,
			headerSig: "sha256=abcdef1234567890",
			wantValid: false,
		},
		{
			name:      "empty secret permits any request",
			secret:    "",
			body:      payload,
			headerSig: "",
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := webhook.VerifySignature(tt.secret, tt.body, tt.headerSig)
			if got != tt.wantValid {
				t.Errorf("VerifySignature() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestWebhookServer_HTTPHandler(t *testing.T) {
	secret := "webhook-secret"
	cfg := webhook.Config{
		Host:             "127.0.0.1",
		Port:             8080,
		Path:             "/webhook",
		Secret:           secret,
		DefaultSessionID: "webhook-default",
		Target: &webhook.Target{
			Adapter:   "telegram",
			Recipient: "123456",
		},
	}

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	repo, err := memory.NewRepository(ctx, dbPath)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	defer repo.Close()

	brain := &mockBrain{
		repo:      repo,
		replyText: "Processed alert successfully",
	}
	sender := &fakeSender{}

	srv := webhook.NewServer(cfg,
		webhook.WithBrain(brain),
		webhook.WithRepo(repo),
		webhook.WithSender(sender),
	)

	t.Run("GET method returns 405 Method Not Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
		rr := httptest.NewRecorder()

		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("wrong path returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/other-path", bytes.NewBufferString("{}"))
		rr := httptest.NewRecorder()

		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("POST without valid signature returns 401 Unauthorized", func(t *testing.T) {
		body := []byte(`{"alert":"cpu_spike"}`)
		req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
		req.Header.Set("X-Hub-Signature-256", "sha256=invalid-signature")
		rr := httptest.NewRecorder()

		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("POST with valid signature executes Brain and forwards to Target", func(t *testing.T) {
		payload := map[string]string{
			"event":  "alert",
			"server": "web-01",
		}
		body, _ := json.Marshal(payload)
		sig := computeHMACSHA256(secret, body)

		req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
		req.Header.Set("X-Hub-Signature-256", "sha256="+sig)
		rr := httptest.NewRecorder()

		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", rr.Code, http.StatusOK, rr.Body.String())
		}

		// Check if target sender received the notification
		sender.mu.Lock()
		defer sender.mu.Unlock()
		if len(sender.sentMsgs) == 0 {
			t.Fatal("expected target notification to be sent, but got 0 sent messages")
		}
		lastMsg := sender.sentMsgs[len(sender.sentMsgs)-1]
		if lastMsg.Adapter != "telegram" || lastMsg.Recipient != "123456" {
			t.Errorf("unexpected target delivery: %+v", lastMsg)
		}
		if lastMsg.Msg != "Processed alert successfully" {
			t.Errorf("sent message = %q, want 'Processed alert successfully'", lastMsg.Msg)
		}
	})
}

func TestWebhookServer_LifecycleGracefulShutdown(t *testing.T) {
	cfg := webhook.Config{
		Host:             "127.0.0.1",
		Port:             0, // dynamic port
		Path:             "/webhook",
		DefaultSessionID: "webhook-test",
	}

	srv := webhook.NewServer(cfg)
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	// Give server time to bind and run
	time.Sleep(50 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled && err != http.ErrServerClosed {
			t.Errorf("unexpected Start() error on shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down within 2s")
	}
}
