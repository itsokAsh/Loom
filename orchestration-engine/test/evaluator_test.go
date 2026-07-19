package test

import (
	"encoding/json"
	"testing"

	"github.com/loom/orchestration-engine/internal/dag"
	contracts "github.com/loom/shared/queue-contracts"
)

func TestEvaluator_NextActionableNodes(t *testing.T) {
	eval := dag.NewEvaluator()

	tests := []struct {
		name       string
		dagDef     contracts.DAGDefinition
		nodeStates map[string]string
		state      map[string]interface{}
		wantRun    []string
		wantSkip   []string
	}{
		{
			name: "linear success, no conditions",
			dagDef: contracts.DAGDefinition{
				Nodes: []contracts.Node{{ID: "A"}, {ID: "B"}},
				Edges: []contracts.Edge{{Source: "A", Target: "B"}},
			},
			nodeStates: map[string]string{"A": "SUCCESS"},
			state:      map[string]interface{}{},
			wantRun:    []string{"B"},
			wantSkip:   nil,
		},
		{
			name: "condition evaluates to true",
			dagDef: contracts.DAGDefinition{
				Nodes: []contracts.Node{{ID: "A"}, {ID: "B"}},
				Edges: []contracts.Edge{{Source: "A", Target: "B", Condition: "trigger.action == 'deploy'"}},
			},
			nodeStates: map[string]string{"A": "SUCCESS"},
			state: map[string]interface{}{
				"trigger": map[string]interface{}{"action": "deploy"},
			},
			wantRun:  []string{"B"},
			wantSkip: nil,
		},
		{
			name: "condition evaluates to false, node is skipped",
			dagDef: contracts.DAGDefinition{
				Nodes: []contracts.Node{{ID: "A"}, {ID: "B"}},
				Edges: []contracts.Edge{{Source: "A", Target: "B", Condition: "trigger.action == 'deploy'"}},
			},
			nodeStates: map[string]string{"A": "SUCCESS"},
			state: map[string]interface{}{
				"trigger": map[string]interface{}{"action": "test"},
			},
			wantRun:  nil,
			wantSkip: []string{"B"},
		},
		{
			name: "cascading skip: B is skipped, so C should be skipped",
			dagDef: contracts.DAGDefinition{
				Nodes: []contracts.Node{{ID: "A"}, {ID: "B"}, {ID: "C"}},
				Edges: []contracts.Edge{{Source: "A", Target: "B"}, {Source: "B", Target: "C"}},
			},
			nodeStates: map[string]string{"A": "SUCCESS", "B": "SKIPPED"},
			state:      map[string]interface{}{},
			wantRun:    nil,
			wantSkip:   []string{"C"},
		},
		{
			name: "multiple parents (AND logic): both true",
			dagDef: contracts.DAGDefinition{
				Nodes: []contracts.Node{{ID: "A"}, {ID: "B"}, {ID: "C"}},
				Edges: []contracts.Edge{
					{Source: "A", Target: "C", Condition: "outputs.A.success == true"},
					{Source: "B", Target: "C", Condition: "outputs.B.success == true"},
				},
			},
			nodeStates: map[string]string{"A": "SUCCESS", "B": "SUCCESS"},
			state: map[string]interface{}{
				"outputs": map[string]interface{}{
					"A": map[string]interface{}{"success": true},
					"B": map[string]interface{}{"success": true},
				},
			},
			wantRun:  []string{"C"},
			wantSkip: nil,
		},
		{
			name: "multiple parents (AND logic): one false, node is skipped",
			dagDef: contracts.DAGDefinition{
				Nodes: []contracts.Node{{ID: "A"}, {ID: "B"}, {ID: "C"}},
				Edges: []contracts.Edge{
					{Source: "A", Target: "C", Condition: "outputs.A.success == true"},
					{Source: "B", Target: "C", Condition: "outputs.B.success == true"},
				},
			},
			nodeStates: map[string]string{"A": "SUCCESS", "B": "SUCCESS"},
			state: map[string]interface{}{
				"outputs": map[string]interface{}{
					"A": map[string]interface{}{"success": true},
					"B": map[string]interface{}{"success": false},
				},
			},
			wantRun:  nil,
			wantSkip: []string{"C"},
		},
		{
			name: "graceful failure on missing condition fields",
			dagDef: contracts.DAGDefinition{
				Nodes: []contracts.Node{{ID: "A"}, {ID: "B"}},
				Edges: []contracts.Edge{{Source: "A", Target: "B", Condition: "trigger.missing.field == true"}},
			},
			nodeStates: map[string]string{"A": "SUCCESS"},
			state:      map[string]interface{}{"trigger": map[string]interface{}{}},
			wantRun:    nil,
			wantSkip:   []string{"B"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dagBytes, _ := json.Marshal(tt.dagDef)
			toRun, toSkip, err := eval.NextActionableNodes(dagBytes, tt.nodeStates, tt.state)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			runIDs := []string{}
			for _, n := range toRun {
				runIDs = append(runIDs, n.ID)
			}
			skipIDs := []string{}
			for _, n := range toSkip {
				skipIDs = append(skipIDs, n.ID)
			}

			if !equalStrings(runIDs, tt.wantRun) {
				t.Errorf("toRun = %v, want %v", runIDs, tt.wantRun)
			}
			if !equalStrings(skipIDs, tt.wantSkip) {
				t.Errorf("toSkip = %v, want %v", skipIDs, tt.wantSkip)
			}
		})
	}
}

func TestEvaluator_EvaluateConfig(t *testing.T) {
	eval := dag.NewEvaluator()

	state := map[string]interface{}{
		"trigger": map[string]interface{}{
			"id":     123,
			"action": "deploy",
		},
		"outputs": map[string]interface{}{
			"NodeA": map[string]interface{}{
				"url": "https://api.example.com",
			},
		},
	}

	config := map[string]interface{}{
		"url":           "{{ outputs.NodeA.url }}/resource/{{ trigger.id }}",
		"should_deploy": "{{ trigger.action == 'deploy' }}",
		"nested": map[string]interface{}{
			"list": []interface{}{
				"static",
				"{{ trigger.action }}",
			},
		},
	}

	resBytes, err := eval.EvaluateConfig(config, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(resBytes, &res); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if got := res["url"]; got != "https://api.example.com/resource/123" {
		t.Errorf("expected url 'https://api.example.com/resource/123', got %v", got)
	}

	if got := res["should_deploy"]; got != true {
		t.Errorf("expected should_deploy true, got %v", got)
	}

	nested := res["nested"].(map[string]interface{})
	list := nested["list"].([]interface{})
	if list[1] != "deploy" {
		t.Errorf("expected list[1] to be 'deploy', got %v", list[1])
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
