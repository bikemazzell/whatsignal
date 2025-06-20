# Development Guide

This guide provides essential information for developers contributing to WhatSignal.

## Project Structure

```
whatsignal/
├── cmd/whatsignal/         # Main application entry point
├── internal/               # Private application code
│   ├── config/            # Configuration management
│   ├── database/          # Database operations with encryption
│   ├── models/            # Data models and type definitions
│   ├── security/          # Security utilities
│   └── service/           # Core business logic (bridge, contacts, etc.)
├── pkg/                   # Public libraries
│   ├── whatsapp/         # WhatsApp/WAHA client
│   ├── signal/           # Signal CLI client
│   └── media/            # Media handling
└── docs/                 # Documentation
```

## Development Environment Setup

Required tools:
```bash
# Go development tools
go install golang.org/x/vuln/cmd/govulncheck@latest
go install honnef.co/go/tools/cmd/staticcheck@latest

# Optional: golangci-lint for comprehensive linting
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

## Development Workflow

### Testing
```bash
# Run tests with coverage
make test

# Run tests with race detection (for concurrent code)
make test-race
```

### Code Quality
```bash
# Format code
make fmt

# Run linter
make lint

# Run static analysis
make vet

# Security scan
govulncheck ./...
```

### Building
```bash
# Debug build
make debug

# Release build  
make release

# Both versions
make both
```

See `make help` for all available targets.

## Contributing

1. Fork the repository
2. Create feature branch
3. Write tests for new functionality  
4. Ensure all tests pass (`make test`)
5. Run quality checks (`make fmt lint vet`)
6. Update documentation
7. Submit pull request

### Commit Guidelines
Use semantic commit messages:
- `feat:` New feature
- `fix:` Bug fix  
- `docs:` Documentation
- `test:` Testing
- `refactor:` Code refactoring
- `chore:` Maintenance

For detailed security practices and vulnerability scanning, see [Security Guide](security.md).
