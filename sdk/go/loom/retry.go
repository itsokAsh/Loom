package loom

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

func (c *WebhookClient) doWithRetry(ctx context.Context, attemptFunc func(context.Context) (*TriggerResult, error)) (*TriggerResult, error) {
	var lastErr error
	baseDelay := 500 * time.Millisecond
	maxDelay := 5 * time.Second

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for attempt := 0; attempt < c.maxRetries; attempt++ {
		res, err := attemptFunc(ctx)
		if err == nil {
			return res, nil
		}
		
		lastErr = err

		// Check context cancellation before delaying
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		var delay time.Duration

		var rateLimitErr *RateLimitedError
		if errors.As(err, &rateLimitErr) {
			delay = rateLimitErr.RetryAfter
		} else if isRetryableError(err) {
			// Exponential backoff
			backoff := time.Duration(float64(baseDelay) * float64(int(1)<<attempt))
			if backoff > maxDelay {
				backoff = maxDelay
			}
			jitter := time.Duration(r.Float64() * float64(time.Second))
			delay = backoff + jitter
		} else {
			// Non-retryable error (e.g. 400, 401, 404)
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// continue to next attempt
		}
	}

	return nil, lastErr
}

func isRetryableError(err error) bool {
	var internalErr *InternalServerError
	if errors.As(err, &internalErr) {
		return true
	}
	
	var isErr *InvalidSignatureError
	if errors.As(err, &isErr) {
		return false
	}
	var wnErr *WebhookNotFoundError
	if errors.As(err, &wnErr) {
		return false
	}
	var ipErr *InvalidPayloadError
	if errors.As(err, &ipErr) {
		return false
	}
	
	// Network errors
	var apiErr *LoomAPIError
	if errors.As(err, &apiErr) {
		return false
	}
	
	return true
}
