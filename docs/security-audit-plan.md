# WhatSignal Security Audit Plan

Last updated: 2026-04-19

## Scope

Assess WhatSignal's exposed HTTP service, webhook authentication, media download path, configuration handling, deployment artifacts, Docker hardening, logging/privacy controls, and production runtime defaults.

Out of scope unless explicitly authorized: attacking third-party WAHA or Signal CLI services, scanning external networks, credential guessing against live systems, and destructive testing against production data.

## Reviewed Assumptions

- The trusted upstream service endpoints are the configured WAHA base URL and Signal RPC URL, not arbitrary Docker network hostnames.
- Media URLs can be attacker-controlled indirectly through WAHA responses, so URL validation must defend before any fetch and again on redirects.
- Production mode is indicated by `WHATSIGNAL_ENV=production`.
- Host-published WAHA and Signal CLI ports are unsafe by default because Signal CLI has no built-in auth and WAHA is only protected by its API key.
- Docker image tags without digests are mutable and should be treated as a supply-chain risk.

## Corrected Plan

Each issue below must follow TDD: write the failing security test first, confirm it fails while the issue is present, implement the smallest fix, then rerun the focused test and broader regression suite.

| ID | Task | Analysis Type | Test Gate | Quality Check | Status |
| --- | --- | --- | --- | --- | --- |
| SEC-01 | Enforce production encryption salts and separate lookup salt | Static + unit | Production config rejects missing or duplicate salts | `go test ./internal/config` | Done |
| SEC-02 | Bind WAHA webhook HMAC to timestamp with replay skew | Static + integration | Valid timestamped signature passes; stale, missing, and body-replay signatures fail | `go test ./cmd/whatsignal` | Done |
| SEC-03 | Require admin token for production admin endpoints | Unit + integration | `/metrics` and `/session/status` reject missing or bad token in production | `go test ./cmd/whatsignal` | Done |
| SEC-04 | Harden media URL SSRF validation | Static + unit | Blocks loopback, RFC1918, link-local, IPv6 loopback, redirects, decimal/hex/octal IPs, and DNS resolutions to blocked IPs | `go test ./pkg/media` | Done |
| SEC-05 | Restrict Docker single-label host allowlist | Static + unit | Only configured WAHA and Signal RPC hosts are allowed; arbitrary `redis`, `postgres`, or `internal-api` hosts fail | `go test ./pkg/media` | Done |
| SEC-06 | Add Dockerfile static security tests | Static | Builder and runtime images are digest-pinned; final image uses nonroot user; final stage has no package manager or shell install step | `go test ./internal/config` | Done |
| SEC-07 | Add Compose static security tests | Static | `whatsignal` has `read_only`, `cap_drop: ALL`, `no-new-privileges`; WAHA and Signal CLI do not publish host ports by default | `go test ./internal/config` | Done |
| SEC-08 | Fix Compose runtime user mismatch | Static | Compose does not override the distroless `nonroot:nonroot` runtime user with UID 1000 | `go test ./internal/config` | Done |
| SEC-09 | Reject placeholder production API key | Unit + static | Production rejects `WHATSAPP_API_KEY=your-api-key`; Compose does not inject placeholder secrets | `go test ./cmd/whatsignal ./internal/config` | Done |
| SEC-10 | Replace brittle oversized-body string detection | Unit | Wrapped `*http.MaxBytesError` returns HTTP 413; plain matching text does not trigger 413 | `go test ./cmd/whatsignal` | Done |
| SEC-11 | Run static security tooling | Static | `go vet`, `govulncheck`, `gosec`, and `staticcheck` complete without actionable findings | Tool-specific commands | Done |
| SEC-12 | Run full regression suite | Dynamic | All packages pass after changes | `go test ./...` | Done |

## Active Testing Checklist

- Unit tests for config, security helpers, URL validation, and request body handling.
- Integration-style handler tests for webhook authentication and admin endpoint authorization.
- Static deployment tests for `Dockerfile`, `docker-compose.yml`, scripts, and example environment files.
- Smoke-level verification through full package test execution and Go static analysis.

## Quality Gates

Before calling the audit implementation complete:

- `go test ./... -count=1`
- `go vet ./...`
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
- `go run github.com/securego/gosec/v2/cmd/gosec@latest ./...`
- `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`
- `git diff --check`

## Open Follow-Ups

- Decide whether to add an explicit development override Compose file that publishes WAHA and Signal CLI ports for local debugging only.
- Pin third-party WAHA and Signal CLI images by digest if production deployments use this Compose file directly.
