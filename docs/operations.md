# Operations Guide

This guide covers deployment, monitoring, and maintenance of WhatSignal in production environments.

## Production Deployment

### Environment Setup

1. **Create Production Environment File**:
```bash
cp env.example .env.production
```

2. **Configure Security Settings**:
```bash
# Required: Strong webhook secret (32+ characters)
WEBHOOK_SECRET=$(openssl rand -base64 32)

# Required: Enable encryption
WHATSIGNAL_ENABLE_ENCRYPTION=true

# Required: Strong encryption secret (32+ characters)
WHATSIGNAL_ENCRYPTION_SECRET=$(openssl rand -base64 32)
```

3. **Set Production Configuration**:
```json
{
  "server": {
    "port": 8082,
    "host": "0.0.0.0"
  },
  "whatsapp": {
    "apiUrl": "https://your-waha-instance.com",
    "webhookSecret": "env:WEBHOOK_SECRET"
  },
  "signal": {
    "rpcUrl": "https://your-signal-cli-instance.com"
  },
  "database": {
    "path": "/app/data/whatsignal.db"
  },
  "media": {
    "cacheDir": "/app/cache",
    "maxSizeMB": 50
  },
  "retentionDays": 30,
  "logLevel": "info"
}
```

### Docker Deployment

1. **Build Production Image**:
```bash
docker build -t whatsignal:production .
```

2. **Run with Security**:
```bash
docker run -d \
  --name whatsignal \
  --env-file .env.production \
  -v whatsignal-data:/app/data \
  -v whatsignal-cache:/app/cache \
  -p 8082:8082 \
  --restart unless-stopped \
  --security-opt no-new-privileges:true \
  --read-only \
  --tmpfs /tmp \
  whatsignal:production
```

### Docker Compose Production

```yaml
version: '3.8'
services:
  whatsignal:
    build: .
    env_file: .env.production
    ports:
      - "8082:8082"
    volumes:
      - whatsignal-data:/app/data
      - whatsignal-cache:/app/cache
    restart: unless-stopped
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8082/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

volumes:
  whatsignal-data:
  whatsignal-cache:
```

## Monitoring

### Health Checks

**Endpoint**: `GET /health`
```bash
curl http://localhost:8082/health
```

**Expected Response**:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T12:00:00Z",
  "version": "1.0.0"
}
```

### Log Monitoring

**Key Log Patterns to Monitor**:
```bash
# Successful operations
grep "Successfully completed cleanup" /var/log/whatsignal.log

# Error patterns
grep -E "(ERROR|FATAL)" /var/log/whatsignal.log

# Security events
grep -E "(authentication|encryption|unauthorized)" /var/log/whatsignal.log

# Performance metrics
grep -E "(cleanup|processing time)" /var/log/whatsignal.log
```

### Metrics Collection

**Prometheus Metrics** (if implemented):
- `whatsignal_messages_processed_total`
- `whatsignal_cleanup_duration_seconds`
- `whatsignal_database_size_bytes`
- `whatsignal_media_files_count`

### Alerting Rules

**Critical Alerts**:
- Service down (health check fails)
- Database encryption errors
- Disk space > 90%
- Memory usage > 80%

**Warning Alerts**:
- Cleanup failures
- High message processing latency
- Authentication failures

## Maintenance

### Regular Tasks

**Daily**:
- Check service health
- Review error logs
- Monitor disk usage

**Weekly**:
- Review cleanup effectiveness
- Check for security updates
- Validate backup integrity

**Monthly**:
- Rotate encryption keys (if required)
- Update dependencies
- Performance review

### Database Maintenance

**Check Database Size**:
```bash
docker exec whatsignal ls -lh /app/data/whatsignal.db
```

**Manual Cleanup** (if needed):
```bash
# Connect to running container
docker exec -it whatsignal /bin/sh

# Check retention configuration
cat /app/config.json | grep retentionDays
```

**Backup Database**:
```bash
# Create backup
docker cp whatsignal:/app/data/whatsignal.db ./backup-$(date +%Y%m%d).db

# Verify backup
sqlite3 ./backup-$(date +%Y%m%d).db ".tables"
```

### Media Cache Management

**Check Cache Size**:
```bash
docker exec whatsignal du -sh /app/cache
```

**Manual Cache Cleanup**:
```bash
# Remove files older than 7 days
docker exec whatsignal find /app/cache -type f -mtime +7 -delete
```

### Security Maintenance

**Key Rotation Process**:
1. Generate new encryption secret:
   ```bash
   NEW_SECRET=$(openssl rand -base64 32)
   ```

2. Update environment:
   ```bash
   # Update .env.production
   WHATSIGNAL_ENCRYPTION_SECRET=$NEW_SECRET
   ```

3. Restart service:
   ```bash
   docker restart whatsignal
   ```

**Note**: Old encrypted data becomes inaccessible after key rotation.

### Updates and Patches

**Update Process**:
1. **Backup**: Create database and configuration backups
2. **Test**: Deploy to staging environment first
3. **Deploy**: Update production with minimal downtime
4. **Verify**: Check health and functionality
5. **Monitor**: Watch logs for issues

**Rolling Update**:
```bash
# Pull latest code
git pull origin main

# Build new image
docker build -t whatsignal:latest .

# Update with zero downtime
docker-compose up -d --no-deps whatsignal
```

## Troubleshooting

### Common Issues

**Service Won't Start**:
```bash
# Check logs
docker logs whatsignal

# Common causes:
# - Missing environment variables
# - Database permission issues
# - Port conflicts
```

**Encryption Errors**:
```bash
# Check encryption configuration
docker exec whatsignal env | grep WHATSIGNAL_

# Verify secret length (should be 32+ characters)
echo $WHATSIGNAL_ENCRYPTION_SECRET | wc -c
```

**Database Issues**:
```bash
# Check database file permissions
docker exec whatsignal ls -la /app/data/

# Test database connectivity
docker exec whatsignal sqlite3 /app/data/whatsignal.db ".tables"
```

**Memory Issues**:
```bash
# Check memory usage
docker stats whatsignal

# Check for memory leaks in logs
docker logs whatsignal | grep -i "memory\|oom"
```

### Performance Tuning

**Database Optimization**:
- Monitor database size growth
- Adjust retention period based on usage
- Consider VACUUM operations for SQLite

**Memory Management**:
- Monitor Go garbage collection
- Adjust container memory limits
- Watch for memory leaks

**Disk I/O**:
- Use SSD storage for database
- Monitor cache directory growth
- Implement log rotation

## Disaster Recovery

### Backup Strategy

**Automated Backups**:
```bash
#!/bin/bash
# backup-whatsignal.sh
DATE=$(date +%Y%m%d_%H%M%S)
docker cp whatsignal:/app/data/whatsignal.db /backups/whatsignal_$DATE.db
docker cp whatsignal:/app/config.json /backups/config_$DATE.json

# Retain last 30 days
find /backups -name "whatsignal_*.db" -mtime +30 -delete
```

**Recovery Process**:
1. Stop service
2. Restore database file
3. Restore configuration
4. Start service
5. Verify functionality

### High Availability

**Load Balancing**:
- Multiple instances behind load balancer
- Shared database storage
- Session affinity for webhooks

**Failover**:
- Health check-based failover
- Automated restart policies
- Monitoring and alerting

## Security Operations

### Security Monitoring

**Log Analysis**:
```bash
# Authentication failures
grep "authentication failed" /var/log/whatsignal.log

# Encryption errors
grep "encryption\|decrypt" /var/log/whatsignal.log

# Suspicious activity
grep -E "(unauthorized|invalid|malformed)" /var/log/whatsignal.log
```

**Security Audits**:
- Regular dependency scans
- Container image vulnerability scans
- Configuration reviews
- Access log analysis

### Incident Response

**Security Incident Checklist**:
1. **Isolate**: Stop service and network access
2. **Assess**: Determine scope and impact
3. **Contain**: Prevent further damage
4. **Investigate**: Analyze logs and evidence
5. **Recover**: Restore from clean state
6. **Learn**: Update security measures

**Contact Information**:
- Security team: security@yourcompany.com
- On-call engineer: +1-xxx-xxx-xxxx
- Incident management: incident@yourcompany.com 