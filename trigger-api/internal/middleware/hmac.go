package middleware

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/loom/trigger-api/internal/db"
)

type WebhookContextKey string

const WebhookKey WebhookContextKey = "webhook"

func WebhookAuthMiddleware(store *db.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := chi.URLParam(r, "path")
			ctx := r.Context()

			webhook, err := store.GetWebhookByPath(ctx, path)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{
					"error_code": "webhook_not_found",
					"error":      "Webhook not found",
				})
				return
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error_code": "invalid_payload",
					"error":      "Failed to read body",
				})
				return
			}
			// Restore the io.ReadCloser to its original state
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			signature := r.Header.Get("X-Signature")
			if signature == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error_code": "invalid_signature",
					"error":      "Missing X-Signature header",
				})
				return
			}

			if !verifyHMAC(webhook.Secret, bodyBytes, signature) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error_code": "invalid_signature",
					"error":      "HMAC signature verification failed",
				})
				return
			}

			var envelope struct {
				Timestamp int64 `json:"timestamp"`
			}
			if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error_code": "invalid_payload",
					"error":      "Invalid request body format",
				})
				return
			}

			if err := validateTimestampEnvelope(envelope.Timestamp); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error_code": "invalid_signature",
					"error":      fmt.Sprintf("Timestamp validation failed: %v", err),
				})
				return
			}

			ctx = context.WithValue(ctx, WebhookKey, webhook)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func verifyHMAC(secret string, rawBody []byte, receivedSignature string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expectedSignature), []byte(receivedSignature))
}

func validateTimestampEnvelope(timestamp int64) error {
	const replayWindow = 5 * time.Minute
	requestTime := time.Unix(timestamp, 0)
	now := time.Now()

	if now.Sub(requestTime) > replayWindow {
		return fmt.Errorf("request timestamp too old")
	}
	if requestTime.Sub(now) > time.Minute {
		return fmt.Errorf("request timestamp in future")
	}
	return nil
}
