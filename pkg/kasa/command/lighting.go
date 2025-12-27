package command

// GetLightState returns command to get bulb light state.
func GetLightState() Command {
	return New(NSLightingService, "get_light_state", nil)
}

// GetLightStateLightStrip returns command to get light strip state.
func GetLightStateLightStrip() Command {
	return New(NSLightStrip, "get_light_state", nil)
}

// SetLightState sets bulb on/off state.
func SetLightState(on bool) Command {
	state := 0
	if on {
		state = 1
	}
	return New(NSLightingService, "transition_light_state", map[string]interface{}{
		"on_off": state,
	})
}

// SetLightStateLightStrip sets light strip on/off state.
func SetLightStateLightStrip(on bool) Command {
	state := 0
	if on {
		state = 1
	}
	return New(NSLightStrip, "set_light_state", map[string]interface{}{
		"on_off": state,
	})
}

// SetBrightness sets bulb brightness (0-100).
func SetBrightness(brightness int) Command {
	return New(NSLightingService, "transition_light_state", map[string]interface{}{
		"on_off":     1,
		"brightness": brightness,
	})
}

// SetBrightnessLightStrip sets light strip brightness (0-100).
func SetBrightnessLightStrip(brightness int) Command {
	return New(NSLightStrip, "set_light_state", map[string]interface{}{
		"on_off":     1,
		"brightness": brightness,
	})
}

// SetHSV sets bulb color using HSV values.
// hue: 0-360, saturation: 0-100, brightness: 0-100
func SetHSV(hue, saturation, brightness int) Command {
	return New(NSLightingService, "transition_light_state", map[string]interface{}{
		"on_off":     1,
		"hue":        hue,
		"saturation": saturation,
		"brightness": brightness,
		"color_temp": 0, // Must be 0 for HSV mode
	})
}

// SetHSVLightStrip sets light strip color using HSV values.
func SetHSVLightStrip(hue, saturation, brightness int) Command {
	return New(NSLightStrip, "set_light_state", map[string]interface{}{
		"on_off":     1,
		"hue":        hue,
		"saturation": saturation,
		"brightness": brightness,
		"color_temp": 0,
	})
}

// SetColorTemp sets bulb color temperature.
func SetColorTemp(temp int) Command {
	return New(NSLightingService, "transition_light_state", map[string]interface{}{
		"on_off":     1,
		"color_temp": temp,
	})
}

// SetColorTempLightStrip sets light strip color temperature.
func SetColorTempLightStrip(temp int) Command {
	return New(NSLightStrip, "set_light_state", map[string]interface{}{
		"on_off":     1,
		"color_temp": temp,
	})
}

// SetLightStateWithTransition sets light state with a transition duration.
// transitionMs is the transition time in milliseconds.
func SetLightStateWithTransition(on bool, transitionMs int) Command {
	state := 0
	if on {
		state = 1
	}
	return New(NSLightingService, "transition_light_state", map[string]interface{}{
		"on_off":          state,
		"transition_time": transitionMs,
	})
}

// SetBrightnessWithTransition sets brightness with a transition duration.
func SetBrightnessWithTransition(brightness, transitionMs int) Command {
	return New(NSLightingService, "transition_light_state", map[string]interface{}{
		"on_off":          1,
		"brightness":      brightness,
		"transition_time": transitionMs,
	})
}

// SetHSVWithTransition sets color with a transition duration.
func SetHSVWithTransition(hue, saturation, brightness, transitionMs int) Command {
	return New(NSLightingService, "transition_light_state", map[string]interface{}{
		"on_off":          1,
		"hue":             hue,
		"saturation":      saturation,
		"brightness":      brightness,
		"color_temp":      0,
		"transition_time": transitionMs,
	})
}

// GetPreferredState returns command to get preferred/default light state.
func GetPreferredState() Command {
	return New(NSLightingService, "get_default_behavior", nil)
}

// SetPreferredState returns command to set preferred/default light state.
func SetPreferredState(mode string, state map[string]interface{}) Command {
	args := map[string]interface{}{
		"mode": mode,
	}
	// Merge state into args
	for k, v := range state {
		args[k] = v
	}
	return New(NSLightingService, "set_default_behavior", args)
}
