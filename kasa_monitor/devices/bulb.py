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

from .device import Device, DeviceType
from .exceptions import DeviceError


class Bulb(Device):

    LIGHT_STATE = 'smartlife.iot.smartbulb.lightingservice'

    def __init__(self, type=DeviceType.BULB, **kwargs):
        super(Bulb, self).__init__(type=type, **kwargs)

    def __repr__(self):
        return '<Bulb: {}>'.format(self.address)

    def GetBrightness(self):
        if not self.IsColorSupported():
            raise DeviceError('Color changes are not supported')
        info = self.GetLightState()
        if self.IsOn():
            return int(info['dft_on_state']['brightness'])
        return int(info.get('brightness', 0))

    def GetEmeterType(self):
        if not self.HasEmeter():
            raise DeviceError('Device does not support the emeter')
        return 'smartlife.iot.common.emeter'

    def GetHue(self):
        if not self.IsColorSupported():
            raise DeviceError('Color changes are not supported')
        info = self.GetLightState()
        if self.IsOn():
            return int(info['dft_on_state']['hue'])
        return int(info.get('hue', 0))

    def GetLightState(self, key=None):
        info = self.cache.Get(self.LIGHT_STATE)
        if info is None:
            info = self.Send(
                self.QueryHelper(self.LIGHT_STATE, 'get_light_state'))
        if info is not None:
            self.cache.Insert(self.LIGHT_STATE, info)
        if key is not None:
            return info.get(key)
        return info

    def GetSaturation(self):
        if not self.IsColorSupported():
            raise DeviceError('Color changes are not supported')
        info = self.GetLightState()
        if self.IsOn():
            return int(info['dft_on_state']['saturation'])
        return int(info.get('saturation', 0))

    def GetTemperature(self):
        if not self.IsTempSupported():
            raise DeviceError('Color changes are not supported')
        info = self.GetLightState()
        if self.IsOn():
            return int(info['dft_on_state']['color_temp'])
        return int(info.get('color_temp', 0))

    def HasEmeter(self):
        return True

    def IsBrightnessSupported(self):
        return bool(self.GetSysInfo('is_dimmable'))

    def IsColorSupported(self):
        return bool(self.GetSysInfo('is_color'))

    def IsTempSupported(self):
        return bool(self.GetSysInfo('is_variable_color_temp'))

    def IsOff(self):
        return not self.IsOn()

    def IsOn(self):
        return bool(self.GetLightState('on_off'))

    def On(self, transition=0):
        return self.SetLightState({'on_off': 1})

    def Off(self, transition=0):
        return self.SetLightState({'on_off': 0})

    def SetBrightness(self, value):
        if not self.IsColorSupported():
            raise DeviceError('Color changes are not supported')

        self.__ValidateBrightness(value)
        return self.SetLightState({'brightness': value})

    def SetTemperature(self, value):
        if not self.IsTempSupported():
            raise DeviceError('Color changes are not supported')

        self.__ValidateTemperature(value, self.GetModel())
        return self.SetLightState({'color_temp': value})

    def SetLightState(self, state):
        return self.Send(
            self.QueryHelper(self.LIGHT_STATE, 'transition_light_state', state))

    def SetValues(self, hue, saturation, value):
        if not self.IsColorSupported():
            raise DeviceError('Color changes are not supported')

        self.__ValidateBrightness(value)
        self.__ValidateHue(hue)
        self.__ValidateSaturation(saturation)

        return self.SetLightState({
            'hue': hue, 'saturation': saturation, 'brightness': value,
            'color_temp': 0})

    @staticmethod
    def __ValidateBrightness(value):
        if not isinstance(value, int) or not (0 <= value <= 100):
            raise ValueError('Invalid brightness value')

    @staticmethod
    def __ValidateHue(value):
        if not isinstance(value, int) or not (0 <= value < 240):
            raise ValueError('Invalid hue value')

    @staticmethod
    def __ValidateSaturation(value):
        if not isinstance(value, int) or not (0 <= value <= 100):
            raise ValueError('Invalid saturation value')

    @staticmethod
    def __ValidateTemperature(value, model):
        ranges = {'LB130': (2500, 9000)}
        if not isinstance(value, int):
            raise ValueError('Temperature must be an integer in kelvin')
        tempMin, tempMax = None, None
        for key, value in ranges.items():
            if model.startswith(key):
                tempMin, tempMax = value
        if tempMin is None:
            raise DeviceError('Unsupported bulb model: {}'.format(model))
        if value < tempMin or value > tempMax:
            raise ValueError('Temperature exceeds minimum or maximum values')
