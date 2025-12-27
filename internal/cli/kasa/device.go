package kasa

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/device"
)

var aliasCmd = &cobra.Command{
	Use:   "alias [new-alias]",
	Short: "Get or set device alias",
	Long: `Get or set the device alias (friendly name).

Without arguments, displays current alias.
With an argument, sets the alias to the specified value.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAlias,
}

func runAlias(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	if len(args) == 0 {
		fmt.Printf("Alias: %s\n", dev.Alias())
		return nil
	}

	if err := dev.SetAlias(ctx, args[0]); err != nil {
		return fmt.Errorf("failed to set alias: %w", err)
	}

	fmt.Printf("Alias set to: %s\n", args[0])
	return nil
}

var (
	rebootDelay time.Duration
)

var rebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Reboot the device",
	RunE: runReboot,
}

func init() {
	rebootCmd.Flags().DurationVar(&rebootDelay, "delay", time.Second, "delay before reboot")
}

func runReboot(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	if err := dev.Reboot(ctx, rebootDelay); err != nil {
		return fmt.Errorf("failed to reboot: %w", err)
	}

	fmt.Printf("%s will reboot in %v\n", dev.Alias(), rebootDelay)
	return nil
}

var (
	ledState bool
)

var ledCmd = &cobra.Command{
	Use:   "led [on|off]",
	Short: "Control device LED",
	Long: `Control the device status LED.

Arguments:
  on  - Turn LED on
  off - Turn LED off (LED will be off when device is on)`,
	Args: cobra.ExactArgs(1),
	RunE: runLED,
}

func runLED(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	plug, ok := dev.(*device.Plug)
	if !ok {
		return fmt.Errorf("LED control is only supported on smart plugs")
	}

	switch args[0] {
	case "on":
		if err := plug.SetLED(ctx, true); err != nil {
			return fmt.Errorf("failed to turn LED on: %w", err)
		}
		fmt.Println("LED turned on")
	case "off":
		if err := plug.SetLED(ctx, false); err != nil {
			return fmt.Errorf("failed to turn LED off: %w", err)
		}
		fmt.Println("LED turned off")
	default:
		return fmt.Errorf("invalid argument: %s (use 'on' or 'off')", args[0])
	}

	return nil
}
