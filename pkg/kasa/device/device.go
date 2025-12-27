// Package device provides high-level interfaces for KASA smart devices.
package device

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/command"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/protocol"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/types"
)

// Device is the base interface all KASA devices implement.
type Device interface {
	// Connection management
	Connect(ctx context.Context) error
	Close() error
	IsConnected() bool

	// Basic info
	Host() string
	Alias() string
	Model() string
	MAC() string
	DeviceID() string
	Type() types.DeviceType

	// State
	Update(ctx context.Context) error
	IsOn() bool

	// Control
	TurnOn(ctx context.Context) error
	TurnOff(ctx context.Context) error
	SetAlias(ctx context.Context, alias string) error
	Reboot(ctx context.Context, delay time.Duration) error

	// Features
	HasEmeter() bool
	HasChildren() bool

	// Raw command access
	SendCommand(ctx context.Context, cmd interface{}) ([]byte, error)

	// SysInfo returns the cached system info
	SysInfo() *types.SysInfo
}

// EmeterDevice interface for devices with energy monitoring.
type EmeterDevice interface {
	Device
	GetEmeterRealtime(ctx context.Context) (*types.EmeterData, error)
	GetEmeterDaily(ctx context.Context, year, month int) ([]types.DailyUsage, error)
	GetEmeterMonthly(ctx context.Context, year int) ([]types.MonthlyUsage, error)
}

// Dimmable interface for devices that support brightness control.
type Dimmable interface {
	Device
	Brightness() int
	SetBrightness(ctx context.Context, brightness int) error
	IsDimmable() bool
}

// Colorable interface for devices that support color control.
type Colorable interface {
	Dimmable
	HSV() (hue, saturation, brightness int)
	SetHSV(ctx context.Context, hue, saturation, brightness int) error
	IsColor() bool
}

// TemperatureControllable interface for color temperature control.
type TemperatureControllable interface {
	Dimmable
	ColorTemp() int
	ColorTempRange() (min, max int)
	SetColorTemp(ctx context.Context, temp int) error
	IsVariableColorTemp() bool
}

// ParentDevice interface for devices with child outlets.
type ParentDevice interface {
	Device
	Children() []Device
	GetChild(index int) (Device, error)
	ChildCount() int
}

// BaseDevice provides common functionality for all device types.
type BaseDevice struct {
	mu        sync.RWMutex
	transport *protocol.Transport
	sysinfo   *types.SysInfo
	devType   types.DeviceType
}

// NewBaseDevice creates a new base device.
func NewBaseDevice(host string, opts ...protocol.TransportOption) *BaseDevice {
	return &BaseDevice{
		transport: protocol.NewTransport(host, opts...),
	}
}

// Connect establishes connection and fetches sysinfo.
func (d *BaseDevice) Connect(ctx context.Context) error {
	if err := d.transport.Connect(ctx); err != nil {
		return err
	}
	return d.Update(ctx)
}

// Close closes the connection.
func (d *BaseDevice) Close() error {
	return d.transport.Close()
}

// IsConnected returns true if connected.
func (d *BaseDevice) IsConnected() bool {
	return d.transport.IsConnected()
}

// Host returns the device IP address.
func (d *BaseDevice) Host() string {
	return d.transport.Host()
}

// Update refreshes device state from hardware.
func (d *BaseDevice) Update(ctx context.Context) error {
	resp, err := d.SendCommand(ctx, command.GetSysInfo())
	if err != nil {
		return err
	}

	var result types.SysInfoResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse sysinfo: %w", err)
	}

	d.mu.Lock()
	d.sysinfo = &result.System.GetSysinfo
	d.devType = d.sysinfo.DetectDeviceType()
	d.mu.Unlock()

	return nil
}

// SendCommand sends a command and returns raw response.
func (d *BaseDevice) SendCommand(ctx context.Context, cmd interface{}) ([]byte, error) {
	return d.transport.Send(ctx, cmd)
}

// SysInfo returns the cached system info.
func (d *BaseDevice) SysInfo() *types.SysInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.sysinfo
}

// Alias returns device alias.
func (d *BaseDevice) Alias() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.sysinfo == nil {
		return ""
	}
	return d.sysinfo.Alias
}

// Model returns device model.
func (d *BaseDevice) Model() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.sysinfo == nil {
		return ""
	}
	return d.sysinfo.Model
}

// MAC returns device MAC address.
func (d *BaseDevice) MAC() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.sysinfo == nil {
		return ""
	}
	return d.sysinfo.MAC
}

// DeviceID returns device ID.
func (d *BaseDevice) DeviceID() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.sysinfo == nil {
		return ""
	}
	return d.sysinfo.DeviceID
}

// Type returns the device type.
func (d *BaseDevice) Type() types.DeviceType {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.devType
}

// HasEmeter returns true if device has energy monitoring.
func (d *BaseDevice) HasEmeter() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.sysinfo != nil && d.sysinfo.HasEmeter()
}

// HasChildren returns true if device has child outlets.
func (d *BaseDevice) HasChildren() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.sysinfo != nil && d.sysinfo.HasChildren()
}

// SetAlias sets the device alias.
func (d *BaseDevice) SetAlias(ctx context.Context, alias string) error {
	_, err := d.SendCommand(ctx, command.SetAlias(alias))
	if err != nil {
		return err
	}
	d.mu.Lock()
	if d.sysinfo != nil {
		d.sysinfo.Alias = alias
	}
	d.mu.Unlock()
	return nil
}

// Reboot reboots the device with optional delay.
func (d *BaseDevice) Reboot(ctx context.Context, delay time.Duration) error {
	_, err := d.SendCommand(ctx, command.Reboot(int(delay.Seconds())))
	return err
}
