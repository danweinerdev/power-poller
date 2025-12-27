# Makefile for KASA Monitor (Go)
# Copyright 2019-2024 Daniel Weiner

.PHONY: help build test test-verbose test-coverage clean install lint docker-build docker-run docker-stop all

# Default target
.DEFAULT_GOAL := help

# Package information
PACKAGE_NAME = kasa-monitor
BINARY = kasa-monitor
MODULE = github.com/danweinerdev/go-power-poller

# Build settings
GO = go
BUILD_DIR = bin
LDFLAGS = -ldflags="-s -w"

# Colors for output
COLOR_RESET = \033[0m
COLOR_BOLD = \033[1m
COLOR_GREEN = \033[32m
COLOR_YELLOW = \033[33m
COLOR_BLUE = \033[34m

##@ Help

help: ## Display this help message
	@echo "$(COLOR_BOLD)KASA Monitor (Go) - Build System$(COLOR_RESET)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make $(COLOR_BLUE)<target>$(COLOR_RESET)\n"} \
		/^[a-zA-Z_-]+:.*?##/ { printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2 } \
		/^##@/ { printf "\n$(COLOR_BOLD)%s$(COLOR_RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Building

build: ## Build the binary
	@echo "$(COLOR_GREEN)Building $(BINARY)...$(COLOR_RESET)"
	$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/kasa-monitor
	@echo "$(COLOR_GREEN)Binary built: $(BUILD_DIR)/$(BINARY)$(COLOR_RESET)"

build-linux: ## Build for Linux (static)
	@echo "$(COLOR_GREEN)Building $(BINARY) for Linux...$(COLOR_RESET)"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/kasa-monitor

build-darwin: ## Build for macOS
	@echo "$(COLOR_GREEN)Building $(BINARY) for macOS...$(COLOR_RESET)"
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 ./cmd/kasa-monitor
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 ./cmd/kasa-monitor

build-windows: ## Build for Windows
	@echo "$(COLOR_GREEN)Building $(BINARY) for Windows...$(COLOR_RESET)"
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe ./cmd/kasa-monitor

build-all: build-linux build-darwin build-windows ## Build for all platforms
	@echo "$(COLOR_GREEN)All platforms built!$(COLOR_RESET)"

install: ## Install the binary to GOPATH/bin
	@echo "$(COLOR_GREEN)Installing $(BINARY)...$(COLOR_RESET)"
	$(GO) install $(LDFLAGS) ./cmd/kasa-monitor
	@echo "$(COLOR_GREEN)Installed!$(COLOR_RESET)"

##@ Testing

test: ## Run all tests
	@echo "$(COLOR_YELLOW)Running tests...$(COLOR_RESET)"
	$(GO) test ./...

test-verbose: ## Run tests with verbose output
	@echo "$(COLOR_YELLOW)Running tests (verbose)...$(COLOR_RESET)"
	$(GO) test -v ./...

test-coverage: ## Run tests with coverage report
	@echo "$(COLOR_YELLOW)Running tests with coverage...$(COLOR_RESET)"
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(COLOR_GREEN)Coverage report: coverage.html$(COLOR_RESET)"

test-race: ## Run tests with race detector
	@echo "$(COLOR_YELLOW)Running tests with race detector...$(COLOR_RESET)"
	$(GO) test -race ./...

bench: ## Run benchmarks
	@echo "$(COLOR_YELLOW)Running benchmarks...$(COLOR_RESET)"
	$(GO) test -bench=. -benchmem ./...

##@ Code Quality

lint: ## Run linting (requires golangci-lint)
	@echo "$(COLOR_YELLOW)Running linting checks...$(COLOR_RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "$(COLOR_YELLOW)golangci-lint not found. Running go vet instead...$(COLOR_RESET)"; \
		$(GO) vet ./...; \
	fi

fmt: ## Format code
	@echo "$(COLOR_YELLOW)Formatting code...$(COLOR_RESET)"
	$(GO) fmt ./...
	@echo "$(COLOR_GREEN)Formatting complete!$(COLOR_RESET)"

vet: ## Run go vet
	@echo "$(COLOR_YELLOW)Running go vet...$(COLOR_RESET)"
	$(GO) vet ./...

check: test lint ## Run all checks (tests + linting)

##@ Cleaning

clean: ## Clean build artifacts
	@echo "$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)"
	rm -rf $(BUILD_DIR) coverage.out coverage.html
	$(GO) clean
	@echo "$(COLOR_GREEN)Clean complete!$(COLOR_RESET)"

##@ Docker

docker-build: ## Build Docker image
	@echo "$(COLOR_GREEN)Building Docker image...$(COLOR_RESET)"
	docker build -f Containerfile -t kasa-monitor:latest .
	@echo "$(COLOR_GREEN)Docker image built!$(COLOR_RESET)"

docker-run: ## Run Docker container
	@echo "$(COLOR_GREEN)Running Docker container...$(COLOR_RESET)"
	docker run -d \
		-v $$(pwd)/config/example.toml:/etc/kasa-monitor/config.toml:ro \
		--name kasa-monitor \
		kasa-monitor:latest

docker-stop: ## Stop Docker container
	@echo "$(COLOR_YELLOW)Stopping Docker container...$(COLOR_RESET)"
	docker stop kasa-monitor
	docker rm kasa-monitor

##@ Dependencies

deps: ## Download dependencies
	@echo "$(COLOR_GREEN)Downloading dependencies...$(COLOR_RESET)"
	$(GO) mod download

deps-tidy: ## Tidy dependencies
	@echo "$(COLOR_GREEN)Tidying dependencies...$(COLOR_RESET)"
	$(GO) mod tidy

deps-upgrade: ## Upgrade all dependencies
	@echo "$(COLOR_GREEN)Upgrading dependencies...$(COLOR_RESET)"
	$(GO) get -u ./...
	$(GO) mod tidy

##@ Development

run: build ## Build and run the poller with echo mode
	@echo "$(COLOR_GREEN)Running with echo mode...$(COLOR_RESET)"
	./$(BUILD_DIR)/$(BINARY) poll --echo -c config/example.toml

status: build ## Build and run status command
	@if [ -z "$(DEVICE)" ]; then \
		echo "$(COLOR_YELLOW)Usage: make status DEVICE=192.168.1.100$(COLOR_RESET)"; \
	else \
		./$(BUILD_DIR)/$(BINARY) status -d $(DEVICE); \
	fi

version: build ## Show version
	./$(BUILD_DIR)/$(BINARY) version

##@ CI/CD

ci: deps test lint ## Run CI pipeline
	@echo "$(COLOR_GREEN)CI pipeline complete!$(COLOR_RESET)"

all: clean deps test build ## Run full pipeline (clean, deps, test, build)
	@echo "$(COLOR_GREEN)Full pipeline complete!$(COLOR_RESET)"
