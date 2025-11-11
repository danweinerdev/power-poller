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

class MessageError(Exception):
    message = 'Whoops! Something went wrong'

    def __init__(self, message=None, *args, **kwargs):
        self.message = message or self.message
        if args:
            self.message = message.format(*args)
        elif kwargs:
            self.message = message.format(**kwargs)


class ExecutorError(MessageError):
    message = 'An error occurred in the Executor'
