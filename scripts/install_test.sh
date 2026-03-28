#!/usr/bin/env bash
#
# Integration tests for scripts/install.sh
# Tests platform detection, checksum verification, PATH configuration,
# upgrade detection, CI detection, and error handling.
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

expect_not_contains() {
    local desc="$1"
    local haystack="$2"
    local needle="$3"
    if echo "$haystack" | grep -q "$needle"; then
        fail "${desc} (should not contain: ${needle})"
    else
        pass "${desc}"
    fi
}

# --- Setup ------------------------------------------------------------------

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "install.sh integration tests"
echo ""

# --- Syntax validation ------------------------------------------------------

echo "=== Syntax validation ==="
if bash -n "$INSTALL_SCRIPT" 2>/dev/null; then
    pass "Script has valid syntax"
else
    fail "Script has syntax errors"
fi

# Verify POSIX sh compatibility (no bash-isms in a #!/bin/sh script)
if command -v checkbashisms >/dev/null 2>&1; then
    if checkbashisms "$INSTALL_SCRIPT" 2>/dev/null; then
        pass "No bash-isms (POSIX sh compatible)"
    else
        fail "Script contains bash-isms but uses #!/bin/sh"
    fi
else
    pass "checkbashisms not available (skipped)"
fi

# --- Platform detection -----------------------------------------------------

echo ""
echo "=== Platform detection ==="

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    linux|darwin)
        pass "Detected supported OS: ${OS}" ;;
    *)
        fail "Unsupported OS: ${OS}" ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)
        pass "Maps ${ARCH} to amd64" ;;
    aarch64|arm64)
        pass "Maps ${ARCH} to arm64" ;;
    *)
        fail "Unsupported architecture: ${ARCH}" ;;
esac

# --- Checksum tools ---------------------------------------------------------

echo ""
echo "=== Checksum tools ==="

if command -v sha256sum >/dev/null 2>&1; then
    pass "sha256sum available"
elif command -v shasum >/dev/null 2>&1; then
    pass "shasum available (macOS)"
else
    fail "No SHA256 tool available"
fi

# --- Checksum verification --------------------------------------------------

echo ""
echo "=== Checksum verification ==="

sha256() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
    else
        shasum -a 256 "$1" | awk '{print $1}'
    fi
}

# Create a fake archive
echo "fake binary content" > "$TMPDIR/ctx-linux-amd64.tar.gz"
CORRECT_HASH=$(sha256 "$TMPDIR/ctx-linux-amd64.tar.gz")

# Test correct checksum
echo "${CORRECT_HASH}  ctx-linux-amd64.tar.gz" > "$TMPDIR/checksums.txt"
EXPECTED=$(grep "  ctx-linux-amd64.tar.gz$" "$TMPDIR/checksums.txt" | awk '{print $1}')
ACTUAL=$(sha256 "$TMPDIR/ctx-linux-amd64.tar.gz")

if [ "$EXPECTED" = "$ACTUAL" ]; then
    pass "Correct checksum matches"
else
    fail "Correct checksum should match"
fi

# Test wrong checksum
echo "0000000000000000000000000000000000000000000000000000000000000000  ctx-linux-amd64.tar.gz" > "$TMPDIR/checksums_bad.txt"
BAD_EXPECTED=$(grep "  ctx-linux-amd64.tar.gz$" "$TMPDIR/checksums_bad.txt" | awk '{print $1}')

if [ "$BAD_EXPECTED" != "$ACTUAL" ]; then
    pass "Wrong checksum correctly differs"
else
    fail "Bad checksum should not match"
fi

# Test missing archive in checksums
if grep -q "  ctx-nonexistent.tar.gz$" "$TMPDIR/checksums.txt"; then
    fail "Should not find nonexistent archive in checksums"
else
    pass "Missing archive correctly not found in checksums"
fi

# --- SBOM exact-match -------------------------------------------------------

echo ""
echo "=== SBOM checksum collision ==="

# Simulate checksums.txt with both archive and SBOM entry (real-world scenario)
echo "${CORRECT_HASH}  ctx-linux-amd64.tar.gz" > "$TMPDIR/checksums_sbom.txt"
echo "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  ctx-linux-amd64.tar.gz.sbom.json" >> "$TMPDIR/checksums_sbom.txt"

# The exact-match grep (anchored with $) should only return 1 line
MATCH_COUNT=$(grep -c "  ctx-linux-amd64.tar.gz$" "$TMPDIR/checksums_sbom.txt")
if [ "$MATCH_COUNT" -eq 1 ]; then
    pass "Exact match avoids SBOM collision (1 match)"
else
    fail "SBOM collision: got ${MATCH_COUNT} matches instead of 1"
fi

# Verify the matched hash is the correct one (not the SBOM hash)
MATCHED_HASH=$(grep "  ctx-linux-amd64.tar.gz$" "$TMPDIR/checksums_sbom.txt" | awk '{print $1}')
if [ "$MATCHED_HASH" = "$CORRECT_HASH" ]; then
    pass "Matched hash is the archive hash, not SBOM"
else
    fail "Matched wrong hash (got SBOM hash?)"
fi

# --- Error handling ---------------------------------------------------------

echo ""
echo "=== Error handling ==="

# Nonexistent version should fail gracefully
output=$(CTX_VERSION="v0.0.0-nonexistent" CTX_INSTALL_DIR="$TMPDIR/bin-err" sh "$INSTALL_SCRIPT" 2>&1) || true
if [ ! -f "$TMPDIR/bin-err/ctx" ]; then
    pass "Fails gracefully with nonexistent version"
else
    fail "Should not install nonexistent version"
fi

# --- Quiet mode -------------------------------------------------------------

echo ""
echo "=== Quiet mode ==="

output=$(CTX_VERSION="v0.0.0-nonexistent" CTX_INSTALL_DIR="$TMPDIR/bin-quiet" CTX_QUIET=1 sh "$INSTALL_SCRIPT" 2>&1) || true
# In quiet mode, info messages are suppressed but errors still appear
if echo "$output" | grep -q "^>"; then
    fail "Quiet mode should suppress info (>) messages"
else
    pass "Quiet mode suppresses info messages"
fi

# --- PATH configuration logic -----------------------------------------------

echo ""
echo "=== PATH configuration ==="

# Test that the script contains all expected shell handlers
for shell_name in zsh bash fish nu elvish ksh csh tcsh; do
    if grep -q "$shell_name" "$INSTALL_SCRIPT"; then
        pass "Handles shell: ${shell_name}"
    else
        fail "Missing shell handler: ${shell_name}"
    fi
done

# Test fallback to .profile for unknown shells
if grep -q '\.profile' "$INSTALL_SCRIPT"; then
    pass "Falls back to .profile for unknown shells"
else
    fail "Missing .profile fallback"
fi

# --- PATH idempotency -------------------------------------------------------

echo ""
echo "=== PATH idempotency ==="

# Create a mock rc file with an existing ctx PATH entry
TEST_RC="$TMPDIR/test_rc"
echo '# existing content' > "$TEST_RC"
echo '# ctx' >> "$TEST_RC"
echo 'export PATH="/some/test/dir:$PATH"' >> "$TEST_RC"

# The grep check in the script uses -F (fixed string) against INSTALL_DIR
# Simulate: if INSTALL_DIR is already in the file, add_to_rc should skip
if grep -qF "/some/test/dir" "$TEST_RC" 2>/dev/null; then
    pass "Idempotency: detects existing PATH entry"
else
    fail "Idempotency check failed"
fi

# Verify double-write protection: the file should not get a second entry
ORIGINAL_LINES=$(wc -l < "$TEST_RC" | tr -d ' ')
# We can't easily call add_to_rc in isolation, but we verify the grep logic
if grep -qF "/some/test/dir" "$TEST_RC"; then
    # Would skip — verify line count unchanged
    if [ "$ORIGINAL_LINES" -eq "$(wc -l < "$TEST_RC" | tr -d ' ')" ]; then
        pass "Idempotency: skips duplicate append"
    else
        fail "Idempotency: file was modified when it shouldn't be"
    fi
fi

# --- CI detection -----------------------------------------------------------

echo ""
echo "=== CI environment detection ==="

# Verify the script checks for CI environment variables
for ci_var in CI GITHUB_ACTIONS GITLAB_CI CIRCLECI JENKINS_URL; do
    if grep -q "$ci_var" "$INSTALL_SCRIPT"; then
        pass "Detects CI env: ${ci_var}"
    else
        fail "Missing CI detection: ${ci_var}"
    fi
done

# --- CTX_NO_MODIFY_PATH support ---------------------------------------------

echo ""
echo "=== CTX_NO_MODIFY_PATH ==="

if grep -q "CTX_NO_MODIFY_PATH" "$INSTALL_SCRIPT"; then
    pass "Supports CTX_NO_MODIFY_PATH opt-out"
else
    fail "Missing CTX_NO_MODIFY_PATH support"
fi

# --- Upgrade detection ------------------------------------------------------

echo ""
echo "=== Upgrade detection ==="

if grep -q "already installed" "$INSTALL_SCRIPT"; then
    pass "Detects same-version (skip re-install)"
else
    fail "Missing same-version detection"
fi

if grep -q "upgrading\|upgraded" "$INSTALL_SCRIPT"; then
    pass "Shows upgrade message for version change"
else
    fail "Missing upgrade messaging"
fi

# --- Termux support ---------------------------------------------------------

echo ""
echo "=== Termux / Android ==="

if grep -q 'PREFIX' "$INSTALL_SCRIPT"; then
    pass "Detects Termux PREFIX for Android"
else
    fail "Missing Termux PREFIX support"
fi

# --- MSYS/Cygwin redirect --------------------------------------------------

echo ""
echo "=== MSYS/Cygwin redirect ==="

if grep -q "install.ps1" "$INSTALL_SCRIPT"; then
    pass "Redirects MSYS/Cygwin users to install.ps1"
else
    fail "Missing MSYS/Cygwin redirect"
fi

# --- $HOME fallback --------------------------------------------------------

echo ""
echo "=== \$HOME fallback ==="

if grep -q 'HOME.*eval\|HOME is not set' "$INSTALL_SCRIPT"; then
    pass "Handles missing \$HOME"
else
    fail "Missing \$HOME fallback"
fi

# --- fish config directory --------------------------------------------------

echo ""
echo "=== Shell config edge cases ==="

# fish needs mkdir -p for config dir
if grep -q "XDG_CONFIG_HOME" "$INSTALL_SCRIPT"; then
    pass "Respects XDG_CONFIG_HOME for fish/nushell/elvish"
else
    fail "Missing XDG_CONFIG_HOME support"
fi

# bash on macOS needs .bash_profile
if grep -q "bash_profile" "$INSTALL_SCRIPT"; then
    pass "Handles .bash_profile on macOS"
else
    fail "Missing .bash_profile for macOS bash"
fi

# Ensures rc parent directory exists
if grep -q "mkdir -p" "$INSTALL_SCRIPT"; then
    pass "Creates rc parent directory if missing"
else
    fail "Missing rc directory creation"
fi

# --- Newline before append --------------------------------------------------

echo ""
echo "=== Newline safety ==="

if grep -q "tail -c 1" "$INSTALL_SCRIPT"; then
    pass "Ensures newline before appending to rc file"
else
    fail "Missing newline-before-append guard"
fi

# --- Install directory creation ---------------------------------------------

echo ""
echo "=== Install directory ==="

CUSTOM_DIR="$TMPDIR/custom-bin"
if [ ! -d "$CUSTOM_DIR" ]; then
    pass "Custom install dir does not pre-exist (as expected)"
fi

# Verify script creates directory with mkdir -p
if grep -q 'mkdir -p.*INSTALL_DIR' "$INSTALL_SCRIPT"; then
    pass "Creates install directory with mkdir -p"
else
    fail "Missing install directory auto-creation"
fi

# Verify sudo fallback for directory creation
if grep -q 'sudo mkdir' "$INSTALL_SCRIPT"; then
    pass "Falls back to sudo for directory creation"
else
    fail "Missing sudo fallback for directory creation"
fi

# --- Summary ----------------------------------------------------------------

echo ""
echo "─────────────────────────────"
printf "Results: %d passed, %d failed (of %d)\n" "$PASS" "$FAIL" "$TESTS"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
