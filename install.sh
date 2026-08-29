#!/usr/bin/env bash
# AGIS Installer for Linux and macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/SalvucciFacundo/agis/main/install.sh | bash

set -e

REPO="SalvucciFacundo/agis"
INSTALL_DIR="/usr/local/bin"
ALT_INSTALL_DIR="$HOME/.local/bin"

echo "🚀 Installing AGIS (Autonomous Go Intelligent System)..."

# 1. Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux*)  OS="linux" ;;
  darwin*) OS="darwin" ;;
  *)
    echo "❌ Unsupported operating system: $OS"
    exit 1
    ;;
esac

# 2. Detect Architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "❌ Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

BINARY_NAME="agis-${OS}-${ARCH}"

echo "🔍 Detected Platform: ${OS}/${ARCH}"

# 3. Get latest release tag from GitHub API
LATEST_RELEASE=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || echo "")

if [ -z "$LATEST_RELEASE" ]; then
  DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}"
else
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_RELEASE}/${BINARY_NAME}"
  echo "📦 Downloading AGIS release ${LATEST_RELEASE}..."
fi

TMP_DIR=$(mktemp -d)
TMP_FILE="${TMP_DIR}/agis"

if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE"; then
  echo "⚠️ Prebuilt release binary download failed. Building from source if Go is available..."
  if command -v go >/dev/null 2>&1; then
    go install "github.com/${REPO}/cmd/agis@latest"
    echo "✅ AGIS installed successfully via 'go install'!"
    rm -rf "$TMP_DIR"
    exit 0
  else
    echo "❌ Download failed and Go compiler not found. Please check network connection or release page."
    rm -rf "$TMP_DIR"
    exit 1
  fi
fi

chmod +x "$TMP_FILE"

# 4. Install binary to PATH
TARGET_DIR="$INSTALL_DIR"
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_FILE" "${INSTALL_DIR}/agis"
elif sudo -n true 2>/dev/null; then
  sudo mv "$TMP_FILE" "${INSTALL_DIR}/agis"
else
  mkdir -p "$ALT_INSTALL_DIR"
  mv "$TMP_FILE" "${ALT_INSTALL_DIR}/agis"
  TARGET_DIR="$ALT_INSTALL_DIR"
fi

rm -rf "$TMP_DIR"

echo "✅ AGIS installed successfully at ${TARGET_DIR}/agis!"
echo ""
echo "Type 'agis' in your terminal to start."
