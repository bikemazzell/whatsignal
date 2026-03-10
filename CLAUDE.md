# CLAUDE.md — Project instructions for Claude Code

## Release checklist

- Always check for the latest Go version and update `go.mod` (`go` and `toolchain` directives) and `Dockerfile` (base image `golang:X.Y.Z-alpine`) before finishing a commit or release.
- Run `go mod tidy` after updating the toolchain.
