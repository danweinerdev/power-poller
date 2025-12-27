package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestCollector_New(t *testing.T) {
	c := NewCollector()
	if c == nil {
		t.Fatal("NewCollector() returned nil")
	}
	if c.DeviceCount() != 0 {
		t.Errorf("DeviceCount() = %d, want 0", c.DeviceCount())
	}
}

func TestCollector_Update(t *testing.T) {
	c := NewCollector()

	m := NewMetric("power_metrics").
		WithTag("address", "192.168.1.100").
		WithField("voltage", 120.5).
		WithField("current", 0.84).
		WithField("power", 101.22).
		WithField("total", 1234.56)

	c.Update("test_device", m)

	if c.DeviceCount() != 1 {
		t.Errorf("DeviceCount() = %d, want 1", c.DeviceCount())
	}
}

func TestCollector_Update_AllFieldTypes(t *testing.T) {
	tests := []struct {
		name     string
		fields   map[string]interface{}
		hasEmeter bool
		hasBulb  bool
	}{
		{
			name: "emeter fields",
			fields: map[string]interface{}{
				"voltage": 120.5,
				"current": 0.84,
				"power":   101.22,
				"total":   1234.56,
			},
			hasEmeter: true,
			hasBulb:   false,
		},
		{
			name: "bulb fields",
			fields: map[string]interface{}{
				"brightness": 75,
				"hue":        180,
				"saturation": 50,
				"color_temp": 2700,
			},
			hasEmeter: false,
			hasBulb:   true,
		},
		{
			name: "state field on_off",
			fields: map[string]interface{}{
				"on_off": 1,
			},
			hasEmeter: false,
			hasBulb:   false,
		},
		{
			name: "state field state",
			fields: map[string]interface{}{
				"state": 0,
			},
			hasEmeter: false,
			hasBulb:   false,
		},
		{
			name: "mixed fields",
			fields: map[string]interface{}{
				"voltage":    120.5,
				"brightness": 75,
			},
			hasEmeter: true,
			hasBulb:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewCollector()
			m := NewMetric("test").WithFields(tc.fields)
			c.Update("device", m)

			if c.DeviceCount() != 1 {
				t.Errorf("DeviceCount() = %d, want 1", c.DeviceCount())
			}
		})
	}
}

func TestCollector_Update_MultipleTimes(t *testing.T) {
	c := NewCollector()

	// First update
	m1 := NewMetric("power_metrics").
		WithTag("address", "192.168.1.100").
		WithField("voltage", 120.0).
		WithField("power", 100.0)

	c.Update("test_device", m1)

	// Second update - should update values
	m2 := NewMetric("power_metrics").
		WithTag("address", "192.168.1.100").
		WithField("voltage", 121.0).
		WithField("power", 150.0)

	c.Update("test_device", m2)

	// Should still be just 1 device
	if c.DeviceCount() != 1 {
		t.Errorf("DeviceCount() = %d, want 1", c.DeviceCount())
	}
}

func TestCollector_Update_MultipleDevices(t *testing.T) {
	c := NewCollector()

	m1 := NewMetric("power_metrics").
		WithTag("address", "192.168.1.100").
		WithField("voltage", 120.5)

	m2 := NewMetric("power_metrics").
		WithTag("address", "192.168.1.101").
		WithField("voltage", 119.5)

	c.Update("device1", m1)
	c.Update("device2", m2)

	if c.DeviceCount() != 2 {
		t.Errorf("DeviceCount() = %d, want 2", c.DeviceCount())
	}
}

func TestCollector_Describe(t *testing.T) {
	c := NewCollector()
	ch := make(chan *prometheus.Desc, 20)

	go func() {
		c.Describe(ch)
		close(ch)
	}()

	descs := make([]*prometheus.Desc, 0)
	for desc := range ch {
		descs = append(descs, desc)
	}

	// Should have 9 descriptors
	expectedCount := 9
	if len(descs) != expectedCount {
		t.Errorf("Describe() sent %d descriptors, want %d", len(descs), expectedCount)
	}
}

func TestCollector_Collect_Empty(t *testing.T) {
	c := NewCollector()
	ch := make(chan prometheus.Metric, 20)

	go func() {
		c.Collect(ch)
		close(ch)
	}()

	metrics := make([]prometheus.Metric, 0)
	for m := range ch {
		metrics = append(metrics, m)
	}

	if len(metrics) != 0 {
		t.Errorf("Collect() with no devices sent %d metrics, want 0", len(metrics))
	}
}

func TestCollector_Collect_WithEmeterDevice(t *testing.T) {
	c := NewCollector()

	m := NewMetric("power_metrics").
		WithTag("address", "192.168.1.100").
		WithField("voltage", 120.5).
		WithField("current", 0.84).
		WithField("power", 101.22).
		WithField("total", 1234.56)

	c.Update("test_device", m)

	ch := make(chan prometheus.Metric, 20)

	go func() {
		c.Collect(ch)
		close(ch)
	}()

	metrics := make([]prometheus.Metric, 0)
	for pm := range ch {
		metrics = append(metrics, pm)
	}

	// Should have: state + 4 emeter metrics = 5
	if len(metrics) != 5 {
		t.Errorf("Collect() sent %d metrics, want 5", len(metrics))
	}
}

func TestCollector_Collect_WithBulbDevice(t *testing.T) {
	c := NewCollector()

	m := NewMetric("bulb_metrics").
		WithTag("address", "192.168.1.100").
		WithField("brightness", 75).
		WithField("hue", 180).
		WithField("saturation", 50).
		WithField("color_temp", 2700)

	c.Update("bulb_device", m)

	ch := make(chan prometheus.Metric, 20)

	go func() {
		c.Collect(ch)
		close(ch)
	}()

	metrics := make([]prometheus.Metric, 0)
	for pm := range ch {
		metrics = append(metrics, pm)
	}

	// Should have: state + 4 bulb metrics = 5
	if len(metrics) != 5 {
		t.Errorf("Collect() sent %d metrics, want 5", len(metrics))
	}
}

func TestCollector_Collect_WithMixedDevice(t *testing.T) {
	c := NewCollector()

	m := NewMetric("mixed_metrics").
		WithTag("address", "192.168.1.100").
		WithField("voltage", 120.5).
		WithField("current", 0.84).
		WithField("power", 101.22).
		WithField("total", 1234.56).
		WithField("brightness", 75).
		WithField("hue", 180).
		WithField("saturation", 50).
		WithField("color_temp", 2700)

	c.Update("mixed_device", m)

	ch := make(chan prometheus.Metric, 20)

	go func() {
		c.Collect(ch)
		close(ch)
	}()

	metrics := make([]prometheus.Metric, 0)
	for pm := range ch {
		metrics = append(metrics, pm)
	}

	// Should have: state + 4 emeter + 4 bulb = 9
	if len(metrics) != 9 {
		t.Errorf("Collect() sent %d metrics, want 9", len(metrics))
	}
}

func TestCollector_Collect_MissingAddress(t *testing.T) {
	c := NewCollector()

	m := NewMetric("power_metrics").
		WithField("voltage", 120.5)

	c.Update("test_device", m)

	ch := make(chan prometheus.Metric, 20)

	go func() {
		c.Collect(ch)
		close(ch)
	}()

	// Should not panic - uses "unknown" for missing address
	metrics := make([]prometheus.Metric, 0)
	for pm := range ch {
		metrics = append(metrics, pm)
	}

	if len(metrics) == 0 {
		t.Error("expected at least state metric")
	}
}

func TestCollector_Clear(t *testing.T) {
	c := NewCollector()

	m := NewMetric("test").
		WithTag("address", "192.168.1.100").
		WithField("voltage", 120.5)

	c.Update("device1", m)
	c.Update("device2", m)

	if c.DeviceCount() != 2 {
		t.Fatalf("DeviceCount() before clear = %d, want 2", c.DeviceCount())
	}

	c.Clear()

	if c.DeviceCount() != 0 {
		t.Errorf("DeviceCount() after clear = %d, want 0", c.DeviceCount())
	}
}

func TestCollector_DeviceCount(t *testing.T) {
	c := NewCollector()

	if c.DeviceCount() != 0 {
		t.Errorf("initial DeviceCount() = %d, want 0", c.DeviceCount())
	}

	m := NewMetric("test").WithField("value", 1)

	c.Update("device1", m)
	if c.DeviceCount() != 1 {
		t.Errorf("DeviceCount() = %d, want 1", c.DeviceCount())
	}

	c.Update("device2", m)
	if c.DeviceCount() != 2 {
		t.Errorf("DeviceCount() = %d, want 2", c.DeviceCount())
	}

	// Update existing device shouldn't increase count
	c.Update("device1", m)
	if c.DeviceCount() != 2 {
		t.Errorf("DeviceCount() after updating existing = %d, want 2", c.DeviceCount())
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  float64
	}{
		{"float64", float64(123.456), 123.456},
		{"float32", float32(123.456), 123.456},
		{"int", int(42), 42},
		{"int8", int8(42), 42},
		{"int16", int16(42), 42},
		{"int32", int32(42), 42},
		{"int64", int64(42), 42},
		{"uint", uint(42), 42},
		{"uint8", uint8(42), 42},
		{"uint16", uint16(42), 42},
		{"uint32", uint32(42), 42},
		{"uint64", uint64(42), 42},
		{"bool true", true, 1},
		{"bool false", false, 0},
		{"string", "hello", 0},
		{"nil", nil, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := toFloat64(tc.input)
			// Handle float32 precision issues
			if tc.name == "float32" {
				if got < 123.455 || got > 123.457 {
					t.Errorf("toFloat64(%v) = %v, want ~%v", tc.input, got, tc.want)
				}
				return
			}
			if got != tc.want {
				t.Errorf("toFloat64(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
