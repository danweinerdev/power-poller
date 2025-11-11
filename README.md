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

# Install dependencies
pip install -r requirements.txt
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
```

**Note**: Device polling uses asyncio for concurrent operations, dramatically improving performance when monitoring multiple devices.

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
