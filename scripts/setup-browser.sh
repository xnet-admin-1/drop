#!/bin/bash
# Setup script to install Chromium for the drop browser component
# Usage: ./scripts/setup-browser.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
CHROMIUM_DIR="$PROJECT_DIR/.chromium"

echo "Setting up browser dependencies..."

# Check for working system browser
check_system_chromium() {
    for cmd in google-chrome chromium-browser chromium chrome; do
        if command -v "$cmd" &> /dev/null; then
            local version=$($cmd --version 2>/dev/null || echo "unknown")
            echo "Found: $cmd ($version)"
            return 0
        fi
    done
    return 1
}

# Check local extraction
check_local_chromium() {
    local chrome="$CHROMIUM_DIR/chrome-linux/chrome"
    if [ -f "$chrome" ]; then
        # Quick test - check for critical missing libs
        if ldd "$chrome" 2>&1 | grep -q "libgtk-3\|libgobject"; then
            # Likely has critical deps - let's try a test run
            if timeout 3 "$chrome" --headless --no-sandbox --disable-gpu --dump-dom about:blank >/dev/null 2>&1; then
                echo "Found working local Chromium: $chrome"
                return 0
            fi
        fi
        echo "Local Chromium has missing dependencies"
    fi
    return 1
}

# Try using snap (if available)
setup_snap_chromium() {
    if command -v snap &> /dev/null && command -v sudo &> /dev/null; then
        echo "Snap available, you could run:"
        echo "  sudo snap install chromium"
    fi
}

# Main
if check_system_chromium; then
    echo "Using system browser"
    exit 0
fi

if check_local_chromium; then
    echo "Using local browser"
    exit 0
fi

echo "No working Chromium found."
echo ""
echo "OPTIONS TO SET UP CHROMIUM:"
echo ""
echo "1. Install via apt (recommended for Ubuntu/Debian):"
echo "   sudo apt update && sudo apt install chromium"
echo ""
echo "2. Install Chrome:"
echo "   wget -q -O /tmp/chrome.deb \\"
echo "     https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb"
echo "   sudo dpkg -i /tmp/chrome.deb; sudo apt-get -f install"
echo ""
echo "3. Use Snap (if snapd is installed):"
echo "   sudo snap install chromium"
echo ""
echo "4. After installation, verify with:"
echo "   chromium --version"
echo ""
echo "The drop server will automatically find your browser."
echo ""

# If we have root, try apt directly
if [ "$EUID" = "0" ]; then
    echo "Running as root, attempting apt install..."
    apt-get update -qq 2>/dev/null
    apt-get install -y chromium 2>/dev/null && {
        echo "Chromium installed!"
        check_system_chromium
        exit 0
    }
fi

exit 1