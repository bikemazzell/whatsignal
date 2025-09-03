# Docker Security Guide for WhatsSignal

This document outlines the security hardening implemented in WhatsSignal's Docker configuration.

## Security Measures Implemented

### 1. Base Image Hardening

**Before (Vulnerable):**
```dockerfile
FROM alpine:latest
```

**After (Secured):**
```dockerfile
# Pin Go Alpine image with digest for security
FROM golang:1.22-alpine@sha256:ace970c91d8b2dd0b60eab4c4d52e5b6b7e33b50c50ad67b24e2cfcdf0f29a4e AS builder

# Final stage - use distroless image for security
FROM gcr.io/distroless/static-debian12:nonroot@sha256:e9ac71e2b8e279a8372741b7a0293afda06650142b63365b4dfa0a5fc0f3fdc7
```

**Benefits:**
- **Image pinning with SHA digests**: Prevents supply chain attacks via image tag poisoning
- **Distroless base image**: Removes shell, package managers, and unnecessary tools
- **Minimal attack surface**: Only contains essential runtime libraries

### 2. Runtime Security

#### Read-only Root Filesystem
```yaml
services:
  whatsignal:
    read_only: true  # Prevents malicious file creation
    tmpfs:
      - /tmp:noexec,nosuid,size=100m  # Temporary files without execution
```

#### Non-root User
```dockerfile
# Distroless nonroot user (uid=65532)
USER nonroot:nonroot
```

#### Dropped Capabilities
```yaml
cap_drop:
  - ALL  # Remove all Linux capabilities
# Only add back what's absolutely necessary (none for WhatsSignal)
```

#### Security Options
```yaml
security_opt:
  - no-new-privileges:true  # Prevent privilege escalation
```

### 3. Resource Constraints

```yaml
deploy:
  resources:
    limits:
      memory: 512M     # Prevent memory exhaustion attacks
      cpus: '0.5'      # CPU usage limits
    reservations:
      memory: 128M     # Guaranteed minimum resources
```

### 4. Network Security (optional)

```yaml
networks:
  whatsignal-net:
    driver: bridge
```

- Optional custom network for isolation and service-name DNS
- Do not disable inter-container communication (ICC) on this network if services must talk to each other
- Using the default bridge is also fine; the repo’s compose now uses the default bridge by default

### 5. Volume Security

**Principle**: Only mount what's necessary as writable
```yaml
volumes:
  # Read-only configuration
  - ./config.json:/app/config.json:ro

  # Writable data volumes (minimal)
### Practical Security Balance

WhatsSignal uses a balanced approach to security that maintains strong protection while ensuring functionality:

#### Service-Specific Security Requirements

**WhatsSignal (Main Bridge):**
- `read_only: true` - Works with distroless image
- `tmpfs` mounts for `/tmp` and `/var/cache`
- Minimal volume mounts for essential data only

**WAHA (WhatsApp API):**
- `read_only: true` - But requires extensive tmpfs mounts for Chrome
- Chrome needs `/tmp`, `/var/tmp`, `/dev/shm`, and `/var/crashes`
- Requires `PUPPETEER_ARGS` for container compatibility

**Signal-CLI REST API:**
- `read_only: false` - **Must be writable** due to system file modifications
- Uses tmpfs for `/tmp` to avoid conflicts
- Requires write access for user/group management during startup

#### Example Configuration (from docker-compose.yml):
```yaml
services:
  # WhatsSignal - can use read-only with tmpfs
  whatsignal:
    read_only: true
    user: "1000:1000"
    tmpfs:
      - /tmp:size=100M,mode=1777
      - /var/cache:size=50M,mode=1777
    volumes:
      - /mnt/data/whatsignal/config.json:/app/config.json:ro
      - /mnt/data/whatsignal/data:/app/data
      - signal_attachments:/app/signal-attachments:rw

  # WAHA - requires Chrome-specific tmpfs mounts
  waha:
    read_only: true
    tmpfs:
      - /tmp:size=200M,mode=1777
      - /var/tmp:size=50M,mode=1777
      - /dev/shm:size=2G,mode=1777
      - /var/crashes:size=100M,mode=1777
    environment:
      - PUPPETEER_ARGS=--no-sandbox --disable-setuid-sandbox --crash-dumps-dir=/var/crashes

  # Signal-CLI - cannot use read-only due to system modifications
  signal-cli-rest-api:
    read_only: false  # Required for system file access
    tmpfs:
      - /tmp:size=100M,mode=1777
```

**Important Notes:**
- **Do not** bind-mount `/tmp` or `/var/cache` when using `read_only: true`
- Use `tmpfs` mounts instead to avoid filesystem conflicts
- Signal-CLI **must** have `read_only: false` to function properly

  - whatsignal-data:/app/data:rw
  - whatsignal-cache:/app/media-cache:rw
```

## Deployment Options

### Standard Deployment
Use docker-compose.yml for development and production setups:
```bash
docker compose up -d
```

### Security-Hardened Example
A hardened variant docker-hardened.yml is provided; it applies read-only root, dropped capabilities, and no-new-privileges while keeping user 1000:
```bash
docker compose -f docker-hardened.yml up -d
```

### Manual Secure Deployment
For environments requiring custom security policies:
```bash
docker run -d \
  --name whatsignal-secure \
  --read-only \
  --tmpfs /tmp:noexec,nosuid,size=100m \
  --tmpfs /var/tmp:noexec,nosuid,size=100m \
  -v whatsignal-data:/app/data:rw \
  -v whatsignal-cache:/app/media-cache:rw \
  -v whatsignal-attachments:/app/signal-attachments:rw \
  --cap-drop ALL \
  --security-opt no-new-privileges=true \
  --user 65532:65532 \
  --memory 512m \
  --cpus 0.5 \
  -p 8082:8082 \
  whatsignal:latest
```

## Security Verification

### 1. Verify Image Signatures (Future Enhancement)
```bash
# Enable Docker Content Trust
export DOCKER_CONTENT_TRUST=1
docker pull whatsignal:latest
```

### 2. Scan Images for Vulnerabilities
```bash
# Using Trivy
trivy image ghcr.io/bikemazzell/whatsignal:latest

# Using Docker Scout
docker scout quickview ghcr.io/bikemazzell/whatsignal:latest
```

### 3. Runtime Security Monitoring
```bash
# Check running containers
docker inspect whatsignal-secure | jq '.[0].HostConfig | {ReadonlyRootfs, CapDrop, SecurityOpt}'

# Verify user
docker exec whatsignal-secure id
```

## Security Checklist

- [ ] **Base Images**: Pinned with SHA digests
- [ ] **Distroless**: Using minimal runtime image (WhatsSignal only)
- [ ] **Non-root**: Running as unprivileged user where possible
- [ ] **Read-only**: Root filesystem read-only where feasible (WhatsSignal: ✓, WAHA: ✓, Signal-CLI: ✗)
- [ ] **Capabilities**: All capabilities dropped
- [ ] **No Privileges**: Privilege escalation disabled
- [ ] **Resource Limits**: Memory and CPU constraints set (optional)
- [ ] **Network**: Default bridge network (sufficient for most use cases)
- [ ] **Volumes**: Minimal writable mounts only
- [ ] **Tmpfs**: Use tmpfs instead of volume binds for temporary directories
- [ ] **Chrome Security**: WAHA configured with proper Puppeteer arguments
- [ ] **Secrets**: No secrets in environment variables (use Docker secrets or external management)
- [ ] **Health Checks**: Container health monitoring enabled

## Environment-Specific Recommendations

### Development
- Use standard `docker-compose.yml`
- Enable debug logging: `LOG_LEVEL=debug`
- Mount source code for hot reload

### Staging
- Use docker-hardened.yml as a baseline hardened configuration
- Test with production-like security constraints
- Enable security scanning in CI/CD

### Production
- Start from docker-hardened.yml and tailor to your environment
- Implement external secret management (HashiCorp Vault, AWS Secrets Manager)
- Enable runtime security monitoring (Falco, Sysdig)
- Regular vulnerability scanning
- Network policies with Kubernetes NetworkPolicy or Docker network isolation
- Log aggregation and monitoring

## Compliance Standards

This security configuration helps meet:
- **CIS Docker Benchmark**: Container security best practices
- **NIST Cybersecurity Framework**: Security controls implementation
- **OWASP Container Security**: Top 10 container risks mitigation
- **SOC 2 Type II**: Security and availability controls

## Troubleshooting Common Issues

### Chrome/Puppeteer Errors in WAHA
**Error**: `chrome_crashpad_handler: --database is required`
**Solution**: Add tmpfs mounts and Puppeteer arguments:
```yaml
waha:
  tmpfs:
    - /tmp:size=200M,mode=1777
    - /dev/shm:size=2G,mode=1777
    - /var/crashes:size=100M,mode=1777
  environment:
    - PUPPETEER_ARGS=--no-sandbox --disable-setuid-sandbox --crash-dumps-dir=/var/crashes
```

### Signal-CLI Group Modification Errors
**Error**: `groupmod: cannot lock /etc/group; try again later`
**Solution**: Set `read_only: false` for signal-cli-rest-api:
```yaml
signal-cli-rest-api:
  read_only: false  # Required for system file modifications
```

### Read-only Filesystem Conflicts
**Error**: `EROFS: read-only file system, mkdir '/tmp/...'`
**Solution**: Use tmpfs mounts instead of volume binds for temporary directories:
```yaml
# ✗ Wrong - causes conflicts
volumes:
  - /host/tmp:/tmp

# ✓ Correct - use tmpfs
tmpfs:
  - /tmp:size=100M,mode=1777
```

### Distroless Container Shell Errors
**Error**: `exec: "/bin/sh": stat /bin/sh: no such file or directory`
**Solution**: Remove shell-based entrypoints and healthchecks. Use direct binary execution.

## Maintenance

### Regular Security Updates
1. **Base Image Updates**: Monitor for new distroless image releases
2. **Dependency Scanning**: Regular `govulncheck` and `trivy` scans
3. **Security Patches**: Apply Go and system library updates
4. **Configuration Review**: Quarterly security configuration audit

### Monitoring and Alerting
- Container escape attempts
- Unusual network connections
- Resource usage anomalies
- Failed health checks
- Security policy violations