package kasa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/command"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/types"
)

var wifiCmd = &cobra.Command{
	Use:   "wifi",
	Short: "WiFi configuration commands",
	Long: `Commands for managing device WiFi settings.

Subcommands:
  scan    - Scan for available WiFi networks
  join    - Connect device to a WiFi network
  status  - Show current WiFi connection info`,
}

var wifiScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for available WiFi networks",
	Long: `Scan for available WiFi networks in range of the device.

Shows SSID, signal strength, and security type for each network.`,
	RunE: runWifiScan,
}

var (
	wifiSSID     string
	wifiPassword string
	wifiKeyType  int
)

var wifiJoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Connect device to a WiFi network",
	Long: `Connect the device to a specified WiFi network.

WARNING: This will disconnect the device from its current network.
Make sure you can access the device on the new network.

Key types:
  0 - Open (no security)
  1 - WEP
  2 - WPA-PSK
  3 - WPA2-PSK (most common, default)`,
	Example: `  kasa-monitor kasa -H 192.168.0.1 wifi join --ssid "MyNetwork" --password "secret"
  kasa-monitor kasa -H 192.168.0.1 wifi join --ssid "OpenNetwork" --keytype 0`,
	RunE: runWifiJoin,
}

var wifiStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current WiFi connection info",
	Long:  `Display the device's current WiFi connection information.`,
	RunE:  runWifiStatus,
}

func init() {
	// Join command flags
	wifiJoinCmd.Flags().StringVar(&wifiSSID, "ssid", "", "WiFi network name (required)")
	wifiJoinCmd.Flags().StringVar(&wifiPassword, "password", "", "WiFi password")
	wifiJoinCmd.Flags().IntVar(&wifiKeyType, "keytype", types.KeyTypeWPA2, "Security type (0=open, 1=WEP, 2=WPA, 3=WPA2)")
	wifiJoinCmd.MarkFlagRequired("ssid")

	// Add subcommands
	wifiCmd.AddCommand(wifiScanCmd)
	wifiCmd.AddCommand(wifiJoinCmd)
	wifiCmd.AddCommand(wifiStatusCmd)
}

func runWifiScan(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	// Send scan command
	resp, err := dev.SendCommand(ctx, command.GetScanInfo(true))
	if err != nil {
		return fmt.Errorf("failed to scan: %w", err)
	}

	var result types.ScanInfoResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	scanInfo := result.Netif.GetScaninfo
	if scanInfo.ErrCode != 0 {
		return fmt.Errorf("scan failed with error code: %d", scanInfo.ErrCode)
	}

	if len(scanInfo.APList) == 0 {
		fmt.Println("No WiFi networks found")
		return nil
	}

	// Sort by signal strength (strongest first)
	sort.Slice(scanInfo.APList, func(i, j int) bool {
		return scanInfo.APList[i].RSSI > scanInfo.APList[j].RSSI
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SSID\tSecurity\tSignal\tRSSI")
	fmt.Fprintln(w, "────\t────────\t──────\t────")
	for _, ap := range scanInfo.APList {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d dBm\n",
			ap.SSID,
			ap.KeyTypeName(),
			ap.SignalQuality(),
			ap.RSSI,
		)
	}
	w.Flush()

	fmt.Printf("\nFound %d networks\n", len(scanInfo.APList))
	return nil
}

func runWifiJoin(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	// Validate key type
	if wifiKeyType < 0 || wifiKeyType > 3 {
		return fmt.Errorf("invalid key type: %d (must be 0-3)", wifiKeyType)
	}

	// Warn about open networks without password
	if wifiKeyType == types.KeyTypeOpen && wifiPassword != "" {
		fmt.Println("Warning: Password provided but key type is Open (0). Password will be ignored.")
	}

	// Warn about secured networks without password
	if wifiKeyType != types.KeyTypeOpen && wifiPassword == "" {
		return fmt.Errorf("password required for secured networks (key type %d)", wifiKeyType)
	}

	keyTypeName := types.AccessPoint{KeyType: wifiKeyType}.KeyTypeName()
	fmt.Printf("Connecting to '%s' (%s)...\n", wifiSSID, keyTypeName)
	fmt.Println("WARNING: Device will disconnect and reconnect on the new network.")

	// Send join command
	resp, err := dev.SendCommand(ctx, command.SetStaInfo(wifiSSID, wifiPassword, wifiKeyType))
	if err != nil {
		return fmt.Errorf("failed to join network: %w", err)
	}

	var result types.SetStaInfoResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Netif.SetStainfo.ErrCode != 0 {
		return fmt.Errorf("join failed with error code: %d", result.Netif.SetStainfo.ErrCode)
	}

	fmt.Println("Join command sent successfully.")
	fmt.Println("The device will now attempt to connect to the new network.")
	fmt.Println("You may need to reconnect to the device using its new IP address.")
	return nil
}

func runWifiStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	sysinfo := dev.SysInfo()
	if sysinfo == nil {
		return fmt.Errorf("failed to get device info")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "WIFI STATUS")
	fmt.Fprintln(w, "───────────")
	fmt.Fprintf(w, "SSID:\t%s\n", sysinfo.SSID)
	fmt.Fprintf(w, "RSSI:\t%d dBm\n", sysinfo.RSSI)

	// Determine signal quality from RSSI
	ap := types.AccessPoint{RSSI: sysinfo.RSSI}
	fmt.Fprintf(w, "Signal:\t%s\n", ap.SignalQuality())

	fmt.Fprintf(w, "MAC:\t%s\n", sysinfo.MAC)
	w.Flush()

	return nil
}
