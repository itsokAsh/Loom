package loom

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type WebhookClient struct {
	baseURL       string
	path          string
	secret        string
	allowInsecure bool
	maxRetries    int
	httpClient    *http.Client
}

type Option func(*WebhookClient)

func WithAllowInsecure(allow bool) Option {
	return func(c *WebhookClient) {
		c.allowInsecure = allow
	}
}

func WithMaxRetries(retries int) Option {
	return func(c *WebhookClient) {
		c.maxRetries = retries
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *WebhookClient) {
		c.httpClient = client
	}
}

func NewWebhookClient(baseURL, path, secret string, opts ...Option) (*WebhookClient, error) {
	c := &WebhookClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		path:       path,
		secret:     secret,
		maxRetries: 5,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	for _, opt := range opts {
		opt(c)
	}

	if !c.allowInsecure && !strings.HasPrefix(c.baseURL, "https://") && !strings.HasPrefix(c.baseURL, "http://localhost") && !strings.HasPrefix(c.baseURL, "http://127.0.0.1") {
		return nil, fmt.Errorf("insecure base URL '%s' rejected. Use https:// or set WithAllowInsecure(true)", c.baseURL)
	}

	return c, nil
}

func (c *WebhookClient) String() string {
	return fmt.Sprintf("WebhookClient{baseURL: %s, path: %s, secret: <REDACTED>}", c.baseURL, c.path)
}

type TriggerResult struct {
	ExecutionID string
	Status      string
}

func (c *WebhookClient) Trigger(ctx context.Context, payload any) (*TriggerResult, error) {
	idempKey := uuid.New().String()
	
	return c.doWithRetry(ctx, func(ctx context.Context) (*TriggerResult, error) {
		return c.doAttempt(ctx, payload, idempKey)
	})
}

func (c *WebhookClient) doAttempt(ctx context.Context, payload any, idempKey string) (*TriggerResult, error) {
	envelope := map[string]any{
		"timestamp": time.Now().Unix(),
		"payload":   payload,
	}

	// Canonical signing rule: Marshal once, sign these bytes, send these bytes
	bodyBytes, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(c.secret))
	mac.Write(bodyBytes)
	signature := hex.EncodeToString(mac.Sum(nil))

	reqURL := fmt.Sprintf("%s/webhooks/%s", c.baseURL, c.path)
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	req.Header.Set("Idempotency-Key", idempKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusAccepted {
		var res map[string]string
		if err := json.Unmarshal(respBody, &res); err != nil {
			return nil, fmt.Errorf("failed to parse success response: %w", err)
		}
		return &TriggerResult{
			ExecutionID: res["execution_id"],
			Status:      res["status"],
		}, nil
	}

	var errRes struct {
		ErrorCode string `json:"error_code"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &errRes); err != nil {
		errRes.Error = string(respBody)
		errRes.ErrorCode = "unknown_error"
	}

	baseErr := &LoomAPIError{
		ErrorCode: errRes.ErrorCode,
		Message:   errRes.Error,
		Body:      respBody,
	}

	switch errRes.ErrorCode {
	case "invalid_signature":
		return nil, &InvalidSignatureError{baseErr}
	case "rate_limited":
		retryAfterStr := resp.Header.Get("Retry-After")
		var delay time.Duration
		if d, err := time.ParseDuration(retryAfterStr + "s"); err == nil {
			delay = d
		} else {
			delay = 1 * time.Second
		}
		return nil, &RateLimitedError{LoomAPIError: baseErr, RetryAfter: delay}
	case "webhook_not_found":
		return nil, &WebhookNotFoundError{baseErr}
	case "invalid_payload":
		return nil, &InvalidPayloadError{baseErr}
	case "internal_error":
		return nil, &InternalServerError{baseErr}
	default:
		if resp.StatusCode >= 500 {
			return nil, &InternalServerError{baseErr}
		}
		return nil, baseErr
	}
}
