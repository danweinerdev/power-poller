package kasa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/protocol"
)

var (
	discoverTimeout time.Duration
	discoverJSON    bool
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover KASA devices on the network",
	Long: `Broadcast a discovery request to find all KASA devices on the local network.

This command sends a UDP broadcast and collects responses from all devices.
No --host flag is required for this command.`,
	Example: `  kasa-monitor kasa discover
  kasa-monitor kasa discover --timeout 10s
  kasa-monitor kasa discover --json`,
	RunE: runDiscover,
}

func init() {
	discoverCmd.Flags().DurationVar(&discoverTimeout, "timeout", 5*time.Second, "Discovery timeout")
	discoverCmd.Flags().BoolVar(&discoverJSON, "json", false, "Output as JSON")
}

func runDiscover(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	fmt.Printf("Discovering devices (timeout: %s)...\n\n", discoverTimeout)

	devices, err := protocol.Discover(ctx, discoverTimeout)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	if len(devices) == 0 {
		fmt.Println("No devices found")
		return nil
	}

	// Sort by IP address
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].IP < devices[j].IP
	})

	if discoverJSON {
		// Output as JSON
		output := make([]map[string]interface{}, len(devices))
		for i, dev := range devices {
			output[i] = map[string]interface{}{
				"ip":        dev.IP,
				"alias":     dev.SysInfo.Alias,
				"model":     dev.SysInfo.Model,
				"device_id": dev.SysInfo.DeviceID,
				"type":      dev.SysInfo.DetectDeviceType().String(),
				"mac":       dev.SysInfo.MAC,
			}
		}
		jsonOut, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(jsonOut))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "IP\tAlias\tModel\tType\tMAC")
	fmt.Fprintln(w, "──\t─────\t─────\t────\t───")
	for _, dev := range devices {
		devType := dev.SysInfo.DetectDeviceType().String()
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			dev.IP,
			dev.SysInfo.Alias,
			dev.SysInfo.Model,
			devType,
			dev.SysInfo.MAC,
		)
	}
	w.Flush()

	fmt.Printf("\nFound %d device(s)\n", len(devices))
	return nil
}
