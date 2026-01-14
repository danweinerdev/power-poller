// Package types defines the data structures for KASA device responses.
package types

// CountdownRule represents a countdown timer rule on a device.
type CountdownRule struct {
	ID     string `json:"id"`     // Rule ID
	Name   string `json:"name"`   // Rule name
	Enable int    `json:"enable"` // 1 = enabled, 0 = disabled
	Delay  int    `json:"delay"`  // Total delay in seconds
	Act    int    `json:"act"`    // Action: 0 = off, 1 = on
	Remain int    `json:"remain"` // Remaining time in seconds
}

// IsEnabled returns true if the countdown rule is enabled.
func (r *CountdownRule) IsEnabled() bool {
	return r.Enable == 1
}

// ActionString returns a human-readable action string.
func (r *CountdownRule) ActionString() string {
	if r.Act == 1 {
		return "turn on"
	}
	return "turn off"
}

// CountdownRulesResponse wraps the get_rules response from count_down namespace.
type CountdownRulesResponse struct {
	CountDown struct {
		GetRules struct {
			RuleList []CountdownRule `json:"rule_list"`
			ErrCode  int             `json:"err_code"`
			ErrMsg   string          `json:"err_msg,omitempty"`
		} `json:"get_rules"`
	} `json:"count_down"`
}

// CountdownAddResponse wraps the add_rule response from count_down namespace.
type CountdownAddResponse struct {
	CountDown struct {
		AddRule struct {
			ID      string `json:"id,omitempty"`
			ErrCode int    `json:"err_code"`
			ErrMsg  string `json:"err_msg,omitempty"`
		} `json:"add_rule"`
	} `json:"count_down"`
}

// CountdownDeleteResponse wraps the delete_rule response from count_down namespace.
type CountdownDeleteResponse struct {
	CountDown struct {
		DeleteRule struct {
			ErrCode int    `json:"err_code"`
			ErrMsg  string `json:"err_msg,omitempty"`
		} `json:"delete_rule"`
	} `json:"count_down"`
}

// CountdownDeleteAllResponse wraps the delete_all_rules response from count_down namespace.
type CountdownDeleteAllResponse struct {
	CountDown struct {
		DeleteAllRules struct {
			ErrCode int    `json:"err_code"`
			ErrMsg  string `json:"err_msg,omitempty"`
		} `json:"delete_all_rules"`
	} `json:"count_down"`
}
