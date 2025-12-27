# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

KASA Monitor is a Go CLI tool for monitoring and controlling TP-Link KASA smart home devices (plugs, bulbs, light strips, power strips). The project supports device discovery, control, and monitoring with energy meter (emeter) data collection for InfluxDB and Prometheus backends.

## Build Commands

```bash
# Build the binary
make build

# Run all tests
make test
go test ./...

# Run tests with verbose output
make test-verbose

# Run tests with coverage
make test-coverage

# Format code
make fmt
go fmt ./...

# Lint code
make lint
go vet ./...

# Clean build artifacts
make clean

# Build Docker image
make docker-build
```

## Code Structure

```
kasa-monitor/
├── cmd/
│   └── kasa-monitor/
│       └── main.go                 # CLI entry point
├── internal/
│   ├── cli/                        # Cobra CLI commands
│   │   ├── root.go                 # Root command, global flags
│   │   ├── poll.go                 # Poll subcommand (daemon mode)
│   │   ├── status.go               # Status subcommand
│   │   └── kasa/                   # Device control commands
│   │       ├── kasa.go             # Kasa command group
│   │       ├── info.go             # info/state/on/off/toggle
│   │       ├── emeter.go           # Energy meter commands
│   │       ├── bulb.go             # brightness/hsv/temperature
│   │       └── device.go           # alias/reboot/led
│   ├── config/                     # TOML configuration
│   │   ├── config.go               # Config loading
│   │   ├── types.go                # Config structs
│   │   └── validation.go           # Config validation
│   ├── poller/                     # Polling system
│   │   ├── poller.go               # Main polling loop
│   │   └── worker.go               # Concurrent device polling
│   ├── metrics/                    # Metrics system
│   │   ├── metric.go               # Metric type
│   │   ├── pipeline.go             # Batching and queuing
│   │   └── collector.go            # Prometheus collector
│   ├── backend/                    # Storage backends
│   │   ├── backend.go              # Backend interface
│   │   ├── influxdb.go             # InfluxDB 2.x
│   │   ├── prometheus.go           # Prometheus exporter
│   │   └── echo.go                 # Debug output
│   └── daemon/                     # Daemon utilities
│       ├── daemon.go               # PID file management
│       └── signals.go              # Signal handling
├── pkg/
│   ├── kasa/                       # Public device library
│   │   ├── protocol/
│   │   │   ├── encrypt.go          # XOR cipher
│   │   │   ├── message.go          # Message framing
│   │   │   └── transport.go        # TCP communication
│   │   ├── device/
│   │   │   ├── device.go           # Device interface
│   │   │   ├── plug.go             # Smart plug
│   │   │   ├── bulb.go             # Smart bulb
│   │   │   ├── lightstrip.go       # Light strip
│   │   │   ├── powerstrip.go       # Power strip with children
│   │   │   ├── discovery.go        # Device type detection
│   │   │   └── errors.go           # Error definitions
│   │   ├── command/
│   │   │   ├── command.go          # Command builders base
│   │   │   ├── system.go           # System commands
│   │   │   ├── emeter.go           # Energy meter commands
│   │   │   └── lighting.go         # Bulb lighting commands
│   │   └── types/
│   │       ├── sysinfo.go          # SysInfo types
│   │       ├── emeter.go           # Emeter types
│   │       └── lighting.go         # Light state types
│   └── mockdevice/                 # Mock device for testing
│       ├── device.go               # TCP server mock
│       └── options.go              # Configuration options
├── config/
│   └── example.toml                # Example configuration
└── go.mod
```

## Key Architecture Patterns

### Device Protocol
- XOR cipher with rolling key (initial key 0xAB)
- 4-byte big-endian length header + encrypted JSON payload
- TCP port 9999 (default), configurable timeout

### Device Communication Flow
1. `device.Load(ctx, address)` creates appropriate device type based on sysinfo
2. Device connects and calls `Update()` to fetch current state
3. Commands are sent via `SendCommand()` which handles encryption/framing
4. Responses are decrypted and parsed

### Polling Architecture
The `poll` command runs a polling loop that:
1. Creates a `Worker` which polls all configured devices concurrently
2. Each device poll fetches emeter data and creates `Metric` objects
3. Metrics are pushed to the `Pipeline` which batches and sends to backends
4. Signal handling: SIGINT/SIGTERM for shutdown, SIGHUP for config reload

### Configuration (TOML)
```toml
[global]
poll_interval = "10s"
log_level = "info"
device_timeout = "5s"

[influxdb]
enabled = true
server = "localhost"
port = 8086
token = "..."
org = "myorg"
bucket = "kasa"

[prometheus]
enabled = true
port = 9090
path = "/metrics"

[devices.office_plug]
address = "192.168.1.100"
measurements = ["power_metrics"]
tags = { location = "office" }

[devices.power_strip]
address = "192.168.1.101"
has_children = true
poll_parent = true

[devices.power_strip.children.outlet_0]
index = 0
name = "server"
measurements = ["power_metrics"]

[measurements.power_metrics.fields]
voltage = "float"
current = "float"
power = "float"
total = "float"
```

## CLI Commands

```bash
# Run polling daemon
kasa-monitor poll -c config.toml

# Run in foreground with echo output (debug)
kasa-monitor poll -o --echo -c config.toml

# Check device status
kasa-monitor status -d 192.168.1.100

# Device control
kasa-monitor kasa -H 192.168.1.100 info
kasa-monitor kasa -H 192.168.1.100 on
kasa-monitor kasa -H 192.168.1.100 off
kasa-monitor kasa -H 192.168.1.100 toggle
kasa-monitor kasa -H 192.168.1.100 emeter

# Bulb control
kasa-monitor kasa -H 192.168.1.101 brightness 75
kasa-monitor kasa -H 192.168.1.101 hsv 180 50 80
kasa-monitor kasa -H 192.168.1.101 temperature 3000
```

## Dependencies

- github.com/spf13/cobra (CLI)
- github.com/BurntSushi/toml (config)
- github.com/influxdata/influxdb-client-go/v2
- github.com/prometheus/client_golang
- log/slog (standard library - structured logging)

## Docker Usage

```bash
# Build container
docker build -f Containerfile -t kasa-monitor:latest .

# Run in polling mode with config
docker run -it -v $PWD/config/example.toml:/etc/kasa-monitor/config.toml:ro kasa-monitor:latest

# Device status check
docker run -it kasa-monitor:latest status -d 10.0.0.100
```
