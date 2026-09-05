package server

import (
	"net/http"
	"strings"
)

// CORSMiddleware returns a middleware configuring Cross-Origin Resource Sharing.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowedOrigins) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			origin := r.Header.Get("Origin")
			allowOrigin := resolveAllowOrigin(allowedOrigins, origin)

			if allowOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			}

			if r.Method == http.MethodOptions {
				if allowOrigin != "" {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Session-ID, Accept")
					w.Header().Set("Access-Control-Max-Age", "86400")
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func resolveAllowOrigin(allowed []string, origin string) string {
	for _, o := range allowed {
		if o == "*" {
			return "*"
		}
		if origin != "" && strings.EqualFold(o, origin) {
			return origin
		}
	}
	return ""
}
