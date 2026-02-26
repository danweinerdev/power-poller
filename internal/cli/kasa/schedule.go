package kasa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/power-poller/pkg/kasa/command"
	"github.com/danweinerdev/power-poller/pkg/kasa/types"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage device schedules",
	Long: `Commands for viewing and managing device schedules.

Subcommands:
  list    - List all schedule rules
  add     - Add a new schedule rule
  edit    - Edit an existing schedule rule
  enable  - Enable a schedule rule
  disable - Disable a schedule rule
  delete  - Delete a schedule rule by ID
  clear   - Delete all schedule rules`,
}

var scheduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all schedule rules",
	RunE:  runScheduleList,
}

var scheduleDeleteID string

var scheduleDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a schedule rule",
	Long:  `Delete a schedule rule by its ID. Use 'schedule list' to find rule IDs.`,
	RunE:  runScheduleDelete,
}

var scheduleClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all schedule rules",
	Long: `Delete ALL schedule rules from the device.

WARNING: This action cannot be undone!`,
	RunE: runScheduleClear,
}

var scheduleAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new schedule rule",
	Long: `Add a new schedule rule to the device.

Examples:
  kasa schedule add --name "Morning" --time 07:00 --action on --days Mon,Tue,Wed,Thu,Fri
  kasa schedule add --name "Night Off" --time 23:00 --action off --repeat
  kasa schedule add --name "Once" --time 14:00 --action on --date 2024-12-25`,
	RunE: runScheduleAdd,
}

var scheduleEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit an existing schedule rule",
	Long: `Edit an existing schedule rule. Only specified fields will be updated.

Examples:
  kasa schedule edit --id ABC123 --time 08:00
  kasa schedule edit --id ABC123 --name "New Name" --action off`,
	RunE: runScheduleEdit,
}

var scheduleEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable a schedule rule",
	Long:  `Enable a disabled schedule rule by its ID.`,
	RunE:  runScheduleEnable,
}

var scheduleDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable a schedule rule",
	Long:  `Disable an active schedule rule by its ID.`,
	RunE:  runScheduleDisable,
}

// Schedule command flags
var (
	scheduleAddName     string
	scheduleAddTime     string
	scheduleAddEndTime  string
	scheduleAddAction   string
	scheduleAddEndAction string
	scheduleAddDays     string
	scheduleAddDate     string
	scheduleEditID      string
)

func init() {
	scheduleDeleteCmd.Flags().StringVar(&scheduleDeleteID, "id", "", "Schedule rule ID to delete (required)")
	scheduleDeleteCmd.MarkFlagRequired("id")

	// Add command flags
	scheduleAddCmd.Flags().StringVar(&scheduleAddName, "name", "", "Rule name (required)")
	scheduleAddCmd.Flags().StringVar(&scheduleAddTime, "time", "", "Start time in HH:MM format (required)")
	scheduleAddCmd.Flags().StringVar(&scheduleAddEndTime, "end-time", "", "End time in HH:MM format (optional)")
	scheduleAddCmd.Flags().StringVar(&scheduleAddAction, "action", "", "Start action: on or off (required)")
	scheduleAddCmd.Flags().StringVar(&scheduleAddEndAction, "end-action", "", "End action: on or off (optional)")
	scheduleAddCmd.Flags().StringVar(&scheduleAddDays, "days", "", "Days: Mon,Tue,Wed,Thu,Fri,Sat,Sun or 'daily' (optional)")
	scheduleAddCmd.Flags().StringVar(&scheduleAddDate, "date", "", "Single date in YYYY-MM-DD format (optional, for one-time rules)")
	scheduleAddCmd.MarkFlagRequired("name")
	scheduleAddCmd.MarkFlagRequired("time")
	scheduleAddCmd.MarkFlagRequired("action")

	// Edit command flags (reuse some add flags via persistent)
	scheduleEditCmd.Flags().StringVar(&scheduleEditID, "id", "", "Schedule rule ID to edit (required)")
	scheduleEditCmd.Flags().StringVar(&scheduleAddName, "name", "", "New rule name")
	scheduleEditCmd.Flags().StringVar(&scheduleAddTime, "time", "", "New start time in HH:MM format")
	scheduleEditCmd.Flags().StringVar(&scheduleAddEndTime, "end-time", "", "New end time in HH:MM format")
	scheduleEditCmd.Flags().StringVar(&scheduleAddAction, "action", "", "New start action: on or off")
	scheduleEditCmd.Flags().StringVar(&scheduleAddEndAction, "end-action", "", "New end action: on or off")
	scheduleEditCmd.Flags().StringVar(&scheduleAddDays, "days", "", "New days: Mon,Tue,Wed,Thu,Fri,Sat,Sun or 'daily'")
	scheduleEditCmd.MarkFlagRequired("id")

	// Enable/disable command flags
	scheduleEnableCmd.Flags().StringVar(&scheduleEditID, "id", "", "Schedule rule ID to enable (required)")
	scheduleEnableCmd.MarkFlagRequired("id")
	scheduleDisableCmd.Flags().StringVar(&scheduleEditID, "id", "", "Schedule rule ID to disable (required)")
	scheduleDisableCmd.MarkFlagRequired("id")

	scheduleCmd.AddCommand(scheduleListCmd)
	scheduleCmd.AddCommand(scheduleAddCmd)
	scheduleCmd.AddCommand(scheduleEditCmd)
	scheduleCmd.AddCommand(scheduleEnableCmd)
	scheduleCmd.AddCommand(scheduleDisableCmd)
	scheduleCmd.AddCommand(scheduleDeleteCmd)
	scheduleCmd.AddCommand(scheduleClearCmd)
}

func runScheduleList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	resp, err := dev.SendCommand(ctx, command.GetScheduleRules())
	if err != nil {
		return fmt.Errorf("failed to get schedules: %w", err)
	}

	var result types.ScheduleRulesResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	rules := result.Schedule.GetRules
	if rules.ErrCode != 0 {
		return fmt.Errorf("get schedules failed with error code: %d", rules.ErrCode)
	}

	if len(rules.RuleList) == 0 {
		fmt.Println("No schedule rules configured")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tName\tEnabled\tDays\tStart\tAction\tEnd\tAction")
	fmt.Fprintln(w, "──\t────\t───────\t────\t─────\t──────\t───\t──────")

	for _, rule := range rules.RuleList {
		enabled := "No"
		if rule.IsEnabled() {
			enabled = "Yes"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			rule.ID,
			rule.Name,
			enabled,
			rule.DaysString(),
			rule.StartTimeString(),
			rule.StartActionString(),
			rule.EndTimeString(),
			rule.EndActionString(),
		)
	}
	w.Flush()

	fmt.Printf("\n%d schedule rule(s)\n", len(rules.RuleList))
	return nil
}

func runScheduleDelete(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	fmt.Printf("Deleting schedule rule: %s\n", scheduleDeleteID)

	resp, err := dev.SendCommand(ctx, command.DeleteScheduleRule(scheduleDeleteID))
	if err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}

	var result types.DeleteRuleResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Schedule.DeleteRule.ErrCode != 0 {
		return fmt.Errorf("delete failed with error code: %d", result.Schedule.DeleteRule.ErrCode)
	}

	fmt.Println("Schedule rule deleted successfully")
	return nil
}

func runScheduleClear(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	fmt.Println("Deleting all schedule rules...")

	resp, err := dev.SendCommand(ctx, command.DeleteAllScheduleRules())
	if err != nil {
		return fmt.Errorf("failed to clear schedules: %w", err)
	}

	var result types.DeleteAllRulesResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Schedule.DeleteAllRules.ErrCode != 0 {
		return fmt.Errorf("clear failed with error code: %d", result.Schedule.DeleteAllRules.ErrCode)
	}

	fmt.Println("All schedule rules deleted successfully")
	return nil
}

func runScheduleAdd(cmd *cobra.Command, args []string) error {
	// Parse time
	stime, err := parseTimeToMinutes(scheduleAddTime)
	if err != nil {
		return fmt.Errorf("invalid start time: %w", err)
	}

	// Parse end time (optional)
	etime := -1
	if scheduleAddEndTime != "" {
		etime, err = parseTimeToMinutes(scheduleAddEndTime)
		if err != nil {
			return fmt.Errorf("invalid end time: %w", err)
		}
	}

	// Parse action
	sact, err := parseAction(scheduleAddAction)
	if err != nil {
		return fmt.Errorf("invalid start action: %w", err)
	}

	// Parse end action (optional)
	eact := -1
	if scheduleAddEndAction != "" {
		eact, err = parseAction(scheduleAddEndAction)
		if err != nil {
			return fmt.Errorf("invalid end action: %w", err)
		}
	}

	// Parse days or date
	var wday []int
	var year, month, day int
	repeat := 0

	if scheduleAddDate != "" {
		// One-time schedule on specific date
		parts := strings.Split(scheduleAddDate, "-")
		if len(parts) != 3 {
			return fmt.Errorf("invalid date format, use YYYY-MM-DD")
		}
		year, _ = strconv.Atoi(parts[0])
		month, _ = strconv.Atoi(parts[1])
		day, _ = strconv.Atoi(parts[2])
		wday = []int{0, 0, 0, 0, 0, 0, 0}
	} else if scheduleAddDays != "" {
		wday, err = parseDays(scheduleAddDays)
		if err != nil {
			return fmt.Errorf("invalid days: %w", err)
		}
		repeat = 1
	} else {
		// Default to daily
		wday = []int{1, 1, 1, 1, 1, 1, 1}
		repeat = 1
	}

	params := command.ScheduleRuleParams{
		Name:   scheduleAddName,
		Enable: 1,
		WDay:   wday,
		STime:  stime,
		ETime:  etime,
		SAct:   sact,
		EAct:   eact,
		Repeat: repeat,
		Year:   year,
		Month:  month,
		Day:    day,
	}

	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	resp, err := dev.SendCommand(ctx, command.AddScheduleRule(params))
	if err != nil {
		return fmt.Errorf("failed to add schedule: %w", err)
	}

	var result types.AddRuleResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	addResult := result.Schedule.AddRule
	if addResult.ErrCode != 0 {
		return fmt.Errorf("add failed: %s (code %d)", addResult.ErrMsg, addResult.ErrCode)
	}

	fmt.Printf("Schedule rule '%s' added successfully (ID: %s)\n", scheduleAddName, addResult.ID)
	return nil
}

func runScheduleEdit(cmd *cobra.Command, args []string) error {
	// Build params from flags that were set
	params := command.ScheduleRuleParams{}

	if scheduleAddName != "" {
		params.Name = scheduleAddName
	}

	if scheduleAddTime != "" {
		stime, err := parseTimeToMinutes(scheduleAddTime)
		if err != nil {
			return fmt.Errorf("invalid start time: %w", err)
		}
		params.STime = stime
	}

	if scheduleAddEndTime != "" {
		etime, err := parseTimeToMinutes(scheduleAddEndTime)
		if err != nil {
			return fmt.Errorf("invalid end time: %w", err)
		}
		params.ETime = etime
	} else {
		params.ETime = -1
	}

	if scheduleAddAction != "" {
		sact, err := parseAction(scheduleAddAction)
		if err != nil {
			return fmt.Errorf("invalid start action: %w", err)
		}
		params.SAct = sact
	}

	if scheduleAddEndAction != "" {
		eact, err := parseAction(scheduleAddEndAction)
		if err != nil {
			return fmt.Errorf("invalid end action: %w", err)
		}
		params.EAct = eact
	} else {
		params.EAct = -1
	}

	if scheduleAddDays != "" {
		wday, err := parseDays(scheduleAddDays)
		if err != nil {
			return fmt.Errorf("invalid days: %w", err)
		}
		params.WDay = wday
		params.Repeat = 1
	}

	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	resp, err := dev.SendCommand(ctx, command.EditScheduleRule(scheduleEditID, params))
	if err != nil {
		return fmt.Errorf("failed to edit schedule: %w", err)
	}

	var result types.EditRuleResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	editResult := result.Schedule.EditRule
	if editResult.ErrCode != 0 {
		return fmt.Errorf("edit failed: %s (code %d)", editResult.ErrMsg, editResult.ErrCode)
	}

	fmt.Printf("Schedule rule %s updated successfully\n", scheduleEditID)
	return nil
}

func runScheduleEnable(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	resp, err := dev.SendCommand(ctx, command.EnableScheduleRule(scheduleEditID))
	if err != nil {
		return fmt.Errorf("failed to enable schedule: %w", err)
	}

	var result types.EditRuleResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	editResult := result.Schedule.EditRule
	if editResult.ErrCode != 0 {
		return fmt.Errorf("enable failed: %s (code %d)", editResult.ErrMsg, editResult.ErrCode)
	}

	fmt.Printf("Schedule rule %s enabled\n", scheduleEditID)
	return nil
}

func runScheduleDisable(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	resp, err := dev.SendCommand(ctx, command.DisableScheduleRule(scheduleEditID))
	if err != nil {
		return fmt.Errorf("failed to disable schedule: %w", err)
	}

	var result types.EditRuleResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	editResult := result.Schedule.EditRule
	if editResult.ErrCode != 0 {
		return fmt.Errorf("disable failed: %s (code %d)", editResult.ErrMsg, editResult.ErrCode)
	}

	fmt.Printf("Schedule rule %s disabled\n", scheduleEditID)
	return nil
}

// parseTimeToMinutes parses a time string (HH:MM) to minutes from midnight.
func parseTimeToMinutes(timeStr string) (int, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid time format, use HH:MM")
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil || hours < 0 || hours > 23 {
		return 0, fmt.Errorf("invalid hours")
	}
	mins, err := strconv.Atoi(parts[1])
	if err != nil || mins < 0 || mins > 59 {
		return 0, fmt.Errorf("invalid minutes")
	}
	return hours*60 + mins, nil
}

// parseAction parses an action string (on/off) to integer.
func parseAction(action string) (int, error) {
	switch strings.ToLower(action) {
	case "on":
		return 1, nil
	case "off":
		return 0, nil
	default:
		return 0, fmt.Errorf("must be 'on' or 'off'")
	}
}

// parseDays parses a days string to a weekday array.
func parseDays(daysStr string) ([]int, error) {
	if strings.ToLower(daysStr) == "daily" {
		return []int{1, 1, 1, 1, 1, 1, 1}, nil
	}

	dayMap := map[string]int{
		"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
	}

	wday := []int{0, 0, 0, 0, 0, 0, 0}
	parts := strings.Split(daysStr, ",")
	for _, part := range parts {
		day := strings.ToLower(strings.TrimSpace(part))
		idx, ok := dayMap[day]
		if !ok {
			return nil, fmt.Errorf("unknown day '%s'", part)
		}
		wday[idx] = 1
	}
	return wday, nil
}
