package protocol

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// ProtocolType identifies the communication protocol for a device.
type ProtocolType string

const (
	// ProtocolLegacy uses TCP port 9999 with XOR encryption.
	ProtocolLegacy ProtocolType = "legacy"

	// ProtocolKLAP uses HTTP port 80 with binary KLAP handshake.
	ProtocolKLAP ProtocolType = "klap"

	// ProtocolSecurePassthrough uses HTTP port 80 with JSON-RPC and RSA/AES encryption.
	ProtocolSecurePassthrough ProtocolType = "securepassthrough"

	// ProtocolUnknown indicates the protocol couldn't be detected.
	ProtocolUnknown ProtocolType = "unknown"

	// DetectTimeout is the timeout for protocol detection probes.
	DetectTimeout = 500 * time.Millisecond
)

// ProtocolCache caches detected protocol types for devices.
type ProtocolCache struct {
	mu    sync.RWMutex
	cache map[string]ProtocolType
}

// NewProtocolCache creates a new protocol cache.
func NewProtocolCache() *ProtocolCache {
	return &ProtocolCache{
		cache: make(map[string]ProtocolType),
	}
}

// Get returns the cached protocol type for a host, if known.
func (c *ProtocolCache) Get(host string) (ProtocolType, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	proto, ok := c.cache[host]
	return proto, ok
}

// Set caches the protocol type for a host.
func (c *ProtocolCache) Set(host string, proto ProtocolType) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[host] = proto
}

// Detect determines the protocol type for a device by probing its ports.
// Results are cached for subsequent calls.
func (c *ProtocolCache) Detect(ctx context.Context, host string) ProtocolType {
	// Check cache first
	if proto, ok := c.Get(host); ok {
		return proto
	}

	// Detect protocol
	proto := detectProtocol(ctx, host)

	// Cache the result
	c.Set(host, proto)

	return proto
}

// DetectAll detects protocols for multiple hosts concurrently.
func (c *ProtocolCache) DetectAll(ctx context.Context, hosts []string) map[string]ProtocolType {
	results := make(map[string]ProtocolType)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, host := range hosts {
		wg.Add(1)
		go func(h string) {
			defer wg.Done()
			proto := c.Detect(ctx, h)
			mu.Lock()
			results[h] = proto
			mu.Unlock()
		}(host)
	}

	wg.Wait()
	return results
}

// Clear removes all cached entries.
func (c *ProtocolCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]ProtocolType)
}

// detectProtocol probes a device to determine its protocol type.
func detectProtocol(ctx context.Context, host string) ProtocolType {
	// Create a context with timeout for detection
	detectCtx, cancel := context.WithTimeout(ctx, DetectTimeout*3)
	defer cancel()

	// Try all probes concurrently
	type probeResult struct {
		proto ProtocolType
		ok    bool
	}

	legacyCh := make(chan probeResult, 1)
	httpCh := make(chan probeResult, 1)

	// Probe legacy port (TCP 9999)
	go func() {
		ok := probeLegacy(detectCtx, host)
		legacyCh <- probeResult{ProtocolLegacy, ok}
	}()

	// Probe HTTP port 80 and determine if KLAP or SecurePassthrough
	go func() {
		proto := probeHTTPProtocol(detectCtx, host)
		httpCh <- probeResult{proto, proto != ProtocolUnknown}
	}()

	// Wait for results
	var legacyOK bool
	var httpProto ProtocolType = ProtocolUnknown
	for i := 0; i < 2; i++ {
		select {
		case r := <-legacyCh:
			legacyOK = r.ok
		case r := <-httpCh:
			httpProto = r.proto
		case <-detectCtx.Done():
			break
		}
	}

	// Prefer legacy if available (more devices support it)
	if legacyOK {
		return ProtocolLegacy
	}
	if httpProto != ProtocolUnknown {
		return httpProto
	}

	return ProtocolUnknown
}

// probeLegacy tests if a device responds on TCP port 9999.
func probeLegacy(ctx context.Context, host string) bool {
	addr := fmt.Sprintf("%s:%d", host, DefaultPort)

	dialer := &net.Dialer{
		Timeout: DetectTimeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// probeHTTPProtocol tests HTTP port 80 and determines if KLAP or SecurePassthrough.
func probeHTTPProtocol(ctx context.Context, host string) ProtocolType {
	client := &http.Client{
		Timeout: DetectTimeout * 2, // Give more time for HTTP probes
	}

	// First, check the server header - "SHIP" indicates SecurePassthrough (TAPO)
	headReq, err := http.NewRequestWithContext(ctx, "HEAD", fmt.Sprintf("http://%s:%d/", host, SecurePassthroughPort), nil)
	if err != nil {
		return ProtocolUnknown
	}

	headResp, err := client.Do(headReq)
	if err != nil {
		return ProtocolUnknown
	}
	headResp.Body.Close()

	// Check Server header for SHIP (SecurePassthrough/TAPO devices)
	serverHeader := headResp.Header.Get("Server")
	if len(serverHeader) >= 4 && serverHeader[:4] == "SHIP" {
		return ProtocolSecurePassthrough
	}

	// Try KLAP handshake1 endpoint - returns exactly 48 bytes for KLAP devices
	klapURL := fmt.Sprintf("http://%s:%d/app/handshake1", host, KLAPPort)
	klapReq, err := http.NewRequestWithContext(ctx, "POST", klapURL, bytes.NewReader(make([]byte, 16)))
	if err != nil {
		return ProtocolUnknown
	}
	klapReq.Header.Set("Content-Type", "application/octet-stream")

	klapResp, err := client.Do(klapReq)
	if err != nil {
		return ProtocolUnknown
	}
	defer klapResp.Body.Close()

	if klapResp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(klapResp.Body, 64))
		// KLAP returns exactly 48 bytes of binary data (16 byte seed + 32 byte hash)
		// and it should NOT be JSON or HTML
		if len(body) == 48 && !isJSONResponse(body) && !isHTMLResponse(body) {
			return ProtocolKLAP
		}
	}

	return ProtocolUnknown
}

// isJSONResponse checks if the response body looks like JSON.
func isJSONResponse(body []byte) bool {
	// Trim whitespace and check for JSON markers
	for _, b := range body {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		return b == '{' || b == '['
	}
	return false
}

// isHTMLResponse checks if the response body looks like HTML.
func isHTMLResponse(body []byte) bool {
	// Check for HTML markers
	for _, b := range body {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		return b == '<'
	}
	return false
}
