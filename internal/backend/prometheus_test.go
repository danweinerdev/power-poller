package backend

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/danweinerdev/power-poller/internal/config"
	"github.com/danweinerdev/power-poller/internal/metrics"
)

func findFreePort() int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

func TestPrometheus_New(t *testing.T) {
	cfg := config.PrometheusConfig{
		Port: 9090,
		Path: "/metrics",
	}
	p := NewPrometheus(cfg, nil)
	if p == nil {
		t.Fatal("NewPrometheus() returned nil")
	}
	if p.Name() != "prometheus" {
		t.Errorf("Name() = %q, want %q", p.Name(), "prometheus")
	}
}

func TestPrometheus_Initialize_Success(t *testing.T) {
	port := findFreePort()
	cfg := config.PrometheusConfig{
		Enabled: true,
		Port:    port,
		Path:    "/metrics",
	}
	p := NewPrometheus(cfg, nil)

	ctx := context.Background()
	if err := p.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	defer p.Close()

	if !p.Healthy() {
		t.Error("should be healthy after Initialize()")
	}

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	// Test that server is running
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("health check status = %d, want 200", resp.StatusCode)
	}
}

func TestPrometheus_Initialize_MetricsEndpoint(t *testing.T) {
	port := findFreePort()
	cfg := config.PrometheusConfig{
		Enabled: true,
		Port:    port,
		Path:    "/metrics",
	}
	p := NewPrometheus(cfg, nil)

	ctx := context.Background()
	if err := p.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	defer p.Close()

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	// Test metrics endpoint
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", port))
	if err != nil {
		t.Fatalf("metrics request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("metrics status = %d, want 200", resp.StatusCode)
	}
}

func TestPrometheus_Write(t *testing.T) {
	port := findFreePort()
	cfg := config.PrometheusConfig{
		Enabled: true,
		Port:    port,
		Path:    "/metrics",
	}
	p := NewPrometheus(cfg, nil)

	ctx := context.Background()
	if err := p.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)
	defer p.Close()

	m := metrics.NewMetric("test_measurement").
		WithTag("device", "test_device").
		WithTag("address", "192.168.1.100").
		WithField("voltage", 120.5).
		WithField("power", 100.0)

	if err := p.Write(ctx, []*metrics.Metric{m}); err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// Verify collector was updated
	if p.Collector().DeviceCount() != 1 {
		t.Errorf("DeviceCount() = %d, want 1", p.Collector().DeviceCount())
	}
}

func TestPrometheus_Write_UsesDeviceTag(t *testing.T) {
	port := findFreePort()
	cfg := config.PrometheusConfig{
		Enabled: true,
		Port:    port,
		Path:    "/metrics",
	}
	p := NewPrometheus(cfg, nil)

	ctx := context.Background()
	if err := p.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)
	defer p.Close()

	// With device tag
	m1 := metrics.NewMetric("measurement1").
		WithTag("device", "device_from_tag").
		WithField("value", 1)

	// Without device tag - falls back to measurement name
	m2 := metrics.NewMetric("measurement2").
		WithField("value", 2)

	if err := p.Write(ctx, []*metrics.Metric{m1, m2}); err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// Both should be tracked as separate devices
	if p.Collector().DeviceCount() != 2 {
		t.Errorf("DeviceCount() = %d, want 2", p.Collector().DeviceCount())
	}
}

func TestPrometheus_Close(t *testing.T) {
	port := findFreePort()
	cfg := config.PrometheusConfig{
		Enabled: true,
		Port:    port,
		Path:    "/metrics",
	}
	p := NewPrometheus(cfg, nil)

	ctx := context.Background()
	if err := p.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if !p.Healthy() {
		t.Error("should be healthy before Close()")
	}

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	if err := p.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if p.Healthy() {
		t.Error("should not be healthy after Close()")
	}

	// Wait for server to stop
	time.Sleep(50 * time.Millisecond)

	// Server should be stopped
	_, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err == nil {
		t.Error("expected connection error after Close()")
	}
}

func TestPrometheus_Healthy(t *testing.T) {
	port := findFreePort()
	cfg := config.PrometheusConfig{
		Enabled: true,
		Port:    port,
		Path:    "/metrics",
	}
	p := NewPrometheus(cfg, nil)

	if p.Healthy() {
		t.Error("should not be healthy before Initialize()")
	}

	ctx := context.Background()
	if err := p.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	if !p.Healthy() {
		t.Error("should be healthy after Initialize()")
	}

	p.Close()

	if p.Healthy() {
		t.Error("should not be healthy after Close()")
	}
}

func TestPrometheus_Collector(t *testing.T) {
	cfg := config.PrometheusConfig{
		Port: 9090,
		Path: "/metrics",
	}
	p := NewPrometheus(cfg, nil)

	if p.Collector() == nil {
		t.Error("Collector() returned nil")
	}
}

func TestPrometheus_Close_NotInitialized(t *testing.T) {
	cfg := config.PrometheusConfig{
		Port: 9090,
		Path: "/metrics",
	}
	p := NewPrometheus(cfg, nil)

	// Close without Initialize should be safe
	if err := p.Close(); err != nil {
		t.Errorf("Close() without Initialize error = %v", err)
	}
}
