# KASA Monitor

A Go CLI tool for monitoring and controlling TP-Link KASA smart home devices.

## Features

- **Device Support**: Smart plugs, bulbs, light strips, and power strips with individual outlet control
- **Real-time Monitoring**: Energy meter (emeter) data collection with configurable polling
- **Multiple Backends**: InfluxDB 2.x and Prometheus metrics export
- **Device Control**: Full control via CLI (on/off, brightness, color, temperature)
- **Concurrent Polling**: Efficient parallel device polling with batched metric delivery
- **Docker Support**: Run as a containerized service

## Quick Start

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd kasa-monitor

# Build the binary
make build

# Or build directly with Go
go build -o kasa-monitor ./cmd/kasa-monitor
```

### Basic Usage

```bash
# Check device status
kasa-monitor status -d 192.168.1.100

# Device control
kasa-monitor kasa -H 192.168.1.100 info
kasa-monitor kasa -H 192.168.1.100 on
kasa-monitor kasa -H 192.168.1.100 off
kasa-monitor kasa -H 192.168.1.100 toggle

# Energy meter readings
kasa-monitor kasa -H 192.168.1.100 emeter

# Bulb control
kasa-monitor kasa -H 192.168.1.100 brightness 75
kasa-monitor kasa -H 192.168.1.100 hsv 180 50 80
kasa-monitor kasa -H 192.168.1.100 temperature 3000

# Discover devices on the network
kasa-monitor kasa discover

# Run continuous monitoring
kasa-monitor poll -c config.toml
```

## Configuration

Create a TOML configuration file for continuous monitoring. See `config/example.toml` for a complete template:

```toml
[global]
poll_interval = "10s"
log_level = "info"
device_timeout = "5s"
batch_size = 10
retry_attempts = 3
retry_delay = "1s"

[influxdb]
enabled = true
server = "localhost"
port = 8086
token = "your-influxdb-token"
org = "myorg"
bucket = "kasa"
tls = false

[prometheus]
enabled = true
port = 9090
path = "/metrics"

[devices.office_plug]
address = "192.168.1.100"
measurements = ["power_metrics"]
tags = { location = "office" }

[measurements.power_metrics.fields]
voltage = "float"
current = "float"
power = "float"
total = "float"
```

### Power Strips with Child Outlets

KASA Monitor supports power strips with individual outlet monitoring:

```toml
[devices.server_rack]
address = "192.168.1.102"
has_children = true
poll_parent = true
measurements = ["power_metrics"]
tags = { location = "server_room" }

[devices.server_rack.children.outlet_0]
index = 0
name = "router"
measurements = ["power_metrics"]
tags = { device_type = "network" }

[devices.server_rack.children.outlet_1]
index = 1
name = "switch"
measurements = ["power_metrics"]
tags = { device_type = "network" }
```

## Command Reference

### Poll (Daemon Mode)

Continuously poll devices and send metrics to configured backends:

```bash
# Run in foreground
kasa-monitor poll -c config.toml

# Run in foreground with echo output (debug)
kasa-monitor poll -o --echo -c config.toml

# Run with debug logging
kasa-monitor poll -c config.toml -l debug
```

### Status

Check device status:

```bash
kasa-monitor status -d 192.168.1.100
```

### Device Control

The `kasa` subcommand provides full device control:

```bash
# Device information
kasa-monitor kasa -H 192.168.1.100 info
kasa-monitor kasa -H 192.168.1.100 sysinfo
kasa-monitor kasa -H 192.168.1.100 state

# Power control
kasa-monitor kasa -H 192.168.1.100 on
kasa-monitor kasa -H 192.168.1.100 off
kasa-monitor kasa -H 192.168.1.100 toggle

# Energy meter
kasa-monitor kasa -H 192.168.1.100 emeter

# Bulb brightness (0-100)
kasa-monitor kasa -H 192.168.1.100 brightness 75

# Bulb HSV color (hue 0-360, saturation 0-100, value 0-100)
kasa-monitor kasa -H 192.168.1.100 hsv 180 50 80

# Bulb color temperature (Kelvin)
kasa-monitor kasa -H 192.168.1.100 temperature 3000

# Device management
kasa-monitor kasa -H 192.168.1.100 alias "Office Lamp"
kasa-monitor kasa -H 192.168.1.100 led on
kasa-monitor kasa -H 192.168.1.100 reboot

# Discovery
kasa-monitor kasa discover
kasa-monitor kasa discover --timeout 10s
```

## Docker Usage

### Build Container

```bash
docker build -f Containerfile -t kasa-monitor:latest .
# Or use make
make docker-build
```

### Run Container

```bash
# Continuous monitoring with config file
docker run -d \
  -v $PWD/config/example.toml:/etc/kasa-monitor/config.toml:ro \
  --name kasa-monitor \
  kasa-monitor:latest

# Device status check
docker run -it kasa-monitor:latest status -d 192.168.1.100

# Device control
docker run -it kasa-monitor:latest kasa -H 192.168.1.100 info
```

## Architecture

### Package Structure

```
kasa-monitor/
├── cmd/kasa-monitor/           # CLI entry point
├── internal/
│   ├── cli/                    # Cobra CLI commands
│   │   ├── root.go             # Root command, global flags
│   │   ├── poll.go             # Poll subcommand (daemon mode)
│   │   ├── status.go           # Status subcommand
│   │   └── kasa/               # Device control commands
│   ├── config/                 # TOML configuration
│   ├── poller/                 # Polling system
│   ├── metrics/                # Metrics pipeline
│   ├── backend/                # Storage backends (InfluxDB, Prometheus)
│   └── daemon/                 # Daemon utilities
└── pkg/
    ├── kasa/                   # Public device library
    │   ├── protocol/           # XOR cipher, message framing, TCP transport
    │   ├── device/             # Device types (plug, bulb, strip)
    │   ├── command/            # Command builders
    │   └── types/              # Data types
    └── mockdevice/             # Mock device for testing
```

### Device Protocol

- XOR cipher with rolling key (initial key 0xAB)
- 4-byte big-endian length header + encrypted JSON payload
- TCP port 9999 (default), configurable timeout

### Polling Architecture

1. `Worker` polls all configured devices concurrently
2. Each poll fetches emeter data and creates `Metric` objects
3. Metrics are pushed to `Pipeline` which batches and sends to backends
4. Signal handling: SIGINT/SIGTERM for shutdown, SIGHUP for config reload

## Development

### Build Commands

```bash
# Build the binary
make build

# Run all tests
make test

# Run tests with coverage
make test-coverage

# Format code
make fmt

# Lint code
make lint

# Clean build artifacts
make clean
```

### Dependencies

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [BurntSushi/toml](https://github.com/BurntSushi/toml) - TOML configuration
- [InfluxDB Client](https://github.com/influxdata/influxdb-client-go) - InfluxDB 2.x
- [Prometheus Client](https://github.com/prometheus/client_golang) - Prometheus metrics
- `log/slog` - Structured logging (standard library)

## Supported Devices

- **Smart Plugs**: HS100, HS103, HS105, HS110 (with emeter)
- **Smart Bulbs**: LB100, LB110, LB120, LB130 (color), KL110, KL120, KL130
- **Light Strips**: KL400, KL430
- **Power Strips**: HS300, KP303 (with per-outlet monitoring)

Energy meter (emeter) support varies by model.

## Troubleshooting

### Device Not Found

- Ensure device is on the same network
- Check firewall rules for port 9999
- Verify IP address is correct

### Connection Errors

- Devices may take a few seconds to respond after power on
- Some devices have rate limiting - reduce polling interval
- Check for firmware updates on devices

### Metrics Not Appearing in InfluxDB

- Verify InfluxDB credentials in config file
- Check that bucket/org exists
- Review logs for connection errors
- Ensure measurements are configured correctly

## Requirements

- Go 1.21+
- InfluxDB 2.x (optional, for metrics export)
- Network access to KASA devices on port 9999

## Platform Support

- **Linux**: Full support including signal handling
- **macOS**: Full support
- **Windows**: Full support

## License

Copyright 2019-2024 Daniel Weiner

Licensed under the Apache License, Version 2.0. See LICENSE file for details.

## Disclaimer

This project is not affiliated with or endorsed by TP-Link. Use at your own risk. The protocol is proprietary and subject to change.
