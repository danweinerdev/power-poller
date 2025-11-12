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
Comprehensive test suite for config parser.

Tests cover:
- Type conversion functions
- Config file parsing
- Error handling
- Database configuration (optional and required)
- Device and measurement validation
"""

import pytest
import tempfile
import os
from pathlib import Path

from kasa_monitor.core.config import (
    Config,
    ConfigError,
    InvalidConfigError,
    ConversionFailure,
    ConvertBoolean,
    ConvertHashType,
    ConvertValue,
    DefaultValue
)


class TestConvertBoolean:
    """Test boolean conversion function."""

    def test_convert_true_values(self):
        """Test conversion of various true values."""
        assert ConvertBoolean('yes') is True
        assert ConvertBoolean('YES') is True
        assert ConvertBoolean('Yes') is True
        assert ConvertBoolean('1') is True
        assert ConvertBoolean('true') is True
        assert ConvertBoolean('TRUE') is True
        assert ConvertBoolean('True') is True

    def test_convert_false_values(self):
        """Test conversion of various false values."""
        assert ConvertBoolean('no') is False
        assert ConvertBoolean('NO') is False
        assert ConvertBoolean('No') is False
        assert ConvertBoolean('0') is False
        assert ConvertBoolean('false') is False
        assert ConvertBoolean('FALSE') is False
        assert ConvertBoolean('False') is False

    def test_convert_invalid_values(self):
        """Test conversion of invalid boolean values."""
        assert ConvertBoolean('maybe') is None
        assert ConvertBoolean('2') is None
        assert ConvertBoolean('') is None
        assert ConvertBoolean('invalid') is None


class TestConvertHashType:
    """Test hash/dictionary conversion function."""

    def test_convert_single_pair(self):
        """Test conversion of single key=value pair."""
        result = ConvertHashType('key=value')
        assert result == {'key': 'value'}

    def test_convert_multiple_pairs(self):
        """Test conversion of multiple key=value pairs."""
        result = ConvertHashType('foo=bar baz=qux location=office')
        assert result == {'foo': 'bar', 'baz': 'qux', 'location': 'office'}

    def test_convert_with_whitespace(self):
        """Test conversion with extra whitespace."""
        result = ConvertHashType('key1=value1  key2=value2')
        assert result == {'key1': 'value1', 'key2': 'value2'}

    def test_convert_empty_string(self):
        """Test conversion of empty string."""
        result = ConvertHashType('')
        assert result == {}

    def test_convert_invalid_format(self):
        """Test conversion with invalid format."""
        with pytest.raises(ConversionFailure) as exc_info:
            ConvertHashType('invalidformat')
        assert 'Invalid option' in str(exc_info.value)

    def test_convert_partial_invalid(self):
        """Test conversion with one invalid pair."""
        with pytest.raises(ConversionFailure):
            ConvertHashType('valid=pair invalid')


class TestConvertValue:
    """Test generic value conversion function."""

    def test_convert_integer(self):
        """Test integer conversion."""
        assert ConvertValue('42', hint=Config.INT_TYPE) == 42
        assert ConvertValue('0', hint=Config.INT_TYPE) == 0
        assert ConvertValue('-100', hint=Config.INT_TYPE) == -100

    def test_convert_float_as_integer(self):
        """Test float string converted to integer."""
        assert ConvertValue('42.7', hint=Config.INT_TYPE) == 42
        assert ConvertValue('3.14', hint=Config.INT_TYPE) == 3

    def test_convert_float(self):
        """Test float conversion."""
        assert ConvertValue('3.14', hint=Config.FLOAT_TYPE) == 3.14
        assert ConvertValue('0.0', hint=Config.FLOAT_TYPE) == 0.0
        assert ConvertValue('-2.5', hint=Config.FLOAT_TYPE) == -2.5

    def test_convert_boolean_with_hint(self):
        """Test boolean conversion with hint."""
        assert ConvertValue('true', hint=Config.BOOL_TYPE) is True
        assert ConvertValue('false', hint=Config.BOOL_TYPE) is False
        assert ConvertValue('yes', hint=Config.BOOL_TYPE) is True
        assert ConvertValue('no', hint=Config.BOOL_TYPE) is False

    def test_convert_string(self):
        """Test string conversion."""
        assert ConvertValue('hello', hint=Config.STRING_TYPE) == 'hello'
        assert ConvertValue('  spaced  ', hint=Config.STRING_TYPE) == 'spaced'

    def test_convert_array(self):
        """Test array conversion."""
        result = ConvertValue('item1 item2 item3', hint=Config.ARRAY_TYPE)
        assert result == ['item1', 'item2', 'item3']

    def test_convert_hash(self):
        """Test hash conversion."""
        result = ConvertValue('key1=val1 key2=val2', hint=Config.HASH_TYPE)
        assert result == {'key1': 'val1', 'key2': 'val2'}

    def test_convert_without_hint_integer(self):
        """Test conversion without hint defaults to integer."""
        assert ConvertValue('42') == 42
        assert ConvertValue('0') == 0

    def test_convert_without_hint_float(self):
        """Test conversion without hint for float."""
        assert ConvertValue('3.14') == 3.14

    def test_convert_without_hint_boolean(self):
        """Test conversion without hint for boolean."""
        assert ConvertValue('true') is True
        assert ConvertValue('false') is False

    def test_convert_without_hint_string(self):
        """Test conversion without hint defaults to string."""
        assert ConvertValue('not_a_number') == 'not_a_number'

    def test_convert_invalid_type(self):
        """Test conversion with invalid type hint."""
        with pytest.raises(ConversionFailure):
            ConvertValue('value', hint='invalid_type')

    def test_convert_invalid_integer(self):
        """Test conversion of invalid integer."""
        with pytest.raises(ConversionFailure):
            ConvertValue('not_a_number', hint=Config.INT_TYPE)

    def test_convert_invalid_float(self):
        """Test conversion of invalid float."""
        with pytest.raises(ConversionFailure):
            ConvertValue('not_a_float', hint=Config.FLOAT_TYPE)


class TestDefaultValue:
    """Test default value generation function."""

    def test_default_array(self):
        """Test default array value."""
        assert DefaultValue(Config.ARRAY_TYPE) == []

    def test_default_hash(self):
        """Test default hash value."""
        assert DefaultValue(Config.HASH_TYPE) == {}

    def test_default_integer(self):
        """Test default integer value."""
        assert DefaultValue(Config.INT_TYPE) == 0

    def test_default_float(self):
        """Test default float value."""
        assert DefaultValue(Config.FLOAT_TYPE) == 0.0

    def test_default_boolean(self):
        """Test default boolean value."""
        assert DefaultValue(Config.BOOL_TYPE) is False

    def test_default_string(self):
        """Test default string value."""
        assert DefaultValue(Config.STRING_TYPE) == ''

    def test_default_invalid_type(self):
        """Test default value with invalid type."""
        with pytest.raises(ConversionFailure):
            DefaultValue('invalid_type')


class TestConfigInit:
    """Test Config class initialization."""

    def test_init_with_path(self):
        """Test Config initialization with file path."""
        config = Config('/path/to/config.conf', 'devices')
        assert config.path == '/path/to/config.conf'
        assert config.root == 'devices'
        assert config.config == {}
        assert config.database is None

    def test_init_with_none_path(self):
        """Test Config initialization with None path."""
        config = Config(None, 'devices')
        assert config.path is None
        assert config.root == 'devices'


class TestConfigIsLoaded:
    """Test Config.IsLoaded() method."""

    def test_is_loaded_empty_config(self):
        """Test IsLoaded with empty config."""
        config = Config('/path/to/config.conf', 'devices')
        assert not config.IsLoaded()

    def test_is_loaded_with_data(self):
        """Test IsLoaded with loaded config."""
        config = Config('/path/to/config.conf', 'devices')
        config.config = {'some': 'data'}
        assert config.IsLoaded()

    def test_is_loaded_none_path(self):
        """Test IsLoaded with None path."""
        config = Config(None, 'devices')
        assert config.IsLoaded()


@pytest.fixture
def temp_config_file():
    """Create a temporary config file for testing."""
    fd, path = tempfile.mkstemp(suffix='.conf', text=True)
    yield path
    os.close(fd)
    os.unlink(path)


@pytest.fixture
def valid_config_with_database(temp_config_file):
    """Create a valid config file with database configuration."""
    config_content = """[global]
database = influxdb
devices = plug1 plug2

[influxdb]
server = 127.0.0.1
port = 8086
ssl = true
verify = false
org = myorg
token = mytoken
bucket = mybucket

[plug1]
address = 192.168.1.100
measurements = emeter
tags = location=office room=main

[plug2]
address = 192.168.1.101
measurements = emeter

[emeter]
voltage = float
current = float
power = float
total = float
"""
    with open(temp_config_file, 'w') as f:
        f.write(config_content)
    return temp_config_file


@pytest.fixture
def valid_config_without_database(temp_config_file):
    """Create a valid config file without database configuration."""
    config_content = """[global]
devices = plug1

[plug1]
address = 192.168.1.100
measurements = emeter

[emeter]
voltage = float
current = float
power = float
"""
    with open(temp_config_file, 'w') as f:
        f.write(config_content)
    return temp_config_file


class TestConfigLoad:
    """Test Config.Load() method."""

    def test_load_nonexistent_file(self):
        """Test loading non-existent config file."""
        config = Config('/nonexistent/path.conf', 'devices')
        with pytest.raises(ConfigError) as exc_info:
            config.Load()
        assert 'does not exist' in str(exc_info.value)

    def test_load_valid_config_with_database(self, valid_config_with_database):
        """Test loading valid config with database."""
        config = Config(valid_config_with_database, 'devices')
        config.Load()

        # Check database configuration
        assert config.database == 'influxdb'
        assert config.config['influxdb']['server'] == '127.0.0.1'
        assert config.config['influxdb']['port'] == 8086
        assert config.config['influxdb']['ssl'] is True
        assert config.config['influxdb']['verify'] is False

        # Check device configuration
        assert 'plug1' in config.config['devices']
        assert 'plug2' in config.config['devices']
        assert config.config['devices']['plug1']['address'] == '192.168.1.100'

        # Check tags
        assert config.config['devices']['plug1']['tags'] == {
            'location': 'office',
            'room': 'main'
        }

        # Check measurements
        assert 'emeter' in config.config['measurements']
        assert config.config['measurements']['emeter']['voltage'] == 'float'

    def test_load_valid_config_without_database(self, valid_config_without_database):
        """Test loading valid config without database (echo mode)."""
        config = Config(valid_config_without_database, 'devices')
        config.Load()

        # Check no database configured
        assert config.database is None

        # Check device configuration still works
        assert 'plug1' in config.config['devices']
        assert config.config['devices']['plug1']['address'] == '192.168.1.100'

    def test_load_missing_global_section(self, temp_config_file):
        """Test loading config without global section."""
        with open(temp_config_file, 'w') as f:
            f.write('[influxdb]\nserver = localhost\n')

        config = Config(temp_config_file, 'devices')
        with pytest.raises(InvalidConfigError) as exc_info:
            config.Load()
        assert 'global section' in str(exc_info.value).lower()

    def test_load_missing_devices_in_global(self, temp_config_file):
        """Test loading config without devices field in global."""
        with open(temp_config_file, 'w') as f:
            f.write('[global]\ndatabase = influxdb\n')

        config = Config(temp_config_file, 'devices')
        with pytest.raises(InvalidConfigError) as exc_info:
            config.Load()
        assert 'missing required field' in str(exc_info.value).lower()

    def test_load_unsupported_database(self, temp_config_file):
        """Test loading config with unsupported database type."""
        with open(temp_config_file, 'w') as f:
            f.write('[global]\ndatabase = mongodb\ndevices = plug1\n')

        config = Config(temp_config_file, 'devices')
        with pytest.raises(InvalidConfigError) as exc_info:
            config.Load()
        assert 'unsupported database' in str(exc_info.value).lower()

    def test_load_missing_device_section(self, temp_config_file):
        """Test loading config with device listed but no section."""
        with open(temp_config_file, 'w') as f:
            f.write('[global]\ndevices = plug1\n')

        config = Config(temp_config_file, 'devices')
        with pytest.raises(InvalidConfigError) as exc_info:
            config.Load()
        assert 'Missing device configuration' in str(exc_info.value)

    def test_load_missing_measurements(self, temp_config_file):
        """Test loading config with device missing measurements field."""
        config_content = """[global]
devices = plug1

[plug1]
address = 192.168.1.100
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        config = Config(temp_config_file, 'devices')
        with pytest.raises(InvalidConfigError) as exc_info:
            config.Load()
        assert 'missing required field' in str(exc_info.value).lower()

    def test_load_undefined_measurement(self, temp_config_file):
        """Test loading config with undefined measurement."""
        config_content = """[global]
devices = plug1

[plug1]
address = 192.168.1.100
measurements = undefined_measurement
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        config = Config(temp_config_file, 'devices')
        with pytest.raises(InvalidConfigError) as exc_info:
            config.Load()
        assert 'Unknown measurement' in str(exc_info.value)

    def test_load_invalid_measurement_type(self, temp_config_file):
        """Test loading config with invalid measurement type."""
        config_content = """[global]
devices = plug1

[plug1]
measurements = emeter

[emeter]
voltage = invalid_type
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        config = Config(temp_config_file, 'devices')
        with pytest.raises(InvalidConfigError) as exc_info:
            config.Load()
        assert 'Invalid type' in str(exc_info.value)

    def test_load_invalid_database_field_type(self, temp_config_file):
        """Test loading config with invalid database field type."""
        config_content = """[global]
database = influxdb
devices = plug1

[influxdb]
port = not_a_number

[plug1]
measurements = emeter

[emeter]
voltage = float
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        config = Config(temp_config_file, 'devices')
        with pytest.raises(InvalidConfigError) as exc_info:
            config.Load()
        assert "Invalid field 'port'" in str(exc_info.value)

    def test_load_already_loaded(self, valid_config_with_database):
        """Test loading config that's already loaded (should be no-op)."""
        config = Config(valid_config_with_database, 'devices')
        config.Load()
        initial_config = config.config.copy()

        # Load again - should not change anything
        config.Load()
        assert config.config == initial_config


class TestConfigGetDatabase:
    """Test Config.GetDatabase() method."""

    def test_get_database_with_influxdb(self, valid_config_with_database):
        """Test getting database configuration."""
        config = Config(valid_config_with_database, 'devices')
        db_type, db_config = config.GetDatabase()

        assert db_type == 'influxdb'
        assert db_config['server'] == '127.0.0.1'
        assert db_config['port'] == 8086

    def test_get_database_without_database(self, valid_config_without_database):
        """Test getting database when none configured."""
        config = Config(valid_config_without_database, 'devices')
        db_type, db_config = config.GetDatabase()

        assert db_type is None
        assert db_config == {}

    def test_get_database_loads_config(self, valid_config_with_database):
        """Test that GetDatabase loads config if not loaded."""
        config = Config(valid_config_with_database, 'devices')
        assert not config.IsLoaded()

        db_type, db_config = config.GetDatabase()
        assert config.IsLoaded()


class TestConfigGetField:
    """Test Config.GetField() method."""

    def test_get_field_valid(self, valid_config_with_database):
        """Test getting valid field."""
        config = Config(valid_config_with_database, 'devices')
        config.Load()

        hint = config.GetField('emeter', 'voltage')
        assert hint == 'float'

    def test_get_field_none_measurement(self, valid_config_with_database):
        """Test getting field with None measurement."""
        config = Config(valid_config_with_database, 'devices')
        config.Load()

        with pytest.raises(KeyError) as exc_info:
            config.GetField(None, 'voltage')
        assert 'Unknown measurement' in str(exc_info.value)

    def test_get_field_none_field(self, valid_config_with_database):
        """Test getting field with None field name."""
        config = Config(valid_config_with_database, 'devices')
        config.Load()

        with pytest.raises(KeyError) as exc_info:
            config.GetField('emeter', None)
        assert 'Unknown measurement' in str(exc_info.value)

    def test_get_field_unknown_measurement(self, valid_config_with_database):
        """Test getting field from unknown measurement."""
        config = Config(valid_config_with_database, 'devices')
        config.Load()

        with pytest.raises(KeyError) as exc_info:
            config.GetField('unknown', 'voltage')
        assert "Unknown measurement 'unknown'" in str(exc_info.value)

    def test_get_field_unknown_field(self, valid_config_with_database):
        """Test getting unknown field from measurement."""
        config = Config(valid_config_with_database, 'devices')
        config.Load()

        with pytest.raises(KeyError) as exc_info:
            config.GetField('emeter', 'unknown')
        assert "Unknown field 'unknown'" in str(exc_info.value)


class TestConfigGetRoot:
    """Test Config.GetRoot() method."""

    def test_get_root_valid(self, valid_config_with_database):
        """Test getting root configuration."""
        config = Config(valid_config_with_database, 'devices')
        root = config.GetRoot()

        assert 'plug1' in root
        assert 'plug2' in root
        assert root['plug1']['address'] == '192.168.1.100'

    def test_get_root_none_path(self):
        """Test getting root with None path."""
        config = Config(None, 'devices')
        root = config.GetRoot()
        assert root == {}

    def test_get_root_loads_config(self, valid_config_with_database):
        """Test that GetRoot loads config if not loaded."""
        config = Config(valid_config_with_database, 'devices')
        assert not config.IsLoaded()

        root = config.GetRoot()
        assert config.IsLoaded()


class TestConfigGetTags:
    """Test Config.GetTags() method."""

    def test_get_tags_with_tags(self, valid_config_with_database):
        """Test getting tags for device with tags."""
        config = Config(valid_config_with_database, 'devices')
        tags = config.GetTags('plug1')

        assert tags == {'location': 'office', 'room': 'main'}

    def test_get_tags_without_tags(self, valid_config_with_database):
        """Test getting tags for device without tags."""
        config = Config(valid_config_with_database, 'devices')
        tags = config.GetTags('plug2')

        # Should return empty dict or list (default value)
        assert tags == [] or tags == {}

    def test_get_tags_none_entity(self, valid_config_with_database):
        """Test getting tags with None entity."""
        config = Config(valid_config_with_database, 'devices')

        with pytest.raises(KeyError) as exc_info:
            config.GetTags(None)
        assert 'Unknown entity' in str(exc_info.value)

    def test_get_tags_none_path(self):
        """Test getting tags with None path."""
        config = Config(None, 'devices')
        tags = config.GetTags('anything')
        assert tags == {}


class TestConfigReload:
    """Test Config.Reload() method."""

    def test_reload_config(self, valid_config_with_database):
        """Test reloading configuration."""
        config = Config(valid_config_with_database, 'devices')
        config.Load()

        # Modify loaded config
        config.config['test'] = 'value'
        assert 'test' in config.config

        # Reload should clear and reload
        config.Reload()
        assert 'test' not in config.config
        assert config.database == 'influxdb'

    def test_reload_clears_database(self, valid_config_with_database):
        """Test that reload clears database reference."""
        config = Config(valid_config_with_database, 'devices')
        config.Load()

        original_db = config.database
        config.Reload()

        # Database should be reloaded
        assert config.database == original_db


class TestConfigParseOption:
    """Test Config.ParseOption() static method."""

    def test_parse_option_valid_types(self, temp_config_file):
        """Test parsing options with valid types."""
        config_content = """[test]
field1 = int
field2 = float
field3 = bool
field4 = string
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        from configparser import ConfigParser
        parser = ConfigParser()
        parser.read(temp_config_file)

        assert Config.ParseOption(parser, 'test', 'field1') == 'int'
        assert Config.ParseOption(parser, 'test', 'field2') == 'float'
        assert Config.ParseOption(parser, 'test', 'field3') == 'bool'
        assert Config.ParseOption(parser, 'test', 'field4') == 'string'

    def test_parse_option_invalid_type(self, temp_config_file):
        """Test parsing option with invalid type."""
        config_content = """[test]
field = invalid_type
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        from configparser import ConfigParser
        parser = ConfigParser()
        parser.read(temp_config_file)

        with pytest.raises(InvalidConfigError) as exc_info:
            Config.ParseOption(parser, 'test', 'field')
        assert 'Invalid type' in str(exc_info.value)

    def test_parse_option_missing_option(self, temp_config_file):
        """Test parsing non-existent option."""
        config_content = """[test]
field = int
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        from configparser import ConfigParser
        parser = ConfigParser()
        parser.read(temp_config_file)

        with pytest.raises(ConfigError) as exc_info:
            Config.ParseOption(parser, 'test', 'nonexistent')
        assert 'does not exist' in str(exc_info.value)


class TestConfigRequiredFields:
    """Test Config.RequiredFields() static method."""

    def test_required_fields_all_present(self, temp_config_file):
        """Test required fields when all are present."""
        config_content = """[test]
field1 = value1
field2 = value2
field3 = value3
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        from configparser import ConfigParser
        parser = ConfigParser()
        parser.read(temp_config_file)

        # Should not raise
        Config.RequiredFields(parser, 'test', ['field1', 'field2', 'field3'])

    def test_required_fields_missing(self, temp_config_file):
        """Test required fields when some are missing."""
        config_content = """[test]
field1 = value1
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        from configparser import ConfigParser
        parser = ConfigParser()
        parser.read(temp_config_file)

        with pytest.raises(InvalidConfigError) as exc_info:
            Config.RequiredFields(parser, 'test', ['field1', 'field2'])
        assert 'missing required field' in str(exc_info.value).lower()


class TestConfigIntegration:
    """Integration tests for full config workflows."""

    def test_complete_workflow_with_database(self, valid_config_with_database):
        """Test complete workflow: load, query database, get devices."""
        config = Config(valid_config_with_database, 'devices')

        # Get database config
        db_type, db_config = config.GetDatabase()
        assert db_type == 'influxdb'

        # Get root devices
        devices = config.GetRoot()
        assert len(devices) == 2

        # Get field hints
        hint = config.GetField('emeter', 'voltage')
        assert hint == 'float'

        # Get tags
        tags = config.GetTags('plug1')
        assert 'location' in tags

    def test_complete_workflow_without_database(self, valid_config_without_database):
        """Test complete workflow without database (echo mode)."""
        config = Config(valid_config_without_database, 'devices')

        # Get database config - should be None
        db_type, db_config = config.GetDatabase()
        assert db_type is None

        # Get root devices - should still work
        devices = config.GetRoot()
        assert 'plug1' in devices

        # Get field hints - should still work
        hint = config.GetField('emeter', 'voltage')
        assert hint == 'float'

    def test_reload_after_file_change(self, temp_config_file):
        """Test reloading after file is modified."""
        # Create initial config
        config_content = """[global]
devices = plug1

[plug1]
address = 192.168.1.100
measurements = emeter

[emeter]
voltage = float
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        config = Config(temp_config_file, 'devices')
        devices = config.GetRoot()
        assert devices['plug1']['address'] == '192.168.1.100'

        # Modify file
        config_content_modified = """[global]
devices = plug1

[plug1]
address = 192.168.1.200
measurements = emeter

[emeter]
voltage = float
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content_modified)

        # Reload and verify change
        config.Reload()
        devices = config.GetRoot()
        assert devices['plug1']['address'] == '192.168.1.200'


class TestChildDevices:
    """Test child device configuration for smart power strips."""

    @pytest.fixture
    def powerstrip_config_file(self, temp_config_file):
        """Create a valid config file with child devices."""
        config_content = """[global]
database = influxdb
devices = strip1

[influxdb]
server = 127.0.0.1
port = 8086
database = smart-home

[strip1]
address = 10.0.0.100
has_children = true
poll_parent = true
measurements = power-metrics
tags = location=office type=strip

[strip1.child_0]
device = desk_lamp
measurements = power-metrics
tags = outlet=0 appliance=lamp

[strip1.child_1]
device = monitor
measurements = power-metrics
tags = outlet=1 appliance=monitor

[power-metrics]
voltage = float
current = float
power = float
total = float
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)
        return temp_config_file

    def test_child_device_parsing(self, powerstrip_config_file):
        """Test that child devices are parsed correctly."""
        config = Config(powerstrip_config_file, 'devices')
        config.Load()

        devices = config.GetRoot()
        assert 'strip1' in devices

        strip_config = devices['strip1']
        assert strip_config['has_children'] is True
        assert strip_config['poll_parent'] is True
        assert 'children' in strip_config
        assert len(strip_config['children']) == 2

    def test_child_device_indices(self, powerstrip_config_file):
        """Test that child device indices are correct."""
        config = Config(powerstrip_config_file, 'devices')
        config.Load()

        devices = config.GetRoot()
        children = devices['strip1']['children']

        assert 0 in children
        assert 1 in children
        assert children[0]['child_index'] == 0
        assert children[1]['child_index'] == 1

    def test_child_device_names(self, powerstrip_config_file):
        """Test that child device friendly names are parsed."""
        config = Config(powerstrip_config_file, 'devices')
        config.Load()

        devices = config.GetRoot()
        children = devices['strip1']['children']

        assert children[0]['device'] == 'desk_lamp'
        assert children[1]['device'] == 'monitor'

    def test_child_device_tags(self, powerstrip_config_file):
        """Test that child device tags are parsed."""
        config = Config(powerstrip_config_file, 'devices')
        config.Load()

        devices = config.GetRoot()
        children = devices['strip1']['children']

        assert children[0]['tags']['outlet'] == '0'
        assert children[0]['tags']['appliance'] == 'lamp'
        assert children[1]['tags']['outlet'] == '1'
        assert children[1]['tags']['appliance'] == 'monitor'

    def test_child_device_measurements(self, powerstrip_config_file):
        """Test that child device measurements are validated."""
        config = Config(powerstrip_config_file, 'devices')
        config.Load()

        devices = config.GetRoot()
        children = devices['strip1']['children']

        assert 'power-metrics' in children[0]['measurements']
        assert 'power-metrics' in children[1]['measurements']
        assert 'voltage' in children[0]['measurements']['power-metrics']
        assert 'current' in children[0]['measurements']['power-metrics']

    def test_child_without_has_children_flag(self, temp_config_file):
        """Test that child sections require has_children flag on parent."""
        config_content = """[global]
database = influxdb
devices = strip1

[influxdb]
server = 127.0.0.1
port = 8086
database = smart-home

[strip1]
address = 10.0.0.100
measurements = power-metrics

[strip1.child_0]
device = outlet1
measurements = power-metrics

[power-metrics]
voltage = float
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        config = Config(temp_config_file, 'devices')
        with pytest.raises(InvalidConfigError, match="must have 'has_children = true'"):
            config.Load()

    def test_child_with_unknown_parent(self, temp_config_file):
        """Test that child sections require valid parent device."""
        config_content = """[global]
database = influxdb
devices = strip1

[influxdb]
server = 127.0.0.1
port = 8086
database = smart-home

[strip1]
address = 10.0.0.100
has_children = true
measurements = power-metrics

[strip2.child_0]
device = outlet1
measurements = power-metrics

[power-metrics]
voltage = float
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        config = Config(temp_config_file, 'devices')
        with pytest.raises(InvalidConfigError, match="references unknown parent"):
            config.Load()

    def test_child_missing_required_field(self, temp_config_file):
        """Test that child sections require all required fields."""
        config_content = """[global]
database = influxdb
devices = strip1

[influxdb]
server = 127.0.0.1
port = 8086
database = smart-home

[strip1]
address = 10.0.0.100
has_children = true
measurements = power-metrics

[strip1.child_0]
measurements = power-metrics

[power-metrics]
voltage = float
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        config = Config(temp_config_file, 'devices')
        with pytest.raises(InvalidConfigError, match="missing required field"):
            config.Load()

    def test_child_with_invalid_measurement(self, temp_config_file):
        """Test that child measurements must be defined."""
        config_content = """[global]
database = influxdb
devices = strip1

[influxdb]
server = 127.0.0.1
port = 8086
database = smart-home

[strip1]
address = 10.0.0.100
has_children = true
measurements = power-metrics

[strip1.child_0]
device = outlet1
measurements = unknown-measurement

[power-metrics]
voltage = float
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        config = Config(temp_config_file, 'devices')
        with pytest.raises(InvalidConfigError, match="Unknown measurement"):
            config.Load()

    def test_multiple_children_different_indices(self, temp_config_file):
        """Test configuration with many children at different indices."""
        config_content = """[global]
database = influxdb
devices = strip1

[influxdb]
server = 127.0.0.1
port = 8086
database = smart-home

[strip1]
address = 10.0.0.100
has_children = true
poll_parent = false
measurements = power-metrics

[strip1.child_0]
device = outlet0
measurements = power-metrics

[strip1.child_2]
device = outlet2
measurements = power-metrics

[strip1.child_5]
device = outlet5
measurements = power-metrics

[power-metrics]
voltage = float
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        config = Config(temp_config_file, 'devices')
        config.Load()

        devices = config.GetRoot()
        children = devices['strip1']['children']

        assert len(children) == 3
        assert 0 in children
        assert 2 in children
        assert 5 in children
        assert 1 not in children
        assert children[0]['device'] == 'outlet0'
        assert children[2]['device'] == 'outlet2'
        assert children[5]['device'] == 'outlet5'

    def test_poll_parent_defaults_to_false(self, temp_config_file):
        """Test that poll_parent defaults to False if not specified."""
        config_content = """[global]
database = influxdb
devices = strip1

[influxdb]
server = 127.0.0.1
port = 8086
database = smart-home

[strip1]
address = 10.0.0.100
has_children = true
measurements = power-metrics

[strip1.child_0]
device = outlet1
measurements = power-metrics

[power-metrics]
voltage = float
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        config = Config(temp_config_file, 'devices')
        config.Load()

        devices = config.GetRoot()
        assert devices['strip1']['poll_parent'] is False

    def test_child_with_no_tags(self, temp_config_file):
        """Test that child devices can have no tags (optional field)."""
        config_content = """[global]
database = influxdb
devices = strip1

[influxdb]
server = 127.0.0.1
port = 8086
database = smart-home

[strip1]
address = 10.0.0.100
has_children = true
measurements = power-metrics

[strip1.child_0]
device = outlet1
measurements = power-metrics

[power-metrics]
voltage = float
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        config = Config(temp_config_file, 'devices')
        config.Load()

        devices = config.GetRoot()
        children = devices['strip1']['children']

        # Tags should default to empty dict
        assert children[0]['tags'] == {}

    def test_child_additional_fields(self, temp_config_file):
        """Test that child devices can have additional custom fields."""
        config_content = """[global]
database = influxdb
devices = strip1

[influxdb]
server = 127.0.0.1
port = 8086
database = smart-home

[strip1]
address = 10.0.0.100
has_children = true
measurements = power-metrics

[strip1.child_0]
device = outlet1
measurements = power-metrics
custom_field = some_value
priority = 5

[power-metrics]
voltage = float
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        config = Config(temp_config_file, 'devices')
        config.Load()

        devices = config.GetRoot()
        children = devices['strip1']['children']

        assert children[0]['custom_field'] == 'some_value'
        assert children[0]['priority'] == 5

    def test_no_database_with_children(self, temp_config_file):
        """Test that child devices work without database configuration (echo mode)."""
        config_content = """[global]
devices = strip1

[strip1]
address = 10.0.0.100
has_children = true
poll_parent = true
measurements = power-metrics

[strip1.child_0]
device = outlet1
measurements = power-metrics

[power-metrics]
voltage = float
"""
        with open(temp_config_file, 'w') as f:
            f.write(config_content)

        config = Config(temp_config_file, 'devices')
        config.Load()

        devices = config.GetRoot()
        children = devices['strip1']['children']

        assert len(children) == 1
        assert children[0]['device'] == 'outlet1'
        # Database should be None
        db_type, db_config = config.GetDatabase()
        assert db_type is None
