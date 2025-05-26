# WhatSignal

WhatSignal is a bridge service that enables one-to-one chat between WhatsApp and Signal. It forwards messages (text, images, videos, audio) between the platforms while maintaining conversation context and supporting replies.

## Features

- One-to-one chat bridging between WhatsApp and Signal
- Full WAHA API compliance with best practices:
  - Natural typing simulation
  - Message seen status
  - Proper message flow handling
- Comprehensive media support:
  - Images (JPEG, PNG)
  - Videos (MP4)
  - Documents (PDF, DOC, etc.)
  - Voice messages (OGG)
- Message Features:
  - Text with formatting
  - URL previews
  - Reply context preservation
  - Media in replies
- Platform Integration:
  - Webhook-based message delivery
  - Session management
  - Delivery status tracking
- System Features:
  - Media file caching
  - Size limit enforcement
  - Configurable message retention
  - Health monitoring endpoint
  - Comprehensive test coverage
  - Type-safe message handling

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
        # Edit .env with your actual values, especially WEBHOOK_SECRET
        nano .env
        ```
    *   Set your `WEBHOOK_SECRET` in the `.env` file (this is the only place it needs to be configured)

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