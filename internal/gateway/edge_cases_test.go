package gateway_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/goleak"

	"github.com/SalvucciFacundo/agis/internal/gateway"
	"github.com/SalvucciFacundo/agis/internal/memory"
)

func TestGateway_EdgeCases(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("SplitMessage with zero or negative limit", func(t *testing.T) {
		chunks := gateway.SplitMessage("hello world", 0)
		if len(chunks) != 1 || chunks[0] != "hello world" {
			t.Errorf("SplitMessage with 0 limit = %v", chunks)
		}
	})

	t.Run("TelegramAdapter Send empty message is no-op", func(t *testing.T) {
		adapter := gateway.NewTelegramAdapter(gateway.TelegramConfig{
			Enabled:   true,
			Token:     "tok",
			Allowlist: []string{"123"},
		})
		if err := adapter.Send(context.Background(), "123", "   "); err != nil {
			t.Errorf("Send empty string error = %v", err)
		}
	})

	t.Run("DiscordAdapter Send empty message is no-op", func(t *testing.T) {
		adapter := gateway.NewDiscordAdapter(gateway.DiscordConfig{
			Enabled:   true,
			Token:     "tok",
			Allowlist: []string{"123"},
		})
		if err := adapter.Send(context.Background(), "123", ""); err != nil {
			t.Errorf("Send empty string error = %v", err)
		}
	})

	t.Run("TelegramAdapter Send HTTP error returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "bad request", http.StatusBadRequest)
		}))
		defer server.Close()

		adapter := gateway.NewTelegramAdapter(gateway.TelegramConfig{
			Enabled: true,
			Token:   "tok",
		}, gateway.WithTelegramBaseURL(server.URL))

		err := adapter.Send(context.Background(), "123", "hello")
		if err == nil {
			t.Fatal("Send() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "400") {
			t.Errorf("error = %v, want 400 status error", err)
		}
	})

	t.Run("DiscordAdapter Send HTTP error returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}))
		defer server.Close()

		adapter := gateway.NewDiscordAdapter(gateway.DiscordConfig{
			Enabled: true,
			Token:   "tok",
		}, gateway.WithDiscordBaseURL(server.URL))

		err := adapter.Send(context.Background(), "123", "hello")
		if err == nil {
			t.Fatal("Send() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Errorf("error = %v, want 401 status error", err)
		}
	})

	t.Run("Multiplexer Start on closed multiplexer returns ErrAdapterClosed", func(t *testing.T) {
		mux := gateway.NewMultiplexer()
		_ = mux.Stop()
		if err := mux.Start(context.Background()); !errors.Is(err, gateway.ErrAdapterClosed) {
			t.Errorf("Start() error = %v, want ErrAdapterClosed", err)
		}
	})

	t.Run("Multiplexer HandleEvent with failing brain returns error", func(t *testing.T) {
		ctx := context.Background()
		repo, err := memory.NewRepository(ctx, ":memory:")
		if err != nil {
			t.Fatalf("NewRepository error = %v", err)
		}
		defer repo.Close()

		tg := newMockAdapter("telegram")
		mux := gateway.NewMultiplexer(
			gateway.WithMultiplexerBrain(&failingBrainWrapper{}),
			gateway.WithMultiplexerRepository(repo),
		)
		mux.RegisterAdapter(tg)

		err = mux.HandleEvent(ctx, gateway.MessageEvent{
			Adapter: "telegram",
			ChatID:  "c1",
			Content: "fail please",
		})
		if err == nil {
			t.Fatal("HandleEvent error = nil, want error")
		}
	})
}

type failingBrainWrapper struct{}

func (f *failingBrainWrapper) SetActiveConversation(id string) {}
func (f *failingBrainWrapper) Step(ctx context.Context, input string) error {
	return errors.New("brain execution failed")
}
