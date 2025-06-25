# Release Process

This document describes the release process for WhatSignal.

## Version Numbering

WhatSignal follows [Semantic Versioning](https://semver.org/):
- **MAJOR** version (X.0.0) - Incompatible API changes
- **MINOR** version (0.X.0) - New functionality, backwards compatible
- **PATCH** version (0.0.X) - Bug fixes, backwards compatible

Current version is stored in the `VERSION` file.

## Creating a Release

### 1. Update Version

```bash
# For bug fixes
make version-bump-patch

# For new features
make version-bump-minor

# For breaking changes
make version-bump-major
```

### 2. Update Changelog

Edit `CHANGELOG.md` to document:
- New features
- Bug fixes
- Breaking changes
- Security updates

### 3. Commit Changes

```bash
git add VERSION CHANGELOG.md
git commit -m "chore: bump version to v$(cat VERSION)"
```

### 4. Create Release Tag

```bash
make release-tag
# or manually:
git tag -a v$(cat VERSION) -m "Release v$(cat VERSION)"
```

### 5. Push to Repository

```bash
git push origin main
git push origin --tags
```

### 6. GitHub Release

The GitHub Actions workflow will automatically:
1. Build multi-architecture Docker images
2. Push images to GitHub Container Registry
3. Tag images with version numbers

### 7. Create GitHub Release

1. Go to GitHub Releases page
2. Click "Create a new release"
3. Select the version tag
4. Copy changelog entries for release notes
5. Attach deployment artifacts if needed

## Docker Image Tags

Each release creates multiple Docker image tags:
- `ghcr.io/bikemazzell/whatsignal:latest` - Latest stable release
- `ghcr.io/bikemazzell/whatsignal:1.0.0` - Specific version
- `ghcr.io/bikemazzell/whatsignal:1.0` - Minor version (latest patch)
- `ghcr.io/bikemazzell/whatsignal:main` - Latest main branch build

## Version Information

Version is available in multiple places:
- `whatsignal --version` - CLI flag
- `/health` endpoint - JSON response includes version
- Docker image labels
- Application startup logs

## Rollback Process

To rollback to a previous version:

```bash
# Update docker-compose.yml to specific version
sed -i 's/whatsignal:latest/whatsignal:0.54.0/g' docker-compose.yml

# Restart services
docker compose down
docker compose pull
docker compose up -d
```