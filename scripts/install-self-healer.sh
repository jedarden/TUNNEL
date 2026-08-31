#!/bin/bash
# Installation script for TUNNEL self-healer systemd service
# This script installs the systemd service and timer for proactive bead maintenance

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
CONFIG_DIR="$PROJECT_DIR/configs/systemd"
USER="$(whoami)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "=== TUNNEL Self-Healer Installation ==="
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
        echo "The healer will use the current working directory during execution"
    fi
fi

echo -e "${GREEN}Using workspace directory: $WORKSPACE_DIR${NC}"
echo

# Create systemd user directory if it doesn't exist
SYSTEMD_USER_DIR="$HOME/.config/systemd/user"
mkdir -p "$SYSTEMD_USER_DIR"

# Installation mode selection
echo "Select installation mode:"
echo "  1) Timer mode (recommended) - Runs every 5 minutes via systemd timer"
echo "  2) Daemon mode - Runs continuously as a background service"
echo "  3) Both - Install timer and daemon"
echo -n "Choice [1-3]: "
read -r CHOICE

case $CHOICE in
    1)
        MODE="timer"
        ;;
    2)
        MODE="daemon"
        ;;
    3)
        MODE="both"
        ;;
    *)
        echo -e "${RED}Invalid choice. Defaulting to timer mode${NC}"
        MODE="timer"
        ;;
esac

echo
echo "Installing in $MODE mode..."
echo

# Install timer mode
if [ "$MODE" = "timer" ] || [ "$MODE" = "both" ]; then
    echo "Installing timer mode..."

    # Install service file
    sed "s|/usr/local/bin/tunnel|$TUNNEL_BIN|g" "$CONFIG_DIR/tunnel-self-healer@.service" \
        | sed "s|/home/coding|$HOME|g" \
        > "$SYSTEMD_USER_DIR/tunnel-self-healer@.service"

    # Install timer file
    cp "$CONFIG_DIR/tunnel-self-healer.timer" "$SYSTEMD_USER_DIR/"

    # Reload systemd daemon
    systemctl --user daemon-reload

    # Enable and start timer
    systemctl --user enable tunnel-self-healer.timer || {
        echo -e "${YELLOW}Warning: Failed to enable timer. You may need to log out and back in for user systemd services to work.${NC}"
        echo "To enable manually after logging back in:"
        echo "  systemctl --user enable --now tunnel-self-healer.timer"
    }

    # Start timer if not already running
    if systemctl --user is-active --quiet tunnel-self-healer.timer; then
        echo -e "${GREEN}Timer is already running${NC}"
    else
        echo "Starting timer..."
        systemctl --user start tunnel-self-healer.timer
    fi

    echo -e "${GREEN}Timer mode installed successfully${NC}"
fi

# Install daemon mode
if [ "$MODE" = "daemon" ] || [ "$MODE" = "both" ]; then
    echo "Installing daemon mode..."

    # Install service file
    sed "s|/usr/local/bin/tunnel|$TUNNEL_BIN|g" "$CONFIG_DIR/tunnel-self-healer.service" \
        | sed "s|/home/coding/TUNNEL|$WORKSPACE_DIR|g" \
        > "$SYSTEMD_USER_DIR/tunnel-self-healer.service"

    # Reload systemd daemon
    systemctl --user daemon-reload

    # Enable and start service
    systemctl --user enable tunnel-self-healer.service || {
        echo -e "${YELLOW}Warning: Failed to enable service. You may need to log out and back in for user systemd services to work.${NC}"
        echo "To enable manually after logging back in:"
        echo "  systemctl --user enable --now tunnel-self-healer.service"
    }

    # Start service if not already running
    if systemctl --user is-active --quiet tunnel-self-healer.service; then
        echo -e "${GREEN}Daemon is already running${NC}"
    else
        echo "Starting daemon..."
        systemctl --user start tunnel-self-healer.service
    fi

    echo -e "${GREEN}Daemon mode installed successfully${NC}"
fi

echo
echo -e "${GREEN}=== Installation Complete ===${NC}"
echo

if [ "$MODE" = "timer" ] || [ "$MODE" = "both" ]; then
    echo "${BLUE}Timer Mode:${NC}"
    echo "  The self-healer will run every 5 minutes automatically."
    echo
    echo "  # Check timer status"
    echo "  systemctl --user status tunnel-self-healer.timer"
    echo
    echo "  # View healing logs"
    echo "  journalctl --user -u tunnel-self-healer@*"
    echo
    echo "  # Run healing manually"
    echo "  tunnel self-healer --scheduled"
    echo
fi

if [ "$MODE" = "daemon" ] || [ "$MODE" = "both" ]; then
    echo "${BLUE}Daemon Mode:${NC}"
    echo "  The self-healer is running continuously in the background."
    echo
    echo "  # Check service status"
    echo "  systemctl --user status tunnel-self-healer.service"
    echo
    echo "  # View service logs"
    echo "  journalctl --user -u tunnel-self-healer.service -f"
    echo
    echo "  # Restart service"
    echo "  systemctl --user restart tunnel-self-healer.service"
    echo
fi

echo "${BLUE}Manual Commands:${NC}"
echo "  # Run single healing check"
echo "  tunnel self-healer"
echo
echo "  # Run in dry-run mode (no changes applied)"
echo "  tunnel self-healer --dry-run"
echo
echo "  # Run with auto-repair disabled"
echo "  tunnel self-healer --auto-repair=false"
echo
echo "  # Run in daemon mode manually"
echo "  tunnel self-healer --daemon"
echo
echo "  # Customize thresholds"
echo "  tunnel self-healer --stale-threshold 12h --long-running-threshold 14d"
echo
echo "${YELLOW}To uninstall:${NC}"
if [ "$MODE" = "timer" ] || [ "$MODE" = "both" ]; then
    echo "  systemctl --user disable --now tunnel-self-healer.timer"
    echo "  rm $SYSTEMD_USER_DIR/tunnel-self-healer@.service"
    echo "  rm $SYSTEMD_USER_DIR/tunnel-self-healer.timer"
fi
if [ "$MODE" = "daemon" ] || [ "$MODE" = "both" ]; then
    echo "  systemctl --user disable --now tunnel-self-healer.service"
    echo "  rm $SYSTEMD_USER_DIR/tunnel-self-healer.service"
fi
echo "  systemctl --user daemon-reload"
echo
