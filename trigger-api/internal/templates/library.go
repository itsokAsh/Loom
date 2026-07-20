package templates

import (
	"fmt"
	"time"
)

// BuiltInTemplates contains all pre-defined workflow templates
var BuiltInTemplates = []Template{
	{
		ID:          "welcome-email",
		Name:        "Welcome Email",
		Description: "Send a welcome email to new users upon signup",
		Category:    "email",
		Version:     1,
		CreatedAt:   time.Now(),
		RequiredFields: []TemplateField{
			{
				Name:        "email",
				Type:        "email",
				Required:    true,
				Description: "Recipient email address",
				Example:     "user@example.com",
			},
			{
				Name:        "name",
				Type:        "string",
				Required:    true,
				Description: "User's full name",
				Example:     "John Doe",
			},
		},
		WorkflowDAG: WorkflowDAG{
			Nodes: []Node{
				{
					ID:   "send_welcome",
					Type: "EMAIL",
					Config: map[string]interface{}{
						"to":      "{{trigger.email}}",
						"from":    "{{config.sendgrid_from_email}}",
						"subject": "Welcome to {{config.app_name}}!",
						"body":    "Hi {{trigger.name}},\n\nWelcome aboard! We're thrilled to have you.\n\nBest,\nThe {{config.app_name}} Team",
					},
				},
			},
			Edges: []Edge{},
		},
	},
	{
		ID:          "password-reset",
		Name:        "Password Reset",
		Description: "Send password reset link to users",
		Category:    "email",
		Version:     1,
		CreatedAt:   time.Now(),
		RequiredFields: []TemplateField{
			{
				Name:        "email",
				Type:        "email",
				Required:    true,
				Description: "User's email address",
				Example:     "user@example.com",
			},
			{
				Name:        "reset_link",
				Type:        "url",
				Required:    true,
				Description: "Password reset link with token",
				Example:     "https://app.com/reset?token=abc123",
			},
		},
		WorkflowDAG: WorkflowDAG{
			Nodes: []Node{
				{
					ID:   "send_reset_email",
					Type: "EMAIL",
					Config: map[string]interface{}{
						"to":      "{{trigger.email}}",
						"from":    "{{config.sendgrid_from_email}}",
						"subject": "Reset Your Password",
						"body":    "Click here to reset your password:\n\n{{trigger.reset_link}}\n\nLink expires in 1 hour.",
					},
				},
			},
			Edges: []Edge{},
		},
	},
	{
		ID:          "admin-notification",
		Name:        "Admin Notification",
		Description: "Send critical alerts to admin",
		Category:    "email",
		Version:     1,
		CreatedAt:   time.Now(),
		RequiredFields: []TemplateField{
			{
				Name:        "event_type",
				Type:        "string",
				Required:    true,
				Description: "Type of event (signup, error, payment)",
				Example:     "new_signup",
			},
			{
				Name:        "details",
				Type:        "string",
				Required:    true,
				Description: "Event details",
				Example:     "User john@example.com signed up",
			},
		},
		WorkflowDAG: WorkflowDAG{
			Nodes: []Node{
				{
					ID:   "notify_admin",
					Type: "EMAIL",
					Config: map[string]interface{}{
						"to":      "{{config.admin_email}}",
						"from":    "{{config.sendgrid_from_email}}",
						"subject": "Admin Alert: {{trigger.event_type}}",
						"body":    "Event: {{trigger.event_type}}\n\nDetails:\n{{trigger.details}}",
					},
				},
			},
			Edges: []Edge{},
		},
	},
}

// GetTemplateByID retrieves a template by its ID
func GetTemplateByID(id string) (*Template, error) {
	for i := range BuiltInTemplates {
		if BuiltInTemplates[i].ID == id {
			return &BuiltInTemplates[i], nil
		}
	}
	return nil, fmt.Errorf("template not found: %s", id)
}

// ListTemplates returns all templates, optionally filtered by category
func ListTemplates(category string) []Template {
	if category == "" {
		return BuiltInTemplates
	}
	filtered := []Template{}
	for _, t := range BuiltInTemplates {
		if t.Category == category {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
