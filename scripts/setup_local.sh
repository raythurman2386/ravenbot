#!/bin/bash

# setup_local.sh
# Sets up the local development environment for ravenbot.

set -e

echo "🦅 Setting up ravenbot local environment..."

# 1. Check Dependencies
echo "🔍 Checking dependencies..."

if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.25+."
    exit 1
fi
echo "✅ Go found: $(go version)"

if ! command -v chromium &> /dev/null && ! command -v google-chrome &> /dev/null; then
    echo "⚠️  Chromium/Chrome not found. 'BrowseWeb' tool might fail."
    echo "   Please install 'chromium' or 'google-chrome' and set CHROME_BIN in your .env."
else
    echo "✅ Chromium/Chrome found."
fi

if ! command -v npm &> /dev/null; then
    echo "⚠️  npm is not installed. MCP servers will not work."
else
    echo "✅ npm found: $(npm -v)"
fi

# 2. Go Dependencies
echo "📦 Installing Go dependencies..."
go mod download
echo "✅ Dependencies installed."

# 3. Environment Setup
if [ ! -f .env ]; then
    echo "📝 Creating .env from .env.example..."
    cp .env.example .env
    echo "⚠️  Please edit .env and add your API keys!"
else
    echo "✅ .env already exists."
fi

# 4. Create Directories
echo "📂 Creating necessary directories..."
mkdir -p daily_logs
echo "✅ 'daily_logs' directory ready."

# 5. Build
echo "🔨 Building ravenbot..."
if make build; then
    echo "✅ Build successful! Binary is at ./ravenbot"
else
    echo "❌ Build failed."
    exit 1
fi

echo "🦅 Setup complete! Run ./scripts/run_local.sh to start the bot."
