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
KASA Monitor command implementations.

Commands use async device interface for efficient concurrent operations.
"""

from .async_poll import AsyncPoll

# Legacy command aliases for backwards compatibility
Poll = AsyncPoll
Interactive = None  # Removed - use python-kasa CLI tools instead
Status = None  # Removed - use python-kasa CLI tools instead

__all__ = ['AsyncPoll', 'Poll', 'Interactive', 'Status']
