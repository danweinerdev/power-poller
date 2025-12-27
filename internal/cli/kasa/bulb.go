package kasa

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/device"
)

var brightnessCmd = &cobra.Command{
	Use:   "brightness [level]",
	Short: "Get or set bulb brightness",
	Long: `Get or set bulb brightness level (0-100).

Without arguments, displays current brightness.
With an argument, sets brightness to the specified level.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runBrightness,
}

func runBrightness(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	dimmable, ok := dev.(device.Dimmable)
	if !ok {
		return fmt.Errorf("device does not support brightness control")
	}

	if !dimmable.IsDimmable() {
		return fmt.Errorf("device is not dimmable")
	}

	if len(args) == 0 {
		fmt.Printf("Brightness: %d%%\n", dimmable.Brightness())
		return nil
	}

	var level int
	if _, err := fmt.Sscanf(args[0], "%d", &level); err != nil {
		return fmt.Errorf("invalid brightness value: %s", args[0])
	}

	if err := dimmable.SetBrightness(ctx, level); err != nil {
		return fmt.Errorf("failed to set brightness: %w", err)
	}

	fmt.Printf("Brightness set to %d%%\n", level)
	return nil
}

var hsvCmd = &cobra.Command{
	Use:   "hsv [hue saturation brightness]",
	Short: "Get or set bulb HSV color",
	Long: `Get or set bulb color using HSV values.

Without arguments, displays current HSV values.
With arguments, sets color to the specified values:
  hue:        0-360 (degrees)
  saturation: 0-100 (percent)
  brightness: 0-100 (percent)`,
	Args: cobra.MaximumNArgs(3),
	RunE: runHSV,
}

func runHSV(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	colorable, ok := dev.(device.Colorable)
	if !ok {
		return fmt.Errorf("device does not support color control")
	}

	if !colorable.IsColor() {
		return fmt.Errorf("device does not support color")
	}

	if len(args) == 0 {
		h, s, b := colorable.HSV()
		fmt.Printf("Hue: %d°, Saturation: %d%%, Brightness: %d%%\n", h, s, b)
		return nil
	}

	if len(args) != 3 {
		return fmt.Errorf("hsv requires exactly 3 arguments: hue saturation brightness")
	}

	var h, s, b int
	if _, err := fmt.Sscanf(args[0], "%d", &h); err != nil {
		return fmt.Errorf("invalid hue value: %s", args[0])
	}
	if _, err := fmt.Sscanf(args[1], "%d", &s); err != nil {
		return fmt.Errorf("invalid saturation value: %s", args[1])
	}
	if _, err := fmt.Sscanf(args[2], "%d", &b); err != nil {
		return fmt.Errorf("invalid brightness value: %s", args[2])
	}

	if err := colorable.SetHSV(ctx, h, s, b); err != nil {
		return fmt.Errorf("failed to set HSV: %w", err)
	}

	fmt.Printf("HSV set to %d°, %d%%, %d%%\n", h, s, b)
	return nil
}

var tempCmd = &cobra.Command{
	Use:   "temperature [kelvin]",
	Short: "Get or set bulb color temperature",
	Long: `Get or set bulb color temperature in Kelvin.

Without arguments, displays current color temperature and range.
With an argument, sets color temperature to the specified value.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTemperature,
}

func runTemperature(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	tempCtrl, ok := dev.(device.TemperatureControllable)
	if !ok {
		return fmt.Errorf("device does not support color temperature control")
	}

	if !tempCtrl.IsVariableColorTemp() {
		return fmt.Errorf("device does not support variable color temperature")
	}

	if len(args) == 0 {
		current := tempCtrl.ColorTemp()
		min, max := tempCtrl.ColorTempRange()
		fmt.Printf("Color Temperature: %dK (range: %d-%dK)\n", current, min, max)
		return nil
	}

	var temp int
	if _, err := fmt.Sscanf(args[0], "%d", &temp); err != nil {
		return fmt.Errorf("invalid temperature value: %s", args[0])
	}

	if err := tempCtrl.SetColorTemp(ctx, temp); err != nil {
		return fmt.Errorf("failed to set color temperature: %w", err)
	}

	fmt.Printf("Color temperature set to %dK\n", temp)
	return nil
}
