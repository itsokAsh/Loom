package nodes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/loom/node-worker-pool/internal/executor"
)

func init() {
	Register("HTTP", &HTTPExecutor{})
}

type HTTPExecutor struct {
	engine *executor.HardenedHTTPEngine
}

type HTTPConfig struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body"`
}

func (e *HTTPExecutor) Execute(ctx context.Context, config json.RawMessage) (json.RawMessage, error) {
	if e.engine == nil {
		e.engine = executor.NewHardenedHTTPEngine()
	}

	var reqConfig HTTPConfig
	if err := json.Unmarshal(config, &reqConfig); err != nil {
		return nil, fmt.Errorf("invalid HTTP node config: %w", err)
	}

	var bodyBytes []byte
	var err error
	if reqConfig.Body != nil {
		bodyBytes, err = json.Marshal(reqConfig.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	method := reqConfig.Method
	if method == "" {
		method = "GET"
	}
	
	if reqConfig.Headers == nil {
		reqConfig.Headers = make(map[string]string)
	}
	
	if reqConfig.Body != nil && reqConfig.Headers["Content-Type"] == "" {
		reqConfig.Headers["Content-Type"] = "application/json"
	}

	req := executor.Request{
		Method:  method,
		URL:     reqConfig.URL,
		Headers: reqConfig.Headers,
		Body:    bodyBytes,
	}

	resp, err := e.engine.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	var parsedBody interface{}
	if err := json.Unmarshal(resp.Body, &parsedBody); err != nil {
		parsedBody = string(resp.Body)
	}

	result := map[string]interface{}{
		"status": resp.StatusCode,
		"body":   parsedBody,
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	if resp.StatusCode >= 400 {
		return resultBytes, fmt.Errorf("HTTP error %d", resp.StatusCode)
	}

	return resultBytes, nil
}
