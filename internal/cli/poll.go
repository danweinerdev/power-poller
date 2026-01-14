package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/danweinerdev/go-power-poller/internal/backend"
	"github.com/danweinerdev/go-power-poller/internal/config"
	"github.com/danweinerdev/go-power-poller/internal/daemon"
	"github.com/danweinerdev/go-power-poller/internal/metrics"
	"github.com/danweinerdev/go-power-poller/internal/poller"
)

var (
	foreground    bool
	pidFile       string
	echoMode      bool
	ignoreUnknown bool
	runFor        int
	runOnce       bool
)

var pollCmd = &cobra.Command{
	Use:   "poll",
	Short: "Start the polling daemon",
	Long: `Start the KASA device polling daemon.

This command runs continuously, polling configured devices at the specified
interval and sending metrics to configured backends (InfluxDB, Prometheus).

Use -o/--foreground to run in foreground mode (no daemonization).
Use --echo to output metrics to stdout instead of backends (for debugging).
Use --ignore-unknown to skip devices that are reachable but have unsupported protocols.
Use --run-for=N to run for N polling iterations then exit.
Use --run-once to run a single polling iteration then exit (same as --run-for=1).`,
	RunE: runPoll,
}

func init() {
	pollCmd.Flags().BoolVarP(&foreground, "foreground", "o", false, "run in foreground (no daemonize)")
	pollCmd.Flags().StringVar(&pidFile, "pid-file", "", "PID file path")
	pollCmd.Flags().BoolVar(&echoMode, "echo", false, "echo metrics to stdout (debug mode)")
	pollCmd.Flags().BoolVar(&ignoreUnknown, "ignore-unknown", false, "ignore devices with unrecognized protocols")
	pollCmd.Flags().IntVar(&runFor, "run-for", 0, "run for N iterations then exit (0 = run indefinitely)")
	pollCmd.Flags().BoolVar(&runOnce, "run-once", false, "run a single iteration then exit (same as --run-for=1)")

	rootCmd.AddCommand(pollCmd)
}

func runPoll(cmd *cobra.Command, args []string) error {
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

	log.Info("loaded configuration", "path", cfgPath, "devices", len(cfg.Devices))

	// Set up PID file
	pid := daemon.NewPIDFile(pidFile)
	if running, existingPid := pid.IsRunning(); running {
		return fmt.Errorf("daemon already running (PID %d)", existingPid)
	}
	if err := pid.Write(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer pid.Remove()

	// Set up signal handler
	sigHandler := daemon.NewSignalHandler(log)
	ctx := sigHandler.Start(context.Background())

	// Create pipeline
	pipelineCfg := metrics.PipelineConfig{
		BatchSize:     cfg.Global.BatchSize,
		FlushInterval: cfg.Global.PollInterval.Duration,
		RetryAttempts: cfg.Global.RetryAttempts,
		RetryDelay:    cfg.Global.RetryDelay.Duration,
		Logger:        log,
	}
	pipeline := metrics.NewPipeline(pipelineCfg)

	// Add backends
	if echoMode {
		pipeline.AddBackend(backend.NewEchoStdout(log))
	} else {
		if cfg.InfluxDB.Enabled {
			pipeline.AddBackend(backend.NewInfluxDB(cfg.InfluxDB, log))
		}
		if cfg.Prometheus.Enabled {
			pipeline.AddBackend(backend.NewPrometheus(cfg.Prometheus, log))
		}
	}

	if pipeline.BackendCount() == 0 {
		return fmt.Errorf("no backends configured (enable influxdb, prometheus, or use --echo)")
	}

	// Start pipeline
	if err := pipeline.Start(ctx); err != nil {
		return fmt.Errorf("failed to start metrics pipeline: %w", err)
	}
	defer func() {
		if err := pipeline.Stop(context.Background()); err != nil {
			log.Error("error stopping pipeline", "error", err)
		}
	}()

	// Determine max iterations
	maxIterations := runFor
	if runOnce {
		maxIterations = 1
	}

	// Create and run poller
	pollerOpts := []func(*poller.Options){
		poller.WithIgnoreUnknownDevices(ignoreUnknown),
	}
	if maxIterations > 0 {
		pollerOpts = append(pollerOpts, poller.WithMaxIterations(maxIterations))
	}
	p := poller.New(cfg, pipeline, log, pollerOpts...)

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
				p.ReloadConfig(newCfg)
			case <-ctx.Done():
				return
			}
		}
	}()

	log.Info("starting poller")
	if err := p.Run(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("poller error: %w", err)
	}

	log.Info("poller stopped")
	return nil
}

// pollVersion prints version info
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("kasa-monitor version 1.0.0")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
