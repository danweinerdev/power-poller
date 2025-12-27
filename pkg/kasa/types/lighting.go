package types

// ColorTempRange defines min/max color temperature for a bulb model.
type ColorTempRange struct {
	Min int
	Max int
}

// KnownColorTempRanges maps model prefixes to their color temperature ranges.
var KnownColorTempRanges = map[string]ColorTempRange{
	"LB130": {2500, 9000},
	"LB120": {2700, 6500},
	"LB230": {2500, 9000},
	"KL130": {2500, 9000},
	"KL120": {2700, 6500},
	"KL430": {2500, 9000}, // Light strip
}

// DefaultColorTempRange is used when the model is not recognized.
var DefaultColorTempRange = ColorTempRange{2700, 6500}

// GetColorTempRange returns the color temperature range for a given model.
func GetColorTempRange(model string) ColorTempRange {
	for prefix, r := range KnownColorTempRanges {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
			return r
		}
	}
	return DefaultColorTempRange
}

// LightingServiceResponse wraps bulb light state responses.
type LightingServiceResponse struct {
	LightingService struct {
		GetLightState LightState `json:"get_light_state"`
	} `json:"smartlife.iot.smartbulb.lightingservice"`
}

// LightStripServiceResponse wraps light strip state responses.
type LightStripServiceResponse struct {
	LightStrip struct {
		GetLightState LightState `json:"get_light_state"`
	} `json:"smartlife.iot.lightStrip"`
}

// TransitionLightStateResponse wraps the transition_light_state response.
type TransitionLightStateResponse struct {
	LightingService struct {
		TransitionLightState struct {
			ErrCode int `json:"err_code"`
		} `json:"transition_light_state"`
	} `json:"smartlife.iot.smartbulb.lightingservice"`
}

// SetRelayStateResponse wraps the set_relay_state response for plugs.
type SetRelayStateResponse struct {
	System struct {
		SetRelayState struct {
			ErrCode int `json:"err_code"`
		} `json:"set_relay_state"`
	} `json:"system"`
}

// GenericResponse is used for commands that just return an error code.
type GenericResponse struct {
	ErrCode int    `json:"err_code"`
	ErrMsg  string `json:"err_msg,omitempty"`
}
