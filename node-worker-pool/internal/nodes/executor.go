package nodes

import (
	"context"
	"encoding/json"
	"fmt"
)

// Executor defines how a specific node type runs
type Executor interface {
	Execute(ctx context.Context, config json.RawMessage) (json.RawMessage, error)
}

var registry = map[string]Executor{}

// Register adds an executor for a given node type
func Register(nodeType string, exec Executor) {
	registry[nodeType] = exec
}

// Get finds the executor for a given node type
func Get(nodeType string) (Executor, error) {
	exec, ok := registry[nodeType]
	if !ok {
		return nil, fmt.Errorf("no executor found for node type: %s", nodeType)
	}
	return exec, nil
}
