# Usage Guide

This guide explains how to use WhatSignal for bridging messages between WhatsApp and Signal.

## Basic Operation

WhatSignal operates as a bridge service that:
1. Receives messages from WhatsApp via webhooks
2. Forwards them to Signal
3. Receives replies from Signal
4. Sends them back to WhatsApp

## Setting Up Message Flow

### 1. WhatsApp Configuration

1. Configure your Waha instance to forward messages to WhatSignal:
   ```
   http://your-server:8080/webhook/whatsapp
   ```

2. Set the webhook secret in your `config.json` to match Waha's configuration:
   ```json
   {
     "whatsapp": {
       "webhookSecret": "your-secure-random-waha-secret-here"
     }
   }
   ```

### 2. Signal Setup

1. Ensure your signal-cli daemon is running and accessible (see [Installation Guide](01-installation.md) for daemon setup).
2. Verify the `signal.rpcUrl`, `signal.authToken` (if any), `signal.phoneNumber`, and `signal.deviceName` in `config.json`.
3. Ensure `signal.webhookSecret` is set in `config.json` if you plan to use webhooks from Signal to WhatSignal (currently, WhatSignal polls Signal via JSON-RPC, but this allows for future webhook use).
4. Initial device communication/check with `signal-cli` is handled by WhatSignal on startup using the configured phone number and device name via the `InitializeDevice` method. Full registration or linking of your phone number must be done directly with `signal-cli` first, as detailed in the [Installation Guide](01-installation.md).

## Message Flow

### WhatsApp Message Processing

WhatSignal follows WAHA's best practices for message handling:

1. **Message Receipt**
   - Receives message via webhook
   - Marks message as seen using `/api/sendSeen`
   - Processes message content and media

2. **Message Sending**
   - Indicates typing status with `/api/startTyping`
   - Simulates natural typing delay based on message length
   - Stops typing with `/api/stopTyping`
   - Sends the actual message

3. **Media Handling**
   - Images: `/api/sendImage` (JPEG, PNG)
   - Documents: `/api/sendFile` (PDF, etc.)
   - Voice Messages: `/api/sendVoice` (OGG)
   - Videos: `/api/sendVideo` (MP4)
   - Size limits:
     - Images: 5MB
     - Videos: 100MB
     - Documents: 100MB
     - Voice Messages: 16MB

### Message Types

WhatSignal supports all WAHA message types:

1. **Text Messages**
   - Plain text
   - URLs with preview
   - Emojis
   - Formatted text

2. **Media Messages**
   - Images (JPEG, PNG)
   - Videos (MP4)
   - Documents (PDF, DOC, etc.)
   - Voice messages (OGG)
   - Size limits enforced per platform

3. **Reply Messages**
   - Maintains context across platforms
   - Original message reference preserved
   - Supports media in replies

### Signal Message Flow

1. **Message Receipt**
   - Receives message via signal-cli JSON-RPC
   - Processes message content and attachments
   - Extracts reply context if present

2. **Message Processing**
   - Validates message format and size
   - Processes media attachments
   - Correlates replies with original messages
   - Handles group messages (if supported)

3. **Media Handling**
   - Images: Supported formats (JPEG, PNG)
   - Documents: PDF, DOC, etc.
   - Voice Messages: OGG format
   - Videos: MP4 format
   - Size limits enforced per platform

4. **Reply Handling**
   - Maintains context across platforms
   - Preserves original message references
   - Supports media in replies
   - Handles failed correlations gracefully

## Message Flow Examples

### WhatsApp to Signal

1. User sends message on WhatsApp
2. Waha receives the message
3. Waha forwards to WhatSignal webhook
4. WhatSignal processes and forwards to Signal
5. Message appears in Signal with source context

### Signal to WhatsApp

1. User replies in Signal
2. signal-cli daemon captures the reply
3. WhatSignal processes the reply
4. Message is sent to WhatsApp via Waha
5. Reply appears in WhatsApp thread

## Media Handling

1. **Sending Media**
   - Media files are automatically cached
   - Converted if needed
   - Size-checked against platform limits

2. **Receiving Media**
   - Downloaded and cached locally
   - Processed for platform compatibility
   - Forwarded with appropriate metadata

## Monitoring

### Health Checks

Monitor service health:
```bash
curl http://localhost:8080/health
```

### Message Status

Messages have the following states:
- Sent
- Delivered
- Read
- Failed

## Troubleshooting

### Common Issues

1. **Message Not Delivered**
   - Check service health endpoint
   - Verify webhook configuration
   - Check network connectivity
   - Review logs for errors

2. **Media Transfer Failed**
   - Verify file size limits
   - Check cache directory permissions
   - Ensure sufficient disk space

3. **Missing Messages**
   - Check retention period setting
   - Verify database connectivity
   - Review webhook logs

### Logs

Set log level in `config.json`:
```json
{
  "logLevel": "debug"  // For detailed logging
}
```

## Best Practices

1. **Regular Maintenance**
   - Monitor disk usage
   - Check service health daily
   - Review logs periodically
   - Update dependencies

2. **Security**
   - Keep webhook secret secure
   - Regularly rotate auth tokens
   - Monitor for unauthorized access
   - Keep services updated

3. **Performance**
   - Clean old cache files
   - Monitor database size
   - Check message queues
   - Optimize media handling

## Next Steps

- See [Development Guide](05-development.md) for contributing
- Review [Requirements](06-requirements.md) for technical details 