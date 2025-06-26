#!/bin/bash

# Version bump script for WhatSignal
# Usage: ./bump-version.sh [patch|minor|major]

set -e

VERSION_FILE="VERSION"

if [ ! -f "$VERSION_FILE" ]; then
    echo "Error: VERSION file not found"
    exit 1
fi

CURRENT_VERSION=$(cat "$VERSION_FILE")
echo "Current version: $CURRENT_VERSION"

# Parse version components
IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT_VERSION"

# Determine bump type
BUMP_TYPE=${1:-patch}

case $BUMP_TYPE in
    patch)
        PATCH=$((PATCH + 1))
        ;;
    minor)
        MINOR=$((MINOR + 1))
        PATCH=0
        ;;
    major)
        MAJOR=$((MAJOR + 1))
        MINOR=0
        PATCH=0
        ;;
    *)
        echo "Usage: $0 [patch|minor|major]"
        exit 1
        ;;
esac

NEW_VERSION="$MAJOR.$MINOR.$PATCH"
echo "New version: $NEW_VERSION"

# Update VERSION file
echo "$NEW_VERSION" > "$VERSION_FILE"

# Update README.md if it contains version references
if grep -q "version.*$CURRENT_VERSION" README.md 2>/dev/null; then
    sed -i.bak "s/$CURRENT_VERSION/$NEW_VERSION/g" README.md && rm README.md.bak
    echo "Updated README.md"
fi

# Update docker-compose.yml if it exists and contains version references
if [ -f "docker-compose.yml" ] && grep -q "whatsignal:$CURRENT_VERSION" docker-compose.yml 2>/dev/null; then
    sed -i.bak "s/whatsignal:$CURRENT_VERSION/whatsignal:$NEW_VERSION/g" docker-compose.yml && rm docker-compose.yml.bak
    echo "Updated docker-compose.yml"
fi

echo ""
echo "Version bumped to $NEW_VERSION"
echo ""
echo "Next steps:"
echo "1. Commit changes: git add VERSION && git commit -m 'chore: bump version to v$NEW_VERSION'"
echo "2. Create tag: git tag -a v$NEW_VERSION -m 'Release v$NEW_VERSION'"
echo "3. Push changes: git push && git push --tags"