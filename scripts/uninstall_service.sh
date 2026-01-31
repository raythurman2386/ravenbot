#!/bin/bash

# uninstall_service.sh
# Removes the ravenbot systemd service.

SERVICE_NAME="ravenbot"

echo "🗑️  Uninstalling $SERVICE_NAME service..."

if [ ! -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
    echo "⚠️  Service file not found. Is it installed?"
    exit 0
fi

echo "Stopping service..."
sudo systemctl stop $SERVICE_NAME

echo "Disabling service..."
sudo systemctl disable $SERVICE_NAME

echo "Removing service file..."
sudo rm "/etc/systemd/system/$SERVICE_NAME.service"

echo "Reloading daemon..."
sudo systemctl daemon-reload

echo "✅ Uninstalled successfully."
