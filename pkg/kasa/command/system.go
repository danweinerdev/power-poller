package command

// GetSysInfo returns the get_sysinfo command.
func GetSysInfo() Command {
	return New(NSSystem, "get_sysinfo", nil)
}

// SetRelayState returns command to set relay state (on/off).
func SetRelayState(on bool) Command {
	state := 0
	if on {
		state = 1
	}
	return New(NSSystem, "set_relay_state", map[string]interface{}{
		"state": state,
	})
}

// SetRelayStateForChild returns command to set relay state for a child device.
func SetRelayStateForChild(childID string, on bool) Command {
	state := 0
	if on {
		state = 1
	}
	return Merge(
		New(NSContext, "child_ids", map[string]interface{}{
			"child_ids": []string{childID},
		}),
		New(NSSystem, "set_relay_state", map[string]interface{}{
			"state": state,
		}),
	)
}

// SetAlias returns command to set device alias.
func SetAlias(alias string) Command {
	return New(NSSystem, "set_dev_alias", map[string]interface{}{
		"alias": alias,
	})
}

// Reboot returns command to reboot device with optional delay in seconds.
func Reboot(delaySec int) Command {
	return New(NSSystem, "reboot", map[string]interface{}{
		"delay": delaySec,
	})
}

// SetLEDOff returns command to control the LED state on plugs.
// off=true turns the LED off, off=false turns it on.
func SetLEDOff(off bool) Command {
	state := 0
	if off {
		state = 1
	}
	return New(NSSystem, "set_led_off", map[string]interface{}{
		"off": state,
	})
}

// GetScheduleRules returns command to get schedule rules.
func GetScheduleRules() Command {
	return New("schedule", "get_rules", nil)
}

// GetCountdownRules returns command to get countdown rules.
func GetCountdownRules() Command {
	return New("count_down", "get_rules", nil)
}

// GetCloudInfo returns command to get cloud connection info.
func GetCloudInfo() Command {
	return New("cnCloud", "get_info", nil)
}

// GetTime returns command to get device time.
func GetTime() Command {
	return New("time", "get_time", nil)
}

// GetTimezone returns command to get device timezone.
func GetTimezone() Command {
	return New("time", "get_timezone", nil)
}
