package kasa

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/device"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/protocol"
)

var (
	host    string
	timeout time.Duration
)

// KasaCmd is the parent command for device control.
var KasaCmd = &cobra.Command{
	Use:   "kasa",
	Short: "KASA device control commands",
	Long: `Control KASA smart devices directly.

Use --host to specify the device IP address for all subcommands.`,
}

func init() {
	KasaCmd.PersistentFlags().StringVarP(&host, "host", "H", "", "device IP address (required)")
	KasaCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "t", 5*time.Second, "connection timeout")
	KasaCmd.MarkPersistentFlagRequired("host")

	// Add subcommands
	KasaCmd.AddCommand(infoCmd)
	KasaCmd.AddCommand(stateCmd)
	KasaCmd.AddCommand(onCmd)
	KasaCmd.AddCommand(offCmd)
	KasaCmd.AddCommand(toggleCmd)
	KasaCmd.AddCommand(emeterCmd)
	KasaCmd.AddCommand(brightnessCmd)
	KasaCmd.AddCommand(hsvCmd)
	KasaCmd.AddCommand(tempCmd)
	KasaCmd.AddCommand(aliasCmd)
	KasaCmd.AddCommand(rebootCmd)
	KasaCmd.AddCommand(ledCmd)
	KasaCmd.AddCommand(newTestCommand())
}

// loadDevice connects to a device and returns it.
func loadDevice(ctx context.Context) (device.Device, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	opts := []protocol.TransportOption{
		protocol.WithTimeout(timeout),
	}

	return device.Load(ctx, host, opts...)
}
