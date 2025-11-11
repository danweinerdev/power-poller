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

from configparser import ConfigParser
import os
from .exceptions import MessageError


class ConfigError(MessageError):
    message = 'Invalid configuration'


class InvalidConfigError(ConfigError):
    pass


class ConversionFailure(MessageError):
    message = 'Invalid type conversion requested'


def ConvertBoolean(value):
    """
    Attempt to convert value to a boolean. If the value is not a possible boolean
    string the result will return None instead of True or False.

    :param value: Value which should be checked as a boolean.
    :return: True or False if the value is recognized, or None if it is not a valid
            boolean string.
    """
    value = str(value).lower()
    if value in ['yes', '1', 'true']:
        return True
    if value in ['no', '0', 'false']:
        return False
    return None


def ConvertHashType(value):
    """
    Attempt to convert a space separated series of key=value pairs into a dictionary
    of pairs. If any value fails to split successfully an error will be raised.

    :param value: Space delimited string of key-value pairs
    :return: Dictionary of key-value pairs.
    """
    collection = dict()
    for option in value.split():
        try:
            k, v = option.split('=')
        except ValueError:
            raise ConversionFailure("Invalid option '{}' for key-value pair: {}"
                .format(option, value))
        collection[k] = v.strip()
    return collection


def ConvertValue(value, hint=None):
    """
    Attempt to convert value to a supported type. Hint can be used to influence
    how the value is coerced. In the event of a failure an error will be raised.

    Supported hints are:
    - Array
    - Hash
    - Integer
    - Float
    - Boolean
    - String

    :param value: Value to be coerced.
    :param hint: Optional hint to indicate desired type.
    :return: Value coerced to type.
    """
    if isinstance(value, str):
        value = value.strip()
    try:
        if hint is not None:
            if hint == Config.ARRAY_TYPE:
                return value.split()
            elif hint == Config.HASH_TYPE:
                return ConvertHashType(value)
            elif hint == Config.INT_TYPE:
                if isinstance(value, str) and '.' in value:
                    return int(float(value))
                return int(value)
            elif hint == Config.BOOL_TYPE:
                return ConvertBoolean(value)
            elif hint == Config.FLOAT_TYPE:
                return float(value)
            elif hint == Config.STRING_TYPE:
                return value
            raise ConversionFailure
        else:
            b = ConvertBoolean(value)
            if b is not None:
                return b
            v = float(value)
            if isinstance(value, str) and '.' in value:
                return v
            return int(v)
    except (TypeError, ValueError):
        if hint is not None:
            raise ConversionFailure('Failed to convert the given value to the requested type')
        else:
            if ConvertBoolean(value) is None:
                return value
            return ConvertBoolean(value)


def DefaultValue(hint):
    """

    :param hint:
    :return:
    """
    if hint == Config.ARRAY_TYPE:
        return []
    elif hint == Config.HASH_TYPE:
        return {}
    elif hint == Config.INT_TYPE:
        return int(0)
    elif hint == Config.BOOL_TYPE:
        return False
    elif hint == Config.FLOAT_TYPE:
        return float(0)
    elif hint == Config.STRING_TYPE:
        return str()
    raise ConversionFailure


class Config(object):
    REQUIRED = True
    OPTIONAL = False

    ARRAY_TYPE = 'array'
    BOOL_TYPE = 'bool'
    FLOAT_TYPE = 'float'
    HASH_TYPE = 'hash'
    INT_TYPE = 'int'
    STRING_TYPE = 'string'

    GLOBAL_SECTION = 'global'
    INFLUXDB_SECTION = 'influxdb'
    SUPPORTED_DATABASES = [INFLUXDB_SECTION]
    DATABASE_FIELDS = {
        INFLUXDB_SECTION: [
            ('database', STRING_TYPE),
            ('port', INT_TYPE),
            ('server', STRING_TYPE),
            ('ssl', BOOL_TYPE),
            ('verify', BOOL_TYPE),
            ('org', STRING_TYPE),
            ('token', STRING_TYPE),
            ('bucket', STRING_TYPE)
        ]}
    ENTRY_FIELDS = [
        ('measurements', ARRAY_TYPE, REQUIRED),
        ('tags', HASH_TYPE, OPTIONAL)
    ]

    def __init__(self, path, root):
        """
        Constructor for the config object. The path value should represent an
        existing config file that the parser should read in. The root corresponds
        to the application defined root element indicating which config elements
        should be parsed form the config file.

        :param path: Path o the config file.
        :param root: Root element in the global section indicating all entries
                     to parse.
        """
        self.path = path
        self.root = root
        self.config = {}
        self.database = None

    def GetDatabase(self):
        """
        Retrieve the database configuration from the underlying config.

        :return: A tuple consisting of the database type and the respective configuration.
        """
        if not self.IsLoaded():
            self.Load()
        return self.database, self.config[self.database]

    def GetField(self, measurement, field):
        """
        Query the main fields list and return the type hint for the given field if it
        exists. The function will throw a KeyError in the event the field does not
        exist or is an invalid submission.

        :param measurement: name of the measurement with the given field
        :param field: String field to lookup in the config.
        :return: Type hint if the field is known.
        """
        if measurement is None or field is None:
            raise KeyError('Unknown measurement or field')

        if not self.IsLoaded():
            self.Load()

        if measurement not in self.config['measurements']:
            raise KeyError("Unknown measurement '{}'".format(measurement))

        hint = self.config['measurements'][measurement].get(field, None)
        if not hint:
            raise KeyError("Unknown field '{}' on measurement '{}'".format(
                field, measurement))

        return hint

    def GetRoot(self):
        """
        Returns the application root of the configuration file.

        :return: Application data from the root element.
        """
        if not self.IsLoaded():
            self.Load()
        if self.path is None:
            return {}
        return self.config[self.root]

    def GetTags(self, entity):
        """
        Lookup the tags for an entity if it is known. I fthe entity is not known a
        key error is returned. If the entity has no tags an empty list will be
        returned.

        :param entity: An entity to lookup in the config.
        :return: Tag list if the entity is known or an empty list if no tags are given.
        """
        if entity is None:
            raise KeyError('Unknown entity')
        if not self.IsLoaded():
            self.Load()
        if self.path is None:
            return {}
        return self.config[self.root][entity].get('tags', [])

    def IsLoaded(self):
        """
        Predicate checking if the cofnig has already been loaded.

        :return: True or False if the config has been loaded.
        """
        if self.path is None:
            return True
        return len(self.config) > 0

    def Load(self):
        """
        Load the configuration for the given file.

        On an error, this function will raise an error indicating what the failure
        was when it occurred.

        :return: None
        """
        if self.IsLoaded():
            return

        if not os.path.isfile(self.path):
            raise ConfigError('Config file does not exist')

        parser = ConfigParser()
        try:
            parser.read(self.path)
        except IOError:
            raise ConfigError('Failed to parse configuration')

        if not parser.has_section(self.GLOBAL_SECTION):
            raise InvalidConfigError('No global section')

        self.RequiredFields(parser, self.GLOBAL_SECTION, ['database', self.root])
        self.database = parser.get(self.GLOBAL_SECTION, 'database')

        if not self.database or self.database not in self.SUPPORTED_DATABASES:
            raise InvalidConfigError('Invalid or unsupported database value')

        self.config[self.database] = {}
        for field, hint in self.DATABASE_FIELDS[self.database]:
            if parser.has_option(self.database, field):
                try:
                    self.config[self.database][field] = ConvertValue(
                        parser.get(self.database, field),
                        hint=hint)
                except ConversionFailure:
                    raise InvalidConfigError("Invalid field '{}' expected type '{}'"
                        .format(field, hint))

        root = parser.get(self.GLOBAL_SECTION, self.root)
        if not root:
            raise InvalidConfigError("Missing config for root '{}".format(self.root))

        self.config['measurements'] = {}
        self.config[self.root] = {}
        for field in root.split():
            self.config[self.root][field] = {}

        if len(self.config[self.root]) == 0:
            raise InvalidConfigError("Root element '{}' has no entries".format(self.root))

        for device in self.config[self.root]:
            if not parser.has_section(device):
                raise InvalidConfigError("Missing device configuration '{}'".format(device))

        measurements = {}
        for entry in self.config[self.root].keys():
            self.RequiredFields(parser, entry, [k for k, v, r in self.ENTRY_FIELDS if r])
            self.OptionalFields(parser, entry, [k for k, v, r in self.ENTRY_FIELDS if not r])
            self.config[self.root][entry] = {'device': entry}

            for field, hint, required in self.ENTRY_FIELDS:
                if parser.has_option(entry, field):
                    try:
                        self.config[self.root][entry][field] = ConvertValue(
                            parser.get(entry, field),
                            hint=hint)
                    except ConversionFailure:
                        raise InvalidConfigError("Invalid field '{}' expected type '{}'"
                            .format(field, hint))
                else:
                    if required:
                        raise InvalidConfigError("Section '{}' missing required field '{}'".format(
                            entry, field))
                    self.config[self.root][entry][field] = DefaultValue(hint)

            for option in parser.options(entry):
                if option in self.config[self.root][entry]:
                    continue
                try:
                    self.config[self.root][entry][option] = ConvertValue(
                        parser.get(entry, option))
                except ConversionFailure:
                    raise InvalidConfigError("Invalid field '{}' unable to determine type"
                        .format(option))

            for measurement in self.config[self.root][entry]['measurements']:
                if measurement not in measurements:
                    measurements[measurement] = []

        for measurement in measurements.keys():
            if not parser.has_section(measurement):
                raise InvalidConfigError("Unknown measurement '{}' in config".format(measurement))
            for option in parser.options(measurement):
                if measurement not in self.config['measurements']:
                    self.config['measurements'][measurement] = dict()
                if 'option' not in self.config['measurements'][measurement]:
                    self.config['measurements'][measurement][option] = self.ParseOption(parser, measurement, option)
                    measurements[measurement].append(option)
                else:
                    raise InvalidConfigError("Duplicate measurement definition for '{}' in section '{}'"
                        .format(option, measurement))

        for entry, values in self.config[self.root].items():
            measurementMap = {}
            for measurement in values['measurements']:
                if measurement not in measurements:
                    raise InvalidConfigError("Unknown measurement '{}' in entry '{}'".format(
                        measurement, entry))
                measurementMap[measurement] = measurements[measurement]
            values['measurements'] = measurementMap

    @staticmethod
    def ParseOption(parser, section, option):
        """
        Parse a given option as a defined type. This attempts to match the value of an option
        to a known type which can be coerced into a native value.

        :param parser: ConfigParser instance
        :param section: Section containing fields
        :param option: Option with a value representing a known coerseable type.
        :return:
        """
        if not parser.has_option(section, option):
            raise ConfigError("Option '{}' does not exist in section '{}'"
                .format(option, section))
        value = parser.get(section, option)
        if value in [Config.BOOL_TYPE, Config.FLOAT_TYPE,
                     Config.INT_TYPE, Config.STRING_TYPE]:
            return value
        raise InvalidConfigError("Invalid type '{}' for option '{}'".format(value, option))

    @staticmethod
    def RequiredFields(parser, section, fields):
        """
        Check a section for required fields. This function will raise an error if all
        required fields are not present within the given section.

        :param parser: ConfigParser instance
        :param section: Section containing fields
        :param fields: Array of fields (strings) which should be validated for existence.
        :return: None
        """
        for field in fields:
            if not parser.has_option(section, field):
                raise InvalidConfigError("Section '{}' missing required field '{}'"
                    .format(section, field))

    @staticmethod
    def OptionalFields(parser, section, fields):
        pass

    def Reload(self):
        """
        Clean the config state and reload the config from disk.

        :return: None
        """
        self.config = {}
        self.database = None
        self.Load()
