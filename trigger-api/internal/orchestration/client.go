package orchestration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client fetches per-node execution status from the orchestration engine.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient() *Client {
	base := os.Getenv("ORCHESTRATION_URL")
	if base == "" {
		base = "http://orchestration-engine:8081"
	}
	return &Client{
		baseURL: base,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type NodeExecution struct {
	NodeID       string          `json:"nodeId"`
	Status       string          `json:"status"`
	AttemptCount int32           `json:"attemptCount"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	Output       json.RawMessage `json:"output,omitempty"`
}

func (c *Client) GetExecutionTriggerData(executionID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/executions/%s", c.baseURL, executionID)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("orchestration API %d: %s", resp.StatusCode, string(body))
	}
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	td, ok := out["triggerData"].(map[string]interface{})
	if !ok || td == nil {
		return map[string]interface{}{}, nil
	}
	return td, nil
}

func (c *Client) ListNodeExecutions(executionID string) ([]NodeExecution, error) {
	url := fmt.Sprintf("%s/executions/%s/nodes", c.baseURL, executionID)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("orchestration API %d: %s", resp.StatusCode, string(body))
	}
	var nodes []NodeExecution
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}
