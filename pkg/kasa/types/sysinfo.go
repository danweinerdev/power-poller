// Package types defines the data structures for KASA device responses.
package types

import "strings"

// DeviceType represents the type of KASA device.
type DeviceType int

const (
	DeviceTypeUnknown DeviceType = iota
	DeviceTypePlug
	DeviceTypeBulb
	DeviceTypeLightStrip
	DeviceTypePowerStrip
)

// String returns the device type name.
func (d DeviceType) String() string {
	switch d {
	case DeviceTypePlug:
		return "Plug"
	case DeviceTypeBulb:
		return "Bulb"
	case DeviceTypeLightStrip:
		return "LightStrip"
	case DeviceTypePowerStrip:
		return "PowerStrip"
	default:
		return "Unknown"
	}
}

// SysInfo represents the device system information response.
type SysInfo struct {
	// Common fields
	Alias      string `json:"alias"`
	DeviceID   string `json:"deviceId"`
	Model      string `json:"model"`
	MAC        string `json:"mac"`
	HWID       string `json:"hwId"`
	FirmwareID string `json:"fwId"`
	OEMID      string `json:"oemId"`
	HWVersion  string `json:"hw_ver"`
	SWVersion  string `json:"sw_ver"`
	Type       string `json:"type"`
	MicType    string `json:"mic_type"`
	DevName    string `json:"dev_name"`

	// Network
	SSID string `json:"ssid"`
	RSSI int    `json:"rssi"`

	// Feature flags (colon-separated: "TIM:ENE")
	Feature string `json:"feature"`

	// Plug-specific
	RelayState int `json:"relay_state"`
	OnTime     int `json:"on_time"`
	LEDOff     int `json:"led_off"`

	// Bulb-specific
	LightState *LightState `json:"light_state,omitempty"`
	IsDimmable int         `json:"is_dimmable"`
	IsColor    int         `json:"is_color"`
	IsVariable int         `json:"is_variable_color_temp"`

	// Light strip specific
	Length int `json:"length,omitempty"`

	// Power strip (children)
	Children []ChildInfo `json:"children,omitempty"`
	ChildNum int         `json:"child_num,omitempty"`

	// Error handling
	ErrCode int    `json:"err_code"`
	ErrMsg  string `json:"err_msg,omitempty"`
}

// ChildInfo represents a child device (outlet) in a power strip.
type ChildInfo struct {
	ID     string `json:"id"`
	State  int    `json:"state"`
	Alias  string `json:"alias"`
	OnTime int    `json:"on_time"`
}

// LightState represents bulb lighting state.
type LightState struct {
	OnOff      int    `json:"on_off"`
	Mode       string `json:"mode"`
	Hue        int    `json:"hue"`
	Saturation int    `json:"saturation"`
	ColorTemp  int    `json:"color_temp"`
	Brightness int    `json:"brightness"`
}

// HasFeature checks if a feature flag is present.
func (s *SysInfo) HasFeature(feature string) bool {
	features := strings.Split(s.Feature, ":")
	for _, f := range features {
		if f == feature {
			return true
		}
	}
	return false
}

// HasEmeter returns true if device has energy monitoring.
func (s *SysInfo) HasEmeter() bool {
	return s.HasFeature("ENE")
}

// IsOn returns true if the device is on.
func (s *SysInfo) IsOn() bool {
	// For plugs, check relay_state
	if s.RelayState != 0 {
		return true
	}
	// For bulbs, check light_state
	if s.LightState != nil && s.LightState.OnOff != 0 {
		return true
	}
	return false
}

// HasChildren returns true if the device has child outlets.
func (s *SysInfo) HasChildren() bool {
	return s.ChildNum > 0 || len(s.Children) > 0
}

// DetectDeviceType determines the device type from sysinfo.
func (s *SysInfo) DetectDeviceType() DeviceType {
	// Check for children (power strip)
	if s.HasChildren() {
		return DeviceTypePowerStrip
	}

	typeLower := strings.ToLower(s.Type)

	// Bulb detection
	if strings.Contains(typeLower, "smartbulb") || strings.Contains(typeLower, "bulb") {
		// Light strips have a length parameter
		if s.Length > 0 {
			return DeviceTypeLightStrip
		}
		return DeviceTypeBulb
	}

	// Plug detection
	if strings.Contains(typeLower, "smartplug") || strings.Contains(typeLower, "plug") {
		return DeviceTypePlug
	}

	// Check mic_type for additional hints
	micType := strings.ToLower(s.MicType)
	if strings.Contains(micType, "bulb") {
		return DeviceTypeBulb
	}
	if strings.Contains(micType, "plug") {
		return DeviceTypePlug
	}

	return DeviceTypeUnknown
}

// SysInfoResponse wraps the get_sysinfo response.
type SysInfoResponse struct {
	System struct {
		GetSysinfo SysInfo `json:"get_sysinfo"`
	} `json:"system"`
}
