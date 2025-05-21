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
       "webhookSecret": "your-secret-here"
     }
   }
   ```

### 2. Signal Setup

1. Ensure your signal-cli daemon is running and accessible
2. Verify the RPC URL and auth token in `config.json`

## Message Types

WhatSignal supports the following message types:

1. **Text Messages**
   - Plain text
   - URLs
   - Emojis

2. **Media Messages**
   - Images (JPEG, PNG)
   - Videos (MP4)
   - Audio files
   - Size limits:
     - Images: 5MB
     - Videos: 100MB
     - GIFs: 25MB

3. **Reply Messages**
   - Replies maintain context across platforms
   - Original message reference is preserved

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