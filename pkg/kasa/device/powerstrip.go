package device

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/command"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/protocol"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/types"
)

// PowerStrip represents a KASA smart power strip with multiple outlets.
type PowerStrip struct {
	*BaseDevice
	children []*ChildOutlet
}

// ChildOutlet represents a single outlet on a power strip.
type ChildOutlet struct {
	parent *PowerStrip
	id     string
	index  int
	alias  string
	isOn   bool
}

// NewPowerStrip creates a new PowerStrip instance.
func NewPowerStrip(host string, opts ...protocol.TransportOption) *PowerStrip {
	return &PowerStrip{
		BaseDevice: NewBaseDevice(host, opts...),
	}
}

// Connect establishes connection and fetches sysinfo including children.
func (p *PowerStrip) Connect(ctx context.Context) error {
	if err := p.transport.Connect(ctx); err != nil {
		return err
	}
	return p.Update(ctx)
}

// Type returns TypePowerStrip.
func (p *PowerStrip) Type() types.DeviceType {
	return types.DeviceTypePowerStrip
}

// Update refreshes device state and child outlet info.
func (p *PowerStrip) Update(ctx context.Context) error {
	if err := p.BaseDevice.Update(ctx); err != nil {
		return err
	}

	// Update children from sysinfo
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sysinfo == nil {
		return nil
	}

	p.children = make([]*ChildOutlet, len(p.sysinfo.Children))
	for i, child := range p.sysinfo.Children {
		p.children[i] = &ChildOutlet{
			parent: p,
			id:     child.ID,
			index:  i,
			alias:  child.Alias,
			isOn:   child.State == 1,
		}
	}

	return nil
}

// IsOn returns true if any outlet is on.
func (p *PowerStrip) IsOn() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, child := range p.children {
		if child.isOn {
			return true
		}
	}
	return false
}

// TurnOn turns on all outlets.
func (p *PowerStrip) TurnOn(ctx context.Context) error {
	_, err := p.SendCommand(ctx, command.SetRelayState(true))
	if err != nil {
		return err
	}
	p.mu.Lock()
	for _, child := range p.children {
		child.isOn = true
	}
	p.mu.Unlock()
	return nil
}

// TurnOff turns off all outlets.
func (p *PowerStrip) TurnOff(ctx context.Context) error {
	_, err := p.SendCommand(ctx, command.SetRelayState(false))
	if err != nil {
		return err
	}
	p.mu.Lock()
	for _, child := range p.children {
		child.isOn = false
	}
	p.mu.Unlock()
	return nil
}

// GetEmeterRealtime returns aggregate energy data for the power strip.
func (p *PowerStrip) GetEmeterRealtime(ctx context.Context) (*types.EmeterData, error) {
	if !p.HasEmeter() {
		return nil, ErrNoEmeter
	}

	resp, err := p.SendCommand(ctx, command.GetEmeterRealtime())
	if err != nil {
		return nil, err
	}

	var result types.EmeterRealtimeResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse emeter: %w", err)
	}

	normalized := result.Emeter.GetRealtime.Normalize()
	return &normalized, nil
}

// GetEmeterDaily returns daily energy stats.
func (p *PowerStrip) GetEmeterDaily(ctx context.Context, year, month int) ([]types.DailyUsage, error) {
	if !p.HasEmeter() {
		return nil, ErrNoEmeter
	}

	resp, err := p.SendCommand(ctx, command.GetEmeterDaily(year, month))
	if err != nil {
		return nil, err
	}

	var result types.EmeterDayStatResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse daystat: %w", err)
	}

	return result.Emeter.GetDayStat.DayList, nil
}

// GetEmeterMonthly returns monthly energy stats.
func (p *PowerStrip) GetEmeterMonthly(ctx context.Context, year int) ([]types.MonthlyUsage, error) {
	if !p.HasEmeter() {
		return nil, ErrNoEmeter
	}

	resp, err := p.SendCommand(ctx, command.GetEmeterMonthly(year))
	if err != nil {
		return nil, err
	}

	var result types.EmeterMonthStatResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse monthstat: %w", err)
	}

	return result.Emeter.GetMonthStat.MonthList, nil
}

// ChildCount returns the number of child outlets.
func (p *PowerStrip) ChildCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.children)
}

// Children returns all child outlets.
func (p *PowerStrip) Children() []Device {
	p.mu.RLock()
	defer p.mu.RUnlock()
	devices := make([]Device, len(p.children))
	for i, child := range p.children {
		devices[i] = child
	}
	return devices
}

// GetChild returns a child outlet by index.
func (p *PowerStrip) GetChild(index int) (Device, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if index < 0 || index >= len(p.children) {
		return nil, ErrChildNotFound
	}
	return p.children[index], nil
}

// GetChildByID returns a child outlet by ID.
func (p *PowerStrip) GetChildByID(id string) (*ChildOutlet, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, child := range p.children {
		if child.id == id {
			return child, nil
		}
	}
	return nil, ErrChildNotFound
}

// ChildOutlet methods

// Connect is a no-op for child outlets (they share parent connection).
func (c *ChildOutlet) Connect(ctx context.Context) error {
	return nil
}

// Close is a no-op for child outlets.
func (c *ChildOutlet) Close() error {
	return nil
}

// IsConnected returns true if parent is connected.
func (c *ChildOutlet) IsConnected() bool {
	return c.parent.IsConnected()
}

// Host returns the parent's host.
func (c *ChildOutlet) Host() string {
	return c.parent.Host()
}

// Alias returns the child outlet's alias.
func (c *ChildOutlet) Alias() string {
	return c.alias
}

// Model returns the parent's model.
func (c *ChildOutlet) Model() string {
	return c.parent.Model()
}

// MAC returns the parent's MAC.
func (c *ChildOutlet) MAC() string {
	return c.parent.MAC()
}

// DeviceID returns the child's ID.
func (c *ChildOutlet) DeviceID() string {
	return c.id
}

// Type returns TypePlug (children behave like plugs).
func (c *ChildOutlet) Type() types.DeviceType {
	return types.DeviceTypePlug
}

// Update refreshes the child's state from parent.
func (c *ChildOutlet) Update(ctx context.Context) error {
	return c.parent.Update(ctx)
}

// IsOn returns true if the outlet is on.
func (c *ChildOutlet) IsOn() bool {
	return c.isOn
}

// TurnOn turns on the outlet.
func (c *ChildOutlet) TurnOn(ctx context.Context) error {
	_, err := c.parent.SendCommand(ctx, command.SetRelayStateForChild(c.id, true))
	if err != nil {
		return err
	}
	c.isOn = true
	return nil
}

// TurnOff turns off the outlet.
func (c *ChildOutlet) TurnOff(ctx context.Context) error {
	_, err := c.parent.SendCommand(ctx, command.SetRelayStateForChild(c.id, false))
	if err != nil {
		return err
	}
	c.isOn = false
	return nil
}

// SetAlias sets the outlet's alias.
func (c *ChildOutlet) SetAlias(ctx context.Context, alias string) error {
	// Child alias setting would need a specific command
	// For now, return not supported
	return ErrNotSupported
}

// Reboot is not supported for child outlets.
func (c *ChildOutlet) Reboot(ctx context.Context, delay time.Duration) error {
	return ErrNotSupported
}

// HasEmeter returns true if the child supports energy monitoring.
func (c *ChildOutlet) HasEmeter() bool {
	return c.parent.HasEmeter()
}

// HasChildren returns false (children don't have children).
func (c *ChildOutlet) HasChildren() bool {
	return false
}

// SendCommand sends a command through the parent.
func (c *ChildOutlet) SendCommand(ctx context.Context, cmd interface{}) ([]byte, error) {
	return c.parent.SendCommand(ctx, cmd)
}

// SysInfo returns the parent's sysinfo.
func (c *ChildOutlet) SysInfo() *types.SysInfo {
	return c.parent.SysInfo()
}

// GetEmeterRealtime returns energy data for this specific outlet.
func (c *ChildOutlet) GetEmeterRealtime(ctx context.Context) (*types.EmeterData, error) {
	if !c.HasEmeter() {
		return nil, ErrNoEmeter
	}

	resp, err := c.parent.SendCommand(ctx, command.GetEmeterRealtimeForChild(c.id))
	if err != nil {
		return nil, err
	}

	var result types.EmeterRealtimeResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse emeter: %w", err)
	}

	normalized := result.Emeter.GetRealtime.Normalize()
	return &normalized, nil
}

// GetEmeterDaily returns daily energy stats for this outlet.
func (c *ChildOutlet) GetEmeterDaily(ctx context.Context, year, month int) ([]types.DailyUsage, error) {
	// Would need child-specific command
	return nil, ErrNotSupported
}

// GetEmeterMonthly returns monthly energy stats for this outlet.
func (c *ChildOutlet) GetEmeterMonthly(ctx context.Context, year int) ([]types.MonthlyUsage, error) {
	// Would need child-specific command
	return nil, ErrNotSupported
}

// Index returns the outlet's index in the power strip.
func (c *ChildOutlet) Index() int {
	return c.index
}

// ParentID returns the parent device ID.
func (c *ChildOutlet) ParentID() string {
	return c.parent.DeviceID()
}

// Ensure interfaces are implemented.
var (
	_ Device       = (*PowerStrip)(nil)
	_ EmeterDevice = (*PowerStrip)(nil)
	_ ParentDevice = (*PowerStrip)(nil)
	_ Device       = (*ChildOutlet)(nil)
	_ EmeterDevice = (*ChildOutlet)(nil)
)
