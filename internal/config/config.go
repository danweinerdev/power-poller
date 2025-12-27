package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Load reads and parses a TOML configuration file.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// LoadFromString parses configuration from a TOML string.
func LoadFromString(data string) (*Config, error) {
	cfg := DefaultConfig()

	if err := toml.Unmarshal([]byte(data), cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// FindConfigFile searches for a config file in common locations.
func FindConfigFile(name string) (string, error) {
	// Check if absolute path
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
		return "", fmt.Errorf("config file not found: %s", name)
	}

	// Search paths
	searchPaths := []string{
		name,                                    // Current directory
		filepath.Join("/etc", name),             // System config
		filepath.Join("/etc/kasa-monitor", name), // App-specific
	}

	// Add user config directory
	if home, err := os.UserHomeDir(); err == nil {
		searchPaths = append(searchPaths,
			filepath.Join(home, ".config", "kasa-monitor", name),
			filepath.Join(home, "."+name),
		)
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("config file not found: %s (searched: %v)", name, searchPaths)
}

// URL returns the full InfluxDB URL.
func (c *InfluxDBConfig) URL() string {
	scheme := "http"
	if c.TLS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, c.Server, c.Port)
}

// GetDeviceTags returns merged tags for a device (device tags override global).
func (cfg *Config) GetDeviceTags(deviceName string, globalTags map[string]string) map[string]string {
	result := make(map[string]string)

	// Copy global tags
	for k, v := range globalTags {
		result[k] = v
	}

	// Override with device-specific tags
	if dev, ok := cfg.Devices[deviceName]; ok {
		for k, v := range dev.Tags {
			result[k] = v
		}
		// Add device name as a tag
		result["device"] = deviceName
	}

	return result
}

// GetChildTags returns merged tags for a child device.
func (cfg *Config) GetChildTags(deviceName, childName string, parentTags map[string]string) map[string]string {
	result := make(map[string]string)

	// Copy parent tags
	for k, v := range parentTags {
		result[k] = v
	}

	// Override with child-specific tags
	if dev, ok := cfg.Devices[deviceName]; ok {
		if child, ok := dev.Children[childName]; ok {
			for k, v := range child.Tags {
				result[k] = v
			}
			// Add child name as a tag
			result["outlet"] = childName
			if child.Name != "" {
				result["outlet_name"] = child.Name
			}
		}
	}

	return result
}

// GetMeasurement returns a measurement config by name.
func (cfg *Config) GetMeasurement(name string) (*MeasurementConfig, bool) {
	m, ok := cfg.Measurements[name]
	return &m, ok
}
