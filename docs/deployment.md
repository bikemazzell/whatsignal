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
├── env.example          # Environment template
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
   docker compose up -d
   ```

4. **Verify Deployment**:
   ```bash
   # Check all services are running
   docker compose ps
   
   # View logs
   docker compose logs -f
   
   # Test health endpoint
   curl http://localhost:8082/health
   ```

## Service Endpoints

- **WhatSignal**: http://localhost:8082
- **WAHA (WhatsApp)**: http://localhost:3000
- **Signal-CLI**: http://localhost:8080

## Management Commands

```bash
# Start services
docker compose up -d

# Stop services
docker compose down

# View logs
docker compose logs -f

# Restart a specific service
docker compose restart whatsignal

# Update to latest images
docker compose pull && docker compose up -d
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