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
VERSION := $(shell cat VERSION 2>/dev/null || echo "0.50.0")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG := $(shell git describe --tags --exact-match 2>/dev/null || echo "")

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
	@echo ""
	@echo "Build targets:"
	@echo "  debug      - Build debug version (default)"
	@echo "  release    - Build release version"
	@echo "  both       - Build both debug and release versions"
	@echo "  clean      - Clean build artifacts"
	@echo ""
	@echo "Development targets:"
	@echo "  test       - Run tests"
	@echo "  test-race  - Run tests with race detection"
	@echo "  lint       - Run linter (requires golangci-lint)"
	@echo "  fmt        - Format code"
	@echo "  vet        - Run go vet"
	@echo "  deps       - Download dependencies"
	@echo "  tidy       - Tidy go modules"
	@echo ""
	@echo "Run targets:"
	@echo "  run        - Run debug version"
	@echo "  run-release - Run release version"
	@echo "  run-verbose - Run debug version with verbose logging"
	@echo "  install    - Install debug version to GOPATH/bin"
	@echo ""
	@echo "Docker targets:"
	@echo "  docker-up      - Start all services with Docker Compose"
	@echo "  docker-down    - Stop all services"
	@echo "  docker-logs    - Follow logs for all services"
	@echo "  docker-status  - Show service status"
	@echo "  docker-restart - Restart all services"
	@echo "  docker-clean   - Clean up Docker resources"
	@echo "  docker-build   - Build Docker image only"
	@echo ""
	@echo "Version targets:"
	@echo "  version             - Show current version"
	@echo "  version-bump-patch  - Bump patch version (0.0.X)"
	@echo "  version-bump-minor  - Bump minor version (0.X.0)"
	@echo "  version-bump-major  - Bump major version (X.0.0)"
	@echo "  release-tag         - Create git tag for current version"
	@echo ""
	@echo "Utility targets:"
	@echo "  help       - Show this help"
	@echo "  info       - Show build information"

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

# Docker operations
.PHONY: docker
docker:
	@echo "Building Docker image..."
	@docker build -t $(APP_NAME):$(VERSION) .
	@docker tag $(APP_NAME):$(VERSION) $(APP_NAME):latest
	@echo "Docker image built: $(APP_NAME):$(VERSION)"

.PHONY: docker-build
docker-build: docker

.PHONY: docker-up
docker-up:
	@echo "Starting all services with Docker Compose..."
	@docker compose up -d --build

.PHONY: docker-down
docker-down:
	@echo "Stopping all services..."
	@docker compose down

.PHONY: docker-logs
docker-logs:
	@echo "Following logs for all services..."
	@docker compose logs -f

.PHONY: docker-logs-whatsignal
docker-logs-whatsignal:
	@echo "Following logs for WhatSignal..."
	@docker compose logs -f whatsignal

.PHONY: docker-status
docker-status:
	@echo "Service status:"
	@docker compose ps

.PHONY: docker-clean
docker-clean:
	@echo "Cleaning up Docker resources..."
	@docker compose down -v
	@docker system prune -f

.PHONY: docker-restart
docker-restart:
	@echo "Restarting all services..."
	@docker compose restart

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

# Version management
.PHONY: version
version:
	@echo "Current version: $(VERSION)"

.PHONY: version-bump-patch
version-bump-patch:
	@echo "Bumping patch version..."
	@./scripts/bump-version.sh patch

.PHONY: version-bump-minor
version-bump-minor:
	@echo "Bumping minor version..."
	@./scripts/bump-version.sh minor

.PHONY: version-bump-major
version-bump-major:
	@echo "Bumping major version..."
	@./scripts/bump-version.sh major

.PHONY: release-tag
release-tag:
	@echo "Creating release tag v$(VERSION)..."
	@git tag -a v$(VERSION) -m "Release v$(VERSION)"
	@echo "Don't forget to push tags: git push origin v$(VERSION)"
