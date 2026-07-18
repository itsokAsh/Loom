package dag

import (
	"encoding/json"

	contracts "github.com/loom/shared/queue-contracts"
)

type Evaluator struct {
}

func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// NextActionableNodes determines which nodes can run next.
func (e *Evaluator) NextActionableNodes(dagDef []byte, nodeStates map[string]string) ([]contracts.Node, error) {
	var dag contracts.DAGDefinition
	if err := json.Unmarshal(dagDef, &dag); err != nil {
		return nil, err
	}
	
	nodesByID := make(map[string]contracts.Node)
	for _, n := range dag.Nodes {
		nodesByID[n.ID] = n
	}
	
	incomingEdges := make(map[string][]contracts.Edge)
	for _, edge := range dag.Edges {
		incomingEdges[edge.Target] = append(incomingEdges[edge.Target], edge)
	}

	var actionable []contracts.Node

	for _, n := range dag.Nodes {
		state, exists := nodeStates[n.ID]
		// If node has already started/completed, it's not "next"
		if exists && (state == "QUEUED" || state == "RUNNING" || state == "SUCCESS" || state == "SKIPPED" || state == "ERROR") {
			continue
		}

		parents := incomingEdges[n.ID]
		canRun := true
		for _, edge := range parents {
			parentState, parentExists := nodeStates[edge.Source]
			if !parentExists || (parentState != "SUCCESS" && parentState != "SKIPPED") {
				canRun = false
				break
			}
		}

		if canRun {
			actionable = append(actionable, n)
		}
	}

	return actionable, nil
}

// EvaluateConfig interpolates the config for a node.
func (e *Evaluator) EvaluateConfig(nodeConfig map[string]interface{}, state map[string]interface{}) (json.RawMessage, error) {
	// Stub for M1: just return JSON without expr evaluation
	return json.Marshal(nodeConfig)
}
