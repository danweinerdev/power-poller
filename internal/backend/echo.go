package backend

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/danweinerdev/power-poller/internal/metrics"
)

// Echo is a debug backend that writes metrics to an io.Writer.
type Echo struct {
	writer io.Writer
	logger *slog.Logger

	mu      sync.RWMutex
	healthy bool
}

// NewEcho creates a new Echo backend that writes to the given writer.
func NewEcho(w io.Writer, logger *slog.Logger) *Echo {
	if w == nil {
		w = os.Stdout
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Echo{
		writer:  w,
		logger:  logger,
		healthy: true,
	}
}

// NewEchoStdout creates an Echo backend that writes to stdout.
func NewEchoStdout(logger *slog.Logger) *Echo {
	return NewEcho(os.Stdout, logger)
}

// Name returns "echo".
func (e *Echo) Name() string {
	return "echo"
}

// Initialize is a no-op for Echo.
func (e *Echo) Initialize(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.healthy = true
	e.logger.Info("echo backend initialized")
	return nil
}

// Write outputs metrics in line protocol format.
func (e *Echo) Write(ctx context.Context, batch []*metrics.Metric) error {
	e.mu.RLock()
	writer := e.writer
	e.mu.RUnlock()

	if writer == nil {
		return fmt.Errorf("echo backend not initialized")
	}

	for _, m := range batch {
		line := m.ToLineProtocol()
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("failed to write metric: %w", err)
		}
	}

	e.logger.Debug("echoed metrics", "count", len(batch))
	return nil
}

// Close is a no-op for Echo.
func (e *Echo) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.healthy = false
	e.logger.Info("echo backend closed")
	return nil
}

// Healthy returns true.
func (e *Echo) Healthy() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.healthy
}
