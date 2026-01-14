package config

import "time"

// Config represents the complete application configuration.
type Config struct {
	Global       GlobalConfig                 `toml:"global"`
	InfluxDB     InfluxDBConfig               `toml:"influxdb"`
	Prometheus   PrometheusConfig             `toml:"prometheus"`
	KLAP         KLAPConfig                   `toml:"klap"`
	API          APIConfig                    `toml:"api"`
	Devices      map[string]DeviceConfig      `toml:"devices"`
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

// KLAPConfig contains global KLAP protocol settings.
type KLAPConfig struct {
	// DefaultUsername is used for devices that don't specify credentials.
	// Defaults to "kasa@tp-link.net" for unprovisioned devices.
	DefaultUsername string `toml:"default_username"`

	// DefaultPassword is used for devices that don't specify credentials.
	// Defaults to empty string for unprovisioned devices.
	DefaultPassword string `toml:"default_password"`
}

// APIConfig contains REST API server settings.
type APIConfig struct {
	Enabled            bool     `toml:"enabled"`
	Listen             string   `toml:"listen"`
	Port               int      `toml:"port"`
	CORSEnabled        bool     `toml:"cors_enabled"`
	CORSAllowedOrigins []string `toml:"cors_allowed_origins"`
	AuthEnabled        bool     `toml:"auth_enabled"`
	AuthUsername       string   `toml:"auth_username"`
	AuthPassword       string   `toml:"auth_password"`
}

// DeviceConfig represents a single device configuration.
type DeviceConfig struct {
	Address      string                 `toml:"address"`
	Measurements []string               `toml:"measurements"`
	Tags         map[string]string      `toml:"tags"`
	HasChildren  bool                   `toml:"has_children"`
	PollParent   bool                   `toml:"poll_parent"`
	Children     map[string]ChildConfig `toml:"children"`

	// Protocol specifies the communication protocol: "legacy", "klap", or "" for auto-detect
	Protocol string `toml:"protocol"`

	// KLAP authentication credentials (optional, uses defaults if not specified)
	Username string `toml:"username"`
	Password string `toml:"password"`
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

// GetDeviceCredentials returns the KLAP credentials for a device.
// It returns the device-specific credentials if set, otherwise falls back to global KLAP defaults.
// Returns nil if no credentials are configured (use protocol defaults).
func (c *Config) GetDeviceCredentials(deviceName string) (username, password string, hasCredentials bool) {
	devCfg, ok := c.Devices[deviceName]
	if !ok {
		return "", "", false
	}

	// Use device-specific credentials if set
	if devCfg.Username != "" {
		return devCfg.Username, devCfg.Password, true
	}

	// Fall back to global KLAP defaults
	if c.KLAP.DefaultUsername != "" {
		return c.KLAP.DefaultUsername, c.KLAP.DefaultPassword, true
	}

	// No credentials configured, use protocol defaults
	return "", "", false
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
		KLAP: KLAPConfig{
			// Empty strings = use protocol defaults (kasa@tp-link.net / "")
			DefaultUsername: "",
			DefaultPassword: "",
		},
		API: APIConfig{
			Enabled:            false,
			Listen:             "0.0.0.0",
			Port:               8080,
			CORSEnabled:        true,
			CORSAllowedOrigins: []string{"http://localhost:5173"},
			AuthEnabled:        false,
		},
		Devices:      make(map[string]DeviceConfig),
		Measurements: make(map[string]MeasurementConfig),
	}
}
