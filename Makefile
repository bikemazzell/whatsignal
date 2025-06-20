# Makefile for whatsignal
# Supports debug and release builds

# Application name
APP_NAME := whatsignal

# Build directories
BUILD_DIR := build
DEBUG_DIR := $(BUILD_DIR)/debug
RELEASE_DIR := $(BUILD_DIR)/release

# Source files
MAIN_PACKAGE := ./cmd/whatsignal
GO_FILES := $(shell find . -name "*.go" -type f)

# Version information
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go build flags
GO_VERSION := $(shell go version | cut -d' ' -f3)
LDFLAGS_COMMON := -X main.Version=$(VERSION) \
                  -X main.BuildTime=$(BUILD_TIME) \
                  -X main.GitCommit=$(GIT_COMMIT) \
                  -X main.GoVersion=$(GO_VERSION)

# Debug build flags
DEBUG_LDFLAGS := $(LDFLAGS_COMMON)
DEBUG_GCFLAGS := -N -l

# Release build flags  
RELEASE_LDFLAGS := $(LDFLAGS_COMMON) -s -w
RELEASE_GCFLAGS := 

# CGO is required for sqlite3
CGO_ENABLED := 1

# Default target
.PHONY: all
all: debug

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  debug      - Build debug version (default)"
	@echo "  release    - Build release version"
	@echo "  both       - Build both debug and release versions"
	@echo "  clean      - Clean build artifacts"
	@echo "  test       - Run tests"
	@echo "  test-race  - Run tests with race detection"
	@echo "  lint       - Run linter (requires golangci-lint)"
	@echo "  fmt        - Format code"
	@echo "  vet        - Run go vet"
	@echo "  deps       - Download dependencies"
	@echo "  tidy       - Tidy go modules"
	@echo "  install    - Install debug version to GOPATH/bin"
	@echo "  run        - Run debug version"
	@echo "  run-release - Run release version"
	@echo "  run-verbose - Run debug version with verbose logging"
	@echo "  docker     - Build Docker image"
	@echo "  help       - Show this help"

# Create build directories
$(DEBUG_DIR):
	@mkdir -p $(DEBUG_DIR)

$(RELEASE_DIR):
	@mkdir -p $(RELEASE_DIR)

# Debug build
.PHONY: debug
debug: $(DEBUG_DIR)/$(APP_NAME)

$(DEBUG_DIR)/$(APP_NAME): $(GO_FILES) | $(DEBUG_DIR)
	@echo "Building debug version..."
	CGO_ENABLED=$(CGO_ENABLED) go build \
		-ldflags "$(DEBUG_LDFLAGS)" \
		-gcflags "$(DEBUG_GCFLAGS)" \
		-o $(DEBUG_DIR)/$(APP_NAME) \
		$(MAIN_PACKAGE)
	@echo "Debug build completed: $(DEBUG_DIR)/$(APP_NAME)"

# Release build
.PHONY: release
release: $(RELEASE_DIR)/$(APP_NAME)

$(RELEASE_DIR)/$(APP_NAME): $(GO_FILES) | $(RELEASE_DIR)
	@echo "Building release version..."
	CGO_ENABLED=$(CGO_ENABLED) go build \
		-ldflags "$(RELEASE_LDFLAGS)" \
		-gcflags "$(RELEASE_GCFLAGS)" \
		-o $(RELEASE_DIR)/$(APP_NAME) \
		$(MAIN_PACKAGE)
	@echo "Release build completed: $(RELEASE_DIR)/$(APP_NAME)"

# Build both versions
.PHONY: both
both: debug release

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@echo "Clean completed"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	CGO_ENABLED=$(CGO_ENABLED) go test -v ./...

# Run tests with race detection
.PHONY: test-race
test-race:
	@echo "Running tests with race detection..."
	CGO_ENABLED=$(CGO_ENABLED) go test -race -v ./...

# Run linter (requires golangci-lint)
.PHONY: lint
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run go vet
.PHONY: vet
vet:
	@echo "Running go vet..."
	@go vet ./...

# Download dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	@go mod download

# Tidy go modules
.PHONY: tidy
tidy:
	@echo "Tidying go modules..."
	@go mod tidy

# Install debug version
.PHONY: install
install: debug
	@echo "Installing $(APP_NAME) to GOPATH/bin..."
	@cp $(DEBUG_DIR)/$(APP_NAME) $(shell go env GOPATH)/bin/

# Run debug version
.PHONY: run
run: debug
	@echo "Running debug version..."
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs) && ./$(DEBUG_DIR)/$(APP_NAME); \
	else \
		./$(DEBUG_DIR)/$(APP_NAME); \
	fi

# Run release version
.PHONY: run-release
run-release: release
	@echo "Running release version..."
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs) && ./$(RELEASE_DIR)/$(APP_NAME); \
	else \
		./$(RELEASE_DIR)/$(APP_NAME); \
	fi

# Run debug version with verbose logging
.PHONY: run-verbose
run-verbose: debug
	@echo "Running debug version with verbose logging..."
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs) && ./$(DEBUG_DIR)/$(APP_NAME) --verbose; \
	else \
		./$(DEBUG_DIR)/$(APP_NAME) --verbose; \
	fi

# Build Docker image
.PHONY: docker
docker:
	@echo "Building Docker image..."
	@docker build -t $(APP_NAME):$(VERSION) .
	@docker tag $(APP_NAME):$(VERSION) $(APP_NAME):latest
	@echo "Docker image built: $(APP_NAME):$(VERSION)"

# Check if required tools are available
.PHONY: check-tools
check-tools:
	@echo "Checking required tools..."
	@command -v go >/dev/null 2>&1 || { echo "Go is required but not installed"; exit 1; }
	@echo "Go version: $(GO_VERSION)"
	@echo "All required tools are available"

# Development workflow targets
.PHONY: dev-setup
dev-setup: deps tidy fmt vet test

.PHONY: pre-commit
pre-commit: fmt vet lint test

# Show build information
.PHONY: info
info:
	@echo "Build Information:"
	@echo "  App Name:    $(APP_NAME)"
	@echo "  Version:     $(VERSION)"
	@echo "  Build Time:  $(BUILD_TIME)"
	@echo "  Git Commit:  $(GIT_COMMIT)"
	@echo "  Go Version:  $(GO_VERSION)"
	@echo "  CGO Enabled: $(CGO_ENABLED)"

# Force rebuild
.PHONY: rebuild
rebuild: clean all

.PHONY: rebuild-release
rebuild-release: clean release
