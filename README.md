# WhatSignal

[![Version](https://img.shields.io/badge/version-0.51.0-blue.svg)](CHANGELOG.md)
[![Go Version](https://img.shields.io/badge/go-1.22+-blue.svg)](go.mod)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

WhatSignal is a bridge service that enables one-to-one chat between WhatsApp and Signal. It forwards messages (text, images, videos, audio) between the platforms while maintaining conversation context and supporting replies.

## Features

- **Core Bridging**:
  - One-to-one chat bridging between WhatsApp and Signal
  - Bidirectional message forwarding with context preservation
  - Message metadata preservation

- **Smart Contact Management**:
  - **Automatic contact name display**: Messages show "John Doe: message" instead of "+1234567890: message"
  - **Startup contact sync**: All WhatsApp contacts cached on startup for instant performance (configurable)
  - **Intelligent caching**: 24-hour contact cache with configurable refresh intervals (default: 24 hours)
  - **Fallback handling**: Graceful degradation to phone numbers when contacts unavailable

- **WAHA API Integration**:
  - Full WAHA API compliance with best practices
  - Natural typing simulation
  - Message seen status
  - Proper message flow handling
  - Session management and recovery
  
- **Comprehensive Media Support**:
  - Images (JPEG, PNG) - up to 5MB
  - Videos (MP4, MOV) - up to 100MB
  - Documents (PDF, DOC, DOCX) - up to 100MB
  - Voice messages (OGG) - up to 16MB
  - GIFs - up to 25MB
  - Intelligent media caching and cleanup

- **Message Features**:
  - Text with formatting preservation
  - URL previews
  - Media attachments in replies
  - Message delivery status tracking
  
- **Security & Privacy**:
  - Database encryption at rest
  - Webhook authentication and validation
  - Path traversal protection
  - Configurable data retention
  - Automated cleanup scheduling
  - Contact information encryption
  - Deterministic encryption for message lookup optimization

- **System Features**:
  - Health monitoring endpoint
  - Structured JSON logging
  - Comprehensive test coverage (>80%)
  - Type-safe message handling
  - Graceful error handling and retries
  - Docker deployment ready

## Building

This project includes a comprehensive Makefile for building debug and release versions:

### Quick Start
```bash
# Build debug version (default)
make

# Build release version
make release

# Build both versions
make both

# Clean build artifacts
make clean

# Run tests
make test

# Show all available targets
make help
```

### Build Output

Binaries are created in:
- `build/debug/whatsignal` - Debug version
- `build/release/whatsignal` - Release version


## Prerequisite Accounts
To use WhatSignal, you must have:

1. **WhatsApp Number**: Registered and active, activated as a session on WAHA.
2. **Signal Bridge Number**: Dedicated for the bridge, used by Signal-CLI, and different from the destination number.
3. **Signal Destination Number**: The final recipient, typically on your mobile or desktop Signal app. This must not be the same as the bridge number.


## Quick Start

### 🚀 Deploy with Pre-built Image (Easiest)

**One-liner deployment (no source code needed):**
```bash
curl -fsSL https://raw.githubusercontent.com/bikemazzell/whatsignal/main/scripts/deploy.sh | bash
```

📋 **Then follow the [Quick Start Checklist](docs/quickstart.md) for complete setup steps.**

**Manual deployment:**
```bash
# Download deployment script
curl -fsSL https://raw.githubusercontent.com/bikemazzell/whatsignal/main/scripts/deploy.sh -o deploy.sh
chmod +x deploy.sh
./deploy.sh

# Configure your settings
cd whatsignal-deploy
nano .env          # API keys and secrets
nano config.json   # Signal/WhatsApp phone numbers

# Start services
docker compose up -d

# Check status
docker compose ps
curl http://localhost:8082/health
```

📖 **For detailed deployment instructions, see:** [Deployment Guide](docs/deployment.md)

### 🐳 Build from Source (Developers)

**For development or customization:**
```bash
git clone https://github.com/bikemazzell/whatsignal.git
cd whatsignal
./scripts/setup.sh  # Creates .env and config.json
make docker-up      # Build and start all services
```

### 🔧 Docker Commands

```bash
make docker-up         # Start all services
make docker-down       # Stop all services  
make docker-logs       # Follow logs
make docker-status     # Check service status
make docker-restart    # Restart services
make docker-clean      # Clean up everything
```

### 🔧 Troubleshooting

**Services won't start:**
```bash
# Check service status
make docker-status

# View logs for issues
make docker-logs

# Restart services
make docker-restart
```

**Port conflicts:**
- WhatSignal: 8082
- WAHA: 3000  
- Signal-CLI: 8080

**Clean slate restart:**
```bash
make docker-clean     # Removes all data!
./scripts/setup.sh    # Recreate config
make docker-up        # Start fresh
```

## Documentation

### Getting Started
- [Quick Start Checklist](docs/quickstart.md) - Step-by-step deployment checklist
- [Deployment Guide](docs/deployment.md) - Complete deployment instructions

### Configuration & Operations  
- [Configuration Guide](docs/configuration.md) - Configuration options and settings
- [Security Guide](docs/security.md) - Encryption and security features

### Development
- [Development Guide](docs/development.md) - Contributing and development setup
- [Technical Requirements](docs/requirements.md) - Design specifications
- [Release Process](docs/release.md) - Version management and releases

## Contributing

1. Fork the repository
2. Create your feature branch
3. Write tests for new functionality
4. Update documentation
5. Submit pull request

See [Development Guide](docs/development.md) for detailed instructions.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [Waha](https://github.com/devlikeapro/waha) for the WhatsApp HTTP API
- [signal-cli-rest-api](https://github.com/bbernhard/signal-cli-rest-api) for the Signal CLI interface 