package cli

import (
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/go-power-poller/internal/cli/kasa"
)

var (
	// Global flags
	cfgFile  string
	logLevel string
	debug    bool

	// Logger
	logger *slog.Logger
)

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "kasa-monitor",
	Short: "KASA device monitoring and control",
	Long: `KASA Monitor is a tool for monitoring and controlling TP-Link KASA smart devices.

It supports:
  - Polling devices for energy metrics
  - Sending data to InfluxDB and Prometheus
  - Device control (on/off, brightness, color)
  - Device discovery and status`,
	SilenceUsage: true, // Don't show usage on runtime errors
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initLogger()
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug mode")

	// Add kasa subcommand
	rootCmd.AddCommand(kasa.KasaCmd)
}

func initLogger() {
	level := slog.LevelInfo
	switch strings.ToLower(logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	if debug {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewTextHandler(os.Stderr, opts)
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// GetLogger returns the configured logger.
func GetLogger() *slog.Logger {
	if logger == nil {
		initLogger()
	}
	return logger
}

// GetConfigFile returns the config file path.
func GetConfigFile() string {
	return cfgFile
}
