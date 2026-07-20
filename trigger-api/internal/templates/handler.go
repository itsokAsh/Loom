package templates

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Store interface defines required database operations
type Store interface {
	FindWorkflowByFingerprint(ctx context.Context, fingerprint string) (*WorkflowWithWebhook, bool, error)
	CreateWorkflowFromTemplate(ctx context.Context, params CreateWorkflowFromTemplateParams) (string, error)
	CreateWebhook(ctx context.Context, params CreateWebhookParams) (*WebhookResponse, error)
	GetWebhookByWorkflowID(ctx context.Context, workflowID string) (*WebhookResponse, error)
}

// WorkflowWithWebhook represents a workflow with its webhook
type WorkflowWithWebhook struct {
	ID string
}

// CreateWorkflowFromTemplateParams contains params for creating a workflow from template
type CreateWorkflowFromTemplateParams struct {
	Name            string
	Fingerprint     string
	TemplateID      string
	TemplateVersion int
	DAG             []byte
}

// CreateWebhookParams contains params for creating a webhook
type CreateWebhookParams struct {
	WorkflowID string
	Path       string
	Secret     string
}

// WebhookResponse represents webhook data
type WebhookResponse struct {
	ID     string
	Path   string
	Secret string
}

// TemplateHandler handles template-related HTTP requests
type TemplateHandler struct {
	store   Store
	baseURL string
}

// NewTemplateHandler creates a new template handler
func NewTemplateHandler(store Store, baseURL string) *TemplateHandler {
	return &TemplateHandler{
		store:   store,
		baseURL: baseURL,
	}
}

// ListTemplates handles GET /v1/templates
func (h *TemplateHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	templates := ListTemplates(category)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"templates": templates,
	})
}

// CreateFromTemplate handles POST /v1/templates/{template_id}/create
func (h *TemplateHandler) CreateFromTemplate(w http.ResponseWriter, r *http.Request) {
	templateID := chi.URLParam(r, "template_id")

	template, err := GetTemplateByID(templateID)
	if err != nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	if err := validateTemplate(*template); err != nil {
		// A built-in template should never fail this, but a future custom
		// template must go through the same check.
		http.Error(w, "Template failed security validation: "+err.Error(), http.StatusBadRequest)
		return
	}

	var req struct {
		Config map[string]string `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Idempotency: reuse the existing workflow for this template+config
	// fingerprint rather than creating a duplicate workflow/webhook pair
	// every time initTemplate() runs.
	fingerprint := configFingerprint(templateID, req.Config)
	if existing, ok, err := h.store.FindWorkflowByFingerprint(r.Context(), fingerprint); err == nil && ok {
		webhook, err := h.store.GetWebhookByWorkflowID(r.Context(), existing.ID)
		if err == nil {
			h.writeWebhookResponse(w, existing.ID, webhook)
			return
		}
	}

	dag, err := interpolateConfig(template.WorkflowDAG, req.Config)
	if err != nil {
		http.Error(w, "Config interpolation failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	dagBytes, err := json.Marshal(dag)
	if err != nil {
		http.Error(w, "Failed to marshal DAG", http.StatusInternalServerError)
		return
	}

	workflowID, err := h.store.CreateWorkflowFromTemplate(r.Context(), CreateWorkflowFromTemplateParams{
		Name:            template.Name,
		Fingerprint:     fingerprint,
		TemplateID:      template.ID,
		TemplateVersion: template.Version,
		DAG:             dagBytes,
	})
	if err != nil {
		http.Error(w, "Failed to create workflow", http.StatusInternalServerError)
		return
	}

	// Each webhook gets its own secret. This is the secret the SDK must
	// cache and sign with — NOT a global loom.webhook_secret.
	webhook, err := h.store.CreateWebhook(r.Context(), CreateWebhookParams{
		WorkflowID: workflowID,
		Path:       generateRandomPath(),
		Secret:     generateWebhookSecret(),
	})
	if err != nil {
		http.Error(w, "Failed to create webhook", http.StatusInternalServerError)
		return
	}

	h.writeWebhookResponse(w, workflowID, webhook)
}

func (h *TemplateHandler) writeWebhookResponse(w http.ResponseWriter, workflowID string, webhook *WebhookResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workflow_id": workflowID,
		"webhook": map[string]string{
			"path":   webhook.Path,
			"secret": webhook.Secret,
			"url":    fmt.Sprintf("%s/webhooks/%s", h.baseURL, webhook.Path),
		},
	})
}

// generateRandomPath creates a random webhook path
func generateRandomPath() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:16]
}

// generateWebhookSecret creates a cryptographically secure webhook secret
func generateWebhookSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "whsec_" + base64.URLEncoding.EncodeToString(b)
}
