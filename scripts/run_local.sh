#!/bin/bash

# run_local.sh
# Runs the ravenbot locally.

set -e

# Load .env if it exists
if [ -f .env ]; then
    export $(cat .env | xargs)
fi

# Check if binary exists
if [ ! -f ./ravenbot ]; then
    echo "❌ ravenbot binary not found. Running setup..."
    ./scripts/setup_local.sh
fi

# Try to detect Chrome if not set
if [ -z "$CHROME_BIN" ]; then
    if command -v chromium &> /dev/null; then
        export CHROME_BIN=$(command -v chromium)
    elif command -v google-chrome &> /dev/null; then
        export CHROME_BIN=$(command -v google-chrome)
    elif command -v /usr/bin/chromium-browser &> /dev/null; then # Common on Alpine/some Linux
        export CHROME_BIN=/usr/bin/chromium-browser
    fi
    
    if [ ! -z "$CHROME_BIN" ]; then
        echo "✅ Auto-detected Chrome at: $CHROME_BIN"
    else
        echo "⚠️  Could not auto-detect Chrome. Please set CHROME_BIN in .env if using browser tools."
    fi
fi

echo "🦅 Starting ravenbot..."
./ravenbot
