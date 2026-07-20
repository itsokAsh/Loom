package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/loom/trigger-api/internal/db"
	"golang.org/x/time/rate"
)

type rateLimiter struct {
	limiters sync.Map
}

func NewWebhookRateLimiter() *rateLimiter {
	return &rateLimiter{}
}

func (rl *rateLimiter) getLimiter(webhookID pgtype.UUID) *rate.Limiter {
	id := fmt.Sprintf("%x", webhookID.Bytes)
	
	v, exists := rl.limiters.Load(id)
	if !exists {
		// 1000 requests per hour = ~0.27 requests/second. Burst of 50.
		limit := rate.Every(time.Hour / 1000)
		limiter := rate.NewLimiter(limit, 50)
		v, _ = rl.limiters.LoadOrStore(id, limiter)
	}
	
	return v.(*rate.Limiter)
}

func (rl *rateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			webhook, ok := ctx.Value(WebhookKey).(db.Webhook)
			if !ok {
				// Should not happen if WebhookAuthMiddleware runs first
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error_code": "internal_error",
					"error":      "Webhook context missing",
				})
				return
			}

			limiter := rl.getLimiter(webhook.ID)
			
			// We use Reserve to see if it would exceed limits and by how much
			res := limiter.Reserve()
			if !res.OK() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error_code": "internal_error",
					"error":      "Rate limiter error",
				})
				return
			}

			delay := res.Delay()
			if delay == 0 {
				// We are within limits, proceed!
				next.ServeHTTP(w, r)
				return
			}

			// We are over the limit. Cancel the reservation so we don't consume tokens.
			res.Cancel()

			seconds := int(delay.Seconds())
			if seconds < 1 {
				seconds = 1
			}
			
			w.Header().Set("Retry-After", fmt.Sprintf("%d", seconds))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"error_code": "rate_limited",
				"error":      "Rate limit exceeded",
			})
		})
	}
}
