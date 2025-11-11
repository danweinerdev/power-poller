#!/usr/bin/env python3

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
Legacy CLI entry point for backwards compatibility.

This file is maintained for backwards compatibility with existing scripts
and workflows. New users should use the recommended entry point:

    python -m kasa_monitor [command] [options]

This legacy entry point will continue to work but may be deprecated in
a future release.
"""

from kasa_monitor.commands import AsyncPoll
from kasa_monitor.core import Execute


if __name__ == '__main__':
    # Use AsyncPoll for concurrent device polling
    Execute(AsyncPoll, 'devices', command='run', commands=None)
