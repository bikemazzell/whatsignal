# WhatSignal

WhatSignal is a bridge service that enables one-to-one chat between WhatsApp and Signal. It forwards messages (text, images, videos, audio) between the platforms while maintaining conversation context and supporting replies.

## Features

- One-to-one chat bridging between WhatsApp and Signal
- Support for text messages, images, videos, and audio files
- Media file caching and size limit enforcement
- Reply correlation between platforms
- Configurable message retention
- Webhook-based message delivery
- Health monitoring endpoint
- Comprehensive test coverage
- Type-safe message handling

## Prerequisites

- Go 1.22 or later
- SQLite 3
- [Waha](https://github.com/devlikeapro/waha) - WhatsApp HTTP API
- [signal-cli](https://github.com/AsamK/signal-cli) - Signal CLI daemon

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/whatsignal.git
   cd whatsignal
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Build the application:
   ```bash
   go build -o whatsignal cmd/whatsignal/main.go
   ```

## Configuration

Copy the example configuration file and modify it according to your needs:

```bash
cp config.json.example config.json
```

Configuration options:

- `whatsapp.apiBaseUrl`: URL of your Waha instance
- `whatsapp.webhookSecret`: Secret for webhook authentication
- `whatsapp.pollIntervalSec`: Polling interval if webhooks are not available
- `signal.rpcUrl`: URL of your signal-cli JSON-RPC daemon
- `signal.authToken`: Authentication token for Signal API
- `retry`: Message delivery retry configuration
- `retentionDays`: Number of days to retain message history
- `logLevel`: Logging level (debug, info, warn, error)

## Running the Service

1. Start the signal-cli daemon in JSON-RPC mode:
   ```bash
   signal-cli daemon --json-rpc
   ```

2. Start the Waha service:
   ```bash
   waha start
   ```

3. Run WhatSignal:
   ```bash
   ./whatsignal -config config.json -db whatsignal.db -cache ./cache
   ```

The service will start on port 8080 by default.

## Usage

1. Configure your Waha instance to forward messages to WhatSignal's webhook endpoint:
   ```
   http://your-server:8080/webhook/whatsapp
   ```

2. Messages from WhatsApp will be forwarded to Signal with metadata headers.

3. Reply to messages in Signal to send responses back to WhatsApp.

## Health Monitoring

Check the service health status:
```bash
curl http://localhost:8080/health
```

## Development

### Project Structure

The project follows standard Go project layout:

- `cmd/whatsignal/`: Main application entry point and server setup
- `internal/`: Private application code
  - `config/`: Configuration management (96% test coverage)
  - `database/`: Database operations and migrations (65% test coverage)
  - `models/`: Data models and type definitions
  - `service/`: Core business logic (74% test coverage)
    - `bridge.go`: Message bridging between platforms
    - `message_service.go`: Message handling and storage
- `pkg/`: Public libraries that can be used by external applications
  - `whatsapp/`: WhatsApp client (78% test coverage)
  - `signal/`: Signal client (80% test coverage)
  - `media/`: Media file handling (57% test coverage)
- `scripts/`: Database migrations and utility scripts
- `docs/`: Documentation files


## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

Please ensure:
- Tests are added for new functionality
- Existing tests pass
- Code follows project structure and conventions
- Documentation is updated if needed

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [Waha](https://github.com/devlikeapro/waha) for the WhatsApp HTTP API
- [signal-cli](https://github.com/AsamK/signal-cli) for the Signal CLI interface 