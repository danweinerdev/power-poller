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

from kasa_monitor.core import ConversionFailure, Metric, Result
from kasa_monitor.devices import LoadDevice, ConnectionError, IsValidIPv4


def ProcessDevice(pipeline, name, config, logger=None):
    address = config['address']
    if not IsValidIPv4(address):
        if logger:
            logger.error('Invalid device configuration: {}'.format(name))
        return False

    try:
        device = LoadDevice(address, logger=logger)
    except ConnectionError as e:
        if logger:
            logger.warning('Failed to connect to: {}'.format(address))
        return True

    if not device.HasEmeter():
        if logger:
            logger.warning("Device '{}' does not support electronic metering".format(
                device.GetAlias()))
        return False

    emeter = device.GetEmeter()
    try:
        result = emeter.GetRealtime(cache=False)
    except ConnectionError:
        if logger:
            logger.warning('Failed to get realtime data for: {}'.format(address))
        return True

    if 'err_code' not in result or result['err_code'] != 0:
        if logger:
            logger.error("Failed to load device '{}' emeter data".format(device.GetAlias()))
        return False

    tags = {'device': config['device']}
    tags.update(config.get('tags', {}))

    metric = Metric(name, 'emeter', tags=tags)
    measurements = config['measurements'][metric.measurement]

    for key, value in result.items():
        if key not in measurements:
            continue
        metric.AddField(key, value)

    try:
        pipeline(metric)
    except ConversionFailure:
        pass

    return True


def Poll(config, logger, pipeline):
    """

    :param config:
    :param logger:
    :param pipeline:
    :return:
    """
    success = True
    for device, cfg in config.items():
        try:
            ProcessDevice(pipeline, device, cfg, logger=logger)
        except ConnectionError as e:
            if logger:
                logger.error("Failed to connect to '{}': {}".format(device, e.message))

    return Result.SUCCESS if success else Result.FAILURE
