package credentials

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.store.List(r.Context())
	if err != nil {
		http.Error(w, "Failed to list credentials", http.StatusInternalServerError)
		return
	}
	out := make([]map[string]interface{}, 0, len(list))
	for _, c := range list {
		out = append(out, map[string]interface{}{
			"id":        UUIDString(c.ID),
			"name":      c.Name,
			"type":      c.Type,
			"createdAt": FormatTime(c.CreatedAt),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Type   string `json:"type"`
		APIKey string `json:"apiKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Type != "sendgrid" {
		http.Error(w, "Only type \"sendgrid\" is supported", http.StatusBadRequest)
		return
	}
	c, err := h.store.CreateSendGrid(r.Context(), req.Name, req.APIKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":   UUIDString(c.ID),
		"name": c.Name,
		"type": c.Type,
	})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := ParseUUID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid credential ID", http.StatusBadRequest)
		return
	}
	if err := h.store.Delete(r.Context(), id); err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to delete", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ResolveSecret returns the raw SendGrid API key for workers (service token auth).
func (h *Handler) ResolveSecret(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-Service-Token")
	expected := os.Getenv("SERVICE_TOKEN")
	if expected == "" {
		expected = "loom-dev-service-token"
	}
	if token == "" || token != expected {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	id, err := ParseUUID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid credential ID", http.StatusBadRequest)
		return
	}
	key, err := h.store.GetSendGridAPIKey(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to resolve credential", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"apiKey": key})
}
