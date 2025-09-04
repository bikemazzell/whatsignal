# WhatSignal

[![Version](https://img.shields.io/badge/version-1.1.5-blue.svg)](CHANGELOG.md)
[![Go Version](https://img.shields.io/badge/go-1.22+-blue.svg)](go.mod)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

WhatSignal is a bridge service that enables one-to-one chat between WhatsApp and Signal. It forwards messages (text, images, videos, audio, reactions) between the platforms while maintaining conversation context, supporting replies, and providing intelligent auto-reply functionality.

## Features

- WhatsApp ↔ Signal one-to-one bridge with preserved context (replies, quotes, metadata)
- Smart contacts: show names (not numbers), warm cache at startup, periodic refresh, graceful fallback
- Media that just works: images, videos, documents, voice notes; config-driven types; binary sniffing; WAHA Plus/Core‑aware routing
- WAHA integration: auto version detection, typing indicators, seen status, session management and recovery
- Message extras: reactions, auto‑reply to last sender, delivery status tracking
- Security by default: encrypted storage, webhook auth/validation, configurable retention, automated cleanup
- Operational hygiene: health endpoint, structured JSON logs, graceful retries
- Developer‑friendly: Dockerized, type‑safe handling, comprehensive tests (>80% coverage)


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