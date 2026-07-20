package templates

import "time"

// Template represents a pre-built workflow template
type Template struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Category       string          `json:"category"` // email, webhook, notification
	Version        int             `json:"version"`  // bumped whenever the built-in DAG changes
	RequiredFields []TemplateField `json:"required_fields"`
	WorkflowDAG    WorkflowDAG     `json:"workflow_dag"`
	CreatedAt      time.Time       `json:"created_at"`
}

// TemplateField describes a required field for a template
type TemplateField struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // string, email, url, array
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// WorkflowDAG represents the workflow structure
type WorkflowDAG struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Node represents a workflow node
type Node struct {
	ID     string                 `json:"id"`
	Type   string                 `json:"type"` // EMAIL, HTTP, TRANSFORM, etc.
	Config map[string]interface{} `json:"config"`
}

// Edge represents a connection between nodes
type Edge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Condition string `json:"condition,omitempty"`
}
