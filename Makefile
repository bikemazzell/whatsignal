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
	@echo "  test           - Run tests"
	@echo "  test-race      - Run tests with race detection"
	@echo "  coverage       - Run tests with coverage report"
	@echo "  lint           - Run linter (requires golangci-lint)"
	@echo "  lint-fix       - Run linter with --fix (requires golangci-lint)"
	@echo "  fmt            - Format code (gofmt -s -w)"
	@echo "  format-check   - Check formatting (fails if files need formatting)"
	@echo "  vet            - Run go vet"
	@echo "  staticcheck    - Run staticcheck (requires staticcheck)"
	@echo "  security       - Run security scans (gosec, govulncheck)"
	@echo "  deps           - Download dependencies"
	@echo "  tidy           - Tidy go modules"
	@echo "  install-tools  - Install development and CI tools"
	@echo "  ci             - Run all CI checks (fmt, vet, lint, staticcheck, security, test-race, coverage)"
	@echo ""
	@echo "Run targets:"
	@echo "  run        - Run debug version"
	@echo "  run-release - Run release version"
	@echo "  run-verbose - Run debug version with verbose logging"
	@echo "  install    - Install debug version to GOPATH/bin"
	@echo "  migrate    - Run database migrations"
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
	@echo "Removing .out files recursively..."
	@find . -type f -name '*.out' -print -delete
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

# Run tests with coverage
.PHONY: coverage
coverage:
	@echo "Running tests with coverage..."
	@CGO_ENABLED=$(CGO_ENABLED) go test -coverprofile=coverage.out -covermode=atomic ./...
	@echo "Coverage report:"
	@go tool cover -func=coverage.out
	@echo "To view HTML coverage report, run: go tool cover -html=coverage.out"

# Run linter (requires golangci-lint)
.PHONY: lint
lint:
	@echo "Running linter..."
	@if [ -x "$$(go env GOPATH)/bin/golangci-lint" ]; then \
		$$(go env GOPATH)/bin/golangci-lint run --timeout=5m ./...; \
	elif command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=5m ./...; \
	else \
		echo "golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

# Auto-fix lint where supported
.PHONY: lint-fix
lint-fix:
	@echo "Running linter with --fix..."
	@if [ -x "$$(go env GOPATH)/bin/golangci-lint" ]; then \
		$$(go env GOPATH)/bin/golangci-lint run --fix --timeout=5m ./...; \
	elif command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --fix --timeout=5m ./...; \
	else \
		echo "golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code (gofmt -s -w)..."
	@gofmt -s -w .

# Check formatting only (CI-friendly)
.PHONY: format-check
format-check:
	@echo "Checking formatting..."
	@if [ -n "$$($(shell which gofmt) -l .)" ]; then \
		echo "The following files are not formatted:"; \
		$$(which gofmt) -l .; \
		exit 1; \
	fi

# Run go vet
.PHONY: vet
vet:
	@echo "Running go vet..."
	@go vet ./...

# Security scans
.PHONY: security
security:
	@echo "Running security scans..."
	@echo "Running gosec..."
	@if [ -x "$$(go env GOPATH)/bin/gosec" ]; then \
		$$(go env GOPATH)/bin/gosec -quiet ./...; \
	elif command -v gosec >/dev/null 2>&1; then \
		gosec -quiet ./...; \
	else \
		echo "gosec not installed. Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
		exit 1; \
	fi
	@echo "Running govulncheck..."
	@if [ -x "$$(go env GOPATH)/bin/govulncheck" ]; then \
		$$(go env GOPATH)/bin/govulncheck ./...; \
	elif command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "govulncheck not installed. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
		exit 1; \
	fi

# Install CI/CD tools
.PHONY: install-tools
install-tools:
	@echo "Installing development and CI tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest

# Run staticcheck
.PHONY: staticcheck
staticcheck:
	@echo "Running staticcheck..."
	@if [ -x "$$(go env GOPATH)/bin/staticcheck" ]; then \
		PATH="$$(go env GOPATH)/bin:$$PATH" staticcheck ./...; \
	elif command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "staticcheck not installed. Install with: go install honnef.co/go/tools/cmd/staticcheck@latest"; \
		exit 1; \
	fi

# Run all CI checks
.PHONY: ci
ci: fmt vet lint staticcheck security test-race coverage
	@echo "All CI checks passed!"

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

# Run database migrations
.PHONY: migrate
migrate:
	@echo "Running database migrations..."
	@go run ./cmd/migrate

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
pre-commit:
	@echo "Pre-commit: formatting, re-staging, vet, lint, test..."
	@$(MAKE) fmt
	@# Check if formatting made any changes and re-stage them
	@if ! git diff --quiet; then \
		echo "Formatting applied changes; re-staging modified files..."; \
		git add -u; \
	fi
	@$(MAKE) vet
	@$(MAKE) lint
	@$(MAKE) test
	@echo "All pre-commit checks passed."

.PHONY: hooks-install
hooks-install:
	@echo "Installing git hooks..."
	@mkdir -p .githooks
	@chmod +x .githooks/pre-commit || true
	@git config core.hooksPath .githooks
	@echo "Git hooks installed (core.hooksPath=.githooks)"

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
