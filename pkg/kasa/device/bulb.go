package device

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/command"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/protocol"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/types"
)

// Bulb represents a KASA smart bulb.
type Bulb struct {
	*BaseDevice
}

// NewBulb creates a new Bulb instance.
func NewBulb(host string, opts ...protocol.TransportOption) *Bulb {
	return &Bulb{
		BaseDevice: NewBaseDevice(host, opts...),
	}
}

// Type returns TypeBulb.
func (b *Bulb) Type() types.DeviceType {
	return types.DeviceTypeBulb
}

// IsOn returns true if bulb is on.
func (b *Bulb) IsOn() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.sysinfo == nil || b.sysinfo.LightState == nil {
		return false
	}
	return b.sysinfo.LightState.OnOff == 1
}

// TurnOn turns on the bulb.
func (b *Bulb) TurnOn(ctx context.Context) error {
	_, err := b.SendCommand(ctx, command.SetLightState(true))
	if err != nil {
		return err
	}
	b.mu.Lock()
	if b.sysinfo != nil && b.sysinfo.LightState != nil {
		b.sysinfo.LightState.OnOff = 1
	}
	b.mu.Unlock()
	return nil
}

// TurnOff turns off the bulb.
func (b *Bulb) TurnOff(ctx context.Context) error {
	_, err := b.SendCommand(ctx, command.SetLightState(false))
	if err != nil {
		return err
	}
	b.mu.Lock()
	if b.sysinfo != nil && b.sysinfo.LightState != nil {
		b.sysinfo.LightState.OnOff = 0
	}
	b.mu.Unlock()
	return nil
}

// HasEmeter always returns true for bulbs.
func (b *Bulb) HasEmeter() bool {
	return true
}

// GetEmeterRealtime returns real-time energy data.
func (b *Bulb) GetEmeterRealtime(ctx context.Context) (*types.EmeterData, error) {
	resp, err := b.SendCommand(ctx, command.GetEmeterRealtimeBulb())
	if err != nil {
		return nil, err
	}

	var result types.BulbEmeterRealtimeResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse emeter: %w", err)
	}

	normalized := result.Emeter.GetRealtime.Normalize()
	return &normalized, nil
}

// GetEmeterDaily returns daily energy stats for a given month.
func (b *Bulb) GetEmeterDaily(ctx context.Context, year, month int) ([]types.DailyUsage, error) {
	resp, err := b.SendCommand(ctx, command.GetEmeterDaily(year, month))
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
func (b *Bulb) GetEmeterMonthly(ctx context.Context, year int) ([]types.MonthlyUsage, error) {
	resp, err := b.SendCommand(ctx, command.GetEmeterMonthly(year))
	if err != nil {
		return nil, err
	}

	var result types.EmeterMonthStatResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse monthstat: %w", err)
	}

	return result.Emeter.GetMonthStat.MonthList, nil
}

// IsDimmable returns true if bulb supports brightness control.
func (b *Bulb) IsDimmable() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.sysinfo != nil && b.sysinfo.IsDimmable == 1
}

// Brightness returns current brightness (0-100).
func (b *Bulb) Brightness() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.sysinfo == nil || b.sysinfo.LightState == nil {
		return 0
	}
	return b.sysinfo.LightState.Brightness
}

// SetBrightness sets brightness (0-100).
func (b *Bulb) SetBrightness(ctx context.Context, brightness int) error {
	if brightness < 0 || brightness > 100 {
		return fmt.Errorf("brightness must be 0-100, got %d", brightness)
	}
	_, err := b.SendCommand(ctx, command.SetBrightness(brightness))
	if err != nil {
		return err
	}
	b.mu.Lock()
	if b.sysinfo != nil && b.sysinfo.LightState != nil {
		b.sysinfo.LightState.Brightness = brightness
		b.sysinfo.LightState.OnOff = 1
	}
	b.mu.Unlock()
	return nil
}

// IsColor returns true if bulb supports color control.
func (b *Bulb) IsColor() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.sysinfo != nil && b.sysinfo.IsColor == 1
}

// HSV returns current hue, saturation, brightness.
func (b *Bulb) HSV() (hue, saturation, brightness int) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.sysinfo == nil || b.sysinfo.LightState == nil {
		return 0, 0, 0
	}
	ls := b.sysinfo.LightState
	return ls.Hue, ls.Saturation, ls.Brightness
}

// SetHSV sets color using HSV values.
// hue: 0-360, saturation: 0-100, brightness: 0-100
func (b *Bulb) SetHSV(ctx context.Context, hue, saturation, brightness int) error {
	if hue < 0 || hue > 360 {
		return fmt.Errorf("hue must be 0-360, got %d", hue)
	}
	if saturation < 0 || saturation > 100 {
		return fmt.Errorf("saturation must be 0-100, got %d", saturation)
	}
	if brightness < 0 || brightness > 100 {
		return fmt.Errorf("brightness must be 0-100, got %d", brightness)
	}
	_, err := b.SendCommand(ctx, command.SetHSV(hue, saturation, brightness))
	if err != nil {
		return err
	}
	b.mu.Lock()
	if b.sysinfo != nil && b.sysinfo.LightState != nil {
		b.sysinfo.LightState.Hue = hue
		b.sysinfo.LightState.Saturation = saturation
		b.sysinfo.LightState.Brightness = brightness
		b.sysinfo.LightState.OnOff = 1
		b.sysinfo.LightState.ColorTemp = 0
	}
	b.mu.Unlock()
	return nil
}

// IsVariableColorTemp returns true if bulb supports color temperature.
func (b *Bulb) IsVariableColorTemp() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.sysinfo != nil && b.sysinfo.IsVariable == 1
}

// ColorTemp returns current color temperature.
func (b *Bulb) ColorTemp() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.sysinfo == nil || b.sysinfo.LightState == nil {
		return 0
	}
	return b.sysinfo.LightState.ColorTemp
}

// ColorTempRange returns the valid color temperature range for this bulb.
func (b *Bulb) ColorTempRange() (min, max int) {
	b.mu.RLock()
	model := ""
	if b.sysinfo != nil {
		model = b.sysinfo.Model
	}
	b.mu.RUnlock()

	r := types.GetColorTempRange(model)
	return r.Min, r.Max
}

// SetColorTemp sets color temperature.
func (b *Bulb) SetColorTemp(ctx context.Context, temp int) error {
	min, max := b.ColorTempRange()
	if temp < min || temp > max {
		return fmt.Errorf("color temperature must be %d-%d for this device, got %d", min, max, temp)
	}
	_, err := b.SendCommand(ctx, command.SetColorTemp(temp))
	if err != nil {
		return err
	}
	b.mu.Lock()
	if b.sysinfo != nil && b.sysinfo.LightState != nil {
		b.sysinfo.LightState.ColorTemp = temp
		b.sysinfo.LightState.OnOff = 1
	}
	b.mu.Unlock()
	return nil
}

// Ensure Bulb implements the interfaces.
var (
	_ Device                  = (*Bulb)(nil)
	_ EmeterDevice            = (*Bulb)(nil)
	_ Dimmable                = (*Bulb)(nil)
	_ Colorable               = (*Bulb)(nil)
	_ TemperatureControllable = (*Bulb)(nil)
)
