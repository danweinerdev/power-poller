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

from datetime import datetime, UTC
from typing import Union
from .config import Config, ConversionFailure, ConvertValue, DefaultValue
from .database import InfluxDatabase
from influxdb_client import Point, WritePrecision

from requests.exceptions import ConnectionError, HTTPError, Timeout, TooManyRedirects, RequestException
from urllib3.exceptions import ConnectTimeoutError, MaxRetryError, NewConnectionError, ProtocolError

MetricValue = Union[int, float, str]


class Metric(object):

    def __init__(self, entity: str, measurement: str, tags: dict = None):
        """
        Constructor for a single metric.

        :param entity: Identifier for the config entry which generated this measurement.
        :param measurement: String describing what this metric is recording.
        :param tags: List of tags associated with this metric.
        """
        self.entity = entity
        self.timestamp = datetime.now(UTC)
        self.tags = tags or {}
        self.measurement = measurement
        self.fields = {}

    def __str__(self):
        return '<Metric({}): {}>'.format(self.measurement, self.entity)

    def AddField(self, field: str, value: MetricValue) -> None:
        self.fields[field] = {'original': value, 'clean': None}

    @staticmethod
    def Sanitize(measurement):
        """
        Sanitize a measurement name. Removes whitespace and other separators and replaces
        them with an '_" (underscore).

        :param measurement: Measurement name
        :return: Sanitized string
        """
        for c in [' ', '.']:
            measurement = measurement.replace(c, '_')
        return measurement

    @staticmethod
    def TimeStamp(now=None):
        """
        Serialize a datetime into a ISO8601 date-time stamp.

        :param now: Optional datetime value. If None will default to utcnow().
        :return: Serialized date-time string.
        """
        now = now or datetime.now(UTC)
        return now.strftime('%Y-%m-%dT%H:%M:%SZ')


class MetricPipeline(object):

    DEFAULT_BATCH_SIZE = 10

    def __init__(self, config: Config, batchSize=DEFAULT_BATCH_SIZE, logger=None):
        """
        Constructor for the measurement pipeline.

        :param config: Full config from the Executor context.
        :param batchSize: Optional batch size. How many messages should be grouped into a single
                          call to the database.
        :param logger: Optional logger instance. If no instance is provided log messages will
                       be skipped.
        """
        self.config = config
        self.logger = logger
        self.batchSize = int(batchSize)
        self.queue = []
        self.database = None
        self.shutdown = False

    def __call__(self, metrics: list):
        self.Enqueue(metrics)

    def Enqueue(self, metrics: list):
        """
        Append a set of metrics onto the metric queue for sending to the backend database.
        Note that metrics are attempted after every iteration however if the database is
        unresponsive or unavailable then the metrics will remain in the queue until the
        database accepts them.

        :param metrics: A single metric or list of metrics which should be appended to the
                        end of the queue.
        :return: None
        """
        if self.shutdown:
            return
        if not isinstance(metrics, (list, set)):
            metrics = [metrics]
        for metric in metrics:
            if not isinstance(metric, Metric):
                if self.logger:
                    self.logger.warning('Invalid metric sent to the queue')
                continue
            skip = False
            for field in metric.fields:
                value = metric.fields[field]['original']
                hint = self.config.GetField(metric.measurement, field)
                try:
                    metric.fields[field]['clean'] = ConvertValue(value, hint=hint)
                except KeyError:
                    if self.logger:
                        self.logger.warning("Invalid metric '{}'".format(metric.measurement))
                        skip = True
                    break
                except ConversionFailure:
                    if self.logger:
                        self.logger.warning("Unable to convert measurement '{}' value '{}'"
                            .format(metric.measurement, value))
                    metric.fields[field]['clean'] = DefaultValue(hint)
            if not skip:
                self.queue.append(metric)

    def Flush(self):
        """
        Flush the queue to the database. If any messages fail to send the queue will be unwound
        and retried on the next iteration pass. The inner loop will automatically handle
        batching based on the configured batch size.

        :return: None
        """
        if self.shutdown:
            return (False, 0)

        sent = 0
        while not self.IsEmpty():
            # Copy a slice of metrics from the front of the queue equal to the batch size
            # or the queue size, whatever is less. We copy here to ensure we can successfully
            # send the metrics before popping them off the queue.
            count = min(len(self.queue), self.batchSize)

            points = []
            for i in range(count):
                metric = self.queue[i]

                if len(metric.fields) == 0:
                    continue

                point = Point(metric.measurement)
                for tag, value in metric.tags.items():
                    point.tag(tag, value)
                for name, field in metric.fields.items():
                    point.field(name, field['clean'])
                point.time(metric.timestamp, WritePrecision.MS)
                points.append(point)

            try:
                if not self.database:
                    dbtype, config = self.config.GetDatabase()
                    if dbtype == 'influxdb':
                        self.database = InfluxDatabase(config, logger=self.logger, precision='ms')
                    else:
                        raise RuntimeError("Unknown database type '{}'".format(dbtype))

                self.database.Write(points)
                self.database.Flush()
            except RuntimeError as e:
                # Push any RuntimeErrors up to the main loop to handle. These usually indicate
                # that we need to crash.
                raise e
            except (Timeout, ConnectTimeoutError) as e:
                if self.logger:
                    self.logger.warning('Failed to connect to database')
                return (False, sent)
            except (ConnectionError, NewConnectionError, MaxRetryError, ProtocolError) as e:
                if self.logger:
                    self.logger.warning('Network error communicating with database: {}'.format(e))
                return (False, sent)
            except HTTPError as e:
                statusCode = e.response.status_code if e.response else 0
                if 500 <= statusCode < 600:
                    if self.logger:
                        self.logger.warning('Server side HTTP error ({}) communicating with database'.format(statusCode))
                else:
                    if self.logger:
                        self.logger.error('Unexpected HTTP Error ({}) from database'.format(statusCode))
                return (False, sent)
            except TooManyRedirects as e:
                if self.logger:
                    self.logger.warning('Unexpected redirects communicating with database')
                return (False, sent)
            except RequestException as e:
                if self.logger:
                    self.logger.error('Unexpected request error: {}'.format(e))
                return (False, sent)
            except Exception as e:
                if self.logger:
                    self.logger.error('Unhandled error sending metrics: {}'.format(e))
                return (False, sent)

            # If an exception was not raised it is likely that the call to sending the metrics to
            # the database succeeded as planned and we should count the metrics as part of the total
            # sent.
            sent += len(points)

            # Ensure we purge the first count number of elements from the queue by popping it the
            # required number of times.
            while count > 0:
                self.queue.pop(0)
                count -= 1

        return (True, sent)

    def IsEmpty(self):
        """
        Predicate to check if the message queue is empty.

        :return:
        """
        return len(self.queue) == 0

    def Reload(self, config):
        """
        Reload the configuration. This will trigger a full flush of the message queue to try
        and send the remaining messages to the database before re-initializing the connection.

        In the event of a failure the config will not be overwritten and reloaded.
        In the event of a successful flush, the database connection will be reconnected to handle
        a possible change in the database config.

        :param config: New config object from the Executor context.
        :return: None
        """
        if self.logger:
            self.logger.warning('Reloading metrics pipeline')
        try:
            self.Flush()
            if self.database:
                self.database.Close()
            self.database = None
            self.config = config
        except Exception as e:
            if self.logger:
                self.logger.warning('Error during reload: {}'.format(e))

    def Shutdown(self, crash=False):
        """
        Shutdown the message pipeline. This will flush the remaining metrics to the database
        before disconnecting the connection and setting the shutdown flag. If the database
        is not available this call wil not block and data could be lost.

        :return: None
        """
        try:
            self.Flush()
            if self.database:
                self.database.Close()
            self.database = None
        except Exception as e:
            if self.logger and not crash:
                self.logger.warning('Error during metric pipeline shutdown: {}'.format(e))
        self.shutdown = True
