#!/usr/bin/env bash
# AGIS Installer for Linux and macOS (Multi-Distro Support)
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
  freebsd*) OS="freebsd" ;;
  *)
    echo "❌ Unsupported operating system: $OS"
    exit 1
    ;;
esac

# 2. Detect Architecture
RAW_ARCH="$(uname -m)"
case "$RAW_ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  armv7*|armv8l|armhf) ARCH="armv7" ;;
  i386|i686) ARCH="386" ;;
  *)
    echo "❌ Unsupported architecture: $RAW_ARCH"
    exit 1
    ;;
esac

echo "🔍 Detected Platform: ${OS}/${ARCH}"

# 3. Determine release version and download URL
TARGET_VERSION="${VERSION:-}"
if [ -z "$TARGET_VERSION" ]; then
  LATEST_RELEASE=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || echo "")
  TARGET_VERSION="$LATEST_RELEASE"
fi

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

SUCCESS=0

if [ -n "$TARGET_VERSION" ]; then
  echo "📦 Attempting download for release: ${TARGET_VERSION}..."

  # Try GoReleaser tar.gz archive pattern: agis_<VERSION>_<OS>_<ARCH>.tar.gz
  CLEAN_VER="${TARGET_VERSION#v}"
  ARCHIVE_NAME="agis_${CLEAN_VER}_${OS}_${ARCH}.tar.gz"
  TAR_URL="https://github.com/${REPO}/releases/download/${TARGET_VERSION}/${ARCHIVE_NAME}"

  # Also try standalone binary pattern: agis-<OS>-<ARCH>
  BIN_URL="https://github.com/${REPO}/releases/download/${TARGET_VERSION}/agis-${OS}-${ARCH}"

  if curl -fsSL "$TAR_URL" -o "${TMP_DIR}/archive.tar.gz" 2>/dev/null; then
    tar -xzf "${TMP_DIR}/archive.tar.gz" -C "$TMP_DIR"
    if [ -f "${TMP_DIR}/agis" ]; then
      SUCCESS=1
    fi
  elif curl -fsSL "$BIN_URL" -o "${TMP_DIR}/agis" 2>/dev/null; then
    SUCCESS=1
  fi
fi

if [ "$SUCCESS" -eq 0 ]; then
  echo "⚠️ Prebuilt release binary download not found or failed."
  if command -v go >/dev/null 2>&1; then
    echo "🔨 Building and installing from source via 'go install github.com/${REPO}/cmd/agis@latest'..."
    if go install "github.com/${REPO}/cmd/agis@latest"; then
      echo "✅ AGIS installed successfully via 'go install'!"
      GOBIN="$(go env GOBIN)"
      if [ -z "$GOBIN" ]; then
        GOBIN="$(go env GOPATH)/bin"
      fi
      echo "Binary location: ${GOBIN}/agis"
      exit 0
    fi
  fi
  echo "❌ Could not download or build AGIS. Please check your connection or Go installation."
  exit 1
fi

chmod +x "${TMP_DIR}/agis"

# 4. Install binary into PATH
TARGET_DIR="$INSTALL_DIR"
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMP_DIR}/agis" "${INSTALL_DIR}/agis"
elif command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
  sudo mv "${TMP_DIR}/agis" "${INSTALL_DIR}/agis"
else
  mkdir -p "$ALT_INSTALL_DIR"
  mv "${TMP_DIR}/agis" "${ALT_INSTALL_DIR}/agis"
  TARGET_DIR="$ALT_INSTALL_DIR"
fi

echo "✅ AGIS installed successfully at ${TARGET_DIR}/agis!"

# Check if target dir is in user PATH
case ":$PATH:" in
  *":$TARGET_DIR:"*) ;;
  *)
    echo ""
    echo "⚠️ Note: '${TARGET_DIR}' is not currently in your PATH."
    echo "Add it to your shell configuration file (~/.bashrc or ~/.zshrc):"
    echo "  export PATH=\"\$PATH:${TARGET_DIR}\""
    ;;
esac

echo ""
echo "🚀 Type 'agis' in your terminal to start!"
