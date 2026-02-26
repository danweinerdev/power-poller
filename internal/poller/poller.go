package poller

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/danweinerdev/power-poller/internal/config"
	"github.com/danweinerdev/power-poller/internal/metrics"
	"github.com/danweinerdev/power-poller/pkg/kasa/protocol"
)

// Poller manages the main polling loop.
type Poller struct {
	cfg           *config.Config
	pipeline      *metrics.Pipeline
	worker        *Worker
	logger        *slog.Logger
	protocolCache *protocol.ProtocolCache
	options       Options

	mu       sync.RWMutex
	running  bool
	lastPoll time.Time
	stats    Stats
}

// Options configures poller behavior.
type Options struct {
	// IgnoreUnknownDevices allows the poller to continue even if some devices
	// are reachable but have an unrecognized protocol. If false (default),
	// the poller will error on startup if any device is reachable but unsupported.
	IgnoreUnknownDevices bool

	// MaxIterations limits the number of poll cycles. If 0 (default), the poller
	// runs indefinitely until the context is cancelled.
	MaxIterations int
}

// Stats holds polling statistics.
type Stats struct {
	TotalPolls       int64
	SuccessfulPolls  int64
	FailedPolls      int64
	TotalMetrics     int64
	LastPollDuration time.Duration
}

// New creates a new Poller.
func New(cfg *config.Config, pipeline *metrics.Pipeline, logger *slog.Logger, opts ...func(*Options)) *Poller {
	if logger == nil {
		logger = slog.Default()
	}

	// Initialize device logger to capture HTTP-level log messages with device context
	protocol.InitDeviceLogger(logger)

	options := Options{}
	for _, opt := range opts {
		opt(&options)
	}

	cache := protocol.NewProtocolCache()
	return &Poller{
		cfg:           cfg,
		pipeline:      pipeline,
		worker:        NewWorker(cfg, cache, logger),
		logger:        logger,
		protocolCache: cache,
		options:       options,
	}
}

// WithIgnoreUnknownDevices sets whether to ignore unknown devices.
func WithIgnoreUnknownDevices(ignore bool) func(*Options) {
	return func(o *Options) {
		o.IgnoreUnknownDevices = ignore
	}
}

// WithMaxIterations sets the maximum number of poll iterations.
// If n <= 0, the poller runs indefinitely.
func WithMaxIterations(n int) func(*Options) {
	return func(o *Options) {
		o.MaxIterations = n
	}
}

// Run starts the polling loop and blocks until context is cancelled
// or MaxIterations is reached (if configured).
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
	maxIter := p.options.MaxIterations

	logAttrs := []any{"interval", interval, "devices", len(p.cfg.Devices)}
	if maxIter > 0 {
		logAttrs = append(logAttrs, "max_iterations", maxIter)
	}
	p.logger.Info("starting poller", logAttrs...)

	// Detect protocols for all devices at startup
	if err := p.detectProtocols(ctx); err != nil {
		return fmt.Errorf("protocol detection failed: %w", err)
	}

	// Do initial poll immediately
	p.doPoll(ctx)
	iteration := 1

	// Check if we've reached max iterations after initial poll
	if maxIter > 0 && iteration >= maxIter {
		p.logger.Info("max iterations reached", "iterations", iteration)
		return nil
	}

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
			iteration++

			// Check if we've reached max iterations
			if maxIter > 0 && iteration >= maxIter {
				p.logger.Info("max iterations reached", "iterations", iteration)
				return nil
			}
		}
	}
}

// detectProtocols probes all configured devices to detect their protocols.
// Returns an error if:
// - Any KLAP device requires credentials that are not configured
// - Any device is reachable but has an unsupported protocol (unless IgnoreUnknownDevices is set)
func (p *Poller) detectProtocols(ctx context.Context) error {
	// Build host list and reverse mapping from address to device name
	hosts := make([]string, 0, len(p.cfg.Devices))
	addressToName := make(map[string]string)
	for name, devCfg := range p.cfg.Devices {
		hosts = append(hosts, devCfg.Address)
		addressToName[devCfg.Address] = name
	}

	if len(hosts) == 0 {
		return nil
	}

	p.logger.Info("detecting protocols for devices", "count", len(hosts))
	start := time.Now()

	results := p.protocolCache.DetectAll(ctx, hosts)

	legacyCount := 0
	klapCount := 0
	klapAuthCount := 0
	securePassthroughCount := 0
	unknownCount := 0
	unreachableCount := 0
	var missingCredentials []string
	var unknownDevices []string

	for host, proto := range results {
		deviceName := addressToName[host]
		switch proto {
		case protocol.ProtocolLegacy:
			legacyCount++
			p.logger.Debug("detected protocol", "host", host, "device", deviceName, "protocol", "legacy")
		case protocol.ProtocolKLAP:
			klapCount++
			p.logger.Debug("detected protocol", "host", host, "device", deviceName, "protocol", "klap", "auth", "default")
		case protocol.ProtocolKLAPAuthRequired:
			klapAuthCount++
			// Check if credentials are configured for this device
			if _, _, hasCredentials := p.cfg.GetDeviceCredentials(deviceName); !hasCredentials {
				p.logger.Error("KLAP device requires credentials but none configured",
					"host", host,
					"device", deviceName,
				)
				missingCredentials = append(missingCredentials, deviceName)
			} else {
				p.logger.Debug("detected protocol", "host", host, "device", deviceName, "protocol", "klap", "auth", "custom")
			}
			// Store as KLAP in the cache (the auth requirement is handled by config)
			p.protocolCache.Set(host, protocol.ProtocolKLAP)
		case protocol.ProtocolSecurePassthrough:
			securePassthroughCount++
			p.logger.Debug("detected protocol", "host", host, "device", deviceName, "protocol", "securepassthrough")
		case protocol.ProtocolUnreachable:
			unreachableCount++
			p.logger.Warn("device unreachable", "host", host, "device", deviceName)
		case protocol.ProtocolUnknown:
			unknownCount++
			p.logger.Error("device reachable but protocol not recognized",
				"host", host,
				"device", deviceName,
			)
			unknownDevices = append(unknownDevices, deviceName)
		default:
			unknownCount++
			p.logger.Warn("unexpected protocol type", "host", host, "device", deviceName, "protocol", proto)
		}
	}

	p.logger.Info("protocol detection complete",
		"duration", time.Since(start),
		"legacy", legacyCount,
		"klap", klapCount,
		"klap_auth_required", klapAuthCount,
		"securepassthrough", securePassthroughCount,
		"unknown", unknownCount,
		"unreachable", unreachableCount,
	)

	// Collect errors
	var errors []string

	if len(missingCredentials) > 0 {
		errors = append(errors, fmt.Sprintf("missing credentials for KLAP devices: %s (configure username/password in device config or [klap] section)",
			strings.Join(missingCredentials, ", ")))
	}

	if len(unknownDevices) > 0 && !p.options.IgnoreUnknownDevices {
		errors = append(errors, fmt.Sprintf("unsupported devices (reachable but protocol not recognized): %s (use --ignore-unknown to skip these devices)",
			strings.Join(unknownDevices, ", ")))
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}

	return nil
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
	// Clear protocol cache on reload to re-detect new devices
	p.protocolCache.Clear()
	p.worker = NewWorker(cfg, p.protocolCache, p.logger)
	p.logger.Info("configuration reloaded", "devices", len(cfg.Devices))
}
