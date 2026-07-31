package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/loom/shared/queue-contracts"
	"github.com/loom/trigger-api/internal/db"
	"github.com/loom/trigger-api/internal/middleware"
	"github.com/loom/trigger-api/internal/templates"
	"github.com/loom/trigger-api/internal/workflows"
)

type WebhookStore interface {
	GetLatestWorkflowVersion(ctx context.Context, workflowID pgtype.UUID) (db.WorkflowVersion, error)
	GetWorkflowByID(ctx context.Context, workflowID pgtype.UUID) (db.Workflow, error)
	CreateExecution(ctx context.Context, arg db.CreateExecutionParams) (db.Execution, error)
	GetIdempotentExecution(ctx context.Context, webhookID pgtype.UUID, idempotencyKey string) (pgtype.UUID, bool, error)
	SaveIdempotentExecution(ctx context.Context, webhookID pgtype.UUID, idempotencyKey string, executionID pgtype.UUID, expiresAt time.Time) (bool, error)
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
	ctx := r.Context()

	// Webhook is injected by WebhookAuthMiddleware
	webhook, ok := ctx.Value(middleware.WebhookKey).(db.Webhook)
	if !ok {
		respondError(w, http.StatusInternalServerError, "internal_error", "Webhook context missing")
		return
	}

	idempKey := r.Header.Get("Idempotency-Key")
	if idempKey == "" {
		respondError(w, http.StatusBadRequest, "missing_idempotency_key", "Idempotency-Key header is required")
		return
	}

	var envelope struct {
		Timestamp int64           `json:"timestamp"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_payload", "Invalid request body")
		return
	}

	var payload map[string]interface{}
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			respondError(w, http.StatusBadRequest, "invalid_payload", "Invalid payload JSON")
			return
		}
	}

	workflow, err := h.store.GetWorkflowByID(ctx, webhook.WorkflowID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch workflow")
		return
	}

	if workflow.TemplateID.Valid && workflow.TemplateID.String != "" {
		template, err := templates.GetTemplateByID(workflow.TemplateID.String)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal_error", "Template not found")
			return
		}

		if err := validatePayloadAgainstTemplate(template, payload); err != nil {
			respondError(w, http.StatusBadRequest, "invalid_payload", err.Error())
			return
		}
	}

	wv, err := h.store.GetLatestWorkflowVersion(ctx, webhook.WorkflowID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch latest workflow version")
		return
	}

	// Step 1: Create execution. The DB has ON CONFLICT (workflow_id, idempotency_key) DO NOTHING.
	exec, err := h.store.CreateExecution(ctx, db.CreateExecutionParams{
		WorkflowID:      webhook.WorkflowID,
		WorkflowVersion: wv.Version,
		IdempotencyKey:  idempKey,
		Status:          "PENDING",
	})

	isDuplicate := false

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			isDuplicate = true
		} else {
			respondError(w, http.StatusInternalServerError, "internal_error", "Failed to create execution")
			return
		}
	}

	if isDuplicate {
		// Execution was not created because of conflict. Lookup the duplicate.
		existingExecID, found, err := h.store.GetIdempotentExecution(ctx, webhook.ID, idempKey)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal_error", "Failed to check idempotency")
			return
		}
		if found {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"execution_id": fmt.Sprintf("%x", existingExecID.Bytes),
				"status":       "duplicate_request",
			})
			return
		}
		// Fallback if not found in webhook_idempotency but execution conflicted
		respondError(w, http.StatusInternalServerError, "internal_error", "Idempotency conflict but record missing")
		return
	}

	// Step 2: Since execution was newly created, save to webhook_idempotency mapping
	expiresAt := time.Now().Add(24 * time.Hour)
	inserted, err := h.store.SaveIdempotentExecution(ctx, webhook.ID, idempKey, exec.ID, expiresAt)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to save idempotency mapping")
		return
	}
	if !inserted {
		// Conflict on webhook_idempotency insert, meaning another request hit this at the exact same time
		// and got the lock. We should treat this as a duplicate request.
		existingExecID, found, err := h.store.GetIdempotentExecution(ctx, webhook.ID, idempKey)
		if err == nil && found {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"execution_id": fmt.Sprintf("%x", existingExecID.Bytes),
				"status":       "duplicate_request",
			})
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "Concurrent idempotency conflict")
		return
	}

	if !json.Valid(wv.DagDefinition) {
		respondError(w, http.StatusInternalServerError, "internal_error", "Invalid DAG JSON")
		return
	}

	workerDAG := wv.DagDefinition
	// Strip UI / start nodes if present (saved canvas format)
	if stripped, err := workflows.StripUIFromDAG(wv.DagDefinition); err == nil {
		workerDAG = stripped
	}

	msg := contracts.NewRunMessage{
		ExecutionID:     fmt.Sprintf("%x", exec.ID.Bytes),
		WorkflowID:      fmt.Sprintf("%x", exec.WorkflowID.Bytes),
		WorkflowVersion: int(exec.WorkflowVersion),
		TriggerData:     payload,
		DAGDefinition:   workerDAG,
		TriggeredAt:     time.Now(),
	}

	if err := h.publisher.PublishNewRun(ctx, msg); err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to publish task")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"execution_id": fmt.Sprintf("%x", exec.ID.Bytes),
		"status":       "PENDING", // Note: Using PENDING as per plan for success
	})
}

func respondError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error_code": code,
		"error":      msg,
	})
}

func validatePayloadAgainstTemplate(template *templates.Template, payload map[string]interface{}) error {
	for _, field := range template.RequiredFields {
		if !field.Required {
			continue
		}
		value, exists := payload[field.Name]
		if !exists {
			return fmt.Errorf("missing required field: %s", field.Name)
		}
		if value == nil {
			return fmt.Errorf("required field %s cannot be nil", field.Name)
		}
		strValue, ok := value.(string)
		if !ok {
			return fmt.Errorf("field %s must be a string", field.Name)
		}
		if strValue == "" {
			return fmt.Errorf("required field %s cannot be empty", field.Name)
		}
		if _, err := templates.SanitizeTriggerField(field.Type, strValue); err != nil {
			return fmt.Errorf("invalid field %s: %w", field.Name, err)
		}
	}
	return nil
}
