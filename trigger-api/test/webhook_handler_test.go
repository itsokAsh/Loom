package test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/loom/trigger-api/internal/db"
	"github.com/loom/trigger-api/internal/middleware"
	"github.com/loom/trigger-api/internal/webhooks"
)

// Helper to create a request with HMAC signature
func createRequestWithHMAC(method, target string, body []byte, secret string, idempotencyKey string, webhook db.Webhook) *http.Request {
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

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("path", "test-webhook")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, middleware.WebhookKey, webhook)
	req = req.WithContext(ctx)

	return req
}

func TestHandleIncomingWebhook_Success(t *testing.T) {
	mockStore := &MockWebhookStore{
		GetWorkflowByIDFn: func(ctx context.Context, workflowID pgtype.UUID) (db.Workflow, error) {
			return db.Workflow{}, nil
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

	body := []byte(`{"timestamp":1234567890,"payload":{"event":"test"}}`)
	wh := db.Webhook{
		ID:         pgtype.UUID{Bytes: [16]byte{9}, Valid: true},
		WorkflowID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		Secret:     "my-secret",
		Path:       "test-webhook",
	}
	req := createRequestWithHMAC("POST", "/webhooks/test-webhook", body, "my-secret", "idemp-123", wh)
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
	existingID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	mockStore := &MockWebhookStore{
		GetWorkflowByIDFn: func(ctx context.Context, workflowID pgtype.UUID) (db.Workflow, error) {
			return db.Workflow{}, nil
		},
		GetLatestWorkflowVersionFn: func(ctx context.Context, workflowID pgtype.UUID) (db.WorkflowVersion, error) {
			return db.WorkflowVersion{
				Version:       1,
				DagDefinition: []byte(`{"nodes":[]}`),
			}, nil
		},
		CreateExecutionFn: func(ctx context.Context, arg db.CreateExecutionParams) (db.Execution, error) {
			return db.Execution{}, pgx.ErrNoRows
		},
		GetIdempotentExecutionFn: func(ctx context.Context, webhookID pgtype.UUID, idempotencyKey string) (pgtype.UUID, bool, error) {
			return existingID, true, nil
		},
	}

	mockPublisher := &MockRunPublisher{}
	handler := webhooks.NewHandler(mockStore, mockPublisher)

	body := []byte(`{"timestamp":1234567890,"payload":{"event":"test"}}`)
	wh := db.Webhook{
		ID:         pgtype.UUID{Bytes: [16]byte{9}, Valid: true},
		WorkflowID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		Secret:     "my-secret",
	}
	req := createRequestWithHMAC("POST", "/webhooks/test-webhook", body, "my-secret", "idemp-dup", wh)
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
	if response["execution_id"] == nil {
		t.Errorf("expected execution_id in response, got %v", response)
	}
}

func TestHandleIncomingWebhook_MissingWebhookContext(t *testing.T) {
	handler := webhooks.NewHandler(&MockWebhookStore{}, &MockRunPublisher{})

	req := httptest.NewRequest("POST", "/webhooks/test-webhook", bytes.NewReader([]byte(`{}`)))
	rr := httptest.NewRecorder()
	handler.HandleIncomingWebhook(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestHandleIncomingWebhook_InvalidHMAC(t *testing.T) {
	t.Skip("HMAC verification is handled by middleware, not the handler")
}

func TestHandleIncomingWebhook_MissingHMAC(t *testing.T) {
	t.Skip("HMAC verification is handled by middleware, not the handler")
}

func TestHandleIncomingWebhook_MissingIdempotencyKey(t *testing.T) {
	handler := webhooks.NewHandler(&MockWebhookStore{}, &MockRunPublisher{})

	wh := db.Webhook{ID: pgtype.UUID{Bytes: [16]byte{9}, Valid: true}}
	req := createRequestWithHMAC("POST", "/webhooks/test-webhook", []byte(`{"timestamp":1,"payload":{}}`), "", "", wh)
	rr := httptest.NewRecorder()

	handler.HandleIncomingWebhook(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}
