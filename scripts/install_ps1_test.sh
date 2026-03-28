#!/usr/bin/env bash
#
# Validation tests for scripts/install.ps1
# Runs on any platform (Linux CI included) — validates structure, not execution.
# Run: bash scripts/install_ps1_test.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PS1_SCRIPT="${SCRIPT_DIR}/install.ps1"
PASS=0
FAIL=0
TESTS=0

# --- Helpers ----------------------------------------------------------------

pass() { PASS=$((PASS + 1)); TESTS=$((TESTS + 1)); printf "  \033[0;32m✓\033[0m %s\n" "$1"; }
fail() { FAIL=$((FAIL + 1)); TESTS=$((TESTS + 1)); printf "  \033[0;31m✗\033[0m %s\n" "$1"; }

expect_contains() {
    local desc="$1"
    local file="$2"
    local pattern="$3"
    if grep -q "$pattern" "$file"; then
        pass "${desc}"
    else
        fail "${desc} (expected to contain: ${pattern})"
    fi
}

echo "install.ps1 validation tests"
echo ""

# --- Test 1: File exists and is non-empty ------------------------------------

echo "=== File integrity ==="

if [ -f "$PS1_SCRIPT" ]; then
    pass "install.ps1 exists"
else
    fail "install.ps1 not found at ${PS1_SCRIPT}"
    echo ""
    printf "Results: %d passed, %d failed (of %d)\n" "$PASS" "$FAIL" "$TESTS"
    exit 1
fi

if [ -s "$PS1_SCRIPT" ]; then
    pass "install.ps1 is non-empty"
else
    fail "install.ps1 is empty"
fi

# --- Test 2: No UTF-8 BOM ---------------------------------------------------

echo ""
echo "=== Encoding ==="

FIRST_BYTES=$(xxd -l 3 -p "$PS1_SCRIPT" 2>/dev/null || od -A n -t x1 -N 3 "$PS1_SCRIPT" | tr -d ' ')
if [ "$FIRST_BYTES" = "efbbbf" ]; then
    fail "File has UTF-8 BOM (breaks curl piping)"
else
    pass "No UTF-8 BOM"
fi

# Verify no carriage returns (should use Unix line endings for curl compatibility)
# Use tr + wc instead of grep -P which is unsupported on macOS/BSD
CR_COUNT=$(tr -cd '\r' < "$PS1_SCRIPT" | wc -c | tr -d ' ')
if [ "$CR_COUNT" -gt 0 ]; then
    fail "File contains ${CR_COUNT} carriage return(s) (CRLF line endings)"
else
    pass "Unix line endings (LF)"
fi

# --- Test 3: Required components present -------------------------------------

echo ""
echo "=== Required components ==="

expect_contains "Has SHA256 verification" "$PS1_SCRIPT" "SHA256"
expect_contains "Has install directory variable" "$PS1_SCRIPT" "INSTALL_DIR\|InstallDir"
expect_contains "Has version resolution" "$PS1_SCRIPT" "CTX_VERSION"
expect_contains "Has GitHub API call" "$PS1_SCRIPT" "api.github.com"
expect_contains "Has archive download" "$PS1_SCRIPT" "github.com.*releases"
expect_contains "Has checksum download" "$PS1_SCRIPT" "checksums.txt"
expect_contains "Has error handling" "$PS1_SCRIPT" "ErrorActionPreference"
expect_contains "Has cleanup/finally block" "$PS1_SCRIPT" "finally"
expect_contains "Has PATH integration" "$PS1_SCRIPT" "Path.*User"
expect_contains "Has architecture detection" "$PS1_SCRIPT" "PROCESSOR_ARCHITECTURE"
expect_contains "Detects ARM64" "$PS1_SCRIPT" "ARM64"
expect_contains "Has zip extraction" "$PS1_SCRIPT" "Expand-Archive"
expect_contains "Has TLS configuration" "$PS1_SCRIPT" "Tls12"

# --- Test 4: Matches install.sh repo reference --------------------------------

echo ""
echo "=== Consistency with install.sh ==="

SH_REPO=$(grep 'REPO=' "${SCRIPT_DIR}/install.sh" | head -1 | sed 's/.*"\(.*\)".*/\1/')
if grep -q "$SH_REPO" "$PS1_SCRIPT"; then
    pass "Same repo reference as install.sh (${SH_REPO})"
else
    fail "Repo reference differs from install.sh"
fi

# --- Summary ----------------------------------------------------------------

echo ""
echo "---"
printf "Results: %d passed, %d failed (of %d)\n" "$PASS" "$FAIL" "$TESTS"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
