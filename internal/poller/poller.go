package poller

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/danweinerdev/go-power-poller/internal/config"
	"github.com/danweinerdev/go-power-poller/internal/metrics"
)

// Poller manages the main polling loop.
type Poller struct {
	cfg      *config.Config
	pipeline *metrics.Pipeline
	worker   *Worker
	logger   *slog.Logger

	mu       sync.RWMutex
	running  bool
	lastPoll time.Time
	stats    Stats
}

// Stats holds polling statistics.
type Stats struct {
	TotalPolls     int64
	SuccessfulPolls int64
	FailedPolls    int64
	TotalMetrics   int64
	LastPollDuration time.Duration
}

// New creates a new Poller.
func New(cfg *config.Config, pipeline *metrics.Pipeline, logger *slog.Logger) *Poller {
	if logger == nil {
		logger = slog.Default()
	}
	return &Poller{
		cfg:      cfg,
		pipeline: pipeline,
		worker:   NewWorker(cfg, logger),
		logger:   logger,
	}
}

// Run starts the polling loop and blocks until context is cancelled.
func (p *Poller) Run(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return nil
	}
	p.running = true
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	interval := p.cfg.Global.PollInterval.Duration
	p.logger.Info("starting poller",
		"interval", interval,
		"devices", len(p.cfg.Devices),
	)

	// Do initial poll immediately
	p.doPoll(ctx)

	// Start ticker for subsequent polls
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("poller stopping", "reason", ctx.Err())
			return ctx.Err()
		case <-ticker.C:
			p.doPoll(ctx)
		}
	}
}

// doPoll performs a single polling cycle.
func (p *Poller) doPoll(ctx context.Context) {
	start := time.Now()

	p.mu.Lock()
	p.lastPoll = start
	p.stats.TotalPolls++
	p.mu.Unlock()

	results := p.worker.PollAll(ctx)

	var successCount, failCount int64
	var metricCount int64

	for _, result := range results {
		if result.Error != nil {
			failCount++
		} else {
			successCount++
		}

		// Push all metrics to pipeline (including device_stats for failures)
		metricCount += int64(len(result.Metrics))
		for _, m := range result.Metrics {
			p.pipeline.Push(m)
		}
	}

	duration := time.Since(start)

	// Create and push poller stats metric
	pollerStats := metrics.PollerStatsToMetric(
		float64(duration.Milliseconds()),
		len(results),
		int(successCount),
		int(failCount),
		int(metricCount),
	)
	p.pipeline.Push(pollerStats)

	p.mu.Lock()
	p.stats.SuccessfulPolls += successCount
	p.stats.FailedPolls += failCount
	p.stats.TotalMetrics += metricCount + 1 // +1 for poller_stats
	p.stats.LastPollDuration = duration
	p.mu.Unlock()

	p.logger.Info("poll completed",
		"duration", duration,
		"success", successCount,
		"failed", failCount,
		"metrics", metricCount,
	)
}

// Stats returns current polling statistics.
func (p *Poller) Stats() Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// LastPoll returns the time of the last poll.
func (p *Poller) LastPoll() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastPoll
}

// IsRunning returns true if the poller is running.
func (p *Poller) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// ReloadConfig reloads the configuration.
func (p *Poller) ReloadConfig(cfg *config.Config) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cfg = cfg
	p.worker = NewWorker(cfg, p.logger)
	p.logger.Info("configuration reloaded", "devices", len(cfg.Devices))
}
