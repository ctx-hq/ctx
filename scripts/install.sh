#!/bin/sh
# ctx installer — installs the latest ctx binary
# Usage: curl -fsSL https://getctx.org/install.sh | sh
#
# Environment variables:
#   CTX_INSTALL_DIR    — override installation directory
#   CTX_VERSION        — specific version to install (default: latest)
#   CTX_QUIET          — suppress output (set to 1)
#   CTX_NO_MODIFY_PATH — skip PATH auto-configuration (set to 1)

set -e

REPO="ctx-hq/ctx"
BINARY="ctx"
PATH_MODIFIED=""
UPGRADE=""

# --- Logging ----------------------------------------------------------------

if [ "${CTX_QUIET:-0}" = "1" ]; then
  info() { :; }
else
  info() { printf "\033[0;36m>\033[0m %s\n" "$1" >&2; }
fi
die()  { printf "\033[0;31merror:\033[0m %s\n" "$1" >&2; exit 1; }
ok()   { printf "\033[0;32m>\033[0m %s\n" "$1" >&2; }
warn() { printf "\033[0;33m>\033[0m %s\n" "$1" >&2; }

# --- Helpers ----------------------------------------------------------------

fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$1"
  else
    die "curl or wget is required"
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
    die "sha256sum or shasum is required"
  fi
}

# --- Ensure $HOME is set ---------------------------------------------------

if [ -z "${HOME:-}" ]; then
  HOME=$(eval echo "~")
  [ -d "$HOME" ] || die "\$HOME is not set and cannot be inferred"
  export HOME
fi

# --- Resolve install directory ----------------------------------------------

resolve_install_dir() {
  # User explicitly set
  if [ -n "${CTX_INSTALL_DIR:-}" ]; then
    echo "$CTX_INSTALL_DIR"
    return
  fi

  # Termux on Android
  if [ -n "${PREFIX:-}" ] && [ -d "$PREFIX/bin" ]; then
    echo "$PREFIX/bin"
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
  x86_64|amd64)       ARCH="amd64" ;;
  aarch64|arm64)       ARCH="arm64" ;;
  *) die "unsupported architecture: $ARCH" ;;
esac

case "$OS" in
  linux|darwin) ;;
  msys*|mingw*|cygwin*) die "use install.ps1 for Windows: irm https://getctx.org/install.ps1 | iex" ;;
  *) die "unsupported OS: $OS" ;;
esac

PLATFORM="${OS}-${ARCH}"

# --- Resolve version --------------------------------------------------------

if [ -n "${CTX_VERSION:-}" ]; then
  VERSION="${CTX_VERSION#v}"
else
  info "detecting latest version..."
  VERSION=$(fetch "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
    | grep '"tag_name"' | sed -E 's/.*"v?([^"]+)".*/\1/') || true
  [ -n "$VERSION" ] || die "could not detect latest version (GitHub API may be rate-limited). Set CTX_VERSION=x.y.z to install manually."
fi

# Check if this is an upgrade
if [ -f "$INSTALL_DIR/$BINARY" ]; then
  CURRENT=$("$INSTALL_DIR/$BINARY" version 2>/dev/null | grep -o '"version"[[:space:]]*:[[:space:]]*"[^"]*"' | cut -d'"' -f4) || true
  if [ "$CURRENT" = "$VERSION" ]; then
    ok "ctx v${VERSION} is already installed"
    exit 0
  fi
  if [ -n "$CURRENT" ]; then
    UPGRADE="$CURRENT"
    info "upgrading ctx v${CURRENT} → v${VERSION} (${PLATFORM})"
  else
    info "installing ctx v${VERSION} (${PLATFORM})"
  fi
else
  info "installing ctx v${VERSION} (${PLATFORM})"
fi

# --- Download and verify ----------------------------------------------------

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

ARCHIVE_NAME="${BINARY}-${PLATFORM}.tar.gz"
ARCHIVE_URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ARCHIVE_NAME}"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt"

download_with_retry() {
  _url="$1"
  _dest="$2"
  _retries=3
  _delay=5
  _i=1
  while [ "$_i" -le "$_retries" ]; do
    if download "$_url" "$_dest"; then
      return 0
    fi
    if [ "$_i" -lt "$_retries" ]; then
      warn "download failed (attempt ${_i}/${_retries}), retrying in ${_delay}s..."
      sleep "$_delay"
      _delay=$((_delay * 2))
    fi
    _i=$((_i + 1))
  done
  return 1
}

download_with_retry "$ARCHIVE_URL" "$TMPDIR/${ARCHIVE_NAME}" || die "failed to download ${ARCHIVE_URL}"
download_with_retry "$CHECKSUM_URL" "$TMPDIR/checksums.txt"  || die "failed to download checksums"

# Exact match on archive name (avoid SBOM filename collision)
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
  mv -f "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
else
  info "writing to $INSTALL_DIR (requires sudo)"
  sudo mv -f "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
fi

# --- Auto-configure PATH ---------------------------------------------------

# Append a PATH entry to an rc file (idempotent)
#   $1 = rc file path
#   $2 = line to append
add_to_rc() {
  _rc="$1"
  _line="$2"

  # Already contains this path — skip
  if [ -f "$_rc" ] && grep -qF "$INSTALL_DIR" "$_rc" 2>/dev/null; then
    return 0
  fi

  # Ensure parent dir exists (e.g. fish: ~/.config/fish/)
  _rc_dir=$(dirname "$_rc")
  [ -d "$_rc_dir" ] || mkdir -p "$_rc_dir"

  # Ensure file ends with newline before appending
  if [ -f "$_rc" ] && [ -s "$_rc" ]; then
    if [ "$(tail -c 1 "$_rc" | wc -l)" -eq 0 ]; then
      printf '\n' >> "$_rc"
    fi
  fi

  printf '# ctx\n%s\n' "$_line" >> "$_rc"
  info "added PATH to ${_rc}"
  PATH_MODIFIED="$_rc"
}

ensure_path() {
  # Already in PATH — nothing to do
  case ":$PATH:" in
    *":${INSTALL_DIR}:"*) return 0 ;;
  esac

  # User opted out
  if [ "${CTX_NO_MODIFY_PATH:-0}" = "1" ]; then
    warn "${INSTALL_DIR} is not in PATH (CTX_NO_MODIFY_PATH is set)"
    return 0
  fi

  # CI environments — don't modify rc files, just export
  if [ "${CI:-}" = "true" ] || [ -n "${GITHUB_ACTIONS:-}" ] || [ -n "${GITLAB_CI:-}" ] || [ -n "${CIRCLECI:-}" ] || [ -n "${JENKINS_URL:-}" ]; then
    export PATH="${INSTALL_DIR}:$PATH"
    return 0
  fi

  POSIX_LINE="export PATH=\"${INSTALL_DIR}:\$PATH\""

  # Detect user's login shell
  SHELL_NAME=""
  if [ -n "${SHELL:-}" ]; then
    SHELL_NAME=$(basename "$SHELL")
  fi

  case "$SHELL_NAME" in

    # ── zsh ────────────────────────────────────────────────────────────
    zsh)
      add_to_rc "${ZDOTDIR:-$HOME}/.zshrc" "$POSIX_LINE"
      ;;

    # ── bash ───────────────────────────────────────────────────────────
    bash)
      # macOS: login shell reads .bash_profile first
      # Linux: interactive shell reads .bashrc
      # Write to whichever exists; if both exist, write to both
      _wrote=0
      if [ "$OS" = "darwin" ]; then
        for _f in "$HOME/.bash_profile" "$HOME/.bashrc"; do
          if [ -f "$_f" ]; then
            add_to_rc "$_f" "$POSIX_LINE"
            _wrote=1
          fi
        done
        [ "$_wrote" -eq 0 ] && add_to_rc "$HOME/.bash_profile" "$POSIX_LINE"
      else
        for _f in "$HOME/.bashrc" "$HOME/.bash_profile"; do
          if [ -f "$_f" ]; then
            add_to_rc "$_f" "$POSIX_LINE"
            _wrote=1
          fi
        done
        [ "$_wrote" -eq 0 ] && add_to_rc "$HOME/.bashrc" "$POSIX_LINE"
      fi
      ;;

    # ── fish ───────────────────────────────────────────────────────────
    fish)
      _fish_rc="${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish"
      add_to_rc "$_fish_rc" "fish_add_path ${INSTALL_DIR}"
      ;;

    # ── nushell ────────────────────────────────────────────────────────
    nu)
      _nu_dir="${XDG_CONFIG_HOME:-$HOME/.config}/nushell"
      _nu_rc="$_nu_dir/env.nu"
      if [ -d "$_nu_dir" ] || [ -f "$_nu_rc" ]; then
        add_to_rc "$_nu_rc" "\$env.PATH = (\$env.PATH | split row (char esep) | prepend '${INSTALL_DIR}')"
      else
        add_to_rc "$HOME/.profile" "$POSIX_LINE"
      fi
      ;;

    # ── elvish ─────────────────────────────────────────────────────────
    elvish)
      _elv_rc="${XDG_CONFIG_HOME:-$HOME/.config}/elvish/rc.elv"
      if [ -d "$(dirname "$_elv_rc")" ] || [ -f "$_elv_rc" ]; then
        add_to_rc "$_elv_rc" "set paths = [${INSTALL_DIR} \$@paths]"
      else
        add_to_rc "$HOME/.profile" "$POSIX_LINE"
      fi
      ;;

    # ── ksh ────────────────────────────────────────────────────────────
    ksh|ksh93)
      if [ -f "$HOME/.kshrc" ]; then
        add_to_rc "$HOME/.kshrc" "$POSIX_LINE"
      else
        add_to_rc "$HOME/.profile" "$POSIX_LINE"
      fi
      ;;

    # ── csh / tcsh ─────────────────────────────────────────────────────
    csh|tcsh)
      _csh_rc="$HOME/.cshrc"
      if [ "$SHELL_NAME" = "tcsh" ] && [ -f "$HOME/.tcshrc" ]; then
        _csh_rc="$HOME/.tcshrc"
      fi
      add_to_rc "$_csh_rc" "setenv PATH ${INSTALL_DIR}:\$PATH"
      ;;

    # ── fallback (sh, dash, ash, busybox, unknown) ─────────────────────
    *)
      add_to_rc "$HOME/.profile" "$POSIX_LINE"
      ;;
  esac

  # Export for current session
  export PATH="${INSTALL_DIR}:$PATH"
}

ensure_path

# --- Done -------------------------------------------------------------------

if [ -n "$UPGRADE" ]; then
  ok "ctx upgraded v${UPGRADE} → v${VERSION}"
else
  ok "ctx v${VERSION} installed to ${INSTALL_DIR}/${BINARY}"
fi
echo ""
echo "  run \`ctx --help\` to get started"

# Hint to reload shell if we modified an rc file
if [ -n "$PATH_MODIFIED" ]; then
  echo ""
  SHELL_NAME=$(basename "${SHELL:-/bin/sh}")
  case "$SHELL_NAME" in
    fish)     info "run: source ${PATH_MODIFIED}" ;;
    nu)       info "run: source ${PATH_MODIFIED}" ;;
    csh|tcsh) info "run: source ${PATH_MODIFIED}" ;;
    *)        info "restart your terminal or run: source ${PATH_MODIFIED}" ;;
  esac
fi
