# Makefile for KASA Monitor
# Copyright 2019-2024 Daniel Weiner

.PHONY: help venv install install-dev test test-verbose test-coverage test-fast clean build dist upload check format lint all

# Default target
.DEFAULT_GOAL := help

# Package information
PACKAGE_NAME = kasa-monitor

# Detect virtual environment
ifeq ($(OS),Windows_NT)
    VENV_BIN = .venv/Scripts
    PYTHON = $(VENV_BIN)/python.exe
else
    VENV_BIN = .venv/bin
    PYTHON = $(VENV_BIN)/python
endif

# Check if venv exists, otherwise use system python
ifeq ($(wildcard $(PYTHON)),)
    PYTHON = python
endif

PIP = $(PYTHON) -m pip
PYTEST = $(PYTHON) -m pytest
BUILD = $(PYTHON) -m build

# Directories
SRC_DIR = kasa_monitor
TEST_DIR = tests
DIST_DIR = dist
BUILD_DIR = build
EGG_DIR = *.egg-info
COVERAGE_DIR = htmlcov
CACHE_DIRS = .pytest_cache __pycache__

# Colors for output
COLOR_RESET = \033[0m
COLOR_BOLD = \033[1m
COLOR_GREEN = \033[32m
COLOR_YELLOW = \033[33m
COLOR_BLUE = \033[34m

##@ Help

help: ## Display this help message
	@echo "$(COLOR_BOLD)KASA Monitor - Build System$(COLOR_RESET)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make $(COLOR_BLUE)<target>$(COLOR_RESET)\n"} \
		/^[a-zA-Z_-]+:.*?##/ { printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2 } \
		/^##@/ { printf "\n$(COLOR_BOLD)%s$(COLOR_RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Installation

venv: ## Create virtual environment
	@if [ ! -d ".venv" ]; then \
		echo "$(COLOR_GREEN)Creating virtual environment...$(COLOR_RESET)"; \
		python -m venv .venv; \
		echo "$(COLOR_GREEN)Virtual environment created in .venv/$(COLOR_RESET)"; \
		echo ""; \
		echo "$(COLOR_BOLD)Activate with:$(COLOR_RESET)"; \
		if [ "$(OS)" = "Windows_NT" ]; then \
			echo "  .venv\\Scripts\\activate"; \
		else \
			echo "  source .venv/bin/activate"; \
		fi; \
	else \
		echo "$(COLOR_YELLOW)Virtual environment already exists$(COLOR_RESET)"; \
	fi

install: venv ## Install package and runtime dependencies
	@echo "$(COLOR_GREEN)Installing runtime dependencies...$(COLOR_RESET)"
	$(PIP) install -r requirements.txt
	@echo "$(COLOR_GREEN)Installing package...$(COLOR_RESET)"
	$(PIP) install -e .
	@echo "$(COLOR_GREEN)Installation complete!$(COLOR_RESET)"

install-dev: venv ## Install package with development dependencies
	@echo "$(COLOR_GREEN)Installing runtime dependencies...$(COLOR_RESET)"
	$(PIP) install -r requirements.txt
	@echo "$(COLOR_GREEN)Installing test dependencies...$(COLOR_RESET)"
	$(PIP) install -r requirements-test.txt
	@echo "$(COLOR_GREEN)Installing package in editable mode...$(COLOR_RESET)"
	$(PIP) install -e ".[dev]"
	@echo "$(COLOR_GREEN)Development environment ready!$(COLOR_RESET)"

##@ Testing

test: ## Run all tests
	@echo "$(COLOR_YELLOW)Running tests...$(COLOR_RESET)"
	@$(PYTHON) -c "import pytest" 2>/dev/null || (echo "$(COLOR_YELLOW)pytest not installed. Run 'make install-dev' first.$(COLOR_RESET)" && exit 1)
	$(PYTEST)

test-verbose: ## Run tests with verbose output
	@echo "$(COLOR_YELLOW)Running tests (verbose)...$(COLOR_RESET)"
	$(PYTEST) -v -s

test-coverage: ## Run tests with coverage report
	@echo "$(COLOR_YELLOW)Running tests with coverage...$(COLOR_RESET)"
	$(PYTEST) --cov=$(SRC_DIR) --cov-report=term-missing --cov-report=html
	@echo "$(COLOR_GREEN)Coverage report generated in $(COVERAGE_DIR)/index.html$(COLOR_RESET)"

test-fast: ## Run tests without coverage (faster)
	@echo "$(COLOR_YELLOW)Running tests (fast mode)...$(COLOR_RESET)"
	$(PYTEST) --tb=short -q

test-unit: ## Run only unit tests
	@echo "$(COLOR_YELLOW)Running unit tests...$(COLOR_RESET)"
	$(PYTEST) -m unit

test-integration: ## Run only integration tests
	@echo "$(COLOR_YELLOW)Running integration tests...$(COLOR_RESET)"
	$(PYTEST) -m integration

test-watch: ## Run tests in watch mode (requires pytest-watch)
	@echo "$(COLOR_YELLOW)Running tests in watch mode...$(COLOR_RESET)"
	$(PYTHON) -m pytest_watch

##@ Code Quality

check: test lint ## Run all checks (tests + linting)

lint: ## Run linting checks (requires ruff or flake8)
	@echo "$(COLOR_YELLOW)Running linting checks...$(COLOR_RESET)"
	@if command -v ruff >/dev/null 2>&1; then \
		echo "Running ruff..."; \
		ruff check $(SRC_DIR); \
	elif command -v flake8 >/dev/null 2>&1; then \
		echo "Running flake8..."; \
		flake8 $(SRC_DIR); \
	else \
		echo "$(COLOR_YELLOW)No linter found. Install ruff or flake8.$(COLOR_RESET)"; \
	fi

format: ## Format code (requires black or ruff)
	@echo "$(COLOR_YELLOW)Formatting code...$(COLOR_RESET)"
	@if command -v ruff >/dev/null 2>&1; then \
		echo "Running ruff format..."; \
		ruff format $(SRC_DIR) $(TEST_DIR); \
	elif command -v black >/dev/null 2>&1; then \
		echo "Running black..."; \
		black $(SRC_DIR) $(TEST_DIR); \
	else \
		echo "$(COLOR_YELLOW)No formatter found. Install ruff or black.$(COLOR_RESET)"; \
	fi

type-check: ## Run type checking (requires mypy)
	@echo "$(COLOR_YELLOW)Running type checks...$(COLOR_RESET)"
	@if command -v mypy >/dev/null 2>&1; then \
		mypy $(SRC_DIR); \
	else \
		echo "$(COLOR_YELLOW)mypy not found. Install with: pip install mypy$(COLOR_RESET)"; \
	fi

##@ Building

build: clean ## Build distribution packages (wheel and sdist)
	@echo "$(COLOR_GREEN)Building distribution packages...$(COLOR_RESET)"
	$(BUILD)
	@echo "$(COLOR_GREEN)Build complete! Packages in $(DIST_DIR)/$(COLOR_RESET)"
	@ls -lh $(DIST_DIR)/

dist: build ## Alias for build

wheel: clean ## Build wheel package only
	@echo "$(COLOR_GREEN)Building wheel package...$(COLOR_RESET)"
	$(BUILD) --wheel
	@echo "$(COLOR_GREEN)Wheel built in $(DIST_DIR)/$(COLOR_RESET)"

sdist: clean ## Build source distribution only
	@echo "$(COLOR_GREEN)Building source distribution...$(COLOR_RESET)"
	$(BUILD) --sdist
	@echo "$(COLOR_GREEN)Source distribution built in $(DIST_DIR)/$(COLOR_RESET)"

##@ Publishing

upload: build ## Upload package to PyPI (requires twine)
	@echo "$(COLOR_YELLOW)Uploading to PyPI...$(COLOR_RESET)"
	@if command -v twine >/dev/null 2>&1; then \
		twine upload $(DIST_DIR)/*; \
		echo "$(COLOR_GREEN)Upload complete!$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)twine not found. Install with: pip install twine$(COLOR_RESET)"; \
		exit 1; \
	fi

upload-test: build ## Upload package to TestPyPI
	@echo "$(COLOR_YELLOW)Uploading to TestPyPI...$(COLOR_RESET)"
	@if command -v twine >/dev/null 2>&1; then \
		twine upload --repository testpypi $(DIST_DIR)/*; \
		echo "$(COLOR_GREEN)Upload to TestPyPI complete!$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)twine not found. Install with: pip install twine$(COLOR_RESET)"; \
		exit 1; \
	fi

##@ Cleaning

clean: ## Clean build artifacts and caches
	@echo "$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)"
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(EGG_DIR)
	find . -type d -name "$(CACHE_DIRS)" -exec rm -rf {} + 2>/dev/null || true
	find . -type f -name "*.pyc" -delete
	find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	rm -rf $(COVERAGE_DIR) .coverage
	@echo "$(COLOR_GREEN)Clean complete!$(COLOR_RESET)"

clean-all: clean ## Clean everything including test cache and virtual environments
	@echo "$(COLOR_YELLOW)Deep cleaning...$(COLOR_RESET)"
	rm -rf .tox .mypy_cache .ruff_cache .venv
	find . -type d -name "*.egg-info" -exec rm -rf {} + 2>/dev/null || true
	@echo "$(COLOR_GREEN)Deep clean complete!$(COLOR_RESET)"

##@ Docker

docker-build: ## Build Docker image
	@echo "$(COLOR_GREEN)Building Docker image...$(COLOR_RESET)"
	docker build -f Containerfile -t kasa-monitor:latest .
	@echo "$(COLOR_GREEN)Docker image built!$(COLOR_RESET)"

docker-run: ## Run Docker container
	@echo "$(COLOR_GREEN)Running Docker container...$(COLOR_RESET)"
	docker run -d \
		-v $$(pwd)/config/monitor.conf:/etc/monitor.conf:ro \
		--name kasa-monitor \
		kasa-monitor:latest

docker-stop: ## Stop Docker container
	@echo "$(COLOR_YELLOW)Stopping Docker container...$(COLOR_RESET)"
	docker stop kasa-monitor
	docker rm kasa-monitor

##@ Development

dev-setup: clean install-dev ## Complete development setup
	@echo "$(COLOR_GREEN)Development environment is ready!$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Next steps:$(COLOR_RESET)"
	@echo "  1. Run tests: make test"
	@echo "  2. Check coverage: make test-coverage"
	@echo "  3. Build package: make build"

run-echo: ## Run with echo-metrics for testing (requires config)
	@if [ -f config/monitor.conf ]; then \
		$(PYTHON) -m kasa_monitor --echo-metrics --run-once -o config/monitor.conf; \
	else \
		echo "$(COLOR_YELLOW)config/monitor.conf not found$(COLOR_RESET)"; \
		exit 1; \
	fi

version: ## Show package version
	@echo "$(COLOR_BOLD)KASA Monitor$(COLOR_RESET)"
	@$(PYTHON) -c "import tomllib; print(tomllib.load(open('pyproject.toml', 'rb'))['project']['version'])" 2>/dev/null || grep 'version = ' pyproject.toml | sed 's/.*"\(.*\)".*/\1/'

info: ## Show project information
	@echo "$(COLOR_BOLD)KASA Monitor - Project Information$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Package:$(COLOR_RESET)     $(PACKAGE_NAME)"
	@echo "$(COLOR_BOLD)Version:$(COLOR_RESET)     2.0.0"
	@echo "$(COLOR_BOLD)Python:$(COLOR_RESET)      $(PYTHON)"
	@echo "$(COLOR_BOLD)Source:$(COLOR_RESET)      $(SRC_DIR)/"
	@echo "$(COLOR_BOLD)Tests:$(COLOR_RESET)       $(TEST_DIR)/"
	@echo ""
	@echo "$(COLOR_BOLD)Main dependencies:$(COLOR_RESET)"
	@echo "  - python-kasa>=0.10.2"
	@echo "  - influxdb-client>=1.36.0"
	@echo "  - requests>=2.31.0"

##@ CI/CD

ci: clean install-dev test-coverage lint ## Run CI pipeline (install, test, lint)
	@echo "$(COLOR_GREEN)CI pipeline complete!$(COLOR_RESET)"

all: clean install-dev test-coverage build ## Run full pipeline (clean, install, test, build)
	@echo "$(COLOR_GREEN)Full pipeline complete!$(COLOR_RESET)"
