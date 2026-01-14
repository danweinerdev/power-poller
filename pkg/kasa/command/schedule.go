package command

// Note: GetScheduleRules and GetCountdownRules are defined in system.go

// ----------------------
// Schedule Rule Commands
// ----------------------

// ScheduleRuleParams defines parameters for creating or editing a schedule rule.
type ScheduleRuleParams struct {
	Name   string `json:"name"`             // Rule name
	Enable int    `json:"enable"`           // 1 = enabled, 0 = disabled
	WDay   []int  `json:"wday"`             // [Sun,Mon,Tue,Wed,Thu,Fri,Sat] 0/1
	STime  int    `json:"stime"`            // Start time in minutes from midnight
	ETime  int    `json:"etime"`            // End time in minutes (-1 = none)
	SAct   int    `json:"sact"`             // Start action: 0 = off, 1 = on
	EAct   int    `json:"eact"`             // End action: -1 = none, 0 = off, 1 = on
	Repeat int    `json:"repeat"`           // 1 = repeat, 0 = once
	Year   int    `json:"year,omitempty"`   // For non-repeating rules
	Month  int    `json:"month,omitempty"`  // For non-repeating rules
	Day    int    `json:"day,omitempty"`    // For non-repeating rules
}

// AddScheduleRule returns a command to add a new schedule rule.
func AddScheduleRule(params ScheduleRuleParams) Command {
	return New("schedule", "add_rule", map[string]interface{}{
		"name":   params.Name,
		"enable": params.Enable,
		"wday":   params.WDay,
		"stime":  params.STime,
		"etime":  params.ETime,
		"sact":   params.SAct,
		"eact":   params.EAct,
		"repeat": params.Repeat,
		"year":   params.Year,
		"month":  params.Month,
		"day":    params.Day,
	})
}

// EditScheduleRule returns a command to edit an existing schedule rule.
func EditScheduleRule(id string, params ScheduleRuleParams) Command {
	return New("schedule", "edit_rule", map[string]interface{}{
		"id":     id,
		"name":   params.Name,
		"enable": params.Enable,
		"wday":   params.WDay,
		"stime":  params.STime,
		"etime":  params.ETime,
		"sact":   params.SAct,
		"eact":   params.EAct,
		"repeat": params.Repeat,
		"year":   params.Year,
		"month":  params.Month,
		"day":    params.Day,
	})
}

// EnableScheduleRule returns a command to enable a schedule rule.
func EnableScheduleRule(id string) Command {
	return New("schedule", "edit_rule", map[string]interface{}{
		"id":     id,
		"enable": 1,
	})
}

// DisableScheduleRule returns a command to disable a schedule rule.
func DisableScheduleRule(id string) Command {
	return New("schedule", "edit_rule", map[string]interface{}{
		"id":     id,
		"enable": 0,
	})
}

// DeleteScheduleRule returns a command to delete a schedule rule by ID.
func DeleteScheduleRule(id string) Command {
	return New("schedule", "delete_rule", map[string]interface{}{
		"id": id,
	})
}

// DeleteAllScheduleRules returns a command to delete all schedule rules.
func DeleteAllScheduleRules() Command {
	return New("schedule", "delete_all_rules", nil)
}

// GetNextScheduledAction returns a command to get the next scheduled action.
func GetNextScheduledAction() Command {
	return New("schedule", "get_next_action", nil)
}

// ----------------------
// Countdown Commands
// ----------------------

// AddCountdownRule returns a command to add a countdown timer.
// action: 0 = turn off, 1 = turn on
// delaySec: delay in seconds before the action
// name: optional name for the timer
func AddCountdownRule(action int, delaySec int, name string) Command {
	return New("count_down", "add_rule", map[string]interface{}{
		"enable": 1,
		"delay":  delaySec,
		"act":    action,
		"name":   name,
	})
}

// DeleteCountdownRule returns a command to delete a countdown rule by ID.
func DeleteCountdownRule(id string) Command {
	return New("count_down", "delete_rule", map[string]interface{}{
		"id": id,
	})
}

// DeleteAllCountdownRules returns a command to delete all countdown rules.
func DeleteAllCountdownRules() Command {
	return New("count_down", "delete_all_rules", nil)
}
