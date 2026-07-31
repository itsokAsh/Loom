package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

// AdminAPIKeyMiddleware protects management routes with a static API key.
// Accepts Authorization: Bearer <key> or X-Admin-API-Key: <key>.
func AdminAPIKeyMiddleware(validAPIKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := extractAdminKey(r)
			if key == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error_code": "missing_admin_key",
					"error":      "Missing admin API key (Authorization: Bearer or X-Admin-API-Key)",
				})
				return
			}

			if key != validAPIKey {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error_code": "missing_admin_key",
					"error":      "Invalid admin API key",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractAdminKey(r *http.Request) string {
	if h := r.Header.Get("X-Admin-API-Key"); h != "" {
		return h
	}
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) == 2 && parts[0] == "Bearer" {
		return parts[1]
	}
	return ""
}
