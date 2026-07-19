package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// HardenedHTTPEngine provides a secure HTTP client with SSRF protection
type HardenedHTTPEngine struct {
	ipBlocklist  *IPBlocklist
	rateLimiter  *rate.Limiter
	maxRedirects int
	maxBodySize  int64
	dialTimeout  time.Duration
	tlsTimeout   time.Duration
}

// Request represents an HTTP request configuration
type Request struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    []byte
	Timeout time.Duration
}

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}

// NewHardenedHTTPEngine creates a new hardened HTTP engine with default security settings
func NewHardenedHTTPEngine() *HardenedHTTPEngine {
	return &HardenedHTTPEngine{
		ipBlocklist:  NewIPBlocklist(),
		rateLimiter:  rate.NewLimiter(rate.Limit(100), 100), // 100 req/sec
		maxRedirects: 3,
		maxBodySize:  10 * 1024 * 1024, // 10 MB
		dialTimeout:  10 * time.Second,
		tlsTimeout:   10 * time.Second,
	}
}

// Execute performs an HTTP request with full security checks
func (e *HardenedHTTPEngine) Execute(ctx context.Context, req Request) (*Response, error) {
	// Rate limiting
	if err := e.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Parse URL
	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Check if hostname is blocked
	if e.ipBlocklist.IsHostnameBlocked(parsedURL.Hostname()) {
		return nil, fmt.Errorf("blocked hostname: %s", parsedURL.Hostname())
	}

	// Validate and sanitize headers
	sanitizedHeaders := make(map[string]string)
	for k, v := range req.Headers {
		// Validate header name (alphanumeric + dash only)
		if !isValidHeaderName(k) {
			return nil, fmt.Errorf("invalid header name: %s", k)
		}
		// Strip CRLF from header values
		sanitizedHeaders[k] = stripCRLF(v)
	}

	// Block sensitive headers from user input
	blockedHeaders := map[string]bool{
		"host":              true,
		"connection":        true,
		"transfer-encoding": true,
	}
	for k := range sanitizedHeaders {
		if blockedHeaders[strings.ToLower(k)] {
			return nil, fmt.Errorf("cannot override protected header: %s", k)
		}
	}

	// Check body size
	if int64(len(req.Body)) > e.maxBodySize {
		return nil, fmt.Errorf("request body exceeds max size of %d bytes", e.maxBodySize)
	}

	// Set default timeout if not specified
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Create context with timeout
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create custom dialer with DNS rebinding protection
	dialer := &net.Dialer{
		Timeout: e.dialTimeout,
	}

	// Custom transport with DNS validation on EVERY dial
	transport := &http.Transport{
		DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address: %w", err)
			}

			// Resolve hostname to IPs (happens fresh on EVERY dial)
			ips, err := net.DefaultResolver.LookupIP(dialCtx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("DNS lookup failed: %w", err)
			}

			if len(ips) == 0 {
				return nil, fmt.Errorf("no IP addresses found for host: %s", host)
			}

			// Validate ALL resolved IPs against blocklist
			for _, ip := range ips {
				if e.ipBlocklist.IsBlocked(ip) {
					return nil, fmt.Errorf("blocked destination: %s resolves to blocked IP %v", host, ip)
				}
			}

			// Connect to first valid IP
			validIP := ips[0].String()
			return dialer.DialContext(dialCtx, network, net.JoinHostPort(validIP, port))
		},
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: e.tlsTimeout,
		ForceAttemptHTTP2:   false, // Disable HTTP/2 to prevent connection caching issues
	}

	// Redirect counter and validation
	redirectCount := 0
	visitedURLs := make(map[string]bool)

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			redirectCount++
			if redirectCount > e.maxRedirects {
				return fmt.Errorf("too many redirects (max %d)", e.maxRedirects)
			}

			// Check for redirect loops
			urlStr := req.URL.String()
			if visitedURLs[urlStr] {
				return fmt.Errorf("redirect loop detected")
			}
			visitedURLs[urlStr] = true

			// Block protocol downgrade (HTTPS -> HTTP)
			if len(via) > 0 && via[0].URL.Scheme == "https" && req.URL.Scheme == "http" {
				return fmt.Errorf("protocol downgrade not allowed (HTTPS -> HTTP)")
			}

			// DNS validation happens automatically in DialContext for each redirect
			return nil
		},
		Timeout: timeout,
	}

	// Build HTTP request
	httpReq, err := http.NewRequestWithContext(reqCtx, req.Method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for k, v := range sanitizedHeaders {
		httpReq.Header.Set(k, v)
	}

	// Execute request
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Check response body size
	limitedReader := io.LimitReader(httpResp.Body, e.maxBodySize+1)
	respBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if int64(len(respBody)) > e.maxBodySize {
		return nil, fmt.Errorf("response body exceeds max size of %d bytes", e.maxBodySize)
	}

	return &Response{
		StatusCode: httpResp.StatusCode,
		Headers:    httpResp.Header,
		Body:       respBody,
	}, nil
}

// isValidHeaderName checks if header name contains only allowed characters
func isValidHeaderName(name string) bool {
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return len(name) > 0
}

// stripCRLF removes CRLF characters from header values to prevent injection
func stripCRLF(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}
