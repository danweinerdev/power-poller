package kasa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/command"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/types"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage device schedules",
	Long: `Commands for viewing and managing device schedules.

Subcommands:
  list   - List all schedule rules
  delete - Delete a schedule rule by ID
  clear  - Delete all schedule rules`,
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

func init() {
	scheduleDeleteCmd.Flags().StringVar(&scheduleDeleteID, "id", "", "Schedule rule ID to delete (required)")
	scheduleDeleteCmd.MarkFlagRequired("id")

	scheduleCmd.AddCommand(scheduleListCmd)
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
