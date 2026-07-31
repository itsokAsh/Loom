package templates

import (
	"fmt"
	"time"
)

// BuiltInTemplates contains all pre-defined workflow templates
var BuiltInTemplates = []Template{
	{
		ID:            "call-an-api",
		Name:          "Call an API",
		Description:   "Manual start → GET a public URL → shape the response. No secrets needed.",
		Category:      "automation",
		Version:       2,
		CreatedAt:     time.Now(),
		BeginnerReady: true,
		NeedsSendGrid: false,
		SetupHint:     "Save, open the Test tab, click Run. Watch green checkmarks on the canvas.",
		TriggerMode:   "manual",
		ConfigFields:  nil,
		SampleTrigger: map[string]interface{}{"note": "optional"},
		RequiredFields: []TemplateField{},
		WorkflowDAG: WorkflowDAG{
			Nodes: []Node{
				{ID: "ping", Type: "HTTP", Config: map[string]interface{}{
					"method":  "GET",
					"url":     "https://jsonplaceholder.typicode.com/todos/1",
					"headers": map[string]interface{}{},
					"body":    nil,
				}},
				{ID: "shape", Type: "TRANSFORM", Config: map[string]interface{}{
					"mapping": map[string]interface{}{
						"status": "{{ outputs.ping.status }}",
						"body":   "{{ outputs.ping.body }}",
					},
				}},
			},
			Edges: []Edge{{Source: "ping", Target: "shape"}},
		},
	},
	{
		ID:            "welcome-email",
		Name:          "Welcome Email",
		Description:   "Send a welcome email when you click Test (or later via webhook).",
		Category:      "email",
		Version:       2,
		CreatedAt:     time.Now(),
		BeginnerReady: false,
		NeedsSendGrid: true,
		SetupHint:     "Needs SendGrid: set From to a verified sender, and a SendGrid credential (or SENDGRID_API_KEY in .env).",
		TriggerMode:   "manual",
		ConfigFields: []ConfigFieldDef{
			{Key: "sendgrid_from_email", Label: "From email (SendGrid verified)", Default: "noreply@yourdomain.com", Required: true, Hint: "Must be verified in SendGrid."},
			{Key: "app_name", Label: "App name", Default: "Loom", Required: true},
		},
		SampleTrigger: map[string]interface{}{"email": "you@example.com", "name": "Ash"},
		RequiredFields: []TemplateField{
			{Name: "email", Type: "email", Required: true, Description: "Recipient email", Example: "user@example.com"},
			{Name: "name", Type: "string", Required: true, Description: "User name", Example: "John Doe"},
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
		ID:            "password-reset",
		Name:          "Password Reset",
		Description:   "Email a password reset link to a user.",
		Category:      "email",
		Version:       2,
		CreatedAt:     time.Now(),
		BeginnerReady: false,
		NeedsSendGrid: true,
		SetupHint:     "Needs SendGrid credential and a verified From address.",
		TriggerMode:   "manual",
		ConfigFields: []ConfigFieldDef{
			{Key: "sendgrid_from_email", Label: "From email", Default: "noreply@yourdomain.com", Required: true},
		},
		SampleTrigger: map[string]interface{}{
			"email":      "you@example.com",
			"reset_link": "https://app.example.com/reset?token=demo",
		},
		RequiredFields: []TemplateField{
			{Name: "email", Type: "email", Required: true, Description: "User email", Example: "user@example.com"},
			{Name: "reset_link", Type: "url", Required: true, Description: "Reset link", Example: "https://app.com/reset?token=abc123"},
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
		ID:            "admin-notification",
		Name:          "Admin Notification",
		Description:   "Email an admin when something important happens.",
		Category:      "email",
		Version:       2,
		CreatedAt:     time.Now(),
		BeginnerReady: false,
		NeedsSendGrid: true,
		SetupHint:     "Needs SendGrid and admin + From addresses.",
		TriggerMode:   "manual",
		ConfigFields: []ConfigFieldDef{
			{Key: "sendgrid_from_email", Label: "From email", Default: "noreply@yourdomain.com", Required: true},
			{Key: "admin_email", Label: "Admin email", Default: "admin@yourdomain.com", Required: true},
		},
		SampleTrigger: map[string]interface{}{"event_type": "new_signup", "details": "User joined"},
		RequiredFields: []TemplateField{
			{Name: "event_type", Type: "string", Required: true, Description: "Event type", Example: "new_signup"},
			{Name: "details", Type: "string", Required: true, Description: "Details", Example: "User signed up"},
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
	{
		ID:            "user-onboarding",
		Name:          "User Onboarding Pipeline",
		Description:   "Shape signup data, POST to a CRM URL, then send welcome email.",
		Category:      "automation",
		Version:       2,
		CreatedAt:     time.Now(),
		BeginnerReady: false,
		NeedsSendGrid: true,
		SetupHint:     "Needs a CRM URL and SendGrid. Prefer Call an API first if you are learning.",
		TriggerMode:   "manual",
		ConfigFields: []ConfigFieldDef{
			{Key: "crm_api_url", Label: "CRM API URL", Default: "https://jsonplaceholder.typicode.com/posts", Required: true, Hint: "Use jsonplaceholder to try without a real CRM."},
			{Key: "sendgrid_from_email", Label: "From email", Default: "noreply@yourdomain.com", Required: true},
			{Key: "app_name", Label: "App name", Default: "Loom", Required: true},
		},
		SampleTrigger: map[string]interface{}{"email": "you@example.com", "name": "Ash", "user_id": "usr_1"},
		RequiredFields: []TemplateField{
			{Name: "email", Type: "email", Required: true, Description: "User email", Example: "user@example.com"},
			{Name: "name", Type: "string", Required: true, Description: "User name", Example: "Jane"},
			{Name: "user_id", Type: "string", Required: true, Description: "User ID", Example: "usr_123"},
		},
		WorkflowDAG: WorkflowDAG{
			Nodes: []Node{
				{ID: "format_user", Type: "TRANSFORM", Config: map[string]interface{}{
					"mapping": map[string]interface{}{
						"email":   "{{trigger.email}}",
						"name":    "{{trigger.name}}",
						"user_id": "{{trigger.user_id}}",
					},
				}},
				{ID: "crm_sync", Type: "HTTP", Config: map[string]interface{}{
					"method":  "POST",
					"url":     "{{config.crm_api_url}}",
					"headers": map[string]interface{}{"Content-Type": "application/json"},
					"body": map[string]interface{}{
						"email":   "{{trigger.email}}",
						"name":    "{{trigger.name}}",
						"user_id": "{{trigger.user_id}}",
					},
				}},
				{ID: "welcome_email", Type: "EMAIL", Config: map[string]interface{}{
					"to":      "{{trigger.email}}",
					"from":    "{{config.sendgrid_from_email}}",
					"subject": "Welcome to {{config.app_name}}!",
					"body":    "Hi {{trigger.name}}, welcome to {{config.app_name}}!",
				}},
			},
			Edges: []Edge{
				{Source: "format_user", Target: "crm_sync"},
				{Source: "crm_sync", Target: "welcome_email"},
			},
		},
	},
	{
		ID:            "signup-dual-notify",
		Name:          "Signup + Admin Notify",
		Description:   "Welcome the user and email an admin on signup.",
		Category:      "automation",
		Version:       2,
		CreatedAt:     time.Now(),
		BeginnerReady: false,
		NeedsSendGrid: true,
		SetupHint:     "Needs SendGrid, From address, and admin email.",
		TriggerMode:   "manual",
		ConfigFields: []ConfigFieldDef{
			{Key: "sendgrid_from_email", Label: "From email", Default: "noreply@yourdomain.com", Required: true},
			{Key: "admin_email", Label: "Admin email", Default: "admin@yourdomain.com", Required: true},
		},
		SampleTrigger: map[string]interface{}{"email": "you@example.com", "name": "Ash"},
		RequiredFields: []TemplateField{
			{Name: "email", Type: "email", Required: true, Description: "User email", Example: "user@example.com"},
			{Name: "name", Type: "string", Required: true, Description: "User name", Example: "Jane"},
		},
		WorkflowDAG: WorkflowDAG{
			Nodes: []Node{
				{ID: "format", Type: "TRANSFORM", Config: map[string]interface{}{
					"mapping": map[string]interface{}{"email": "{{trigger.email}}", "name": "{{trigger.name}}"},
				}},
				{ID: "user_email", Type: "EMAIL", Config: map[string]interface{}{
					"to":      "{{trigger.email}}",
					"from":    "{{config.sendgrid_from_email}}",
					"subject": "Welcome!",
					"body":    "Hi {{trigger.name}}, thanks for signing up.",
				}},
				{ID: "admin_email", Type: "EMAIL", Config: map[string]interface{}{
					"to":      "{{config.admin_email}}",
					"from":    "{{config.sendgrid_from_email}}",
					"subject": "New signup: {{trigger.email}}",
					"body":    "User {{trigger.name}} ({{trigger.email}}) signed up.",
				}},
			},
			Edges: []Edge{
				{Source: "format", Target: "user_email"},
				{Source: "format", Target: "admin_email"},
			},
		},
	},
	{
		ID:            "api-health-check",
		Name:          "API Health Check",
		Description:   "Ping a public health URL and shape the result. No email or secrets.",
		Category:      "monitoring",
		Version:       2,
		CreatedAt:     time.Now(),
		BeginnerReady: true,
		NeedsSendGrid: false,
		SetupHint:     "Defaults to a public JSON API. Change the URL after you learn the flow.",
		TriggerMode:   "manual",
		ConfigFields: []ConfigFieldDef{
			{Key: "health_check_url", Label: "URL to ping", Default: "https://jsonplaceholder.typicode.com/todos/1", Required: true},
		},
		SampleTrigger: map[string]interface{}{},
		RequiredFields: []TemplateField{},
		WorkflowDAG: WorkflowDAG{
			Nodes: []Node{
				{ID: "ping", Type: "HTTP", Config: map[string]interface{}{
					"method": "GET",
					"url":    "{{config.health_check_url}}",
				}},
				{ID: "parse", Type: "TRANSFORM", Config: map[string]interface{}{
					"mapping": map[string]interface{}{
						"checked": true,
						"status":  "{{ outputs.ping.status }}",
						"body":    "{{ outputs.ping.body }}",
					},
				}},
			},
			Edges: []Edge{{Source: "ping", Target: "parse"}},
		},
	},
	{
		ID:            "webhook-relay",
		Name:          "Webhook to HTTP",
		Description:   "Receive a webhook, shape the payload, POST it to a URL.",
		Category:      "automation",
		Version:       2,
		CreatedAt:     time.Now(),
		BeginnerReady: true,
		NeedsSendGrid: false,
		SetupHint:     "Creates a webhook URL. Default target is jsonplaceholder so you can try immediately.",
		TriggerMode:   "webhook",
		ConfigFields: []ConfigFieldDef{
			{Key: "relay_target_url", Label: "Forward to URL", Default: "https://jsonplaceholder.typicode.com/posts", Required: true},
		},
		SampleTrigger: map[string]interface{}{"hello": "world"},
		RequiredFields: []TemplateField{},
		WorkflowDAG: WorkflowDAG{
			Nodes: []Node{
				{ID: "format", Type: "TRANSFORM", Config: map[string]interface{}{
					"mapping": map[string]interface{}{"payload": "{{trigger}}"},
				}},
				{ID: "relay", Type: "HTTP", Config: map[string]interface{}{
					"method":  "POST",
					"url":     "{{config.relay_target_url}}",
					"headers": map[string]interface{}{"Content-Type": "application/json"},
					"body":    map[string]interface{}{"data": "{{trigger}}"},
				}},
			},
			Edges: []Edge{{Source: "format", Target: "relay"}},
		},
	},
	{
		ID:            "vip-conditional",
		Name:          "VIP Conditional Branch",
		Description:   "Send VIP or standard welcome based on is_vip in the Test JSON.",
		Category:      "automation",
		Version:       2,
		CreatedAt:     time.Now(),
		BeginnerReady: false,
		NeedsSendGrid: true,
		SetupHint:     "Needs SendGrid. In Test JSON set is_vip to the string \"true\" or \"false\".",
		TriggerMode:   "manual",
		ConfigFields: []ConfigFieldDef{
			{Key: "sendgrid_from_email", Label: "From email", Default: "noreply@yourdomain.com", Required: true},
		},
		SampleTrigger: map[string]interface{}{"email": "vip@example.com", "name": "VIP User", "is_vip": "true"},
		RequiredFields: []TemplateField{
			{Name: "email", Type: "email", Required: true, Description: "User email", Example: "vip@example.com"},
			{Name: "name", Type: "string", Required: true, Description: "User name", Example: "VIP User"},
			{Name: "is_vip", Type: "string", Required: true, Description: "true or false string", Example: "true"},
		},
		WorkflowDAG: WorkflowDAG{
			Nodes: []Node{
				{ID: "format", Type: "TRANSFORM", Config: map[string]interface{}{
					"mapping": map[string]interface{}{
						"email":  "{{trigger.email}}",
						"name":   "{{trigger.name}}",
						"is_vip": "{{trigger.is_vip}}",
					},
				}},
				{ID: "vip_email", Type: "EMAIL", Config: map[string]interface{}{
					"to":      "{{trigger.email}}",
					"from":    "{{config.sendgrid_from_email}}",
					"subject": "VIP Welcome",
					"body":    "Hi {{trigger.name}}, welcome to our VIP program!",
				}},
				{ID: "standard_email", Type: "EMAIL", Config: map[string]interface{}{
					"to":      "{{trigger.email}}",
					"from":    "{{config.sendgrid_from_email}}",
					"subject": "Welcome",
					"body":    "Hi {{trigger.name}}, welcome!",
				}},
			},
			Edges: []Edge{
				{Source: "format", Target: "vip_email", Condition: `trigger.is_vip == "true"`},
				{Source: "format", Target: "standard_email", Condition: `trigger.is_vip != "true"`},
			},
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
