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
  - Default: `30000` (30 seconds)
  - Adjust based on network conditions and WAHA response times

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

### Session Health Monitoring

WhatSignal includes automatic session health monitoring to detect and recover from WhatsApp session issues.

- `whatsapp.sessionAutoRestart`: Enable automatic session restart when unhealthy
  - Default: `false`
  - Recommended: `true` for production deployments
  - When enabled, monitors session health and automatically restarts sessions in unhealthy states

- `whatsapp.sessionHealthCheckSec`: How often to check session health (in seconds)
  - Default: `30` seconds
  - Recommended: `30-60` seconds for most deployments
  - Lower values = more responsive but higher API load

- `whatsapp.sessionStartupTimeoutSec`: Maximum time a session can remain in STARTING status (in seconds)
  - Default: `30` seconds
  - **Purpose**: Prevents sessions from getting stuck during initialization
  - **Environment variable override**: `WHATSAPP_SESSION_STARTUP_TIMEOUT_SEC`
  - **How it works**:
    - Monitors sessions in 'STARTING' status
    - If a session remains in 'STARTING' for longer than this timeout, automatically triggers a restart
    - Helps recover from WAHA initialization issues where sessions never transition to 'WORKING'
  - **Recommended values**:
    - Fast networks: `30` seconds
    - Slow networks or high-latency connections: `60` seconds
    - If you see frequent "session stuck in STARTING" warnings, increase this value

**Example Configuration**:
```json
"whatsapp": {
  "api_base_url": "http://192.168.1.23:3000",
  "sessionAutoRestart": true,
  "sessionHealthCheckSec": 30,
  "sessionStartupTimeoutSec": 30
}
```

**Session Status Flow**:
```
Session Created → STARTING → WORKING (healthy)
                     ↓
                  (timeout)
                     ↓
              Automatic Restart
```

**Monitored Unhealthy States**:
- `STARTING` (if stuck beyond timeout threshold)
- `OPENING` (stuck in opening state)
- `STOPPED` (session stopped)
- `FAILED` (session failed)
- `error` (error state)
- `disconnected` (disconnected from WhatsApp)

### Container Restart Recovery (Advanced)

**NEW**: WhatSignal can automatically restart the WAHA container when session restarts repeatedly fail, helping recover from WAHA service-level issues.

#### When to Use This Feature

This feature is designed for scenarios where:
- WAHA sessions get stuck and session-level restarts don't help
- The WAHA service itself is in a bad state
- You want fully automated recovery without manual intervention
- You're running in a containerized environment (Docker, Docker Compose, Kubernetes)

#### Configuration Options

- `whatsapp.containerRestart.enabled`: Enable/disable container restart feature
  - Default: `false` (disabled)
  - **Important**: This is an opt-in feature for safety
  - Only enable if you understand the implications and have proper monitoring

- `whatsapp.containerRestart.maxConsecutiveFailures`: Number of consecutive session restart failures before triggering container restart
  - Default: `3`
  - Recommended: `3-5` for most deployments
  - Higher values = more tolerant of transient failures
  - Lower values = faster recovery but more aggressive

- `whatsapp.containerRestart.cooldownMinutes`: Minimum time between container restart attempts
  - Default: `5` minutes
  - **Purpose**: Prevents restart loops
  - Recommended: `5-10` minutes for most deployments
  - If container restart also fails, this prevents continuous restart attempts

- `whatsapp.containerRestart.method`: How to restart the container
  - Default: `"webhook"`
  - Options:
    - `"webhook"` (recommended): Calls external webhook endpoint
    - `"docker"` (future): Direct Docker API integration

- `whatsapp.containerRestart.webhookURL`: Webhook endpoint URL for container restart
  - Required when `method` is `"webhook"`
  - Example: `"http://localhost:9000/restart-waha"`
  - The webhook should handle the actual container restart logic
  - See "Webhook Implementation" section below

- `whatsapp.containerRestart.containerName`: Name of the WAHA container
  - Default: `"waha"`
  - Must match your actual WAHA container name
  - Used in webhook payload and Docker API calls

- `whatsapp.containerRestart.dockerSocketPath`: Path to Docker socket
  - Default: `"/var/run/docker.sock"`
  - Only used when `method` is `"docker"`
  - Requires Docker socket to be mounted in whatsignal container

#### Example Configuration

```json
"whatsapp": {
  "api_base_url": "http://waha:3000",
  "sessionAutoRestart": true,
  "sessionHealthCheckSec": 30,
  "sessionStartupTimeoutSec": 30,

  "containerRestart": {
    "enabled": true,
    "maxConsecutiveFailures": 3,
    "cooldownMinutes": 5,
    "method": "webhook",
    "webhookURL": "http://restart-service:9000/restart-waha",
    "containerName": "waha"
  }
}
```

#### Recovery Flow

```
Session Restart Fails (1st time)
  ↓
Session Restart Fails (2nd time)
  ↓
Session Restart Fails (3rd time)
  ↓
Threshold Reached (3 consecutive failures)
  ↓
Check Cooldown Period
  ↓
Trigger Container Restart (via webhook or Docker API)
  ↓
Reset Failure Counter
  ↓
Wait for Container to Restart
  ↓
Session Monitor Resumes Normal Operation
```

#### Webhook Implementation

When using the webhook method, you need to implement a webhook endpoint that handles container restart requests.

**Webhook Request Format**:
```json
POST /restart-waha
Content-Type: application/json
User-Agent: whatsignal-container-restarter

{
  "action": "restart",
  "container_name": "waha",
  "timestamp": "2025-11-08T12:34:56Z"
}
```

**Example Webhook Implementation (Python/Flask)**:
```python
from flask import Flask, request, jsonify
import subprocess

app = Flask(__name__)

@app.route('/restart-waha', methods=['POST'])
def restart_waha():
    data = request.json
    container_name = data.get('container_name', 'waha')

    try:
        # Restart the container using Docker CLI
        subprocess.run(['docker', 'restart', container_name], check=True)
        return jsonify({"status": "success"}), 200
    except subprocess.CalledProcessError as e:
        return jsonify({"status": "error", "message": str(e)}), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=9000)
```

**Example Webhook Implementation (Bash Script)**:
```bash
#!/bin/bash
# restart-waha-webhook.sh

# Simple HTTP server that restarts WAHA container
while true; do
  echo -e "HTTP/1.1 200 OK\n\n" | nc -l -p 9000 -q 1 > /dev/null
  docker restart waha
  echo "$(date): WAHA container restarted"
done
```

**Docker Compose Integration**:
```yaml
services:
  restart-service:
    image: python:3.11-slim
    container_name: restart-service
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./restart-webhook.py:/app/restart-webhook.py
    command: python /app/restart-webhook.py
    ports:
      - "9000:9000"
    networks:
      - whatsignal-network
```

#### Security Considerations

1. **Webhook Method** (Recommended):
   - ✅ Better security isolation
   - ✅ Works in any deployment scenario
   - ✅ No Docker socket access needed in whatsignal container
   - ⚠️ Requires separate webhook service
   - ⚠️ Webhook endpoint should be secured (authentication, rate limiting)

2. **Docker Method** (Future):
   - ⚠️ Requires Docker socket mount (`/var/run/docker.sock`)
   - ⚠️ Gives whatsignal container control over Docker daemon
   - ⚠️ Security risk if whatsignal is compromised
   - ✅ No external dependencies

#### Monitoring and Logging

The container restart feature logs all events:

```
INFO  Session restart successful, resetting failure counter
WARN  Session restart failed consecutive_failures=1 threshold=3
WARN  Session restart failed consecutive_failures=2 threshold=3
WARN  Session restart failed consecutive_failures=3 threshold=3
WARN  Attempting WAHA container restart due to repeated session restart failures
INFO  WAHA container restart initiated successfully
```

Monitor these logs to:
- Track session restart failures
- Verify container restart triggers
- Identify patterns in failures
- Adjust thresholds if needed

#### Troubleshooting

**Container restart not triggering**:
- Verify `enabled` is set to `true`
- Check that consecutive failures reach the threshold
- Ensure cooldown period has elapsed since last restart
- Check logs for failure tracking

**Webhook failures**:
- Verify webhook URL is accessible from whatsignal container
- Check webhook service logs
- Test webhook endpoint manually: `curl -X POST http://webhook-url/restart-waha`
- Ensure webhook returns 2xx status code

**Container restart fails**:
- Check webhook service has Docker socket access
- Verify container name matches actual WAHA container
- Check Docker daemon is running
- Review webhook service logs for errors

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

### Signal Polling Configuration

- `signal.pollIntervalSec`: How often to poll Signal for new messages (in seconds)
  - Default: `5` seconds
  - Recommended: `5-10` seconds for responsive message delivery
  - Lower values = more responsive but higher API load

- `signal.pollTimeoutSec`: Long-polling timeout for Signal message retrieval (in seconds)
  - Default: `15` seconds
  - **Important**: Must be greater than Signal-CLI's internal timeout (10 seconds)
  - Recommended: `15-20` seconds to accommodate Signal-CLI's internal processing
  - This is the `?timeout=` parameter sent to Signal-CLI REST API

- `signal.httpTimeoutSec`: HTTP client timeout for all Signal API requests (in seconds)
  - Default: `30` seconds (if not specified, uses 60 seconds)
  - **Important**: Must be greater than `pollTimeoutSec` to prevent premature client timeouts
  - Recommended: At least `pollTimeoutSec + 10` seconds for buffer
  - This prevents "context deadline exceeded" errors during long-polling

#### Timeout Relationship

The timeout values must follow this hierarchy:
```
Signal-CLI Internal Timeout (10s, fixed)
  < pollTimeoutSec (15s recommended)
  < httpTimeoutSec (30s recommended)
  < Polling Context Timeout (45s, internal constant)
```

**Example Configuration**:
```json
"signal": {
  "rpc_url": "http://192.168.1.23:8081",
  "intermediaryPhoneNumber": "+1234567890",
  "pollIntervalSec": 5,
  "pollTimeoutSec": 15,
  "httpTimeoutSec": 30,
  "pollingEnabled": true,
  "attachmentsDir": "./signal-attachments"
}
```

### Signal Polling Behavior

- `signal.pollingEnabled`: Enable/disable automatic Signal message polling
  - Default: `true`
  - Set to `false` to disable automatic polling (messages won't be received from Signal)


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

## Groups & Reply Threading

This section describes how WhatSignal handles Signal → WhatsApp group routing and reply threading using WAHA.

### Signal → WhatsApp Groups
- Detection: Signal group messages are identified when the Signal sender begins with `group.`.
- Target: WhatsApp group chats always end with `@g.us` (e.g., `12036...@g.us`). WhatSignal enforces group-only routing for Signal group messages.
- Fallback (no quote): If a Signal group message has no quote, WhatSignal resolves the target group by scanning the most recent mappings for the session and selecting the latest group chat (`@g.us`). If none exists, the message is rejected (no WA send).

### Reply Threading
- When the Signal message quotes a previous message and a mapping exists, WhatSignal resolves the original WhatsApp message ID and passes it to WAHA via `reply_to`.
- Applies to both text and media messages.
- If the mapping for a quoted message does not exist, the message is rejected to avoid mis-threading.

### WAHA Payloads (best practices)
- Text (`/api/sendText`):
  ```json
  { "chatId": "12036...@g.us", "text": "Hello group", "session": "personal", "reply_to": "wamid.groupMsgId" }
  ```
- Media (`/api/sendImage`, `/api/sendVideo`, `/api/sendVoice`, `/api/sendDocument`):
  ```json
  {
    "chatId": "12036...@g.us",
    "session": "personal",
    "caption": "Optional caption",
    "reply_to": "wamid.groupMsgId",
    "file": { "data": "<base64>", "mimetype": "image/png", "filename": "pic.png" }
  }
  ```
- Always send media via the dedicated media endpoints (do not use `/api/sendText` for media).
- Ensure your WAHA version supports `reply_to` on these endpoints; WhatSignal includes it when present.

### Security & Media
- Local media paths are validated to prevent directory traversal.
- Media type detection is configuration-driven (see Media Configuration). File extensions can include or omit the leading dot; matching is case-insensitive.


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

- `media.allowedTypes`: File extensions for each media type (case-insensitive; entries may include or omit the leading dot)
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
   curl http://192.168.X.X:3000/api/sessions
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