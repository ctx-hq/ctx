#!/bin/sh
# ctx installer — installs the latest ctx binary
# Usage: curl -fsSL https://getctx.org/install.sh | sh
#
# Environment variables:
#   CTX_INSTALL_DIR  — override installation directory
#   CTX_VERSION      — specific version to install (default: latest)

set -e

REPO="ctx-hq/ctx"
BINARY="ctx"

# --- Helpers ----------------------------------------------------------------

die() { echo "Error: $1" >&2; exit 1; }

info() { printf "\033[0;36m>\033[0m %s\n" "$1"; }

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

# --- Resolve install directory ----------------------------------------------

resolve_install_dir() {
  # User explicitly set — use as-is
  if [ -n "$CTX_INSTALL_DIR" ]; then
    echo "$CTX_INSTALL_DIR"
    return
  fi

  # Prefer ~/.local/bin (XDG standard, widely supported)
  if [ -d "$HOME/.local/bin" ] && echo "$PATH" | grep -q "$HOME/.local/bin"; then
    echo "$HOME/.local/bin"
    return
  fi

  # /usr/local/bin writable without sudo (e.g. Homebrew on macOS)
  if [ -w "/usr/local/bin" ]; then
    echo "/usr/local/bin"
    return
  fi

  # Fall back to ~/.local/bin (create it)
  echo "$HOME/.local/bin"
}

INSTALL_DIR=$(resolve_install_dir)

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
  *) die "Unsupported OS: $OS (use install.ps1 for Windows)" ;;
esac

PLATFORM="${OS}-${ARCH}"

# --- Resolve version --------------------------------------------------------

if [ -n "$CTX_VERSION" ]; then
  VERSION="${CTX_VERSION#v}"
else
  info "Detecting latest version..."
  VERSION=$(fetch "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v?([^"]+)".*/\1/')
  [ -n "$VERSION" ] || die "Could not determine latest version. Set CTX_VERSION=0.1.0 to install manually."
fi

info "Installing ctx v${VERSION} (${PLATFORM})..."

# --- Download and verify ----------------------------------------------------

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

ARCHIVE_NAME="${BINARY}-${PLATFORM}.tar.gz"
ARCHIVE_URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ARCHIVE_NAME}"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt"

info "Downloading..."
download "$ARCHIVE_URL" "$TMPDIR/${ARCHIVE_NAME}"
download "$CHECKSUM_URL" "$TMPDIR/checksums.txt"

info "Verifying checksum..."
EXPECTED=$(grep "  ${ARCHIVE_NAME}$" "$TMPDIR/checksums.txt" | awk '{print $1}')
[ -n "$EXPECTED" ] || die "Checksum not found for ${ARCHIVE_NAME}"

ACTUAL=$(sha256 "$TMPDIR/${ARCHIVE_NAME}")
if [ "$EXPECTED" != "$ACTUAL" ]; then
  die "Checksum mismatch!\n  Expected: ${EXPECTED}\n  Actual:   ${ACTUAL}"
fi

# --- Extract and install ----------------------------------------------------

tar -xzf "$TMPDIR/${ARCHIVE_NAME}" -C "$TMPDIR"
chmod +x "$TMPDIR/$BINARY"

# Ensure install directory exists
if [ ! -d "$INSTALL_DIR" ]; then
  mkdir -p "$INSTALL_DIR" 2>/dev/null || sudo mkdir -p "$INSTALL_DIR"
fi

# Install binary
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
else
  info "Writing to $INSTALL_DIR requires sudo..."
  sudo mv "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
fi

# --- Post-install -----------------------------------------------------------

echo ""
echo "  \033[1;32mctx v${VERSION}\033[0m installed to ${INSTALL_DIR}/${BINARY}"
echo ""

# Check if install dir is in PATH
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo "  \033[1;33mNote:\033[0m ${INSTALL_DIR} is not in your PATH."
    SHELL_NAME=$(basename "${SHELL:-/bin/sh}")
    case "$SHELL_NAME" in
      zsh)  RC="~/.zshrc" ;;
      bash) RC="~/.bashrc" ;;
      fish) RC="~/.config/fish/config.fish" ;;
      *)    RC="your shell config" ;;
    esac
    echo "  Add it by running:"
    echo ""
    if [ "$SHELL_NAME" = "fish" ]; then
      echo "    fish_add_path ${INSTALL_DIR}"
    else
      echo "    echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ${RC}"
    fi
    echo ""
    ;;
esac

echo "  Get started:"
echo "    ctx search \"code review\""
echo "    ctx install @scope/name"
echo "    ctx --help"
