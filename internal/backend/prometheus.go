package backend

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/danweinerdev/power-poller/internal/config"
	"github.com/danweinerdev/power-poller/internal/metrics"
)

// Prometheus implements the Backend interface for Prometheus.
// It runs an HTTP server that exposes metrics at the configured path.
type Prometheus struct {
	cfg       config.PrometheusConfig
	collector *metrics.Collector
	server    *http.Server
	logger    *slog.Logger

	mu      sync.RWMutex
	healthy bool
}

// NewPrometheus creates a new Prometheus backend.
func NewPrometheus(cfg config.PrometheusConfig, logger *slog.Logger) *Prometheus {
	if logger == nil {
		logger = slog.Default()
	}
	return &Prometheus{
		cfg:       cfg,
		collector: metrics.NewCollector(),
		logger:    logger,
		healthy:   false,
	}
}

// Name returns "prometheus".
func (p *Prometheus) Name() string {
	return "prometheus"
}

// Initialize starts the Prometheus HTTP server.
func (p *Prometheus) Initialize(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Register collector
	if err := prometheus.Register(p.collector); err != nil {
		// If already registered, that's okay
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			return fmt.Errorf("failed to register Prometheus collector: %w", err)
		}
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.Handle(p.cfg.Path, promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%d", p.cfg.Port)
	p.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start server in background
	go func() {
		p.logger.Info("starting Prometheus server", "addr", addr, "path", p.cfg.Path)
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			p.logger.Error("Prometheus server error", "error", err)
			p.mu.Lock()
			p.healthy = false
			p.mu.Unlock()
		}
	}()

	p.healthy = true
	return nil
}

// Write updates the Prometheus collector with new metrics.
func (p *Prometheus) Write(ctx context.Context, batch []*metrics.Metric) error {
	p.mu.RLock()
	collector := p.collector
	p.mu.RUnlock()

	if collector == nil {
		return fmt.Errorf("Prometheus not initialized")
	}

	for _, m := range batch {
		// Use device tag as the device name, or measurement if not available
		deviceName := m.Tags["device"]
		if deviceName == "" {
			deviceName = m.Measurement
		}
		collector.Update(deviceName, m)
	}

	p.logger.Debug("updated Prometheus metrics", "count", len(batch))
	return nil
}

// Close stops the Prometheus HTTP server.
func (p *Prometheus) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := p.server.Shutdown(ctx); err != nil {
			p.logger.Error("error shutting down Prometheus server", "error", err)
			return err
		}
		p.server = nil
	}

	p.healthy = false
	p.logger.Info("Prometheus server stopped")
	return nil
}

// Healthy returns true if the server is running.
func (p *Prometheus) Healthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.healthy
}

// Collector returns the underlying Prometheus collector.
func (p *Prometheus) Collector() *metrics.Collector {
	return p.collector
}
