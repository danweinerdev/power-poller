package device

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danweinerdev/power-poller/pkg/kasa/command"
	"github.com/danweinerdev/power-poller/pkg/kasa/protocol"
	"github.com/danweinerdev/power-poller/pkg/kasa/types"
)

// Plug represents a KASA smart plug.
type Plug struct {
	*BaseDevice
}

// NewPlug creates a new Plug instance with legacy TCP transport.
func NewPlug(host string, opts ...protocol.TransportOption) *Plug {
	return &Plug{
		BaseDevice: NewBaseDevice(host, opts...),
	}
}

// NewPlugWithTransport creates a new Plug with a specific transport.
func NewPlugWithTransport(transport protocol.Transporter) *Plug {
	return &Plug{
		BaseDevice: NewBaseDeviceWithTransport(transport),
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
	var err error
	if p.IsTapoProtocol() {
		_, err = p.SendCommand(ctx, command.TapoSetDeviceInfo(true))
	} else {
		_, err = p.SendCommand(ctx, command.SetRelayState(true))
	}
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
	var err error
	if p.IsTapoProtocol() {
		_, err = p.SendCommand(ctx, command.TapoSetDeviceInfo(false))
	} else {
		_, err = p.SendCommand(ctx, command.SetRelayState(false))
	}
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

	if p.IsTapoProtocol() {
		return p.getEmeterRealtimeTapo(ctx)
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

// getEmeterRealtimeTapo returns energy data using TAPO commands.
// It tries get_emeter_data first, then falls back to get_current_power.
func (p *Plug) getEmeterRealtimeTapo(ctx context.Context) (*types.EmeterData, error) {
	// Try get_emeter_data first (provides full data: voltage, current, power, energy)
	resp, err := p.SendCommand(ctx, command.TapoGetEmeterData())
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

	// Fall back to get_current_power (only provides power in mW)
	resp, err = p.SendCommand(ctx, command.TapoGetCurrentPower())
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
