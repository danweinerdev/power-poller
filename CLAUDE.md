# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

KASA Monitor is a unified Python 3 library and CLI tool for monitoring and controlling TP-Link KASA smart home devices (plugs, bulbs, light strips). The project supports device discovery, control, and monitoring with energy meter (emeter) data collection for InfluxDB integration.

This project combines what were previously two separate projects: PyMonitorLib (monitoring framework) and TPLink device library into a single cohesive package called `kasa_monitor`.

## Code Structure

The codebase is organized into a single unified package structure:

### Package: `kasa_monitor/`

#### 1. Core Module (`kasa_monitor/core/`)
- **Executor Framework** (`executor.py`): Main daemon/service execution framework with:
  - Daemonization support (double-fork on POSIX systems)
  - Signal handling (SIGTERM/SIGINT for shutdown, SIGHUP for config reload)
  - Interval-based polling with configurable callback
  - PID file management
- **Config System** (`config.py`): INI-based configuration with type coercion
- **Metrics Pipeline** (`metrics.py`): Batches and sends metrics to InfluxDB
- **Database** (`database.py`): InfluxDB client abstraction
- **Daemon** (`daemon.py`): Process daemonization utilities
- **Utils** (`utils.py`): System utilities (user/group management, file descriptors, etc.)

#### 2. Devices Module (`kasa_monitor/devices/`)
- **Device Protocol**: Custom encryption protocol using XOR cipher (key 0xAB) over TCP port 9999
- **Device Hierarchy**: Base `Device` class with specialized subclasses:
  - `Plug` - Smart plugs with relay control and optional emeter
  - `Bulb` - Smart bulbs with color/brightness/temperature controls
  - `LightStrip` - Light strips (extends Bulb with length parameter)
- **Discovery** (`discovery.py`): Auto-detection of device types by querying `get_sysinfo`
  - `LoadDevice(address)` - Load a single device
  - `LoadDevices(addresses)` - Load multiple devices
  - `GetDeviceType(info)` - Determine device type from sysinfo
- **Emeter Handler** (`emeter.py`): Energy monitoring for supported devices
- **Utils** (`utils.py`): Device utilities (IP/MAC validation, caching)

#### 3. Commands Module (`kasa_monitor/commands/`)
- **Poll** (`poll.py`) - Main polling loop that collects emeter data and sends to InfluxDB
- **Status** (`status.py`) - Display device information, state, and emeter readings
- **Interactive** (`interactive.py`) - REPL for device control (toggle, reboot, set alias, etc.)

#### 4. CLI Entry Point (`kasa_monitor/__main__.py`)
- Main entry point using the Executor framework with `Poll` as the default callback
- Subcommands: `run`, `status`, `interactive`

## Key Architecture Patterns

### Device Communication Flow
1. Device discovery via `LoadDevice(address)` creates base Device, queries sysinfo
2. `GetDeviceType()` examines sysinfo to determine device class (Plug/Bulb/LightStrip)
3. Appropriate subclass instantiated with cached sysinfo
4. All commands use `QueryHelper()` to build JSON payload, `Send()` to encrypt/transmit
5. Responses decrypted and cached in device's `Cache` object

### Polling Architecture
The `Poll` command iterates through configured devices, calling:
1. `LoadDevice()` to create device instance
2. `device.GetEmeter().GetRealtime()` to fetch current power data
3. `Metric()` creation with configured tags/measurements
4. `pipeline(metric)` to queue for batch upload
5. `pipeline.Flush()` sends batched metrics to InfluxDB

### Configuration Requirements
- Config file uses INI format with `[global]`, `[influxdb]`, device sections, and measurement sections
- Device sections must have `address` and `measurements` fields
- Measurement sections define field names and their types (float, int, string, bool)
- See `config/example.conf` for reference

## Development Commands

### Running the CLI

The application can be run as a Python module. Import paths use the unified package structure:
- `from kasa_monitor.devices import Bulb, Plug, LightStrip, Device, LoadDevice, LoadDevices`
- `from kasa_monitor.core import Execute, Config, Metric`
- `from kasa_monitor.commands import Poll, Status, Interactive`

### Docker Container
```bash
# Build container
docker build -f Containerfile -t kasa-monitor:latest .

# Run in polling mode with config
docker run -it -v $PWD/config/example.conf:/etc/monitor.conf:ro kasa-monitor:latest

# Interactive mode
docker run -it kasa-monitor:latest interactive --device 10.0.0.100

# Status check
docker run -it kasa-monitor:latest status --device 10.0.0.100
```

### Common Operations
```bash
# Run as Python module (recommended)
python -m kasa_monitor status --device 10.0.0.100
python -m kasa_monitor interactive --device 10.0.0.100 --device 10.0.0.101
python -m kasa_monitor run -o --loglevel=INFO /path/to/config.conf

# Run in foreground with debug logging
python -m kasa_monitor run -o --loglevel=DEBUG --debug /path/to/config.conf
```

## Dependencies

All dependencies are now self-contained in the `kasa_monitor` package. No external PyMonitorLib dependency. Main dependencies: influxdb-client, requests, reactivex.

## Notes

- This is a unified refactoring of PyMonitorLib and the TPLink library into a single package
- Device protocol is proprietary TP-Link encryption; no authentication/TLS
- Emeter support varies by device model - check `HasEmeter()` before accessing
- Color bulb models have different temperature ranges (e.g., LB130: 2500-9000K)
- Windows support exists but daemonization features are POSIX-only
- The old `lib/monitor/` and `lib/tplink/` directories can be removed after migration
