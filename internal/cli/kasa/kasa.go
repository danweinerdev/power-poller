package kasa

import (
	"context"
	"fmt"
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

Use --host to specify the device IP address for most subcommands.
Some commands like 'discover' do not require --host.`,
}

func init() {
	KasaCmd.PersistentFlags().StringVarP(&host, "host", "H", "", "device IP address")
	KasaCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "t", 5*time.Second, "connection timeout")

	// Commands that require --host
	hostRequiredCmds := []*cobra.Command{
		infoCmd,
		stateCmd,
		onCmd,
		offCmd,
		toggleCmd,
		emeterCmd,
		brightnessCmd,
		hsvCmd,
		tempCmd,
		aliasCmd,
		rebootCmd,
		ledCmd,
		wifiCmd,
		timeCmd,
		sysinfoCmd,
		scheduleCmd,
		countdownCmd,
		cloudCmd,
		firmwareCmd,
	}

	for _, cmd := range hostRequiredCmds {
		KasaCmd.AddCommand(cmd)
		// Mark host as required for this specific command
		cmd.PreRunE = requireHost(cmd.PreRunE)
	}

	// Commands that don't require --host
	KasaCmd.AddCommand(newTestCommand()) // test has its own host validation
	KasaCmd.AddCommand(discoverCmd)      // discover uses broadcast, no host needed
}

// requireHost wraps a PreRunE to validate that --host is set.
func requireHost(existing func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if host == "" {
			return fmt.Errorf("required flag \"host\" not set")
		}
		if existing != nil {
			return existing(cmd, args)
		}
		return nil
	}
}

// loadDevice connects to a device and returns it.
func loadDevice(ctx context.Context) (device.Device, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	opts := []device.LoadOption{
		device.WithTransportOptions(protocol.WithTimeout(timeout)),
		device.WithKLAPOptions(protocol.WithKLAPTimeout(timeout)),
	}

	return device.Load(ctx, host, opts...)
}
