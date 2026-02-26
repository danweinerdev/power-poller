package device

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danweinerdev/power-poller/pkg/kasa/command"
	"github.com/danweinerdev/power-poller/pkg/kasa/protocol"
	"github.com/danweinerdev/power-poller/pkg/kasa/types"
)

// LoadOptions configures device loading behavior.
type LoadOptions struct {
	// TransportOptions for legacy TCP transport
	TransportOptions []protocol.TransportOption

	// KLAPOptions for KLAP HTTP transport
	KLAPOptions []protocol.KLAPOption

	// SecurePassthroughOptions for SecurePassthrough HTTP transport
	SecurePassthroughOptions []protocol.SecurePassthroughOption

	// Credentials for KLAP authentication (optional, uses defaults if nil)
	Credentials *protocol.Credentials

	// ProtocolCache for caching detected protocols (optional)
	ProtocolCache *protocol.ProtocolCache

	// ForceProtocol forces a specific protocol instead of auto-detecting
	ForceProtocol protocol.ProtocolType
}

// LoadOption is a functional option for Load.
type LoadOption func(*LoadOptions)

// WithTransportOptions sets transport options for legacy connections.
func WithTransportOptions(opts ...protocol.TransportOption) LoadOption {
	return func(o *LoadOptions) {
		o.TransportOptions = opts
	}
}

// WithKLAPOptions sets options for KLAP connections.
func WithKLAPOptions(opts ...protocol.KLAPOption) LoadOption {
	return func(o *LoadOptions) {
		o.KLAPOptions = opts
	}
}

// WithSecurePassthroughOptions sets options for SecurePassthrough connections.
func WithSecurePassthroughOptions(opts ...protocol.SecurePassthroughOption) LoadOption {
	return func(o *LoadOptions) {
		o.SecurePassthroughOptions = opts
	}
}

// WithCredentials sets KLAP authentication credentials.
func WithCredentials(creds *protocol.Credentials) LoadOption {
	return func(o *LoadOptions) {
		o.Credentials = creds
	}
}

// WithProtocolCache sets a protocol cache for detection results.
func WithProtocolCache(cache *protocol.ProtocolCache) LoadOption {
	return func(o *LoadOptions) {
		o.ProtocolCache = cache
	}
}

// WithForceProtocol forces a specific protocol.
func WithForceProtocol(proto protocol.ProtocolType) LoadOption {
	return func(o *LoadOptions) {
		o.ForceProtocol = proto
	}
}

// Load connects to a device and returns the appropriate typed instance.
// It auto-detects whether the device uses legacy (TCP/XOR) or KLAP (HTTP/AES) protocol.
func Load(ctx context.Context, host string, opts ...LoadOption) (Device, error) {
	options := &LoadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Detect or use forced protocol
	proto := options.ForceProtocol
	if proto == "" {
		if options.ProtocolCache != nil {
			proto = options.ProtocolCache.Detect(ctx, host)
		} else {
			// Create temporary cache for detection
			cache := protocol.NewProtocolCache()
			proto = cache.Detect(ctx, host)
		}
	}

	// Create appropriate transport
	var transport protocol.Transporter
	switch proto {
	case protocol.ProtocolKLAP:
		klapOpts := options.KLAPOptions
		if options.Credentials != nil {
			klapOpts = append(klapOpts, protocol.WithCredentials(options.Credentials))
		}
		transport = protocol.NewKLAPTransport(host, klapOpts...)
	case protocol.ProtocolSecurePassthrough:
		transport = protocol.NewSecurePassthroughTransport(host, options.SecurePassthroughOptions...)
	default:
		// Use legacy transport (also for unknown)
		transport = protocol.NewTransport(host, options.TransportOptions...)
	}

	return LoadWithTransport(ctx, transport)
}

// LoadWithTransport connects to a device using a specific transport.
func LoadWithTransport(ctx context.Context, transport protocol.Transporter) (Device, error) {
	// Connect and query sysinfo
	if err := transport.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	var sysinfo *types.SysInfo

	// Check if this is a SecurePassthrough (TAPO) transport
	if _, ok := transport.(*protocol.SecurePassthroughTransport); ok {
		// Use TAPO command format
		resp, err := transport.Send(ctx, command.TapoGetDeviceInfo())
		if err != nil {
			transport.Close()
			return nil, fmt.Errorf("failed to get device info: %w", err)
		}

		var result types.TapoDeviceInfoResponse
		if err := json.Unmarshal(resp, &result); err != nil {
			transport.Close()
			return nil, fmt.Errorf("failed to parse device info: %w", err)
		}

		if result.ErrorCode != 0 {
			transport.Close()
			return nil, fmt.Errorf("device error: code %d", result.ErrorCode)
		}

		sysinfo = result.Result.ToSysInfo()
	} else {
		// Use legacy KASA command format
		resp, err := transport.Send(ctx, command.GetSysInfo())
		if err != nil {
			transport.Close()
			return nil, fmt.Errorf("failed to get sysinfo: %w", err)
		}

		var result types.SysInfoResponse
		if err := json.Unmarshal(resp, &result); err != nil {
			transport.Close()
			return nil, fmt.Errorf("failed to parse sysinfo: %w", err)
		}

		if result.System.GetSysinfo.ErrCode != 0 {
			transport.Close()
			return nil, fmt.Errorf("device error: %s (code %d)", result.System.GetSysinfo.ErrMsg, result.System.GetSysinfo.ErrCode)
		}

		sysinfo = &result.System.GetSysinfo
	}

	// Close the temporary connection - device will reconnect
	transport.Close()

	// Determine device type and create appropriate instance
	devType := sysinfo.DetectDeviceType()

	var dev Device
	switch devType {
	case types.DeviceTypePlug:
		dev = NewPlugWithTransport(transport)
	case types.DeviceTypeBulb:
		dev = NewBulbWithTransport(transport)
	case types.DeviceTypeLightStrip:
		dev = NewLightStripWithTransport(transport)
	case types.DeviceTypePowerStrip:
		dev = NewPowerStripWithTransport(transport)
	default:
		// Fall back to a basic plug for unknown types
		dev = NewPlugWithTransport(transport)
	}

	// Connect and update with fresh sysinfo
	if err := dev.Connect(ctx); err != nil {
		return nil, err
	}

	return dev, nil
}

// LoadLegacy connects to a device using the legacy TCP/XOR protocol.
// This is a convenience function for devices known to use legacy protocol.
func LoadLegacy(ctx context.Context, host string, opts ...protocol.TransportOption) (Device, error) {
	return Load(ctx, host, WithForceProtocol(protocol.ProtocolLegacy), WithTransportOptions(opts...))
}

// LoadKLAP connects to a device using the KLAP HTTP/AES protocol.
// This is a convenience function for devices known to use KLAP protocol.
func LoadKLAP(ctx context.Context, host string, creds *protocol.Credentials, opts ...protocol.KLAPOption) (Device, error) {
	loadOpts := []LoadOption{WithForceProtocol(protocol.ProtocolKLAP), WithKLAPOptions(opts...)}
	if creds != nil {
		loadOpts = append(loadOpts, WithCredentials(creds))
	}
	return Load(ctx, host, loadOpts...)
}

// LoadSecurePassthrough connects to a device using the SecurePassthrough protocol.
// This is a convenience function for TAPO devices using SecurePassthrough.
func LoadSecurePassthrough(ctx context.Context, host string, opts ...protocol.SecurePassthroughOption) (Device, error) {
	loadOpts := []LoadOption{WithForceProtocol(protocol.ProtocolSecurePassthrough), WithSecurePassthroughOptions(opts...)}
	return Load(ctx, host, loadOpts...)
}

// LoadMultiple connects to multiple devices concurrently.
// If a protocol cache is provided, protocols are detected in parallel first.
func LoadMultiple(ctx context.Context, hosts []string, opts ...LoadOption) ([]Device, []error) {
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
// It auto-detects the protocol if not specified.
func Query(ctx context.Context, host string, cmd interface{}, opts ...LoadOption) ([]byte, error) {
	options := &LoadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Detect or use forced protocol
	proto := options.ForceProtocol
	if proto == "" {
		if options.ProtocolCache != nil {
			proto = options.ProtocolCache.Detect(ctx, host)
		} else {
			cache := protocol.NewProtocolCache()
			proto = cache.Detect(ctx, host)
		}
	}

	// Create appropriate transport
	var transport protocol.Transporter
	switch proto {
	case protocol.ProtocolKLAP:
		klapOpts := options.KLAPOptions
		if options.Credentials != nil {
			klapOpts = append(klapOpts, protocol.WithCredentials(options.Credentials))
		}
		transport = protocol.NewKLAPTransport(host, klapOpts...)
	case protocol.ProtocolSecurePassthrough:
		transport = protocol.NewSecurePassthroughTransport(host, options.SecurePassthroughOptions...)
	default:
		transport = protocol.NewTransport(host, options.TransportOptions...)
	}

	if err := transport.Connect(ctx); err != nil {
		return nil, err
	}
	defer transport.Close()

	return transport.Send(ctx, cmd)
}

// QueryLegacy sends a command using legacy protocol (for backwards compatibility).
func QueryLegacy(ctx context.Context, host string, cmd interface{}, opts ...protocol.TransportOption) ([]byte, error) {
	return protocol.Query(ctx, host, cmd, opts...)
}

// GetSysInfo queries a device's system info without creating a full device instance.
func GetSysInfo(ctx context.Context, host string, opts ...LoadOption) (*types.SysInfo, error) {
	options := &LoadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Detect protocol to determine command format
	proto := options.ForceProtocol
	if proto == "" {
		if options.ProtocolCache != nil {
			proto = options.ProtocolCache.Detect(ctx, host)
		} else {
			cache := protocol.NewProtocolCache()
			proto = cache.Detect(ctx, host)
		}
	}

	if proto == protocol.ProtocolSecurePassthrough {
		// Use TAPO command format
		resp, err := Query(ctx, host, command.TapoGetDeviceInfo(), opts...)
		if err != nil {
			return nil, err
		}

		var result types.TapoDeviceInfoResponse
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to parse device info: %w", err)
		}

		if result.ErrorCode != 0 {
			return nil, fmt.Errorf("device error: code %d", result.ErrorCode)
		}

		return result.Result.ToSysInfo(), nil
	}

	// Use legacy KASA command format
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
func GetEmeterRealtime(ctx context.Context, host string, opts ...LoadOption) (*types.EmeterData, error) {
	options := &LoadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Detect protocol to determine command format
	proto := options.ForceProtocol
	if proto == "" {
		if options.ProtocolCache != nil {
			proto = options.ProtocolCache.Detect(ctx, host)
		} else {
			cache := protocol.NewProtocolCache()
			proto = cache.Detect(ctx, host)
		}
	}

	if proto == protocol.ProtocolSecurePassthrough {
		// Use TAPO command format - try get_emeter_data first
		resp, err := Query(ctx, host, command.TapoGetEmeterData(), opts...)
		if err != nil {
			return nil, err
		}

		var result types.TapoEmeterDataResponse
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to parse emeter data: %w", err)
		}

		if result.ErrorCode == 0 {
			return result.Result.ToEmeterData(), nil
		}

		// Fall back to get_current_power
		resp, err = Query(ctx, host, command.TapoGetCurrentPower(), opts...)
		if err != nil {
			return nil, err
		}

		var powerResult types.TapoCurrentPowerResponse
		if err := json.Unmarshal(resp, &powerResult); err != nil {
			return nil, fmt.Errorf("failed to parse current power: %w", err)
		}

		if powerResult.ErrorCode != 0 {
			return nil, fmt.Errorf("emeter error: code %d", powerResult.ErrorCode)
		}

		return powerResult.Result.ToEmeterData(), nil
	}

	// Use legacy KASA command format
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
