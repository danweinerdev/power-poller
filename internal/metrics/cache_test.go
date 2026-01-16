package metrics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCache_NewDisabled(t *testing.T) {
	c := NewCache(CacheConfig{Path: ""})
	if c.Enabled() {
		t.Error("cache should be disabled when path is empty")
	}
}

func TestCache_NewEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cache")

	c := NewCache(CacheConfig{Path: path})
	if !c.Enabled() {
		t.Error("cache should be enabled when path is set")
	}
	if c.Count() != 0 {
		t.Errorf("new cache should be empty, got %d", c.Count())
	}
}

func TestCache_AppendAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cache")

	c := NewCache(CacheConfig{Path: path, MaxMetrics: 100})

	// Create some metrics
	metrics := []*Metric{
		NewMetric("test1").WithTag("host", "server1").WithField("value", 1.5),
		NewMetric("test2").WithTag("host", "server2").WithField("count", 42),
	}

	// Append metrics
	if err := c.Append(metrics); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	if c.Count() != 2 {
		t.Errorf("Count() = %d, want 2", c.Count())
	}

	// Load metrics back
	loaded, err := c.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("Load() returned %d metrics, want 2", len(loaded))
	}

	// Verify content
	if loaded[0].Measurement != "test1" {
		t.Errorf("loaded[0].Measurement = %s, want test1", loaded[0].Measurement)
	}
	if loaded[0].Tags["host"] != "server1" {
		t.Errorf("loaded[0].Tags[host] = %s, want server1", loaded[0].Tags["host"])
	}
	if loaded[0].Fields["value"] != 1.5 {
		t.Errorf("loaded[0].Fields[value] = %v, want 1.5", loaded[0].Fields["value"])
	}
}

func TestCache_AppendMultipleTimes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cache")

	c := NewCache(CacheConfig{Path: path, MaxMetrics: 100})

	// Append in batches
	for i := 0; i < 3; i++ {
		metrics := []*Metric{
			NewMetric("test").WithField("batch", i),
		}
		if err := c.Append(metrics); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	if c.Count() != 3 {
		t.Errorf("Count() = %d, want 3", c.Count())
	}

	loaded, err := c.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded) != 3 {
		t.Errorf("Load() returned %d metrics, want 3", len(loaded))
	}
}

func TestCache_Clear(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cache")

	c := NewCache(CacheConfig{Path: path, MaxMetrics: 100})

	// Add some metrics
	metrics := []*Metric{
		NewMetric("test").WithField("value", 1),
	}
	if err := c.Append(metrics); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	if c.Count() != 1 {
		t.Errorf("Count() = %d, want 1", c.Count())
	}

	// Clear
	if err := c.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if c.Count() != 0 {
		t.Errorf("Count() after clear = %d, want 0", c.Count())
	}

	// Verify file is gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("cache file should not exist after clear")
	}
}

func TestCache_MaxMetrics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cache")

	c := NewCache(CacheConfig{Path: path, MaxMetrics: 5})

	// Add metrics up to limit
	for i := 0; i < 5; i++ {
		metrics := []*Metric{
			NewMetric("test").WithField("value", i),
		}
		if err := c.Append(metrics); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	if c.Count() != 5 {
		t.Errorf("Count() = %d, want 5", c.Count())
	}

	// Try to add more - should fail
	metrics := []*Metric{
		NewMetric("test").WithField("value", 999),
	}
	err := c.Append(metrics)
	if err == nil {
		t.Error("Append() should fail when cache is full")
	}

	// Count should still be 5
	if c.Count() != 5 {
		t.Errorf("Count() after full = %d, want 5", c.Count())
	}
}

func TestCache_FlushTo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cache")

	c := NewCache(CacheConfig{Path: path, MaxMetrics: 100})

	// Add some metrics
	metrics := []*Metric{
		NewMetric("test1").WithField("value", 1),
		NewMetric("test2").WithField("value", 2),
	}
	if err := c.Append(metrics); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// Create mock backend
	backend := &mockCacheBackend{healthy: true}

	// Flush to backend
	flushed, err := c.FlushTo(context.Background(), backend)
	if err != nil {
		t.Fatalf("FlushTo() error = %v", err)
	}

	if flushed != 2 {
		t.Errorf("FlushTo() = %d, want 2", flushed)
	}

	// Verify backend received metrics
	if len(backend.received) != 2 {
		t.Errorf("backend received %d metrics, want 2", len(backend.received))
	}

	// Cache should be cleared
	if c.Count() != 0 {
		t.Errorf("Count() after flush = %d, want 0", c.Count())
	}
}

func TestCache_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.cache")

	c := NewCache(CacheConfig{Path: path})

	loaded, err := c.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded) != 0 {
		t.Errorf("Load() returned %d metrics for nonexistent file, want 0", len(loaded))
	}
}

func TestCache_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cache")

	// Create cache and add metrics
	c1 := NewCache(CacheConfig{Path: path, MaxMetrics: 100})
	metrics := []*Metric{
		NewMetric("test").WithTag("key", "value").WithField("count", 42),
	}
	if err := c1.Append(metrics); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// Create new cache instance with same path (simulating restart)
	c2 := NewCache(CacheConfig{Path: path, MaxMetrics: 100})

	// Should load existing metrics
	if c2.Count() != 1 {
		t.Errorf("Count() on new instance = %d, want 1", c2.Count())
	}

	loaded, err := c2.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("Load() returned %d metrics, want 1", len(loaded))
	}

	if loaded[0].Measurement != "test" {
		t.Errorf("loaded[0].Measurement = %s, want test", loaded[0].Measurement)
	}
}

func TestCache_TimestampPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cache")

	c := NewCache(CacheConfig{Path: path, MaxMetrics: 100})

	// Create metric with specific timestamp
	ts := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	m := NewMetric("test").WithField("value", 1).WithTimestamp(ts)

	if err := c.Append([]*Metric{m}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	loaded, err := c.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !loaded[0].Timestamp.Equal(ts) {
		t.Errorf("timestamp not preserved: got %v, want %v", loaded[0].Timestamp, ts)
	}
}

// mockCacheBackend is a simple mock for testing cache flush
type mockCacheBackend struct {
	healthy  bool
	received []*Metric
}

func (m *mockCacheBackend) Name() string                         { return "mock" }
func (m *mockCacheBackend) Initialize(ctx context.Context) error { return nil }
func (m *mockCacheBackend) Close() error                         { return nil }
func (m *mockCacheBackend) Healthy() bool                        { return m.healthy }

func (m *mockCacheBackend) Write(ctx context.Context, metrics []*Metric) error {
	m.received = append(m.received, metrics...)
	return nil
}
