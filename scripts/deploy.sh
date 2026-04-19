#!/bin/bash

# WhatSignal Deployment Script
# This script deploys WhatSignal using pre-built Docker images

set -e

REPO_URL="https://raw.githubusercontent.com/bikemazzell/whatsignal/main"
VERSION="latest"

echo "🚀 WhatSignal Deployment"
echo "======================="
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

# Create deployment directory
DEPLOY_DIR="whatsignal-deploy"
if [ -d "$DEPLOY_DIR" ]; then
    echo "📁 Directory $DEPLOY_DIR already exists"
    read -p "Do you want to continue and overwrite? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    mkdir -p "$DEPLOY_DIR"
fi

cd "$DEPLOY_DIR"

echo "📥 Downloading configuration files..."

# Download required files
curl -fsSL "$REPO_URL/docker-compose.yml" -o docker-compose.yml
curl -fsSL "$REPO_URL/.env.example" -o .env.example
curl -fsSL "$REPO_URL/config.json.example" -o config.json.example

echo "✅ Downloaded configuration files"

# Create .env file if it doesn't exist
if [ ! -f .env ]; then
    echo "📝 Creating .env file..."
    cp .env.example .env
    
    # Generate secure secrets
    echo "🔐 Generating secure secrets..."
    if command -v openssl &> /dev/null; then
        WHATSAPP_WEBHOOK_SECRET=$(openssl rand -base64 32)
        WHATSIGNAL_ENCRYPTION_SECRET=$(openssl rand -base64 32)
        WHATSIGNAL_ENCRYPTION_SALT=$(openssl rand -base64 24)
        WHATSIGNAL_ENCRYPTION_LOOKUP_SALT=$(openssl rand -base64 24)
        WHATSIGNAL_ADMIN_TOKEN=$(openssl rand -base64 32)
        API_KEY=$(openssl rand -base64 16 | tr -d '=')
    else
        WHATSAPP_WEBHOOK_SECRET=$(head -c 32 /dev/urandom | base64)
        WHATSIGNAL_ENCRYPTION_SECRET=$(head -c 32 /dev/urandom | base64)
        WHATSIGNAL_ENCRYPTION_SALT=$(head -c 24 /dev/urandom | base64)
        WHATSIGNAL_ENCRYPTION_LOOKUP_SALT=$(head -c 24 /dev/urandom | base64)
        WHATSIGNAL_ADMIN_TOKEN=$(head -c 32 /dev/urandom | base64)
        API_KEY=$(head -c 16 /dev/urandom | base64 | tr -d '=')
    fi
    
    # Update .env file with generated secrets (use | delimiter to avoid conflicts with base64 /)
    sed -i.bak "s|your-waha-api-key|$API_KEY|" .env
    sed -i.bak "s|your-very-secure-whatsapp-webhook-secret|$WHATSAPP_WEBHOOK_SECRET|" .env
    sed -i.bak "s|your-very-secure-encryption-secret-change-this|$WHATSIGNAL_ENCRYPTION_SECRET|" .env
    sed -i.bak "s|your-unique-encryption-salt|$WHATSIGNAL_ENCRYPTION_SALT|" .env
    sed -i.bak "s|your-unique-lookup-salt|$WHATSIGNAL_ENCRYPTION_LOOKUP_SALT|" .env
    sed -i.bak "s|your-very-secure-admin-token|$WHATSIGNAL_ADMIN_TOKEN|" .env
    
    # Remove backup file
    rm -f .env.bak
    
    echo "✅ Generated .env file with secure secrets"
else
    echo "✅ .env file already exists"
fi

# Create config.json if it doesn't exist
if [ ! -f config.json ]; then
    echo "📝 Creating config.json..."
    cp config.json.example config.json
    echo "✅ Created config.json from example"
else
    echo "✅ config.json already exists"
fi

echo
echo "🐳 Pulling Docker images..."
docker compose pull

echo
echo "✅ Deployment files ready!"
echo
echo "📋 Next steps:"
echo "1. Edit .env file with your actual API keys:"
echo "   nano .env"
echo
echo "2. Edit config.json with your Signal/WhatsApp configuration:"
echo "   nano config.json"
echo "   (Update phone numbers and API URLs)"
echo
echo "3. Start the services:"
echo "   docker compose up -d"
echo
echo "4. Check status:"
echo "   docker compose ps"
echo "   docker compose logs -f"
echo
echo "5. Test health:"
echo "   curl http://localhost:8082/health"
echo
echo "🔧 Useful commands:"
echo "  docker compose up -d      # Start services"
echo "  docker compose down       # Stop services"
echo "  docker compose logs -f    # View logs"
echo "  docker compose ps         # Check status"
echo "  docker compose restart    # Restart services"
echo
echo "📖 For detailed configuration help:"
echo "   https://github.com/bikemazzell/whatsignal/blob/main/docs/configuration.md"
echo
echo "🎉 Happy bridging!"
