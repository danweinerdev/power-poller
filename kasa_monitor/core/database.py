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

import logging

try:
    from influxdb_client import InfluxDBClient, Point
    from influxdb_client.client.write_api import SYNCHRONOUS
    INFLUXDB_SUPPORTED = True
except ImportError:
    INFLUXDB_SUPPORTED = False


class Database(object):

    def Close(self):
        raise NotImplementedError

    def Initialize(self):
        raise NotImplementedError

    @staticmethod
    def CheckConfig(config: dict, required: list = None, optional: list = None) -> None:
        pass

    def Write(self, metrics):
        raise NotImplementedError


class InfluxDatabase(Database):
    REQUIRED_KEYS = [('bucket', str), ('org', str), ('server', str), ('token', str)]
    OPTIONAL_KEYS = [('protocol', ['http', 'https']), ('port', int)]

    def __init__(self, config: dict, precision: str = 'ms', logger: logging.Logger = None):
        """
        InfluxDB database constructor.

        This class will throw a RuntimeError if InfluxDB is not supported in the
        current environment.

        :param config: Database configuration from the main configuration object.
        :param precision: Timestamp precision. Defaults to ms.
        :param logger: Logger instance to use otherwise logging will be ignored.
        """
        if not INFLUXDB_SUPPORTED:
            raise RuntimeError('InfluxDB is not installed')

        self.CheckConfig(config,
                         required=self.REQUIRED_KEYS,
                         optional=self.OPTIONAL_KEYS)

        self.config = config
        self.precision = precision
        self.logger = logger
        self.client = None

        self.writer = None
        self.query = None

    def Close(self):
        """
        Close the InfluxDB connection if one is open.

        This function will only throw in the event that InfluxDB is not supported
        otherwise it is safe to call during shutdown.

        :return: None
        """
        if not INFLUXDB_SUPPORTED:
            raise RuntimeError('InfluxDB is not installed')
        if self.client:
            try:
                self.client.close()
            except Exception as e:
                if self.logger:
                    self.logger.warning('Failed to close influx db connection: {}'.format(e))

    def Flush(self):
        """
        Flush the database.

        :return:
        """
        if not INFLUXDB_SUPPORTED:
            raise RuntimeError('InfluxDB is not installed')
        if not self.writer and not self.Initialize():
            return
        self.writer.flush()

    def Initialize(self):
        """
        Initialize the InfluxDB connection.

        This class will throw a RuntimeError if InfluxDB is not supported in the
        current environment.

        :return: True on success or False if the connection fails.
        """
        if not INFLUXDB_SUPPORTED:
            raise RuntimeError('InfluxDB is not installed')
        try:
            self.client = InfluxDBClient(
                url="{}://{}:{}".format(
                    self.config.get('protocol', 'https'),
                    self.config['server'],
                    self.config.get('port', 443)),
                token=self.config['token'],
                org=self.config['org'])
        except KeyError as e:
            # If any of the configuration options are missing we need to trigger a fatal
            # error because we cannot recover from this.
            raise RuntimeError("Missing configuration option '{}'".format(e.args[0]))
        except Exception as e:
            if self.logger:
                self.logger.error('Failed to initiate influx db connection: {}'.format(e))
            self.client = None
            return False

        writeOptions = SYNCHRONOUS
        writeOptions.batch_size = 16

        self.writer = self.client.write_api(write_options=writeOptions)
        self.query = self.client.query_api()

        return True

    def Write(self, metrics):
        """
        Write metrics to the InfluxDB database.

        This class will throw a RuntimeError if InfluxDB is not supported in the
        current environment.

        :param metrics: metrics object generated that should be sent to InfluxDB.
        :return: True on success or False on failure.
        """
        if not INFLUXDB_SUPPORTED:
            raise RuntimeError('InfluxDB is not installed')
        if not self.writer and not self.Initialize():
            return False
        self.writer.write(bucket=self.config['bucket'], record=metrics)
        return True
