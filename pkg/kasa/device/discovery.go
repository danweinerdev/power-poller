package device

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/command"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/protocol"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/types"
)

// Load connects to a device and returns the appropriate typed instance.
func Load(ctx context.Context, host string, opts ...protocol.TransportOption) (Device, error) {
	// Create temporary transport to query sysinfo
	transport := protocol.NewTransport(host, opts...)
	if err := transport.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer transport.Close()

	// Query sysinfo
	resp, err := transport.Send(ctx, command.GetSysInfo())
	if err != nil {
		return nil, fmt.Errorf("failed to get sysinfo: %w", err)
	}

	var result types.SysInfoResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse sysinfo: %w", err)
	}

	sysinfo := &result.System.GetSysinfo
	if sysinfo.ErrCode != 0 {
		return nil, fmt.Errorf("device error: %s (code %d)", sysinfo.ErrMsg, sysinfo.ErrCode)
	}

	// Determine device type and create appropriate instance
	devType := sysinfo.DetectDeviceType()

	var dev Device
	switch devType {
	case types.DeviceTypePlug:
		dev = NewPlug(host, opts...)
	case types.DeviceTypeBulb:
		dev = NewBulb(host, opts...)
	case types.DeviceTypeLightStrip:
		dev = NewLightStrip(host, opts...)
	case types.DeviceTypePowerStrip:
		dev = NewPowerStrip(host, opts...)
	default:
		// Fall back to a basic plug for unknown types
		dev = NewPlug(host, opts...)
	}

	// Connect and update with fresh sysinfo
	if err := dev.Connect(ctx); err != nil {
		return nil, err
	}

	return dev, nil
}

// LoadMultiple connects to multiple devices concurrently.
func LoadMultiple(ctx context.Context, hosts []string, opts ...protocol.TransportOption) ([]Device, []error) {
	devices := make([]Device, len(hosts))
	errors := make([]error, len(hosts))

	// Use a semaphore to limit concurrent connections
	sem := make(chan struct{}, 10)
	done := make(chan struct{})

	for i, host := range hosts {
		go func(idx int, h string) {
			sem <- struct{}{}
			defer func() { <-sem }()

			dev, err := Load(ctx, h, opts...)
			devices[idx] = dev
			errors[idx] = err

			select {
			case done <- struct{}{}:
			case <-ctx.Done():
			}
		}(i, host)
	}

	// Wait for all to complete or context cancelled
	for i := 0; i < len(hosts); i++ {
		select {
		case <-done:
		case <-ctx.Done():
			return devices, errors
		}
	}

	return devices, errors
}

// Query sends a command to a device without maintaining a persistent connection.
// Returns the raw response bytes.
func Query(ctx context.Context, host string, cmd interface{}, opts ...protocol.TransportOption) ([]byte, error) {
	return protocol.Query(ctx, host, cmd, opts...)
}

// GetSysInfo queries a device's system info without creating a full device instance.
func GetSysInfo(ctx context.Context, host string, opts ...protocol.TransportOption) (*types.SysInfo, error) {
	resp, err := Query(ctx, host, command.GetSysInfo(), opts...)
	if err != nil {
		return nil, err
	}

	var result types.SysInfoResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse sysinfo: %w", err)
	}

	sysinfo := &result.System.GetSysinfo
	if sysinfo.ErrCode != 0 {
		return nil, fmt.Errorf("device error: %s (code %d)", sysinfo.ErrMsg, sysinfo.ErrCode)
	}

	return sysinfo, nil
}

// GetEmeterRealtime queries a device's energy meter without creating a full device instance.
func GetEmeterRealtime(ctx context.Context, host string, opts ...protocol.TransportOption) (*types.EmeterData, error) {
	resp, err := Query(ctx, host, command.GetEmeterRealtime(), opts...)
	if err != nil {
		return nil, err
	}

	var result types.EmeterRealtimeResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse emeter: %w", err)
	}

	if result.Emeter.GetRealtime.ErrCode != 0 {
		return nil, fmt.Errorf("emeter error: code %d", result.Emeter.GetRealtime.ErrCode)
	}

	normalized := result.Emeter.GetRealtime.Normalize()
	return &normalized, nil
}
