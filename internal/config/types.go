package config

import "time"

// Config represents the complete application configuration.
type Config struct {
	Global       GlobalConfig                `toml:"global"`
	InfluxDB     InfluxDBConfig              `toml:"influxdb"`
	Prometheus   PrometheusConfig            `toml:"prometheus"`
	Devices      map[string]DeviceConfig     `toml:"devices"`
	Measurements map[string]MeasurementConfig `toml:"measurements"`
}

// GlobalConfig contains global application settings.
type GlobalConfig struct {
	PollInterval  Duration `toml:"poll_interval"`
	LogLevel      string   `toml:"log_level"`
	DeviceTimeout Duration `toml:"device_timeout"`
	BatchSize     int      `toml:"batch_size"`
	RetryAttempts int      `toml:"retry_attempts"`
	RetryDelay    Duration `toml:"retry_delay"`
}

// InfluxDBConfig contains InfluxDB connection settings.
type InfluxDBConfig struct {
	Enabled bool   `toml:"enabled"`
	Server  string `toml:"server"`
	Port    int    `toml:"port"`
	Token   string `toml:"token"`
	Org     string `toml:"org"`
	Bucket  string `toml:"bucket"`
	TLS     bool   `toml:"tls"`
}

// PrometheusConfig contains Prometheus exporter settings.
type PrometheusConfig struct {
	Enabled bool   `toml:"enabled"`
	Port    int    `toml:"port"`
	Path    string `toml:"path"`
}

// DeviceConfig represents a single device configuration.
type DeviceConfig struct {
	Address      string                   `toml:"address"`
	Measurements []string                 `toml:"measurements"`
	Tags         map[string]string        `toml:"tags"`
	HasChildren  bool                     `toml:"has_children"`
	PollParent   bool                     `toml:"poll_parent"`
	Children     map[string]ChildConfig   `toml:"children"`
}

// ChildConfig represents a child device (outlet) configuration.
type ChildConfig struct {
	Index        int               `toml:"index"`
	Name         string            `toml:"name"`
	Measurements []string          `toml:"measurements"`
	Tags         map[string]string `toml:"tags"`
}

// MeasurementConfig defines field types for a measurement.
type MeasurementConfig struct {
	Fields map[string]FieldType `toml:"fields"`
}

// FieldType represents the data type of a measurement field.
type FieldType string

const (
	FieldTypeFloat   FieldType = "float"
	FieldTypeInt     FieldType = "int"
	FieldTypeString  FieldType = "string"
	FieldTypeBool    FieldType = "bool"
)

// Duration is a wrapper around time.Duration that supports TOML parsing.
type Duration struct {
	time.Duration
}

// UnmarshalText implements encoding.TextUnmarshaler for Duration.
func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

// MarshalText implements encoding.TextMarshaler for Duration.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Global: GlobalConfig{
			PollInterval:  Duration{10 * time.Second},
			LogLevel:      "info",
			DeviceTimeout: Duration{5 * time.Second},
			BatchSize:     10,
			RetryAttempts: 3,
			RetryDelay:    Duration{1 * time.Second},
		},
		InfluxDB: InfluxDBConfig{
			Enabled: false,
			Server:  "localhost",
			Port:    8086,
			Bucket:  "kasa",
		},
		Prometheus: PrometheusConfig{
			Enabled: false,
			Port:    9090,
			Path:    "/metrics",
		},
		Devices:      make(map[string]DeviceConfig),
		Measurements: make(map[string]MeasurementConfig),
	}
}
