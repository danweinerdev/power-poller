package kasa

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/command"
)

var sysinfoCmd = &cobra.Command{
	Use:   "sysinfo",
	Short: "Display raw device system info",
	Long: `Display the full raw system information from the device as JSON.

This is useful for debugging or examining device capabilities not exposed
through other commands.`,
	RunE: runSysinfo,
}

func runSysinfo(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dev, err := loadDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer dev.Close()

	// Get raw sysinfo response
	resp, err := dev.SendCommand(ctx, command.GetSysInfo())
	if err != nil {
		return fmt.Errorf("failed to get sysinfo: %w", err)
	}

	// Pretty-print the JSON
	var prettyJSON map[string]interface{}
	if err := json.Unmarshal(resp, &prettyJSON); err != nil {
		// If it doesn't parse, just print raw
		fmt.Println(string(resp))
		return nil
	}

	output, err := json.MarshalIndent(prettyJSON, "", "  ")
	if err != nil {
		fmt.Println(string(resp))
		return nil
	}

	fmt.Println(string(output))
	return nil
}
