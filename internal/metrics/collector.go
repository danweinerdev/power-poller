package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Collector provides Prometheus metrics collection for KASA devices.
type Collector struct {
	mu      sync.RWMutex
	metrics map[string]*deviceMetrics

	// Prometheus descriptors
	voltageDesc   *prometheus.Desc
	currentDesc   *prometheus.Desc
	powerDesc     *prometheus.Desc
	totalDesc     *prometheus.Desc
	stateDesc     *prometheus.Desc
	brightnessDesc *prometheus.Desc
	hueDesc       *prometheus.Desc
	saturationDesc *prometheus.Desc
	colorTempDesc *prometheus.Desc
}

// deviceMetrics holds the latest values for a device.
type deviceMetrics struct {
	tags       map[string]string
	voltage    float64
	current    float64
	power      float64
	total      float64
	state      float64 // 0 or 1
	brightness float64
	hue        float64
	saturation float64
	colorTemp  float64
	hasEmeter  bool
	hasBulb    bool
}

// NewCollector creates a new Prometheus collector.
func NewCollector() *Collector {
	return &Collector{
		metrics: make(map[string]*deviceMetrics),
		voltageDesc: prometheus.NewDesc(
			"kasa_voltage_volts",
			"Current voltage in volts",
			[]string{"device", "address"}, nil,
		),
		currentDesc: prometheus.NewDesc(
			"kasa_current_amperes",
			"Current in amperes",
			[]string{"device", "address"}, nil,
		),
		powerDesc: prometheus.NewDesc(
			"kasa_power_watts",
			"Current power consumption in watts",
			[]string{"device", "address"}, nil,
		),
		totalDesc: prometheus.NewDesc(
			"kasa_total_kwh",
			"Total energy consumption in kWh",
			[]string{"device", "address"}, nil,
		),
		stateDesc: prometheus.NewDesc(
			"kasa_state",
			"Device on/off state (1=on, 0=off)",
			[]string{"device", "address"}, nil,
		),
		brightnessDesc: prometheus.NewDesc(
			"kasa_brightness_percent",
			"Bulb brightness percentage (0-100)",
			[]string{"device", "address"}, nil,
		),
		hueDesc: prometheus.NewDesc(
			"kasa_hue_degrees",
			"Bulb hue in degrees (0-360)",
			[]string{"device", "address"}, nil,
		),
		saturationDesc: prometheus.NewDesc(
			"kasa_saturation_percent",
			"Bulb saturation percentage (0-100)",
			[]string{"device", "address"}, nil,
		),
		colorTempDesc: prometheus.NewDesc(
			"kasa_color_temp_kelvin",
			"Bulb color temperature in Kelvin",
			[]string{"device", "address"}, nil,
		),
	}
}

// Update records new metric values for a device.
func (c *Collector) Update(deviceName string, m *Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	dm, ok := c.metrics[deviceName]
	if !ok {
		dm = &deviceMetrics{
			tags: make(map[string]string),
		}
		c.metrics[deviceName] = dm
	}

	// Copy tags
	for k, v := range m.Tags {
		dm.tags[k] = v
	}

	// Update values from fields
	for k, v := range m.Fields {
		fv := toFloat64(v)
		switch k {
		case "voltage":
			dm.voltage = fv
			dm.hasEmeter = true
		case "current":
			dm.current = fv
			dm.hasEmeter = true
		case "power":
			dm.power = fv
			dm.hasEmeter = true
		case "total":
			dm.total = fv
			dm.hasEmeter = true
		case "on_off", "state":
			dm.state = fv
		case "brightness":
			dm.brightness = fv
			dm.hasBulb = true
		case "hue":
			dm.hue = fv
			dm.hasBulb = true
		case "saturation":
			dm.saturation = fv
			dm.hasBulb = true
		case "color_temp":
			dm.colorTemp = fv
			dm.hasBulb = true
		}
	}
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.voltageDesc
	ch <- c.currentDesc
	ch <- c.powerDesc
	ch <- c.totalDesc
	ch <- c.stateDesc
	ch <- c.brightnessDesc
	ch <- c.hueDesc
	ch <- c.saturationDesc
	ch <- c.colorTempDesc
}

// Collect implements prometheus.Collector.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for deviceName, dm := range c.metrics {
		address := dm.tags["address"]
		if address == "" {
			address = "unknown"
		}

		labels := []string{deviceName, address}

		// Always report state
		ch <- prometheus.MustNewConstMetric(c.stateDesc, prometheus.GaugeValue, dm.state, labels...)

		// Emeter metrics
		if dm.hasEmeter {
			ch <- prometheus.MustNewConstMetric(c.voltageDesc, prometheus.GaugeValue, dm.voltage, labels...)
			ch <- prometheus.MustNewConstMetric(c.currentDesc, prometheus.GaugeValue, dm.current, labels...)
			ch <- prometheus.MustNewConstMetric(c.powerDesc, prometheus.GaugeValue, dm.power, labels...)
			ch <- prometheus.MustNewConstMetric(c.totalDesc, prometheus.GaugeValue, dm.total, labels...)
		}

		// Bulb metrics
		if dm.hasBulb {
			ch <- prometheus.MustNewConstMetric(c.brightnessDesc, prometheus.GaugeValue, dm.brightness, labels...)
			ch <- prometheus.MustNewConstMetric(c.hueDesc, prometheus.GaugeValue, dm.hue, labels...)
			ch <- prometheus.MustNewConstMetric(c.saturationDesc, prometheus.GaugeValue, dm.saturation, labels...)
			ch <- prometheus.MustNewConstMetric(c.colorTempDesc, prometheus.GaugeValue, dm.colorTemp, labels...)
		}
	}
}

// toFloat64 converts various numeric types to float64.
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int8:
		return float64(val)
	case int16:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	case uint:
		return float64(val)
	case uint8:
		return float64(val)
	case uint16:
		return float64(val)
	case uint32:
		return float64(val)
	case uint64:
		return float64(val)
	case bool:
		if val {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// Clear removes all stored metrics.
func (c *Collector) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics = make(map[string]*deviceMetrics)
}

// DeviceCount returns the number of devices being tracked.
func (c *Collector) DeviceCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.metrics)
}
