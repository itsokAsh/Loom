package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

// AdminAPIKeyMiddleware protects management routes with a static API key.
func AdminAPIKeyMiddleware(validAPIKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error_code": "missing_admin_key",
					"error":      "Missing Authorization header",
				})
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" || parts[1] != validAPIKey {
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
