package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/server"
)

func TestAuthMiddleware(t *testing.T) {
	apiKey := "sk-secret-key-12345"
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("authorized"))
	})

	tests := []struct {
		name         string
		apiKey       string
		authHeader   string
		path         string
		wantStatus   int
		wantBodyText string
		checkError   bool
	}{
		{
			name:         "valid bearer token passes",
			apiKey:       apiKey,
			authHeader:   "Bearer sk-secret-key-12345",
			path:         "/v1/models",
			wantStatus:   http.StatusOK,
			wantBodyText: "authorized",
		},
		{
			name:         "invalid bearer token returns 401",
			apiKey:       apiKey,
			authHeader:   "Bearer sk-wrong-token",
			path:         "/v1/models",
			wantStatus:   http.StatusUnauthorized,
			checkError:   true,
		},
		{
			name:         "missing authorization header returns 401",
			apiKey:       apiKey,
			authHeader:   "",
			path:         "/v1/models",
			wantStatus:   http.StatusUnauthorized,
			checkError:   true,
		},
		{
			name:         "malformed authorization header returns 401",
			apiKey:       apiKey,
			authHeader:   "Basic dXNlcjpwYXNz",
			path:         "/v1/models",
			wantStatus:   http.StatusUnauthorized,
			checkError:   true,
		},
		{
			name:         "empty api key allows unauthenticated access (open mode)",
			apiKey:       "",
			authHeader:   "",
			path:         "/v1/models",
			wantStatus:   http.StatusOK,
			wantBodyText: "authorized",
		},
		{
			name:         "health check bypasses auth even when api key is set",
			apiKey:       apiKey,
			authHeader:   "",
			path:         "/healthz",
			wantStatus:   http.StatusOK,
			wantBodyText: "authorized",
		},
		{
			name:         "v1 health check bypasses auth even when api key is set",
			apiKey:       apiKey,
			authHeader:   "",
			path:         "/v1/health",
			wantStatus:   http.StatusOK,
			wantBodyText: "authorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := server.AuthMiddleware(tt.apiKey)(nextHandler)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantBodyText != "" && rec.Body.String() != tt.wantBodyText {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBodyText)
			}

			if tt.checkError {
				var errResp server.ErrorResponse
				if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
					t.Fatalf("failed to decode error response JSON: %v", err)
				}
				if errResp.Error.Type != "invalid_request_error" {
					t.Errorf("error.type = %q, want 'invalid_request_error'", errResp.Error.Type)
				}
				if errResp.Error.Code != "invalid_api_key" {
					t.Errorf("error.code = %q, want 'invalid_api_key'", errResp.Error.Code)
				}
			}
		})
	}
}
