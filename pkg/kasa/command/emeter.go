package command

import "time"

// GetEmeterRealtime returns command to get real-time emeter data for plugs.
func GetEmeterRealtime() Command {
	return New(NSEmeter, "get_realtime", nil)
}

// GetEmeterRealtimeBulb returns command for bulb emeter (different namespace).
func GetEmeterRealtimeBulb() Command {
	return New(NSBulbEmeter, "get_realtime", nil)
}

// GetEmeterRealtimeForChild returns command to get emeter data for a child device.
// For HS300 power strips, context.child_ids must be a direct array, not nested.
func GetEmeterRealtimeForChild(childID string) Command {
	return Command{
		"context": {
			"child_ids": []string{childID},
		},
		"emeter": {
			"get_realtime": map[string]interface{}{},
		},
	}
}

// GetEmeterDaily returns command to get daily stats for a given month.
func GetEmeterDaily(year, month int) Command {
	return New(NSEmeter, "get_daystat", map[string]interface{}{
		"year":  year,
		"month": month,
	})
}

// GetEmeterDailyThisMonth returns command to get daily stats for the current month.
func GetEmeterDailyThisMonth() Command {
	now := time.Now()
	return GetEmeterDaily(now.Year(), int(now.Month()))
}

// GetEmeterMonthly returns command to get monthly stats for a given year.
func GetEmeterMonthly(year int) Command {
	return New(NSEmeter, "get_monthstat", map[string]interface{}{
		"year": year,
	})
}

// GetEmeterMonthlyThisYear returns command to get monthly stats for the current year.
func GetEmeterMonthlyThisYear() Command {
	return GetEmeterMonthly(time.Now().Year())
}

// EraseEmeterStats returns command to clear emeter statistics.
func EraseEmeterStats() Command {
	return New(NSEmeter, "erase_emeter_stat", nil)
}

// GetEmeterVGain returns command to get voltage gain calibration.
func GetEmeterVGain() Command {
	return New(NSEmeter, "get_vgain_igain", nil)
}

// SetEmeterVGain returns command to set voltage gain calibration.
func SetEmeterVGain(vgain, igain int) Command {
	return New(NSEmeter, "set_vgain_igain", map[string]interface{}{
		"vgain": vgain,
		"igain": igain,
	})
}
