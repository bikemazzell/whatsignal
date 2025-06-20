# Configuration Guide

This document describes all configuration options available in `config.json`.

## WhatsApp Configuration

- `whatsapp.api_base_url`: URL of your Waha instance (WhatsApp HTTP API)
  - Default: `http://localhost:3000`
  - Example: `https://waha.your-domain.com`

- `whatsapp.api_key`: API key for authenticating with WAHA
  - Required if WAHA is configured with `WHATSAPP_API_KEY`
  - Must match WAHA's `WHATSAPP_API_KEY` setting
  - Used in `X-Api-Key` header for all requests

- `whatsapp.session_name`: Name of the WhatsApp session
  - Default: `default`
  - Used in API endpoints as `/api/{sessionName}/...`
  - Must be unique if running multiple instances

- `whatsapp.webhook_secret`: Secret for authenticating incoming webhooks from WAHA.
  - **Required for security.**
  - Must be set to a secure random string.
  - Used to verify that webhook calls are coming from your Waha instance using the `X-Waha-Signature-256` header.

- `whatsapp.timeout_ms`: Timeout for API requests in milliseconds
  - Default: `10000` (10 seconds)
  - Adjust based on network conditions

- `whatsapp.retry_count`: Maximum number of retry attempts for failed requests
  - Default: `3`
  - Set to `0` to disable retries

- `whatsapp.media`: Media handling configuration (moved from nested mediaConfig)
  ```json
  {
    "maxSizeMB": {
      "image": 5,
      "video": 100,
      "document": 100,
      "voice": 16
    },
    "allowedTypes": {
      "image": ["jpg", "jpeg", "png"],
      "video": ["mp4"],
      "document": ["pdf", "doc", "docx"],
      "voice": ["ogg"]
    }
  }
  ```

## Signal Configuration

### Phone Number Configuration

Signal configuration requires **two phone numbers** for the bridge to work:

- `signal.intermediaryPhoneNumber`: The phone number that the Signal-CLI service runs on
  - This is the "intermediate" number that receives WhatsApp messages and forwards them
  - Must be registered with Signal-CLI beforehand
  - Format: International format with country code (e.g., "+1234567890")

- `signal.destinationPhoneNumber`: YOUR Signal phone number that receives the forwarded messages
  - This is your personal Signal number where you'll receive WhatsApp messages
  - Format: International format with country code (e.g., "+0987654321")

### Message Flow

```
WhatsApp User → WAHA → WhatsSignal → Signal-CLI (intermediaryPhoneNumber) → Your Signal App (destinationPhoneNumber)
Your Signal App (destinationPhoneNumber) → Signal-CLI (intermediaryPhoneNumber) → WhatsSignal → WAHA → WhatsApp User
```

### Other Signal Settings

- `signal.rpc_url`: URL of your signal-cli REST API daemon
  - Default: `http://localhost:8080`
  - Example: `http://signal-cli:8080` (if running in Docker)

- `signal.auth_token`: Authentication token for Signal API access
  - Required if your signal-cli daemon requires authentication
  - Leave empty if authentication is not enabled

- `signal.device_name`: Name for this Signal device
  - Default: "whatsignal-device"
  - Used during registration to identify this device

- `signal.webhook_secret`: Secret for authenticating incoming webhooks from Signal (if you configure Signal to send webhooks to WhatSignal).
  - **Recommended if Signal webhooks are used.**
  - Must be set to a secure random string.
  - Used to verify webhook calls using the `X-Signal-Signature-256` header.

## Retry Configuration

- `retry.initial_backoff_ms`: Initial delay before first retry
  - Default: `1000` (1 second)
  - Each subsequent retry will increase exponentially

- `retry.max_backoff_ms`: Maximum delay between retries
  - Default: `60000` (1 minute)
  - Retries will not wait longer than this value

- `retry.max_attempts`: Maximum number of retry attempts
  - Default: `5`
  - Set to `0` to disable retries

## Message Retention

- `retentionDays`: Number of days to keep message history
  - Default: `30`
  - Messages older than this will be automatically deleted
  - Set to `0` to keep messages indefinitely

## Database Configuration

- `database.path`: Path to the SQLite database file
  - Default: `./whatsignal.db`
  - Ensure the directory is writable
  - File will be created automatically if it doesn't exist

## Media Configuration

- `media.cache_dir`: Directory to store cached media files
  - Default: `./media_cache`
  - Directory will be created automatically if it doesn't exist

- `media.maxSizeMB`: Maximum file sizes in MB for different media types
  - `image`: Maximum size for images (default: 5 MB)
  - `video`: Maximum size for videos (default: 100 MB)
  - `gif`: Maximum size for GIFs (default: 25 MB)
  - `document`: Maximum size for documents (default: 100 MB)
  - `voice`: Maximum size for voice messages (default: 16 MB)

- `media.allowedTypes`: Allowed file extensions for different media types
  - `image`: Allowed image formats (default: ["jpg", "jpeg", "png"])
  - `video`: Allowed video formats (default: ["mp4", "mov"])
  - `document`: Allowed document formats (default: ["pdf", "doc", "docx"])
  - `voice`: Allowed voice formats (default: ["ogg"])

## Logging

- `log_level`: Controls the verbosity of logging
  - Valid values: `debug`, `info`, `warn`, `error`
  - Default: `info`
  - Use `debug` for development and troubleshooting
  - Use `info` for normal operation
  - Use `warn` or `error` for production with minimal logging

## Example Configuration

```json
{
  "whatsapp": {
    "api_base_url": "http://localhost:3000",
    "api_key": "your-waha-api-key",
    "session_name": "default",
    "timeout_ms": 10000,
    "retry_count": 3,
    "webhook_secret": "your-whatsapp-webhook-secret"
  },
  "signal": {
    "rpc_url": "http://localhost:8080",
    "auth_token": "your-signal-auth-token",
    "intermediaryPhoneNumber": "+1234567890",
    "destinationPhoneNumber": "+0987654321",
    "device_name": "whatsignal-device",
    "webhook_secret": "your-signal-webhook-secret"
  },
  "retry": {
    "initial_backoff_ms": 1000,
    "max_backoff_ms": 60000,
    "max_attempts": 5
  },
  "retentionDays": 30,
  "log_level": "info",
  "database": {
    "path": "./whatsignal.db"
  },
  "media": {
    "cache_dir": "./media_cache",
    "maxSizeMB": {
      "image": 5,
      "video": 100,
      "gif": 25,
      "document": 100,
      "voice": 16
    },
    "allowedTypes": {
      "image": ["jpg", "jpeg", "png"],
      "video": ["mp4", "mov"],
      "document": ["pdf", "doc", "docx"],
      "voice": ["ogg"]
    }
  }
}
```

## Setting Up

1. Copy `config.json.example` to `config.json`:
   ```bash
   cp config.json.example config.json
   ```

2. Edit `config.json` and replace the default values with your configuration:
   ```bash
   nano config.json  # or use your preferred editor
   ```

3. Ensure proper file permissions:
   ```bash
   chmod 600 config.json  # Restrict access since it contains secrets
   ``` 