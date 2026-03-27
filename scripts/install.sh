#!/bin/sh
# ctx installer — installs the latest ctx binary
# Usage: curl -fsSL https://getctx.org/install.sh | sh

set -e

REPO="getctx/ctx"
INSTALL_DIR="${CTX_INSTALL_DIR:-/usr/local/bin}"
BINARY="ctx"

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

PLATFORM="${OS}-${ARCH}"

# Get latest release
echo "→ Detecting latest version..."
if command -v curl >/dev/null 2>&1; then
  LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v?([^"]+)".*/\1/')
elif command -v wget >/dev/null 2>&1; then
  LATEST=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v?([^"]+)".*/\1/')
else
  echo "Error: curl or wget required"
  exit 1
fi

if [ -z "$LATEST" ]; then
  echo "Error: could not determine latest version"
  exit 1
fi

URL="https://github.com/${REPO}/releases/download/v${LATEST}/${BINARY}-${PLATFORM}"
echo "→ Downloading ctx v${LATEST} for ${PLATFORM}..."

TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$URL" -o "$TMPDIR/$BINARY"
else
  wget -q "$URL" -O "$TMPDIR/$BINARY"
fi

chmod +x "$TMPDIR/$BINARY"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
else
  echo "→ Installing to $INSTALL_DIR (requires sudo)..."
  sudo mv "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "✓ ctx v${LATEST} installed to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "  Get started:"
echo "    ctx search \"code review\""
echo "    ctx install @scope/name"
echo "    ctx --help"
