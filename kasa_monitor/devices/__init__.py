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
KASA device interface using python-kasa library.

This module provides async device discovery and polling using the official
python-kasa library for TP-Link KASA and Tapo smart home devices.
"""

# Async device interface
from .async_device import (
    KasaDeviceWrapper,
    discover_device,
    discover_devices,
    poll_devices
)

# Utilities and exceptions
from .exceptions import ConnectionError, DeviceError, InputError
from .utils import IsValidIPv4, IsValidMacAddress

__all__ = [
    # Device API
    'KasaDeviceWrapper',
    'discover_device',
    'discover_devices',
    'poll_devices',
    # Utilities
    'ConnectionError',
    'DeviceError',
    'InputError',
    'IsValidIPv4',
    'IsValidMacAddress'
]
