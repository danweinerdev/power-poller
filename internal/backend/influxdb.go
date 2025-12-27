package backend

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"

	"github.com/danweinerdev/go-power-poller/internal/config"
	"github.com/danweinerdev/go-power-poller/internal/metrics"
)

// InfluxDB implements the Backend interface for InfluxDB 2.x.
type InfluxDB struct {
	cfg    config.InfluxDBConfig
	client influxdb2.Client
	writer api.WriteAPIBlocking
	logger *slog.Logger

	mu      sync.RWMutex
	healthy bool
}

// NewInfluxDB creates a new InfluxDB backend.
func NewInfluxDB(cfg config.InfluxDBConfig, logger *slog.Logger) *InfluxDB {
	if logger == nil {
		logger = slog.Default()
	}
	return &InfluxDB{
		cfg:     cfg,
		logger:  logger,
		healthy: false,
	}
}

// Name returns "influxdb".
func (i *InfluxDB) Name() string {
	return "influxdb"
}

// Initialize connects to InfluxDB.
func (i *InfluxDB) Initialize(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	url := i.cfg.URL()
	i.logger.Info("connecting to InfluxDB", "url", url, "org", i.cfg.Org, "bucket", i.cfg.Bucket)

	// Create client
	opts := influxdb2.DefaultOptions()
	i.client = influxdb2.NewClientWithOptions(url, i.cfg.Token, opts)

	// Test connection
	health, err := i.client.Health(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to InfluxDB: %w", err)
	}

	if health.Status != "pass" {
		return fmt.Errorf("InfluxDB health check failed: %s", health.Status)
	}

	// Create blocking write API
	i.writer = i.client.WriteAPIBlocking(i.cfg.Org, i.cfg.Bucket)
	i.healthy = true

	i.logger.Info("connected to InfluxDB", "version", *health.Version)
	return nil
}

// Write sends metrics to InfluxDB.
func (i *InfluxDB) Write(ctx context.Context, batch []*metrics.Metric) error {
	if len(batch) == 0 {
		return nil
	}

	i.mu.RLock()
	if i.writer == nil {
		i.mu.RUnlock()
		return fmt.Errorf("InfluxDB not initialized")
	}
	writer := i.writer
	i.mu.RUnlock()

	// Convert metrics to InfluxDB points
	points := make([]*write.Point, 0, len(batch))
	for _, m := range batch {
		point := influxdb2.NewPoint(
			m.Measurement,
			m.Tags,
			m.Fields,
			m.Timestamp,
		)
		points = append(points, point)
	}

	// Write points
	if err := writer.WritePoint(ctx, points...); err != nil {
		i.mu.Lock()
		i.healthy = false
		i.mu.Unlock()
		return fmt.Errorf("failed to write to InfluxDB: %w", err)
	}

	i.mu.Lock()
	i.healthy = true
	i.mu.Unlock()

	i.logger.Debug("wrote metrics to InfluxDB", "count", len(batch))
	return nil
}

// Close closes the InfluxDB connection.
func (i *InfluxDB) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.client != nil {
		i.client.Close()
		i.client = nil
		i.writer = nil
	}
	i.healthy = false

	i.logger.Info("InfluxDB connection closed")
	return nil
}

// Healthy returns true if the connection is healthy.
func (i *InfluxDB) Healthy() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.healthy
}
