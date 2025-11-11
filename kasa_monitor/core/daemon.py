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
import sys
from .utils import GetGroupId, GetUserId, RedirectStream, SetProcessOwner, SetProcessUmask


class Daemonize(object):

    DEFAULT_UMASK = 0
    EXIT_FAILURE = 1
    EXIT_SUCCESS = 0

    def __init__(self, launch, logger, user=None, group=None,
                 stdin=None, stdout=None, stderr=None,
                 umask=DEFAULT_UMASK):
        """
        Constructor for the daemon context.

        :param launch: Boolean indicating if the process should fork.
        :param logger: Logger instance shared from the caller.
        :param user: Optional user which should own the forked process. Only used
                     if the process daemonizes.
        :param group: Optional group which should own the forked process. Only used
                     if the process daemonizes.
        :param stdin: New STDIN target file descriptor.
        :param stdout: New STDOUT target file descriptor.
        :param stderr: New STDERR target file descriptor.
        :param umask: Process umask.
        """
        self.launch = bool(launch)
        self.logger = logger
        self.umask = umask
        self.gid = GetGroupId(group) if group else None
        self.uid = GetUserId(user) if user else None
        self.stdin = stdin
        self.stdout = stdout
        self.stderr = stderr
        self.launched = False

    def __enter__(self):
        """
        Initialize the daemon process. If the process calls for daemonizing this
        context will result in the application forking from the parent. It performs
        a standard double-fork and will call the relevant setUID/GUID/etc functions
        determined by the constructor.

        :return: None
        """
        if self.logger:
            if self.uid:
                self.logger.debug('Launching with UID={}'.format(self.uid))
            if self.gid:
                self.logger.debug('Launching with GID={}'.format(self.gid))
            self.logger.debug('--- START DAEMON CONTEXT ---')

        self.pid = os.getpid()
        if self.launch:
            self.__Daemonize()

        SetProcessUmask(self.umask, logger=self.logger)
        SetProcessOwner(self.uid, self.gid, logger=self.logger)

    def __exit__(self, exc, value, tb):
        """
        Shuts down the process. If an exception resulted in the shutdown the resulting
        exit code will indicate a failure otherwise an exit code of '0' is returned
        indicating success.

        :param exc: Exception type
        :param value: Exception message
        :param tb: Traceback of the exception
        :return: None
        """
        self.logger.debug('--- STOP DAEMON CONTEXT ---')
        if exc:
            sys.exit(self.EXIT_FAILURE)
        else:
            sys.exit(self.EXIT_SUCCESS)

    def __Daemonize(self):
        """
        Daemonize the application by doing a standard double fork. This will result
        in the UID/GID/etc options being set after the fork.

        :return: None
        """
        if self.launch and not self.launched:
            # Don't fork if our parent PID is 1. This usually indicates that we
            # are already running as a Daemon or at least launched by init.
            if os.getppid() == 1:
                return

            try:
                self.__Fork('first fork')
            except OSError:
                exit(self.EXIT_FAILURE)

            os.setsid()

            try:
                self.__Fork('second fork')
            except OSError:
                exit(self.EXIT_FAILURE)

            self.launched = True
            RedirectStream(sys.stdin, self.stdin)
            RedirectStream(sys.stdout, self.stdout)
            RedirectStream(sys.stderr, self.stderr)

    def __Fork(self, error=None):
        """
        Wrapper around the form call to handle exiting from the parent cleanly.

        :param error: Error message indicating which fork this call represents this will
                      get added to an optional log message in the event of a failure.
        :return: None
        """
        try:
            if os.fork() > 0:
                exit(self.EXIT_SUCCESS)
        except OSError as e:
            if error and self.logger:
                self.logger.error("Fork error for '{}': [{}] {}".format(
                    error, e.errno, os.strerror(e.errno)))
            elif self.logger:
                self.logger.error('Failed to fork parent: [{}] {}'.format(
                    e.errno, os.strerror(e.errno)))
            raise
