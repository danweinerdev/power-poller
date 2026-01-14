package kasa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/command"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/types"
)

var firmwareCmd = &cobra.Command{
	Use:   "firmware",
	Short: "Firmware management commands",
	Long:  `Check for firmware updates, download, and flash firmware.`,
}

var firmwareCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for available firmware updates",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dev, err := loadDevice(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer dev.Close()

		// Display current firmware version
		sysInfo := dev.SysInfo()
		if sysInfo != nil && sysInfo.SWVersion != "" {
			fmt.Printf("Current firmware: %s\n", sysInfo.SWVersion)
		}

		resp, err := dev.SendCommand(ctx, command.GetAvailableFirmwares())
		if err != nil {
			return fmt.Errorf("failed to check for updates: %w", err)
		}

		var result types.AvailableFirmwareResponse
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		fwResult := result.System.GetAvailableFirmwares
		if fwResult.ErrCode != 0 {
			return fmt.Errorf("device error: %s (code %d)", fwResult.ErrMsg, fwResult.ErrCode)
		}

		if len(fwResult.FWList) == 0 {
			fmt.Println("No firmware updates available")
			return nil
		}

		fmt.Printf("\n%d firmware update(s) available:\n\n", len(fwResult.FWList))
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "VERSION\tRELEASE DATE\n")
		for _, fw := range fwResult.FWList {
			fmt.Fprintf(w, "%s\t%s\n", fw.FWVer, fw.ReleaseDate)
			if fw.ReleaseNote != "" {
				fmt.Printf("  Notes: %s\n", fw.ReleaseNote)
			}
		}
		w.Flush()

		return nil
	},
}

var firmwareStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get firmware download status",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dev, err := loadDevice(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer dev.Close()

		resp, err := dev.SendCommand(ctx, command.GetDownloadState())
		if err != nil {
			return fmt.Errorf("failed to get download status: %w", err)
		}

		var result types.DownloadStateResponse
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		state := result.System.GetDownloadState
		if state.ErrCode != 0 {
			return fmt.Errorf("device error: %s (code %d)", state.ErrMsg, state.ErrCode)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Status:\t%s\n", state.StatusString())
		if state.Status == 1 {
			fmt.Fprintf(w, "Progress:\t%d%%\n", state.Ratio)
		}
		w.Flush()

		return nil
	},
}

var firmwareUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Download and flash the latest firmware",
	Long: `Download and flash the latest available firmware.

WARNING: This will update your device firmware. The device will reboot
during the update process. Do not interrupt power during the update.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dev, err := loadDevice(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer dev.Close()

		// First, check for available updates
		resp, err := dev.SendCommand(ctx, command.GetAvailableFirmwares())
		if err != nil {
			return fmt.Errorf("failed to check for updates: %w", err)
		}

		var fwResult types.AvailableFirmwareResponse
		if err := json.Unmarshal(resp, &fwResult); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		if fwResult.System.GetAvailableFirmwares.ErrCode != 0 {
			return fmt.Errorf("device error: %s (code %d)",
				fwResult.System.GetAvailableFirmwares.ErrMsg,
				fwResult.System.GetAvailableFirmwares.ErrCode)
		}

		fwList := fwResult.System.GetAvailableFirmwares.FWList
		if len(fwList) == 0 {
			fmt.Println("No firmware updates available")
			return nil
		}

		// Get the latest firmware
		latest := fwList[0]
		fmt.Printf("Downloading firmware %s...\n", latest.FWVer)

		downloadURL := latest.DownloadURL
		if downloadURL == "" {
			downloadURL = latest.ObjURL
		}

		if downloadURL == "" {
			return fmt.Errorf("no download URL available for firmware")
		}

		// Start download
		resp, err = dev.SendCommand(ctx, command.DownloadFirmware(downloadURL))
		if err != nil {
			return fmt.Errorf("failed to start download: %w", err)
		}

		var dlResult types.DownloadFirmwareResponse
		if err := json.Unmarshal(resp, &dlResult); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		if dlResult.System.DownloadFirmware.ErrCode != 0 {
			return fmt.Errorf("download failed: %s (code %d)",
				dlResult.System.DownloadFirmware.ErrMsg,
				dlResult.System.DownloadFirmware.ErrCode)
		}

		fmt.Println("Firmware download initiated.")
		fmt.Println("Use 'kasa firmware status' to check download progress.")
		fmt.Println("\nOnce download is complete, the device may automatically flash the firmware.")
		fmt.Println("Do not interrupt power during the update process.")

		return nil
	},
}

func init() {
	firmwareCmd.AddCommand(firmwareCheckCmd)
	firmwareCmd.AddCommand(firmwareStatusCmd)
	firmwareCmd.AddCommand(firmwareUpdateCmd)
}
