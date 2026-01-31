#!/bin/bash

# install_service.sh
# Installs ravenbot as a systemd service for 24/7 background operation.
# Requires sudo privileges.

set -e

SERVICE_NAME="ravenbot"
USER_NAME=$(whoami)
GROUP_NAME=$(id -gn)
WORK_DIR=$(pwd)
EXEC_PATH="$WORK_DIR/ravenbot"
ENV_FILE="$WORK_DIR/.env"
SYSTEMD_PATH="/etc/systemd/system/$SERVICE_NAME.service"

echo "🦅 Installing $SERVICE_NAME as a systemd service..."

# 1. Checks
if [ ! -f "$EXEC_PATH" ]; then
    echo "❌ Binary not found at $EXEC_PATH"
    echo "   Please run 'make build' or './scripts/setup_local.sh' first."
    exit 1
fi

if [ ! -f "$ENV_FILE" ]; then
    echo "❌ .env file not found at $ENV_FILE"
    exit 1
fi

# Detect Chrome for environment variable if not in .env
CHROME_PATH=""
if ! grep -q "CHROME_BIN" "$ENV_FILE"; then
    if command -v chromium &> /dev/null; then
        CHROME_PATH=$(command -v chromium)
    elif command -v google-chrome &> /dev/null; then
        CHROME_PATH=$(command -v google-chrome)
    elif command -v /usr/bin/chromium-browser &> /dev/null; then
        CHROME_PATH="/usr/bin/chromium-browser"
    fi
fi

# 2. Generate Service File
echo "📝 Generating service file..."

cat <<EOF > ${SERVICE_NAME}.service.tmp
[Unit]
Description=ravenbot Autonomous Agent
After=network.target

[Service]
Type=simple
User=$USER_NAME
Group=$GROUP_NAME
WorkingDirectory=$WORK_DIR
ExecStart=$EXEC_PATH
Restart=always
RestartSec=5
EnvironmentFile=$ENV_FILE
# Explicitly set CHROME_BIN if we detected it and it wasn't in .env
${CHROME_PATH:+Environment="CHROME_BIN=$CHROME_PATH"}

[Install]
WantedBy=multi-user.target
EOF

# 3. Install Service
echo "sudo required to copy service file to /etc/systemd/system/"
sudo mv ${SERVICE_NAME}.service.tmp $SYSTEMD_PATH
sudo chown root:root $SYSTEMD_PATH
sudo chmod 644 $SYSTEMD_PATH

# 4. Enable and Start
echo "🚀 Enabling and starting service..."
sudo systemctl daemon-reload
sudo systemctl enable $SERVICE_NAME
sudo systemctl start $SERVICE_NAME

echo "✅ ravenbot is now running in the background!"
echo "   Status:  sudo systemctl status $SERVICE_NAME"
echo "   Logs:    sudo journalctl -u $SERVICE_NAME -f"
echo "   Stop:    sudo systemctl stop $SERVICE_NAME"
