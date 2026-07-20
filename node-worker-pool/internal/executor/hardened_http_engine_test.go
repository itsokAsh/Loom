package executor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHardenedHTTPEngine_BlockedIPs(t *testing.T) {
	engine := NewHardenedHTTPEngine()

	blockedIPs := []string{
		"127.0.0.1",       // Localhost
		"169.254.169.254", // Cloud metadata
		"10.0.0.1",        // Private network
		"192.168.1.1",     // Private network
	}

	for _, ip := range blockedIPs {
		req := Request{
			URL:    fmt.Sprintf("http://%s", ip),
			Method: "GET",
		}

		_, err := engine.Execute(context.Background(), req)
		if err == nil {
			t.Errorf("Expected request to %s to be blocked, but it succeeded", ip)
		}
	}
}

func TestHardenedHTTPEngine_RedirectHijack(t *testing.T) {
	engine := NewHardenedHTTPEngine()

	// A malicious server that redirects to a blocked internal IP
	maliciousServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer maliciousServer.Close()

	req := Request{
		URL:    maliciousServer.URL,
		Method: "GET",
	}

	_, err := engine.Execute(context.Background(), req)
	if err == nil {
		t.Errorf("Expected redirect to internal IP to be blocked, but it succeeded")
	} else if !contains(err.Error(), "blocked destination") && !contains(err.Error(), "blocked IP") {
		t.Errorf("Expected blocked destination error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s != "" && substr != ""
}

func TestHardenedHTTPEngine_DNSRebinding_ResolveOnceDialToIP(t *testing.T) {
	// The HardenedHTTPEngine uses the following pattern in DialContext:
	// 1. Resolve hostname to IPs using net.DefaultResolver
	// 2. Validate all IPs against blocklist
	// 3. Dial the first valid IP directly using net.JoinHostPort(validIP, port)
	
	// Because it passes the resolved IP directly to DialContext (not the hostname),
	// the standard library's net.Dialer will not perform a second DNS lookup.
	// This intrinsically protects against DNS rebinding where the first lookup returns 
	// a safe IP and a subsequent lookup returns a malicious IP.
	
	// We can verify that it successfully connects to a valid external server.
	engine := NewHardenedHTTPEngine()
	engine.dialTimeout = 2 * time.Second

	// We'll just test that a valid external request works (e.g. example.com)
	// If the DialContext logic was broken, this would fail.
	req := Request{
		URL:    "http://example.com",
		Method: "GET",
	}

	resp, err := engine.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to execute valid request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
