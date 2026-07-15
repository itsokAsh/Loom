package contracts

// DAGDefinition represents the parsed JSON of nodes and edges for a workflow.
type DAGDefinition struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Node struct {
	ID     string                 `json:"id"`
	Type   string                 `json:"type"` // "HTTP", "TRANSFORM", "DELAY"
	Config map[string]interface{} `json:"config"`
	Retry  *RetryPolicy           `json:"retry,omitempty"`
}

type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type RetryPolicy struct {
	MaxRetries     int `json:"maxRetries"`
	TimeoutSeconds int `json:"timeoutSeconds"`
}

// Sent by Trigger/API -> Orchestration
type NewRunMessage struct {
	ExecutionID     string                 `json:"executionId"`
	WorkflowID      string                 `json:"workflowId"`
	WorkflowVersion int                    `json:"workflowVersion"`
	IdempotencyKey  string                 `json:"idempotencyKey"`
	TriggerData     map[string]interface{} `json:"triggerData"` // Webhook payload or cron trigger time
	WorkflowDAG     DAGDefinition          `json:"workflowDag"`
}

// Sent by Orchestration -> Worker
type NodeTaskMessage struct {
	ExecutionID string                 `json:"executionId"`
	NodeID      string                 `json:"nodeId"`
	NodeType    string                 `json:"nodeType"` // "HTTP", "TRANSFORM", "DELAY"
	InputData   map[string]interface{} `json:"inputData"`
	Config      map[string]interface{} `json:"config"`
}

// Sent by Worker -> Orchestration
type NodeResultMessage struct {
	ExecutionID string                 `json:"executionId"`
	NodeID      string                 `json:"nodeId"`
	Status      string                 `json:"status"` // "SUCCESS", "ERROR"
	OutputData  map[string]interface{} `json:"outputData,omitempty"`
	Error       string                 `json:"error,omitempty"`
}
