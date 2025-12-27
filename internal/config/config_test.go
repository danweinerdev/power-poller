package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFromString_Valid(t *testing.T) {
	toml := `
[global]
poll_interval = "15s"
log_level = "debug"
device_timeout = "3s"
batch_size = 5

[influxdb]
enabled = true
server = "influx.local"
port = 8086
token = "test-token"
org = "testorg"
bucket = "testbucket"

[devices.test_plug]
address = "192.168.1.100"
measurements = ["power"]
tags = { location = "office" }

[measurements.power.fields]
voltage = "float"
power = "float"
`

	cfg, err := LoadFromString(toml)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Check global settings
	if cfg.Global.PollInterval.Duration != 15*time.Second {
		t.Errorf("poll_interval = %v, want 15s", cfg.Global.PollInterval)
	}
	if cfg.Global.LogLevel != "debug" {
		t.Errorf("log_level = %q, want %q", cfg.Global.LogLevel, "debug")
	}
	if cfg.Global.DeviceTimeout.Duration != 3*time.Second {
		t.Errorf("device_timeout = %v, want 3s", cfg.Global.DeviceTimeout)
	}
	if cfg.Global.BatchSize != 5 {
		t.Errorf("batch_size = %d, want 5", cfg.Global.BatchSize)
	}

	// Check InfluxDB settings
	if !cfg.InfluxDB.Enabled {
		t.Error("influxdb.enabled should be true")
	}
	if cfg.InfluxDB.Server != "influx.local" {
		t.Errorf("influxdb.server = %q, want %q", cfg.InfluxDB.Server, "influx.local")
	}
	if cfg.InfluxDB.URL() != "http://influx.local:8086" {
		t.Errorf("InfluxDB URL = %q, want %q", cfg.InfluxDB.URL(), "http://influx.local:8086")
	}

	// Check device
	dev, ok := cfg.Devices["test_plug"]
	if !ok {
		t.Fatal("device test_plug not found")
	}
	if dev.Address != "192.168.1.100" {
		t.Errorf("device address = %q, want %q", dev.Address, "192.168.1.100")
	}
	if len(dev.Measurements) != 1 || dev.Measurements[0] != "power" {
		t.Errorf("device measurements = %v, want [power]", dev.Measurements)
	}
	if dev.Tags["location"] != "office" {
		t.Errorf("device tag location = %q, want %q", dev.Tags["location"], "office")
	}

	// Check measurement
	m, ok := cfg.Measurements["power"]
	if !ok {
		t.Fatal("measurement power not found")
	}
	if m.Fields["voltage"] != FieldTypeFloat {
		t.Errorf("voltage field type = %q, want %q", m.Fields["voltage"], FieldTypeFloat)
	}
}

func TestLoadFromString_Defaults(t *testing.T) {
	// Minimal valid config
	toml := `
[devices.plug]
address = "192.168.1.1"
measurements = ["power"]

[measurements.power.fields]
power = "float"
`

	cfg, err := LoadFromString(toml)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Check defaults applied
	if cfg.Global.PollInterval.Duration != 10*time.Second {
		t.Errorf("default poll_interval = %v, want 10s", cfg.Global.PollInterval)
	}
	if cfg.Global.LogLevel != "info" {
		t.Errorf("default log_level = %q, want %q", cfg.Global.LogLevel, "info")
	}
	if cfg.Global.DeviceTimeout.Duration != 5*time.Second {
		t.Errorf("default device_timeout = %v, want 5s", cfg.Global.DeviceTimeout)
	}
	if cfg.Global.BatchSize != 10 {
		t.Errorf("default batch_size = %d, want 10", cfg.Global.BatchSize)
	}
}

func TestLoadFromString_PowerStrip(t *testing.T) {
	toml := `
[devices.strip]
address = "192.168.1.50"
has_children = true
poll_parent = true
measurements = ["power"]
tags = { location = "rack" }

[devices.strip.children.outlet_0]
index = 0
name = "server"
measurements = ["power"]
tags = { role = "compute" }

[devices.strip.children.outlet_1]
index = 1
name = "switch"
measurements = ["power"]

[measurements.power.fields]
power = "float"
`

	cfg, err := LoadFromString(toml)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	dev := cfg.Devices["strip"]
	if !dev.HasChildren {
		t.Error("has_children should be true")
	}
	if !dev.PollParent {
		t.Error("poll_parent should be true")
	}
	if len(dev.Children) != 2 {
		t.Errorf("children count = %d, want 2", len(dev.Children))
	}

	child := dev.Children["outlet_0"]
	if child.Index != 0 {
		t.Errorf("child index = %d, want 0", child.Index)
	}
	if child.Name != "server" {
		t.Errorf("child name = %q, want %q", child.Name, "server")
	}
	if child.Tags["role"] != "compute" {
		t.Errorf("child role tag = %q, want %q", child.Tags["role"], "compute")
	}
}

func TestLoadFromString_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		toml    string
		wantErr string
	}{
		{
			name: "no devices",
			toml: `
[global]
poll_interval = "10s"
`,
			wantErr: "devices: at least one device",
		},
		{
			name: "missing address",
			toml: `
[devices.plug]
measurements = ["power"]
[measurements.power.fields]
power = "float"
`,
			wantErr: "address: required",
		},
		{
			name: "missing measurements definition",
			toml: `
[devices.plug]
address = "192.168.1.1"
measurements = ["undefined"]
`,
			wantErr: "measurement \"undefined\" not defined",
		},
		{
			name: "invalid log level",
			toml: `
[global]
log_level = "invalid"
[devices.plug]
address = "192.168.1.1"
measurements = ["power"]
[measurements.power.fields]
power = "float"
`,
			wantErr: "log_level: must be one of",
		},
		{
			name: "influxdb enabled without token",
			toml: `
[influxdb]
enabled = true
server = "localhost"
org = "test"
bucket = "test"
[devices.plug]
address = "192.168.1.1"
measurements = ["power"]
[measurements.power.fields]
power = "float"
`,
			wantErr: "token: required",
		},
		{
			name: "has_children without children",
			toml: `
[devices.strip]
address = "192.168.1.1"
has_children = true
[measurements.power.fields]
power = "float"
`,
			wantErr: "at least one child required",
		},
		{
			name: "invalid field type",
			toml: `
[devices.plug]
address = "192.168.1.1"
measurements = ["power"]
[measurements.power.fields]
power = "invalid"
`,
			wantErr: "invalid field type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadFromString(tt.toml)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsString(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoad_File(t *testing.T) {
	content := `
[devices.test]
address = "192.168.1.1"
measurements = ["power"]

[measurements.power.fields]
power = "float"
`

	// Create temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if _, ok := cfg.Devices["test"]; !ok {
		t.Error("device test not found")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.toml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestGetDeviceTags(t *testing.T) {
	toml := `
[devices.plug]
address = "192.168.1.1"
measurements = ["power"]
tags = { location = "office", room = "A1" }

[measurements.power.fields]
power = "float"
`

	cfg, err := LoadFromString(toml)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	globalTags := map[string]string{
		"env":      "prod",
		"location": "default", // Should be overridden
	}

	tags := cfg.GetDeviceTags("plug", globalTags)

	if tags["env"] != "prod" {
		t.Errorf("env tag = %q, want %q", tags["env"], "prod")
	}
	if tags["location"] != "office" {
		t.Errorf("location tag = %q, want %q (should override global)", tags["location"], "office")
	}
	if tags["room"] != "A1" {
		t.Errorf("room tag = %q, want %q", tags["room"], "A1")
	}
	if tags["device"] != "plug" {
		t.Errorf("device tag = %q, want %q", tags["device"], "plug")
	}
}

func TestGetChildTags(t *testing.T) {
	toml := `
[devices.strip]
address = "192.168.1.1"
has_children = true
tags = { location = "rack" }

[devices.strip.children.outlet_0]
index = 0
name = "server"
measurements = ["power"]
tags = { role = "compute" }

[measurements.power.fields]
power = "float"
`

	cfg, err := LoadFromString(toml)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	parentTags := map[string]string{
		"device":   "strip",
		"location": "rack",
	}

	tags := cfg.GetChildTags("strip", "outlet_0", parentTags)

	if tags["location"] != "rack" {
		t.Errorf("location tag = %q, want %q", tags["location"], "rack")
	}
	if tags["role"] != "compute" {
		t.Errorf("role tag = %q, want %q", tags["role"], "compute")
	}
	if tags["outlet"] != "outlet_0" {
		t.Errorf("outlet tag = %q, want %q", tags["outlet"], "outlet_0")
	}
	if tags["outlet_name"] != "server" {
		t.Errorf("outlet_name tag = %q, want %q", tags["outlet_name"], "server")
	}
}

func TestInfluxDBURL(t *testing.T) {
	tests := []struct {
		cfg  InfluxDBConfig
		want string
	}{
		{
			cfg:  InfluxDBConfig{Server: "localhost", Port: 8086, TLS: false},
			want: "http://localhost:8086",
		},
		{
			cfg:  InfluxDBConfig{Server: "influx.example.com", Port: 443, TLS: true},
			want: "https://influx.example.com:443",
		},
	}

	for _, tt := range tests {
		got := tt.cfg.URL()
		if got != tt.want {
			t.Errorf("URL() = %q, want %q", got, tt.want)
		}
	}
}

func TestDuration_UnmarshalText(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"10s", 10 * time.Second},
		{"5m", 5 * time.Minute},
		{"1h", time.Hour},
		{"100ms", 100 * time.Millisecond},
		{"1h30m", 90 * time.Minute},
	}

	for _, tt := range tests {
		var d Duration
		err := d.UnmarshalText([]byte(tt.input))
		if err != nil {
			t.Errorf("UnmarshalText(%q) error = %v", tt.input, err)
			continue
		}
		if d.Duration != tt.want {
			t.Errorf("UnmarshalText(%q) = %v, want %v", tt.input, d.Duration, tt.want)
		}
	}
}

func TestDuration_UnmarshalText_Invalid(t *testing.T) {
	var d Duration
	err := d.UnmarshalText([]byte("invalid"))
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
