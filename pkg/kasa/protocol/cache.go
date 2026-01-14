package protocol

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
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
	// This indicates default credentials work.
	ProtocolKLAP ProtocolType = "klap"

	// ProtocolKLAPAuthRequired uses HTTP port 80 with KLAP but requires
	// custom credentials (default credentials don't work).
	ProtocolKLAPAuthRequired ProtocolType = "klap_auth_required"

	// ProtocolSecurePassthrough uses HTTP port 80 with JSON-RPC and RSA/AES encryption.
	ProtocolSecurePassthrough ProtocolType = "securepassthrough"

	// ProtocolUnknown indicates the device is reachable but protocol couldn't be identified.
	// This is a configuration error - the device may not be a supported KASA device.
	ProtocolUnknown ProtocolType = "unknown"

	// ProtocolUnreachable indicates the device could not be reached on any port.
	// This may be a temporary network issue.
	ProtocolUnreachable ProtocolType = "unreachable"

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

// httpProbeResult holds the result of an HTTP protocol probe.
type httpProbeResult struct {
	proto     ProtocolType
	reachable bool // true if HTTP port responded (even if protocol unknown)
}

// detectProtocol probes a device to determine its protocol type.
func detectProtocol(ctx context.Context, host string) ProtocolType {
	// Create a context with timeout for detection
	detectCtx, cancel := context.WithTimeout(ctx, DetectTimeout*3)
	defer cancel()

	legacyCh := make(chan bool, 1)
	httpCh := make(chan httpProbeResult, 1)

	// Probe legacy port (TCP 9999)
	go func() {
		legacyCh <- probeLegacy(detectCtx, host)
	}()

	// Probe HTTP port 80 and determine if KLAP or SecurePassthrough
	go func() {
		httpCh <- probeHTTPProtocol(detectCtx, host)
	}()

	// Wait for results
	var legacyOK bool
	var httpResult httpProbeResult
	resultsReceived := 0
waitLoop:
	for resultsReceived < 2 {
		select {
		case legacyOK = <-legacyCh:
			resultsReceived++
		case httpResult = <-httpCh:
			resultsReceived++
		case <-detectCtx.Done():
			break waitLoop
		}
	}

	// Prefer legacy if available (more devices support it)
	if legacyOK {
		return ProtocolLegacy
	}

	// Check HTTP result - only if we got a valid protocol
	switch httpResult.proto {
	case ProtocolKLAP, ProtocolKLAPAuthRequired, ProtocolSecurePassthrough:
		return httpResult.proto
	case ProtocolUnknown:
		// Device was reachable but protocol not recognized
		return ProtocolUnknown
	}

	// Neither port was reachable - device is offline or unreachable
	return ProtocolUnreachable
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
// For KLAP devices, it also verifies if default credentials work.
// Returns both the detected protocol and whether the device was reachable.
func probeHTTPProtocol(ctx context.Context, host string) httpProbeResult {
	client := &http.Client{
		Timeout: DetectTimeout * 2, // Give more time for HTTP probes
	}

	// First, check the server header - "SHIP" indicates SecurePassthrough (TAPO)
	headReq, err := http.NewRequestWithContext(ctx, "HEAD", fmt.Sprintf("http://%s:%d/", host, SecurePassthroughPort), nil)
	if err != nil {
		return httpProbeResult{proto: ProtocolUnreachable, reachable: false}
	}

	headResp, err := client.Do(headReq)
	if err != nil {
		return httpProbeResult{proto: ProtocolUnreachable, reachable: false}
	}
	headResp.Body.Close()

	// Device is reachable on HTTP - from here on, reachable=true
	reachable := true

	// Check Server header for SHIP (SecurePassthrough/TAPO devices)
	serverHeader := headResp.Header.Get("Server")
	if len(serverHeader) >= 4 && serverHeader[:4] == "SHIP" {
		return httpProbeResult{proto: ProtocolSecurePassthrough, reachable: reachable}
	}

	// Generate a random client seed for the handshake
	clientSeed := make([]byte, 16)
	if _, err := rand.Read(clientSeed); err != nil {
		return httpProbeResult{proto: ProtocolUnknown, reachable: reachable}
	}

	// Try KLAP handshake1 endpoint
	klapURL := fmt.Sprintf("http://%s:%d/app/handshake1", host, KLAPPort)
	klapReq, err := http.NewRequestWithContext(ctx, "POST", klapURL, bytes.NewReader(clientSeed))
	if err != nil {
		return httpProbeResult{proto: ProtocolUnknown, reachable: reachable}
	}
	klapReq.Header.Set("Content-Type", "application/octet-stream")

	klapResp, err := client.Do(klapReq)
	if err != nil {
		return httpProbeResult{proto: ProtocolUnknown, reachable: reachable}
	}
	defer klapResp.Body.Close()

	if klapResp.StatusCode != http.StatusOK {
		return httpProbeResult{proto: ProtocolUnknown, reachable: reachable}
	}

	body, _ := io.ReadAll(io.LimitReader(klapResp.Body, 64))
	// KLAP returns exactly 48 bytes of binary data (16 byte seed + 32 byte hash)
	// and it should NOT be JSON or HTML
	if len(body) != 48 || isJSONResponse(body) || isHTMLResponse(body) {
		return httpProbeResult{proto: ProtocolUnknown, reachable: reachable}
	}

	// Parse handshake1 response: 16 byte server seed + 32 byte server hash
	serverSeed := body[:16]
	serverHash := body[16:48]

	// Verify if default credentials work by checking the server hash
	// Server hash = SHA256(localHash + serverSeed + clientSeed)
	// where localHash = SHA256(SHA1(username) + SHA1(password))
	defaultLocalHash := computeDetectLocalHash(DefaultKLAPUsername, DefaultKLAPPassword)
	expectedHash := computeDetectHash(defaultLocalHash, serverSeed, clientSeed)

	if bytes.Equal(serverHash, expectedHash) {
		// Default credentials work
		return httpProbeResult{proto: ProtocolKLAP, reachable: reachable}
	}

	// KLAP device but requires custom credentials
	return httpProbeResult{proto: ProtocolKLAPAuthRequired, reachable: reachable}
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

// computeDetectLocalHash computes SHA256(SHA1(username) + SHA1(password)) for detection.
func computeDetectLocalHash(username, password string) []byte {
	userHash := sha1.Sum([]byte(username))
	passHash := sha1.Sum([]byte(password))

	combined := append(userHash[:], passHash[:]...)
	result := sha256.Sum256(combined)
	return result[:]
}

// computeDetectHash computes SHA256(localHash + seed1 + seed2) for detection.
func computeDetectHash(localHash, seed1, seed2 []byte) []byte {
	data := append(localHash, seed1...)
	data = append(data, seed2...)
	result := sha256.Sum256(data)
	return result[:]
}
