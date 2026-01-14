package poller

import (
	"context"
	"testing"
	"time"

	"github.com/danweinerdev/go-power-poller/internal/config"
	"github.com/danweinerdev/go-power-poller/pkg/mockdevice"
)

func TestWorker_NewWorker(t *testing.T) {
	cfg := newTestConfig()

	t.Run("with nil logger", func(t *testing.T) {
		w := NewWorker(cfg, nil)
		if w == nil {
			t.Fatal("NewWorker() returned nil")
		}
		if w.cfg != cfg {
			t.Error("config not set correctly")
		}
		if w.timeout != cfg.Global.DeviceTimeout.Duration {
			t.Errorf("timeout = %v, want %v", w.timeout, cfg.Global.DeviceTimeout.Duration)
		}
	})
}

func TestWorker_PollAll_NoDevices(t *testing.T) {
	cfg := newTestConfig()
	w := NewWorker(cfg, nil)

	ctx := context.Background()
	results := w.PollAll(ctx)

	if len(results) != 0 {
		t.Errorf("PollAll() with no devices returned %d results, want 0", len(results))
	}
}

func TestWorker_PollAll_Success(t *testing.T) {
	mock := mockdevice.New(
		mockdevice.WithEmeter(true),
		mockdevice.WithEmeterValues(120.5, 0.84, 101.22, 1234.56),
	)
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	cfg := newTestConfig()
	cfg.Devices["test_device"] = config.DeviceConfig{
		Address:      mock.Address(),
		Measurements: []string{"power_metrics"},
	}

	w := NewWorker(cfg, nil)
	ctx := context.Background()

	results := w.PollAll(ctx)

	if len(results) != 1 {
		t.Fatalf("PollAll() returned %d results, want 1", len(results))
	}

	result := results[0]
	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}
	if result.DeviceName != "test_device" {
		t.Errorf("DeviceName = %q, want %q", result.DeviceName, "test_device")
	}
	if len(result.Metrics) == 0 {
		t.Error("expected at least 1 metric")
	}
	if result.Duration == 0 {
		t.Error("Duration should not be zero")
	}
}

func TestWorker_PollAll_MultipleDevices(t *testing.T) {
	mock1 := mockdevice.New(
		mockdevice.WithEmeter(true),
		mockdevice.WithAlias("Device 1"),
	)
	mock2 := mockdevice.New(
		mockdevice.WithEmeter(true),
		mockdevice.WithAlias("Device 2"),
	)

	if err := mock1.Start(); err != nil {
		t.Fatalf("failed to start mock device 1: %v", err)
	}
	defer mock1.Stop()

	if err := mock2.Start(); err != nil {
		t.Fatalf("failed to start mock device 2: %v", err)
	}
	defer mock2.Stop()

	cfg := newTestConfig()
	cfg.Devices["device1"] = config.DeviceConfig{
		Address:      mock1.Address(),
		Measurements: []string{"power_metrics"},
	}
	cfg.Devices["device2"] = config.DeviceConfig{
		Address:      mock2.Address(),
		Measurements: []string{"power_metrics"},
	}

	w := NewWorker(cfg, nil)
	ctx := context.Background()

	results := w.PollAll(ctx)

	if len(results) != 2 {
		t.Fatalf("PollAll() returned %d results, want 2", len(results))
	}

	// Both should succeed
	for _, result := range results {
		if result.Error != nil {
			t.Errorf("device %s error: %v", result.DeviceName, result.Error)
		}
	}
}

func TestWorker_PollAll_PartialFailure(t *testing.T) {
	mock := mockdevice.New(
		mockdevice.WithEmeter(true),
	)
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	cfg := newTestConfig()
	cfg.Global.DeviceTimeout = config.Duration{Duration: 500 * time.Millisecond}
	cfg.Devices["good_device"] = config.DeviceConfig{
		Address:      mock.Address(),
		Measurements: []string{"power_metrics"},
	}
	cfg.Devices["bad_device"] = config.DeviceConfig{
		Address:      "10.255.255.1:9999", // Non-routable address
		Measurements: []string{"power_metrics"},
	}

	w := NewWorker(cfg, nil)
	ctx := context.Background()

	results := w.PollAll(ctx)

	if len(results) != 2 {
		t.Fatalf("PollAll() returned %d results, want 2", len(results))
	}

	var successCount, failCount int
	for _, result := range results {
		if result.Error != nil {
			failCount++
		} else {
			successCount++
		}
	}

	if successCount != 1 {
		t.Errorf("expected 1 success, got %d", successCount)
	}
	if failCount != 1 {
		t.Errorf("expected 1 failure, got %d", failCount)
	}
}

func TestWorker_PollAll_ContextTimeout(t *testing.T) {
	mock := mockdevice.New(
		mockdevice.WithEmeter(true),
		mockdevice.WithErrorBehavior(mockdevice.ErrorBehavior{
			ResponseDelay: 5 * time.Second,
		}),
	)
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	cfg := newTestConfig()
	cfg.Global.DeviceTimeout = config.Duration{Duration: 100 * time.Millisecond}
	cfg.Devices["slow_device"] = config.DeviceConfig{
		Address:      mock.Address(),
		Measurements: []string{"power_metrics"},
	}

	w := NewWorker(cfg, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	results := w.PollAll(ctx)

	if len(results) != 1 {
		t.Fatalf("PollAll() returned %d results, want 1", len(results))
	}

	if results[0].Error == nil {
		t.Error("expected timeout error")
	}
}

func TestWorker_PollDevice_NoEmeter(t *testing.T) {
	mock := mockdevice.New(
		mockdevice.WithEmeter(false),
	)
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	cfg := newTestConfig()
	cfg.Devices["no_emeter_device"] = config.DeviceConfig{
		Address:      mock.Address(),
		Measurements: []string{"power_metrics"},
	}

	w := NewWorker(cfg, nil)
	ctx := context.Background()

	results := w.PollAll(ctx)

	if len(results) != 1 {
		t.Fatalf("PollAll() returned %d results, want 1", len(results))
	}

	result := results[0]
	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}
	// No emeter means only device_stats metric
	if len(result.Metrics) != 1 {
		t.Errorf("expected 1 metric (device_stats only), got %d", len(result.Metrics))
	}
	if len(result.Metrics) > 0 && result.Metrics[0].Measurement != "device_stats" {
		t.Errorf("expected device_stats metric, got %s", result.Metrics[0].Measurement)
	}
}

func TestWorker_PollDevice_EmeterData(t *testing.T) {
	mock := mockdevice.New(
		mockdevice.WithEmeter(true),
		mockdevice.WithEmeterValues(120.5, 0.84, 101.22, 1234.56),
	)
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	cfg := newTestConfig()
	cfg.Devices["test_device"] = config.DeviceConfig{
		Address:      mock.Address(),
		Measurements: []string{"power_metrics"},
		Tags:         map[string]string{"location": "office"},
	}

	w := NewWorker(cfg, nil)
	ctx := context.Background()

	results := w.PollAll(ctx)

	if len(results) != 1 {
		t.Fatalf("PollAll() returned %d results, want 1", len(results))
	}

	result := results[0]
	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}
	if len(result.Metrics) == 0 {
		t.Fatal("expected at least 1 metric")
	}

	m := result.Metrics[0]
	if m.Measurement != "power_metrics" {
		t.Errorf("Measurement = %q, want %q", m.Measurement, "power_metrics")
	}

	// Check fields exist
	if _, ok := m.Fields["voltage"]; !ok {
		t.Error("voltage field missing")
	}
	if _, ok := m.Fields["current"]; !ok {
		t.Error("current field missing")
	}
	if _, ok := m.Fields["power"]; !ok {
		t.Error("power field missing")
	}
	if _, ok := m.Fields["total"]; !ok {
		t.Error("total field missing")
	}

	// Check tags
	if m.Tags["location"] != "office" {
		t.Errorf("location tag = %q, want %q", m.Tags["location"], "office")
	}
}

func TestWorker_PollChildren(t *testing.T) {
	mock := mockdevice.New(
		mockdevice.WithDeviceType(mockdevice.DeviceTypePowerStrip),
		mockdevice.WithChildren(3),
		mockdevice.WithEmeter(true),
	)
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	cfg := newTestConfig()
	cfg.Devices["power_strip"] = config.DeviceConfig{
		Address:     mock.Address(),
		HasChildren: true,
		PollParent:  false,
		Children: map[string]config.ChildConfig{
			"outlet_0": {
				Index:        0,
				Measurements: []string{"power_metrics"},
				Tags:         map[string]string{"outlet": "0"},
			},
			"outlet_1": {
				Index:        1,
				Measurements: []string{"power_metrics"},
				Tags:         map[string]string{"outlet": "1"},
			},
		},
	}

	w := NewWorker(cfg, nil)
	ctx := context.Background()

	results := w.PollAll(ctx)

	if len(results) != 1 {
		t.Fatalf("PollAll() returned %d results, want 1", len(results))
	}

	result := results[0]
	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}

	// Should have metrics from 2 children
	if len(result.Metrics) < 2 {
		t.Errorf("expected at least 2 metrics (one per child), got %d", len(result.Metrics))
	}
}

func TestWorker_PollChildren_IndexOutOfRange(t *testing.T) {
	mock := mockdevice.New(
		mockdevice.WithDeviceType(mockdevice.DeviceTypePowerStrip),
		mockdevice.WithChildren(2), // Only 2 children
		mockdevice.WithEmeter(true),
	)
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	cfg := newTestConfig()
	cfg.Devices["power_strip"] = config.DeviceConfig{
		Address:     mock.Address(),
		HasChildren: true,
		PollParent:  false,
		Children: map[string]config.ChildConfig{
			"outlet_0": {
				Index:        0,
				Measurements: []string{"power_metrics"},
			},
			"outlet_invalid": {
				Index:        10, // Out of range
				Measurements: []string{"power_metrics"},
			},
		},
	}

	w := NewWorker(cfg, nil)
	ctx := context.Background()

	results := w.PollAll(ctx)

	if len(results) != 1 {
		t.Fatalf("PollAll() returned %d results, want 1", len(results))
	}

	result := results[0]
	// Should not error - just skip the invalid child
	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}

	// Should only have metrics from the valid child
	if len(result.Metrics) < 1 {
		t.Errorf("expected at least 1 metric from valid child, got %d", len(result.Metrics))
	}
}

func TestWorker_PollDevice_ConnectionError(t *testing.T) {
	cfg := newTestConfig()
	cfg.Global.DeviceTimeout = config.Duration{Duration: 500 * time.Millisecond}
	cfg.Devices["unreachable_device"] = config.DeviceConfig{
		Address:      "10.255.255.1:9999", // Non-routable
		Measurements: []string{"power_metrics"},
	}

	w := NewWorker(cfg, nil)
	ctx := context.Background()

	results := w.PollAll(ctx)

	if len(results) != 1 {
		t.Fatalf("PollAll() returned %d results, want 1", len(results))
	}

	result := results[0]
	if result.Error == nil {
		t.Error("expected connection error")
	}
	if result.Duration == 0 {
		t.Error("Duration should not be zero even on error")
	}
}

func TestWorker_PollDevice_MultipleMeasurements(t *testing.T) {
	mock := mockdevice.New(
		mockdevice.WithEmeter(true),
		mockdevice.WithEmeterValues(120.5, 0.84, 101.22, 1234.56),
	)
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	cfg := newTestConfig()
	cfg.Devices["test_device"] = config.DeviceConfig{
		Address:      mock.Address(),
		Measurements: []string{"power_metrics", "energy_stats"},
	}

	w := NewWorker(cfg, nil)
	ctx := context.Background()

	results := w.PollAll(ctx)

	if len(results) != 1 {
		t.Fatalf("PollAll() returned %d results, want 1", len(results))
	}

	result := results[0]
	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}

	// Should have metrics for each measurement + device_stats
	if len(result.Metrics) != 3 {
		t.Errorf("expected 3 metrics (2 measurements + device_stats), got %d", len(result.Metrics))
	}

	// Check measurements names
	measurements := make(map[string]bool)
	for _, m := range result.Metrics {
		measurements[m.Measurement] = true
	}

	if !measurements["power_metrics"] {
		t.Error("missing power_metrics measurement")
	}
	if !measurements["energy_stats"] {
		t.Error("missing energy_stats measurement")
	}
	if !measurements["device_stats"] {
		t.Error("missing device_stats measurement")
	}
}
