package templates

import (
	"testing"
)

func TestSanitizeValue(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		{
			name:    "valid value",
			key:     "app_name",
			value:   "MyApp",
			wantErr: false,
		},
		{
			name:    "CRLF in value (header injection attempt)",
			key:     "app_name",
			value:   "MyApp\r\nBcc: hacker@evil.com",
			wantErr: true,
		},
		{
			name:    "newline only",
			key:     "sendgrid_from_email",
			value:   "test@example.com\nBcc: hacker@evil.com",
			wantErr: true,
		},
		{
			name:    "carriage return only",
			key:     "admin_email",
			value:   "admin@example.com\rX-Evil: header",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sanitizeValue(tt.key, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("sanitizeValue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTemplate(t *testing.T) {
	tests := []struct {
		name    string
		template Template
		wantErr bool
	}{
		{
			name: "valid template - from uses config",
			template: Template{
				WorkflowDAG: WorkflowDAG{
					Nodes: []Node{
						{
							ID:   "send_email",
							Type: "EMAIL",
							Config: map[string]interface{}{
								"from": "{{config.sendgrid_from_email}}",
								"to":   "{{trigger.email}}",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "INVALID - from uses trigger data (spoofing risk)",
			template: Template{
				WorkflowDAG: WorkflowDAG{
					Nodes: []Node{
						{
							ID:   "send_email",
							Type: "EMAIL",
							Config: map[string]interface{}{
								"from": "{{trigger.from_email}}",
								"to":   "{{trigger.email}}",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid HTTP node - url uses config",
			template: Template{
				WorkflowDAG: WorkflowDAG{
					Nodes: []Node{
						{
							ID:   "call_api",
							Type: "HTTP",
							Config: map[string]interface{}{
								"url":    "{{config.api_url}}",
								"method": "POST",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "INVALID - HTTP url uses trigger data (SSRF risk)",
			template: Template{
				WorkflowDAG: WorkflowDAG{
					Nodes: []Node{
						{
							ID:   "call_api",
							Type: "HTTP",
							Config: map[string]interface{}{
								"url":    "{{trigger.callback_url}}",
								"method": "POST",
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTemplate(tt.template)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTemplate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInterpolateConfig(t *testing.T) {
	tests := []struct {
		name    string
		dag     WorkflowDAG
		config  map[string]string
		wantErr bool
		check   func(WorkflowDAG) bool
	}{
		{
			name: "valid config interpolation",
			dag: WorkflowDAG{
				Nodes: []Node{
					{
						ID:   "send_email",
						Type: "EMAIL",
						Config: map[string]interface{}{
							"from":    "{{config.sendgrid_from_email}}",
							"subject": "Welcome to {{config.app_name}}!",
						},
					},
				},
			},
			config: map[string]string{
				"sendgrid_from_email": "noreply@myapp.com",
				"app_name":            "MyApp",
			},
			wantErr: false,
			check: func(dag WorkflowDAG) bool {
				from := dag.Nodes[0].Config["from"].(string)
				subject := dag.Nodes[0].Config["subject"].(string)
				return from == "noreply@myapp.com" && subject == "Welcome to MyApp!"
			},
		},
		{
			name: "unknown config keys are silently dropped",
			dag: WorkflowDAG{
				Nodes: []Node{
					{
						ID:   "send_email",
						Type: "EMAIL",
						Config: map[string]interface{}{
							"from": "{{config.sendgrid_from_email}}",
							"note": "{{config.evil_key}}", // Should remain literal
						},
					},
				},
			},
			config: map[string]string{
				"sendgrid_from_email": "noreply@myapp.com",
				"evil_key":            "malicious_value", // Not in allowlist
			},
			wantErr: false,
			check: func(dag WorkflowDAG) bool {
				// evil_key should NOT be interpolated - placeholder should remain
				note := dag.Nodes[0].Config["note"].(string)
				return note == "{{config.evil_key}}"
			},
		},
		{
			name: "CRLF in config value is rejected",
			dag: WorkflowDAG{
				Nodes: []Node{
					{
						ID:   "send_email",
						Type: "EMAIL",
						Config: map[string]interface{}{
							"from": "{{config.sendgrid_from_email}}",
						},
					},
				},
			},
			config: map[string]string{
				"sendgrid_from_email": "test@example.com\r\nBcc: hacker@evil.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := interpolateConfig(tt.dag, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("interpolateConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.check != nil {
				if !tt.check(result) {
					t.Errorf("interpolateConfig() result check failed")
				}
			}
		})
	}
}

func TestConfigFingerprint(t *testing.T) {
	tests := []struct {
		name        string
		templateID1 string
		templateID2 string
		config1     map[string]string
		config2     map[string]string
		shouldMatch bool
	}{
		{
			name:        "identical configs produce same fingerprint",
			templateID1: "welcome-email",
			templateID2: "welcome-email",
			config1: map[string]string{
				"sendgrid_from_email": "test@example.com",
				"app_name":            "MyApp",
			},
			config2: map[string]string{
				"sendgrid_from_email": "test@example.com",
				"app_name":            "MyApp",
			},
			shouldMatch: true,
		},
		{
			name:        "key order doesn't matter (deterministic)",
			templateID1: "welcome-email",
			templateID2: "welcome-email",
			config1: map[string]string{
				"app_name":            "MyApp",
				"sendgrid_from_email": "test@example.com",
			},
			config2: map[string]string{
				"sendgrid_from_email": "test@example.com",
				"app_name":            "MyApp",
			},
			shouldMatch: true,
		},
		{
			name:        "different values produce different fingerprints",
			templateID1: "welcome-email",
			templateID2: "welcome-email",
			config1: map[string]string{
				"sendgrid_from_email": "test1@example.com",
				"app_name":            "MyApp",
			},
			config2: map[string]string{
				"sendgrid_from_email": "test2@example.com",
				"app_name":            "MyApp",
			},
			shouldMatch: false,
		},
		{
			name:        "different template IDs produce different fingerprints",
			templateID1: "welcome-email",
			templateID2: "password-reset",
			config1: map[string]string{
				"sendgrid_from_email": "test@example.com",
				"app_name":            "MyApp",
			},
			config2: map[string]string{
				"sendgrid_from_email": "test@example.com",
				"app_name":            "MyApp",
			},
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp1 := configFingerprint(tt.templateID1, tt.config1)
			fp2 := configFingerprint(tt.templateID2, tt.config2)

			if tt.shouldMatch && fp1 != fp2 {
				t.Errorf("fingerprints should match but don't: %s != %s", fp1, fp2)
			}
			if !tt.shouldMatch && fp1 == fp2 {
				t.Errorf("fingerprints should differ but match: %s == %s", fp1, fp2)
			}
		})
	}
}

func TestSanitizeTriggerField(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		value     string
		wantErr   bool
	}{
		{
			name:      "valid email",
			fieldType: "email",
			value:     "test@example.com",
			wantErr:   false,
		},
		{
			name:      "invalid email (no @)",
			fieldType: "email",
			value:     "not-an-email",
			wantErr:   true,
		},
		{
			name:      "email with CRLF",
			fieldType: "email",
			value:     "test@example.com\r\nBcc: hacker@evil.com",
			wantErr:   true,
		},
		{
			name:      "valid HTTPS URL",
			fieldType: "url",
			value:     "https://example.com/reset?token=abc123",
			wantErr:   false,
		},
		{
			name:      "valid HTTP URL",
			fieldType: "url",
			value:     "http://example.com/callback",
			wantErr:   false,
		},
		{
			name:      "invalid URL (no scheme)",
			fieldType: "url",
			value:     "example.com/path",
			wantErr:   true,
		},
		{
			name:      "URL with CRLF",
			fieldType: "url",
			value:     "https://example.com\r\nX-Evil: header",
			wantErr:   true,
		},
		{
			name:      "valid string",
			fieldType: "string",
			value:     "Some normal text",
			wantErr:   false,
		},
		{
			name:      "string with CRLF",
			fieldType: "string",
			value:     "Text\r\nwith newlines",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SanitizeTriggerField(tt.fieldType, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeTriggerField() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}


func TestContainsUserInput(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{
			name:  "lowercase trigger placeholder",
			value: "{{trigger.email}}",
			want:  true,
		},
		{
			name:  "trigger in middle of string",
			value: "Hello {{trigger.name}}, welcome!",
			want:  true,
		},
		{
			name:  "uppercase TRIGGER should not match (case-sensitive)",
			value: "{{TRIGGER.email}}",
			want:  false,
		},
		{
			name:  "config with trigger in key name (false positive check)",
			value: "{{config.trigger_name}}",
			want:  false,
		},
		{
			name:  "plain text no placeholders",
			value: "plain text",
			want:  false,
		},
		{
			name:  "config placeholder only",
			value: "{{config.app_name}}",
			want:  false,
		},
		{
			name:  "multiple trigger placeholders",
			value: "{{trigger.first}} and {{trigger.second}}",
			want:  true,
		},
		{
			name:  "non-string value",
			value: 12345,
			want:  false,
		},
		{
			name:  "nil value",
			value: nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsUserInput(tt.value)
			if got != tt.want {
				t.Errorf("containsUserInput(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
