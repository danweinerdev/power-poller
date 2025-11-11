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

import argparse
from datetime import datetime, UTC
import logging
from logging.handlers import TimedRotatingFileHandler
import os
import socket
import signal
import sys
from .config import Config, ConfigError
from .daemon import Daemonize
from .exceptions import ExecutorError
from .metrics import MetricPipeline
from .result import Result
from .utils import Callbacks, CloseDescriptor, Select, SetNonBlocking


class Executor(object):

    def __init__(self, args, root, callbacks=None):
        """
        Constructor for the Executor. It takes the argument parser args and the root
        configuration element from the caller and builds the corresponding data structures.

        :param args: Argument parser resultset.
        :param root: Root element in the configuration file determined by the caller.
        :param callbacks: Callback helper for handling user-generated sub-commands.
        """
        self.args = args
        self.callbacks = getattr(callbacks, 'callbacks', None)
        self.command = getattr(callbacks, 'command', None)
        self.action = getattr(args, 'command', None)
        self.debug = getattr(args, 'debug', False)
        self.config = Config(args.config, root)

        if not self.action or self.action == self.command:
            self.interval = int(args.interval)
            self.pidFile = args.pidfile
            self.logfile = args.logfile
            self.logger = self.SetupLogging(self.logfile, args.loglevel)
            self.context = Daemonize(args.daemon, self.logger,
                group=args.group,
                user=args.user)
            self.pipeline = MetricPipeline(self.config, logger=self.logger)
            self.__shutdown = False
            self.__rd, self.__wr = None, None
            self.__reload = False

    @staticmethod
    def Configure(parser):
        """
        Configure an argument parser with the necessary options for running the Executor.

        :param parser: Argument parser instance (can also be a sub-command)
        :return: None
        """
        parser.add_argument('--daemon', '-d', action='store_true', default=False,
            help='Daemonize the application (only supported on Linux).')
        parser.add_argument('--debug', action='store_true', default=False,
            help='Debug mode indicates that most of the daemonizing and process handling should skip')
        parser.add_argument('--interval', '-i', type=int, default=10,
            help='Interval at which the runnable callback should be executed')
        parser.add_argument('--logfile', '-f', required=False,
            help='Path to the log file which should be used.')
        parser.add_argument('--loglevel', '-l',
            default='INFO',
            choices=['DEBUG', 'INFO', 'ERROR', 'FATAL', 'WARNING'],
            help='Log level used for logging.')
        parser.add_argument('--stdout', '-o', action='store_true', default=False,
            help='Add a StreamHandler to STDOUT for the executor application.')
        parser.add_argument('--pidfile', '-p', required=False,
            help='Path to where the PID file should be written')
        parser.add_argument('--uid', '-u', dest='user', required=False,
            help='After daemonizing run the process as the following user id.')
        parser.add_argument('--gid', '-g', dest='group', required=False,
            help='After daemonizing run the process as the following group id.')
        parser.add_argument('config',
            help='Path to the config file')

    def Notify(self):
        """
        Notify a sleeping iterator loop that an event occurred which must be handled.
        This could be a signal or another error that the system needs to wakeup and react
        too.

        :return:
        """
        if self.__wr:
            if self.logger:
                self.logger.info('Notify Selector to wake up')
            try:
                if os.name == 'nt':
                    self.__wr.send(b'.')
                else:
                    os.write(self.__wr, b'.')
            except (IOError, OSError):
                pass

    def Reload(self):
        """
        Reload the config from disk and re-initialize the Metrics pipeline if necessary.
        This is a blocking operation which will force flush the Mertics internal queue
        and could potentially cause a long block if the underlying database is slow or
        unresponsive.

        :return: None
        """
        if self.__reload:
            self.config.Reload()
            self.pipeline.Reload(self.config)

        self.__reload = False

    def Run(self, callback):
        """
        Execute the service. This will run the daemon context and fork the application
        if daemoniing has been selected otherwise it will create all of the resources
        and begin the calls of the callback at the given interval.

        :param callback: Runnable function callback which should be executed every
                         interval. This function MUST be idempotent and return a
                         value from the Result enumerable type.
        :return:
        """
        if self.action is not None and self.action != self.command:
            if self.action in self.callbacks:
                return self.callbacks[self.action](self.config, self.args)
            print("Unknown command: {}".format(self.action))
            raise SystemExit

        if not self.Start(validate=True):
            raise SystemExit

        crash = False
        with self.context:
            try:
                if os.name == 'nt':
                    self.__rd, self.__wr = socket.socketpair()
                else:
                    self.__rd, self.__wr = os.pipe()
                SetNonBlocking(self.__rd)
                SetNonBlocking(self.__wr)
            except (IOError, OSError):
                self.logger.error('Failed to initialize notify socket')
                raise

            try:
                if not self.Start():
                    raise SystemExit
                failures = 0
                while not self.__shutdown:
                    self.Reload()
                    try:
                        result = callback(self.config.GetRoot(), self.logger, self.pipeline)
                        if result == Result.CANCEL:
                            self.logger.info('Callback initiated shutdown')
                            self.__shutdown = True
                        elif result == result.FAILURE:
                            failures += 1
                            if failures > 10:
                                self.logger.error('Callback returned too many failures. Initiating shutdown.')
                                self.__shutdown = True
                        failures = 0
                        if not self.__shutdown:
                            # This actually flushes the metric pipeline. We will attempt to batch
                            # everything we currently have and return a tuple indicating what was
                            # actually sent.

                            start = datetime.now(UTC)
                            flush, count = self.pipeline.Flush()
                            stop = datetime.now(UTC)

                            if flush and count > 0:
                                self.logger.info("Uploaded '{}' metrics in {:.3f}ms".format(
                                    count, (stop - start).total_seconds() * 1000))
                            elif self.logger:
                                self.logger.info('Flush to database failed. (Queue size: {} metrics)'.format(
                                    len(self.pipeline.queue)))

                            if Select(self.__rd, [], self.interval, logger=self.logger):
                                try:
                                    # TODO: Turn this into a Drain() function
                                    if os.name == 'nt':
                                        self.__rd.recv(64)
                                    else:
                                        os.read(self.__rd, 64)
                                except (IOError, OSError):
                                    pass
                    except KeyboardInterrupt:
                        self.logger.warning('Shutdown initiated')
                        self.__shutdown = True
            except RuntimeError as e:
                if self.logger:
                    self.logger.fatal('Fatal error in main loop: {}'.format(e))
                crash = True
            except Exception as e:
                if self.logger:
                    self.logger.exception('Unexpected exception in main loop: {}'.format(e))
            finally:
                if self.__rd:
                    CloseDescriptor(self.__rd)
                if self.__wr:
                    CloseDescriptor(self.__wr)
                if self.pipeline:
                    try:
                        self.pipeline.Shutdown(crash=crash)
                    except Exception as e:
                        self.logger.warning('Error during database shutdown: {}'.format(e))

    def SetupLogging(self, logfile, loglevel, stdout=False):
        """
        Configure the log handler for the executor using the given logfile and
        loglevel. If no logfile is specified then logging will default to a StreamHandler
        to STDOUT.

        :param logfile: Path to the given logfile.
        :param loglevel: String representing the LogLevel that should be chosen.
        :param stdout: Flag indicating that a StreamHandler logging to STDOUT should be
                       created.
        :return: Application logger
        """
        logger = logging.getLogger('monitor.lib.Executor')

        if isinstance(loglevel, int):
            level = loglevel
        else:
            level = logging.WARNING
            if loglevel == 'DEBUG':
                level = logging.DEBUG
            elif loglevel == 'INFO':
                level = logging.INFO
            elif loglevel == 'WARNING':
                level = logging.WARNING
            elif loglevel == 'ERROR':
                level = logging.ERROR
            elif loglevel == 'CRITICAL':
                level = logging.CRITICAL

        class LogFormatter(logging.Formatter):
            def formatTime(self, record, datefmt=None):
                # Use ISO 8601 format
                return datetime.fromtimestamp(record.created).isoformat()

        formatter = LogFormatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')

        if stdout or not logfile:
            console = logging.StreamHandler(stream=sys.stdout)
            console.setLevel(level)
            console.setFormatter(formatter)
            logger.addHandler(console)
        if logfile:
            handler = TimedRotatingFileHandler(
                logfile,
                backupCount=3,
                when='midnight')
            handler.setLevel(level)
            handler.setFormatter(formatter)
            logger.addHandler(handler)

        logger.setLevel(level)
        logger.propagate = False
        return logger

    def SignalHandler(self, sig, frame):
        """
        Signal handler for the application. The SignalHandler will be installed after
        the application has successfully started up. In the case of a daemonize that
        signals will be registered after to launch has completed.

        :param sig: Signal integer received from the system.
        :return: None
        """
        self.logger.info('Signal received: {}'.format(sig))
        if sig in [signal.SIGINT, signal.SIGTERM]:
            self.__shutdown = True
        elif sig == signal.SIGHUP:
            self.__reload = True
        self.Notify()

    def Start(self, validate=False):
        """
        Start the Executor: this includes creating any logfile, directories, etc. If
        the validate flag is specified, the Start will only create directories and
        attempt check the configuration and other values.

        By default if the application is set to daemonize then Start(...) will be
        called again after daemonizing.

        :param validate: Signal that the call to Start(...) should not actually start
                         but only check the configuration for starting.
        :return:
        """
        if not self.config.IsLoaded():
            try:
                self.config.Load()
            except ConfigError as e:
                self.logger.error('Failed to load config: {}'.format(e.message))
                return False

        if self.logfile:
            path = os.path.basename(self.logfile)
            if not os.path.exists(path):
                try:
                    os.makedirs(path)
                except (IOError, OSError):
                    self.logger.error('Failed to create the log folder')
                    return False
            try:
                with open(self.logfile, 'ab'):
                    pass
            except (IOError, OSError):
                self.logger.error('Failed to create the log file')
                return False

        if not self.debug and self.pidFile:
            path = os.path.basename(self.pidFile)
            if not os.path.exists(path):
                try:
                    os.makedirs(path)
                except (IOError, OSError):
                    self.logger.error('Failed to create PID file directory')
                    return False
            if not validate:
                try:
                    with open(self.pidFile, 'wb') as handle:
                        handle.write(str(os.getpid()).encode('ascii'))
                except (IOError, OSError):
                    self.logger.error('Failed to write pid file')
                    return False

        if not validate:
            self.logger.debug('Installing signal handlers')
            if os.name == 'posix':
                signal.signal(signal.SIGHUP, self.SignalHandler)
            signal.signal(signal.SIGINT, self.SignalHandler)
            signal.signal(signal.SIGTERM, self.SignalHandler)

        return True


def Execute(callback, root, command='run', commands=None):
    """
    Create an Executor instance that runs for the given callback. This function
    acts as an all in one to create the Executor, build the command parser, and
    parse the given args. From there everything will be passed to the Executor
    to be run.

    This function will end the program based on the result of the Run(...) call
    and does not return back to the caller.

    :param callback: Callable object which will be called from the main service
                     every given interval.
    :param root: Root node in the config that lists all other entries. This field
                 should be listed under globals.
    :param command: String which should be used for the sub-command if the caller
                    supplies a callback for more sub-commands. This should
                    represent the command which triggers the daemonized polling
                    process to start.
    :param commands: Callback which can register more one-off sub-commands.
    :return: None
    """
    parser = argparse.ArgumentParser(sys.argv[0])
    callbacks = Callbacks(command)

    if commands is not None:
        if not callable(commands):
            raise ExecutorError('sub-command callbacks must be callable')
        parsers = parser.add_subparsers(dest='command',
            help='Available sub-command options')
        Executor.Configure(parsers.add_parser(command,
            help='Execute the daemonized polling process'))
        callbacks(parsers, commands)
    else:
        Executor.Configure(parser)

    args = parser.parse_args()

    if not hasattr(args, 'config'):
        parser.print_help()
        sys.exit(1)

    executor = Executor(args, root, callbacks=callbacks)
    executor.Run(callback)
