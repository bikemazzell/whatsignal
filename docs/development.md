# Development Guide

This guide provides information for developers who want to contribute to or modify WhatSignal.

## Project Structure

The project follows standard Go project layout:

```
whatsignal/
├── cmd/
│   └── whatsignal/          # Main application entry point
│       ├── main.go          # Application initialization
│       └── server.go        # HTTP server implementation
├── internal/                # Private application code
│   ├── config/             # Configuration management (96% coverage)
│   ├── database/           # Database operations (65% coverage)
│   ├── models/             # Data models and type definitions
│   └── service/            # Core business logic (74% coverage)
│       ├── bridge.go       # Message bridging between platforms
│       └── message_service.go # Message handling and storage
├── pkg/                    # Public libraries
│   ├── whatsapp/          # WhatsApp client (78% coverage)
│   ├── signal/            # Signal client (80% coverage)
│   └── media/             # Media file handling (57% coverage)
├── scripts/               # Database migrations and utilities
├── docs/                  # Documentation
└── config.json.example    # Example configuration
```

## Development Environment Setup

1. Install development tools:
   ```bash
   # Install golangci-lint
   curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

   # Install staticcheck for advanced static analysis
   go install honnef.co/go/tools/cmd/staticcheck@latest

   # Install govulncheck for security vulnerability scanning
   go install golang.org/x/vuln/cmd/govulncheck@latest

   # Install mockgen for generating mocks
   go install github.com/golang/mock/mockgen@latest
   ```

2. Set up pre-commit hooks:
   ```bash
   cp scripts/pre-commit .git/hooks/
   chmod +x .git/hooks/pre-commit
   ```

3. Configure your IDE for automatic code quality checks:
   - Enable `go vet` on save
   - Configure `gofmt` or `goimports` for automatic formatting
   - Set up `golangci-lint` integration for real-time feedback

## Code Quality

### Testing

Run tests with coverage:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

Run tests with race condition detection:
```bash
# Run all tests with race detector
go test -race ./...

# Run tests with both coverage and race detection
go test -race -coverprofile=coverage.out ./...

# Run specific package tests with race detection
go test -race ./internal/service
```

**Note:** The race detector adds significant overhead (~5-10x slower) but is essential for detecting concurrent access issues in production code.

**When to run race tests:**
- Before implementing new concurrent features
- When modifying shared data structures
- Before releases or major deployments
- When debugging intermittent issues that might be race-related
- As part of CI/CD pipeline for critical code paths

**Common race conditions to watch for:**
- Concurrent map access without proper synchronization
- Shared variables accessed by multiple goroutines
- Channel operations without proper coordination
- Resource cleanup in concurrent contexts

### Linting

Run the linter:
```bash
golangci-lint run
```

### Static Analysis Tools

For comprehensive code quality analysis, run these tools regularly:

```bash
# Run go vet for basic static analysis
go vet ./...

# Run staticcheck for advanced static analysis
staticcheck ./...

# Run golangci-lint for comprehensive linting
golangci-lint run
```

### Security Vulnerability Scanning

Scan for known security vulnerabilities in dependencies and standard library:

```bash
# Run vulnerability scan
govulncheck ./...

# Run with verbose output for detailed information
govulncheck -show verbose ./...
```

For comprehensive security practices, workflows, and detailed scanning procedures, see the [Security Guide](security.md).

**Recommended Development Workflow:**
- Run `go vet ./...` before each commit
- Run `go test -race ./...` when working on concurrent code
- Run `staticcheck ./...` daily or before pushing changes
- Run `govulncheck ./...` weekly or before releases
- Run `golangci-lint run` before creating pull requests
- Set up your IDE to run these tools automatically on save

### Code Generation

Generate mocks for interfaces:
```bash
go generate ./...
```

## Design Principles

1. **Clean Architecture**
   - Separation of concerns
   - Dependency injection
   - Interface-based design
   - Clear package boundaries

2. **Error Handling**
   - Use custom error types
   - Wrap errors with context
   - Proper error logging
   - Graceful degradation

3. **Testing**
   - Unit tests for business logic
   - Integration tests for external services
   - Mocking of external dependencies
   - Test coverage targets

4. **Configuration**
   - External configuration file
   - Environment variable overrides
   - Secure secrets handling
   - Validation of settings

## Contributing

### Pull Request Process

1. Fork the repository
2. Create your feature branch
3. Write tests for new functionality
4. Ensure all tests pass
5. Update documentation
6. Submit pull request

### Commit Guidelines

- Use semantic commit messages:
  - feat: New feature
  - fix: Bug fix
  - docs: Documentation
  - test: Testing
  - refactor: Code refactoring
  - chore: Maintenance

### Code Review Process

1. **Automated Checks**
   - Tests must pass (`go test ./...`)
   - Race condition tests must pass (`go test -race ./...`)
   - Coverage must not decrease
   - Static analysis must pass (`go vet ./...`)
   - Advanced static analysis must pass (`staticcheck ./...`)
   - Security scan must pass (`govulncheck ./...`)
   - Linter must pass (`golangci-lint run`)
   - Documentation must be updated

2. **Manual Review**
   - Code readability
   - Error handling
   - Performance considerations
   - Security implications

## Performance Considerations

1. **Message Processing**
   - Efficient message queuing
   - Proper connection pooling
   - Resource cleanup
   - Memory management
   - Race-free concurrent access

2. **Media Handling**
   - Efficient caching
   - Size limit enforcement
   - Format conversion
   - Clean up strategy

3. **Database Operations**
   - Connection pooling
   - Query optimization
   - Index usage
   - Transaction management

## Security

For comprehensive security practices, vulnerability management, and deployment security guidelines, see the [Security Guide](security.md).

Key security considerations for development:
- Run `govulncheck ./...` weekly to scan for vulnerabilities
- Keep Go and dependencies updated
- Follow secure coding practices
- Use proper authentication and encryption

## Debugging

1. **Logging**
   - Structured JSON logging
   - Log levels
   - Context preservation
   - Performance impact

2. **Monitoring**
   - Health checks
   - Metrics collection
   - Error tracking
   - Performance monitoring

3. **Troubleshooting**
   - Debug endpoints
   - Error investigation
   - Performance profiling
   - Memory analysis

## Release Process

1. **Version Management**
   - Semantic versioning
   - Changelog maintenance
   - Release notes
   - Migration guides

2. **Testing**
   - Integration testing
   - Performance testing
   - Security testing
   - Backwards compatibility

3. **Deployment**
   - Build process
   - Container images
   - Configuration management
   - Rollback procedures

### Signal Client Implementation

The Signal client (`pkg/signal/client.go`) follows these best practices:

1. **Registration Flow**
   - Proper device registration with phone number (via `signal-cli` directly, then configured in `whatsignal`)
   - Device name management (configured in `whatsignal`)
   - The `InitializeDevice(ctx context.Context)` method in the client is used to perform any necessary initial communication or checks with the `signal-cli` daemon for the configured account (e.g. verifying account or device status). Full device registration or linking is typically done via `signal-cli` commands prior to running `whatsignal`.

2. **Message Handling**
   - Support for all message types (text, media)
   - Proper metadata handling
   - Reply/quote correlation
   - Group message support (planned)
   - Session recovery

3. **Media Processing**
   - Size limit enforcement
   - Format validation
   - Efficient caching
   - Type-specific handling

4. **Error Handling**
   - Proper retry mechanisms
   - Graceful degradation
   - Detailed error reporting
   - Session recovery

5. **Concurrency and Context**
   - All client methods accept `context.Context` for managing deadlines, cancellation signals, and other request-scoped values, promoting better control over goroutines and external calls.
   - The underlying `http.Client` is configurable (e.g., for timeouts) when the `SignalClient` is instantiated, with a default timeout if no client is provided.
