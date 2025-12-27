package kasa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/command"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/types"
)

var timeCmd = &cobra.Command{
	Use:   "time",
	Short: "Device time commands",
	Long: `Commands for viewing and setting device time.

Subcommands:
  get      - Get the device's current time
  set      - Set the device's time (syncs to current system time)
  timezone - Get the device's timezone index`,
}

var timeGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get the device's current time",
	RunE:  runTimeGet,
}

var timeSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set the device's time to current system time",
	Long: `Set the device's time to the current system time.

Note: Most devices automatically sync via NTP, so this is rarely needed.`,
	RunE: runTimeSet,
}

var timeTimezoneCmd = &cobra.Command{
	Use:   "timezone",
	Short: "Get the device's timezone",
	RunE:  runTimeTimezone,
}

func init() {
	timeCmd.AddCommand(timeGetCmd)
	timeCmd.AddCommand(timeSetCmd)
	timeCmd.AddCommand(timeTimezoneCmd)
}

func runTimeGet(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	resp, err := dev.SendCommand(ctx, command.GetTime())
	if err != nil {
		return fmt.Errorf("failed to get time: %w", err)
	}

	var result types.TimeResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	timeInfo := result.Time.GetTime
	if timeInfo.ErrCode != 0 {
		return fmt.Errorf("get time failed with error code: %d", timeInfo.ErrCode)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "DEVICE TIME")
	fmt.Fprintln(w, "───────────")
	fmt.Fprintf(w, "Date:\t%04d-%02d-%02d\n", timeInfo.Year, timeInfo.Month, timeInfo.Day)
	fmt.Fprintf(w, "Time:\t%02d:%02d:%02d\n", timeInfo.Hour, timeInfo.Min, timeInfo.Sec)

	// Show difference from system time
	deviceTime := timeInfo.ToTime()
	systemTime := time.Now()
	diff := systemTime.Sub(deviceTime)

	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "System Time:\t%s\n", systemTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "Difference:\t%s\n", diff.Round(time.Second))
	w.Flush()

	return nil
}

func runTimeSet(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	now := time.Now()
	fmt.Printf("Setting device time to: %s\n", now.Format("2006-01-02 15:04:05"))

	resp, err := dev.SendCommand(ctx, command.SetTime(now))
	if err != nil {
		return fmt.Errorf("failed to set time: %w", err)
	}

	var result types.SetTimeResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Time.SetTime.ErrCode != 0 {
		return fmt.Errorf("set time failed with error code: %d", result.Time.SetTime.ErrCode)
	}

	fmt.Println("Time set successfully")
	return nil
}

func runTimeTimezone(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	resp, err := dev.SendCommand(ctx, command.GetTimezone())
	if err != nil {
		return fmt.Errorf("failed to get timezone: %w", err)
	}

	var result types.TimezoneResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	tzInfo := result.Time.GetTimezone
	if tzInfo.ErrCode != 0 {
		return fmt.Errorf("get timezone failed with error code: %d", tzInfo.ErrCode)
	}

	fmt.Printf("Timezone Index: %d\n", tzInfo.Index)
	return nil
}
