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

import os
from monitor.lib.utils import GetErrorMessage


class DeviceError(Exception):
    def __init__(self, message=None, *args):
        self.message = message or 'Whoops! Something went wrong'
        self.message = self.message.format(*args)


class ConnectionError(DeviceError):

    def __init__(self, err, *args, **kwargs):
        self.err = err or -1
        self.errstr = GetErrorMessage(err) if err else ''
        super(ConnectionError, self).__init__(*args, **kwargs)


class InputError(DeviceError):
    message = 'Invalid input given'
