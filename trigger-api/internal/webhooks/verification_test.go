package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func TestVerifyHMAC(t *testing.T) {
	tests := []struct {
		name              string
		secret            string
		body              string
		receivedSignature string
		want              bool
	}{
		{
			name:              "valid signature",
			secret:            "test-secret",
			body:              `{"email":"test@example.com","name":"Test User"}`,
			receivedSignature: computeHMAC("test-secret", `{"email":"test@example.com","name":"Test User"}`),
			want:              true,
		},
		{
			name:              "invalid signature",
			secret:            "test-secret",
			body:              `{"email":"test@example.com","name":"Test User"}`,
			receivedSignature: "invalid-signature",
			want:              false,
		},
		{
			name:              "tampered body",
			secret:            "test-secret",
			body:              `{"email":"hacker@evil.com","name":"Hacker"}`,
			receivedSignature: computeHMAC("test-secret", `{"email":"test@example.com","name":"Test User"}`),
			want:              false,
		},
		{
			name:              "wrong secret",
			secret:            "wrong-secret",
			body:              `{"email":"test@example.com","name":"Test User"}`,
			receivedSignature: computeHMAC("test-secret", `{"email":"test@example.com","name":"Test User"}`),
			want:              false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verifyHMAC(tt.secret, []byte(tt.body), tt.receivedSignature)
			if got != tt.want {
				t.Errorf("verifyHMAC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithinReplayWindow(t *testing.T) {
	now := time.Now()
	window := 5 * time.Minute

	tests := []struct {
		name      string
		timestamp int64
		window    time.Duration
		want      bool
	}{
		{
			name:      "current timestamp",
			timestamp: now.Unix(),
			window:    window,
			want:      true,
		},
		{
			name:      "1 minute ago",
			timestamp: now.Add(-1 * time.Minute).Unix(),
			window:    window,
			want:      true,
		},
		{
			name:      "4 minutes ago (within window)",
			timestamp: now.Add(-4 * time.Minute).Unix(),
			window:    window,
			want:      true,
		},
		{
			name:      "6 minutes ago (outside window)",
			timestamp: now.Add(-6 * time.Minute).Unix(),
			window:    window,
			want:      false,
		},
		{
			name:      "10 minutes ago (way outside window)",
			timestamp: now.Add(-10 * time.Minute).Unix(),
			window:    window,
			want:      false,
		},
		{
			name:      "30 seconds in future (acceptable clock skew)",
			timestamp: now.Add(30 * time.Second).Unix(),
			window:    window,
			want:      true,
		},
		{
			name:      "2 minutes in future (too far)",
			timestamp: now.Add(2 * time.Minute).Unix(),
			window:    window,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withinReplayWindow(tt.timestamp, tt.window)
			if got != tt.want {
				t.Errorf("withinReplayWindow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateTimestampEnvelope(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		timestamp int64
		wantErr   bool
	}{
		{
			name:      "valid current timestamp",
			timestamp: now.Unix(),
			wantErr:   false,
		},
		{
			name:      "valid timestamp (4 min ago)",
			timestamp: now.Add(-4 * time.Minute).Unix(),
			wantErr:   false,
		},
		{
			name:      "expired timestamp (6 min ago)",
			timestamp: now.Add(-6 * time.Minute).Unix(),
			wantErr:   true,
		},
		{
			name:      "future timestamp (2 min ahead)",
			timestamp: now.Add(2 * time.Minute).Unix(),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTimestampEnvelope(tt.timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTimestampEnvelope() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper function to compute HMAC for tests
func computeHMAC(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}
