package workflows

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/loom/trigger-api/internal/db"
	"github.com/robfig/cron/v3"
)

type Handler struct {
	service *Service
	store   *db.Store
}

func NewHandler(service *Service, store *db.Store) *Handler {
	return &Handler{service: service, store: store}
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

	var pgID pgtype.UUID
	if err := pgID.Scan(id); err != nil {
		http.Error(w, "Invalid workflow ID format", http.StatusBadRequest)
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

func (h *Handler) AddVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var pgID pgtype.UUID
	if err := pgID.Scan(id); err != nil {
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

	var pgID pgtype.UUID
	if err := pgID.Scan(id); err != nil {
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

	var pgID pgtype.UUID
	if err := pgID.Scan(id); err != nil {
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
	json.NewEncoder(w).Encode(sched)
}

func (h *Handler) ListExecutions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var pgID pgtype.UUID
	if err := pgID.Scan(id); err != nil {
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(executions)
}

func (h *Handler) GetExecution(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var pgID pgtype.UUID
	if err := pgID.Scan(id); err != nil {
		http.Error(w, "Invalid execution ID format", http.StatusBadRequest)
		return
	}

	exec, err := h.store.GetExecution(r.Context(), pgID)
	if err != nil {
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(exec)
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
