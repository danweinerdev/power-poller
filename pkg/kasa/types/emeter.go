package types

// EmeterData represents real-time energy meter readings.
// Note: Some devices return values in milliwatts/milliamps/millivolts,
// while others return in watts/amps/volts. Use Normalize() to convert.
type EmeterData struct {
	// Standard unit fields (some devices)
	Voltage float64 `json:"voltage,omitempty"`
	Current float64 `json:"current,omitempty"`
	Power   float64 `json:"power,omitempty"`
	Total   float64 `json:"total,omitempty"`

	// Milli-unit fields (some devices)
	VoltageMV float64 `json:"voltage_mv,omitempty"`
	CurrentMA float64 `json:"current_ma,omitempty"`
	PowerMW   float64 `json:"power_mw,omitempty"`
	TotalWH   float64 `json:"total_wh,omitempty"`

	ErrCode int    `json:"err_code,omitempty"`
	ErrMsg  string `json:"err_msg,omitempty"`
}

// Normalize converts milli-unit values to standard units.
// Returns a new EmeterData with normalized values.
func (e *EmeterData) Normalize() EmeterData {
	result := EmeterData{
		ErrCode: e.ErrCode,
		ErrMsg:  e.ErrMsg,
	}

	// Use standard units if available, otherwise convert from milli-units
	if e.Voltage != 0 {
		result.Voltage = e.Voltage
	} else if e.VoltageMV != 0 {
		result.Voltage = e.VoltageMV / 1000.0
	}

	if e.Current != 0 {
		result.Current = e.Current
	} else if e.CurrentMA != 0 {
		result.Current = e.CurrentMA / 1000.0
	}

	if e.Power != 0 {
		result.Power = e.Power
	} else if e.PowerMW != 0 {
		result.Power = e.PowerMW / 1000.0
	}

	if e.Total != 0 {
		result.Total = e.Total
	} else if e.TotalWH != 0 {
		result.Total = e.TotalWH / 1000.0 // Convert Wh to kWh
	}

	return result
}

// DailyUsage represents daily energy consumption.
type DailyUsage struct {
	Year     int     `json:"year"`
	Month    int     `json:"month"`
	Day      int     `json:"day"`
	Energy   float64 `json:"energy,omitempty"`
	EnergyWH float64 `json:"energy_wh,omitempty"`
}

// GetEnergy returns the energy value, handling both field formats.
func (d *DailyUsage) GetEnergy() float64 {
	if d.Energy != 0 {
		return d.Energy
	}
	return d.EnergyWH / 1000.0 // Convert Wh to kWh if needed
}

// MonthlyUsage represents monthly energy consumption.
type MonthlyUsage struct {
	Year     int     `json:"year"`
	Month    int     `json:"month"`
	Energy   float64 `json:"energy,omitempty"`
	EnergyWH float64 `json:"energy_wh,omitempty"`
}

// GetEnergy returns the energy value, handling both field formats.
func (m *MonthlyUsage) GetEnergy() float64 {
	if m.Energy != 0 {
		return m.Energy
	}
	return m.EnergyWH / 1000.0
}

// EmeterRealtimeResponse wraps the get_realtime response for plugs.
type EmeterRealtimeResponse struct {
	Emeter struct {
		GetRealtime EmeterData `json:"get_realtime"`
	} `json:"emeter"`
}

// BulbEmeterRealtimeResponse wraps the get_realtime response for bulbs.
// Bulbs use a different namespace.
type BulbEmeterRealtimeResponse struct {
	Emeter struct {
		GetRealtime EmeterData `json:"get_realtime"`
	} `json:"smartlife.iot.common.emeter"`
}

// EmeterDayStatResponse wraps the get_daystat response.
type EmeterDayStatResponse struct {
	Emeter struct {
		GetDayStat struct {
			DayList []DailyUsage `json:"day_list"`
			ErrCode int          `json:"err_code"`
		} `json:"get_daystat"`
	} `json:"emeter"`
}

// EmeterMonthStatResponse wraps the get_monthstat response.
type EmeterMonthStatResponse struct {
	Emeter struct {
		GetMonthStat struct {
			MonthList []MonthlyUsage `json:"month_list"`
			ErrCode   int            `json:"err_code"`
		} `json:"get_monthstat"`
	} `json:"emeter"`
}
