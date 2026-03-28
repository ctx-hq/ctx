#!/bin/sh
# ctx installer — installs the latest ctx binary
# Usage: curl -fsSL https://getctx.org/install.sh | sh
#
# Environment variables:
#   CTX_INSTALL_DIR  — installation directory (default: /usr/local/bin)
#   CTX_VERSION      — specific version to install (default: latest)

set -e

REPO="ctx-hq/ctx"
INSTALL_DIR="${CTX_INSTALL_DIR:-/usr/local/bin}"
BINARY="ctx"

# --- Helpers ----------------------------------------------------------------

die() { echo "Error: $1" >&2; exit 1; }

fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$1"
  else
    die "curl or wget required"
  fi
}

download() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2"
  else
    wget -q "$1" -O "$2"
  fi
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    die "sha256sum or shasum required for checksum verification"
  fi
}

# --- Detect platform --------------------------------------------------------

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) die "Unsupported architecture: $ARCH" ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) die "Unsupported OS: $OS" ;;
esac

PLATFORM="${OS}-${ARCH}"

# --- Resolve version --------------------------------------------------------

if [ -n "$CTX_VERSION" ]; then
  VERSION="${CTX_VERSION#v}"
  echo "-> Installing ctx v${VERSION} for ${PLATFORM}..."
else
  echo "-> Detecting latest version..."
  VERSION=$(fetch "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v?([^"]+)".*/\1/')
  [ -n "$VERSION" ] || die "Could not determine latest version"
  echo "-> Installing ctx v${VERSION} for ${PLATFORM}..."
fi

# --- Download and verify ----------------------------------------------------

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

ARCHIVE_NAME="${BINARY}-${PLATFORM}.tar.gz"
ARCHIVE_URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ARCHIVE_NAME}"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt"

echo "-> Downloading archive..."
download "$ARCHIVE_URL" "$TMPDIR/${ARCHIVE_NAME}"

echo "-> Downloading checksums..."
download "$CHECKSUM_URL" "$TMPDIR/checksums.txt"

echo "-> Verifying SHA256 checksum..."
EXPECTED=$(grep "  ${ARCHIVE_NAME}$" "$TMPDIR/checksums.txt" | awk '{print $1}')
[ -n "$EXPECTED" ] || die "Checksum not found for ${ARCHIVE_NAME} in checksums.txt"

ACTUAL=$(sha256 "$TMPDIR/${ARCHIVE_NAME}")

if [ "$EXPECTED" != "$ACTUAL" ]; then
  die "Checksum mismatch!\n  Expected: ${EXPECTED}\n  Actual:   ${ACTUAL}"
fi
echo "-> Checksum verified"

# --- Extract and install ----------------------------------------------------

echo "-> Extracting..."
tar -xzf "$TMPDIR/${ARCHIVE_NAME}" -C "$TMPDIR"

chmod +x "$TMPDIR/$BINARY"

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
else
  echo "-> Installing to $INSTALL_DIR (requires sudo)..."
  sudo mv "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo ""
echo "ctx v${VERSION} installed to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "  Get started:"
echo "    ctx search \"code review\""
echo "    ctx install @scope/name"
echo "    ctx --help"
