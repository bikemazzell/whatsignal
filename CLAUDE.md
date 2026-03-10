# CLAUDE.md — Project instructions for Claude Code

## Release checklist

- Always check for the latest Go version and update ALL of the following before finishing a commit or release:
  - `go.mod` — `go` and `toolchain` directives
  - `Dockerfile` — base image `golang:X.Y.Z-alpine`
  - `.github/workflows/*.yml` — every `GO_VERSION` env var and any hardcoded `go-version:` values
- Run `go mod tidy` with the new toolchain after updating.
- After bumping Go, rebuild any locally installed Go tools (staticcheck, golangci-lint, etc.) with the new toolchain so local CI matches GitHub Actions.
- The `scripts/bump-version.sh` script creates and pushes a git tag (`vX.Y.Z`) along with the commit. Both the branch and the tag must be pushed to origin for a complete release.
