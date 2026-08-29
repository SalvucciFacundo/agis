#!/usr/bin/env bash
set -e

# ==============================================================================
#  ⚡ AGIS Installer — Autonomous Go Intelligent System
# ==============================================================================

REPO="SalvucciFacundo/agis"
GITHUB_URL="https://github.com/${REPO}"

# ANSI color codes
CYAN='\033[0;36m'
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m' # No Color

echo -e "${CYAN}${BOLD}"
echo "  ⚡ ===================================================== ⚡"
echo "      AGIS — Autonomous Go Intelligent System"
echo "  ⚡ ===================================================== ⚡"
echo -e "${NC}"

# 1. Detect Operating System
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "${OS}" in
  linux*)   OS="linux" ;;
  darwin*)  OS="darwin" ;;
  freebsd*) OS="freebsd" ;;
  *)
    echo -e "${RED}❌ Unsupported operating system: ${OS}${NC}"
    echo "AGIS installer supports Linux, macOS, and FreeBSD. For Windows, use install.ps1."
    exit 1
    ;;
esac

# 2. Detect Machine Architecture
RAW_ARCH="$(uname -m | tr '[:upper:]' '[:lower:]')"
case "${RAW_ARCH}" in
  x86_64|amd64)        ARCH="amd64" ;;
  aarch64|arm64)       ARCH="arm64" ;;
  armv7*|armv8l|armhf) ARCH="armv7" ;;
  i386|i686)           ARCH="386" ;;
  *)
    echo -e "${RED}❌ Unsupported architecture: ${RAW_ARCH}${NC}"
    exit 1
    ;;
esac

echo -e "🖥️  Detected Platform: ${GREEN}${OS}/${ARCH}${NC}"

# 3. Determine Installation Directory
if [ -w "/usr/local/bin" ]; then
  INSTALL_DIR="/usr/local/bin"
elif [ -n "${SUDO_USER}" ] && [ -w "/usr/local/bin" ]; then
  INSTALL_DIR="/usr/local/bin"
else
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "${INSTALL_DIR}"
fi

# 4. Fetch Latest Version Tag from GitHub
echo -e "🔍 Checking latest version from GitHub..."
TARGET_VERSION="${VERSION:-}"

if [ -z "$TARGET_VERSION" ]; then
  if command -v curl >/dev/null 2>&1; then
    TARGET_VERSION=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || true)
  elif command -v wget >/dev/null 2>&1; then
    TARGET_VERSION=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || true)
  fi
fi

if [ -z "$TARGET_VERSION" ]; then
  TARGET_VERSION="v1.0.0"
  echo -e "${YELLOW}⚠️  Could not fetch release tag from API, using default ${TARGET_VERSION}${NC}"
else
  echo -e "📦 Target Release: ${GREEN}${TARGET_VERSION}${NC}"
fi

CLEAN_VER="${TARGET_VERSION#v}"
TARBALL="agis_${CLEAN_VER}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="${GITHUB_URL}/releases/download/${TARGET_VERSION}/${TARBALL}"

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

echo -e "⬇️  Downloading ${CYAN}${TARBALL}${NC}..."
DOWNLOAD_SUCCESS=0

if command -v curl >/dev/null 2>&1; then
  if curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DIR}/${TARBALL}" 2>/dev/null; then
    DOWNLOAD_SUCCESS=1
  fi
elif command -v wget >/dev/null 2>&1; then
  if wget -q "${DOWNLOAD_URL}" -O "${TMP_DIR}/${TARBALL}" 2>/dev/null; then
    DOWNLOAD_SUCCESS=1
  fi
fi

# 5. Extract and Install
if [ "$DOWNLOAD_SUCCESS" -eq 1 ]; then
  echo -e "📦 Extracting archive..."
  tar -xzf "${TMP_DIR}/${TARBALL}" -C "${TMP_DIR}"
  chmod +x "${TMP_DIR}/agis"

  echo -e "🚚 Installing binary to ${CYAN}${INSTALL_DIR}/agis${NC}..."
  if [ -w "${INSTALL_DIR}" ]; then
    mv "${TMP_DIR}/agis" "${INSTALL_DIR}/agis"
  elif command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
    sudo mv "${TMP_DIR}/agis" "${INSTALL_DIR}/agis"
  else
    echo -e "${YELLOW}⚠️  Writing to ${HOME}/.local/bin (no root permissions)${NC}"
    mkdir -p "${HOME}/.local/bin"
    mv "${TMP_DIR}/agis" "${HOME}/.local/bin/agis"
    INSTALL_DIR="${HOME}/.local/bin"
  fi
else
  echo -e "${YELLOW}⚠️  Prebuilt release binary download failed.${NC}"
  if command -v go >/dev/null 2>&1; then
    echo -e "🔨 Building and installing from source via 'go install'..."
    if go install "github.com/${REPO}/cmd/agis@latest"; then
      echo -e "${GREEN}✅ AGIS installed successfully via 'go install'!${NC}"
      GOBIN="$(go env GOBIN)"
      if [ -z "$GOBIN" ]; then
        GOBIN="$(go env GOPATH)/bin"
      fi
      echo "Binary location: ${GOBIN}/agis"
      exit 0
    fi
  fi
  echo -e "${RED}❌ Installation failed. Please check network connection or Go installation.${NC}"
  exit 1
fi

# 6. Verify and Path Hint
echo -e "${GREEN}${BOLD}✅ AGIS ${TARGET_VERSION} installed successfully at ${INSTALL_DIR}/agis!${NC}"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo -e "${YELLOW}⚠️  Notice: ${INSTALL_DIR} is not in your current PATH.${NC}"
    echo "Add the following line to your ~/.bashrc or ~/.zshrc:"
    echo -e "  ${CYAN}export PATH=\"\$PATH:${INSTALL_DIR}\"${NC}"
    ;;
esac

echo ""
echo -e "${CYAN}🚀 Run 'agis' to launch the interactive terminal user interface!${NC}"
