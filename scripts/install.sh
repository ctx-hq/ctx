#!/bin/sh
# ctx installer — installs the latest ctx binary
# Usage: curl -fsSL https://getctx.org/install.sh | sh
#
# Environment variables:
#   CTX_INSTALL_DIR  — override installation directory
#   CTX_VERSION      — specific version to install (default: latest)
#   CTX_QUIET        — suppress output (set to 1)

set -e

REPO="ctx-hq/ctx"
BINARY="ctx"

# --- Logging ----------------------------------------------------------------

if [ "${CTX_QUIET:-0}" = "1" ]; then
  info() { :; }
else
  info() { printf "\033[0;36m>\033[0m %s\n" "$1" >&2; }
fi
die() { printf "\033[0;31merror:\033[0m %s\n" "$1" >&2; exit 1; }
ok()  { printf "\033[0;32m>\033[0m %s\n" "$1" >&2; }

# --- Helpers ----------------------------------------------------------------

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
    die "sha256sum or shasum required"
  fi
}

# --- Resolve install directory ----------------------------------------------

resolve_install_dir() {
  # User explicitly set
  if [ -n "${CTX_INSTALL_DIR:-}" ]; then
    echo "$CTX_INSTALL_DIR"
    return
  fi

  # ~/.local/bin already in PATH — use it
  if echo ":$PATH:" | grep -q ":$HOME/.local/bin:"; then
    echo "$HOME/.local/bin"
    return
  fi

  # /usr/local/bin writable without sudo (e.g. Homebrew on macOS)
  if [ -w "/usr/local/bin" ]; then
    echo "/usr/local/bin"
    return
  fi

  # Default: ~/.local/bin (will auto-configure PATH)
  echo "$HOME/.local/bin"
}

INSTALL_DIR=$(resolve_install_dir)

# --- Detect platform --------------------------------------------------------

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) die "unsupported architecture: $ARCH" ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) die "unsupported OS: $OS (use install.ps1 for Windows)" ;;
esac

PLATFORM="${OS}-${ARCH}"

# --- Resolve version --------------------------------------------------------

if [ -n "${CTX_VERSION:-}" ]; then
  VERSION="${CTX_VERSION#v}"
else
  info "detecting latest version..."
  VERSION=$(fetch "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v?([^"]+)".*/\1/')
  [ -n "$VERSION" ] || die "could not detect latest version — set CTX_VERSION=0.1.0 to install manually"
fi

info "installing ctx v${VERSION} (${PLATFORM})"

# --- Download and verify ----------------------------------------------------

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

ARCHIVE_NAME="${BINARY}-${PLATFORM}.tar.gz"
ARCHIVE_URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ARCHIVE_NAME}"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt"

download "$ARCHIVE_URL" "$TMPDIR/${ARCHIVE_NAME}"
download "$CHECKSUM_URL" "$TMPDIR/checksums.txt"

EXPECTED=$(grep "  ${ARCHIVE_NAME}$" "$TMPDIR/checksums.txt" | awk '{print $1}')
[ -n "$EXPECTED" ] || die "checksum not found for ${ARCHIVE_NAME}"

ACTUAL=$(sha256 "$TMPDIR/${ARCHIVE_NAME}")
if [ "$EXPECTED" != "$ACTUAL" ]; then
  die "checksum mismatch (expected ${EXPECTED}, got ${ACTUAL})"
fi

# --- Extract and install ----------------------------------------------------

tar -xzf "$TMPDIR/${ARCHIVE_NAME}" -C "$TMPDIR"
chmod +x "$TMPDIR/$BINARY"

# Ensure install directory exists
if [ ! -d "$INSTALL_DIR" ]; then
  mkdir -p "$INSTALL_DIR" 2>/dev/null || {
    info "creating $INSTALL_DIR (requires sudo)"
    sudo mkdir -p "$INSTALL_DIR"
  }
fi

# Install binary
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
else
  info "writing to $INSTALL_DIR (requires sudo)"
  sudo mv "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
fi

# --- Auto-configure PATH ---------------------------------------------------

ensure_path() {
  # Already in PATH — nothing to do
  case ":$PATH:" in
    *":${INSTALL_DIR}:"*) return 0 ;;
  esac

  SHELL_NAME=$(basename "${SHELL:-/bin/sh}")
  LINE="export PATH=\"${INSTALL_DIR}:\$PATH\""

  case "$SHELL_NAME" in
    zsh)
      RC="${ZDOTDIR:-$HOME}/.zshrc"
      ;;
    bash)
      # macOS uses .bash_profile for login shells, Linux uses .bashrc
      if [ "$OS" = "darwin" ]; then
        RC="$HOME/.bash_profile"
      else
        RC="$HOME/.bashrc"
      fi
      ;;
    fish)
      RC="$HOME/.config/fish/config.fish"
      LINE="fish_add_path ${INSTALL_DIR}"
      ;;
    *)
      RC="$HOME/.profile"
      ;;
  esac

  # Idempotent: don't append if already present
  if [ -f "$RC" ] && grep -qF "$INSTALL_DIR" "$RC" 2>/dev/null; then
    return 0
  fi

  # Ensure rc file exists
  [ -f "$RC" ] || touch "$RC"

  printf '\n# ctx\n%s\n' "$LINE" >> "$RC"
  info "added ${INSTALL_DIR} to PATH in ${RC}"

  # Export for current script so the final check works
  export PATH="${INSTALL_DIR}:$PATH"
}

ensure_path

# --- Done -------------------------------------------------------------------

ok "ctx v${VERSION} installed successfully"
echo ""
echo "  run \`ctx --help\` to get started"
echo ""

# Hint to reload shell if we modified rc file
if [ "${_CTX_PATH_ADDED:-}" = "1" ]; then
  info "restart your shell or run: source ${RC}"
fi
