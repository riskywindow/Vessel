#!/bin/bash
# Vessel installation script
# Usage: curl -fsSL https://get.vessel.dev | sh

set -e

REPO="vessel/vessel"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="vessel"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

if [ "$OS" != "linux" ]; then
    echo "Vessel only supports Linux. Detected OS: $OS"
    exit 1
fi

# Get latest release
echo "Fetching latest release..."
LATEST=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST" ]; then
    echo "Failed to fetch latest release"
    exit 1
fi

echo "Installing Vessel ${LATEST}..."

# Download binary
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST}/vessel-${OS}-${ARCH}"
TMP_FILE=$(mktemp)

curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE"
chmod +x "$TMP_FILE"

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
else
    echo "Installing to ${INSTALL_DIR} requires sudo..."
    sudo mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
fi

echo "Vessel installed successfully!"
echo ""
echo "Get started:"
echo "  vessel doctor    # Check system prerequisites"
echo "  vessel init      # Create a vessel.toml"
echo "  vessel deploy    # Deploy your app"
