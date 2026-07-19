package webhooks

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
	contracts "github.com/loom/shared/queue-contracts"
	"github.com/jackc/pgx/v5/pgtype"
)

type WebhookStore interface {
	GetWebhookByPath(ctx context.Context, path string) (db.Webhook, error)
	GetLatestWorkflowVersion(ctx context.Context, workflowID pgtype.UUID) (db.WorkflowVersion, error)
	CreateExecution(ctx context.Context, arg db.CreateExecutionParams) (db.Execution, error)
	GetExecutionByWorkflowAndIdempotencyKey(ctx context.Context, arg db.GetExecutionByWorkflowAndIdempotencyKeyParams) (db.Execution, error)
}

type RunPublisher interface {
	PublishNewRun(ctx context.Context, msg contracts.NewRunMessage) error
}

type Handler struct {
	store     WebhookStore
	publisher RunPublisher
}

func NewHandler(store WebhookStore, publisher RunPublisher) *Handler {
	return &Handler{
		store:     store,
		publisher: publisher,
	}
}

func (h *Handler) HandleIncomingWebhook(w http.ResponseWriter, r *http.Request) {
	path := chi.URLParam(r, "path")
	idempKey := r.Header.Get("Idempotency-Key")

	if idempKey == "" {
		http.Error(w, "Idempotency-Key header is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	webhook, err := h.store.GetWebhookByPath(ctx, path)
	if err != nil {
		http.Error(w, "Webhook not found", http.StatusNotFound)
		return
	}

	// Verify HMAC signature
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	signature := r.Header.Get("X-Signature")
	if signature == "" {
		http.Error(w, "Missing X-Signature header", http.StatusUnauthorized)
		return
	}

	mac := hmac.New(sha256.New, []byte(webhook.Secret))
	mac.Write(bodyBytes)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	wv, err := h.store.GetLatestWorkflowVersion(ctx, webhook.WorkflowID)
	if err != nil {
		http.Error(w, "Failed to fetch latest workflow version", http.StatusInternalServerError)
		return
	}

	isNewExecution := true
	exec, err := h.store.CreateExecution(ctx, db.CreateExecutionParams{
		WorkflowID:      webhook.WorkflowID,
		WorkflowVersion: wv.Version,
		IdempotencyKey:  idempKey,
		Status:          "PENDING",
	})
	if err != nil {
		isNewExecution = false
		// If ON CONFLICT DO NOTHING returns empty, we should fetch existing
		exec, err = h.store.GetExecutionByWorkflowAndIdempotencyKey(ctx, db.GetExecutionByWorkflowAndIdempotencyKeyParams{
			WorkflowID:     webhook.WorkflowID,
			IdempotencyKey: idempKey,
		})
		if err != nil {
			http.Error(w, "Failed to create or fetch execution", http.StatusInternalServerError)
			return
		}
	}

	if isNewExecution {
		var payload map[string]interface{}
		if len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, &payload); err != nil && err != io.EOF {
				// Continue without payload if unparseable, or could error here. Assuming continue as per old logic.
			}
		}

		if !json.Valid(wv.DagDefinition) {
			http.Error(w, "Invalid DAG JSON", http.StatusInternalServerError)
			return
		}

		msg := contracts.NewRunMessage{
			ExecutionID:     fmt.Sprintf("%x", exec.ID.Bytes),
			WorkflowID:      fmt.Sprintf("%x", exec.WorkflowID.Bytes),
			WorkflowVersion: int(exec.WorkflowVersion),
			TriggerData:     payload,
			DAGDefinition:   wv.DagDefinition,
			TriggeredAt:     time.Now(),
		}

		if err := h.publisher.PublishNewRun(ctx, msg); err != nil {
			http.Error(w, "Failed to publish task", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"executionId": fmt.Sprintf("%x", exec.ID.Bytes),
		"status":      exec.Status,
	})
}

