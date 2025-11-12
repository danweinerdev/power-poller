# Copyright 2019-2024 Daniel Weiner
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
Async polling command using python-kasa library.

This module implements concurrent device polling using asyncio for efficient
data collection from multiple KASA devices.
"""

import asyncio
from typing import Dict, List, Any
from kasa_monitor.core import Metric, Result
from kasa_monitor.devices import discover_devices, poll_devices, KasaDeviceWrapper, IsValidIPv4


class AsyncPollManager:
    """
    Manages async polling of KASA devices.

    This class handles device discovery, concurrent polling, and metric generation
    using async/await patterns for optimal performance.
    """

    def __init__(self, config: Dict[str, Any], logger=None):
        """
        Initialize async poll manager.

        :param config: Configuration dictionary with device information
        :param logger: Optional logger instance
        """
        self.config = config
        self.logger = logger
        self.devices: List[KasaDeviceWrapper] = []
        self.username = None
        self.password = None

    async def initialize_devices(self) -> bool:
        """
        Discover and initialize all configured devices.

        :return: True if at least one device was initialized
        """
        addresses = []

        # Extract credentials if provided
        for device_name, cfg in self.config.items():
            if 'address' in cfg:
                address = cfg['address']
                if IsValidIPv4(address):
                    addresses.append(address)
                else:
                    if self.logger:
                        self.logger.error(f"Invalid device address for '{device_name}': {address}")

            # Get credentials from first device config (assumes same credentials for all)
            if not self.username and 'username' in cfg:
                self.username = cfg.get('username')
                self.password = cfg.get('password')

        if not addresses:
            if self.logger:
                self.logger.error("No valid device addresses found in configuration")
            return False

        # Discover all devices concurrently
        if self.logger:
            self.logger.info(f"Discovering {len(addresses)} devices concurrently...")

        self.devices = await discover_devices(addresses, self.username, self.password)

        if not self.devices:
            if self.logger:
                self.logger.error("Failed to discover any devices")
            return False

        if self.logger:
            self.logger.info(f"Successfully discovered {len(self.devices)} device(s)")

        return True

    async def poll_once(self, pipeline) -> Result:
        """
        Poll all devices once and send metrics to pipeline.

        Uses concurrent async operations for efficient polling.

        :param pipeline: Metrics pipeline for sending data
        :return: Result status
        """
        if not self.devices:
            if self.logger:
                self.logger.warning("No devices available for polling")
            return Result.FAILURE

        try:
            # Poll all devices concurrently
            poll_results = await poll_devices(self.devices)

            # Process results and create metrics
            for result in poll_results:
                if 'error' in result:
                    if self.logger:
                        self.logger.warning(f"Error polling device {result.get('device')}: {result['error']}")
                    continue

                # Find config for this device
                device_config = None
                device_name = None
                for name, cfg in self.config.items():
                    if cfg.get('address') == result['address']:
                        device_config = cfg
                        device_name = name
                        break

                if not device_config:
                    continue

                # Handle devices with children
                if device_config.get('has_children', False):
                    await self._poll_device_with_children(result, device_name, device_config, pipeline)
                else:
                    # Create metric for regular device if it has emeter data
                    if 'emeter' in result and device_config.get('measurements'):
                        self._create_metrics(result['device'], result['emeter'],
                                           device_config, {}, pipeline)

            return Result.SUCCESS

        except Exception as e:
            if self.logger:
                self.logger.error(f"Error during polling: {e}")
            return Result.FAILURE

    async def _poll_device_with_children(self, result: Dict[str, Any], device_name: str,
                                        device_config: Dict[str, Any], pipeline) -> None:
        """
        Poll a device with children (smart power strip) and create metrics for parent and children.

        :param result: Poll result containing device info
        :param device_name: Name of parent device from config
        :param device_config: Configuration for parent device
        :param pipeline: Metrics pipeline
        """
        # Find the actual device wrapper
        device_wrapper = None
        for dev in self.devices:
            if dev.address == result['address']:
                device_wrapper = dev
                break

        if not device_wrapper:
            return

        # Poll parent device if configured
        if device_config.get('poll_parent', False):
            if 'emeter' in result and device_config.get('measurements'):
                # Add parent=true tag to distinguish from children
                parent_tags = {'parent': 'true'}
                self._create_metrics(result['device'], result['emeter'],
                                   device_config, parent_tags, pipeline)

        # Poll child devices
        children_config = device_config.get('children', {})
        for child_index, child_config in children_config.items():
            try:
                # Get child device by index
                child_wrapper = device_wrapper.get_child_by_index(child_index)
                if not child_wrapper:
                    if self.logger:
                        self.logger.warning(
                            f"Child device at index {child_index} not found for parent '{device_name}'"
                        )
                    continue

                # Update child device state
                await child_wrapper.update()

                # Get child emeter data
                if child_wrapper.has_emeter:
                    child_emeter = await child_wrapper.get_emeter_realtime()
                    if child_emeter and child_config.get('measurements'):
                        # Add child-specific tags
                        child_tags = {
                            'parent': device_name,
                            'child_index': str(child_index)
                        }

                        # Use configured device name for child
                        child_device_name = child_config.get('device', f"{device_name}_child_{child_index}")

                        self._create_metrics(child_device_name, child_emeter,
                                           child_config, child_tags, pipeline)
            except Exception as e:
                if self.logger:
                    self.logger.error(
                        f"Error polling child {child_index} of device '{device_name}': {e}"
                    )

    def _create_metrics(self, device_name: str, emeter_data: Dict[str, Any],
                       config: Dict[str, Any], extra_tags: Dict[str, str], pipeline) -> None:
        """
        Create metrics for a device (parent or child) and send to pipeline.

        :param device_name: Name of the device for the metric
        :param emeter_data: Energy meter data
        :param config: Device configuration with measurements and tags
        :param extra_tags: Additional tags to add (e.g., parent, child_index)
        :param pipeline: Metrics pipeline
        """
        # Build tags from config and extra_tags
        tags = {'device': config.get('device', device_name)}
        tags.update(config.get('tags', {}))
        tags.update(extra_tags)

        # Create metric for each configured measurement
        for measurement in config['measurements']:
            metric = Metric(device_name, measurement, tags=tags)

            # Get configured fields for this measurement
            configured_fields = config['measurements'][measurement]

            # Add configured fields from emeter data
            for field in configured_fields:
                if field in emeter_data:
                    metric.AddField(field, emeter_data[field])

            if metric.fields:
                pipeline(metric)


# Global manager instance to persist devices across poll cycles
_poll_manager = None


# Synchronous wrapper for use with existing executor framework
def AsyncPoll(config, logger, pipeline):
    """
    Synchronous wrapper for async polling.

    This function provides a synchronous interface to the async polling
    mechanism for compatibility with the existing executor framework.

    Maintains a persistent manager instance across calls to avoid
    reconnecting to devices on every poll.

    :param config: Device configuration
    :param logger: Logger instance
    :param pipeline: Metrics pipeline
    :return: Result status
    """
    global _poll_manager

    async def _async_poll():
        global _poll_manager

        # Initialize manager on first call
        if _poll_manager is None:
            _poll_manager = AsyncPollManager(config, logger)
            initialized = await _poll_manager.initialize_devices()
            if not initialized:
                _poll_manager = None
                return Result.FAILURE

        # Poll devices
        return await _poll_manager.poll_once(pipeline)

    # Run async function in event loop
    try:
        return asyncio.run(_async_poll())
    except Exception as e:
        if logger:
            logger.error(f"Fatal error in async poll: {e}")
        _poll_manager = None  # Reset on error
        return Result.FAILURE
