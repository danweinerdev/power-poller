package mockdevice

import (
	"fmt"
	"time"
)

// WithDeviceType sets the device type and applies appropriate defaults.
func WithDeviceType(dt DeviceType) Option {
	return func(d *MockDevice) {
		d.DeviceType = dt
		switch dt {
		case DeviceTypePlug:
			d.Model = "HS110(US)"
			d.Capabilities.HasEmeter = true
		case DeviceTypeBulb:
			d.Model = "LB130(US)"
			d.Capabilities.HasEmeter = true
			d.Capabilities.HasDimmer = true
			d.Capabilities.HasColor = true
			d.Capabilities.HasColorTemp = true
		case DeviceTypeLightStrip:
			d.Model = "KL430(US)"
			d.Capabilities.HasEmeter = true
			d.Capabilities.HasDimmer = true
			d.Capabilities.HasColor = true
			d.Capabilities.HasColorTemp = true
		case DeviceTypePowerStrip:
			d.Model = "HS300(US)"
			d.Capabilities.HasEmeter = true
			d.Capabilities.HasChildren = true
			d.Capabilities.ChildCount = 6
		}
	}
}

// WithEmeter enables or disables emeter support.
func WithEmeter(enabled bool) Option {
	return func(d *MockDevice) {
		d.Capabilities.HasEmeter = enabled
	}
}

// WithAlias sets the device alias.
func WithAlias(alias string) Option {
	return func(d *MockDevice) {
		d.Alias = alias
	}
}

// WithModel sets the device model.
func WithModel(model string) Option {
	return func(d *MockDevice) {
		d.Model = model
	}
}

// WithMAC sets the device MAC address.
func WithMAC(mac string) Option {
	return func(d *MockDevice) {
		d.MAC = mac
	}
}

// WithDeviceID sets the device ID.
func WithDeviceID(id string) Option {
	return func(d *MockDevice) {
		d.DeviceID = id
	}
}

// WithChildren configures child devices for power strips.
func WithChildren(count int) Option {
	return func(d *MockDevice) {
		d.Capabilities.HasChildren = true
		d.Capabilities.ChildCount = count
		d.Children = make([]*ChildDevice, count)
		for i := 0; i < count; i++ {
			d.Children[i] = &ChildDevice{
				ID:      fmt.Sprintf("8006%02dCHILD", i),
				Alias:   fmt.Sprintf("Outlet %d", i+1),
				IsOn:    true,
				Voltage: 120.0 + float64(i)*0.1,
				Current: 0.1 * float64(i+1),
				Power:   12.0 * float64(i+1),
				TotalWh: 100.0 * float64(i+1),
			}
		}
	}
}

// WithEmeterValues sets specific emeter values.
func WithEmeterValues(voltage, current, power, total float64) Option {
	return func(d *MockDevice) {
		d.Voltage = voltage
		d.Current = current
		d.Power = power
		d.TotalWh = total
	}
}

// WithState sets the initial on/off state.
func WithState(on bool) Option {
	return func(d *MockDevice) {
		d.IsOn = on
	}
}

// WithBrightness sets the initial brightness (0-100).
func WithBrightness(brightness int) Option {
	return func(d *MockDevice) {
		d.Brightness = brightness
	}
}

// WithHSV sets the initial HSV color values.
func WithHSV(hue, saturation, brightness int) Option {
	return func(d *MockDevice) {
		d.Hue = hue
		d.Saturation = saturation
		d.Brightness = brightness
	}
}

// WithColorTemp sets the initial color temperature.
func WithColorTemp(temp int) Option {
	return func(d *MockDevice) {
		d.ColorTemp = temp
	}
}

// WithErrorBehavior sets error injection behavior.
func WithErrorBehavior(behavior ErrorBehavior) Option {
	return func(d *MockDevice) {
		d.ErrorBehavior = behavior
	}
}

// WithResponseDelay sets a delay before responding.
func WithResponseDelay(delay time.Duration) Option {
	return func(d *MockDevice) {
		d.ErrorBehavior.ResponseDelay = delay
	}
}

// WithInvalidResponse makes the mock return invalid JSON.
func WithInvalidResponse() Option {
	return func(d *MockDevice) {
		d.ErrorBehavior.InvalidResponse = true
	}
}

// WithDropConnection makes the mock close the connection after receiving a command.
func WithDropConnection() Option {
	return func(d *MockDevice) {
		d.ErrorBehavior.DropConnection = true
	}
}

// WithDisconnectAfter makes the mock disconnect after N commands.
func WithDisconnectAfter(n int) Option {
	return func(d *MockDevice) {
		d.ErrorBehavior.DisconnectAfterCommands = n
	}
}

// WithPartialResponse makes the mock send incomplete responses.
func WithPartialResponse() Option {
	return func(d *MockDevice) {
		d.ErrorBehavior.PartialResponse = true
	}
}

// WithDimmer enables dimmer capability.
func WithDimmer(enabled bool) Option {
	return func(d *MockDevice) {
		d.Capabilities.HasDimmer = enabled
	}
}

// WithColor enables color capability.
func WithColor(enabled bool) Option {
	return func(d *MockDevice) {
		d.Capabilities.HasColor = enabled
	}
}

// WithColorTemp enables color temperature capability.
func WithColorTempCapability(enabled bool) Option {
	return func(d *MockDevice) {
		d.Capabilities.HasColorTemp = enabled
	}
}

// WithProtocol sets the communication protocol for the mock device.
func WithProtocol(proto ProtocolType) Option {
	return func(d *MockDevice) {
		d.Protocol = proto
		// Update model for TAPO devices
		if proto == ProtocolSecurePassthrough {
			switch d.DeviceType {
			case DeviceTypePlug:
				d.Model = "EP25"
			case DeviceTypeBulb:
				d.Model = "L510E"
			case DeviceTypeLightStrip:
				d.Model = "L900"
			case DeviceTypePowerStrip:
				d.Model = "P300"
			}
		}
	}
}
