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

// formatUptime converts seconds to a human-readable duration string.
func formatUptime(seconds int) string {
	if seconds <= 0 {
		return "N/A"
	}
	d := time.Duration(seconds) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display device information",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dev, err := loadDevice(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer dev.Close()

		sysInfo := dev.SysInfo()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Alias:\t%s\n", dev.Alias())
		fmt.Fprintf(w, "Model:\t%s\n", dev.Model())
		fmt.Fprintf(w, "Type:\t%s\n", dev.Type())
		fmt.Fprintf(w, "Device ID:\t%s\n", dev.DeviceID())
		fmt.Fprintf(w, "MAC:\t%s\n", dev.MAC())
		fmt.Fprintf(w, "Host:\t%s\n", dev.Host())

		if parentDev, ok := dev.(device.ParentDevice); ok && parentDev.HasChildren() {
			fmt.Fprintf(w, "Outlets:\t%d\n", len(parentDev.Children()))
		}

		// Additional metadata from sysinfo
		if sysInfo != nil {
			if sysInfo.SWVersion != "" {
				fmt.Fprintf(w, "Firmware:\t%s\n", sysInfo.SWVersion)
			}
			if sysInfo.HWVersion != "" {
				fmt.Fprintf(w, "Hardware:\t%s\n", sysInfo.HWVersion)
			}
			if sysInfo.RSSI != 0 {
				fmt.Fprintf(w, "RSSI:\t%d dBm\n", sysInfo.RSSI)
			}
			if sysInfo.SSID != "" {
				fmt.Fprintf(w, "SSID:\t%s\n", sysInfo.SSID)
			}
			if sysInfo.OnTime > 0 {
				fmt.Fprintf(w, "Uptime:\t%s\n", formatUptime(sysInfo.OnTime))
			}
			if sysInfo.Feature != "" {
				fmt.Fprintf(w, "Features:\t%s\n", sysInfo.Feature)
			}
		}

		w.Flush()
		return nil
	},
}

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Display device power state",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dev, err := loadDevice(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer dev.Close()

		if dev.IsOn() {
			fmt.Println("ON")
		} else {
			fmt.Println("OFF")
		}
		return nil
	},
}

var onCmd = &cobra.Command{
	Use:   "on",
	Short: "Turn device on",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dev, err := loadDevice(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer dev.Close()

		if err := dev.TurnOn(ctx); err != nil {
			return fmt.Errorf("failed to turn on: %w", err)
		}
		fmt.Printf("%s is now ON\n", dev.Alias())
		return nil
	},
}

var offCmd = &cobra.Command{
	Use:   "off",
	Short: "Turn device off",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dev, err := loadDevice(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer dev.Close()

		if err := dev.TurnOff(ctx); err != nil {
			return fmt.Errorf("failed to turn off: %w", err)
		}
		fmt.Printf("%s is now OFF\n", dev.Alias())
		return nil
	},
}

var toggleCmd = &cobra.Command{
	Use:   "toggle",
	Short: "Toggle device power state",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dev, err := loadDevice(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer dev.Close()

		if dev.IsOn() {
			if err := dev.TurnOff(ctx); err != nil {
				return fmt.Errorf("failed to turn off: %w", err)
			}
			fmt.Printf("%s is now OFF\n", dev.Alias())
		} else {
			if err := dev.TurnOn(ctx); err != nil {
				return fmt.Errorf("failed to turn on: %w", err)
			}
			fmt.Printf("%s is now ON\n", dev.Alias())
		}
		return nil
	},
}
