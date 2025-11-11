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

import time
import re
import socket


class Cache(object):

    def __init__(self):
        self.data = {}

    def Clear(self):
        self.data.clear()

    def Get(self, key, timeout=5):
        if key not in self.data:
            return None
        (expiration, data) = self.data[key]
        if time.time() > expiration + (timeout * 1000):
            del self.data[key]
            return None
        return data

    def Insert(self, key, data):
        self.data[key] = (time.time(), data)


def IsValidIPv4(address):
    try:
        socket.inet_pton(socket.AF_INET, address)
    except AttributeError:
        try:
            socket.inet_aton(address)
        except socket.error:
            return False
        return address.count('.')
    except socket.error:
        return False
    return True


def IsValidMacAddress(mac):
    return re.match('[0-9a-f]{2}([-:]?)[0-9a-f]{2}(\\1[0-9a-f]{2}){4}$', mac.lower())
