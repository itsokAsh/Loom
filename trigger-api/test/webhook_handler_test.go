package test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/loom/trigger-api/internal/db"
	"github.com/loom/trigger-api/internal/webhooks"
)

// Helper to create a request with HMAC signature
func createRequestWithHMAC(method, target string, body []byte, secret string, idempotencyKey string) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expectedMAC := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Signature", expectedMAC)
	}

	// Add chi router context so URL parameters are parsed correctly
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("path", "test-webhook")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	return req
}

func TestHandleIncomingWebhook_Success(t *testing.T) {
	mockStore := &MockWebhookStore{
		GetWebhookByPathFn: func(ctx context.Context, path string) (db.Webhook, error) {
			return db.Webhook{
				WorkflowID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
				Secret:     "my-secret",
			}, nil
		},
		GetLatestWorkflowVersionFn: func(ctx context.Context, workflowID pgtype.UUID) (db.WorkflowVersion, error) {
			return db.WorkflowVersion{
				Version:       1,
				DagDefinition: []byte(`{"nodes":[]}`),
			}, nil
		},
		CreateExecutionFn: func(ctx context.Context, arg db.CreateExecutionParams) (db.Execution, error) {
			return db.Execution{
				ID:              pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
				Status:          "PENDING",
			}, nil
		},
	}

	mockPublisher := &MockRunPublisher{}
	handler := webhooks.NewHandler(mockStore, mockPublisher)

	body := []byte(`{"event":"test"}`)
	req := createRequestWithHMAC("POST", "/webhooks/test-webhook", body, "my-secret", "idemp-123")
	rr := httptest.NewRecorder()

	handler.HandleIncomingWebhook(rr, req)

	if status := rr.Code; status != http.StatusAccepted {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusAccepted)
	}

	if mockPublisher.PublishedCount != 1 {
		t.Errorf("expected 1 message published, got %v", mockPublisher.PublishedCount)
	}
}

func TestHandleIncomingWebhook_DuplicateIdempotency(t *testing.T) {
	mockStore := &MockWebhookStore{
		GetWebhookByPathFn: func(ctx context.Context, path string) (db.Webhook, error) {
			return db.Webhook{
				WorkflowID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
				Secret:     "my-secret",
			}, nil
		},
		GetLatestWorkflowVersionFn: func(ctx context.Context, workflowID pgtype.UUID) (db.WorkflowVersion, error) {
			return db.WorkflowVersion{
				Version:       1,
				DagDefinition: []byte(`{"nodes":[]}`),
			}, nil
		},
		CreateExecutionFn: func(ctx context.Context, arg db.CreateExecutionParams) (db.Execution, error) {
			return db.Execution{}, errors.New("conflict")
		},
		GetExecutionByWorkflowAndIdempotencyKeyFn: func(ctx context.Context, arg db.GetExecutionByWorkflowAndIdempotencyKeyParams) (db.Execution, error) {
			return db.Execution{
				ID:     pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
				Status: "PENDING",
			}, nil
		},
	}

	mockPublisher := &MockRunPublisher{}
	handler := webhooks.NewHandler(mockStore, mockPublisher)

	body := []byte(`{"event":"test"}`)
	req := createRequestWithHMAC("POST", "/webhooks/test-webhook", body, "my-secret", "idemp-dup")
	rr := httptest.NewRecorder()

	handler.HandleIncomingWebhook(rr, req)

	if status := rr.Code; status != http.StatusAccepted {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusAccepted)
	}

	// Important: Publisher should NOT be called on duplicate
	if mockPublisher.PublishedCount != 0 {
		t.Errorf("expected 0 messages published for duplicate idempotency key, got %v", mockPublisher.PublishedCount)
	}
	
	// Response should include the execution ID
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	if response["executionId"] != "02000000000000000000000000000000" { // hex of [16]byte{2}
		t.Errorf("expected correct executionId in response, got %v", response["executionId"])
	}
}

func TestHandleIncomingWebhook_MissingHMAC(t *testing.T) {
	mockStore := &MockWebhookStore{
		GetWebhookByPathFn: func(ctx context.Context, path string) (db.Webhook, error) {
			return db.Webhook{
				Secret: "my-secret",
			}, nil
		},
	}
	handler := webhooks.NewHandler(mockStore, &MockRunPublisher{})

	req := createRequestWithHMAC("POST", "/webhooks/test-webhook", []byte(""), "", "idemp-123")
	// explicitly clear header in case createRequestWithHMAC sets it
	req.Header.Del("X-Signature")
	
	rr := httptest.NewRecorder()
	handler.HandleIncomingWebhook(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
	}
}

func TestHandleIncomingWebhook_InvalidHMAC(t *testing.T) {
	mockStore := &MockWebhookStore{
		GetWebhookByPathFn: func(ctx context.Context, path string) (db.Webhook, error) {
			return db.Webhook{
				Secret: "my-secret",
			}, nil
		},
	}
	handler := webhooks.NewHandler(mockStore, &MockRunPublisher{})

	req := createRequestWithHMAC("POST", "/webhooks/test-webhook", []byte(`{"event":"test"}`), "wrong-secret", "idemp-123")
	rr := httptest.NewRecorder()

	handler.HandleIncomingWebhook(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code for invalid HMAC: got %v want %v", status, http.StatusUnauthorized)
	}
}

func TestHandleIncomingWebhook_MissingIdempotencyKey(t *testing.T) {
	handler := webhooks.NewHandler(&MockWebhookStore{}, &MockRunPublisher{})

	req := createRequestWithHMAC("POST", "/webhooks/test-webhook", []byte(""), "", "") // No idempotency key
	rr := httptest.NewRecorder()

	handler.HandleIncomingWebhook(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}
