# Installation Guide

This guide provides detailed instructions for installing and setting up WhatSignal.

## Prerequisites

Before installing WhatSignal, ensure you have the following prerequisites:

- Go 1.22 or later
- SQLite 3
- [Waha](https://github.com/devlikeapro/waha) - WhatsApp HTTP API
- [signal-cli](https://github.com/AsamK/signal-cli) - Signal CLI daemon

## Step-by-Step Installation

### 1. Install Prerequisites

#### Go Installation
```bash
# For Ubuntu/Debian
sudo apt-get update
sudo apt-get install golang-1.22

# For other distributions, visit: https://golang.org/doc/install
```

#### SQLite Installation
```bash
# For Ubuntu/Debian
sudo apt-get install sqlite3 libsqlite3-dev

# For other distributions
# Follow your package manager's instructions
```

#### Waha Installation
```bash
# Using Docker (recommended)
docker pull devlikeapro/whatsapp-http-api
```

#### signal-cli Installation
```bash
# For detailed instructions, visit:
# https://github.com/AsamK/signal-cli#installation
```

### 2. Install WhatSignal

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

### 3. Configure Services

1. Start the signal-cli daemon:
   ```bash
   signal-cli daemon --json-rpc
   ```

2. Start the Waha service:
   ```bash
   docker run -d -p 3000:3000 \
     -v $(pwd)/data:/app/data \
     --name waha \
     devlikeapro/whatsapp-http-api
   ```

3. Configure WhatSignal:
   ```bash
   cp config.json.example config.json
   chmod 600 config.json  # Secure the config file
   ```
   Edit `config.json` according to your needs (see [Configuration Guide](03-configuration.md))

### 4. Directory Setup

1. Create required directories:
   ```bash
   mkdir -p cache  # For media file caching
   mkdir -p data   # For persistent storage
   ```

2. Set appropriate permissions:
   ```bash
   chmod 755 cache data
   ```

## Verifying Installation

1. Start WhatSignal:
   ```bash
   ./whatsignal -config config.json -db whatsignal.db -cache ./cache
   ```

2. Check the health endpoint:
   ```bash
   curl http://localhost:8080/health
   ```
   Should return: `OK`

## Troubleshooting

### Common Issues

1. **Port Conflicts**
   - Default ports used:
     - WhatSignal: 8080
     - Waha: 3000
     - signal-cli: 8081
   - Change ports in respective configurations if needed

2. **Permission Issues**
   - Ensure proper permissions on:
     - `config.json`
     - `whatsignal.db`
     - `cache` directory
     - `data` directory

3. **Service Dependencies**
   - Verify Waha is running:
     ```bash
     docker ps | grep waha
     ```
   - Verify signal-cli daemon is running:
     ```bash
     ps aux | grep signal-cli
     ```

## Next Steps

- Continue to [Configuration Guide](03-configuration.md)
- See [Usage Guide](04-usage.md) for operational instructions 