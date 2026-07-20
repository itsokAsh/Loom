package loom

import (
	"fmt"
	"time"
)

// LoomAPIError is the base error type for all API errors returned by Loom.
type LoomAPIError struct {
	ErrorCode string
	Message   string
	Body      []byte
}

func (e *LoomAPIError) Error() string {
	return fmt.Sprintf("loom API error (%s): %s", e.ErrorCode, e.Message)
}

// InvalidSignatureError is returned when the webhook signature validation fails.
type InvalidSignatureError struct { *LoomAPIError }

// RateLimitedError is returned when the API rate limit is exceeded.
type RateLimitedError struct {
	*LoomAPIError
	RetryAfter time.Duration
}

// WebhookNotFoundError is returned when the specified webhook path doesn't exist.
type WebhookNotFoundError struct { *LoomAPIError }

// InvalidPayloadError is returned when the trigger payload is invalid.
type InvalidPayloadError struct { *LoomAPIError }

// InternalServerError is returned when Loom encounters an internal issue.
type InternalServerError struct { *LoomAPIError }
