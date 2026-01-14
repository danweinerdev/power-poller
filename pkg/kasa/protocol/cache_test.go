package protocol

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewProtocolCache(t *testing.T) {
	cache := NewProtocolCache()
	if cache == nil {
		t.Fatal("NewProtocolCache() returned nil")
	}
	if cache.cache == nil {
		t.Error("cache map should be initialized")
	}
}

func TestProtocolCache_GetSet(t *testing.T) {
	cache := NewProtocolCache()

	// Get on empty cache
	proto, ok := cache.Get("192.168.1.100")
	if ok {
		t.Error("Get() on empty cache should return false")
	}

	// Set and get
	cache.Set("192.168.1.100", ProtocolLegacy)

	proto, ok = cache.Get("192.168.1.100")
	if !ok {
		t.Error("Get() after Set() should return true")
	}
	if proto != ProtocolLegacy {
		t.Errorf("Get() = %v, want %v", proto, ProtocolLegacy)
	}

	// Update existing
	cache.Set("192.168.1.100", ProtocolKLAP)

	proto, ok = cache.Get("192.168.1.100")
	if !ok {
		t.Error("Get() should return true")
	}
	if proto != ProtocolKLAP {
		t.Errorf("Get() = %v, want %v", proto, ProtocolKLAP)
	}
}

func TestProtocolCache_Clear(t *testing.T) {
	cache := NewProtocolCache()

	cache.Set("192.168.1.100", ProtocolLegacy)
	cache.Set("192.168.1.101", ProtocolKLAP)

	cache.Clear()

	if _, ok := cache.Get("192.168.1.100"); ok {
		t.Error("Get() after Clear() should return false")
	}
	if _, ok := cache.Get("192.168.1.101"); ok {
		t.Error("Get() after Clear() should return false")
	}
}

func TestProtocolCache_Detect_Legacy(t *testing.T) {
	// Note: Protocol detection probes specific ports (9999 for legacy, 80 for KLAP)
	// We can't easily test real detection without binding to those specific ports
	// This test verifies that pre-populated cache values are respected

	cache := NewProtocolCache()

	// Pre-populate with legacy
	cache.Set("192.168.1.100", ProtocolLegacy)

	ctx := context.Background()
	proto := cache.Detect(ctx, "192.168.1.100")

	// Should return cached value
	if proto != ProtocolLegacy {
		t.Errorf("Detect() = %v, want %v (cached)", proto, ProtocolLegacy)
	}

	// Second call should still use cache
	proto2 := cache.Detect(ctx, "192.168.1.100")
	if proto2 != proto {
		t.Errorf("cached Detect() = %v, want %v", proto2, proto)
	}
}

func TestProtocolCache_Detect_KLAP(t *testing.T) {
	// Note: Protocol detection probes specific ports (9999 for legacy, 80 for KLAP)
	// httptest uses random ports, so we can't easily test real KLAP detection
	// This test verifies that pre-populated cache values are respected

	cache := NewProtocolCache()

	// Pre-populate with KLAP
	cache.Set("192.168.1.200", ProtocolKLAP)

	ctx := context.Background()
	proto := cache.Detect(ctx, "192.168.1.200")

	if proto != ProtocolKLAP {
		t.Errorf("Detect() = %v, want %v (cached)", proto, ProtocolKLAP)
	}
}

func TestProtocolCache_Detect_Unknown(t *testing.T) {
	cache := NewProtocolCache()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Use a non-routable address
	proto := cache.Detect(ctx, "10.255.255.1")

	if proto != ProtocolUnknown {
		t.Errorf("Detect() for unreachable host = %v, want %v", proto, ProtocolUnknown)
	}
}

func TestProtocolCache_Detect_CachesResult(t *testing.T) {
	cache := NewProtocolCache()

	// Pre-populate cache
	cache.Set("192.168.1.100", ProtocolKLAP)

	ctx := context.Background()
	proto := cache.Detect(ctx, "192.168.1.100")

	// Should return cached value without probing
	if proto != ProtocolKLAP {
		t.Errorf("Detect() should return cached value, got %v, want %v", proto, ProtocolKLAP)
	}
}

func TestProtocolCache_DetectAll(t *testing.T) {
	// Test DetectAll with pre-populated cache values
	// Real detection requires specific ports, so we test the concurrent lookup behavior

	cache := NewProtocolCache()

	hosts := []string{
		"192.168.1.100",
		"192.168.1.101",
		"192.168.1.102",
	}

	// Pre-populate cache
	cache.Set("192.168.1.100", ProtocolLegacy)
	cache.Set("192.168.1.101", ProtocolKLAP)
	cache.Set("192.168.1.102", ProtocolLegacy)

	ctx := context.Background()
	results := cache.DetectAll(ctx, hosts)

	if len(results) != len(hosts) {
		t.Errorf("DetectAll() returned %d results, want %d", len(results), len(hosts))
	}

	expected := map[string]ProtocolType{
		"192.168.1.100": ProtocolLegacy,
		"192.168.1.101": ProtocolKLAP,
		"192.168.1.102": ProtocolLegacy,
	}

	for host, want := range expected {
		got, ok := results[host]
		if !ok {
			t.Errorf("DetectAll() missing result for %s", host)
			continue
		}
		if got != want {
			t.Errorf("DetectAll()[%s] = %v, want %v", host, got, want)
		}
	}
}

func TestProtocolCache_DetectAll_Empty(t *testing.T) {
	cache := NewProtocolCache()
	ctx := context.Background()

	results := cache.DetectAll(ctx, []string{})

	if len(results) != 0 {
		t.Errorf("DetectAll() with empty hosts returned %d results, want 0", len(results))
	}
}

func TestProtocolCache_ConcurrentAccess(t *testing.T) {
	cache := NewProtocolCache()

	var wg sync.WaitGroup
	hosts := []string{
		"192.168.1.100",
		"192.168.1.101",
		"192.168.1.102",
	}

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for _, host := range hosts {
				if id%2 == 0 {
					cache.Set(host, ProtocolLegacy)
				} else {
					cache.Set(host, ProtocolKLAP)
				}
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, host := range hosts {
				cache.Get(host)
			}
		}()
	}

	wg.Wait()

	// Should not panic or corrupt state
	for _, host := range hosts {
		proto, ok := cache.Get(host)
		if !ok {
			t.Errorf("Get(%s) should return true after writes", host)
		}
		if proto != ProtocolLegacy && proto != ProtocolKLAP {
			t.Errorf("Get(%s) = %v, want ProtocolLegacy or ProtocolKLAP", host, proto)
		}
	}
}

func TestProtocolType_Constants(t *testing.T) {
	// Verify protocol type constants
	if ProtocolLegacy != "legacy" {
		t.Errorf("ProtocolLegacy = %q, want %q", ProtocolLegacy, "legacy")
	}
	if ProtocolKLAP != "klap" {
		t.Errorf("ProtocolKLAP = %q, want %q", ProtocolKLAP, "klap")
	}
	if ProtocolUnknown != "unknown" {
		t.Errorf("ProtocolUnknown = %q, want %q", ProtocolUnknown, "unknown")
	}
}

func TestProbeLegacy(t *testing.T) {
	// Start a TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	ctx := context.Background()

	// Parse host from listener address
	addr := listener.Addr().String()
	parts := strings.Split(addr, ":")
	host := parts[0]

	// This won't work directly since probeLegacy uses DefaultPort
	// Instead test the full detectProtocol which uses the address as-is
	// For unit testing probeLegacy directly, we'd need to refactor

	// Test with full address (integration-style)
	result := probeLegacy(ctx, addr)

	// This should return true since we're listening
	// Note: probeLegacy appends DefaultPort, so this test is limited
	_ = host
	_ = result
}

func TestProbeKLAP(t *testing.T) {
	// Start HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// probeKLAP appends KLAPPort, so direct testing is limited
	// This is more of an integration test scenario
	_ = server // Avoid unused variable
}
