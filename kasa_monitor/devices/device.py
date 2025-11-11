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

import json
import os
import socket
import struct
import sys
from kasa_monitor.core.utils import GetErrorMessage
from .emeter import EmeterHandler
from .exceptions import ConnectionError, DeviceError, InputError
from .utils import Cache, IsValidMacAddress


class DeviceType(object):
    NONE = 0
    BULB = 1
    PLUG = 2
    LIGHTSTRIP = 3


class Device(object):

    DEFAULT_PORT = 9999
    ENCRYPTION_KEY = 0xAB

    def __init__(self, address=None, type=DeviceType.NONE, info=None, key=None,
            port=DEFAULT_PORT, logger=None):
        self.address = address
        self.port = int(port)
        self.key = key or self.ENCRYPTION_KEY
        self.type = type or DeviceType.NONE
        self.emeter = None
        self.cache = Cache()
        self.logger = logger

    @staticmethod
    def Decrypt(message, key):
        buffer = []
        for cipherbyte in message:
            plainbyte = key ^ cipherbyte
            key = cipherbyte
            buffer.append(plainbyte)

        plaintext = bytes(buffer)
        return plaintext.decode()

    @staticmethod
    def Encrypt(message, key):
        plainbytes = message.encode()
        buffer = bytearray(struct.pack(">I", len(plainbytes)))

        for plainbyte in plainbytes:
            cipherbyte = key ^ plainbyte
            key = cipherbyte
            buffer.append(cipherbyte)

        return bytes(buffer)

    def GetAlias(self):
        return self.GetSysInfo('alias')

    def GetDescription(self):
        return self.GetSysInfo('description')

    def GetDeviceIdentifier(self):
        return self.GetSysInfo('deviceId')

    def GetDeviceState(self):
        return self.GetSysInfo('dev_state')

    def GetEmeter(self):
        if not self.HasEmeter():
            raise DeviceError('Device does not support the emeter')
        if self.emeter is None:
            self.emeter = EmeterHandler(self)
        return self.emeter

    def GetEmeterType(self):
        raise NotImplementedError

    def GetFeatures(self):
        value = self.GetSysInfo('feature')
        return value.split(':') if value is not None else []

    def GetFirmwareIdentifier(self):
        return self.GetSysInfo('fwId')

    def GetInfo(self, key=None):
        info = self.cache.Get('system')
        if info is None:
            info = self.Send(
                self.QueryHelper('system', 'get_sysinfo'))
        if info is not None:
            self.cache.Insert('system', info)
        if key is not None:
            return info[key]
        return info

    def GetHardwareVersion(self):
        return self.GetSysInfo('hw_ver')

    def GetHardwareIdentifier(self):
        return self.GetSysInfo('hwId')

    def GetMacAddress(self):
        value = self.GetSysInfo('mac')
        if value is not None:
            return str(value)
        value = self.GetSysInfo('mic_mac')
        if value is not None:
            if isinstance(value, str):
                return value
            else:
                return ':'.join(format(s, '02x')
                                for s in bytes.fromhex(value))
        return None

    def GetManufacturerIdentifier(self):
        return self.GetSysInfo('oemId')

    def GetModel(self):
        return self.GetSysInfo('model')

    def GetSignalStrength(self):
        value = self.GetSysInfo('rssi')
        if value is not None:
            return int(value)
        return -1 * sys.maxint

    def GetSoftwareVersion(self):
        return self.GetSysInfo('sw_ver')

    def GetSysInfo(self, key=None):
        if key is not None:
            return self.GetInfo('system')['get_sysinfo'].get(key)
        return self.GetInfo('system')['get_sysinfo']

    def GetType(self):
        match self.type:
            case DeviceType.BULB:
                return 'Bulb'
            case DeviceType.LIGHTSTRIP:
                return 'Light Strip'
            case DeviceType.PLUG:
                return 'Plug'
        return 'Unknown'

    def GetTypeString(self):
        return (self.GetSysInfo('type') or self.GetSysInfo('mic_type')).lower()

    def GetUptime(self):
        value = self.GetSysInfo('on_time')
        if value is not None:
            return int(value)
        return int(-1)

    def HasEmeter(self):
        raise NotImplementedError

    def IsBrightnessSupported(self):
        # Device Specific Implementation
        return False

    def IsBulb(self):
        return self.type == DeviceType.BULB

    def IsOff(self):
        # Device Specific Implementation
        raise NotImplementedError

    def IsOn(self):
        # Device Specific Implementation
        raise NotImplementedError

    def IsPlug(self):
        return self.type == DeviceType.PLUG

    def Off(self):
        # Device Specific Implementation
        raise NotImplementedError

    def On(self):
        # Device Specific Implementation
        raise NotImplementedError

    def Reboot(self, delay=0):
        return self.Send(
            self.QueryHelper('system', 'reboot', {'delay': delay}))

    def Send(self, message):
        encrypted = self.Encrypt(message, self.key)

        sock = None
        try:
            sock = socket.create_connection((self.address, self.port), 3)
            sock.send(encrypted)

            buffer = bytes()
            length = -1
            while True:
                chunk = sock.recv(4096)
                if length == -1:
                    length = struct.unpack(">I", chunk[0:4])[0]
                buffer += chunk
                if (length > 0 and len(buffer) >= length + 4) or not chunk:
                    break
        except OSError as e:
            if self.logger:
                self.logger.exception('Error connecting to: {} ({})'.format(self.address, e))
            raise ConnectionError(e.errno,
                "Error connecting to '{}' ({}): [{}] {}".format(
                    self.GetType(), self.address, e.errno,
                    GetErrorMessage(e.errno) if e.errno else 'None'))
        finally:
            try:
                sock.close()
            except:
                pass

        response = self.Decrypt(buffer[4:], self.key)
        return json.loads(response)

    def Set(self, category, option, value):
        result = self.Send(self.QueryHelper(category, option, value))
        if result is None or len(result) == 0:
            if self.logger:
                self.logger.error("Error: unable to set '{}::{}': invalid response from device".format(
                    category, option))
            return False
        if category not in result:
            if self.logger:
                self.logger.error('Error: invalid response from device')
            return False
        if option not in result[category]:
            if self.logger:
                self.logger.error('Error: invalid response from device')
            return False
        response = result[category][option]
        if 'err_code' not in response or response['err_code'] != 0:
            return False
        self.cache.Clear()
        return True

    def SetAlias(self, alias):
        return self.Set('system', 'set_dev_alias', {'alias': alias})

    def SetMacAddress(self, address):
        if not IsValidMacAddress(address):
            raise InputError('Invalid MAC address: {}'.format(address))
        return self.Send(
            self.QueryHelper('system', 'set_mac_addr', {'mac': address}))

    @staticmethod
    def QueryHelper(target, command, argument=None):
        message = {target: {command: argument or {}}}
        return json.dumps(message)
