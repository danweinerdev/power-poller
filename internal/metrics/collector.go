package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Collector provides Prometheus metrics collection for KASA devices.
type Collector struct {
	mu      sync.RWMutex
	metrics map[string]*deviceMetrics
	poller  *pollerMetrics

	// Prometheus descriptors - emeter
	voltageDesc    *prometheus.Desc
	currentDesc    *prometheus.Desc
	powerDesc      *prometheus.Desc
	totalDesc      *prometheus.Desc
	stateDesc      *prometheus.Desc
	brightnessDesc *prometheus.Desc
	hueDesc        *prometheus.Desc
	saturationDesc *prometheus.Desc
	colorTempDesc  *prometheus.Desc

	// Prometheus descriptors - device stats
	responseTimeDesc *prometheus.Desc
	deviceSuccessDesc *prometheus.Desc
	deviceErrorsDesc *prometheus.Desc
	devicePollsDesc  *prometheus.Desc
	rssiDesc         *prometheus.Desc

	// Prometheus descriptors - poller stats
	pollDurationDesc    *prometheus.Desc
	pollDevicesDesc     *prometheus.Desc
	pollSuccessDesc     *prometheus.Desc
	pollFailedDesc      *prometheus.Desc
	pollMetricsDesc     *prometheus.Desc
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

	// Device stats
	responseTimeMs float64
	success        float64
	errorsTotal    float64 // cumulative counter
	pollsTotal     float64 // cumulative counter
	rssi           float64
	hasStats       bool
}

// pollerMetrics holds aggregate poller stats.
type pollerMetrics struct {
	pollDurationMs   float64
	devicesPolled    float64
	devicesSuccess   float64
	devicesFailed    float64
	metricsCollected float64
}

// NewCollector creates a new Prometheus collector.
func NewCollector() *Collector {
	// Labels for device stats include device info
	deviceStatsLabels := []string{"device", "address", "model", "hw_version", "sw_version", "device_type"}

	return &Collector{
		metrics: make(map[string]*deviceMetrics),
		poller:  &pollerMetrics{},

		// Emeter descriptors
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

		// Device stats descriptors
		responseTimeDesc: prometheus.NewDesc(
			"kasa_device_response_time_seconds",
			"Device poll response time in seconds",
			deviceStatsLabels, nil,
		),
		deviceSuccessDesc: prometheus.NewDesc(
			"kasa_device_success",
			"Device poll success (1=success, 0=failure)",
			deviceStatsLabels, nil,
		),
		deviceErrorsDesc: prometheus.NewDesc(
			"kasa_device_errors_total",
			"Total number of device poll errors",
			deviceStatsLabels, nil,
		),
		devicePollsDesc: prometheus.NewDesc(
			"kasa_device_polls_total",
			"Total number of device polls",
			deviceStatsLabels, nil,
		),
		rssiDesc: prometheus.NewDesc(
			"kasa_device_rssi_dbm",
			"Device WiFi signal strength in dBm",
			deviceStatsLabels, nil,
		),

		// Poller stats descriptors
		pollDurationDesc: prometheus.NewDesc(
			"kasa_poll_duration_seconds",
			"Duration of the last poll cycle in seconds",
			[]string{"poller"}, nil,
		),
		pollDevicesDesc: prometheus.NewDesc(
			"kasa_poll_devices_total",
			"Number of devices polled in the last cycle",
			[]string{"poller"}, nil,
		),
		pollSuccessDesc: prometheus.NewDesc(
			"kasa_poll_devices_success",
			"Number of devices successfully polled in the last cycle",
			[]string{"poller"}, nil,
		),
		pollFailedDesc: prometheus.NewDesc(
			"kasa_poll_devices_failed",
			"Number of devices that failed polling in the last cycle",
			[]string{"poller"}, nil,
		),
		pollMetricsDesc: prometheus.NewDesc(
			"kasa_poll_metrics_collected",
			"Number of metrics collected in the last cycle",
			[]string{"poller"}, nil,
		),
	}
}

// Update records new metric values for a device.
func (c *Collector) Update(deviceName string, m *Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Handle poller_stats separately
	if m.Measurement == "poller_stats" {
		c.updatePollerStats(m)
		return
	}

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

	// Handle device_stats measurement
	if m.Measurement == "device_stats" {
		c.updateDeviceStats(dm, m)
		return
	}

	// Update values from fields (emeter and bulb metrics)
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

// updateDeviceStats updates device stats from a device_stats metric (must hold lock).
func (c *Collector) updateDeviceStats(dm *deviceMetrics, m *Metric) {
	dm.hasStats = true
	dm.pollsTotal++ // Increment poll counter

	for k, v := range m.Fields {
		fv := toFloat64(v)
		switch k {
		case "response_time_ms":
			dm.responseTimeMs = fv
		case "success":
			dm.success = fv
		case "error_count":
			dm.errorsTotal += fv // Accumulate errors
		case "rssi":
			dm.rssi = fv
		}
	}
}

// updatePollerStats updates poller stats from a poller_stats metric (must hold lock).
func (c *Collector) updatePollerStats(m *Metric) {
	for k, v := range m.Fields {
		fv := toFloat64(v)
		switch k {
		case "poll_duration_ms":
			c.poller.pollDurationMs = fv
		case "devices_polled":
			c.poller.devicesPolled = fv
		case "devices_success":
			c.poller.devicesSuccess = fv
		case "devices_failed":
			c.poller.devicesFailed = fv
		case "metrics_collected":
			c.poller.metricsCollected = fv
		}
	}
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	// Emeter descriptors
	ch <- c.voltageDesc
	ch <- c.currentDesc
	ch <- c.powerDesc
	ch <- c.totalDesc
	ch <- c.stateDesc
	ch <- c.brightnessDesc
	ch <- c.hueDesc
	ch <- c.saturationDesc
	ch <- c.colorTempDesc

	// Device stats descriptors
	ch <- c.responseTimeDesc
	ch <- c.deviceSuccessDesc
	ch <- c.deviceErrorsDesc
	ch <- c.devicePollsDesc
	ch <- c.rssiDesc

	// Poller stats descriptors
	ch <- c.pollDurationDesc
	ch <- c.pollDevicesDesc
	ch <- c.pollSuccessDesc
	ch <- c.pollFailedDesc
	ch <- c.pollMetricsDesc
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

		// Device stats metrics
		if dm.hasStats {
			statsLabels := []string{
				deviceName,
				address,
				getTagOrDefault(dm.tags, "model", "unknown"),
				getTagOrDefault(dm.tags, "hw_version", "unknown"),
				getTagOrDefault(dm.tags, "sw_version", "unknown"),
				getTagOrDefault(dm.tags, "device_type", "unknown"),
			}

			// Response time in seconds (convert from ms)
			ch <- prometheus.MustNewConstMetric(c.responseTimeDesc, prometheus.GaugeValue, dm.responseTimeMs/1000.0, statsLabels...)
			ch <- prometheus.MustNewConstMetric(c.deviceSuccessDesc, prometheus.GaugeValue, dm.success, statsLabels...)
			ch <- prometheus.MustNewConstMetric(c.deviceErrorsDesc, prometheus.CounterValue, dm.errorsTotal, statsLabels...)
			ch <- prometheus.MustNewConstMetric(c.devicePollsDesc, prometheus.CounterValue, dm.pollsTotal, statsLabels...)
			ch <- prometheus.MustNewConstMetric(c.rssiDesc, prometheus.GaugeValue, dm.rssi, statsLabels...)
		}
	}

	// Poller stats
	pollerLabel := []string{"kasa-monitor"}
	ch <- prometheus.MustNewConstMetric(c.pollDurationDesc, prometheus.GaugeValue, c.poller.pollDurationMs/1000.0, pollerLabel...)
	ch <- prometheus.MustNewConstMetric(c.pollDevicesDesc, prometheus.GaugeValue, c.poller.devicesPolled, pollerLabel...)
	ch <- prometheus.MustNewConstMetric(c.pollSuccessDesc, prometheus.GaugeValue, c.poller.devicesSuccess, pollerLabel...)
	ch <- prometheus.MustNewConstMetric(c.pollFailedDesc, prometheus.GaugeValue, c.poller.devicesFailed, pollerLabel...)
	ch <- prometheus.MustNewConstMetric(c.pollMetricsDesc, prometheus.GaugeValue, c.poller.metricsCollected, pollerLabel...)
}

// getTagOrDefault returns the tag value or a default if not present.
func getTagOrDefault(tags map[string]string, key, defaultVal string) string {
	if v, ok := tags[key]; ok && v != "" {
		return v
	}
	return defaultVal
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
