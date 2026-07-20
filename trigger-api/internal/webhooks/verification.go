package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// verifyHMAC verifies the HMAC signature over raw request body bytes.
// This MUST be called with the raw bytes received from the request,
// BEFORE any JSON parsing or re-serialization.
//
// This avoids false-negative verification failures from key-ordering
// or whitespace differences between client and server JSON serializers.
func verifyHMAC(secret string, rawBody []byte, receivedSignature string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(expectedSignature), []byte(receivedSignature))
}

// withinReplayWindow checks if a timestamp is within the acceptable window.
// This bounds how long a captured request (from logs, proxy, etc.) remains replayable.
//
// The timestamp is considered valid if:
// - It's not more than `window` duration in the past
// - It's not in the future (allows small clock skew)
func withinReplayWindow(timestamp int64, window time.Duration) bool {
	requestTime := time.Unix(timestamp, 0)
	now := time.Now()

	// Check if timestamp is too old
	if now.Sub(requestTime) > window {
		return false
	}

	// Check if timestamp is in the future (allow small clock skew)
	// Reject if more than 1 minute in the future
	if requestTime.Sub(now) > time.Minute {
		return false
	}

	return true
}

// validateTimestampEnvelope validates the timestamp envelope structure
// and returns an error if the timestamp is outside the acceptable window.
func validateTimestampEnvelope(timestamp int64) error {
	const replayWindow = 5 * time.Minute

	if !withinReplayWindow(timestamp, replayWindow) {
		requestTime := time.Unix(timestamp, 0)
		now := time.Now()

		if now.Sub(requestTime) > replayWindow {
			return fmt.Errorf("request timestamp too old: %v (server time: %v)", requestTime, now)
		}
		if requestTime.Sub(now) > time.Minute {
			return fmt.Errorf("request timestamp in future: %v (server time: %v, possible clock skew)", requestTime, now)
		}
	}

	return nil
}
