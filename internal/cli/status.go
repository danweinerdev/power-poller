package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/device"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/protocol"
)

var (
	statusDevice  string
	statusTimeout time.Duration
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display device status",
	Long: `Display the status of a KASA device.

Shows device information, current state, and energy meter readings (if available).`,
	Example: `  kasa-monitor status --device 192.168.1.100
  kasa-monitor status -d 192.168.1.100 --timeout 10s`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().StringVarP(&statusDevice, "device", "d", "", "device IP address (required)")
	statusCmd.Flags().DurationVarP(&statusTimeout, "timeout", "t", 5*time.Second, "connection timeout")
	statusCmd.MarkFlagRequired("device")

	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	log := GetLogger()

	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()

	log.Debug("connecting to device", "address", statusDevice)

	opts := []protocol.TransportOption{
		protocol.WithTimeout(statusTimeout),
	}

	dev, err := device.Load(ctx, statusDevice, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to device: %w", err)
	}
	defer dev.Close()

	// Print device info
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "DEVICE INFO")
	fmt.Fprintln(w, "───────────────────────────────────────")
	fmt.Fprintf(w, "Alias:\t%s\n", dev.Alias())
	fmt.Fprintf(w, "Model:\t%s\n", dev.Model())
	fmt.Fprintf(w, "Type:\t%s\n", dev.Type())
	fmt.Fprintf(w, "Device ID:\t%s\n", dev.DeviceID())
	fmt.Fprintf(w, "MAC:\t%s\n", dev.MAC())
	fmt.Fprintf(w, "Host:\t%s\n", dev.Host())

	// Print state
	fmt.Fprintln(w)
	fmt.Fprintln(w, "STATE")
	fmt.Fprintln(w, "───────────────────────────────────────")

	state := "OFF"
	if dev.IsOn() {
		state = "ON"
	}
	fmt.Fprintf(w, "Power:\t%s\n", state)

	// Print bulb-specific info
	if dimmable, ok := dev.(device.Dimmable); ok && dimmable.IsDimmable() {
		fmt.Fprintf(w, "Brightness:\t%d%%\n", dimmable.Brightness())
	}

	if colorable, ok := dev.(device.Colorable); ok && colorable.IsColor() {
		h, s, b := colorable.HSV()
		fmt.Fprintf(w, "HSV:\t%d°, %d%%, %d%%\n", h, s, b)
	}

	if tempCtrl, ok := dev.(device.TemperatureControllable); ok && tempCtrl.IsVariableColorTemp() {
		fmt.Fprintf(w, "Color Temp:\t%dK\n", tempCtrl.ColorTemp())
		min, max := tempCtrl.ColorTempRange()
		fmt.Fprintf(w, "Temp Range:\t%d-%dK\n", min, max)
	}

	// Print emeter info
	if emeterDev, ok := dev.(device.EmeterDevice); ok && emeterDev.HasEmeter() {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "ENERGY METER")
		fmt.Fprintln(w, "───────────────────────────────────────")

		data, err := emeterDev.GetEmeterRealtime(ctx)
		if err != nil {
			fmt.Fprintf(w, "Error:\t%v\n", err)
		} else {
			fmt.Fprintf(w, "Voltage:\t%.1f V\n", data.Voltage)
			fmt.Fprintf(w, "Current:\t%.3f A\n", data.Current)
			fmt.Fprintf(w, "Power:\t%.1f W\n", data.Power)
			fmt.Fprintf(w, "Total:\t%.3f kWh\n", data.Total)
		}
	}

	// Print children info for power strips
	if parentDev, ok := dev.(device.ParentDevice); ok && parentDev.HasChildren() {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "OUTLETS")
		fmt.Fprintln(w, "───────────────────────────────────────")

		children := parentDev.Children()
		for i, child := range children {
			state := "OFF"
			if child.IsOn() {
				state = "ON"
			}
			fmt.Fprintf(w, "[%d] %s:\t%s\n", i, child.Alias(), state)
		}
	}

	w.Flush()
	return nil
}
