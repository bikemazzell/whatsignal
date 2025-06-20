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
curl -fsSL "$REPO_URL/docker-compose.prod.yml" -o docker-compose.yml
curl -fsSL "$REPO_URL/env.example" -o env.example
curl -fsSL "$REPO_URL/config.json.example" -o config.json.example

echo "✅ Downloaded configuration files"

# Create .env file if it doesn't exist
if [ ! -f .env ]; then
    echo "📝 Creating .env file..."
    cp env.example .env
    
    # Generate secure secrets
    echo "🔐 Generating secure secrets..."
    if command -v openssl &> /dev/null; then
        WEBHOOK_SECRET=$(openssl rand -base64 32)
        ENCRYPTION_SECRET=$(openssl rand -base64 32)
        API_KEY=$(openssl rand -base64 16 | tr -d '=')
    else
        WEBHOOK_SECRET=$(head -c 32 /dev/urandom | base64)
        ENCRYPTION_SECRET=$(head -c 32 /dev/urandom | base64)
        API_KEY=$(head -c 16 /dev/urandom | base64 | tr -d '=')
    fi
    
    # Update .env file with generated secrets
    sed -i.bak "s/your-waha-api-key/$API_KEY/" .env
    sed -i.bak "s/your-very-secure-random-string-for-waha/$WEBHOOK_SECRET/" .env
    sed -i.bak "s/your-very-secure-encryption-secret-change-this/$ENCRYPTION_SECRET/" .env
    
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