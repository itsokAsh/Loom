package contracts

import (
	"encoding/json"
	"time"
)

type DAGDefinition struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Node struct {
	ID     string                 `json:"id"`
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
}

type Edge struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Condition string `json:"condition,omitempty"`
}

// Sent by Trigger/API -> Orchestration
type NewRunMessage struct {
	ExecutionID     string                 `json:"execution_id"`
	WorkflowID      string                 `json:"workflow_id"`
	WorkflowVersion int                    `json:"workflow_version"`
	TriggerData     map[string]interface{} `json:"trigger_data"` // Webhook payload or cron trigger time
	DAGDefinition   json.RawMessage        `json:"dag_definition"` // snapshot to persist verbatim
	TriggeredAt     time.Time              `json:"triggered_at"`
}

// Published to Node Worker Pool via "orchestration-to-worker"
type NodeTaskMessage struct {
	ExecutionID  string          `json:"execution_id"`
	NodeID       string          `json:"node_id"`
	DispatchID   string          `json:"dispatch_id"` // unique per dispatch attempt, for dedup on the worker side
	AttemptCount int             `json:"attempt_count"`
	NodeType     string          `json:"node_type"` // "HTTP", "TRANSFORM", "DELAY", etc.
	Config       json.RawMessage `json:"config"`    // fully interpolated via expr before publish
}

// Consumed from Node Worker Pool via "worker-to-orchestration"
type NodeResultMessage struct {
	ExecutionID  string          `json:"execution_id"`
	NodeID       string          `json:"node_id"`
	DispatchID   string          `json:"dispatch_id"` // must match the dispatched_tasks row to be accepted
	Status       string          `json:"status"`      // "SUCCESS" or "ERROR"
	OutputData   json.RawMessage `json:"output_data,omitempty"`
	ErrorMessage string          `json:"error_message,omitempty"`
	CompletedAt  time.Time       `json:"completed_at"`
}

// Published to Trigger/API via "orchestration-to-trigger-status"
type ExecutionStatusMessage struct {
	ExecutionID string     `json:"execution_id"`
	Status      string     `json:"status"` // "RUNNING", "COMPLETED", "FAILED", "CANCELLED"
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
