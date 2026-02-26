package device

import (
	"context"

	"github.com/danweinerdev/power-poller/pkg/kasa/command"
	"github.com/danweinerdev/power-poller/pkg/kasa/protocol"
	"github.com/danweinerdev/power-poller/pkg/kasa/types"
)

// LightStrip represents a KASA light strip.
// It extends Bulb with light strip-specific namespaces.
type LightStrip struct {
	*Bulb
}

// NewLightStrip creates a new LightStrip instance with legacy TCP transport.
func NewLightStrip(host string, opts ...protocol.TransportOption) *LightStrip {
	return &LightStrip{
		Bulb: NewBulb(host, opts...),
	}
}

// NewLightStripWithTransport creates a new LightStrip with a specific transport.
func NewLightStripWithTransport(transport protocol.Transporter) *LightStrip {
	return &LightStrip{
		Bulb: NewBulbWithTransport(transport),
	}
}

// Type returns TypeLightStrip.
func (l *LightStrip) Type() types.DeviceType {
	return types.DeviceTypeLightStrip
}

// TurnOn turns on the light strip.
func (l *LightStrip) TurnOn(ctx context.Context) error {
	_, err := l.SendCommand(ctx, command.SetLightStateLightStrip(true))
	if err != nil {
		return err
	}
	l.mu.Lock()
	if l.sysinfo != nil && l.sysinfo.LightState != nil {
		l.sysinfo.LightState.OnOff = 1
	}
	l.mu.Unlock()
	return nil
}

// TurnOff turns off the light strip.
func (l *LightStrip) TurnOff(ctx context.Context) error {
	_, err := l.SendCommand(ctx, command.SetLightStateLightStrip(false))
	if err != nil {
		return err
	}
	l.mu.Lock()
	if l.sysinfo != nil && l.sysinfo.LightState != nil {
		l.sysinfo.LightState.OnOff = 0
	}
	l.mu.Unlock()
	return nil
}

// SetBrightness sets brightness using light strip namespace.
func (l *LightStrip) SetBrightness(ctx context.Context, brightness int) error {
	if brightness < 0 || brightness > 100 {
		return ErrInvalidBrightness
	}
	_, err := l.SendCommand(ctx, command.SetBrightnessLightStrip(brightness))
	if err != nil {
		return err
	}
	l.mu.Lock()
	if l.sysinfo != nil && l.sysinfo.LightState != nil {
		l.sysinfo.LightState.Brightness = brightness
		l.sysinfo.LightState.OnOff = 1
	}
	l.mu.Unlock()
	return nil
}

// SetHSV sets color using light strip namespace.
func (l *LightStrip) SetHSV(ctx context.Context, hue, saturation, brightness int) error {
	if hue < 0 || hue > 360 {
		return ErrInvalidHue
	}
	if saturation < 0 || saturation > 100 {
		return ErrInvalidSaturation
	}
	if brightness < 0 || brightness > 100 {
		return ErrInvalidBrightness
	}
	_, err := l.SendCommand(ctx, command.SetHSVLightStrip(hue, saturation, brightness))
	if err != nil {
		return err
	}
	l.mu.Lock()
	if l.sysinfo != nil && l.sysinfo.LightState != nil {
		l.sysinfo.LightState.Hue = hue
		l.sysinfo.LightState.Saturation = saturation
		l.sysinfo.LightState.Brightness = brightness
		l.sysinfo.LightState.OnOff = 1
		l.sysinfo.LightState.ColorTemp = 0
	}
	l.mu.Unlock()
	return nil
}

// SetColorTemp sets color temperature using light strip namespace.
func (l *LightStrip) SetColorTemp(ctx context.Context, temp int) error {
	min, max := l.ColorTempRange()
	if temp < min || temp > max {
		return ErrInvalidColorTemp
	}
	_, err := l.SendCommand(ctx, command.SetColorTempLightStrip(temp))
	if err != nil {
		return err
	}
	l.mu.Lock()
	if l.sysinfo != nil && l.sysinfo.LightState != nil {
		l.sysinfo.LightState.ColorTemp = temp
		l.sysinfo.LightState.OnOff = 1
	}
	l.mu.Unlock()
	return nil
}

// Length returns the number of LEDs in the strip.
func (l *LightStrip) Length() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.sysinfo == nil {
		return 0
	}
	return l.sysinfo.Length
}

// Ensure LightStrip implements the interfaces.
var (
	_ Device                  = (*LightStrip)(nil)
	_ EmeterDevice            = (*LightStrip)(nil)
	_ Dimmable                = (*LightStrip)(nil)
	_ Colorable               = (*LightStrip)(nil)
	_ TemperatureControllable = (*LightStrip)(nil)
)
