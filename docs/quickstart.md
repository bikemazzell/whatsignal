# WhatSignal Quick Start Checklist

## Prerequisites ✅

- [ ] Docker installed
- [ ] Docker Compose available
- [ ] WhatsApp account for WAHA
- [ ] Signal account for Signal-CLI

## Deployment Steps

### 1. Deploy WhatSignal
```bash
curl -fsSL https://raw.githubusercontent.com/bikemazzell/whatsignal/main/scripts/deploy.sh | bash
cd whatsignal-deploy
```

### 2. Configure Environment
Edit `.env` file:
- [ ] Set `WHATSAPP_API_KEY` (get from your WAHA setup)
- [ ] Verify `WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET` (auto-generated)
- [ ] Verify `WHATSIGNAL_ENCRYPTION_SECRET` (auto-generated)

### 3. Configure Application
Edit `config.json` file:
- [ ] Set `signal.intermediaryPhoneNumber` (your Signal-CLI number)
- [ ] Set `signal.destinationPhoneNumber` (your personal Signal number)
- [ ] Update `whatsapp.api_base_url` if needed (default: http://waha:3000)
- [ ] Update `signal.rpc_url` if needed (default: http://signal-cli-rest-api:8080)

### 4. Start Services
```bash
docker compose up -d
```

### 5. Verify Deployment
- [ ] Check services are running: `docker compose ps`
- [ ] Test health endpoint: `curl http://localhost:8082/health`
- [ ] Check logs for errors: `docker compose logs`

### 6. Set Up WhatsApp (WAHA)
1. [ ] Open http://localhost:3000 in browser
2. [ ] Create/start WhatsApp session
3. [ ] Scan QR code with your phone
4. [ ] Verify session is active

### 7. Set Up Signal (Signal-CLI)
1. [ ] Register your Signal bridge number:
   ```bash
   docker exec -it signal-cli-rest-api signal-cli -u +YOUR_BRIDGE_NUMBER register
   ```
2. [ ] Verify with SMS code:
   ```bash
   docker exec -it signal-cli-rest-api signal-cli -u +YOUR_BRIDGE_NUMBER verify CODE
   ```

### 8. Test the Bridge
1. [ ] Send a WhatsApp message to your bridge number
2. [ ] Verify it appears on your personal Signal
3. [ ] Reply from Signal and verify it goes to WhatsApp

## Quick Commands

```bash
# View all logs
docker compose logs -f

# Restart WhatSignal only
docker compose restart whatsignal

# Stop everything
docker compose down

# Update to latest version
docker compose pull && docker compose up -d
```

## Common Issues

**Health check fails:**
- Check if all services are running: `docker compose ps`
- View logs: `docker compose logs whatsignal`

**WhatsApp not connecting:**
- Check WAHA logs: `docker compose logs waha`
- Verify QR code scan at http://localhost:3000

**Signal not working:**
- Check Signal-CLI logs: `docker compose logs signal-cli-rest-api`
- Verify phone number registration

**Messages not bridging:**
- Check WhatSignal logs: `docker compose logs whatsignal`
- Verify webhook configuration in WAHA
- Test health endpoint: `curl http://localhost:8082/health`

## Need Help?

- 📖 [Full Documentation](../README.md)
- 🔧 [Configuration Guide](configuration.md)
- 🔒 [Security Guide](security.md)
- 🚀 [Deployment Guide](deployment.md)