# Configuration Guide

This document describes all configuration options available in `config.json`.

## WhatsApp Configuration

- `whatsapp.apiBaseUrl`: URL of your Waha instance (WhatsApp HTTP API)
  - Default: `http://localhost:3000`
  - Example: `https://waha.your-domain.com`

- `whatsapp.webhookSecret`: Secret for authenticating incoming webhooks
  - Must be set to a secure random string
  - Used to verify that webhook calls are coming from your Waha instance

- `whatsapp.pollIntervalSec`: How often to poll for messages if webhooks are not available
  - Default: `30` seconds
  - Set to `0` to disable polling

## Signal Configuration

- `signal.rpcUrl`: URL of your signal-cli JSON-RPC daemon
  - Default: `http://localhost:8080`
  - Example: `http://signal-cli:8080` (if running in Docker)

- `signal.authToken`: Authentication token for Signal API access
  - Required if your signal-cli daemon requires authentication
  - Leave empty if authentication is not enabled

## Retry Configuration

- `retry.initialBackoffMs`: Initial delay before first retry
  - Default: `1000` (1 second)
  - Each subsequent retry will increase exponentially

- `retry.maxBackoffMs`: Maximum delay between retries
  - Default: `60000` (1 minute)
  - Retries will not wait longer than this value

- `retry.maxAttempts`: Maximum number of retry attempts
  - Default: `5`
  - Set to `0` to disable retries

## Message Retention

- `retentionDays`: Number of days to keep message history
  - Default: `30`
  - Messages older than this will be automatically deleted
  - Set to `0` to keep messages indefinitely

## Logging

- `logLevel`: Controls the verbosity of logging
  - Valid values: `debug`, `info`, `warn`, `error`
  - Default: `info`
  - Use `debug` for development and troubleshooting
  - Use `info` for normal operation
  - Use `warn` or `error` for production with minimal logging

## Example Configuration

```json
{
  "whatsapp": {
    "apiBaseUrl": "http://localhost:3000",
    "webhookSecret": "your-webhook-secret",
    "pollIntervalSec": 30
  },
  "signal": {
    "rpcUrl": "http://localhost:8080",
    "authToken": "your-signal-auth-token"
  },
  "retry": {
    "initialBackoffMs": 1000,
    "maxBackoffMs": 60000,
    "maxAttempts": 5
  },
  "retentionDays": 30,
  "logLevel": "info"
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