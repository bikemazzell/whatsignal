# WhatSignal

WhatSignal is a bridge service that enables one-to-one chat between WhatsApp and Signal. It forwards messages (text, images, videos, audio) between the platforms while maintaining conversation context and supporting replies.

## Features

- **Core Bridging**:
  - One-to-one chat bridging between WhatsApp and Signal
  - Bidirectional message forwarding with context preservation
  - Reply correlation and threading support
  - Message metadata preservation

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
  - Reply context preservation across platforms
  - Media attachments in replies
  - Message delivery status tracking

- **Security & Privacy**:
  - Database encryption at rest (AES-256-GCM)
  - Webhook authentication and validation
  - Path traversal protection
  - Configurable data retention
  - Automated cleanup scheduling
  - Comprehensive security scanning

- **System Features**:
  - Health monitoring endpoint
  - Structured JSON logging
  - Comprehensive test coverage (>80%)
  - Type-safe message handling
  - Graceful error handling and retries
  - Docker deployment ready

## Prerequisite Accounts
To use WhatSignal, you must have:

1. **WhatsApp Number**: Registered and active, activated as a session on WAHA.
2. **Signal Bridge Number**: Dedicated for the bridge, used by Signal-CLI, and different from the destination number.
3. **Signal Destination Number**: The final recipient, typically on your mobile or desktop Signal app. This must not be the same as the bridge number.

## System Requirements

**Minimum Hardware Requirements:**
- **CPU**: 2 cores (1 GHz+)
- **RAM**: 2 GB
- **Disk Space**: 5 GB (includes Docker images, logs, and media cache)
- **Network**: Stable internet connection for WhatsApp and Signal APIs

**Recommended Hardware:**
- **CPU**: 4 cores (2 GHz+)
- **RAM**: 4 GB
- **Disk Space**: 10 GB
- **Network**: High-speed internet for media file transfers

**Software Requirements:**
- Docker 20.10+
- Docker Compose 2.0+
- Linux/macOS/Windows with Docker support

## Quick Start

**One-liner to start the service:**
```bash
docker compose up -d --build
```

1.  **Prerequisites**:
    *   Docker and Docker Compose installed
    *   Create a `.env` file in the project root:
        ```bash
        cp env.example .env
        # Edit .env with your actual values
        nano .env
        ```
    *   **Required**: Set your `WEBHOOK_SECRET` (minimum 32 characters)
    *   **Recommended**: Enable encryption by setting `WHATSIGNAL_ENABLE_ENCRYPTION=true` and `WHATSIGNAL_ENCRYPTION_SECRET`

2.  **Clone and Configure**:
    ```bash
    git clone https://github.com/yourusername/whatsignal.git
    cd whatsignal
    cp config.json.example config.json
    nano config.json # Configure your Signal and WhatsApp settings
    ```

3.  **Run with Docker Compose**:
    ```bash
    docker compose up -d --build
    ```
    This will build the `whatsignal` image and start all services (`whatsignal`, `waha`, and `signal-cli`).

4.  **Check Health**:
    ```bash
    curl http://localhost:8082/health # Check WhatSignal health
    # You can also check waha (localhost:3000) and signal-cli (localhost:8080) if they expose health/status endpoints.
    ```

5.  **View Logs**:
    ```bash
    docker compose logs -f whatsignal
    docker compose logs -f waha
    docker compose logs -f signal-cli
    ```

## Documentation

- [Installation Guide](docs/installation.md) - Detailed setup instructions
- [Configuration Guide](docs/configuration.md) - Configuration options
- [Security Guide](docs/security.md) - Encryption and security features
- [Operations Guide](docs/operations.md) - Production deployment and maintenance
- [Usage Guide](docs/usage.md) - How to use WhatSignal
- [Development Guide](docs/development.md) - Contributing and development
- [Technical Requirements](docs/requirements.md) - Design specifications

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
- [signal-cli](https://github.com/AsamK/signal-cli) for the Signal CLI interface 