package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func init() {
	Register("HTTP", &HTTPExecutor{})
}

type HTTPExecutor struct {
	client *http.Client
}

type HTTPConfig struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body"`
}

func (e *HTTPExecutor) Execute(ctx context.Context, config json.RawMessage) (json.RawMessage, error) {
	if e.client == nil {
		e.client = &http.Client{Timeout: 30 * time.Second}
	}

	var reqConfig HTTPConfig
	if err := json.Unmarshal(config, &reqConfig); err != nil {
		return nil, fmt.Errorf("invalid HTTP node config: %w", err)
	}

	var bodyReader io.Reader
	if reqConfig.Body != nil {
		bodyBytes, err := json.Marshal(reqConfig.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	method := reqConfig.Method
	if method == "" {
		method = "GET"
	}

	req, err := http.NewRequestWithContext(ctx, method, reqConfig.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range reqConfig.Headers {
		req.Header.Set(k, v)
	}

	// Default to JSON if body exists and content-type isn't set
	if reqConfig.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Attempt to parse response body as JSON, if it fails, return as a string
	var parsedBody interface{}
	if err := json.Unmarshal(respBody, &parsedBody); err != nil {
		parsedBody = string(respBody)
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
