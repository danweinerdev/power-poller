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


class Plug(Device):

    def __init__(self, *args, **kwargs):
        super(Plug, self).__init__(type=DeviceType.PLUG, *args, **kwargs)

    def __repr__(self):
        return '<{}: {}>'.format(self.GetType(), self.address)

    def GetEmeterType(self):
        if not self.HasEmeter():
            raise DeviceError('Device does not support the emeter')
        return 'emeter'

    def GetType(self):
        return 'Plug'

    def HasEmeter(self):
        return 'ENE' in self.GetFeatures()

    def IsOff(self):
        return not self.IsOn()

    def IsOn(self):
        return bool(self.GetSysInfo('relay_state'))

    def IsLedOff(self):
        return not self.IsLedOn()

    def IsLedOn(self):
        return bool(self.GetSysInfo('led_off') == 0)

    def Off(self):
        return self.Send(
            self.QueryHelper('system', 'set_relay_state', {'state': 0}))

    def On(self):
        return self.Send(
            self.QueryHelper('system', 'set_relay_state', {'state': 1}))
