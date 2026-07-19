package nodes

import (
	"context"
	"encoding/json"
	"fmt"
)

func init() {
	Register("TRANSFORM", &TransformExecutor{})
}

type TransformExecutor struct{}

type TransformConfig struct {
	Mapping interface{} `json:"mapping"`
}

func (e *TransformExecutor) Execute(ctx context.Context, config json.RawMessage) (json.RawMessage, error) {
	var reqConfig TransformConfig
	if err := json.Unmarshal(config, &reqConfig); err != nil {
		return nil, fmt.Errorf("invalid TRANSFORM node config: %w", err)
	}

	if reqConfig.Mapping == nil {
		// If no mapping is provided, return an empty object
		return json.Marshal(map[string]interface{}{})
	}

	resultBytes, err := json.Marshal(reqConfig.Mapping)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transform result: %w", err)
	}

	return resultBytes, nil
}
