package types

import (
	"encoding/base64"
)

// TapoDeviceInfo represents the TAPO device_info response.
type TapoDeviceInfo struct {
	DeviceID              string `json:"device_id"`
	FWVersion             string `json:"fw_ver"`
	HWVersion             string `json:"hw_ver"`
	Type                  string `json:"type"`
	Model                 string `json:"model"`
	MAC                   string `json:"mac"`
	HWID                  string `json:"hw_id"`
	FWID                  string `json:"fw_id"`
	OEMID                 string `json:"oem_id"`
	IP                    string `json:"ip"`
	SSID                  string `json:"ssid"` // base64 encoded
	RSSI                  int    `json:"rssi"`
	SignalLevel           int    `json:"signal_level"`
	Nickname              string `json:"nickname"` // base64 encoded
	DeviceOn              bool   `json:"device_on"`
	OnTime                int    `json:"on_time"`
	Overheated            bool   `json:"overheated"`
	PowerProtectionStatus string `json:"power_protection_status"`
	OvercurrentStatus     string `json:"overcurrent_status"`
	
	// Additional fields
	AutoOffStatus      string `json:"auto_off_status"`
	AutoOffRemainTime  int    `json:"auto_off_remain_time"`
	HasSetLocationInfo bool   `json:"has_set_location_info"`
	Latitude           int    `json:"latitude"`
	Longitude          int    `json:"longitude"`
	Lang               string `json:"lang"`
	Avatar             string `json:"avatar"`
	Region             string `json:"region"`
	Specs              string `json:"specs"`
	
	// Default states
	DefaultStates struct {
		Type  string                 `json:"type"`
		State map[string]interface{} `json:"state"`
	} `json:"default_states"`
}

// TapoDeviceInfoResponse wraps the get_device_info response.
type TapoDeviceInfoResponse struct {
	Result    TapoDeviceInfo `json:"result"`
	ErrorCode int            `json:"error_code"`
}

// ToSysInfo converts a TapoDeviceInfo to the standard SysInfo format.
func (t *TapoDeviceInfo) ToSysInfo() *SysInfo {
	// Decode base64 SSID if present
	ssid := t.SSID
	if decoded, err := base64.StdEncoding.DecodeString(t.SSID); err == nil {
		ssid = string(decoded)
	}

	// Decode base64 nickname if present
	alias := t.Nickname
	if decoded, err := base64.StdEncoding.DecodeString(t.Nickname); err == nil && len(decoded) > 0 {
		alias = string(decoded)
	}

	// Convert bool to int for relay state
	relayState := 0
	if t.DeviceOn {
		relayState = 1
	}

	return &SysInfo{
		Alias:      alias,
		DeviceID:   t.DeviceID,
		Model:      t.Model,
		MAC:        t.MAC,
		HWID:       t.HWID,
		FirmwareID: t.FWID,
		OEMID:      t.OEMID,
		HWVersion:  t.HWVersion,
		SWVersion:  t.FWVersion,
		Type:       t.Type,
		SSID:       ssid,
		RSSI:       t.RSSI,
		RelayState: relayState,
		OnTime:     t.OnTime,
		// TAPO devices with energy monitoring have "ENE" feature
		Feature: "ENE",
	}
}

// TapoEnergyUsage represents the TAPO get_energy_usage response.
type TapoEnergyUsage struct {
	TodayRuntime     int `json:"today_runtime"`
	MonthRuntime     int `json:"month_runtime"`
	TodayEnergy      int `json:"today_energy"`      // Wh
	MonthEnergy      int `json:"month_energy"`      // Wh
	LocalTime        string `json:"local_time"`
	ElectricityCharge []int `json:"electricity_charge"` // Three values
	CurrentPower     int    `json:"current_power"`      // mW
}

// TapoEnergyUsageResponse wraps the get_energy_usage response.
type TapoEnergyUsageResponse struct {
	Result    TapoEnergyUsage `json:"result"`
	ErrorCode int             `json:"error_code"`
}

// ToEmeterData converts TapoEnergyUsage to the standard EmeterData format.
func (t *TapoEnergyUsage) ToEmeterData() *EmeterData {
	return &EmeterData{
		// CurrentPower is in mW, convert to W
		Power:   float64(t.CurrentPower) / 1000.0,
		// TodayEnergy is in Wh, convert to kWh for Total
		Total:   float64(t.TodayEnergy) / 1000.0,
		// TAPO doesn't provide voltage/current directly
		Voltage: 0,
		Current: 0,
	}
}

// TapoCurrentPower represents the TAPO get_current_power response.
type TapoCurrentPower struct {
	CurrentPower int `json:"current_power"` // mW
}

// TapoCurrentPowerResponse wraps the get_current_power response.
type TapoCurrentPowerResponse struct {
	Result    TapoCurrentPower `json:"result"`
	ErrorCode int              `json:"error_code"`
}

// ToEmeterData converts TapoCurrentPower to the standard EmeterData format.
func (t *TapoCurrentPower) ToEmeterData() *EmeterData {
	return &EmeterData{
		// CurrentPower is in mW, convert to W
		Power:   float64(t.CurrentPower) / 1000.0,
		Voltage: 0,
		Current: 0,
		Total:   0,
	}
}

// TapoEmeterData represents the TAPO get_emeter_data response.
// This provides full emeter data including voltage, current, and power.
type TapoEmeterData struct {
	CurrentMA int `json:"current_ma"` // milliamps
	VoltageMA int `json:"voltage_mv"` // millivolts
	PowerMW   int `json:"power_mw"`   // milliwatts
	EnergyWh  int `json:"energy_wh"`  // watt-hours total
}

// TapoEmeterDataResponse wraps the get_emeter_data response.
type TapoEmeterDataResponse struct {
	Result    TapoEmeterData `json:"result"`
	ErrorCode int            `json:"error_code"`
}

// ToEmeterData converts TapoEmeterData to the standard EmeterData format.
func (t *TapoEmeterData) ToEmeterData() *EmeterData {
	return &EmeterData{
		// Convert from milliamps to amps
		Current: float64(t.CurrentMA) / 1000.0,
		// Convert from millivolts to volts
		Voltage: float64(t.VoltageMA) / 1000.0,
		// Convert from milliwatts to watts
		Power: float64(t.PowerMW) / 1000.0,
		// Convert from watt-hours to kilowatt-hours
		Total: float64(t.EnergyWh) / 1000.0,
	}
}

// TapoErrorResponse represents a TAPO error response.
type TapoErrorResponse struct {
	ErrorCode int    `json:"error_code"`
	ErrorMsg  string `json:"msg,omitempty"`
}
