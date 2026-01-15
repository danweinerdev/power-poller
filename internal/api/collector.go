package api

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/danweinerdev/go-power-poller/internal/api/manager"
	"github.com/danweinerdev/go-power-poller/internal/config"
	"github.com/danweinerdev/go-power-poller/internal/metrics"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/device"
)

// MetricsCollector collects device metrics and pushes them to a pipeline.
// It uses the DeviceManager's device connections to avoid duplicate polling.
type MetricsCollector struct {
	cfg      *config.Config
	mgr      *manager.Manager
	pipeline *metrics.Pipeline
	logger   *slog.Logger

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector(cfg *config.Config, mgr *manager.Manager, pipeline *metrics.Pipeline, logger *slog.Logger) *MetricsCollector {
	if logger == nil {
		logger = slog.Default()
	}
	return &MetricsCollector{
		cfg:      cfg,
		mgr:      mgr,
		pipeline: pipeline,
		logger:   logger,
	}
}

// Start begins collecting metrics on the configured poll interval.
func (c *MetricsCollector) Start(ctx context.Context) error {
	c.mu.Lock()
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.mu.Unlock()

	c.wg.Add(1)
	go c.collectLoop()

	c.logger.Info("metrics collector started")
	return nil
}

// Stop gracefully shuts down the collector.
func (c *MetricsCollector) Stop() error {
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Unlock()

	c.wg.Wait()
	c.logger.Info("metrics collector stopped")
	return nil
}

// ReloadConfig updates the collector configuration.
func (c *MetricsCollector) ReloadConfig(cfg *config.Config) {
	c.mu.Lock()
	c.cfg = cfg
	c.mu.Unlock()
	c.logger.Info("metrics collector config reloaded")
}

// SetManager sets the device manager (used when manager is started after collector is created).
func (c *MetricsCollector) SetManager(mgr *manager.Manager) {
	c.mu.Lock()
	c.mgr = mgr
	c.mu.Unlock()
}

// collectLoop runs the metric collection on the configured interval.
func (c *MetricsCollector) collectLoop() {
	defer c.wg.Done()

	c.mu.RLock()
	interval := c.cfg.Global.PollInterval.Duration
	c.mu.RUnlock()

	if interval == 0 {
		interval = 10 * time.Second
	}

	// Do an initial collection immediately
	c.collectMetrics()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.collectMetrics()
		}
	}
}

// collectMetrics collects metrics from all devices.
func (c *MetricsCollector) collectMetrics() {
	c.mu.RLock()
	cfg := c.cfg
	timeout := cfg.Global.DeviceTimeout.Duration
	c.mu.RUnlock()

	if timeout == 0 {
		timeout = 5 * time.Second
	}

	start := time.Now()
	states := c.mgr.GetAllStates()

	var (
		devicesPolled    int
		devicesSuccess   int
		devicesFailed    int
		metricsCollected int
	)

	for _, state := range states {
		devicesPolled++

		// Get device config by name
		devCfg, ok := cfg.Devices[state.Name]
		if !ok {
			c.logger.Debug("device not in config", "device", state.Name)
			continue
		}

		if !state.Online {
			devicesFailed++
			// Generate failure metric
			statsTags := map[string]string{
				"device":      state.Name,
				"address":     state.Address,
				"model":       state.Model,
				"hw_version":  "unknown",
				"sw_version":  "unknown",
				"device_type": state.Type,
			}
			statsMetric := metrics.DeviceStatsToMetric(statsTags, 0, false, 0)
			c.pipeline.Push(statsMetric)
			metricsCollected++
			continue
		}

		devicesSuccess++
		deviceStart := time.Now()

		// Build base tags
		tags := cfg.GetDeviceTags(state.Name, map[string]string{
			"address": state.Address,
		})

		// Poll parent device if configured or if no children
		if devCfg.PollParent || !devCfg.HasChildren {
			parentMetrics := c.collectEmeter(state, devCfg.Measurements, tags, timeout)
			for _, m := range parentMetrics {
				c.pipeline.Push(m)
				metricsCollected++
			}
		}

		// Poll children if device has them
		if devCfg.HasChildren && state.HasChildren {
			childMetrics := c.collectChildren(state, devCfg, tags, timeout)
			for _, m := range childMetrics {
				c.pipeline.Push(m)
				metricsCollected++
			}
		}

		deviceDuration := time.Since(deviceStart)

		// Create device stats metric
		statsTags := map[string]string{
			"device":      state.Name,
			"address":     state.Address,
			"model":       state.Model,
			"hw_version":  "unknown",
			"sw_version":  "unknown",
			"device_type": state.Type,
			"device_id":   state.DeviceID,
		}

		// Get additional info from the device if possible
		if dev, ok := c.mgr.GetDevice(state.ID); ok {
			if sysinfo := dev.SysInfo(); sysinfo != nil {
				statsTags["hw_version"] = sysinfo.HWVersion
				statsTags["sw_version"] = sysinfo.SWVersion
			}
		}

		statsMetric := metrics.DeviceStatsToMetric(statsTags, float64(deviceDuration.Milliseconds()), true, state.RSSI)
		c.pipeline.Push(statsMetric)
		metricsCollected++
	}

	pollDuration := time.Since(start)

	// Generate poller stats
	pollerStats := metrics.PollerStatsToMetric(
		float64(pollDuration.Milliseconds()),
		devicesPolled,
		devicesSuccess,
		devicesFailed,
		metricsCollected,
	)
	c.pipeline.Push(pollerStats)

	c.logger.Debug("metrics collection complete",
		"devices_polled", devicesPolled,
		"devices_success", devicesSuccess,
		"devices_failed", devicesFailed,
		"metrics_collected", metricsCollected,
		"duration", pollDuration,
	)
}

// collectEmeter collects emeter data for a device.
func (c *MetricsCollector) collectEmeter(state *manager.DeviceState, measurements []string, tags map[string]string, timeout time.Duration) []*metrics.Metric {
	result := make([]*metrics.Metric, 0)

	if !state.HasEmeter {
		return result
	}

	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()

	data, err := c.mgr.GetEmeter(ctx, state.ID)
	if err != nil {
		c.logger.Debug("failed to get emeter data",
			"device", state.Name,
			"error", err,
		)
		return result
	}

	for _, measurement := range measurements {
		m := metrics.EmeterToMetric(measurement, tags, data.Voltage, data.Current, data.Power, data.Total)
		result = append(result, m)
	}

	return result
}

// collectChildren collects emeter data for child devices (power strips).
func (c *MetricsCollector) collectChildren(state *manager.DeviceState, devCfg config.DeviceConfig, parentTags map[string]string, timeout time.Duration) []*metrics.Metric {
	result := make([]*metrics.Metric, 0)

	dev, ok := c.mgr.GetDevice(state.ID)
	if !ok {
		return result
	}

	parentDev, ok := dev.(device.ParentDevice)
	if !ok {
		return result
	}

	c.mu.RLock()
	cfg := c.cfg
	c.mu.RUnlock()

	children := parentDev.Children()
	for childName, childCfg := range devCfg.Children {
		if childCfg.Index >= len(children) {
			c.logger.Warn("child index out of range",
				"device", state.Name,
				"child", childName,
				"index", childCfg.Index,
				"count", len(children),
			)
			continue
		}

		child := children[childCfg.Index]
		childTags := cfg.GetChildTags(state.Name, childName, parentTags)

		// Get emeter data from child
		emeterDev, ok := child.(device.EmeterDevice)
		if !ok || !child.HasEmeter() {
			continue
		}

		ctx, cancel := context.WithTimeout(c.ctx, timeout)
		data, err := emeterDev.GetEmeterRealtime(ctx)
		cancel()

		if err != nil {
			c.logger.Debug("failed to get child emeter data",
				"device", state.Name,
				"child", childName,
				"error", err,
			)
			continue
		}

		for _, measurement := range childCfg.Measurements {
			m := metrics.EmeterToMetric(measurement, childTags, data.Voltage, data.Current, data.Power, data.Total)
			result = append(result, m)
		}

		// Create device_stats for each child outlet
		childStatsTags := map[string]string{
			"device":      childName,
			"address":     parentTags["address"],
			"model":       child.Model(),
			"hw_version":  "",
			"sw_version":  "",
			"device_type": "ChildOutlet",
			"device_id":   child.DeviceID(),
			"parent":      state.Name,
		}
		if sysinfo := child.SysInfo(); sysinfo != nil {
			childStatsTags["hw_version"] = sysinfo.HWVersion
			childStatsTags["sw_version"] = sysinfo.SWVersion
		}
		childStatsMetric := metrics.DeviceStatsToMetric(childStatsTags, 0, true, 0)
		result = append(result, childStatsMetric)
	}

	return result
}
