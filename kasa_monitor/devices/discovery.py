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

from .bulb import Bulb
from .device import Device
from .lightstrip import LightStrip
from .plug import Plug
from .exceptions import DeviceError


def GetDeviceType(info):
    deviceType = None
    sysinfo = None
    if 'system' in info and 'get_sysinfo' in info['system']:
        sysinfo = info['system']['get_sysinfo']
        if 'type' in sysinfo:
            deviceType = sysinfo['type']
        elif 'mic_type' in sysinfo:
            deviceType = sysinfo['mic_type']

    if deviceType is None:
        raise DeviceError('Unable to detect device type')
    if 'smartplug' in deviceType.lower():
        return Plug
    elif 'smartbulb' in deviceType.lower():
        if 'length' in sysinfo:
            return LightStrip
        return Bulb

    return None


def LoadDevice(address, logger=None):
    device = Device(address, logger=logger)
    info = device.GetInfo()
    if info is not None:
        DeviceType = GetDeviceType(info)
        if DeviceType is not None:
            return DeviceType(address=address, info=info)
    return None


def LoadDevices(addresses, logger=None):
    devices = []
    if not addresses or len(addresses) == 0:
        return []
    for address in addresses:
        device = LoadDevice(address, logger=logger)
        if not device:
            if logger:
                logger.error('Error: Unable to determine device type for: {}'.format(address))
            continue
        devices.append(device)

    return devices
