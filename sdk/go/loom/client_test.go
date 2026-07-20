package loom

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_Trigger_SignatureCorrectness(t *testing.T) {
	secret := "test_secret"
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		
		// Verify signature matches EXACTLY the bytes transmitted
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expectedSig := hex.EncodeToString(mac.Sum(nil))
		
		if r.Header.Get("X-Signature") != expectedSig {
			t.Errorf("Signature mismatch. Got %s, want %s", r.Header.Get("X-Signature"), expectedSig)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error_code":"invalid_signature","error":"bad sig"}`))
			return
		}
		
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"execution_id":"exec-123","status":"PENDING"}`))
	}))
	defer server.Close()

	client, err := NewWebhookClient(server.URL, "testpath", secret, WithAllowInsecure(true))
	if err != nil {
		t.Fatal(err)
	}

	payload := map[string]string{"foo": "bar"}
	res, err := client.Trigger(context.Background(), payload)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if res.ExecutionID != "exec-123" {
		t.Errorf("Expected exec-123, got %s", res.ExecutionID)
	}
}

func TestClient_Trigger_RetryEligibility(t *testing.T) {
	secret := "test_secret"
	
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			// Simulate 500 which IS retryable
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error_code":"internal_error","error":"crash"}`))
			return
		}
		
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"execution_id":"exec-123","status":"PENDING"}`))
	}))
	defer server.Close()

	client, err := NewWebhookClient(server.URL, "testpath", secret, WithAllowInsecure(true))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Trigger(context.Background(), map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatalf("Expected success after retry, got error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestClient_Trigger_ReSigningAndIdempotencyKeyStability(t *testing.T) {
	secret := "test_secret"
	
	attempts := 0
	var firstIdempKey string
	var firstTimestamp int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		idempKey := r.Header.Get("Idempotency-Key")
		
		body, _ := io.ReadAll(r.Body)
		var envelope struct {
			Timestamp int64 `json:"timestamp"`
		}
		json.Unmarshal(body, &envelope)

		if attempts == 1 {
			firstIdempKey = idempKey
			firstTimestamp = envelope.Timestamp
			// Simulate 429 which IS retryable
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error_code":"rate_limited","error":"slow down"}`))
			return
		}

		if attempts == 2 {
			if idempKey != firstIdempKey {
				t.Errorf("Idempotency-Key changed! Got %s, want %s", idempKey, firstIdempKey)
			}
			if envelope.Timestamp <= firstTimestamp {
				t.Errorf("Timestamp did not advance. Got %d, want > %d", envelope.Timestamp, firstTimestamp)
			}
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"execution_id":"exec-123","status":"PENDING"}`))
			return
		}
	}))
	defer server.Close()

	client, _ := NewWebhookClient(server.URL, "testpath", secret, WithAllowInsecure(true))
	
	// Fast test, overwrite max retries to 2
	client.maxRetries = 2
	
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := client.Trigger(ctx, map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatalf("Expected success after retry, got error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestClient_Trigger_NonRetryable(t *testing.T) {
	secret := "test_secret"
	
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error_code":"invalid_payload","error":"bad"}`))
	}))
	defer server.Close()

	client, _ := NewWebhookClient(server.URL, "testpath", secret, WithAllowInsecure(true))

	_, err := client.Trigger(context.Background(), map[string]string{"foo": "bar"})
	if err == nil {
		t.Fatal("Expected error, got success")
	}
	
	var ipErr *InvalidPayloadError
	if !errors.As(err, &ipErr) {
		t.Errorf("Expected InvalidPayloadError, got %T", err)
	}
	
	if attempts != 1 {
		t.Errorf("Expected 1 attempt (no retries for 400), got %d", attempts)
	}
}

func TestClient_InsecureURLRejection(t *testing.T) {
	_, err := NewWebhookClient("http://loom.prod", "testpath", "secret")
	if err == nil {
		t.Fatal("Expected error for http:// without AllowInsecure")
	}
	if !strings.Contains(err.Error(), "insecure base URL") {
		t.Errorf("Unexpected error message: %v", err)
	}
}
