package config

import (
	"fmt"
	"net"
	"strings"
)

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no errors"
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	msgs := make([]string, len(e))
	for i, err := range e {
		msgs[i] = err.Error()
	}
	return fmt.Sprintf("multiple validation errors:\n  - %s", strings.Join(msgs, "\n  - "))
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	var errs ValidationErrors

	errs = append(errs, c.validateGlobal()...)
	errs = append(errs, c.validateInfluxDB()...)
	errs = append(errs, c.validatePrometheus()...)
	errs = append(errs, c.validateDevices()...)
	errs = append(errs, c.validateMeasurements()...)

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (c *Config) validateGlobal() ValidationErrors {
	var errs ValidationErrors

	if c.Global.PollInterval.Duration <= 0 {
		errs = append(errs, ValidationError{
			Field:   "global.poll_interval",
			Message: "must be positive",
		})
	}

	if c.Global.DeviceTimeout.Duration <= 0 {
		errs = append(errs, ValidationError{
			Field:   "global.device_timeout",
			Message: "must be positive",
		})
	}

	if c.Global.BatchSize <= 0 {
		errs = append(errs, ValidationError{
			Field:   "global.batch_size",
			Message: "must be positive",
		})
	}

	validLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLevels[strings.ToLower(c.Global.LogLevel)] {
		errs = append(errs, ValidationError{
			Field:   "global.log_level",
			Message: "must be one of: debug, info, warn, error",
		})
	}

	return errs
}

func (c *Config) validateInfluxDB() ValidationErrors {
	var errs ValidationErrors

	if !c.InfluxDB.Enabled {
		return errs
	}

	if c.InfluxDB.Server == "" {
		errs = append(errs, ValidationError{
			Field:   "influxdb.server",
			Message: "required when InfluxDB is enabled",
		})
	}

	if c.InfluxDB.Port <= 0 || c.InfluxDB.Port > 65535 {
		errs = append(errs, ValidationError{
			Field:   "influxdb.port",
			Message: "must be a valid port (1-65535)",
		})
	}

	if c.InfluxDB.Token == "" {
		errs = append(errs, ValidationError{
			Field:   "influxdb.token",
			Message: "required when InfluxDB is enabled",
		})
	}

	if c.InfluxDB.Org == "" {
		errs = append(errs, ValidationError{
			Field:   "influxdb.org",
			Message: "required when InfluxDB is enabled",
		})
	}

	if c.InfluxDB.Bucket == "" {
		errs = append(errs, ValidationError{
			Field:   "influxdb.bucket",
			Message: "required when InfluxDB is enabled",
		})
	}

	return errs
}

func (c *Config) validatePrometheus() ValidationErrors {
	var errs ValidationErrors

	if !c.Prometheus.Enabled {
		return errs
	}

	if c.Prometheus.Port <= 0 || c.Prometheus.Port > 65535 {
		errs = append(errs, ValidationError{
			Field:   "prometheus.port",
			Message: "must be a valid port (1-65535)",
		})
	}

	if c.Prometheus.Path == "" {
		errs = append(errs, ValidationError{
			Field:   "prometheus.path",
			Message: "required when Prometheus is enabled",
		})
	}

	if !strings.HasPrefix(c.Prometheus.Path, "/") {
		errs = append(errs, ValidationError{
			Field:   "prometheus.path",
			Message: "must start with /",
		})
	}

	return errs
}

func (c *Config) validateDevices() ValidationErrors {
	var errs ValidationErrors

	if len(c.Devices) == 0 {
		errs = append(errs, ValidationError{
			Field:   "devices",
			Message: "at least one device must be configured",
		})
		return errs
	}

	for name, dev := range c.Devices {
		prefix := fmt.Sprintf("devices.%s", name)

		if dev.Address == "" {
			errs = append(errs, ValidationError{
				Field:   prefix + ".address",
				Message: "required",
			})
		} else if ip := net.ParseIP(dev.Address); ip == nil {
			// Try parsing as hostname:port or just hostname
			host := dev.Address
			if h, _, err := net.SplitHostPort(dev.Address); err == nil {
				host = h
			}
			// Basic hostname validation
			if host == "" {
				errs = append(errs, ValidationError{
					Field:   prefix + ".address",
					Message: "invalid IP address or hostname",
				})
			}
		}

		if len(dev.Measurements) == 0 && !dev.HasChildren {
			errs = append(errs, ValidationError{
				Field:   prefix + ".measurements",
				Message: "at least one measurement required (unless has_children is true)",
			})
		}

		// Validate measurements exist
		for _, m := range dev.Measurements {
			if _, ok := c.Measurements[m]; !ok {
				errs = append(errs, ValidationError{
					Field:   prefix + ".measurements",
					Message: fmt.Sprintf("measurement %q not defined", m),
				})
			}
		}

		// Validate children
		if dev.HasChildren {
			if len(dev.Children) == 0 {
				errs = append(errs, ValidationError{
					Field:   prefix + ".children",
					Message: "at least one child required when has_children is true",
				})
			}

			for childName, child := range dev.Children {
				childPrefix := fmt.Sprintf("%s.children.%s", prefix, childName)

				if child.Index < 0 {
					errs = append(errs, ValidationError{
						Field:   childPrefix + ".index",
						Message: "must be non-negative",
					})
				}

				for _, m := range child.Measurements {
					if _, ok := c.Measurements[m]; !ok {
						errs = append(errs, ValidationError{
							Field:   childPrefix + ".measurements",
							Message: fmt.Sprintf("measurement %q not defined", m),
						})
					}
				}
			}
		}
	}

	return errs
}

func (c *Config) validateMeasurements() ValidationErrors {
	var errs ValidationErrors

	for name, m := range c.Measurements {
		prefix := fmt.Sprintf("measurements.%s", name)

		if len(m.Fields) == 0 {
			errs = append(errs, ValidationError{
				Field:   prefix + ".fields",
				Message: "at least one field required",
			})
			continue
		}

		for fieldName, fieldType := range m.Fields {
			switch fieldType {
			case FieldTypeFloat, FieldTypeInt, FieldTypeString, FieldTypeBool:
				// Valid
			default:
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("%s.fields.%s", prefix, fieldName),
					Message: fmt.Sprintf("invalid field type %q (must be float, int, string, or bool)", fieldType),
				})
			}
		}
	}

	return errs
}
