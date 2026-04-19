# WhatSignal

[![Version](https://img.shields.io/badge/version-1.2.45-blue.svg)](CHANGELOG.md)
[![Go Version](https://img.shields.io/badge/go-1.26+-blue.svg)](go.mod)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

WhatSignal is a self-hosted bridge between WhatsApp and Signal. Messages go both ways — text, images, videos, voice notes, reactions — and reply threading is preserved across protocols. You read and reply from Signal; the person on WhatsApp doesn't know the difference.

## Features

- Bidirectional WhatsApp-Signal bridge (individual and group chats)
- Quoted replies route to the correct contact, even across different message ID schemes
- Contact names resolved from cache, with fallback to phone numbers
- Images, videos, documents, voice notes in both directions
- Typing indicators, read receipts, reactions, delivery tracking
- AES-256-GCM field encryption, webhook HMAC auth, configurable data retention
- Health endpoint, structured JSON logs, circuit breakers, graceful retries
- Dockerized, 80%+ test coverage


## Prerequisite Accounts
To use WhatSignal, you must have:

1. **WhatsApp Number**: Registered and active, activated as a session on WAHA.
2. **Signal Bridge Number**: Dedicated for the bridge, used by Signal-CLI, and different from the _destination_ number.
3. **Signal Destination Number**: The final recipient, typically on your mobile or desktop Signal app. This must not be the same as the _bridge_ number.

## Quick Start

### Deploy with Pre-built Image (Easiest)

**One-liner deployment (no source code needed):**
```bash
curl -fsSL https://raw.githubusercontent.com/bikemazzell/whatsignal/main/scripts/deploy.sh | bash
```

**Then follow the [Quick Start Checklist](docs/quickstart.md) for complete setup steps.**

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

**For detailed deployment instructions, see:** [Deployment Guide](docs/deployment.md)

### Build from Source (Developers)

**For development or customization:**
```bash
git clone https://github.com/bikemazzell/whatsignal.git
cd whatsignal
./scripts/setup.sh  # Creates .env and config.json
make docker-up      # Build and start all services
```

### Docker Commands

```bash
make docker-up         # Start all services
make docker-down       # Stop all services  
make docker-logs       # Follow logs
make docker-status     # Check service status
make docker-restart    # Restart services
make docker-clean      # Clean up everything
```

### Troubleshooting

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
- WhatSignal host port: 8083
- WAHA: Compose network only by default
- Signal-CLI: Compose network only by default

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
