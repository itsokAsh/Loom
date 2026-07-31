package middleware

import (
	"net/http"
	"strings"
)

// CORS allows browser calls from the UI (Vercel / Render static) to the API on another host.
// Set CORS_ORIGINS to comma-separated origins, e.g. https://loom.vercel.app,https://loom.onrender.com
// Use "*" to allow any origin (portfolio demos only).
func CORS(origins string) func(http.Handler) http.Handler {
	allowed := parseOrigins(origins)
	allowAll := len(allowed) == 1 && allowed[0] == "*"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowed) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			origin := r.Header.Get("Origin")
			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if origin != "" && originAllowed(origin, allowed) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set(
				"Access-Control-Allow-Headers",
				"Authorization, Content-Type, X-Admin-API-Key, X-Signature, Idempotency-Key",
			)

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func parseOrigins(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func originAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == origin {
			return true
		}
	}
	return false
}
