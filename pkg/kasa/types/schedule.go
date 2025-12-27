package types

import "fmt"

// ScheduleRulesResponse is the response from get_rules command.
type ScheduleRulesResponse struct {
	Schedule struct {
		GetRules ScheduleRuleList `json:"get_rules"`
	} `json:"schedule"`
}

// ScheduleRuleList contains a list of schedule rules.
type ScheduleRuleList struct {
	RuleList []ScheduleRule `json:"rule_list"`
	ErrCode  int            `json:"err_code"`
}

// ScheduleRule represents a single schedule rule.
type ScheduleRule struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Enable   int    `json:"enable"`   // 1=enabled, 0=disabled
	WDay     []int  `json:"wday"`     // Days of week (0=Sun, 1=Mon, etc.)
	STime    int    `json:"stime"`    // Start time in minutes from midnight
	ETime    int    `json:"etime"`    // End time in minutes from midnight (-1 if not used)
	SAction  int    `json:"sact"`     // Start action: 0=off, 1=on
	EAction  int    `json:"eact"`     // End action: 0=off, 1=on, -1=none
	Repeat   int    `json:"repeat"`   // 1=repeat, 0=once
	Year     int    `json:"year"`     // Year (for non-repeating)
	Month    int    `json:"month"`    // Month (for non-repeating)
	Day      int    `json:"day"`      // Day (for non-repeating)
	ErrCode  int    `json:"err_code"`
}

// IsEnabled returns true if the rule is enabled.
func (r ScheduleRule) IsEnabled() bool {
	return r.Enable == 1
}

// StartTimeString returns the start time formatted as HH:MM.
func (r ScheduleRule) StartTimeString() string {
	hours := r.STime / 60
	mins := r.STime % 60
	return fmt.Sprintf("%02d:%02d", hours, mins)
}

// EndTimeString returns the end time formatted as HH:MM, or "-" if not set.
func (r ScheduleRule) EndTimeString() string {
	if r.ETime < 0 {
		return "-"
	}
	hours := r.ETime / 60
	mins := r.ETime % 60
	return fmt.Sprintf("%02d:%02d", hours, mins)
}

// StartActionString returns the start action as a string.
func (r ScheduleRule) StartActionString() string {
	if r.SAction == 1 {
		return "ON"
	}
	return "OFF"
}

// EndActionString returns the end action as a string.
func (r ScheduleRule) EndActionString() string {
	switch r.EAction {
	case 1:
		return "ON"
	case 0:
		return "OFF"
	default:
		return "-"
	}
}

// DaysString returns a string representation of the schedule days.
func (r ScheduleRule) DaysString() string {
	if len(r.WDay) == 0 {
		if r.Year > 0 {
			return fmt.Sprintf("%04d-%02d-%02d", r.Year, r.Month, r.Day)
		}
		return "Once"
	}

	if len(r.WDay) == 7 {
		allSet := true
		for _, d := range r.WDay {
			if d == 0 {
				allSet = false
				break
			}
		}
		if allSet {
			return "Daily"
		}
	}

	days := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	var result string
	for i, d := range r.WDay {
		if d == 1 {
			if result != "" {
				result += ","
			}
			result += days[i]
		}
	}
	return result
}

// DeleteRuleResponse is the response from delete_rule command.
type DeleteRuleResponse struct {
	Schedule struct {
		DeleteRule struct {
			ErrCode int `json:"err_code"`
		} `json:"delete_rule"`
	} `json:"schedule"`
}

// DeleteAllRulesResponse is the response from delete_all_rules command.
type DeleteAllRulesResponse struct {
	Schedule struct {
		DeleteAllRules struct {
			ErrCode int `json:"err_code"`
		} `json:"delete_all_rules"`
	} `json:"schedule"`
}

// NextActionResponse is the response from get_next_action command.
type NextActionResponse struct {
	Schedule struct {
		GetNextAction NextAction `json:"get_next_action"`
	} `json:"schedule"`
}

// NextAction represents the next scheduled action.
type NextAction struct {
	Type    int `json:"type"`     // Action type
	Action  int `json:"action"`   // 0=off, 1=on
	SchTime int `json:"schd_sec"` // Seconds until action
	ErrCode int `json:"err_code"`
}
