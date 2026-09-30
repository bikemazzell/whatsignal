# WhatSignal Deployment Guide

## Quick Deployment (Recommended)

Deploy WhatSignal using pre-built Docker images without needing the source code:

```bash
curl -fsSL https://raw.githubusercontent.com/bikemazzell/whatsignal/main/scripts/deploy.sh | bash
```

This will:
1. Create a `whatsignal-deploy` directory
2. Download necessary configuration files
3. Generate secure secrets automatically
4. Pull the latest Docker images
5. Provide setup instructions

## What You Get

After running the deployment script, you'll have:

```
whatsignal-deploy/
├── docker-compose.yml    # Service orchestration
├── .env                  # Environment variables (with generated secrets)
├── config.json          # Application configuration
├── .env.example          # Environment template
└── config.json.example  # Configuration template
```

## Configuration Steps

1. **Edit Environment Variables** (Required):
   ```bash
   cd whatsignal-deploy
   nano .env
   ```
   Update `WHATSAPP_API_KEY` with your actual WAHA API key.

2. **Edit Application Config** (Required):
   ```bash
   nano config.json
   ```
   Update Signal phone numbers and API URLs for your setup.

3. **Start Services**:
   ```bash
   docker compose up --build -d
   ```

4. **Verify Deployment**:
   ```bash
   # Check all services are running
   docker compose ps
   
   # View logs
   docker compose logs -f
   
   # Test health endpoint
   curl http://localhost:8083/healthz
   ```

## Service Endpoints

- **WhatSignal**: http://localhost:8083
- **WAHA (WhatsApp)**: Compose network only by default
- **Signal-CLI**: Compose network only by default

## Signal directory ownership

Signal REST API 0.101 runs as user ID 1000. Its configuration directory and attachment directory need that owner.
Compose mounts a writable `/run` directory for startup.
It gives `/tmp` a 512 MB limit and permits Java to load its native library there.
If you upgrade an existing installation, stop the Signal service before you change ownership.

```bash
docker compose stop signal-cli-rest-api
docker compose pull signal-cli-rest-api
docker compose run --rm --no-deps --user 0:0 --cap-add CHOWN --cap-add DAC_OVERRIDE --entrypoint chown signal-cli-rest-api -R 1000:1000 /home/.local/share/signal-cli /var/lib/signal-cli-rest-api/attachments
docker compose up -d signal-cli-rest-api
```

On ARM64, set `WAHA_BASE_IMAGE` to the commented ARM64 image in `.env.example` before you build WAHA.
WAHA publishes separate AMD64 and ARM64 images for version 2026.9.1.

## Management Commands

### Upgrade WAHA permissions

The patched WAHA image runs as user ID 1000. Before you start it, give that user ownership of the existing WAHA session directory. This one-shot command runs `chown` as root. The WAHA service itself runs as `node`.

```bash
docker compose build --pull waha
docker compose run --rm --no-deps --user 0:0 --cap-add CHOWN --entrypoint chown waha -R 1000:1000 /app/.sessions
```

Compose mounts the configured session directory at `/app/.sessions`. It keeps the same host path, `./data/waha/sessions`. If `WHATSIGNAL_DATA_DIR` points elsewhere, Compose uses its `waha/sessions` subdirectory. Compose maps `WHATSAPP_API_KEY` to WAHA's `WAHA_API_KEY` setting.

```bash
docker compose up -d
```

```bash
# Start services
docker compose up --build -d

# Stop services
docker compose down

# View logs
docker compose logs -f

# Restart a specific service
docker compose restart whatsignal

# Update images after completing the WAHA permissions step above
docker compose pull whatsignal signal-cli-rest-api
docker compose up -d
```

## Troubleshooting

**Services won't start:**
```bash
docker compose ps       # Check status
docker compose logs     # Check logs
```

**Port conflicts:**
Edit `docker-compose.yml` to change port mappings if needed.

**Start fresh:**
```bash
docker compose down -v  # Removes all data!
./deploy.sh             # Re-run deployment
```

## Security Notes

- The deployment script automatically generates secure secrets
- All services run as non-root users
- Database encryption is enabled by default
- Webhook authentication is enforced

For detailed configuration options, see: [Configuration Guide](configuration.md)
