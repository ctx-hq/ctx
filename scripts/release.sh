#!/usr/bin/env bash
#
# release.sh — Create and push a release tag with preflight safety checks.
#
# Usage:
#   scripts/release.sh v0.2.0            # Create and push tag
#   scripts/release.sh v0.2.0 --dry-run  # Run checks only, don't tag
#
set -euo pipefail

# --- Helpers ----------------------------------------------------------------

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

pass() { printf "  ${GREEN}✓${NC} %s\n" "$1"; }
fail() { printf "  ${RED}✗${NC} %s\n" "$1"; exit 1; }
warn() { printf "  ${YELLOW}!${NC} %s\n" "$1"; }
info() { printf "  → %s\n" "$1"; }

# --- Parse arguments --------------------------------------------------------

VERSION="${1:-}"
DRY_RUN=false

if [[ -z "$VERSION" ]]; then
    echo "Usage: scripts/release.sh <version> [--dry-run]"
    echo ""
    echo "Examples:"
    echo "  scripts/release.sh v0.2.0"
    echo "  scripts/release.sh v0.2.0 --dry-run"
    echo "  scripts/release.sh v1.0.0-rc.1"
    exit 1
fi

shift
while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run) DRY_RUN=true; shift ;;
        *) fail "Unknown argument: $1" ;;
    esac
done

echo "Release preflight for ${VERSION}:"
echo ""

# --- Check 1: Semver format -------------------------------------------------

if [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
    pass "Valid semver: ${VERSION}"
else
    fail "Invalid semver format: ${VERSION} (expected vX.Y.Z or vX.Y.Z-suffix)"
fi

# --- Check 2: On default branch ---------------------------------------------

DEFAULT_BRANCH="main"
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

if [[ "$CURRENT_BRANCH" == "$DEFAULT_BRANCH" ]]; then
    pass "On branch: ${CURRENT_BRANCH}"
else
    fail "Must be on ${DEFAULT_BRANCH} branch (currently on: ${CURRENT_BRANCH})"
fi

# --- Check 3: Clean working tree --------------------------------------------

if [[ -z "$(git status --porcelain)" ]]; then
    pass "Working tree clean"
else
    fail "Working tree has uncommitted changes (run 'git status' to see)"
fi

# --- Check 4: Synced with remote --------------------------------------------

git fetch origin "${DEFAULT_BRANCH}" --quiet

LOCAL_SHA=$(git rev-parse HEAD)
REMOTE_SHA=$(git rev-parse "origin/${DEFAULT_BRANCH}")

if [[ "$LOCAL_SHA" == "$REMOTE_SHA" ]]; then
    pass "Synced with origin/${DEFAULT_BRANCH}"
else
    fail "Not synced with origin/${DEFAULT_BRANCH} (local: ${LOCAL_SHA:0:7}, remote: ${REMOTE_SHA:0:7})"
fi

# --- Check 5: No go.mod replace directives ----------------------------------

if grep -q '^replace' go.mod 2>/dev/null; then
    fail "go.mod contains replace directives (remove before release)"
else
    pass "No go.mod replace directives"
fi

# --- Check 6: Tag doesn't already exist -------------------------------------

if git rev-parse "$VERSION" >/dev/null 2>&1; then
    fail "Tag ${VERSION} already exists"
else
    pass "Tag ${VERSION} is available"
fi

# --- Check 7: Release checks pass -------------------------------------------

echo ""
info "Running make release-check..."
echo ""

if make release-check; then
    echo ""
    pass "release-check passed"
else
    echo ""
    fail "release-check failed (see output above)"
fi

# --- Execute or dry-run -----------------------------------------------------

echo ""

if [[ "$DRY_RUN" == "true" ]]; then
    warn "[dry-run] Would create annotated tag: ${VERSION}"
    warn "[dry-run] Would push tag to: origin"
    echo ""
    info "All checks passed. Run without --dry-run to release."
else
    info "Creating annotated tag ${VERSION}..."
    git tag -a "$VERSION" -m "Release ${VERSION}"

    info "Pushing tag to origin..."
    git push origin "$VERSION"

    echo ""
    pass "Done! Tag ${VERSION} pushed."
    info "GitHub Actions will handle the rest."
    info "Watch progress: https://github.com/ctx-hq/ctx/actions"
fi
