package setup_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/setup"
)

func TestProbeConnectivity_Ollama(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/tags" && r.URL.Path != "/api/version" {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"models":[]}`))
		}))
		defer ts.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := setup.ProbeConnectivity(ctx, "ollama", ts.URL, "")
		if err != nil {
			t.Errorf("expected success, got error: %v", err)
		}
	})

	t.Run("unreachable endpoint", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := setup.ProbeConnectivity(ctx, "ollama", "http://127.0.0.1:59999", "")
		if err == nil {
			t.Error("expected error for unreachable endpoint, got nil")
		}
	})
}

func TestProbeConnectivity_OpenAI(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/models" {
				http.NotFound(w, r)
				return
			}
			auth := r.Header.Get("Authorization")
			if auth != "Bearer sk-valid-key" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
		}))
		defer ts.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := setup.ProbeConnectivity(ctx, "openai", ts.URL, "sk-valid-key")
		if err != nil {
			t.Errorf("expected success, got error: %v", err)
		}
	})

	t.Run("unauthorized 401", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"invalid_api_key"}`, http.StatusUnauthorized)
		}))
		defer ts.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := setup.ProbeConnectivity(ctx, "openai", ts.URL, "sk-invalid-key")
		if err == nil {
			t.Error("expected unauthorized error, got nil")
		} else if !strings.Contains(err.Error(), "unauthorized") && !strings.Contains(err.Error(), "401") {
			t.Errorf("expected unauthorized or 401 in error message, got: %v", err)
		}
	})
}

func TestProbeConnectivity_OpenRouter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer sk-or-valid" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := setup.ProbeConnectivity(ctx, "openrouter", ts.URL, "sk-or-valid")
	if err != nil {
		t.Errorf("expected openrouter success, got: %v", err)
	}
}

func TestProbeConnectivity_Anthropic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("x-api-key")
		if key == "" {
			auth := r.Header.Get("Authorization")
			key = strings.TrimPrefix(auth, "Bearer ")
		}
		if key != "sk-ant-valid" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := setup.ProbeConnectivity(ctx, "anthropic", ts.URL, "sk-ant-valid")
	if err != nil {
		t.Errorf("expected anthropic success, got: %v", err)
	}
}

func TestProbeConnectivity_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := setup.ProbeConnectivity(ctx, "ollama", ts.URL, "")
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}
