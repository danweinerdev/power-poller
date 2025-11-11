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
Async device wrapper using python-kasa library.

This module provides an async interface to KASA devices using the official
python-kasa library, replacing the custom protocol implementation.
"""

import asyncio
from typing import Optional, Dict, Any
from kasa import Discover, Device, Module


class KasaDeviceWrapper:
    """
    Wrapper around python-kasa Device for consistent API with legacy code.

    This class provides async methods for device operations and maintains
    compatibility with the existing monitoring infrastructure.
    """

    def __init__(self, device: Device):
        """
        Initialize wrapper with a python-kasa Device instance.

        :param device: python-kasa Device object
        """
        self.device = device
        self._cached_info = {}

    @property
    def address(self) -> str:
        """Get device IP address."""
        return self.device.host

    @property
    def alias(self) -> str:
        """Get device alias/name."""
        return self.device.alias

    @property
    def model(self) -> str:
        """Get device model."""
        return self.device.model

    @property
    def mac(self) -> str:
        """Get device MAC address."""
        return self.device.mac

    @property
    def rssi(self) -> int:
        """Get WiFi signal strength."""
        return self.device.rssi if hasattr(self.device, 'rssi') else -1

    async def update(self) -> None:
        """
        Update device state from hardware.

        This method must be called before accessing device properties
        to ensure data is current.
        """
        await self.device.update()

    async def turn_on(self) -> None:
        """Turn device on."""
        await self.device.turn_on()

    async def turn_off(self) -> None:
        """Turn device off."""
        await self.device.turn_off()

    @property
    def is_on(self) -> bool:
        """Check if device is on."""
        return self.device.is_on

    @property
    def is_off(self) -> bool:
        """Check if device is off."""
        return not self.device.is_on

    @property
    def has_emeter(self) -> bool:
        """Check if device has energy monitoring."""
        return self.device.has_emeter

    async def get_emeter_realtime(self) -> Optional[Dict[str, Any]]:
        """
        Get real-time energy meter data.

        :return: Dictionary with energy metrics or None if not supported
        """
        if not self.has_emeter:
            return None

        emeter_data = self.device.emeter_realtime

        # Convert to format expected by metrics pipeline
        return {
            'voltage': emeter_data.get('voltage', 0.0) if emeter_data else 0.0,
            'current': emeter_data.get('current', 0.0) if emeter_data else 0.0,
            'power': emeter_data.get('power', 0.0) if emeter_data else 0.0,
            'total': emeter_data.get('total', 0.0) if emeter_data else 0.0,
            'err_code': 0  # python-kasa raises exceptions instead of error codes
        }

    async def set_alias(self, alias: str) -> None:
        """
        Set device alias/name.

        :param alias: New device name
        """
        await self.device.set_alias(alias)

    async def reboot(self, delay: int = 0) -> None:
        """
        Reboot the device.

        :param delay: Delay in seconds before reboot
        """
        if hasattr(self.device, 'reboot'):
            await self.device.reboot(delay=delay)

    def get_device_info(self) -> Dict[str, Any]:
        """
        Get comprehensive device information.

        :return: Dictionary with device details
        """
        return {
            'address': self.address,
            'alias': self.alias,
            'model': self.model,
            'mac': self.mac,
            'rssi': self.rssi,
            'is_on': self.is_on,
            'has_emeter': self.has_emeter,
        }


async def discover_device(address: str, username: Optional[str] = None,
                         password: Optional[str] = None) -> Optional[KasaDeviceWrapper]:
    """
    Discover and connect to a single device.

    :param address: IP address of device
    :param username: Optional TP-Link cloud username
    :param password: Optional TP-Link cloud password
    :return: KasaDeviceWrapper instance or None if connection fails
    """
    try:
        kwargs = {}
        if username and password:
            kwargs['credentials'] = {'username': username, 'password': password}

        device = await Discover.discover_single(address, **kwargs)
        if device:
            await device.update()
            return KasaDeviceWrapper(device)
    except Exception as e:
        # Log error but don't crash - return None for failed connections
        return None

    return None


async def discover_devices(addresses: list[str], username: Optional[str] = None,
                           password: Optional[str] = None) -> list[KasaDeviceWrapper]:
    """
    Discover multiple devices concurrently.

    Uses asyncio.gather for concurrent discovery of all devices.

    :param addresses: List of IP addresses
    :param username: Optional TP-Link cloud username
    :param password: Optional TP-Link cloud password
    :return: List of successfully connected devices
    """
    # Create discovery tasks for all addresses
    tasks = [discover_device(addr, username, password) for addr in addresses]

    # Gather results concurrently
    results = await asyncio.gather(*tasks, return_exceptions=True)

    # Filter out None values and exceptions
    devices = []
    for result in results:
        if isinstance(result, KasaDeviceWrapper):
            devices.append(result)

    return devices


async def poll_devices(devices: list[KasaDeviceWrapper]) -> list[Dict[str, Any]]:
    """
    Poll multiple devices concurrently for emeter data.

    Implements async map-reduce pattern for efficient concurrent polling.

    :param devices: List of KasaDeviceWrapper instances
    :return: List of dictionaries with device data and metrics
    """
    async def poll_single_device(device: KasaDeviceWrapper) -> Optional[Dict[str, Any]]:
        """Poll a single device and return its data."""
        try:
            await device.update()

            result = {
                'device': device.alias,
                'address': device.address,
                'is_on': device.is_on,
            }

            if device.has_emeter:
                emeter_data = await device.get_emeter_realtime()
                if emeter_data:
                    result['emeter'] = emeter_data

            return result
        except Exception as e:
            # Log error but continue with other devices
            return {
                'device': device.alias,
                'address': device.address,
                'error': str(e)
            }

    # Map: Create poll tasks for all devices
    tasks = [poll_single_device(device) for device in devices]

    # Reduce: Gather all results concurrently
    results = await asyncio.gather(*tasks, return_exceptions=True)

    # Filter out exceptions and None values
    return [r for r in results if r is not None and not isinstance(r, Exception)]
