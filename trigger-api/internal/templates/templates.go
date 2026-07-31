package templates

import "time"

// Template represents a pre-built workflow template
type Template struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Category       string                 `json:"category"` // email, webhook, notification, automation, monitoring
	Version        int                    `json:"version"`  // bumped whenever the built-in DAG changes
	RequiredFields []TemplateField        `json:"required_fields"`
	WorkflowDAG    WorkflowDAG            `json:"workflow_dag"`
	CreatedAt      time.Time              `json:"created_at"`
	BeginnerReady  bool                   `json:"beginner_ready"`
	NeedsSendGrid  bool                   `json:"needs_sendgrid"`
	SetupHint      string                 `json:"setup_hint,omitempty"`
	TriggerMode    string                 `json:"trigger_mode"` // manual | webhook
	ConfigFields   []ConfigFieldDef       `json:"config_fields"`
	SampleTrigger  map[string]interface{} `json:"sample_trigger,omitempty"`
}

// ConfigFieldDef describes a human-labeled setup field for gallery create.
type ConfigFieldDef struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Default  string `json:"default"`
	Required bool   `json:"required"`
	Hint     string `json:"hint,omitempty"`
}

// TemplateField describes a required trigger payload field for a template
type TemplateField struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // string, email, url, array
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// WorkflowDAG represents the workflow structure (worker nodes + optional UI canvas)
type WorkflowDAG struct {
	Nodes []Node     `json:"nodes"`
	Edges []Edge     `json:"edges"`
	UI    *CanvasUI  `json:"ui,omitempty"`
}

// CanvasUI stores React Flow layout including Start nodes.
type CanvasUI struct {
	Nodes []UINode `json:"nodes"`
	Edges []UIEdge `json:"edges"`
}

// UINode is a canvas node (may be MANUAL/WEBHOOK/SCHEDULE or a worker).
type UINode struct {
	ID       string                 `json:"id"`
	Position map[string]float64     `json:"position"`
	Data     map[string]interface{} `json:"data"`
}

// UIEdge is a canvas edge (may include Start → first worker).
type UIEdge struct {
	ID     string                 `json:"id"`
	Source string                 `json:"source"`
	Target string                 `json:"target"`
	Data   map[string]interface{} `json:"data,omitempty"`
}

// Node represents a workflow worker node
type Node struct {
	ID     string                 `json:"id"`
	Type   string                 `json:"type"` // EMAIL, HTTP, TRANSFORM, etc.
	Config map[string]interface{} `json:"config"`
}

// Edge represents a connection between nodes (matches shared queue-contracts).
type Edge struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Condition string `json:"condition,omitempty"`
}
