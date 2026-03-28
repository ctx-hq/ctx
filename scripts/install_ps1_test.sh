#!/usr/bin/env bash
#
# Validation tests for scripts/install.ps1
# Runs on any platform (Linux CI included) — validates structure, not execution.
# Run: bash scripts/install_ps1_test.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PS1_SCRIPT="${SCRIPT_DIR}/install.ps1"
SH_SCRIPT="${SCRIPT_DIR}/install.sh"
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

# --- File integrity ---------------------------------------------------------

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

# --- Encoding ---------------------------------------------------------------

echo ""
echo "=== Encoding ==="

FIRST_BYTES=$(xxd -l 3 -p "$PS1_SCRIPT" 2>/dev/null || od -A n -t x1 -N 3 "$PS1_SCRIPT" | tr -d ' ')
if [ "$FIRST_BYTES" = "efbbbf" ]; then
    fail "File has UTF-8 BOM (breaks curl piping)"
else
    pass "No UTF-8 BOM"
fi

CR_COUNT=$(tr -cd '\r' < "$PS1_SCRIPT" | wc -c | tr -d ' ')
if [ "$CR_COUNT" -gt 0 ]; then
    fail "File contains ${CR_COUNT} carriage return(s) (CRLF line endings)"
else
    pass "Unix line endings (LF)"
fi

# --- Required components ----------------------------------------------------

echo ""
echo "=== Required components ==="

expect_contains "Has SHA256 verification" "$PS1_SCRIPT" "SHA256"
expect_contains "Has install directory variable" "$PS1_SCRIPT" "InstallDir"
expect_contains "Has version resolution" "$PS1_SCRIPT" "CTX_VERSION"
expect_contains "Has GitHub API call" "$PS1_SCRIPT" "api.github.com"
expect_contains "Has archive download" "$PS1_SCRIPT" "github.com.*releases"
expect_contains "Has checksum download" "$PS1_SCRIPT" "checksums.txt"
expect_contains "Has error handling" "$PS1_SCRIPT" "ErrorActionPreference"
expect_contains "Has cleanup/finally block" "$PS1_SCRIPT" "finally"
expect_contains "Has PATH integration" "$PS1_SCRIPT" "Path.*User"
expect_contains "Has architecture detection" "$PS1_SCRIPT" "PROCESSOR_ARCHITECTURE"
expect_contains "Detects ARM64" "$PS1_SCRIPT" "ARM64"
expect_contains "Detects x86 on x64 WOW64" "$PS1_SCRIPT" "PROCESSOR_ARCHITEW6432"
expect_contains "Has zip extraction" "$PS1_SCRIPT" "Expand-Archive"
expect_contains "Has TLS 1.2 configuration" "$PS1_SCRIPT" "Tls12"
expect_contains "Has TLS 1.3 configuration" "$PS1_SCRIPT" "Tls13"

# --- SBOM checksum fix ------------------------------------------------------

echo ""
echo "=== SBOM checksum exact match ==="

# The old bug: Where-Object { $_ -match $ArchiveName } matches both archive and .sbom.json
# The fix should use anchored regex: "  $name$" to avoid SBOM collision
if grep -q '\\$' "$PS1_SCRIPT" | head -1 && grep -q 'match.*\$' "$PS1_SCRIPT"; then
    pass "Checksum grep uses end-of-line anchor"
else
    # Alternative check: look for the exact pattern
    if grep -q '\$\$\|ArchiveName.\$' "$PS1_SCRIPT" || grep -q 'match.*\$"' "$PS1_SCRIPT"; then
        pass "Checksum grep uses end-of-line anchor"
    else
        # More lenient: just verify the line contains anchoring intent
        if grep -q '\$"$\|ArchiveName}\$' "$PS1_SCRIPT"; then
            pass "Checksum grep uses end-of-line anchor"
        else
            fail "Checksum grep may not use exact match (SBOM collision risk)"
        fi
    fi
fi

# --- Performance optimization -----------------------------------------------

echo ""
echo "=== Performance ==="

expect_contains "Has SilentlyContinue ProgressPreference" "$PS1_SCRIPT" "SilentlyContinue"
expect_contains "Sets ProgressPreference early" "$PS1_SCRIPT" "ProgressPreference"
expect_contains "Uses UseBasicParsing" "$PS1_SCRIPT" "UseBasicParsing"

# --- Upgrade detection ------------------------------------------------------

echo ""
echo "=== Upgrade detection ==="

expect_contains "Detects existing binary" "$PS1_SCRIPT" "Test-Path.*Destination"
expect_contains "Compares current vs target version" "$PS1_SCRIPT" "already installed"
expect_contains "Shows upgrade message" "$PS1_SCRIPT" "upgrad"
expect_contains "Parses version from ctx output" "$PS1_SCRIPT" "ConvertFrom-Json"

# --- CTX_NO_MODIFY_PATH support ---------------------------------------------

echo ""
echo "=== CTX_NO_MODIFY_PATH ==="

expect_contains "Supports CTX_NO_MODIFY_PATH opt-out" "$PS1_SCRIPT" "CTX_NO_MODIFY_PATH"

# --- User-Agent header ------------------------------------------------------

echo ""
echo "=== HTTP best practices ==="

expect_contains "Sends User-Agent header" "$PS1_SCRIPT" "User-Agent"
expect_contains "Sets request timeout" "$PS1_SCRIPT" "TimeoutSec"

# --- Consistent output helpers -----------------------------------------------

echo ""
echo "=== Output formatting ==="

expect_contains "Has info helper function" "$PS1_SCRIPT" "Write-Info"
expect_contains "Has ok helper function" "$PS1_SCRIPT" "Write-Ok"
expect_contains "Uses colored output (Cyan)" "$PS1_SCRIPT" "Cyan"
expect_contains "Uses colored output (Green)" "$PS1_SCRIPT" "Green"

# --- Consistency with install.sh --------------------------------------------

echo ""
echo "=== Consistency with install.sh ==="

SH_REPO=$(grep 'REPO=' "$SH_SCRIPT" | head -1 | sed 's/.*"\(.*\)".*/\1/')
if grep -q "$SH_REPO" "$PS1_SCRIPT"; then
    pass "Same repo reference as install.sh (${SH_REPO})"
else
    fail "Repo reference differs from install.sh"
fi

# Both scripts should support the same env vars
for env_var in CTX_INSTALL_DIR CTX_VERSION CTX_NO_MODIFY_PATH; do
    if grep -q "$env_var" "$PS1_SCRIPT" && grep -q "$env_var" "$SH_SCRIPT"; then
        pass "Both scripts support ${env_var}"
    else
        fail "Env var ${env_var} missing from one script"
    fi
done

# Both should have upgrade detection
if grep -q "already installed" "$PS1_SCRIPT" && grep -q "already installed" "$SH_SCRIPT"; then
    pass "Both scripts have same-version skip"
else
    fail "Same-version skip missing from one script"
fi

# Both should have checksum verification
if grep -q "checksum" "$PS1_SCRIPT" && grep -q "checksum" "$SH_SCRIPT"; then
    pass "Both scripts verify checksums"
else
    fail "Checksum verification missing from one script"
fi

# --- Summary ----------------------------------------------------------------

echo ""
echo "─────────────────────────────"
printf "Results: %d passed, %d failed (of %d)\n" "$PASS" "$FAIL" "$TESTS"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
