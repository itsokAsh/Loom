package workflows

import (
	"encoding/json"
	"fmt"

	contracts "github.com/loom/shared/queue-contracts"
	"github.com/loom/trigger-api/internal/templates"
)

// StripUIFromDAG extracts worker-only DAG from a saved document that may include a "ui" field
// and start node types (MANUAL/WEBHOOK/SCHEDULE/TRIGGER).
func StripUIFromDAG(dagBytes []byte) (json.RawMessage, error) {
	if len(dagBytes) == 0 {
		return nil, fmt.Errorf("dag is required")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(dagBytes, &raw); err != nil {
		return nil, fmt.Errorf("invalid dag JSON: %w", err)
	}
	var dag contracts.DAGDefinition
	if nodesRaw, ok := raw["nodes"]; ok {
		if err := json.Unmarshal(nodesRaw, &dag.Nodes); err != nil {
			return nil, err
		}
	}
	if edgesRaw, ok := raw["edges"]; ok {
		if err := json.Unmarshal(edgesRaw, &dag.Edges); err != nil {
			return nil, err
		}
	}

	workerNodes := make([]contracts.Node, 0, len(dag.Nodes))
	workerIDs := make(map[string]bool)
	for _, n := range dag.Nodes {
		switch n.Type {
		case "EMAIL", "HTTP", "TRANSFORM":
			cfg := n.Config
			if cfg != nil {
				delete(cfg, "apiKey")
				delete(cfg, "_sendgrid_api_key")
			}
			workerNodes = append(workerNodes, contracts.Node{ID: n.ID, Type: n.Type, Config: cfg})
			workerIDs[n.ID] = true
		}
	}
	workerEdges := make([]contracts.Edge, 0, len(dag.Edges))
	for _, e := range dag.Edges {
		if workerIDs[e.Source] && workerIDs[e.Target] {
			workerEdges = append(workerEdges, e)
		}
	}
	if len(workerNodes) == 0 {
		return nil, fmt.Errorf("dag must contain at least one action node (EMAIL, HTTP, or TRANSFORM)")
	}
	out, err := json.Marshal(contracts.DAGDefinition{Nodes: workerNodes, Edges: workerEdges})
	return out, err
}

// ValidateDAGJSON validates a raw DAG JSON blob for security rules and basic structure.
func ValidateDAGJSON(dagBytes []byte) error {
	workerDAG, err := StripUIFromDAG(dagBytes)
	if err != nil {
		return err
	}
	var dag contracts.DAGDefinition
	if err := json.Unmarshal(workerDAG, &dag); err != nil {
		return fmt.Errorf("invalid dag JSON: %w", err)
	}
	ids := make(map[string]bool)
	for _, n := range dag.Nodes {
		if n.ID == "" {
			return fmt.Errorf("node id is required")
		}
		if ids[n.ID] {
			return fmt.Errorf("duplicate node id %q", n.ID)
		}
		ids[n.ID] = true
		if n.Type == "" {
			return fmt.Errorf("node %q: type is required", n.ID)
		}
		switch n.Type {
		case "EMAIL", "HTTP", "TRANSFORM":
		default:
			return fmt.Errorf("node %q: unsupported type %q", n.ID, n.Type)
		}
	}
	for _, e := range dag.Edges {
		if !ids[e.Source] || !ids[e.Target] {
			return fmt.Errorf("edge references unknown node")
		}
	}
	if hasCycleContract(dag) {
		return fmt.Errorf("dag must not contain cycles")
	}
	tplDAG := templates.WorkflowDAG{
		Nodes: make([]templates.Node, len(dag.Nodes)),
		Edges: make([]templates.Edge, len(dag.Edges)),
	}
	for i, n := range dag.Nodes {
		tplDAG.Nodes[i] = templates.Node{ID: n.ID, Type: n.Type, Config: n.Config}
	}
	for i, e := range dag.Edges {
		tplDAG.Edges[i] = templates.Edge{Source: e.Source, Target: e.Target, Condition: e.Condition}
	}
	return templates.ValidateDAG(tplDAG)
}

func hasCycleContract(dag contracts.DAGDefinition) bool {
	adj := make(map[string][]string)
	for _, e := range dag.Edges {
		adj[e.Source] = append(adj[e.Source], e.Target)
	}
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	var dfs func(string) bool
	dfs = func(n string) bool {
		if inStack[n] {
			return true
		}
		if visited[n] {
			return false
		}
		visited[n] = true
		inStack[n] = true
		for _, next := range adj[n] {
			if dfs(next) {
				return true
			}
		}
		inStack[n] = false
		return false
	}
	for _, node := range dag.Nodes {
		if dfs(node.ID) {
			return true
		}
	}
	return false
}
