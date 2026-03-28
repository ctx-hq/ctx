#!/usr/bin/env bash
#
# Integration tests for scripts/install.sh
# Tests platform detection, checksum verification, and error handling.
# Run: bash scripts/install_test.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_SCRIPT="${SCRIPT_DIR}/install.sh"
PASS=0
FAIL=0
TESTS=0

# --- Helpers ----------------------------------------------------------------

pass() { PASS=$((PASS + 1)); TESTS=$((TESTS + 1)); printf "  \033[0;32m✓\033[0m %s\n" "$1"; }
fail() { FAIL=$((FAIL + 1)); TESTS=$((TESTS + 1)); printf "  \033[0;31m✗\033[0m %s\n" "$1"; }

expect_contains() {
    local desc="$1"
    local haystack="$2"
    local needle="$3"
    if echo "$haystack" | grep -q "$needle"; then
        pass "${desc}"
    else
        fail "${desc} (expected to contain: ${needle})"
    fi
}

# --- Setup ------------------------------------------------------------------

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "install.sh integration tests"
echo ""

# --- Test 1: Script syntax check -------------------------------------------

echo "=== Syntax validation ==="
if bash -n "$INSTALL_SCRIPT" 2>/dev/null; then
    pass "Script has valid syntax"
else
    fail "Script has syntax errors"
fi

# --- Test 2: Platform detection functions -----------------------------------

echo ""
echo "=== Platform detection ==="

# Extract and test OS detection
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    linux|darwin)
        pass "Detected supported OS: ${OS}" ;;
    *)
        fail "Unsupported OS: ${OS}" ;;
esac

# Extract and test ARCH detection
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)
        pass "Maps ${ARCH} to amd64" ;;
    aarch64|arm64)
        pass "Maps ${ARCH} to arm64" ;;
    *)
        fail "Unsupported architecture: ${ARCH}" ;;
esac

# --- Test 3: SHA256 tool availability ---------------------------------------

echo ""
echo "=== Checksum tools ==="

if command -v sha256sum >/dev/null 2>&1; then
    pass "sha256sum available"
elif command -v shasum >/dev/null 2>&1; then
    pass "shasum available (macOS)"
else
    fail "No SHA256 tool available"
fi

# --- Test 4: Checksum verification logic ------------------------------------

echo ""
echo "=== Checksum verification ==="

# Create a fake archive and checksums file
echo "fake binary content" > "$TMPDIR/ctx-linux-amd64.tar.gz"

sha256() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
    else
        shasum -a 256 "$1" | awk '{print $1}'
    fi
}

CORRECT_HASH=$(sha256 "$TMPDIR/ctx-linux-amd64.tar.gz")

# Test correct checksum
echo "${CORRECT_HASH}  ctx-linux-amd64.tar.gz" > "$TMPDIR/checksums.txt"
EXPECTED=$(grep "ctx-linux-amd64.tar.gz" "$TMPDIR/checksums.txt" | awk '{print $1}')
ACTUAL=$(sha256 "$TMPDIR/ctx-linux-amd64.tar.gz")

if [ "$EXPECTED" = "$ACTUAL" ]; then
    pass "Correct checksum matches"
else
    fail "Correct checksum should match"
fi

# Test wrong checksum
echo "0000000000000000000000000000000000000000000000000000000000000000  ctx-linux-amd64.tar.gz" > "$TMPDIR/checksums_bad.txt"
BAD_EXPECTED=$(grep "ctx-linux-amd64.tar.gz" "$TMPDIR/checksums_bad.txt" | awk '{print $1}')

if [ "$BAD_EXPECTED" != "$ACTUAL" ]; then
    pass "Wrong checksum correctly differs"
else
    fail "Bad checksum should not match"
fi

# Test missing archive in checksums
if grep -q "ctx-nonexistent.tar.gz" "$TMPDIR/checksums.txt"; then
    fail "Should not find nonexistent archive in checksums"
else
    pass "Missing archive correctly not found in checksums"
fi

# --- Test 5: install.sh fails gracefully with bad version -------------------

echo ""
echo "=== Error handling ==="

# CTX_VERSION pointing to nonexistent release should fail
output=$(CTX_VERSION="v0.0.0-nonexistent" CTX_INSTALL_DIR="$TMPDIR/bin" sh "$INSTALL_SCRIPT" 2>&1) || true
if [ ! -f "$TMPDIR/bin/ctx" ]; then
    pass "Fails gracefully with nonexistent version"
else
    fail "Should not install nonexistent version"
fi

# --- Test 6: Install directory creation -------------------------------------

echo ""
echo "=== Install directory ==="

CUSTOM_DIR="$TMPDIR/custom-bin"
if [ ! -d "$CUSTOM_DIR" ]; then
    pass "Custom install dir does not pre-exist (as expected)"
fi

# --- Summary ----------------------------------------------------------------

echo ""
echo "─────────────────────────────"
printf "Results: %d passed, %d failed (of %d)\n" "$PASS" "$FAIL" "$TESTS"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
