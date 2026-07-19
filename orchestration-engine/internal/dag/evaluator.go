package dag

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/expr-lang/expr"
	contracts "github.com/loom/shared/queue-contracts"
)

type Evaluator struct {
}

func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// NextActionableNodes determines which nodes can run next.
func (e *Evaluator) NextActionableNodes(dagDef []byte, nodeStates map[string]string, state map[string]interface{}) (toRun []contracts.Node, toSkip []contracts.Node, err error) {
	var dag contracts.DAGDefinition
	if err := json.Unmarshal(dagDef, &dag); err != nil {
		return nil, nil, err
	}
	
	nodesByID := make(map[string]contracts.Node)
	for _, n := range dag.Nodes {
		nodesByID[n.ID] = n
	}
	
	incomingEdges := make(map[string][]contracts.Edge)
	for _, edge := range dag.Edges {
		incomingEdges[edge.Target] = append(incomingEdges[edge.Target], edge)
	}

	for _, n := range dag.Nodes {
		nodeStatus, exists := nodeStates[n.ID]
		// If node has already started/completed, it's not "next"
		if exists && (nodeStatus == "QUEUED" || nodeStatus == "RUNNING" || nodeStatus == "SUCCESS" || nodeStatus == "SKIPPED" || nodeStatus == "ERROR") {
			continue
		}

		parents := incomingEdges[n.ID]
		parentsComplete := true
		anyParentSkipped := false

		for _, edge := range parents {
			parentState, parentExists := nodeStates[edge.Source]
			if !parentExists || (parentState != "SUCCESS" && parentState != "SKIPPED") {
				parentsComplete = false
				break
			}
			if parentState == "SKIPPED" {
				anyParentSkipped = true
			}
		}

		if !parentsComplete {
			continue
		}

		if anyParentSkipped {
			// Cascade skip
			toSkip = append(toSkip, n)
			continue
		}

		// Evaluate conditions (AND logic)
		conditionsMet := true
		for _, edge := range parents {
			if edge.Condition != "" {
				program, err := expr.Compile(edge.Condition, expr.Env(state), expr.AsBool())
				if err != nil {
					// Graceful failure
					conditionsMet = false
					break
				}
				result, err := expr.Run(program, state)
				if err != nil {
					conditionsMet = false
					break
				}
				if passed, ok := result.(bool); !ok || !passed {
					conditionsMet = false
					break
				}
			}
		}

		if conditionsMet {
			toRun = append(toRun, n)
		} else {
			toSkip = append(toSkip, n)
		}
	}

	return toRun, toSkip, nil
}

// EvaluateConfig interpolates the config for a node.
func (e *Evaluator) EvaluateConfig(nodeConfig map[string]interface{}, state map[string]interface{}) (json.RawMessage, error) {
	evalMap(nodeConfig, state)
	return json.Marshal(nodeConfig)
}

func evalMap(m map[string]interface{}, state map[string]interface{}) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			m[k] = evalString(val, state)
		case map[string]interface{}:
			evalMap(val, state)
		case []interface{}:
			evalSlice(val, state)
		}
	}
}

func evalSlice(s []interface{}, state map[string]interface{}) {
	for i, v := range s {
		switch val := v.(type) {
		case string:
			s[i] = evalString(val, state)
		case map[string]interface{}:
			evalMap(val, state)
		case []interface{}:
			evalSlice(val, state)
		}
	}
}

func evalString(s string, state map[string]interface{}) interface{} {
	if strings.HasPrefix(s, "{{") && strings.HasSuffix(s, "}}") {
		inner := strings.TrimSpace(s[2 : len(s)-2])
		if !strings.Contains(inner, "{{") {
			program, err := expr.Compile(inner, expr.Env(state))
			if err == nil {
				res, err := expr.Run(program, state)
				if err == nil {
					return res
				}
			}
		}
	}

	result := s
	start := 0
	for {
		i := strings.Index(result[start:], "{{")
		if i == -1 {
			break
		}
		i += start
		j := strings.Index(result[i:], "}}")
		if j == -1 {
			break
		}
		j += i + 2
		inner := strings.TrimSpace(result[i+2 : j-2])
		
		program, err := expr.Compile(inner, expr.Env(state))
		var evalStr string
		if err == nil {
			res, err := expr.Run(program, state)
			if err == nil {
				evalStr = fmt.Sprintf("%v", res)
			} else {
				evalStr = fmt.Sprintf("{{%s}}", inner)
			}
		} else {
			evalStr = fmt.Sprintf("{{%s}}", inner)
		}
		
		result = result[:i] + evalStr + result[j:]
		start = i + len(evalStr)
	}

	return result
}
