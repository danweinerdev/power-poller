package cli

import (
	"fmt"
	"log/slog"

	"github.com/danweinerdev/go-power-poller/internal/backend"
	"github.com/danweinerdev/go-power-poller/internal/config"
	"github.com/danweinerdev/go-power-poller/internal/metrics"
)

// CreateMetricsPipeline creates and configures a metrics pipeline based on config.
// If echoMode is true, metrics are written to stdout instead of configured backends.
func CreateMetricsPipeline(cfg *config.Config, logger *slog.Logger, echoMode bool) (*metrics.Pipeline, error) {
	pipelineCfg := metrics.PipelineConfig{
		BatchSize:       cfg.Global.BatchSize,
		FlushInterval:   cfg.Global.PollInterval.Duration,
		RetryAttempts:   cfg.Global.RetryAttempts,
		RetryDelay:      cfg.Global.RetryDelay.Duration,
		CachePath:       cfg.Global.MetricsCachePath,
		CacheMaxMetrics: cfg.Global.MetricsCacheMaxMetrics,
		Logger:          logger,
	}
	pipeline := metrics.NewPipeline(pipelineCfg)

	if echoMode {
		pipeline.AddBackend(backend.NewEchoStdout(logger))
	} else {
		if cfg.InfluxDB.Enabled {
			pipeline.AddBackend(backend.NewInfluxDB(cfg.InfluxDB, logger))
		}
		if cfg.Prometheus.Enabled {
			pipeline.AddBackend(backend.NewPrometheus(cfg.Prometheus, logger))
		}
	}

	if pipeline.BackendCount() == 0 {
		return nil, fmt.Errorf("no backends configured (enable influxdb, prometheus, or use echo mode)")
	}

	return pipeline, nil
}
