package templates

import (
	"fmt"
	"strings"
)

const startNodeID = "start"

// attachStartUI adds dag.ui with a Manual or Webhook start node and edges to root workers.
func attachStartUI(dag WorkflowDAG, triggerMode, startLabel string) WorkflowDAG {
	startType := "MANUAL"
	if strings.EqualFold(triggerMode, "webhook") {
		startType = "WEBHOOK"
	}
	if startLabel == "" {
		if startType == "WEBHOOK" {
			startLabel = "Webhook"
		} else {
			startLabel = "Manual Trigger"
		}
	}

	incoming := map[string]bool{}
	for _, e := range dag.Edges {
		incoming[e.Target] = true
	}

	uiNodes := []UINode{
		{
			ID:       startNodeID,
			Position: map[string]float64{"x": 220, "y": 40},
			Data: map[string]interface{}{
				"label":    startLabel,
				"nodeType": startType,
				"config":   map[string]interface{}{},
			},
		},
	}

	uiEdges := []UIEdge{}
	for i, n := range dag.Nodes {
		label := friendlyNodeLabel(n.Type, n.ID)
		uiNodes = append(uiNodes, UINode{
			ID:       n.ID,
			Position: map[string]float64{"x": 200, "y": float64(160 + i*120)},
			Data: map[string]interface{}{
				"label":    label,
				"nodeType": n.Type,
				"config":   n.Config,
			},
		})
		if !incoming[n.ID] {
			uiEdges = append(uiEdges, UIEdge{
				ID:     fmt.Sprintf("e-%s-%s", startNodeID, n.ID),
				Source: startNodeID,
				Target: n.ID,
				Data:   map[string]interface{}{"condition": ""},
			})
		}
	}

	for i, e := range dag.Edges {
		uiEdges = append(uiEdges, UIEdge{
			ID:     fmt.Sprintf("e-%s-%s-%d", e.Source, e.Target, i),
			Source: e.Source,
			Target: e.Target,
			Data:   map[string]interface{}{"condition": e.Condition},
		})
	}

	dag.UI = &CanvasUI{Nodes: uiNodes, Edges: uiEdges}
	return dag
}

func friendlyNodeLabel(nodeType, id string) string {
	switch strings.ToUpper(nodeType) {
	case "HTTP":
		return "HTTP Request"
	case "EMAIL":
		return "Send Email"
	case "TRANSFORM":
		return "Transform Data"
	default:
		return id
	}
}
