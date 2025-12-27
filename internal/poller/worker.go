package poller

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/danweinerdev/go-power-poller/internal/config"
	"github.com/danweinerdev/go-power-poller/internal/metrics"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/device"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/protocol"
)

// DeviceResult represents the result of polling a single device.
type DeviceResult struct {
	DeviceName string
	Metrics    []*metrics.Metric
	Error      error
	Duration   time.Duration
}

// Worker polls devices and generates metrics.
type Worker struct {
	cfg     *config.Config
	timeout time.Duration
	logger  *slog.Logger
}

// NewWorker creates a new device polling worker.
func NewWorker(cfg *config.Config, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{
		cfg:     cfg,
		timeout: cfg.Global.DeviceTimeout.Duration,
		logger:  logger,
	}
}

// PollAll polls all configured devices concurrently.
func (w *Worker) PollAll(ctx context.Context) []DeviceResult {
	var wg sync.WaitGroup
	results := make([]DeviceResult, 0, len(w.cfg.Devices))
	resultCh := make(chan DeviceResult, len(w.cfg.Devices))

	for name, devCfg := range w.cfg.Devices {
		wg.Add(1)
		go func(deviceName string, deviceCfg config.DeviceConfig) {
			defer wg.Done()
			result := w.pollDevice(ctx, deviceName, deviceCfg)
			resultCh <- result
		}(name, devCfg)
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results
	for result := range resultCh {
		results = append(results, result)
	}

	return results
}

// pollDevice polls a single device and returns metrics.
func (w *Worker) pollDevice(ctx context.Context, deviceName string, devCfg config.DeviceConfig) DeviceResult {
	start := time.Now()
	result := DeviceResult{
		DeviceName: deviceName,
		Metrics:    make([]*metrics.Metric, 0),
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	// Connect to device
	opts := []protocol.TransportOption{
		protocol.WithTimeout(w.timeout),
	}

	dev, err := device.Load(ctx, devCfg.Address, opts...)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		w.logger.Error("failed to connect to device",
			"device", deviceName,
			"address", devCfg.Address,
			"error", err,
		)
		return result
	}
	defer dev.Close()

	// Build base tags
	tags := w.cfg.GetDeviceTags(deviceName, map[string]string{
		"address": devCfg.Address,
	})

	// Poll parent device if configured or if no children
	if devCfg.PollParent || !devCfg.HasChildren {
		parentMetrics := w.pollEmeter(ctx, dev, deviceName, devCfg.Measurements, tags)
		result.Metrics = append(result.Metrics, parentMetrics...)
	}

	// Poll children if device has them
	if devCfg.HasChildren {
		parentDev, ok := dev.(device.ParentDevice)
		if !ok {
			w.logger.Warn("device marked as has_children but doesn't implement ParentDevice",
				"device", deviceName,
			)
		} else {
			childMetrics := w.pollChildren(ctx, parentDev, deviceName, devCfg, tags)
			result.Metrics = append(result.Metrics, childMetrics...)
		}
	}

	result.Duration = time.Since(start)
	w.logger.Debug("polled device",
		"device", deviceName,
		"metrics", len(result.Metrics),
		"duration", result.Duration,
	)

	return result
}

// pollEmeter polls the emeter on a device.
func (w *Worker) pollEmeter(ctx context.Context, dev device.Device, deviceName string, measurements []string, tags map[string]string) []*metrics.Metric {
	result := make([]*metrics.Metric, 0)

	emeterDev, ok := dev.(device.EmeterDevice)
	if !ok {
		w.logger.Debug("device does not support emeter", "device", deviceName)
		return result
	}

	if !emeterDev.HasEmeter() {
		w.logger.Debug("device has no emeter", "device", deviceName)
		return result
	}

	data, err := emeterDev.GetEmeterRealtime(ctx)
	if err != nil {
		w.logger.Error("failed to get emeter data",
			"device", deviceName,
			"error", err,
		)
		return result
	}

	// Create metrics for each configured measurement
	for _, measurement := range measurements {
		m := metrics.EmeterToMetric(measurement, tags, data.Voltage, data.Current, data.Power, data.Total)
		result = append(result, m)
	}

	return result
}

// pollChildren polls all configured child devices.
func (w *Worker) pollChildren(ctx context.Context, parent device.ParentDevice, deviceName string, devCfg config.DeviceConfig, parentTags map[string]string) []*metrics.Metric {
	result := make([]*metrics.Metric, 0)

	children := parent.Children()
	for childName, childCfg := range devCfg.Children {
		if childCfg.Index >= len(children) {
			w.logger.Warn("child index out of range",
				"device", deviceName,
				"child", childName,
				"index", childCfg.Index,
				"count", len(children),
			)
			continue
		}

		child := children[childCfg.Index]
		childTags := w.cfg.GetChildTags(deviceName, childName, parentTags)

		childMetrics := w.pollEmeter(ctx, child, childName, childCfg.Measurements, childTags)
		result = append(result, childMetrics...)
	}

	return result
}
