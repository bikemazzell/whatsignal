#!/bin/bash

# WhatSignal Docker Setup Script
# This script helps you quickly set up WhatSignal with Docker

set -e

echo "🚀 WhatSignal Docker Setup"
echo "=========================="
echo

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed. Please install Docker first."
    exit 1
fi

# Check if Docker Compose is available
if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    echo "❌ Docker Compose is not available. Please install Docker Compose first."
    exit 1
fi

echo "✅ Docker is installed"

# Create .env file if it doesn't exist
if [ ! -f .env ]; then
    echo "📝 Creating .env file..."
    cp .env.example .env
    
    # Generate secure secrets
    echo "🔐 Generating secure secrets..."
    WHATSAPP_WEBHOOK_SECRET=$(openssl rand -base64 32 2>/dev/null || head -c 32 /dev/urandom | base64)
    WHATSIGNAL_ENCRYPTION_SECRET=$(openssl rand -base64 32 2>/dev/null || head -c 32 /dev/urandom | base64)
    
    # Update .env file with generated secrets
    sed -i.bak "s/your-waha-api-key/$(openssl rand -base64 16 | tr -d '=' | tr -d '\n')/" .env
    sed -i.bak "s/your-very-secure-whatsapp-webhook-secret/$WHATSAPP_WEBHOOK_SECRET/" .env
    sed -i.bak "s/your-very-secure-encryption-secret-change-this/$WHATSIGNAL_ENCRYPTION_SECRET/" .env
    
    # Remove backup file
    rm -f .env.bak
    
    echo "✅ Generated .env file with secure secrets"
    echo "⚠️  IMPORTANT: Please review and update .env with your actual values"
else
    echo "✅ .env file already exists"
fi

# Create config.json if it doesn't exist
if [ ! -f config.json ]; then
    echo "📝 Creating config.json..."
    cp config.json.example config.json
    echo "✅ Created config.json from example"
    echo "⚠️  IMPORTANT: Please review and update config.json with your actual values"
else
    echo "✅ config.json already exists"
fi

echo
echo "🐳 Docker Setup Complete!"
echo
echo "Next steps:"
echo "1. Edit .env file with your actual API keys and settings"
echo "2. Edit config.json with your Signal/WhatsApp configuration"
echo "3. Start the services:"
echo "   make docker-up"
echo
echo "Useful commands:"
echo "  make docker-status  - Check service status"
echo "  make docker-logs    - View logs"
echo "  make docker-down    - Stop services"
echo "  make help          - Show all available commands"
echo
echo "For detailed setup instructions, see: README.md"