package templates

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Only these keys may be substituted into a workflow DAG.
// Anything else in the request's `config` map is silently dropped.
var allowedConfigKeys = map[string]bool{
	"sendgrid_from_email": true,
	"app_name":            true,
	"admin_email":         true,
	"crm_api_url":         true,
	"health_check_url":    true,
	"relay_target_url":    true,
}

// sanitizeValue rejects control characters (CRLF) that could be used for
// email header injection if a value ever lands in a header-like field
// (Subject, From, Bcc, etc.) rather than the body.
func sanitizeValue(key, value string) (string, error) {
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("config value for %q contains illegal control characters", key)
	}
	return value, nil
}

// interpolateConfig whitelists keys, sanitizes values, and substitutes
// {{config.key}} placeholders into the workflow DAG. This is the ONLY
// interpolation path — do not reintroduce a second, unguarded version.
func interpolateConfig(dag WorkflowDAG, config map[string]string) (WorkflowDAG, error) {
	clean := map[string]string{}
	for key, value := range config {
		if !allowedConfigKeys[key] {
			continue
		}
		sanitized, err := sanitizeValue(key, value)
		if err != nil {
			return WorkflowDAG{}, err
		}
		clean[key] = sanitized
	}

	dagJSON, err := json.Marshal(dag)
	if err != nil {
		return WorkflowDAG{}, err
	}
	dagStr := string(dagJSON)
	for key, value := range clean {
		placeholder := fmt.Sprintf("{{config.%s}}", key)
		dagStr = strings.ReplaceAll(dagStr, placeholder, value)
	}

	var interpolated WorkflowDAG
	if err := json.Unmarshal([]byte(dagStr), &interpolated); err != nil {
		return WorkflowDAG{}, err
	}
	return interpolated, nil
}

// ValidateDAG applies the same security rules as built-in templates to arbitrary DAGs.
func ValidateDAG(dag WorkflowDAG) error {
	return validateTemplate(Template{WorkflowDAG: dag})
}

// validateTemplate enforces structural security rules on any template
// before it can be used to create a workflow. This MUST also run on the
// custom-template creation path (future) — not just on built-ins.
func validateTemplate(t Template) error {
	for _, node := range t.WorkflowDAG.Nodes {
		switch node.Type {
		case "EMAIL":
			if containsUserInput(node.Config["from"]) {
				return fmt.Errorf("node %q: email 'from' cannot use trigger data", node.ID)
			}
		case "HTTP":
			if containsUserInput(node.Config["url"]) {
				return fmt.Errorf("node %q: HTTP url cannot use trigger data", node.ID)
			}
		}
	}
	return nil
}

// containsUserInput checks if a value contains trigger data placeholders
func containsUserInput(value interface{}) bool {
	str, ok := value.(string)
	if !ok {
		return false
	}
	return strings.Contains(str, "{{trigger.")
}

// SanitizeTriggerField applies field-type-specific escaping to
// caller-supplied (trigger) data before it is substituted into the DAG.
// HTML-escaping alone is wrong for this: it doesn't stop header injection
// and it mangles URLs.
func SanitizeTriggerField(fieldType, value string) (string, error) {
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("value contains illegal control characters")
	}
	switch fieldType {
	case "url":
		u, err := url.Parse(value)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return "", fmt.Errorf("invalid URL")
		}
		return value, nil
	case "email":
		if !strings.Contains(value, "@") {
			return "", fmt.Errorf("invalid email address")
		}
		return value, nil
	default: // "string"
		return value, nil
	}
}

// configFingerprint generates a deterministic hash for idempotent workflow creation
func configFingerprint(templateID string, config map[string]string) string {
	// Sort keys for deterministic hash
	keys := make([]string, 0, len(config))
	for k := range config {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	h.Write([]byte(templateID))
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write([]byte(config[k]))
	}
	return hex.EncodeToString(h.Sum(nil))
}
