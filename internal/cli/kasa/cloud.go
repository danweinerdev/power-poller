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

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Cloud connection commands",
}

var cloudInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display cloud connection status",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dev, err := loadDevice(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer dev.Close()

		resp, err := dev.SendCommand(ctx, command.GetCloudInfo())
		if err != nil {
			return fmt.Errorf("failed to get cloud info: %w", err)
		}

		var result types.CloudInfoResponse
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("failed to parse cloud info: %w", err)
		}

		info := result.CloudInfo.GetInfo
		if info.ErrCode != 0 {
			return fmt.Errorf("device error: %s (code %d)", info.ErrMsg, info.ErrCode)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Cloud Bound:\t%s\n", boolString(info.IsBound()))
		fmt.Fprintf(w, "Cloud Connected:\t%s\n", boolString(info.IsConnected()))
		if info.Username != "" {
			fmt.Fprintf(w, "Account:\t%s\n", info.Username)
		}
		if info.Server != "" {
			fmt.Fprintf(w, "Server:\t%s\n", info.Server)
		}
		w.Flush()
		return nil
	},
}

func boolString(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func init() {
	cloudCmd.AddCommand(cloudInfoCmd)
}
