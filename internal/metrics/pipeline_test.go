package metrics

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockBackend is a test backend for the pipeline.
type mockBackend struct {
	name        string
	healthy     bool
	mu          sync.Mutex
	received    [][]*Metric
	initErr     error
	writeErr    error
	writeCalls  int32
	closeCalled bool
}

func (m *mockBackend) Name() string {
	return m.name
}

func (m *mockBackend) Initialize(ctx context.Context) error {
	return m.initErr
}

func (m *mockBackend) Write(ctx context.Context, batch []*Metric) error {
	atomic.AddInt32(&m.writeCalls, 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.received = append(m.received, batch)
	return nil
}

func (m *mockBackend) Close() error {
	m.closeCalled = true
	return nil
}

func (m *mockBackend) Healthy() bool {
	return m.healthy
}

func (m *mockBackend) ReceivedBatches() [][]*Metric {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.received
}

func (m *mockBackend) WriteCalls() int {
	return int(atomic.LoadInt32(&m.writeCalls))
}

func TestPipeline_New_Defaults(t *testing.T) {
	t.Run("with zero values uses defaults", func(t *testing.T) {
		p := NewPipeline(PipelineConfig{})

		if p.batchSize != 10 {
			t.Errorf("batchSize = %d, want 10", p.batchSize)
		}
		if p.flushInterval != 10*time.Second {
			t.Errorf("flushInterval = %v, want 10s", p.flushInterval)
		}
		if p.retryAttempts != 3 {
			t.Errorf("retryAttempts = %d, want 3", p.retryAttempts)
		}
		if p.retryDelay != 1*time.Second {
			t.Errorf("retryDelay = %v, want 1s", p.retryDelay)
		}
	})

	t.Run("with custom values", func(t *testing.T) {
		p := NewPipeline(PipelineConfig{
			BatchSize:     5,
			FlushInterval: 5 * time.Second,
			RetryAttempts: 2,
			RetryDelay:    500 * time.Millisecond,
		})

		if p.batchSize != 5 {
			t.Errorf("batchSize = %d, want 5", p.batchSize)
		}
		if p.flushInterval != 5*time.Second {
			t.Errorf("flushInterval = %v, want 5s", p.flushInterval)
		}
	})
}

func TestPipeline_AddBackend(t *testing.T) {
	p := NewPipeline(DefaultPipelineConfig())
	backend := &mockBackend{name: "test", healthy: true}

	if p.BackendCount() != 0 {
		t.Errorf("BackendCount() = %d, want 0", p.BackendCount())
	}

	p.AddBackend(backend)

	if p.BackendCount() != 1 {
		t.Errorf("BackendCount() = %d, want 1", p.BackendCount())
	}
}

func TestPipeline_Start_Success(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		BatchSize:     10,
		FlushInterval: 100 * time.Millisecond,
	})
	backend := &mockBackend{name: "test", healthy: true}
	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop(ctx)
}

func TestPipeline_Start_BackendInitError(t *testing.T) {
	p := NewPipeline(DefaultPipelineConfig())
	backend := &mockBackend{
		name:    "failing",
		healthy: true,
		initErr: errors.New("init failed"),
	}
	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err == nil {
		p.Stop(ctx)
		t.Error("expected error from failing backend init")
	}
}

func TestPipeline_Push_ValidMetric(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		BatchSize:     100, // High batch size to prevent auto-flush
		FlushInterval: 1 * time.Hour,
	})
	backend := &mockBackend{name: "test", healthy: true}
	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop(ctx)

	m := NewMetric("test").WithField("value", 123)
	p.Push(m)

	if p.BufferLen() != 1 {
		t.Errorf("BufferLen() = %d, want 1", p.BufferLen())
	}
}

func TestPipeline_Push_InvalidMetric(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		BatchSize:     100,
		FlushInterval: 1 * time.Hour,
	})
	backend := &mockBackend{name: "test", healthy: true}
	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop(ctx)

	// Invalid metric - no fields
	m := NewMetric("test")
	p.Push(m)

	// Should be dropped
	if p.BufferLen() != 0 {
		t.Errorf("BufferLen() = %d, want 0 (invalid metric should be dropped)", p.BufferLen())
	}
}

func TestPipeline_Push_AutoFlush(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		BatchSize:     3,
		FlushInterval: 1 * time.Hour,
		RetryAttempts: 1,
	})
	backend := &mockBackend{name: "test", healthy: true}
	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop(ctx)

	// Push enough metrics to trigger auto-flush
	for i := 0; i < 3; i++ {
		m := NewMetric("test").WithField("value", i)
		p.Push(m)
	}

	// Give time for async flush
	time.Sleep(50 * time.Millisecond)

	if backend.WriteCalls() < 1 {
		t.Error("expected at least one write call after reaching batch size")
	}
}

func TestPipeline_Flush_Empty(t *testing.T) {
	p := NewPipeline(DefaultPipelineConfig())
	backend := &mockBackend{name: "test", healthy: true}
	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop(ctx)

	// Flush with empty buffer
	if err := p.Flush(ctx); err != nil {
		t.Errorf("Flush() empty buffer error = %v", err)
	}

	if backend.WriteCalls() != 0 {
		t.Errorf("WriteCalls() = %d, want 0 for empty buffer", backend.WriteCalls())
	}
}

func TestPipeline_Flush_Success(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		BatchSize:     100,
		FlushInterval: 1 * time.Hour,
	})
	backend := &mockBackend{name: "test", healthy: true}
	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop(ctx)

	// Push some metrics
	for i := 0; i < 5; i++ {
		m := NewMetric("test").WithField("value", i)
		p.Push(m)
	}

	if err := p.Flush(ctx); err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	batches := backend.ReceivedBatches()
	if len(batches) != 1 {
		t.Fatalf("received %d batches, want 1", len(batches))
	}
	if len(batches[0]) != 5 {
		t.Errorf("batch size = %d, want 5", len(batches[0]))
	}

	// Buffer should be empty after flush
	if p.BufferLen() != 0 {
		t.Errorf("BufferLen() after flush = %d, want 0", p.BufferLen())
	}
}

// flakyBackend fails the first N writes then succeeds.
type flakyBackend struct {
	name       string
	healthy    bool
	failCount  int
	maxFails   int
	writeCalls int32
}

func (f *flakyBackend) Name() string                         { return f.name }
func (f *flakyBackend) Initialize(ctx context.Context) error { return nil }
func (f *flakyBackend) Close() error                         { return nil }
func (f *flakyBackend) Healthy() bool                        { return f.healthy }

func (f *flakyBackend) Write(ctx context.Context, batch []*Metric) error {
	atomic.AddInt32(&f.writeCalls, 1)
	f.failCount++
	if f.failCount <= f.maxFails {
		return errors.New("temporary failure")
	}
	return nil
}

func (f *flakyBackend) WriteCalls() int {
	return int(atomic.LoadInt32(&f.writeCalls))
}

func TestPipeline_Flush_Retry(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		BatchSize:     100,
		FlushInterval: 1 * time.Hour,
		RetryAttempts: 3,
		RetryDelay:    10 * time.Millisecond,
	})

	backend := &flakyBackend{
		name:     "flaky",
		healthy:  true,
		maxFails: 2, // Fail twice, then succeed
	}

	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop(ctx)

	m := NewMetric("test").WithField("value", 1)
	p.Push(m)

	if err := p.Flush(ctx); err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	if backend.WriteCalls() != 3 {
		t.Errorf("expected 3 write attempts (2 failures + 1 success), got %d", backend.WriteCalls())
	}
}

func TestPipeline_Flush_RetryExhausted(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		BatchSize:     100,
		FlushInterval: 1 * time.Hour,
		RetryAttempts: 2,
		RetryDelay:    10 * time.Millisecond,
	})

	backend := &mockBackend{
		name:     "failing",
		healthy:  true,
		writeErr: errors.New("persistent failure"),
	}
	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop(ctx)

	m := NewMetric("test").WithField("value", 1)
	p.Push(m)

	err := p.Flush(ctx)
	if err == nil {
		t.Error("expected error after retries exhausted")
	}
}

func TestPipeline_Flush_SkipsUnhealthyBackendWithinRecoverInterval(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		BatchSize:       10,
		FlushInterval:   10 * time.Second,
		RetryAttempts:   1,
		RetryDelay:      10 * time.Millisecond,
		RecoverInterval: 1 * time.Hour, // Long interval so we don't recover
	})
	backend := &mockBackend{
		name:    "unhealthy",
		healthy: false,
	}
	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop(ctx)

	// Record a recent attempt so we're within the recovery interval
	p.recordAttempt(backend.Name())

	m := NewMetric("test").WithField("value", 1)
	p.Push(m)

	// Should not error - just skip unhealthy backend
	if err := p.Flush(ctx); err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	if backend.WriteCalls() != 0 {
		t.Errorf("unhealthy backend should not receive writes within recovery interval")
	}
}

func TestPipeline_Flush_AttemptsRecoveryAfterInterval(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		BatchSize:       10,
		FlushInterval:   10 * time.Second,
		RetryAttempts:   1,
		RetryDelay:      10 * time.Millisecond,
		RecoverInterval: 10 * time.Millisecond, // Short interval for testing
	})
	backend := &mockBackend{
		name:    "unhealthy",
		healthy: false,
	}
	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop(ctx)

	// Record an old attempt
	p.mu.Lock()
	p.lastAttempt[backend.Name()] = time.Now().Add(-1 * time.Second)
	p.mu.Unlock()

	m := NewMetric("test").WithField("value", 1)
	p.Push(m)

	// Should attempt recovery since interval has passed
	p.Flush(ctx) // May error since backend is unhealthy, that's expected

	if backend.WriteCalls() == 0 {
		t.Errorf("expected recovery attempt for unhealthy backend after interval")
	}
}

func TestPipeline_Flush_RecoverySucceeds(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		BatchSize:       10,
		FlushInterval:   10 * time.Second,
		RetryAttempts:   1,
		RetryDelay:      10 * time.Millisecond,
		RecoverInterval: 10 * time.Millisecond,
	})

	// Start unhealthy, but write will succeed and set healthy=true
	backend := &mockBackend{
		name:    "recovering",
		healthy: false,
	}
	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop(ctx)

	// Push and flush - first attempt is a recovery attempt
	m := NewMetric("test").WithField("value", 1)
	p.Push(m)

	if err := p.Flush(ctx); err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	if backend.WriteCalls() != 1 {
		t.Errorf("expected 1 write call, got %d", backend.WriteCalls())
	}

	// Backend should now be healthy (mockBackend.Write sets healthy=true)
	// Push another metric - should write normally now
	backend.healthy = true // Simulate successful recovery
	m2 := NewMetric("test").WithField("value", 2)
	p.Push(m2)

	if err := p.Flush(ctx); err != nil {
		t.Errorf("second Flush() error = %v", err)
	}

	if backend.WriteCalls() != 2 {
		t.Errorf("expected 2 write calls after recovery, got %d", backend.WriteCalls())
	}
}

func TestPipeline_Stop_FinalFlush(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		BatchSize:     100,
		FlushInterval: 1 * time.Hour,
	})
	backend := &mockBackend{name: "test", healthy: true}
	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Push metrics without flushing
	for i := 0; i < 5; i++ {
		m := NewMetric("test").WithField("value", i)
		p.Push(m)
	}

	// Stop should flush remaining metrics
	if err := p.Stop(ctx); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	batches := backend.ReceivedBatches()
	if len(batches) != 1 {
		t.Errorf("expected final flush, got %d batches", len(batches))
	}

	if !backend.closeCalled {
		t.Error("backend Close() was not called")
	}
}

func TestPipeline_ConcurrentPush(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		BatchSize:     1000, // High to prevent auto-flush
		FlushInterval: 1 * time.Hour,
	})
	backend := &mockBackend{name: "test", healthy: true}
	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				m := NewMetric("test").WithField("value", n*10+j)
				p.Push(m)
			}
		}(i)
	}
	wg.Wait()

	// Should have 100 metrics buffered
	if p.BufferLen() != 100 {
		t.Errorf("BufferLen() = %d, want 100", p.BufferLen())
	}
}

func TestPipeline_BufferLen(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		BatchSize:     100,
		FlushInterval: 1 * time.Hour,
	})

	if p.BufferLen() != 0 {
		t.Errorf("initial BufferLen() = %d, want 0", p.BufferLen())
	}

	backend := &mockBackend{name: "test", healthy: true}
	p.AddBackend(backend)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop(ctx)

	m := NewMetric("test").WithField("value", 1)
	p.Push(m)

	if p.BufferLen() != 1 {
		t.Errorf("BufferLen() after push = %d, want 1", p.BufferLen())
	}
}

func TestPipeline_BackendCount(t *testing.T) {
	p := NewPipeline(DefaultPipelineConfig())

	if p.BackendCount() != 0 {
		t.Errorf("initial BackendCount() = %d, want 0", p.BackendCount())
	}

	p.AddBackend(&mockBackend{name: "b1", healthy: true})
	p.AddBackend(&mockBackend{name: "b2", healthy: true})

	if p.BackendCount() != 2 {
		t.Errorf("BackendCount() = %d, want 2", p.BackendCount())
	}
}

func TestDefaultPipelineConfig(t *testing.T) {
	cfg := DefaultPipelineConfig()

	if cfg.BatchSize != 10 {
		t.Errorf("BatchSize = %d, want 10", cfg.BatchSize)
	}
	if cfg.FlushInterval != 10*time.Second {
		t.Errorf("FlushInterval = %v, want 10s", cfg.FlushInterval)
	}
	if cfg.RetryAttempts != 3 {
		t.Errorf("RetryAttempts = %d, want 3", cfg.RetryAttempts)
	}
	if cfg.RetryDelay != 1*time.Second {
		t.Errorf("RetryDelay = %v, want 1s", cfg.RetryDelay)
	}
}
