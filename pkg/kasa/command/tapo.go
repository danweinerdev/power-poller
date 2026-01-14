package command

// TapoCommand represents a TAPO-style command.
// TAPO uses a method-based API: {"method": "get_device_info", "params": {...}}
type TapoCommand struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// NewTapoCommand creates a new TAPO command.
func NewTapoCommand(method string, params map[string]interface{}) *TapoCommand {
	return &TapoCommand{
		Method: method,
		Params: params,
	}
}

// TapoGetDeviceInfo returns the TAPO get_device_info command.
func TapoGetDeviceInfo() *TapoCommand {
	return NewTapoCommand("get_device_info", nil)
}

// TapoGetEnergyUsage returns the TAPO get_energy_usage command.
func TapoGetEnergyUsage() *TapoCommand {
	return NewTapoCommand("get_energy_usage", nil)
}

// TapoGetCurrentPower returns the TAPO get_current_power command.
func TapoGetCurrentPower() *TapoCommand {
	return NewTapoCommand("get_current_power", nil)
}

// TapoGetEmeterData returns the TAPO get_emeter_data command.
// This provides full emeter data including voltage, current, and power.
func TapoGetEmeterData() *TapoCommand {
	return NewTapoCommand("get_emeter_data", nil)
}

// TapoSetDeviceInfo sets device info (including on/off state).
func TapoSetDeviceInfo(deviceOn bool) *TapoCommand {
	return NewTapoCommand("set_device_info", map[string]interface{}{
		"device_on": deviceOn,
	})
}

// TapoSetBrightness sets the brightness for a dimmable device.
func TapoSetBrightness(brightness int) *TapoCommand {
	return NewTapoCommand("set_device_info", map[string]interface{}{
		"brightness": brightness,
	})
}

// TapoGetDeviceUsage returns the TAPO get_device_usage command (runtime stats).
func TapoGetDeviceUsage() *TapoCommand {
	return NewTapoCommand("get_device_usage", nil)
}

// TapoComponentNego returns the component negotiation command.
// This is used to discover what features the device supports.
func TapoComponentNego() *TapoCommand {
	return NewTapoCommand("component_nego", nil)
}
