package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/go-power-poller/internal/api"
	"github.com/danweinerdev/go-power-poller/internal/api/manager"
	"github.com/danweinerdev/go-power-poller/internal/config"
	"github.com/danweinerdev/go-power-poller/internal/daemon"
)

var (
	serveListenAddr string
	servePort       int
	serveAPIOnly    bool
	serveAuthUser   string
	serveAuthPass   string
	enableMetrics   bool
	echoMetrics     bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the REST API server",
	Long: `Start a REST API server with WebSocket support for controlling KASA devices.

The server exposes endpoints for device control, status, and discovery.
WebSocket connections receive real-time device state updates.

Optionally enable metrics collection to send data to InfluxDB/Prometheus
using --enable-metrics. This allows a single process to serve the web UI
and collect metrics.

Flags override config file values.`,
	Example: `  kasa-monitor serve -c config.toml
  kasa-monitor serve --port 8080 --listen 0.0.0.0
  kasa-monitor serve --api-only  # No static file serving
  kasa-monitor serve --auth-user admin --auth-pass secret
  kasa-monitor serve -c config.toml --enable-metrics  # With metrics
  kasa-monitor serve -c config.toml --enable-metrics --echo-metrics  # Debug`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().StringVar(&serveListenAddr, "listen", "", "listen address (overrides config)")
	serveCmd.Flags().IntVar(&servePort, "port", 0, "listen port (overrides config)")
	serveCmd.Flags().BoolVar(&serveAPIOnly, "api-only", false, "only serve API, no static files")
	serveCmd.Flags().StringVar(&serveAuthUser, "auth-user", "", "basic auth username")
	serveCmd.Flags().StringVar(&serveAuthPass, "auth-pass", "", "basic auth password")
	serveCmd.Flags().BoolVarP(&enableMetrics, "enable-metrics", "m", false, "enable metrics collection to backends")
	serveCmd.Flags().BoolVar(&echoMetrics, "echo-metrics", false, "output metrics to stdout (debug)")

	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	log := GetLogger()

	// Load configuration
	cfgPath := GetConfigFile()
	if cfgPath == "" {
		var err error
		cfgPath, err = config.FindConfigFile("config.toml")
		if err != nil {
			return fmt.Errorf("no config file specified and none found: %w", err)
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override config with CLI flags
	if serveListenAddr != "" {
		cfg.API.Listen = serveListenAddr
	}
	if servePort != 0 {
		cfg.API.Port = servePort
	}
	if serveAuthUser != "" && serveAuthPass != "" {
		cfg.API.AuthEnabled = true
		cfg.API.AuthUsername = serveAuthUser
		cfg.API.AuthPassword = serveAuthPass
	}

	// Apply defaults if not set
	if cfg.API.Listen == "" {
		cfg.API.Listen = "0.0.0.0"
	}
	if cfg.API.Port == 0 {
		cfg.API.Port = 8080
	}

	log.Info("loaded configuration", "path", cfgPath, "devices", len(cfg.Devices))

	// Set up signal handler
	sigHandler := daemon.NewSignalHandler(log)
	ctx := sigHandler.Start(context.Background())

	// Create metrics pipeline if enabled
	var collector *api.MetricsCollector
	if enableMetrics {
		pipeline, err := CreateMetricsPipeline(cfg, log, echoMetrics)
		if err != nil {
			return fmt.Errorf("failed to create metrics pipeline: %w", err)
		}

		if err := pipeline.Start(ctx); err != nil {
			return fmt.Errorf("failed to start metrics pipeline: %w", err)
		}
		defer func() {
			if err := pipeline.Stop(context.Background()); err != nil {
				log.Error("error stopping pipeline", "error", err)
			}
		}()

		// Collector will be started after manager
		collector = api.NewMetricsCollector(cfg, nil, pipeline, log)
	}

	// Create device manager
	mgr := manager.New(cfg, log)
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("failed to start device manager: %w", err)
	}
	defer mgr.Stop()

	// Start metrics collector if enabled (now that manager is started)
	if collector != nil {
		collector.SetManager(mgr)
		if err := collector.Start(ctx); err != nil {
			return fmt.Errorf("failed to start metrics collector: %w", err)
		}
		defer collector.Stop()
	}

	// Create and start API server
	serverOpts := []api.ServerOption{
		api.WithLogger(log),
		api.WithAPIOnly(serveAPIOnly),
	}

	srv := api.NewServer(cfg, mgr, serverOpts...)

	// Handle config reload
	go func() {
		for {
			select {
			case <-sigHandler.Reload():
				log.Info("reloading configuration")
				newCfg, err := config.Load(cfgPath)
				if err != nil {
					log.Error("failed to reload config", "error", err)
					continue
				}
				mgr.ReloadConfig(newCfg)
				if collector != nil {
					collector.ReloadConfig(newCfg)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	log.Info("starting API server",
		"listen", cfg.API.Listen,
		"port", cfg.API.Port,
		"auth", cfg.API.AuthEnabled,
		"metrics", enableMetrics)

	if err := srv.Run(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("server error: %w", err)
	}

	log.Info("server stopped")
	return nil
}
