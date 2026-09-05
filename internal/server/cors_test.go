package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/server"
)

func TestCORSMiddleware(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	tests := []struct {
		name           string
		origins        []string
		method         string
		requestOrigin  string
		wantStatus     int
		wantAllowOrig  string
		wantAllowMeth  string
		wantAllowHead  string
		wantMaxAge     string
	}{
		{
			name:          "preflight OPTIONS with wildcard origin",
			origins:       []string{"*"},
			method:        http.MethodOptions,
			requestOrigin: "http://localhost:3000",
			wantStatus:    http.StatusNoContent,
			wantAllowOrig: "*",
			wantAllowMeth: "GET, POST, OPTIONS",
			wantAllowHead: "Authorization, Content-Type, X-Session-ID, Accept",
			wantMaxAge:    "86400",
		},
		{
			name:          "preflight OPTIONS with specific matching origin",
			origins:       []string{"https://app.example.com", "http://localhost:3000"},
			method:        http.MethodOptions,
			requestOrigin: "https://app.example.com",
			wantStatus:    http.StatusNoContent,
			wantAllowOrig: "https://app.example.com",
			wantAllowMeth: "GET, POST, OPTIONS",
			wantAllowHead: "Authorization, Content-Type, X-Session-ID, Accept",
			wantMaxAge:    "86400",
		},
		{
			name:          "regular GET request adds CORS header for wildcard",
			origins:       []string{"*"},
			method:        http.MethodGet,
			requestOrigin: "http://localhost:3000",
			wantStatus:    http.StatusOK,
			wantAllowOrig: "*",
		},
		{
			name:          "regular GET request adds CORS header for matching origin",
			origins:       []string{"http://localhost:3000"},
			method:        http.MethodGet,
			requestOrigin: "http://localhost:3000",
			wantStatus:    http.StatusOK,
			wantAllowOrig: "http://localhost:3000",
		},
		{
			name:          "regular GET request with non-matching origin sets no CORS header",
			origins:       []string{"https://app.example.com"},
			method:        http.MethodGet,
			requestOrigin: "http://evil.com",
			wantStatus:    http.StatusOK,
			wantAllowOrig: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := server.CORSMiddleware(tt.origins)(nextHandler)

			req := httptest.NewRequest(tt.method, "/v1/models", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllowOrig {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tt.wantAllowOrig)
			}

			if tt.wantAllowMeth != "" {
				if got := rec.Header().Get("Access-Control-Allow-Methods"); got != tt.wantAllowMeth {
					t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, tt.wantAllowMeth)
				}
			}

			if tt.wantAllowHead != "" {
				if got := rec.Header().Get("Access-Control-Allow-Headers"); got != tt.wantAllowHead {
					t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, tt.wantAllowHead)
				}
			}

			if tt.wantMaxAge != "" {
				if got := rec.Header().Get("Access-Control-Max-Age"); got != tt.wantMaxAge {
					t.Errorf("Access-Control-Max-Age = %q, want %q", got, tt.wantMaxAge)
				}
			}
		})
	}
}
