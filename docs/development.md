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

   # Install mockgen for generating mocks
   go install github.com/golang/mock/mockgen@latest
   ```

2. Set up pre-commit hooks:
   ```bash
   cp scripts/pre-commit .git/hooks/
   chmod +x .git/hooks/pre-commit
   ```

## Code Quality

### Testing

Run tests with coverage:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

Current test coverage:
- Overall: 68.3%
- Key packages:
  - config: 96.3%
  - migrations: 100%
  - signal: 80%
  - whatsapp: 78%
  - service: 74.4%

### Linting

Run the linter:
```bash
golangci-lint run
```

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
   - Tests must pass
   - Coverage must not decrease
   - Linter must pass
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

## Security Best Practices

1. **Authentication**
   - Secure token handling
   - Webhook verification (e.g., HMAC-SHA256 signature checking of `X-Waha-Signature-256` and `X-Signal-Signature-256` headers)
   - Rate limiting
   - Access control

2. **Data Protection**
   - Message encryption
   - Secure storage
   - Data retention
   - Privacy compliance

3. **Dependency Management**
   - Regular updates
   - Security scanning
   - Version pinning
   - License compliance

4. **Concurrency and Context**
   - All client methods accept `context.Context` for managing deadlines, cancellation signals, and other request-scoped values, promoting better control over goroutines and external calls.
   - The underlying `http.Client` is configurable (e.g., for timeouts) when the `SignalClient` is instantiated, with a default timeout if no client is provided.

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
