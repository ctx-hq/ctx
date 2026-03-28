#!/usr/bin/env bash
#
# Integration tests for scripts/release.sh
# Run: bash scripts/release_test.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RELEASE_SCRIPT="${SCRIPT_DIR}/release.sh"
PASS=0
FAIL=0
TESTS=0
ORIGINAL_DIR="$(pwd)"

# --- Helpers ----------------------------------------------------------------

pass() { PASS=$((PASS + 1)); TESTS=$((TESTS + 1)); printf "  \033[0;32m✓\033[0m %s\n" "$1"; }
fail() { FAIL=$((FAIL + 1)); TESTS=$((TESTS + 1)); printf "  \033[0;31m✗\033[0m %s\n" "$1"; }

expect_contains() {
    local desc="$1"
    local output="$2"
    local pattern="$3"
    if echo "$output" | grep -q "$pattern"; then
        pass "${desc}"
    else
        fail "${desc} (expected output to contain: ${pattern})"
    fi
}

# Creates an isolated test repo and prints its path.
# Each call gets a unique directory.
new_test_repo() {
    local repo_dir
    repo_dir=$(mktemp -d)

    (
        cd "$repo_dir"
        git init --initial-branch=main -q
        git config user.email "test@test.com"
        git config user.name "Test"

        cat > go.mod <<'GOMOD'
module test
go 1.25.6
GOMOD

        cat > Makefile <<'MAKEFILE'
.PHONY: release-check fmt-check vet lint test check
fmt-check:
	@true
vet:
	@true
lint:
	@true
test:
	@true
check: fmt-check vet lint test
release-check: check
	@grep -q '^replace' go.mod 2>/dev/null && { echo "Error: go.mod contains replace directives"; exit 1; } || true
	@echo "All release checks passed"
MAKEFILE

        git add .
        git commit -m "init" -q

        local remote_dir
        remote_dir=$(mktemp -d)
        git clone --bare -q "$repo_dir" "$remote_dir" 2>/dev/null
        git remote add origin "$remote_dir" 2>/dev/null
        git fetch origin -q 2>/dev/null
        git branch --set-upstream-to=origin/main main >/dev/null 2>&1
    )

    printf '%s' "$repo_dir"
}

# --- Tests ------------------------------------------------------------------

echo "release.sh integration tests"
echo ""

# Test 1: No arguments shows usage
echo "=== Argument parsing ==="
output=$(bash "$RELEASE_SCRIPT" 2>&1 || true)
expect_contains "No args shows usage" "$output" "Usage:"

# Test 2: Invalid semver rejected
output=$(bash "$RELEASE_SCRIPT" "abc" 2>&1 || true)
expect_contains "Rejects 'abc'" "$output" "Invalid semver"

output=$(bash "$RELEASE_SCRIPT" "1.2.3" 2>&1 || true)
expect_contains "Rejects '1.2.3' (no v)" "$output" "Invalid semver"

output=$(bash "$RELEASE_SCRIPT" "v1" 2>&1 || true)
expect_contains "Rejects 'v1'" "$output" "Invalid semver"

output=$(bash "$RELEASE_SCRIPT" "v1.2" 2>&1 || true)
expect_contains "Rejects 'v1.2'" "$output" "Invalid semver"

# Test 3: Valid semver formats accepted
echo ""
echo "=== Semver validation ==="
for ver in "v1.0.0" "v0.1.0" "v1.2.3-rc.1" "v1.0.0-beta.2"; do
    output=$(bash "$RELEASE_SCRIPT" "$ver" 2>&1 || true)
    if echo "$output" | grep -q "Invalid semver"; then
        fail "Should accept ${ver}"
    else
        pass "Accepts ${ver}"
    fi
done

# Test 4: Non-main branch rejected
echo ""
echo "=== Branch checks ==="
REPO=$(new_test_repo)
cd "$REPO"
git checkout -b feature-branch -q
output=$(bash "$RELEASE_SCRIPT" "v1.0.0" 2>&1 || true)
expect_contains "Rejects non-main branch" "$output" "Must be on main"
cd "$ORIGINAL_DIR"
rm -rf "$REPO"

# Test 5: Dirty working tree rejected
echo ""
echo "=== Clean tree checks ==="
REPO=$(new_test_repo)
cd "$REPO"
echo "dirty" > dirty.txt
output=$(bash "$RELEASE_SCRIPT" "v1.0.0" 2>&1 || true)
expect_contains "Rejects dirty working tree" "$output" "uncommitted changes"
cd "$ORIGINAL_DIR"
rm -rf "$REPO"

# Test 6: replace directive detected
echo ""
echo "=== Replace directive checks ==="
REPO=$(new_test_repo)
cd "$REPO"
echo "replace example.com/foo => ../foo" >> go.mod
git add . && git commit -m "add replace" -q
git push origin main -q 2>/dev/null
output=$(bash "$RELEASE_SCRIPT" "v1.0.0" 2>&1 || true)
expect_contains "Detects replace directive" "$output" "replace"
cd "$ORIGINAL_DIR"
rm -rf "$REPO"

# Test 7: Duplicate tag rejected
echo ""
echo "=== Tag collision checks ==="
REPO=$(new_test_repo)
cd "$REPO"
git tag -a "v1.0.0" -m "existing"
output=$(bash "$RELEASE_SCRIPT" "v1.0.0" 2>&1 || true)
expect_contains "Rejects duplicate tag" "$output" "already exists"
cd "$ORIGINAL_DIR"
rm -rf "$REPO"

# Test 8: Dry-run does not create tag
echo ""
echo "=== Dry-run mode ==="
REPO=$(new_test_repo)
cd "$REPO"
output=$(bash "$RELEASE_SCRIPT" "v9.9.9" "--dry-run" 2>&1 || true)
if git rev-parse "v9.9.9" >/dev/null 2>&1; then
    fail "Dry-run should not create tag"
else
    pass "Dry-run does not create tag"
fi
expect_contains "Dry-run shows message" "$output" "dry-run"
cd "$ORIGINAL_DIR"
rm -rf "$REPO"

# --- Summary ----------------------------------------------------------------

echo ""
echo "─────────────────────────────"
printf "Results: %d passed, %d failed (of %d)\n" "$PASS" "$FAIL" "$TESTS"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
