package backend

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/danweinerdev/power-poller/internal/metrics"
)

// mockBackend is a configurable mock backend for testing.
type mockBackend struct {
	name        string
	healthy     bool
	initErr     error
	writeErr    error
	closeErr    error
	mu          sync.Mutex
	initCalled  bool
	writeCalled bool
	closeCalled bool
	received    [][]*metrics.Metric
}

func (m *mockBackend) Name() string {
	return m.name
}

func (m *mockBackend) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initCalled = true
	return m.initErr
}

func (m *mockBackend) Write(ctx context.Context, batch []*metrics.Metric) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCalled = true
	if m.writeErr != nil {
		return m.writeErr
	}
	m.received = append(m.received, batch)
	return nil
}

func (m *mockBackend) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCalled = true
	return m.closeErr
}

func (m *mockBackend) Healthy() bool {
	return m.healthy
}

func (m *mockBackend) InitCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.initCalled
}

func (m *mockBackend) WriteCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeCalled
}

func (m *mockBackend) CloseCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeCalled
}

func (m *mockBackend) ReceivedBatches() [][]*metrics.Metric {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.received
}

func TestMultiBackend_New(t *testing.T) {
	b1 := &mockBackend{name: "b1", healthy: true}
	b2 := &mockBackend{name: "b2", healthy: true}

	m := NewMultiBackend(b1, b2)
	if m == nil {
		t.Fatal("NewMultiBackend() returned nil")
	}
	if m.Name() != "multi" {
		t.Errorf("Name() = %q, want %q", m.Name(), "multi")
	}
}

func TestMultiBackend_Initialize_AllSucceed(t *testing.T) {
	b1 := &mockBackend{name: "b1", healthy: true}
	b2 := &mockBackend{name: "b2", healthy: true}

	m := NewMultiBackend(b1, b2)

	ctx := context.Background()
	if err := m.Initialize(ctx); err != nil {
		t.Errorf("Initialize() error = %v", err)
	}

	if !b1.InitCalled() {
		t.Error("b1.Initialize() was not called")
	}
	if !b2.InitCalled() {
		t.Error("b2.Initialize() was not called")
	}
}

func TestMultiBackend_Initialize_FirstFails(t *testing.T) {
	b1 := &mockBackend{name: "b1", healthy: true, initErr: errors.New("init failed")}
	b2 := &mockBackend{name: "b2", healthy: true}

	m := NewMultiBackend(b1, b2)

	ctx := context.Background()
	err := m.Initialize(ctx)
	if err == nil {
		t.Error("expected error when first backend fails")
	}

	// First should be called, but second should not be (early exit)
	if !b1.InitCalled() {
		t.Error("b1.Initialize() was not called")
	}
	if b2.InitCalled() {
		t.Error("b2.Initialize() should not be called after first fails")
	}
}

func TestMultiBackend_Write_AllSucceed(t *testing.T) {
	b1 := &mockBackend{name: "b1", healthy: true}
	b2 := &mockBackend{name: "b2", healthy: true}

	m := NewMultiBackend(b1, b2)

	batch := []*metrics.Metric{
		metrics.NewMetric("test").WithField("value", 1),
	}

	ctx := context.Background()
	if err := m.Write(ctx, batch); err != nil {
		t.Errorf("Write() error = %v", err)
	}

	if !b1.WriteCalled() {
		t.Error("b1.Write() was not called")
	}
	if !b2.WriteCalled() {
		t.Error("b2.Write() was not called")
	}

	// Both should have received the batch
	if len(b1.ReceivedBatches()) != 1 {
		t.Errorf("b1 received %d batches, want 1", len(b1.ReceivedBatches()))
	}
	if len(b2.ReceivedBatches()) != 1 {
		t.Errorf("b2 received %d batches, want 1", len(b2.ReceivedBatches()))
	}
}

func TestMultiBackend_Write_PartialFailure(t *testing.T) {
	b1 := &mockBackend{name: "b1", healthy: true}
	b2 := &mockBackend{name: "b2", healthy: true, writeErr: errors.New("write failed")}
	b3 := &mockBackend{name: "b3", healthy: true}

	m := NewMultiBackend(b1, b2, b3)

	batch := []*metrics.Metric{
		metrics.NewMetric("test").WithField("value", 1),
	}

	ctx := context.Background()
	err := m.Write(ctx, batch)

	// Should return error from failing backend
	if err == nil {
		t.Error("expected error when one backend fails")
	}

	// All backends should have been called
	if !b1.WriteCalled() {
		t.Error("b1.Write() was not called")
	}
	if !b2.WriteCalled() {
		t.Error("b2.Write() was not called")
	}
	if !b3.WriteCalled() {
		t.Error("b3.Write() was not called (write should continue despite failures)")
	}

	// b1 and b3 should have received the batch
	if len(b1.ReceivedBatches()) != 1 {
		t.Errorf("b1 received %d batches, want 1", len(b1.ReceivedBatches()))
	}
	if len(b3.ReceivedBatches()) != 1 {
		t.Errorf("b3 received %d batches, want 1", len(b3.ReceivedBatches()))
	}
}

func TestMultiBackend_Close_AllSucceed(t *testing.T) {
	b1 := &mockBackend{name: "b1", healthy: true}
	b2 := &mockBackend{name: "b2", healthy: true}

	m := NewMultiBackend(b1, b2)

	if err := m.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if !b1.CloseCalled() {
		t.Error("b1.Close() was not called")
	}
	if !b2.CloseCalled() {
		t.Error("b2.Close() was not called")
	}
}

func TestMultiBackend_Close_PartialFailure(t *testing.T) {
	b1 := &mockBackend{name: "b1", healthy: true}
	b2 := &mockBackend{name: "b2", healthy: true, closeErr: errors.New("close failed")}
	b3 := &mockBackend{name: "b3", healthy: true}

	m := NewMultiBackend(b1, b2, b3)

	err := m.Close()

	// Should return error from failing backend
	if err == nil {
		t.Error("expected error when one backend fails")
	}

	// All backends should have been closed
	if !b1.CloseCalled() {
		t.Error("b1.Close() was not called")
	}
	if !b2.CloseCalled() {
		t.Error("b2.Close() was not called")
	}
	if !b3.CloseCalled() {
		t.Error("b3.Close() was not called (close should continue despite failures)")
	}
}

func TestMultiBackend_Healthy_AllHealthy(t *testing.T) {
	b1 := &mockBackend{name: "b1", healthy: true}
	b2 := &mockBackend{name: "b2", healthy: true}

	m := NewMultiBackend(b1, b2)

	if !m.Healthy() {
		t.Error("should be healthy when all backends are healthy")
	}
}

func TestMultiBackend_Healthy_OneUnhealthy(t *testing.T) {
	b1 := &mockBackend{name: "b1", healthy: true}
	b2 := &mockBackend{name: "b2", healthy: false}

	m := NewMultiBackend(b1, b2)

	if m.Healthy() {
		t.Error("should not be healthy when one backend is unhealthy")
	}
}

func TestMultiBackend_Healthy_AllUnhealthy(t *testing.T) {
	b1 := &mockBackend{name: "b1", healthy: false}
	b2 := &mockBackend{name: "b2", healthy: false}

	m := NewMultiBackend(b1, b2)

	if m.Healthy() {
		t.Error("should not be healthy when all backends are unhealthy")
	}
}

func TestMultiBackend_Empty(t *testing.T) {
	m := NewMultiBackend()

	// Should work with no backends
	ctx := context.Background()

	if err := m.Initialize(ctx); err != nil {
		t.Errorf("Initialize() error = %v", err)
	}

	batch := []*metrics.Metric{
		metrics.NewMetric("test").WithField("value", 1),
	}
	if err := m.Write(ctx, batch); err != nil {
		t.Errorf("Write() error = %v", err)
	}

	if err := m.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Empty multi-backend is considered healthy
	if !m.Healthy() {
		t.Error("empty multi-backend should be healthy")
	}
}
