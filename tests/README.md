# KASA Monitor Test Suite

Comprehensive test suite for the KASA Monitor application.

## Setup

Install test dependencies:

```bash
pip install -r requirements.txt
pip install -r requirements-test.txt
```

## Running Tests

### Run all tests

```bash
pytest
```

### Run with coverage report

```bash
pytest --cov=kasa_monitor --cov-report=html
```

Coverage report will be generated in `htmlcov/index.html`.

### Run specific test file

```bash
pytest tests/test_config.py
```

### Run specific test class

```bash
pytest tests/test_config.py::TestConvertBoolean
```

### Run specific test method

```bash
pytest tests/test_config.py::TestConvertBoolean::test_convert_true_values
```

### Run tests with markers

```bash
# Run only unit tests
pytest -m unit

# Run only integration tests
pytest -m integration

# Skip slow tests
pytest -m "not slow"
```

### Verbose output

```bash
pytest -v
```

### Show print statements

```bash
pytest -s
```

### Stop on first failure

```bash
pytest -x
```

### Run last failed tests

```bash
pytest --lf
```

## Test Structure

```
tests/
├── __init__.py           # Test package initialization
├── README.md             # This file
├── test_config.py        # Config parser tests
└── conftest.py           # Shared fixtures (if needed)
```

## Test Coverage

Current test coverage for config parser:

- **Conversion Functions**: 100%
  - `ConvertBoolean()` - boolean string conversion
  - `ConvertHashType()` - key=value pair parsing
  - `ConvertValue()` - generic type conversion
  - `DefaultValue()` - default value generation

- **Config Class**: 100%
  - Initialization
  - File loading and parsing
  - Database configuration (with and without)
  - Device configuration
  - Measurement configuration
  - Field validation
  - Tag parsing
  - Error handling

- **Error Cases**: Comprehensive
  - Missing files
  - Invalid configuration
  - Type conversion errors
  - Missing required fields
  - Invalid database types
  - Undefined measurements

## Writing New Tests

### Test File Naming

Test files should follow the pattern `test_<module>.py`.

### Test Class Naming

Test classes should follow the pattern `Test<ClassName>` or `Test<FunctionName>`.

### Test Method Naming

Test methods should follow the pattern `test_<what_is_being_tested>`.

### Using Fixtures

```python
@pytest.fixture
def my_fixture():
    # Setup
    resource = create_resource()
    yield resource
    # Teardown
    resource.cleanup()

def test_something(my_fixture):
    # Test using fixture
    assert my_fixture.do_something()
```

### Parametrized Tests

```python
@pytest.mark.parametrize("input,expected", [
    ("true", True),
    ("false", False),
    ("yes", True),
])
def test_boolean_values(input, expected):
    assert ConvertBoolean(input) == expected
```

## Best Practices

1. **One assertion per test** (when practical)
2. **Clear test names** that describe what is being tested
3. **Arrange-Act-Assert** pattern
4. **Use fixtures** for shared setup
5. **Test edge cases** and error conditions
6. **Mock external dependencies** (network, filesystem when appropriate)
7. **Keep tests independent** - no test should depend on another

## Continuous Integration

Tests can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run tests
  run: |
    pip install -r requirements.txt
    pip install -r requirements-test.txt
    pytest --cov=kasa_monitor
```

## Troubleshooting

### Import Errors

If you get import errors, ensure the package is installed or add to PYTHONPATH:

```bash
export PYTHONPATH=$PYTHONPATH:$(pwd)
pytest
```

### Temporary File Cleanup

Tests use `tempfile` for creating temporary config files. These are automatically cleaned up, but if tests fail unexpectedly, check `/tmp` for leftover files.

### Coverage Not Working

Ensure pytest-cov is installed:

```bash
pip install pytest-cov
```

## Future Test Additions

Planned test modules:

- `test_metrics.py` - Metrics pipeline tests
- `test_executor.py` - Executor framework tests
- `test_async_poll.py` - Async polling tests
- `test_devices.py` - Device wrapper tests
- `test_database.py` - Database integration tests (mocked)
