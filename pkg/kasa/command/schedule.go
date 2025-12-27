package command

// Note: GetScheduleRules is defined in system.go

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
