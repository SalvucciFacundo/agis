package server

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

// AuthMiddleware creates a middleware enforcing Bearer token authentication against apiKey.
// If apiKey is empty, authentication is bypassed (open mode).
// Health check paths (/healthz and /v1/health) always bypass authentication.
func AuthMiddleware(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if apiKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Public unauthenticated endpoints
			path := r.URL.Path
			if path == "/healthz" || path == "/v1/health" {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				writeUnauthorized(w)
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) != 1 {
				writeUnauthorized(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: APIError{
			Message: "Incorrect API key provided or missing Bearer token.",
			Type:    "invalid_request_error",
			Param:   nil,
			Code:    "invalid_api_key",
		},
	})
}
