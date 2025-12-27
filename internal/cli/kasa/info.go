package kasa

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/device"
)

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
