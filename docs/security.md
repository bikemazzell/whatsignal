# Security Guide

WhatSignal implements multiple layers of security to protect your message data and ensure safe operation.

## Database Encryption

### Overview
WhatSignal supports field-level encryption for sensitive data stored in the SQLite database using AES-256-GCM encryption with PBKDF2 key derivation.

### Configuration
Enable encryption by setting these environment variables:

```bash
# Enable encryption (required)
WHATSIGNAL_ENABLE_ENCRYPTION=true

# Custom encryption secret (recommended, minimum 32 characters)
WHATSIGNAL_ENCRYPTION_SECRET=your-very-secure-encryption-secret-change-this
```

### What Gets Encrypted
When encryption is enabled, the following sensitive fields are encrypted:
- Chat IDs (WhatsApp and Signal)
- Message IDs
- Media file paths
- Any personally identifiable information in message mappings

### Encryption Details
- **Algorithm**: AES-256-GCM (Galois/Counter Mode)
- **Key Derivation**: PBKDF2 with SHA-256
- **Iterations**: 100,000 (industry standard)
- **Nonce**: 12 bytes (GCM standard)
- **Salt**: Fixed application salt for consistency

### Backward Compatibility
- Encryption can be enabled/disabled without data loss
- Existing unencrypted data remains readable
- New data will be encrypted when encryption is enabled
- Gradual migration is supported

## Data Retention and Cleanup

### Automatic Cleanup
WhatSignal automatically cleans up old message data:
- **Schedule**: Every 24 hours
- **Default Retention**: 30 days (configurable)
- **Cleanup Scope**: Database records and associated media files

### Configuration
Set retention period in your configuration:
```json
{
  "retentionDays": 30
}
```

## File System Security

### Database File Permissions
- SQLite database file is created with `0600` permissions (owner read/write only)
- Prevents unauthorized access from other system users

### Media File Handling
- Media files are stored in a dedicated cache directory
- Automatic cleanup removes orphaned media files
- Size limits prevent disk space exhaustion

## Network Security

### Webhook Authentication
- All webhook endpoints require authentication
- Webhook secret must be minimum 32 characters
- Secrets are validated on every request

### API Communication
- All external API calls use HTTPS where possible
- Proper error handling prevents information leakage

## Environment Security

### Secret Management
Store all secrets in environment variables, never in code:

```bash
# Required secrets
WEBHOOK_SECRET=your-very-secure-random-string-for-waha
WHATSIGNAL_ENCRYPTION_SECRET=your-very-secure-encryption-secret

# Optional overrides
WHATSAPP_API_URL=https://your-waha-instance
SIGNAL_RPC_URL=https://your-signal-cli-instance
```

### Docker Security
When using Docker:
- Use non-root user in containers
- Mount volumes with appropriate permissions
- Use Docker secrets for sensitive data in production

## Security Best Practices

### Production Deployment
1. **Enable Encryption**: Always enable database encryption in production
2. **Strong Secrets**: Use cryptographically secure random strings (32+ characters)
3. **Regular Updates**: Keep dependencies and base images updated
4. **Access Control**: Restrict network access to necessary ports only
5. **Monitoring**: Monitor logs for suspicious activity

### Secret Generation
Generate secure secrets using:
```bash
# Generate 32-character random string
openssl rand -base64 32

# Or using /dev/urandom
head -c 32 /dev/urandom | base64
```

### Environment Isolation
- Use separate environments for development/staging/production
- Never share secrets between environments
- Use different encryption keys for each environment

## Threat Model

### Protected Against
- **Unauthorized Database Access**: Encryption protects data at rest
- **File System Access**: Proper permissions prevent unauthorized reads
- **Data Retention**: Automatic cleanup limits exposure window
- **Webhook Attacks**: Authentication prevents unauthorized message injection

### Not Protected Against
- **Memory Dumps**: Decrypted data exists in memory during processing
- **Root Access**: Root users can access all data regardless of permissions
- **Side-Channel Attacks**: Standard application-level encryption limitations
- **Endpoint Compromise**: If WhatsApp/Signal APIs are compromised

## Compliance Considerations

### Data Protection
- Minimal data collection (only message routing metadata)
- Configurable retention periods
- Encryption at rest
- Automatic cleanup

### Privacy
- No message content analysis or logging
- Temporary media file storage only
- No persistent user profiling

## Incident Response

### Security Breach Response
1. **Immediate**: Stop the service and isolate the system
2. **Assessment**: Determine scope of potential data exposure
3. **Mitigation**: Rotate all secrets and encryption keys
4. **Recovery**: Restore from clean backups if necessary
5. **Prevention**: Update security measures based on findings

### Key Rotation
To rotate encryption keys:
1. Stop the service
2. Export existing data (if needed)
3. Update `WHATSIGNAL_ENCRYPTION_SECRET`
4. Restart the service
5. Old encrypted data will be inaccessible (by design)

## Security Updates

Stay informed about security updates:
- Monitor the project repository for security advisories
- Subscribe to dependency security alerts
- Regularly update base Docker images
- Review security logs periodically

## Reporting Security Issues

If you discover a security vulnerability:
1. **Do not** create a public issue
2. Email security concerns to the maintainers
3. Provide detailed reproduction steps
4. Allow reasonable time for response and fixes 