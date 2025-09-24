# Configuration Guide

This document describes all configuration options available in `config.json`.

WhatSignal supports multiple WhatsApp-Signal channel pairs, allowing you to route messages between specific WhatsApp sessions and Signal destination numbers with complete isolation between channels.

## Required Environment Variables

Before starting whatsignal, ensure these environment variables are set:

- `WHATSAPP_API_KEY`: API key for authenticating with WAHA
  - **Required** if your WAHA instance uses API key authentication
  - Must match the `WHATSAPP_API_KEY` configured in your WAHA instance
  - Contact sync and all WhatsApp operations will fail without this

- `WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET`: Secret for webhook authentication
  - **Required for security**
  - Used to verify incoming webhooks from WAHA
  - Should be a secure random string

## WhatsApp Configuration

- `whatsapp.api_base_url`: URL of your Waha instance (WhatsApp HTTP API)
  - Default: `http://localhost:3000`
  - Example: `https://waha.your-domain.com`

- `whatsapp.api_key`: API key for authenticating with WAHA
  - Required if WAHA is configured with `WHATSAPP_API_KEY`
  - Must match WAHA's `WHATSAPP_API_KEY` setting
  - Used in `X-Api-Key` header for all requests

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

- `whatsapp.contactSyncOnStartup`: Sync all contacts on startup for better performance
  - Default: `true`
  - Recommended for better user experience
  - **Note**: Requires `WHATSAPP_API_KEY` environment variable to be set
  - If contact sync fails with 500/401 errors, check that your WAHA API key is correctly configured

- `whatsapp.contactCacheHours`: How many hours to cache contact info before refreshing
  - Default: `24` hours
  - Adjust based on how frequently contact names change

### WAHA Version Detection & Video Support

WhatSignal automatically detects your WAHA version and capabilities:

- **WAHA Plus Detection**: Automatically detects if you're running WAHA Plus with Chrome browser support
- **Intelligent Video Handling**: 
  - If WAHA Plus is detected → videos are sent as native video messages
  - If WAHA Core is detected → videos are automatically sent as document attachments for better compatibility
- **Automatic Fallback**: No configuration needed - WhatSignal adapts to your WAHA instance capabilities
- **Version Caching**: WAHA version detection is cached per session to improve performance

**Note**: Media configuration has been moved to a separate `media` section in the root of config.json for better organization. See the Media Configuration section below for details.

## Signal Configuration

### Phone Number Configuration

Signal configuration requires **two phone numbers** for the bridge to work:

- `signal.intermediaryPhoneNumber`: The phone number that the Signal-CLI service runs on
  - This is the "intermediate" number that receives WhatsApp messages and forwards them
  - Must be registered with Signal-CLI beforehand
  - Format: International format with country code (e.g., "+1234567890")

### Message Flow

```
WhatsApp User → WAHA (session) → WhatsSignal → Signal-CLI (intermediaryPhoneNumber) → Your Signal App (channel destinationPhoneNumber)
Your Signal App (channel destinationPhoneNumber) → Signal-CLI (intermediaryPhoneNumber) → WhatsSignal → WAHA (session) → WhatsApp User
```

The routing is now session-aware: each WhatsApp session is mapped to a specific Signal destination number via the `channels` configuration.

### Other Signal Settings

- `signal.rpc_url`: URL of your signal-cli REST API daemon
  - Default: `http://localhost:8080`
  - Example: `http://signal-cli:8080` (if running in Docker)

**Note**: Authentication tokens are not required for Signal CLI REST API. WhatSignal connects directly to the signal-cli daemon without additional authentication.

- `signal.device_name`: Name for this Signal device
  - Default: "whatsignal-device"
  - Used during registration to identify this device


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

## Server Configuration

- `server.webhookMaxSkewSec`: Maximum allowed timestamp skew for authenticated webhooks
  - Default: `300` seconds (5 minutes)
  - Protects against replay attacks by rejecting stale or far-future webhooks

## Channels Configuration

**Required**: WhatSignal now requires the `channels` array configuration for routing messages between WhatsApp sessions and Signal destinations.

### `channels` (array, required)
An array of channel configurations, each containing:

- **`whatsappSessionName`** (string, required): The name of the WhatsApp session in WAHA
  - Used in API endpoints as `/api/{sessionName}/...`
  - Must be unique across all channels
  - Example: "default", "business", "personal"

- **`signalDestinationPhoneNumber`** (string, required): The Signal phone number to receive messages from this WhatsApp session
  - This is YOUR personal Signal number where you'll receive WhatsApp messages from this specific session
  - Format: International format with country code (e.g., "+0987654321")
  - Must be unique across all channels

### Validation Rules

1. **Unique Session Names**: Each `whatsappSessionName` must be unique
2. **Unique Destinations**: Each `signalDestinationPhoneNumber` must be unique  
3. **Non-empty Values**: Both fields are required and cannot be empty
4. **At Least One Channel**: The `channels` array must contain at least one channel

### Message Routing

#### WhatsApp → Signal
When a message is received from WhatsApp:
1. WhatSignal extracts the session name from the WhatsApp webhook payload
2. Looks up the corresponding Signal destination using the channel configuration
3. Forwards the message to the correct Signal number
4. Stores the mapping with session context for reply routing

#### Signal → WhatsApp  
When a message is received from Signal:
1. WhatSignal identifies which Signal destination number received the message
2. Looks up the corresponding WhatsApp session using the channel configuration
3. Routes the message to the correct WhatsApp session
4. Maintains conversation context within the session

#### Message Isolation
Each channel operates independently:
- Messages from different WhatsApp sessions go to their configured Signal numbers only
- Reply messages maintain proper channel isolation
- No cross-channel message leakage

### Example Channel Configurations

#### Single Channel (minimum requirement)
```json
"channels": [
  {
    "whatsappSessionName": "default",
    "signalDestinationPhoneNumber": "+1234567890"
  }
]
```

#### Personal + Business
```json
"channels": [
  {
    "whatsappSessionName": "personal",
    "signalDestinationPhoneNumber": "+1234567890"
  },
  {
    "whatsappSessionName": "business", 
    "signalDestinationPhoneNumber": "+0987654321"
  }
]
```

#### Team Management
```json
"channels": [
  {
    "whatsappSessionName": "support",
    "signalDestinationPhoneNumber": "+1111111111"
  },
  {
    "whatsappSessionName": "sales",
    "signalDestinationPhoneNumber": "+2222222222"
  },
  {
    "whatsappSessionName": "marketing", 
    "signalDestinationPhoneNumber": "+3333333333"
  }
]
```

## Database Configuration

- `database.path`: Path to the SQLite database file
  - Default: `./whatsignal.db`
  - Ensure the directory is writable
  - File will be created automatically if it doesn't exist

## Media Configuration

### File Storage

- `media.cache_dir`: Directory to store cached media files
  - Default: `./media-cache`
  - Directory will be created automatically if it doesn't exist

### File Size Limits

- `media.maxSizeMB`: Maximum file sizes in MB for different media types
  - `image`: Maximum size for images (default: 5 MB)
  - `video`: Maximum size for videos (default: 100 MB)
  - `gif`: Maximum size for GIFs (default: 25 MB)
  - `document`: Maximum size for documents (default: 100 MB)
  - `voice`: Maximum size for voice messages (default: 16 MB)

### File Type Handling

**Important**: WhatSignal uses a config-driven approach for file type detection. You can add new file formats without rebuilding the application.

- `media.allowedTypes`: File extensions for each media type (case-insensitive, no dots required)
  - `image`: Files sent as images that display in chat (default: ["jpg", "jpeg", "png"])
  - `video`: Files sent as videos that display in chat (default: ["mp4", "mov"])
  - `voice`: Files sent as voice messages with audio player (default: ["ogg"])
  - `document`: Files explicitly configured as documents (default: ["pdf", "doc", "docx"])

#### Smart Default Behavior

**Any file type NOT listed in the above categories will automatically be sent as a document attachment.**

This means you can send files like:
- **SVG files** → sent as documents (better than images since SVG doesn't display in chat)
- **ZIP files** → sent as documents
- **Text files** → sent as documents  
- **Any other format** → sent as documents

#### Adding New File Types

To add support for new file types, simply update your `config.json`:

```json
{
  "media": {
    "allowedTypes": {
      "image": ["jpg", "jpeg", "png", "gif", "webp", "bmp"],
      "video": ["mp4", "mov", "avi", "mkv", "webm"],
      "voice": ["ogg", "aac", "m4a", "mp3", "wav"],
      "document": ["pdf", "doc", "docx", "txt", "rtf", "csv"]
    }
  }
}
```

**Application restart required** - configuration is loaded at startup, so restart WhatSignal after making changes.

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
    "timeout_ms": 10000000000,
    "retry_count": 3,
    "webhook_secret": "your-whatsapp-webhook-secret",
    "contactSyncOnStartup": true,
    "contactCacheHours": 24
  },
  "signal": {
    "rpc_url": "http://localhost:8080",
    "intermediaryPhoneNumber": "+1234567890",
    "device_name": "whatsignal-device",
  },
  "channels": [
    {
      "whatsappSessionName": "default",
      "signalDestinationPhoneNumber": "+0987654321"
    },
    {
      "whatsappSessionName": "business",
      "signalDestinationPhoneNumber": "+1122334455"
    }
  ],
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
    "cache_dir": "./media-cache",
    "maxSizeMB": {
      "image": 5,
      "video": 100,
      "gif": 25,
      "document": 100,
      "voice": 16
    },
    "allowedTypes": {
      "image": ["jpg", "jpeg", "png", "gif", "webp"],
      "video": ["mp4", "mov", "avi"],
      "document": ["pdf", "doc", "docx", "txt", "rtf"],
      "voice": ["ogg", "aac", "m4a", "mp3"]
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

## Database Encryption

WhatSignal supports encryption at rest for sensitive data in the database. This feature is controlled by environment variables:

### Environment Variables

- **`WHATSIGNAL_ENABLE_ENCRYPTION`**: Set to `"true"` to enable database encryption
  - Default: `"false"` (disabled)
  - When enabled, encrypts phone numbers, messages, and other sensitive data

- **`WHATSIGNAL_ENCRYPTION_SECRET`**: The encryption key used for database encryption
  - **Required** when encryption is enabled
  - Must be at least 32 characters long
  - Should be a strong, random string
  - **CRITICAL**: Never change this after initial setup - doing so will make existing encrypted data unreadable

### Important Notes on Encryption

1. **Encryption salts are hardcoded**: The application uses internal salts for key derivation and deterministic encryption. These are intentionally not configurable to prevent accidental data loss.

2. **Cannot change encryption secret**: Once you've encrypted data with a specific `WHATSIGNAL_ENCRYPTION_SECRET`, you cannot change it without losing access to all existing encrypted data.

3. **Backup considerations**: Always backup your encryption secret securely. Without it, encrypted data cannot be recovered.

4. **Performance impact**: Encryption adds minimal overhead but enables searching on encrypted fields through deterministic encryption for lookups.

### Example Setup

```bash
# Enable encryption before first run
export WHATSIGNAL_ENABLE_ENCRYPTION=true
export WHATSIGNAL_ENCRYPTION_SECRET="your-very-long-random-secret-at-least-32-chars"

# Start the application
./whatsignal
```

**Warning**: These environment variables must be set consistently every time the application runs. Consider using a `.env` file or systemd environment configuration to ensure they're always available.

## Troubleshooting

### Contact Sync Failures

If you see errors like:
```
Failed to sync contacts on startup: failed to fetch contacts batch (offset 0): request failed with status 500
```

**Common causes:**

1. **Missing API Key**: Ensure `WHATSAPP_API_KEY` environment variable is set
   ```bash
   export WHATSAPP_API_KEY="your-waha-api-key"
   ```

2. **WAHA Service Issues**: Check that your WAHA instance is running and accessible
   ```bash
   curl http://192.168.1.23:3000/api/sessions
   ```

3. **Session Not Ready**: The WhatsApp session may not be fully initialized
   - Check WAHA logs for session status
   - Ensure QR code has been scanned and session is "WORKING"

4. **Authentication Issues**: Verify API key matches WAHA configuration
   - Check WAHA's `WHATSAPP_API_KEY` environment variable
   - Ensure both services use the same key

**Workaround**: You can disable contact sync on startup by setting:
```json
{
  "whatsapp": {
    "contactSyncOnStartup": false
  }
}
```

### Docker Network Issues

If you see connection errors like:
```
Get "http://waha:3000/api/sessions": dial tcp 172.18.0.2:3000: connect: connection refused
```

This indicates the application is trying to use Docker internal service names but network restrictions are blocking access.

**Solutions:**

1. **Environment Variable Override** (Recommended):
   ```bash
   # In your .env file or Docker Compose environment
   WHATSAPP_API_URL=http://YOUR_SERVER_IP:3000
   SIGNAL_RPC_URL=http://YOUR_SERVER_IP:8081
   ```

2. **Use Host Networking** (Less secure):
   ```yaml
   services:
     whatsignal:
       network_mode: host
   ```

3. **Docker Host Gateway**:
   ```yaml
   services:
     whatsignal:
       extra_hosts:
         - "host.docker.internal:host-gateway"
       environment:
         - WHATSAPP_API_URL=http://host.docker.internal:3000
         - SIGNAL_RPC_URL=http://host.docker.internal:8081
   ```

The application automatically detects and rewrites Docker internal hostnames (single-word hostnames without dots) to use the configured external WAHA host, but environment variable overrides provide the most reliable solution.