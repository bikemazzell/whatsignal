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

## Quick Start

1. Install prerequisites:
   - Go 1.22 or later
   - SQLite 3
   - [Waha](https://github.com/devlikeapro/waha) - WhatsApp HTTP API
   - [signal-cli](https://github.com/AsamK/signal-cli) - Signal CLI daemon

2. Install and run:
   ```bash
   # Clone and build
   git clone https://github.com/yourusername/whatsignal.git
   cd whatsignal
   go build -o whatsignal cmd/whatsignal/main.go

   # Configure
   cp config.json.example config.json
   nano config.json  # Edit configuration

   # Run
   ./whatsignal -config config.json -db whatsignal.db -cache ./cache
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