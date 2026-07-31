package workflows

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/loom/trigger-api/internal/db"
	"github.com/loom/trigger-api/internal/orchestration"
	contracts "github.com/loom/shared/queue-contracts"
	"github.com/robfig/cron/v3"
)

type Handler struct {
	service    *Service
	store      *db.Store
	orchClient *orchestration.Client
	publisher  RunPublisher
}

type RunPublisher interface {
	PublishNewRun(ctx context.Context, msg contracts.NewRunMessage) error
}

func NewHandler(service *Service, store *db.Store, orchClient *orchestration.Client, publisher RunPublisher) *Handler {
	return &Handler{service: service, store: store, orchClient: orchClient, publisher: publisher}
}

func (h *Handler) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string          `json:"name"`
		DAG  json.RawMessage `json:"dag"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "\"name\" is required", http.StatusBadRequest)
		return
	}
	if len(req.DAG) == 0 {
		http.Error(w, "\"dag\" is required", http.StatusBadRequest)
		return
	}

	if err := ValidateDAGJSON(req.DAG); err != nil {
		http.Error(w, "DAG validation failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	wf, wv, err := h.service.CreateWorkflow(r.Context(), req.Name, req.DAG)
	if err != nil {
		http.Error(w, "Failed to create workflow", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":        wf.ID,
		"name":      wf.Name,
		"version":   wv.Version,
		"createdAt": wf.CreatedAt,
	})
}

func (h *Handler) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	pgID, err := normalizeUUIDParam(id)
	if err != nil {
		http.Error(w, "Invalid workflow ID format", http.StatusBadRequest)
		return
	}

	wf, err := h.service.GetWorkflow(r.Context(), pgID)
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	wv, err := h.store.GetLatestWorkflowVersion(r.Context(), pgID)
	if err != nil {
		http.Error(w, "Workflow version not found", http.StatusNotFound)
		return
	}

	var dag json.RawMessage
	if err := json.Unmarshal(wv.DagDefinition, &dag); err != nil {
		dag = wv.DagDefinition
	}

	resp := map[string]interface{}{
		"id":        uuidString(wf.ID),
		"name":      wf.Name,
		"createdAt": wf.CreatedAt,
		"version":   wv.Version,
		"dag":       dag,
	}
	if wf.TemplateID.Valid {
		resp["templateId"] = wf.TemplateID.String
	}
	if wf.TemplateVersion.Valid {
		resp["templateVersion"] = wf.TemplateVersion.Int32
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	list, err := h.store.ListWorkflows(r.Context())
	if err != nil {
		http.Error(w, "Failed to list workflows", http.StatusInternalServerError)
		return
	}
	out := make([]map[string]interface{}, 0, len(list))
	for _, wf := range list {
		item := map[string]interface{}{
			"id":        uuidString(wf.ID),
			"name":      wf.Name,
			"createdAt": wf.CreatedAt,
		}
		if wf.Fingerprint.Valid {
			item["fingerprint"] = wf.Fingerprint.String
		}
		if wf.TemplateID.Valid {
			item["templateId"] = wf.TemplateID.String
		}
		if wf.TemplateVersion.Valid {
			item["templateVersion"] = wf.TemplateVersion.Int32
		}
		out = append(out, item)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func uuidString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	b := id.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func normalizeUUIDParam(id string) (pgtype.UUID, error) {
	var pgID pgtype.UUID
	if err := pgID.Scan(id); err == nil {
		return pgID, nil
	}
	// Accept 32-char hex without dashes (template create historically returned this).
	if len(id) == 32 {
		dashed := id[0:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:32]
		if err := pgID.Scan(dashed); err == nil {
			return pgID, nil
		}
	}
	return pgID, fmt.Errorf("invalid uuid")
}

func (h *Handler) UpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pgID, err := normalizeUUIDParam(id)
	if err != nil {
		http.Error(w, "Invalid workflow ID format", http.StatusBadRequest)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "\"name\" is required", http.StatusBadRequest)
		return
	}
	if err := h.store.UpdateWorkflowName(r.Context(), pgID, req.Name); err != nil {
		http.Error(w, "Failed to update workflow", http.StatusInternalServerError)
		return
	}
	wf, err := h.service.GetWorkflow(r.Context(), pgID)
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wf)
}

func (h *Handler) DeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pgID, err := normalizeUUIDParam(id)
	if err != nil {
		http.Error(w, "Invalid workflow ID format", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteWorkflow(r.Context(), pgID); err != nil {
		http.Error(w, "Failed to delete workflow", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pgID, err := normalizeUUIDParam(id)
	if err != nil {
		http.Error(w, "Invalid workflow ID format", http.StatusBadRequest)
		return
	}
	webhooks, err := h.store.ListWebhooksByWorkflow(r.Context(), pgID)
	if err != nil {
		http.Error(w, "Failed to list webhooks", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(webhooks)
}

func scheduleToJSON(s db.Schedule) map[string]interface{} {
	item := map[string]interface{}{
		"id":             uuidString(s.ID),
		"workflowId":     uuidString(s.WorkflowID),
		"cronExpression": s.CronExpression,
		"createdAt":      s.CreatedAt,
	}
	if s.NextRunAt.Valid {
		item["nextRunAt"] = s.NextRunAt.Time
	}
	return item
}

func (h *Handler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pgID, err := normalizeUUIDParam(id)
	if err != nil {
		http.Error(w, "Invalid workflow ID format", http.StatusBadRequest)
		return
	}
	schedules, err := h.store.ListSchedulesByWorkflow(r.Context(), pgID)
	if err != nil {
		http.Error(w, "Failed to list schedules", http.StatusInternalServerError)
		return
	}
	out := make([]map[string]interface{}, 0, len(schedules))
	for _, s := range schedules {
		out = append(out, scheduleToJSON(s))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) ValidateWorkflowDAG(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DAG json.RawMessage `json:"dag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := ValidateDAGJSON(req.DAG); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"valid": false, "error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"valid": true})
}

func (h *Handler) AddVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	pgID, err := normalizeUUIDParam(id)
	if err != nil {
		http.Error(w, "Invalid workflow ID format", http.StatusBadRequest)
		return
	}

	var req struct {
		Version int32           `json:"version"`
		DAG     json.RawMessage `json:"dag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := ValidateDAGJSON(req.DAG); err != nil {
		http.Error(w, "DAG validation failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	wv, err := h.service.AddVersion(r.Context(), pgID, req.Version, req.DAG)
	if err != nil {
		http.Error(w, "Failed to add version", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(wv)
}

func (h *Handler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	pgID, err := normalizeUUIDParam(id)
	if err != nil {
		http.Error(w, "Invalid workflow ID format", http.StatusBadRequest)
		return
	}

	path := generateRandomString(12)
	secret := generateRandomString(32)

	webhook, err := h.store.CreateWebhook(r.Context(), db.CreateWebhookParams{
		WorkflowID: pgID,
		Path:       path,
		Secret:     secret,
	})
	if err != nil {
		http.Error(w, "Failed to create webhook", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(webhook)
}

func (h *Handler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	pgID, err := normalizeUUIDParam(id)
	if err != nil {
		http.Error(w, "Invalid workflow ID format", http.StatusBadRequest)
		return
	}

	var req struct {
		CronExpression string `json:"cronExpression"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(req.CronExpression)
	if err != nil {
		http.Error(w, "Invalid cron expression", http.StatusBadRequest)
		return
	}

	nextRun := schedule.Next(time.Now())

	if err := h.store.DeleteSchedulesByWorkflow(r.Context(), pgID); err != nil {
		http.Error(w, "Failed to replace schedule", http.StatusInternalServerError)
		return
	}

	sched, err := h.store.CreateSchedule(r.Context(), db.CreateScheduleParams{
		WorkflowID:     pgID,
		CronExpression: req.CronExpression,
		NextRunAt:      pgtype.Timestamptz{Time: nextRun, Valid: true},
	})
	if err != nil {
		http.Error(w, "Failed to create schedule", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(scheduleToJSON(sched))
}

func (h *Handler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "id")
	scheduleID := chi.URLParam(r, "scheduleId")

	pgWorkflowID, err := normalizeUUIDParam(workflowID)
	if err != nil {
		http.Error(w, "Invalid workflow ID format", http.StatusBadRequest)
		return
	}
	pgScheduleID, err := normalizeUUIDParam(scheduleID)
	if err != nil {
		http.Error(w, "Invalid schedule ID format", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteSchedule(r.Context(), pgWorkflowID, pgScheduleID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "Schedule not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to delete schedule", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListExecutions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	pgID, err := normalizeUUIDParam(id)
	if err != nil {
		http.Error(w, "Invalid workflow ID format", http.StatusBadRequest)
		return
	}

	limit := int32(20)

	var cursor pgtype.Timestamptz

	executions, err := h.store.ListExecutions(r.Context(), db.ListExecutionsParams{
		WorkflowID: pgID,
		Limit:      limit,
		Cursor:     cursor,
	})
	if err != nil {
		http.Error(w, "Failed to list executions", http.StatusInternalServerError)
		return
	}

	out := make([]map[string]interface{}, 0, len(executions))
	for _, ex := range executions {
		item := map[string]interface{}{
			"id":             uuidString(ex.ID),
			"workflowId":     uuidString(ex.WorkflowID),
			"status":         ex.Status,
			"idempotencyKey": ex.IdempotencyKey,
			"createdAt":      ex.CreatedAt,
			"updatedAt":      ex.UpdatedAt,
		}
		if ex.StartedAt.Valid {
			item["startedAt"] = ex.StartedAt.Time
		}
		if ex.CompletedAt.Valid {
			item["completedAt"] = ex.CompletedAt.Time
		}
		out = append(out, item)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) GetExecution(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	pgID, err := normalizeUUIDParam(id)
	if err != nil {
		http.Error(w, "Invalid execution ID format", http.StatusBadRequest)
		return
	}

	exec, err := h.store.GetExecution(r.Context(), pgID)
	if err != nil {
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	}

	resp := map[string]interface{}{
		"id":               uuidString(exec.ID),
		"workflowId":       uuidString(exec.WorkflowID),
		"workflowVersion":  exec.WorkflowVersion,
		"idempotencyKey":   exec.IdempotencyKey,
		"status":           exec.Status,
		"createdAt":        exec.CreatedAt,
		"updatedAt":        exec.UpdatedAt,
	}
	if exec.StartedAt.Valid {
		resp["startedAt"] = exec.StartedAt.Time
	}
	if exec.CompletedAt.Valid {
		resp["completedAt"] = exec.CompletedAt.Time
	}
	if h.orchClient != nil {
		if td, err := h.orchClient.GetExecutionTriggerData(uuidString(exec.ID)); err == nil && len(td) > 0 {
			resp["triggerData"] = td
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetExecutionNodes(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if h.orchClient == nil {
		http.Error(w, "orchestration client not configured", http.StatusInternalServerError)
		return
	}
	nodes, err := h.orchClient.ListNodeExecutions(id)
	if err != nil {
		http.Error(w, "Failed to fetch node executions: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}

// ExecuteWorkflow starts a run from the admin UI (Manual Trigger / Test) without HMAC.
func (h *Handler) ExecuteWorkflow(w http.ResponseWriter, r *http.Request) {
	if h.publisher == nil {
		http.Error(w, "publisher not configured", http.StatusInternalServerError)
		return
	}
	id := chi.URLParam(r, "id")
	pgID, err := normalizeUUIDParam(id)
	if err != nil {
		http.Error(w, "Invalid workflow ID format", http.StatusBadRequest)
		return
	}

	var req struct {
		TriggerData    map[string]interface{} `json:"triggerData"`
		IdempotencyKey string                 `json:"idempotencyKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.TriggerData == nil {
		req.TriggerData = map[string]interface{}{}
	}
	idempKey := req.IdempotencyKey
	if idempKey == "" {
		idempKey = "manual-" + generateRandomString(16)
	}

	wv, err := h.store.GetLatestWorkflowVersion(r.Context(), pgID)
	if err != nil {
		http.Error(w, "Workflow version not found", http.StatusNotFound)
		return
	}

	workerDAG, err := StripUIFromDAG(wv.DagDefinition)
	if err != nil {
		http.Error(w, "Invalid DAG: "+err.Error(), http.StatusBadRequest)
		return
	}

	exec, err := h.store.CreateExecution(r.Context(), db.CreateExecutionParams{
		WorkflowID:      pgID,
		WorkflowVersion: wv.Version,
		IdempotencyKey:  idempKey,
		Status:          "PENDING",
	})
	if err != nil {
		http.Error(w, "Failed to create execution (duplicate idempotency key?)", http.StatusConflict)
		return
	}

	msg := contracts.NewRunMessage{
		ExecutionID:     fmt.Sprintf("%x", exec.ID.Bytes),
		WorkflowID:      fmt.Sprintf("%x", exec.WorkflowID.Bytes),
		WorkflowVersion: int(exec.WorkflowVersion),
		TriggerData:     req.TriggerData,
		DAGDefinition:   workerDAG,
		TriggeredAt:     time.Now(),
	}
	if err := h.publisher.PublishNewRun(r.Context(), msg); err != nil {
		http.Error(w, "Failed to publish run", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"execution_id": fmt.Sprintf("%x", exec.ID.Bytes),
		"status":       "PENDING",
	})
}

func generateRandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		ret[i] = letters[num.Int64()]
	}
	return string(ret)
}

func NewStatusHandler(store *db.Store) func(context.Context, contracts.ExecutionStatusMessage) error {
	return func(ctx context.Context, msg contracts.ExecutionStatusMessage) error {
		var execID pgtype.UUID
		if err := execID.Scan(msg.ExecutionID); err != nil {
			return err
		}

		var completedAt pgtype.Timestamptz
		if msg.CompletedAt != nil {
			completedAt = pgtype.Timestamptz{Time: *msg.CompletedAt, Valid: true}
		}

		return store.UpdateExecutionStatus(ctx, db.UpdateExecutionStatusParams{
			ID:          execID,
			Status:      msg.Status,
			UpdatedAt:   pgtype.Timestamptz{Time: msg.UpdatedAt, Valid: true},
			CompletedAt: completedAt,
		})
	}
}
