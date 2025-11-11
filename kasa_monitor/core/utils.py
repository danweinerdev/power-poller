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

from datetime import datetime, timedelta
import os
import selectors
import subprocess
from .exceptions import ExecutorError

if os.name == 'posix':
    import fcntl
    import grp
    import pwd
if os.name == 'nt':
    import ctypes


class Callbacks(object):
    """
    Wrapper to allow for the Executor setup o register callbacks for secondary
    commands on the argument parser.
    """

    def __init__(self, command):
        """
        Constructor.

        :param command: Name of the primary run command that should be treated as a
                        protected value.
        """
        self.command = command
        self.callbacks = {}
        self.parsers = None

    def __call__(self, parsers, callback):
        """
        Callable which accepts the ArgumentParser from the Executor and passes this
        wrapper back to the caller to register their sub-commands.

        :param parsers: ArgumentParser sub-parser.
        :param callback: Uer given callback for registering subcommands.
        :return:
        """
        self.parsers = parsers
        callback(self)

    def Register(self, name, callback, **kwargs):
        """
        Register a callback and add a name to it.

        :param name: Name of the sub-command callback.
        :param callback: Callbable object.
        :return: Sub-Parser object
        """
        if not callable(callback):
            raise ExecutorError('Callback is not callable')
        if name == self.command:
            raise ExecutorError("Cannot register a secondary '{}' command".format(name))
        for [k, _] in self.callbacks.items():
            if name == k:
                raise ExecutorError("Command '{}' already registered".format(name))
        self.callbacks[name] = callback
        parser = self.parsers.add_parser(name, **kwargs)
        parser.add_argument('--config', help='Path to the config file')
        return parser


def CloseDescriptor(fd):
    """
    Close a given descriptor. If the object has its own 'close' method that
    will be used otherwise everything will be passed to 'os.close'.

    :param fd: File descriptor or 'closeable' object.
    :return: None
    """
    try:
        if hasattr(fd, 'close'):
            fd.close()
        else:
            os.close(fd)
    except (IOError, OSError):
        pass


def Command(command, stderr=True, cwd=None):
    """
    Execute an external command with the given working directory. The result will
    be the STDOUT data and the return code. If 'stderr' is set to true the STDERR
    output will be piped to STDOUT otherwise it will be ignored.

    :param command: Command parameters to execute.
    :param stderr: Boolean indicating whether STDERR should be piped to STDOUT.
    :param cwd: Current working directory.
    :return: Tuple of process exitcode and STDOUT data.
    """
    process = None
    output = []
    try:
        process = subprocess.Popen(command,
            cwd=cwd,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT if stderr else os.devnull)

        while True:
            if process.poll() is not None:
                break
            for line in iter(process.stdout.readline, b''):
                if len(line.strip()) > 0:
                    output.append(line.strip())
    except KeyboardInterrupt:
        pass
    except OSError:
        raise

    return process.poll(), output


def GetErrorMessage(err):
    if os.name == 'nt':
        FORMAT_MESSAGE_FROM_SYSTEM = 0x00001000
        msg_buffer = ctypes.create_unicode_buffer(256)
        ctypes.windll.kernel32.FormatMessageW(
            FORMAT_MESSAGE_FROM_SYSTEM,
            None,
            err,
            0,
            msg_buffer,
            len(msg_buffer),
            None)

        return msg_buffer.value.strip()
    else:
        return os.strerror(err)


def GetGroupId(group):
    """
    On a Linux system attempt to get the GID value for a give 'group'. This uses
    a system call to obtain this data.

    :param group: Group name to lookup.
    :return: GID value if the group is found, otherwise None
    """
    if isinstance(group, int):
        return group
    if os.name == 'posix':
        try:
            return grp.getgrnam(group).gr_gid
        except KeyError:
            pass
    return None


def GetUserId(user):
    """
    On a Linux system attempt to get the UID value for a give 'user'. This uses
    a system call to obtain this data.

    :param user: User name to lookup.
    :return: UID value if the user is found, otherwise None
    """
    if isinstance(user, int):
        return user
    if os.name == 'posix':
        try:
            return pwd.getpwnam(user).pw_uid
        except KeyError:
            pass
    return None


def RedirectStream(source, target=None):
    """
    Redirect a source file descriptor to a new target file descriptor. If no target
    is specified the source will be redirected to the /dev/null object.

    :param source: Source file descriptor
    :param target: Target file descriptor. If None is provided /dev/null is used.
    :return: None
    """
    if os.name == 'posix':
        if target is None:
            target = os.open(os.devnull, os.O_RDWR)
        else:
            target = target.fileno()
        os.dup2(target, source.fileno())


def Select(rds, wrts, timeout, logger=None):
    """
    Select on the given sockets for events at the given timeout period. This is a wrapper
    around the system call select for handling a set of file descriptors. The function will
    return None in the event of a failure of the resulting list of descriptors which an
    event occurred on.

    :param rds: Set of reader descriptors.
    :param wrts: Set of writer descriptors.
    :param timeout: Timeout (in seconds) value to wait for an event.
    :param logger: Optional logger instance in the event of errors.
    :return: None if a failure occurred, or the descriptors which an event occurred.
    """
    if not isinstance(rds, list):
        rds = [rds]
    if not isinstance(wrts, list):
        wrts = [wrts]

    start = datetime.now()
    end = start + timedelta(seconds=timeout)
    selector = selectors.DefaultSelector()
    res = None

    try:
        for rd in rds:
            selector.register(rd, selectors.EVENT_READ)
        for wr in wrts:
            selector.register(wr, selectors.EVENT_WRITE)

        while datetime.now() < end:
            res = selector.select(timeout=0.1)
            if res:
                break

    except os.error as e:
        if logger:
            logger.error('OSError: [{}] {}'.format(e.errno, GetErrorMessage(e.errno)))
        return None
    except KeyboardInterrupt:
        return None
    finally:
        try:
            selector.close()
        except Exception as e:
            if logger:
                logger.error('Exception during selector close: {}'.format(e))

    if logger:
        duration = datetime.now() - start
        logger.debug('Interval completed in: {}'.format(duration.total_seconds()))

    return res


def SetNonBlocking(fd):
    """
    Set file descriptors to non-blocking.

    :param fd: File descriptor object
    :return: None
    """
    if os.name == 'posix':
        fl = fcntl.fcntl(fd, fcntl.F_GETFL)
        fcntl.fcntl(fd, fcntl.F_SETFL, fl | os.O_NONBLOCK)
    if os.name == 'nt':
        fd.setblocking(False)


def SetProcessOwner(user, group, logger=None):
    """
    Set the given user and group as the process owner. The current process owner
    must have access to set the new user/group combination. This is usually only
    used in the event root needs to drop privileges to a non-privileged user after
    a fork has occurred.

    :param user: Integer representing the new user owner.
    :param group: Integer representing the new group owner.
    :param logger: Optional logger instance in the event of errors.
    :return: None
    """
    if os.name == 'posix':
        try:
            if user is not None:
                os.setuid(user)
        except OSError as e:
            if logger:
                logger.error("Failed to set process user '{}': [{}] {}".format(
                    user, e.errno, os.strerror(e.errno)))
        try:
            if group is not None:
                os.setgid(group)
        except OSError as e:
            if logger:
                logger.error("Failed to set process group '{}': [{}] {}".format(
                    group, e.errno, os.strerror(e.errno)))


def SetProcessUmask(umask, logger=None):
    """
    Set the process umask.

    :param umask: Selected umask.
    :param logger: Optional logger instance in the event of errors.
    :return: None
    """
    if os.name == 'posix':
        try:
            os.umask(umask)
        except OSError as e:
            if logger:
                logger.error('Failed to set umask: [{}] {}'.format(
                    e.errno, os.strerror(e.errno)))
