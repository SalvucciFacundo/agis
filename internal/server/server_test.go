package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/server"
)

func TestServer_HealthEndpoints(t *testing.T) {
	opts := server.Options{
		Host:     "127.0.0.1",
		Port:     8080,
		APIKey:   "secret-token",
		Profile:  "test-profile",
		Version:  "1.0.0",
		Provider: "ollama",
		Model:    "llama3.2",
	}
	srv := server.New(opts)
	handler := srv.Handler()

	paths := []string{"/healthz", "/v1/health"}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}

			var res server.HealthResponse
			if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
				t.Fatalf("failed to decode health response: %v", err)
			}

			if res.Status != "ok" {
				t.Errorf("res.Status = %q, want 'ok'", res.Status)
			}
			if res.Version != "1.0.0" {
				t.Errorf("res.Version = %q, want '1.0.0'", res.Version)
			}
			if res.Profile != "test-profile" {
				t.Errorf("res.Profile = %q, want 'test-profile'", res.Profile)
			}
			if res.ActiveProvider != "ollama" {
				t.Errorf("res.ActiveProvider = %q, want 'ollama'", res.ActiveProvider)
			}
			if res.ActiveModel != "llama3.2" {
				t.Errorf("res.ActiveModel = %q, want 'llama3.2'", res.ActiveModel)
			}
		})
	}
}

func TestServer_ModelsEndpoint(t *testing.T) {
	opts := server.Options{
		Host:     "127.0.0.1",
		Port:     8080,
		APIKey:   "secret-token",
		Provider: "anthropic",
		Model:    "claude-3-5-sonnet",
	}
	srv := server.New(opts)
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var res server.ModelListResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode models response: %v", err)
	}

	if res.Object != "list" {
		t.Errorf("res.Object = %q, want 'list'", res.Object)
	}
	if len(res.Data) != 1 {
		t.Fatalf("got %d models, want 1", len(res.Data))
	}
	if res.Data[0].ID != "claude-3-5-sonnet" {
		t.Errorf("model.id = %q, want 'claude-3-5-sonnet'", res.Data[0].ID)
	}
	if res.Data[0].OwnedBy != "anthropic" {
		t.Errorf("model.owned_by = %q, want 'anthropic'", res.Data[0].OwnedBy)
	}
	if res.Data[0].Object != "model" {
		t.Errorf("model.object = %q, want 'model'", res.Data[0].Object)
	}
}

func TestServer_Lifecycle(t *testing.T) {
	opts := server.Options{
		Host:         "127.0.0.1",
		Port:         0, // auto-allocate port
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	srv := server.New(opts)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	// Wait briefly for server to bind listener
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown error = %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Fatalf("Start() returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Start() to return after Shutdown")
	}
}
