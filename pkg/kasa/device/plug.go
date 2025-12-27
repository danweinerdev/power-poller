package device

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/command"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/protocol"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/types"
)

// Plug represents a KASA smart plug.
type Plug struct {
	*BaseDevice
}

// NewPlug creates a new Plug instance.
func NewPlug(host string, opts ...protocol.TransportOption) *Plug {
	return &Plug{
		BaseDevice: NewBaseDevice(host, opts...),
	}
}

// Type returns TypePlug.
func (p *Plug) Type() types.DeviceType {
	return types.DeviceTypePlug
}

// IsOn returns true if plug relay is on.
func (p *Plug) IsOn() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sysinfo != nil && p.sysinfo.RelayState == 1
}

// TurnOn turns on the plug.
func (p *Plug) TurnOn(ctx context.Context) error {
	_, err := p.SendCommand(ctx, command.SetRelayState(true))
	if err != nil {
		return err
	}
	p.mu.Lock()
	if p.sysinfo != nil {
		p.sysinfo.RelayState = 1
	}
	p.mu.Unlock()
	return nil
}

// TurnOff turns off the plug.
func (p *Plug) TurnOff(ctx context.Context) error {
	_, err := p.SendCommand(ctx, command.SetRelayState(false))
	if err != nil {
		return err
	}
	p.mu.Lock()
	if p.sysinfo != nil {
		p.sysinfo.RelayState = 0
	}
	p.mu.Unlock()
	return nil
}

// GetEmeterRealtime returns real-time energy data.
func (p *Plug) GetEmeterRealtime(ctx context.Context) (*types.EmeterData, error) {
	if !p.HasEmeter() {
		return nil, fmt.Errorf("device does not support energy monitoring")
	}

	resp, err := p.SendCommand(ctx, command.GetEmeterRealtime())
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

// GetEmeterDaily returns daily energy stats for a given month.
func (p *Plug) GetEmeterDaily(ctx context.Context, year, month int) ([]types.DailyUsage, error) {
	if !p.HasEmeter() {
		return nil, fmt.Errorf("device does not support energy monitoring")
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

// GetEmeterMonthly returns monthly energy stats for a given year.
func (p *Plug) GetEmeterMonthly(ctx context.Context, year int) ([]types.MonthlyUsage, error) {
	if !p.HasEmeter() {
		return nil, fmt.Errorf("device does not support energy monitoring")
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

// IsLEDOn returns true if the plug's LED is on.
func (p *Plug) IsLEDOn() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sysinfo != nil && p.sysinfo.LEDOff == 0
}

// SetLED controls the plug's LED state.
func (p *Plug) SetLED(ctx context.Context, on bool) error {
	_, err := p.SendCommand(ctx, command.SetLEDOff(!on))
	return err
}

// Ensure Plug implements the interfaces.
var (
	_ Device      = (*Plug)(nil)
	_ EmeterDevice = (*Plug)(nil)
)
