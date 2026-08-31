#!/bin/bash
# Installation script for TUNNEL bead validator systemd timer
# This script installs the systemd service and timer for automatic hourly validation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
CONFIG_DIR="$PROJECT_DIR/configs/systemd"
USER="$(whoami)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=== TUNNEL Bead Validator Installation ==="
echo

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    echo -e "${RED}Error: This script should not be run as root${NC}"
    echo "The systemd user service runs without root privileges"
    exit 1
fi

# Check if systemd is available
if ! command -v systemctl &> /dev/null; then
    echo -e "${RED}Error: systemctl not found. Is systemd available?${NC}"
    exit 1
fi

# Check if tunnel binary exists
TUNNEL_BIN="/usr/local/bin/tunnel"
if [ ! -f "$TUNNEL_BIN" ]; then
    TUNNEL_BIN="$HOME/.local/bin/tunnel"
    if [ ! -f "$TUNNEL_BIN" ]; then
        echo -e "${YELLOW}Warning: tunnel binary not found in standard locations${NC}"
        echo "Expected: /usr/local/bin/tunnel or ~/.local/bin/tunnel"
        echo -n "Enter full path to tunnel binary (or press Enter to skip): "
        read -r CUSTOM_BIN
        if [ -n "$CUSTOM_BIN" ] && [ -f "$CUSTOM_BIN" ]; then
            TUNNEL_BIN="$CUSTOM_BIN"
        else
            echo -e "${RED}Error: No valid tunnel binary found${NC}"
            echo "Please build and install tunnel first: cd $PROJECT_DIR && make install"
            exit 1
        fi
    fi
fi

echo -e "${GREEN}Found tunnel binary at: $TUNNEL_BIN${NC}"

# Check if workspace has .beads directory
WORKSPACE_DIR="$PROJECT_DIR"
if [ ! -d "$WORKSPACE_DIR/.beads" ]; then
    # Try to find workspace root
    WORKSPACE_DIR="$(git rev-parse --show-toplevel 2>/dev/null || echo "$PROJECT_DIR")"
    if [ ! -d "$WORKSPACE_DIR/.beads" ]; then
        echo -e "${YELLOW}Warning: No .beads directory found in $WORKSPACE_DIR${NC}"
        echo "The validator will use the current working directory during execution"
    fi
fi

echo -e "${GREEN}Using workspace directory: $WORKSPACE_DIR${NC}"
echo

# Create systemd user directory if it doesn't exist
SYSTEMD_USER_DIR="$HOME/.config/systemd/user"
mkdir -p "$SYSTEMD_USER_DIR"

# Install service file
echo "Installing systemd service..."
sed "s|/usr/local/bin/tunnel|$TUNNEL_BIN|g" "$CONFIG_DIR/tunnel-bead-validator.service" \
    | sed "s|/home/%i|$HOME|g" \
    > "$SYSTEMD_USER_DIR/tunnel-bead-validator@.service"

# Install timer file
echo "Installing systemd timer..."
cp "$CONFIG_DIR/tunnel-bead-validator.timer" "$SYSTEMD_USER_DIR/"

# Reload systemd daemon
echo "Reloading systemd daemon..."
systemctl --user daemon-reload

# Enable and start timer
echo "Enabling timer..."
systemctl --user enable tunnel-bead-validator.timer || {
    echo -e "${YELLOW}Warning: Failed to enable timer. You may need to log out and back in for user systemd services to work.${NC}"
    echo "To enable manually after logging back in:"
    echo "  systemctl --user enable --now tunnel-bead-validator.timer"
}

# Start timer if not already running
if systemctl --user is-active --quiet tunnel-bead-validator.timer; then
    echo -e "${GREEN}Timer is already running${NC}"
else
    echo "Starting timer..."
    systemctl --user start tunnel-bead-validator.timer
fi

echo
echo -e "${GREEN}=== Installation Complete ===${NC}"
echo
echo "The bead validator will run hourly automatically."
echo
echo "Manual commands:"
echo "  # Check timer status"
echo "  systemctl --user status tunnel-bead-validator.timer"
echo
echo "  # View validation logs"
echo "  journalctl --user -u tunnel-bead-validator@${USER}"
echo
echo "  # Run validation manually"
echo "  tunnel bead-validator --fix"
echo
echo "  # Run validation in dry-run mode"
echo "  tunnel bead-validator --dry-run"
echo
echo "  # Run scheduled validation manually"
echo "  tunnel bead-validator --scheduled"
echo
echo "To uninstall:"
echo "  systemctl --user disable --now tunnel-bead-validator.timer"
echo "  rm $SYSTEMD_USER_DIR/tunnel-bead-validator@.service"
echo "  rm $SYSTEMD_USER_DIR/tunnel-bead-validator.timer"
echo "  systemctl --user daemon-reload"
