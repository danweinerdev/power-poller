# Makefile for KASA Monitor (Go)
# Copyright 2019-2024 Daniel Weiner

.PHONY: help build test test-verbose test-coverage clean install lint docker-build docker-run docker-stop

# Package information
PACKAGE_NAME = kasa-monitor
BINARY = kasa-monitor
MODULE = github.com/danweinerdev/power-poller

# Build settings
GO = go
BUILD_DIR = bin
LDFLAGS = -ldflags="-s -w"

# Container runtime detection (podman > docker > container)
CONTAINER_RUNTIME := $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null || command -v container 2>/dev/null)

# Colors for output
COLOR_RESET = \033[0m
COLOR_BOLD = \033[1m
COLOR_GREEN = \033[32m
COLOR_YELLOW = \033[33m
COLOR_BLUE = \033[34m

all: clean test test-all build-all
	@printf "$(COLOR_GREEN)Full pipeline complete!$(COLOR_RESET)\n"

##@ Help

help: ## Display this help message
	@printf "$(COLOR_BOLD)KASA Monitor (Go) - Build System$(COLOR_RESET)\n"
	@printf "\n"
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make $(COLOR_BLUE)<target>$(COLOR_RESET)\n"} \
		/^[a-zA-Z_-]+:.*?##/ { printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2 } \
		/^##@/ { printf "\n$(COLOR_BOLD)%s$(COLOR_RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Building

build: ## Build the binary
	@printf "$(COLOR_GREEN)Building $(BINARY)...$(COLOR_RESET)\n"
	$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/kasa-monitor
	@printf "$(COLOR_GREEN)Binary built: $(BUILD_DIR)/$(BINARY)$(COLOR_RESET)\n"

build-linux: ## Build for Linux (static)
	@printf "$(COLOR_GREEN)Building $(BINARY) for Linux...$(COLOR_RESET)\n"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/kasa-monitor

build-darwin: ## Build for macOS
	@printf "$(COLOR_GREEN)Building $(BINARY) for macOS...$(COLOR_RESET)\n"
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 ./cmd/kasa-monitor
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 ./cmd/kasa-monitor

build-windows: ## Build for Windows
	@printf "$(COLOR_GREEN)Building $(BINARY) for Windows...$(COLOR_RESET)\n"
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe ./cmd/kasa-monitor

build-all: build-linux build-darwin build-windows ## Build for all platforms
	@printf "$(COLOR_GREEN)All platforms built!$(COLOR_RESET)\n"

##@ Testing

test: ## Run all tests
	@printf "$(COLOR_YELLOW)Running tests...$(COLOR_RESET)\n"
	$(GO) test ./...

test-verbose: ## Run tests with verbose output
	@printf "$(COLOR_YELLOW)Running tests (verbose)...$(COLOR_RESET)\n"
	$(GO) test -v ./...

test-coverage: ## Run tests with coverage report
	@printf "$(COLOR_YELLOW)Running tests with coverage...$(COLOR_RESET)\n"
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@printf "$(COLOR_GREEN)Coverage report: coverage.html$(COLOR_RESET)\n"

test-race: ## Run tests with race detector
	@printf "$(COLOR_YELLOW)Running tests with race detector...$(COLOR_RESET)\n"
	$(GO) test -race ./...

test-all: test test-coverage test-race

##@ Code Quality

lint: ## Run linting (requires golangci-lint)
	@printf "$(COLOR_YELLOW)Running linting checks...$(COLOR_RESET)\n"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		printf "$(COLOR_YELLOW)golangci-lint not found. Running go vet instead...$(COLOR_RESET)\n"; \
		$(GO) vet ./...; \
	fi

fmt: ## Format code
	@printf "$(COLOR_YELLOW)Formatting code...$(COLOR_RESET)\n"
	$(GO) fmt ./...
	@printf "$(COLOR_GREEN)Formatting complete!$(COLOR_RESET)\n"

vet: ## Run go vet
	@printf "$(COLOR_YELLOW)Running go vet...$(COLOR_RESET)\n"
	$(GO) vet ./...

check: test lint ## Run all checks (tests + linting)

##@ Cleaning

clean: ## Clean build artifacts
	@printf "$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)\n"
	rm -rf $(BUILD_DIR) coverage.out coverage.html
	$(GO) clean
	@printf "$(COLOR_GREEN)Clean complete!$(COLOR_RESET)\n"

##@ Docker

container: ## Build container image
ifndef CONTAINER_RUNTIME
	$(error No container runtime found. Install podman, docker, or container.)
endif
	@printf "$(COLOR_GREEN)Building container image with $(CONTAINER_RUNTIME)...$(COLOR_RESET)\n"
	$(CONTAINER_RUNTIME) build -f Containerfile -t kasa-monitor:latest .
	@printf "$(COLOR_GREEN)Container image built!$(COLOR_RESET)\n"

##@ CI/CD

ci: deps test lint ## Run CI pipeline
	@printf "$(COLOR_GREEN)CI pipeline complete!$(COLOR_RESET)\n"
