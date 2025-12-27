package backend

import (
	"context"

	"github.com/danweinerdev/go-power-poller/internal/metrics"
)

// Backend defines the interface for metric storage backends.
// This is an alias to metrics.Backend for convenience.
type Backend = metrics.Backend

// Compile-time checks that backends implement the interface.
var (
	_ Backend = (*InfluxDB)(nil)
	_ Backend = (*Prometheus)(nil)
	_ Backend = (*Echo)(nil)
)

// MultiBackend wraps multiple backends and writes to all of them.
type MultiBackend struct {
	backends []Backend
}

// NewMultiBackend creates a backend that writes to multiple destinations.
func NewMultiBackend(backends ...Backend) *MultiBackend {
	return &MultiBackend{backends: backends}
}

// Name returns "multi".
func (m *MultiBackend) Name() string {
	return "multi"
}

// Initialize initializes all backends.
func (m *MultiBackend) Initialize(ctx context.Context) error {
	for _, b := range m.backends {
		if err := b.Initialize(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Write writes to all backends.
func (m *MultiBackend) Write(ctx context.Context, batch []*metrics.Metric) error {
	var lastErr error
	for _, b := range m.backends {
		if err := b.Write(ctx, batch); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Close closes all backends.
func (m *MultiBackend) Close() error {
	var lastErr error
	for _, b := range m.backends {
		if err := b.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Healthy returns true if all backends are healthy.
func (m *MultiBackend) Healthy() bool {
	for _, b := range m.backends {
		if !b.Healthy() {
			return false
		}
	}
	return true
}
