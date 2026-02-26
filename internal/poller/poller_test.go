package poller

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/danweinerdev/power-poller/internal/config"
	"github.com/danweinerdev/power-poller/internal/metrics"
)

// mockBackend is a test backend for the pipeline.
type mockBackend struct {
	name     string
	healthy  bool
	mu       sync.Mutex
	received [][]*metrics.Metric
	initErr  error
	writeErr error
}

func (m *mockBackend) Name() string {
	return m.name
}

func (m *mockBackend) Initialize(ctx context.Context) error {
	return m.initErr
}

func (m *mockBackend) Write(ctx context.Context, batch []*metrics.Metric) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.received = append(m.received, batch)
	return nil
}

func (m *mockBackend) Close() error {
	return nil
}

func (m *mockBackend) Healthy() bool {
	return m.healthy
}

func (m *mockBackend) ReceivedBatches() [][]*metrics.Metric {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.received
}

func newTestConfig() *config.Config {
	return &config.Config{
		Global: config.GlobalConfig{
			PollInterval:  config.Duration{Duration: 100 * time.Millisecond},
			DeviceTimeout: config.Duration{Duration: 2 * time.Second},
			BatchSize:     10,
			RetryAttempts: 1,
			RetryDelay:    config.Duration{Duration: 10 * time.Millisecond},
		},
		Devices:      make(map[string]config.DeviceConfig),
		Measurements: make(map[string]config.MeasurementConfig),
	}
}

func newTestPipeline() (*metrics.Pipeline, *mockBackend) {
	backend := &mockBackend{name: "test", healthy: true}
	pipeline := metrics.NewPipeline(metrics.PipelineConfig{
		BatchSize:     10,
		FlushInterval: 10 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
	})
	pipeline.AddBackend(backend)
	return pipeline, backend
}

func TestPoller_New(t *testing.T) {
	cfg := newTestConfig()
	pipeline, _ := newTestPipeline()

	t.Run("with logger", func(t *testing.T) {
		p := New(cfg, pipeline, nil)
		if p == nil {
			t.Fatal("New() returned nil")
		}
		if p.cfg != cfg {
			t.Error("config not set correctly")
		}
		if p.pipeline != pipeline {
			t.Error("pipeline not set correctly")
		}
		if p.worker == nil {
			t.Error("worker not created")
		}
	})
}

func TestPoller_IsRunning(t *testing.T) {
	cfg := newTestConfig()
	pipeline, _ := newTestPipeline()
	p := New(cfg, pipeline, nil)

	if p.IsRunning() {
		t.Error("should not be running initially")
	}
}

func TestPoller_Stats(t *testing.T) {
	cfg := newTestConfig()
	pipeline, _ := newTestPipeline()
	p := New(cfg, pipeline, nil)

	stats := p.Stats()
	if stats.TotalPolls != 0 {
		t.Errorf("TotalPolls = %d, want 0", stats.TotalPolls)
	}
	if stats.SuccessfulPolls != 0 {
		t.Errorf("SuccessfulPolls = %d, want 0", stats.SuccessfulPolls)
	}
	if stats.FailedPolls != 0 {
		t.Errorf("FailedPolls = %d, want 0", stats.FailedPolls)
	}
}

func TestPoller_LastPoll(t *testing.T) {
	cfg := newTestConfig()
	pipeline, _ := newTestPipeline()
	p := New(cfg, pipeline, nil)

	lastPoll := p.LastPoll()
	if !lastPoll.IsZero() {
		t.Errorf("LastPoll should be zero initially, got %v", lastPoll)
	}
}

func TestPoller_Run_ContextCancellation(t *testing.T) {
	cfg := newTestConfig()
	// No devices - poll will be fast
	pipeline, _ := newTestPipeline()

	ctx := context.Background()
	if err := pipeline.Start(ctx); err != nil {
		t.Fatalf("pipeline.Start() error = %v", err)
	}
	defer pipeline.Stop(ctx)

	p := New(cfg, pipeline, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- p.Run(ctx)
	}()

	select {
	case err := <-done:
		if err != context.DeadlineExceeded {
			t.Errorf("Run() error = %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}

	if p.IsRunning() {
		t.Error("should not be running after context cancelled")
	}
}

func TestPoller_Run_InitialPoll(t *testing.T) {
	cfg := newTestConfig()
	cfg.Global.PollInterval = config.Duration{Duration: 1 * time.Hour} // Very long interval

	pipeline, _ := newTestPipeline()
	ctx := context.Background()
	if err := pipeline.Start(ctx); err != nil {
		t.Fatalf("pipeline.Start() error = %v", err)
	}
	defer pipeline.Stop(ctx)

	p := New(cfg, pipeline, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	go p.Run(ctx)

	// Wait a bit for initial poll
	time.Sleep(20 * time.Millisecond)

	stats := p.Stats()
	if stats.TotalPolls < 1 {
		t.Errorf("expected at least 1 poll from initial poll, got %d", stats.TotalPolls)
	}
}

func TestPoller_Run_DoubleRun(t *testing.T) {
	cfg := newTestConfig()
	pipeline, _ := newTestPipeline()

	ctx := context.Background()
	if err := pipeline.Start(ctx); err != nil {
		t.Fatalf("pipeline.Start() error = %v", err)
	}
	defer pipeline.Stop(ctx)

	p := New(cfg, pipeline, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start first run
	go p.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	// Second run should return immediately (already running)
	done := make(chan error, 1)
	go func() {
		done <- p.Run(ctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("second Run() error = %v, want nil", err)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatal("second Run() should have returned immediately")
	}
}

func TestPoller_ReloadConfig(t *testing.T) {
	cfg := newTestConfig()
	pipeline, _ := newTestPipeline()
	p := New(cfg, pipeline, nil)

	newCfg := newTestConfig()
	newCfg.Devices["test_device"] = config.DeviceConfig{
		Address:      "192.168.1.100",
		Measurements: []string{"power_metrics"},
	}

	p.ReloadConfig(newCfg)

	if p.cfg != newCfg {
		t.Error("config not updated after reload")
	}
}

func TestPoller_Stats_ThreadSafe(t *testing.T) {
	cfg := newTestConfig()
	pipeline, _ := newTestPipeline()

	ctx := context.Background()
	if err := pipeline.Start(ctx); err != nil {
		t.Fatalf("pipeline.Start() error = %v", err)
	}
	defer pipeline.Stop(ctx)

	p := New(cfg, pipeline, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go p.Run(ctx)

	// Access stats concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = p.Stats()
				_ = p.LastPoll()
				_ = p.IsRunning()
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()
}
