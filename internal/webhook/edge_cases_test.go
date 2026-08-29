package webhook_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/webhook"
)

type errSender struct{}

func (s *errSender) Send(ctx context.Context, adapter, target, msg string) error {
	return errors.New("network drop")
}

func TestWebhookServer_EdgeCases(t *testing.T) {
	t.Run("Stop on unstarted server is safe", func(t *testing.T) {
		srv := webhook.NewServer(webhook.Config{})
		if err := srv.Stop(); err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})

	t.Run("Server without brain or repo does not panic on event", func(t *testing.T) {
		srv := webhook.NewServer(webhook.Config{Path: "/webhook"})
		req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString("hello world"))
		rr := httptest.NewRecorder()

		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rr.Code)
		}
	})

	t.Run("Target send error is handled gracefully", func(t *testing.T) {
		cfg := webhook.Config{
			Path: "/webhook",
			Target: &webhook.Target{
				Adapter:   "telegram",
				Recipient: "123",
			},
		}
		srv := webhook.NewServer(cfg, webhook.WithSender(&errSender{}))
		req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(`{"event":"test"}`))
		rr := httptest.NewRecorder()

		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rr.Code)
		}
	})
}
