package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/loom/orchestration-engine/internal/db"
)

type Handler struct {
	store *db.Store
}

func NewHandler(store *db.Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) ListNodeExecutions(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	var execID pgtype.UUID
	if err := execID.Scan(idStr); err != nil {
		http.Error(w, "Invalid execution ID", http.StatusBadRequest)
		return
	}

	nodes, err := h.store.ListAllNodeExecutions(r.Context(), execID)
	if err != nil {
		http.Error(w, "Failed to list node executions", http.StatusInternalServerError)
		return
	}

	type nodeResp struct {
		NodeID       string          `json:"nodeId"`
		Status       string          `json:"status"`
		AttemptCount int32           `json:"attemptCount"`
		ErrorMessage string          `json:"errorMessage,omitempty"`
		Output       json.RawMessage `json:"output,omitempty"`
		StartedAt    any             `json:"startedAt,omitempty"`
		CompletedAt  any             `json:"completedAt,omitempty"`
	}
	out := make([]nodeResp, 0, len(nodes))
	for _, n := range nodes {
		resp := nodeResp{
			NodeID:       n.NodeID,
			Status:       n.Status,
			AttemptCount: n.AttemptCount,
		}
		if n.ErrorMessage.Valid {
			resp.ErrorMessage = n.ErrorMessage.String
		}
		if len(n.OutputData) > 0 {
			resp.Output = json.RawMessage(n.OutputData)
		}
		if n.StartedAt.Valid {
			resp.StartedAt = n.StartedAt.Time
		}
		if n.CompletedAt.Valid {
			resp.CompletedAt = n.CompletedAt.Time
		}
		out = append(out, resp)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) GetExecution(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	var execID pgtype.UUID
	if err := execID.Scan(idStr); err != nil {
		http.Error(w, "Invalid execution ID", http.StatusBadRequest)
		return
	}

	run, err := h.store.GetWorkflowRun(r.Context(), execID)
	if err != nil {
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	}

	var triggerData interface{}
	if len(run.TriggerData) > 0 {
		if err := json.Unmarshal(run.TriggerData, &triggerData); err != nil {
			triggerData = json.RawMessage(run.TriggerData)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"executionId": idStr,
		"status":      run.Status,
		"triggerData": triggerData,
	})
}
