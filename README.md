# KASA Monitor

A unified Python 3 library and CLI tool for monitoring and controlling TP-Link KASA smart home devices.

## Features

- **Device Support**: Smart plugs, bulbs, and light strips
- **Real-time Monitoring**: Energy meter (emeter) data collection
- **InfluxDB Integration**: Automatic metrics export for Grafana dashboards
- **Multiple Interfaces**:
  - Daemon mode for continuous monitoring
  - Interactive REPL for device control
  - Status command for one-time queries
- **Docker Support**: Run as a containerized service

## Quick Start

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd tplink-smart-devices

# Quick install using Make
make install

# Or install manually
pip install -r requirements.txt
```

For development:

```bash
# Install with development dependencies
make install-dev

# Or manually
pip install -r requirements.txt -r requirements-test.txt
pip install -e ".[dev]"
```

### Basic Usage

```bash
# Check device status
python -m kasa_monitor status --device 192.168.1.100

# Interactive mode
python -m kasa_monitor interactive --device 192.168.1.100

# Run continuous monitoring (requires config file)
python -m kasa_monitor run /path/to/config.conf
```

## Configuration

Create a configuration file for continuous monitoring. See `config/example.conf` for a template:

```ini
[global]
database = influxdb
devices = plug1

[influxdb]
server = 127.0.0.1
port = 8086
ssl = True
verify = True
org = myorg
token = mytoken
bucket = kasa-monitor

[plug1]
address = 192.168.1.100
device = plug1
measurements = emeter
tags = location=office

[emeter]
current = float
voltage = float
power = float
total = float
```

**Note**: When using `--echo-metrics`, the `[global] database` field and database-specific sections (like `[influxdb]`) are optional and can be omitted.

### Child Devices (Smart Power Strips)

KASA Monitor supports devices with individual child outlets, such as smart power strips. Each outlet can be monitored separately with its own metrics and tags.

See [config/powerstrip-example.conf](config/powerstrip-example.conf) for a complete example.

**Key Configuration:**

```ini
[global]
database = influxdb
devices = office_powerstrip

[influxdb]
server = 127.0.0.1
port = 8086
database = smart-home

; Parent device (the power strip itself)
[office_powerstrip]
address = 10.0.0.100
has_children = true          ; Mark device as having children
poll_parent = true           ; Optional: also collect parent aggregate metrics
measurements = power-metrics
tags = location=office type=strip

; Child devices (individual outlets)
; Format: [parent_name.child_N] where N is zero-based index

[office_powerstrip.child_0]
device = desk_lamp           ; Friendly name for metrics
measurements = power-metrics
tags = outlet=0 appliance=lamp

[office_powerstrip.child_1]
device = monitor
measurements = power-metrics
tags = outlet=1 appliance=monitor

[office_powerstrip.child_2]
device = laptop_charger
measurements = power-metrics
tags = outlet=2 appliance=charger

[power-metrics]
fields = current:float voltage:float power:float total:float
```

**Resulting Metrics:**

With this configuration, you'll get separate metrics for:
- **Parent device** (if `poll_parent = true`): Tagged with `parent=true`
- **Each child outlet**: Tagged with `parent=<parent_name>` and `child_index=<N>`

Example InfluxDB line protocol output:

```
power-metrics,device=office_powerstrip,location=office,type=strip,parent=true current=1.2,voltage=120.0,power=144.0,total=5.2
power-metrics,device=desk_lamp,location=office,type=strip,parent=office_powerstrip,child_index=0,outlet=0,appliance=lamp current=0.1,voltage=120.0,power=12.0,total=0.5
power-metrics,device=monitor,location=office,type=strip,parent=office_powerstrip,child_index=1,outlet=1,appliance=monitor current=0.3,voltage=120.0,power=36.0,total=1.2
power-metrics,device=laptop_charger,location=office,type=strip,parent=office_powerstrip,child_index=2,outlet=2,appliance=charger current=0.8,voltage=120.0,power=96.0,total=3.5
```

**Querying Child Devices:**

```flux
// Total power for all outlets on a strip
from(bucket: "smart-home")
  |> range(start: -1h)
  |> filter(fn: (r) => r.parent == "office_powerstrip")
  |> filter(fn: (r) => r._field == "power")
  |> sum()

// Individual outlet
from(bucket: "smart-home")
  |> range(start: -1h)
  |> filter(fn: (r) => r.device == "desk_lamp")
  |> filter(fn: (r) => r._field == "power")
```

## Command Reference

### Run (Daemon Mode)

Continuously poll devices concurrently and send metrics to InfluxDB:

```bash
# Run in foreground with logging to stdout
python -m kasa_monitor -o --loglevel=INFO /etc/monitor.conf

# Run as daemon (POSIX only)
python -m kasa_monitor -d --pidfile=/var/run/kasa-monitor.pid /etc/monitor.conf

# Debug mode
python -m kasa_monitor -o --loglevel=DEBUG --debug /etc/monitor.conf

# Poll once and exit (no continuous monitoring)
python -m kasa_monitor --run-once -o --loglevel=INFO /etc/monitor.conf

# Echo metrics to stdout (no database required)
python -m kasa_monitor --echo-metrics --run-once -o /etc/monitor.conf
```

**Note**: Device polling uses asyncio for concurrent operations, dramatically improving performance when monitoring multiple devices.

**Options**:
- `--run-once`: Poll all devices one time and exit. Useful for testing or running via cron/scheduled tasks.
- `--echo-metrics`: Print metrics to stdout in InfluxDB line protocol format instead of sending to database. Database configuration becomes optional with this flag.

## Docker Usage

### Build Container

```bash
docker build -f Containerfile -t kasa-monitor:latest .
```

### Run Container

```bash
# Continuous monitoring
docker run -d \
  -v $PWD/config/monitor.conf:/etc/monitor.conf:ro \
  --name kasa-monitor \
  kasa-monitor:latest
```

## Architecture

### Package Structure

```
kasa_monitor/
├── core/           # Monitoring framework
│   ├── config.py   # INI configuration parser
│   ├── daemon.py   # Daemonization support
│   ├── database.py # InfluxDB client
│   ├── executor.py # Main execution framework
│   ├── metrics.py  # Metrics pipeline
│   └── utils.py    # System utilities
├── devices/        # KASA device support (python-kasa)
│   ├── async_device.py # Async device wrapper
│   ├── exceptions.py   # Device exceptions
│   └── utils.py        # Device utilities
└── commands/       # CLI commands
    └── async_poll.py  # Async polling with concurrent operations
```

### Device Protocol

Uses the official [python-kasa](https://python-kasa.readthedocs.io/) library for device communication:
- Async/await interface
- Concurrent device operations
- Automatic protocol handling
- Support for both KASA and Tapo devices
- No custom encryption implementation needed

### Monitoring Flow

1. **Discovery**: `discover_devices()` connects to all devices concurrently
2. **Polling**: `poll_devices()` uses async map-reduce for concurrent data collection
3. **Metrics**: Data converted to InfluxDB Point objects
4. **Batch Upload**: Metrics queued and sent in configurable batches

**Performance**: Concurrent async operations poll multiple devices simultaneously, reducing total cycle time by orders of magnitude.

## Supported Devices

- **Smart Plugs**: HS100, HS103, HS105, HS110 (with emeter)
- **Smart Bulbs**: LB100, LB110, LB120, LB130 (color), KL110, KL120, KL130
- **Light Strips**: KL400, KL430

Energy meter (emeter) support varies by model. Use `HasEmeter()` to check device capabilities.

## Development

### Project Structure

This project combines what were previously two separate libraries:
- **PyMonitorLib**: Monitoring framework with daemon support
- **TPLink Library**: KASA device protocol implementation

Both are now unified in the `kasa_monitor` package.

### Running from Source

```bash
# Set PYTHONPATH if needed
export PYTHONPATH=/path/to/tplink-smart-devices

# Run the application
python -m kasa_monitor [command] [options]
```

### Import Examples

```python
from kasa_monitor.core import Config, Execute, Metric
from kasa_monitor.devices import Bulb, Plug, LoadDevice, LoadDevices
from kasa_monitor.commands import Poll, Status, Interactive
```

## Requirements

- Python 3.10+
- InfluxDB 2.x (for metrics export)
- Network access to KASA devices

See `requirements.txt` for complete dependency list.

## Platform Support

- **Linux**: Full support including daemonization
- **macOS**: Full support including daemonization
- **Windows**: Full support except daemonization features

## Development

### Building and Testing

The project includes a comprehensive Makefile for common tasks:

```bash
# Show all available commands
make help

# Run tests
make test

# Run tests with coverage
make test-coverage

# Build distribution packages
make build

# Clean build artifacts
make clean

# Run full CI pipeline
make ci
```

### Testing

Run the test suite:

```bash
# Using Make
make test              # Run all tests
make test-coverage     # Run with coverage report
make test-fast         # Quick test run
make test-unit         # Unit tests only

# Using pytest directly
pytest
pytest --cov=kasa_monitor --cov-report=html
```

See [tests/README.md](tests/README.md) for detailed testing documentation.

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
- Ensure measurements are configured in config file

## License

Copyright 2019-2024 Daniel Weiner

Licensed under the Apache License, Version 2.0. See LICENSE file for details.

## Acknowledgments

Based on research into the TP-Link KASA protocol from the open-source community. Inspired by various JavaScript implementations adapted to Python.

## Contributing

This is a personal project for monitoring KASA devices. Feel free to fork and adapt to your needs.

## Disclaimer

This project is not affiliated with or endorsed by TP-Link. Use at your own risk. The protocol is proprietary and subject to change.
