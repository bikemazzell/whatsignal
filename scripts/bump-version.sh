#!/bin/bash

set -euo pipefail

VERSION_FILE="VERSION"
BRANCH=$(git rev-parse --abbrev-ref HEAD)

usage() {
    echo "Usage: $0 [patch|minor|major]"
    echo ""
    echo "Bumps the version, runs all quality checks, commits, tags, and pushes."
    echo "Defaults to 'patch' if no argument is given."
    exit 1
}

fail() {
    echo "FAILED: $1" >&2
    exit 1
}

step() {
    echo ""
    echo "--- [$1] $2 ---"
}

BUMP_TYPE=${1:-patch}

case $BUMP_TYPE in
    patch|minor|major) ;;
    -h|--help) usage ;;
    *) usage ;;
esac

if [ ! -f "$VERSION_FILE" ]; then
    fail "VERSION file not found"
fi

CURRENT_VERSION=$(cat "$VERSION_FILE")
IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT_VERSION"

case $BUMP_TYPE in
    patch) PATCH=$((PATCH + 1)) ;;
    minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
    major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
esac

NEW_VERSION="$MAJOR.$MINOR.$PATCH"

echo "=== Release: v$CURRENT_VERSION -> v$NEW_VERSION ($BUMP_TYPE) ==="

# -- Pre-flight checks --

step 1 "Checking working tree"
if ! git diff --quiet || ! git diff --cached --quiet; then
    fail "Working tree has uncommitted changes. Commit or stash them first."
fi

step 2 "Checking branch is up to date with origin"
git fetch origin "$BRANCH" --quiet 2>/dev/null || true
LOCAL=$(git rev-parse HEAD)
REMOTE=$(git rev-parse "origin/$BRANCH" 2>/dev/null || echo "$LOCAL")
if [ "$LOCAL" != "$REMOTE" ]; then
    fail "Local branch is not up to date with origin/$BRANCH. Pull or push first."
fi

# -- Quality checks (run BEFORE touching any files) --

step 3 "Building"
go build ./...

step 4 "Running go vet"
go vet ./...

step 5 "Checking formatting"
UNFORMATTED=$(gofmt -l . 2>/dev/null || true)
if [ -n "$UNFORMATTED" ]; then
    echo "$UNFORMATTED"
    fail "Files above need formatting (gofmt -s -w .)"
fi

step 6 "Running tests"
CGO_ENABLED=1 go test ./...

step 7 "Running race detector"
CGO_ENABLED=1 go test -race ./...

# Optional: lint and staticcheck if available
if command -v golangci-lint >/dev/null 2>&1 || [ -x "$(go env GOPATH)/bin/golangci-lint" ]; then
    step 7b "Running linter"
    LINT_BIN=$(command -v golangci-lint 2>/dev/null || echo "$(go env GOPATH)/bin/golangci-lint")
    "$LINT_BIN" run --timeout=5m ./...
fi

if command -v staticcheck >/dev/null 2>&1 || [ -x "$(go env GOPATH)/bin/staticcheck" ]; then
    step 7c "Running staticcheck"
    SC_BIN=$(command -v staticcheck 2>/dev/null || echo "$(go env GOPATH)/bin/staticcheck")
    PATH="$(go env GOPATH)/bin:$PATH" "$SC_BIN" ./...
fi

echo ""
echo "=== All checks passed ==="

# -- Bump version --

step 8 "Bumping version"
echo "$NEW_VERSION" > "$VERSION_FILE"

if grep -q "version.*$CURRENT_VERSION" README.md 2>/dev/null; then
    sed -i.bak "s/$CURRENT_VERSION/$NEW_VERSION/g" README.md && rm -f README.md.bak
    echo "  Updated README.md"
fi

if [ -f "docker-compose.yml" ] && grep -q "whatsignal:$CURRENT_VERSION" docker-compose.yml 2>/dev/null; then
    sed -i.bak "s/whatsignal:$CURRENT_VERSION/whatsignal:$NEW_VERSION/g" docker-compose.yml && rm -f docker-compose.yml.bak
    echo "  Updated docker-compose.yml"
fi

# -- Commit, tag, push --

step 9 "Committing"
git add -A
git commit -m "chore: bump version to v$NEW_VERSION"

step 10 "Tagging v$NEW_VERSION"
git tag -a "v$NEW_VERSION" -m "Release v$NEW_VERSION"

step 11 "Pushing to origin"
git push origin "$BRANCH"
git push origin "v$NEW_VERSION"

echo ""
echo "=== Released v$NEW_VERSION ==="
