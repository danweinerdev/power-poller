package kasa

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/power-poller/pkg/kasa/device"
)

var (
	emeterYear  int
	emeterMonth int
)

var emeterCmd = &cobra.Command{
	Use:   "emeter",
	Short: "Display energy meter readings",
	Long: `Display energy meter readings from the device.

By default shows realtime data. Use --year and --month for historical data.`,
	RunE: runEmeter,
}

func init() {
	now := time.Now()
	emeterCmd.Flags().IntVarP(&emeterYear, "year", "y", now.Year(), "year for historical data")
	emeterCmd.Flags().IntVarP(&emeterMonth, "month", "m", 0, "month for daily stats (1-12)")
}

func runEmeter(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	emeterDev, ok := dev.(device.EmeterDevice)
	if !ok {
		return fmt.Errorf("device does not support energy monitoring")
	}

	if !emeterDev.HasEmeter() {
		return fmt.Errorf("device does not have an energy meter")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Show monthly or daily stats if requested
	if emeterMonth > 0 {
		data, err := emeterDev.GetEmeterDaily(ctx, emeterYear, emeterMonth)
		if err != nil {
			return fmt.Errorf("failed to get daily stats: %w", err)
		}

		fmt.Fprintf(w, "Daily Usage for %d/%d\n", emeterMonth, emeterYear)
		fmt.Fprintln(w, "Day\tEnergy (kWh)")
		fmt.Fprintln(w, "───\t────────────")
		for _, d := range data {
			fmt.Fprintf(w, "%d\t%.3f\n", d.Day, d.Energy)
		}
		w.Flush()
		return nil
	}

	if cmd.Flags().Changed("year") {
		data, err := emeterDev.GetEmeterMonthly(ctx, emeterYear)
		if err != nil {
			return fmt.Errorf("failed to get monthly stats: %w", err)
		}

		fmt.Fprintf(w, "Monthly Usage for %d\n", emeterYear)
		fmt.Fprintln(w, "Month\tEnergy (kWh)")
		fmt.Fprintln(w, "─────\t────────────")
		for _, m := range data {
			fmt.Fprintf(w, "%d\t%.3f\n", m.Month, m.Energy)
		}
		w.Flush()
		return nil
	}

	// Show realtime data
	data, err := emeterDev.GetEmeterRealtime(ctx)
	if err != nil {
		return fmt.Errorf("failed to get emeter data: %w", err)
	}

	fmt.Fprintln(w, "REALTIME ENERGY DATA")
	fmt.Fprintln(w, "────────────────────")
	fmt.Fprintf(w, "Voltage:\t%.1f V\n", data.Voltage)
	fmt.Fprintf(w, "Current:\t%.3f A\n", data.Current)
	fmt.Fprintf(w, "Power:\t%.1f W\n", data.Power)
	fmt.Fprintf(w, "Total:\t%.3f kWh\n", data.Total)
	w.Flush()

	return nil
}
