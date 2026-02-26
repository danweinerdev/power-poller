package kasa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/power-poller/pkg/kasa/command"
	"github.com/danweinerdev/power-poller/pkg/kasa/types"
)

var countdownCmd = &cobra.Command{
	Use:   "countdown",
	Short: "Countdown timer commands",
	Long:  `Manage countdown timers to automatically turn devices on or off after a delay.`,
}

var countdownListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active countdown timers",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dev, err := loadDevice(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer dev.Close()

		resp, err := dev.SendCommand(ctx, command.GetCountdownRules())
		if err != nil {
			return fmt.Errorf("failed to get countdown rules: %w", err)
		}

		var result types.CountdownRulesResponse
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("failed to parse countdown rules: %w", err)
		}

		rules := result.CountDown.GetRules
		if rules.ErrCode != 0 {
			return fmt.Errorf("device error: %s (code %d)", rules.ErrMsg, rules.ErrCode)
		}

		if len(rules.RuleList) == 0 {
			fmt.Println("No countdown timers active")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "ID\tNAME\tACTION\tDELAY\tREMAINING\tSTATUS\n")
		for _, rule := range rules.RuleList {
			status := "disabled"
			if rule.IsEnabled() {
				status = "active"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				rule.ID,
				rule.Name,
				rule.ActionString(),
				formatDuration(rule.Delay),
				formatDuration(rule.Remain),
				status,
			)
		}
		w.Flush()
		return nil
	},
}

var countdownSetCmd = &cobra.Command{
	Use:   "set <duration> <action>",
	Short: "Set a countdown timer",
	Long: `Set a countdown timer to turn the device on or off after a delay.

Duration format: 30s, 5m, 1h, 1h30m
Action: on, off

Examples:
  kasa countdown set 30m off    # Turn off in 30 minutes
  kasa countdown set 1h on      # Turn on in 1 hour
  kasa countdown set 1h30m off  # Turn off in 1 hour 30 minutes`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		duration, err := time.ParseDuration(args[0])
		if err != nil {
			return fmt.Errorf("invalid duration '%s': %w", args[0], err)
		}

		action := strings.ToLower(args[1])
		var actionInt int
		switch action {
		case "on":
			actionInt = 1
		case "off":
			actionInt = 0
		default:
			return fmt.Errorf("invalid action '%s': must be 'on' or 'off'", args[1])
		}

		ctx := context.Background()
		dev, err := loadDevice(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer dev.Close()

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			name = fmt.Sprintf("timer_%s_%s", args[0], action)
		}

		resp, err := dev.SendCommand(ctx, command.AddCountdownRule(actionInt, int(duration.Seconds()), name))
		if err != nil {
			return fmt.Errorf("failed to set countdown: %w", err)
		}

		var result types.CountdownAddResponse
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		addResult := result.CountDown.AddRule
		if addResult.ErrCode != 0 {
			return fmt.Errorf("device error: %s (code %d)", addResult.ErrMsg, addResult.ErrCode)
		}

		fmt.Printf("Countdown set: %s will turn %s in %s\n", dev.Alias(), action, formatDuration(int(duration.Seconds())))
		return nil
	},
}

var countdownDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a countdown timer by ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id flag is required")
		}

		ctx := context.Background()
		dev, err := loadDevice(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer dev.Close()

		resp, err := dev.SendCommand(ctx, command.DeleteCountdownRule(id))
		if err != nil {
			return fmt.Errorf("failed to delete countdown: %w", err)
		}

		var result types.CountdownDeleteResponse
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		deleteResult := result.CountDown.DeleteRule
		if deleteResult.ErrCode != 0 {
			return fmt.Errorf("device error: %s (code %d)", deleteResult.ErrMsg, deleteResult.ErrCode)
		}

		fmt.Printf("Countdown timer %s deleted\n", id)
		return nil
	},
}

var countdownCancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel all countdown timers",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dev, err := loadDevice(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer dev.Close()

		resp, err := dev.SendCommand(ctx, command.DeleteAllCountdownRules())
		if err != nil {
			return fmt.Errorf("failed to cancel countdowns: %w", err)
		}

		var result types.CountdownDeleteAllResponse
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		deleteResult := result.CountDown.DeleteAllRules
		if deleteResult.ErrCode != 0 {
			return fmt.Errorf("device error: %s (code %d)", deleteResult.ErrMsg, deleteResult.ErrCode)
		}

		fmt.Println("All countdown timers cancelled")
		return nil
	},
}

// formatDuration converts seconds to a human-readable duration string.
func formatDuration(seconds int) string {
	if seconds <= 0 {
		return "0s"
	}
	d := time.Duration(seconds) * time.Second
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh%dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		if secs > 0 {
			return fmt.Sprintf("%dm%ds", minutes, secs)
		}
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", secs)
}

func init() {
	countdownSetCmd.Flags().String("name", "", "optional name for the timer")
	countdownDeleteCmd.Flags().String("id", "", "rule ID to delete")

	countdownCmd.AddCommand(countdownListCmd)
	countdownCmd.AddCommand(countdownSetCmd)
	countdownCmd.AddCommand(countdownDeleteCmd)
	countdownCmd.AddCommand(countdownCancelCmd)
}
