package webhooks

import (
	"encoding/json"
	"net/http"
	"time"
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/loom/trigger-api/internal/db"
	"github.com/loom/trigger-api/internal/queue"
	contracts "github.com/loom/shared/queue-contracts"
)

type Handler struct {
	store     *db.Store
	publisher *queue.Publisher
}

func NewHandler(store *db.Store, publisher *queue.Publisher) *Handler {
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

	wv, err := h.store.GetLatestWorkflowVersion(ctx, webhook.WorkflowID)
	if err != nil {
		http.Error(w, "Failed to fetch latest workflow version", http.StatusInternalServerError)
		return
	}

	exec, err := h.store.CreateExecution(ctx, db.CreateExecutionParams{
		WorkflowID:      webhook.WorkflowID,
		WorkflowVersion: wv.Version,
		IdempotencyKey:  idempKey,
		Status:          "PENDING",
	})
	if err != nil {
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

	var payload map[string]interface{}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&payload)
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

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"executionId": msg.ExecutionID,
		"status":      "PENDING",
	})
}
