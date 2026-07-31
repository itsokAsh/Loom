package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/loom/node-worker-pool/internal/executor"
	"github.com/loom/node-worker-pool/internal/secrets"
)

func init() {
	Register("EMAIL", &EmailExecutor{})
}

// EmailExecutor sends emails via SendGrid with security and idempotency
type EmailExecutor struct {
	engine      *executor.HardenedHTTPEngine
	secretStore secrets.SecretStore
}

// EmailConfig defines the email node configuration
type EmailConfig struct {
	To           string   `json:"to"`
	Subject      string   `json:"subject"`
	Body         string   `json:"body"`
	From         string   `json:"from,omitempty"`
	CC           []string `json:"cc,omitempty"`
	BCC          []string `json:"bcc,omitempty"`
	CredentialID string   `json:"credentialId,omitempty"`
}

// SendGridPayload represents the SendGrid API request format
type SendGridPayload struct {
	Personalizations []Personalization `json:"personalizations"`
	From             EmailAddress      `json:"from"`
	Subject          string            `json:"subject"`
	Content          []Content         `json:"content"`
}

type Personalization struct {
	To  []EmailAddress `json:"to"`
	CC  []EmailAddress `json:"cc,omitempty"`
	BCC []EmailAddress `json:"bcc,omitempty"`
}

type EmailAddress struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type Content struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Execute sends an email via SendGrid
func (e *EmailExecutor) Execute(ctx context.Context, config json.RawMessage) (json.RawMessage, error) {
	// Initialize dependencies if needed
	if e.engine == nil {
		e.engine = executor.NewHardenedHTTPEngine()
	}
	if e.secretStore == nil {
		e.secretStore = secrets.NewEnvSecretStore()
	}

	// Parse email configuration
	var emailConfig EmailConfig
	if err := json.Unmarshal(config, &emailConfig); err != nil {
		return nil, fmt.Errorf("invalid email config: %w", err)
	}

	// Validate email configuration
	if err := e.validateEmailConfig(emailConfig); err != nil {
		return nil, fmt.Errorf("email validation failed: %w", err)
	}

	// Get SendGrid API key: credentialId from Loom UI, else env SENDGRID_API_KEY
	apiKey, err := resolveSendGridAPIKey(ctx, emailConfig.CredentialID, e.secretStore)
	if err != nil {
		return nil, fmt.Errorf("failed to get SendGrid API key: %w", err)
	}

	// Build SendGrid payload
	payload := e.buildSendGridPayload(emailConfig)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SendGrid payload: %w", err)
	}

	// Prepare HTTP request
	req := executor.Request{
		URL:    "https://api.sendgrid.com/v3/mail/send",
		Method: "POST",
		Headers: map[string]string{
			"Authorization": "Bearer " + apiKey,
			"Content-Type":  "application/json",
		},
		Body:    payloadBytes,
		Timeout: 30 * time.Second,
	}

	// Execute request via hardened HTTP engine
	resp, err := e.engine.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("SendGrid API request failed: %w", err)
	}

	// Check response status
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("SendGrid API error: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	// Extract message ID from response headers
	messageID := ""
	if ids, ok := resp.Headers["X-Message-Id"]; ok && len(ids) > 0 {
		messageID = ids[0]
	}

	// Return success result
	result := map[string]interface{}{
		"status":              "sent",
		"sendgrid_message_id": messageID,
		"timestamp":           time.Now().Format(time.RFC3339),
	}

	return json.Marshal(result)
}

// validateEmailConfig validates the email configuration
func (e *EmailExecutor) validateEmailConfig(config EmailConfig) error {
	// Validate recipient
	if config.To == "" {
		return fmt.Errorf("recipient email is required")
	}
	if !isValidEmail(config.To) {
		return fmt.Errorf("invalid recipient email: %s", config.To)
	}

	// Validate subject
	if config.Subject == "" {
		return fmt.Errorf("subject is required")
	}
	if len(config.Subject) > 200 {
		return fmt.Errorf("subject exceeds max length of 200 characters")
	}
	if containsCRLF(config.Subject) {
		return fmt.Errorf("subject contains invalid characters (CRLF)")
	}

	// Validate body
	if config.Body == "" {
		return fmt.Errorf("body is required")
	}
	if len(config.Body) > 50000 {
		return fmt.Errorf("body exceeds max length of 50,000 characters")
	}

	// Validate from address (if provided)
	if config.From != "" && !isValidEmail(config.From) {
		return fmt.Errorf("invalid from email: %s", config.From)
	}

	// Validate CC/BCC recipients
	totalRecipients := 1 // To
	for _, cc := range config.CC {
		if !isValidEmail(cc) {
			return fmt.Errorf("invalid CC email: %s", cc)
		}
		totalRecipients++
	}
	for _, bcc := range config.BCC {
		if !isValidEmail(bcc) {
			return fmt.Errorf("invalid BCC email: %s", bcc)
		}
		totalRecipients++
	}

	if totalRecipients > 10 {
		return fmt.Errorf("too many recipients (max 10, got %d)", totalRecipients)
	}

	return nil
}

// buildSendGridPayload creates the SendGrid API payload
func (e *EmailExecutor) buildSendGridPayload(config EmailConfig) SendGridPayload {
	// Default from address
	fromEmail := config.From
	if fromEmail == "" {
		fromEmail = "noreply@loom.com"
	}

	// Build personalizations
	personalization := Personalization{
		To: []EmailAddress{{Email: config.To}},
	}

	for _, cc := range config.CC {
		personalization.CC = append(personalization.CC, EmailAddress{Email: cc})
	}

	for _, bcc := range config.BCC {
		personalization.BCC = append(personalization.BCC, EmailAddress{Email: bcc})
	}

	return SendGridPayload{
		Personalizations: []Personalization{personalization},
		From:             EmailAddress{Email: fromEmail},
		Subject:          config.Subject,
		Content: []Content{
			{
				Type:  "text/plain",
				Value: config.Body,
			},
		},
	}
}

// isValidEmail validates email format using Go's mail package
func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// containsCRLF checks if string contains CRLF characters
func containsCRLF(s string) bool {
	return strings.Contains(s, "\r") || strings.Contains(s, "\n")
}

func resolveSendGridAPIKey(ctx context.Context, credentialID string, store secrets.SecretStore) (string, error) {
	if credentialID != "" {
		base := os.Getenv("TRIGGER_API_URL")
		if base == "" {
			base = "http://trigger-api:8080"
		}
		token := os.Getenv("SERVICE_TOKEN")
		if token == "" {
			token = "loom-dev-service-token"
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/internal/credentials/"+credentialID, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("X-Service-Token", token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("credential lookup failed: %w", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("credential lookup status %d: %s", resp.StatusCode, string(body))
		}
		var out struct {
			APIKey string `json:"apiKey"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			return "", err
		}
		if out.APIKey == "" {
			return "", fmt.Errorf("empty api key from credential")
		}
		return out.APIKey, nil
	}
	return store.Get("SENDGRID_API_KEY")
}
