# Security Hardening Plan - whatsignal (Revised)

## Context

A 5-agent security audit uncovered one **critical** vulnerability (SSRF via HTTP redirect following) missed by the initial review, plus several medium/low fixes. The codebase has strong fundamentals (HMAC-SHA512 webhook verification, parameterized SQL, AES-GCM encryption, path traversal protection, SSRF URL validation). However, the URL validation is bypassed by Go's default redirect-following behavior, which is the top priority fix.

No `/admin` endpoint exists — the app exposes only 4 routes: `/health`, `/session/status`, `/metrics` (GET), and `POST /webhook/whatsapp` (HMAC-protected).

---

## Fix 1: Prevent SSRF via HTTP redirect following [CRITICAL]

**Why:** Go's default `http.Client` follows redirects automatically. Both the WhatsApp client (`pkg/whatsapp/client.go:52`) and media handler (`pkg/media/handler.go:60`) create clients without custom `CheckRedirect` policies. The `validateDownloadURL()` in `pkg/media/validate_url.go` only validates the *initial* URL — a malicious WAHA server or compromised endpoint could redirect to `http://169.254.169.254/latest/meta-data/` (AWS IMDS), internal Docker services, or other internal resources, completely bypassing SSRF protection.

**Approach:** Add a `CheckRedirect` function to both HTTP clients that validates each redirect target against the same allowlist logic used by `validateDownloadURL()`. Disable redirects entirely for the WhatsApp API client (API calls should never redirect to different hosts). For the media handler, validate each redirect URL against the allowlist before following.

**Files:**
- `pkg/whatsapp/client.go` — Add `CheckRedirect` to `http.Client` in `NewClient()` that returns `http.ErrUseLastResponse` (disable redirect following for API calls)
- `pkg/media/handler.go` — Add `CheckRedirect` to `http.Client` in `NewHandlerWithServices()` that calls `validateDownloadURL()` on each redirect target URL
- `pkg/media/validate_url.go` — Export `validateDownloadURL` or extract its logic into a reusable function that `CheckRedirect` can call (currently it's a method on `handler`)
- `pkg/whatsapp/client_test.go` — Test that redirect to internal IP is blocked
- `pkg/media/handler_test.go` — Test that media download redirect to internal IP is blocked

**Verify:** `go test ./pkg/whatsapp/... ./pkg/media/...`

---

## Fix 2: Add panic recovery middleware [MEDIUM]

**Why:** Neither `ObservabilityMiddleware` nor `WebhookObservabilityMiddleware` (`internal/middleware/observability.go`) includes a `recover()`. An unhandled panic in any handler crashes the entire server process. In production, a single malformed webhook payload that triggers an unexpected nil dereference would take the service offline.

**Approach:** Add a `RecoveryMiddleware` that wraps handlers with `defer recover()`, logs the panic with stack trace, and returns HTTP 500. Apply it as the outermost middleware in the server chain.

**Files:**
- `internal/middleware/recovery.go` — New file: `RecoveryMiddleware(logger)` with `defer func() { if r := recover(); r != nil { ... } }()`
- `cmd/whatsignal/server.go` — Apply `RecoveryMiddleware` as outermost middleware in the router chain
- `internal/middleware/recovery_test.go` — Test panic recovery returns 500, logs error

**Verify:** `go test ./internal/middleware/... ./cmd/whatsignal/...`

---

## Fix 3: Cap rate limiter memory growth [MEDIUM]

**Why:** The IP-based rate limiter (`cmd/whatsignal/security.go`) stores entries in an unbounded `map[string][]time.Time`. A distributed attack from many source IPs could grow this map unboundedly. The `cleanup()` method removes expired timestamps but IPs with no remaining timestamps are the only ones deleted — during a sustained attack, all attacking IPs still have valid timestamps.

**Approach:** Add a `maxTrackedIPs` field (default 10,000). In `cleanup()`, after removing expired timestamps, if the map still exceeds `maxTrackedIPs`, evict entries with the fewest remaining requests (least active IPs). This preserves rate limiting for active attackers while bounding memory.

**Files:**
- `cmd/whatsignal/security.go` — Add `maxTrackedIPs int` field to `RateLimiter`, add eviction logic in `cleanup()`; update `NewRateLimiter` to accept the cap
- `internal/constants/defaults.go` — Add `DefaultMaxTrackedIPs = 10000`
- `cmd/whatsignal/security_test.go` — Test that IPs are evicted when cap is exceeded

**Verify:** `go test ./cmd/whatsignal/...`

---

## Fix 4: Fix chat lock map full-reset race condition [MEDIUM]

**Why:** `internal/service/message_service.go:81-88` resets the entire `chatLockManager` map when it exceeds `MaxChatLocks` (1000). This drops mutexes for in-flight messages, creating a window where two goroutines processing messages for the same chat could both proceed without synchronization.

**Approach:** Track `lastUsed time.Time` per lock entry. In `cleanup()`, evict entries not used in the last 5 minutes instead of resetting the entire map. This preserves locks for active conversations.

**Files:**
- `internal/service/message_service.go` — Change `chatLockManager.locks` from `map[string]*sync.Mutex` to `map[string]*chatLock` where `chatLock` has `mu sync.Mutex` and `lastUsed time.Time`; update `getLock()` to set `lastUsed`; update `cleanup()` to evict stale entries
- `internal/constants/defaults.go` — Add `ChatLockEvictionMinutes = 5` if not already present
- `internal/service/message_service_test.go` — Test that stale locks are evicted, active locks are preserved

**Verify:** `go test ./internal/service/...`

---

## Fix 5: Redact health check error details [LOW]

**Why:** `/health` endpoint (`cmd/whatsignal/server.go:267-299`) returns raw `err.Error()` strings in JSON responses. These can leak internal hostnames, ports, connection strings, or driver error details to anyone who can reach the endpoint.

**Approach:** Remove `"error": err.Error()` from health response JSON. Log the actual error server-side via the logger. Return only `"status": "unhealthy"` in the response body.

**Files:**
- `cmd/whatsignal/server.go` — In `handleHealth()`, replace 3 instances of `"error": err.Error()` with server-side logging; keep only `"status": "unhealthy"` in the response map

**Verify:** `go test ./cmd/whatsignal/...`

---

## Fix 6: Limit io.ReadAll in Signal client error path [LOW]

**Why:** `pkg/signal/client.go:144` uses `io.ReadAll(resp.Body)` on error responses without a size limit. A misbehaving Signal CLI service could return an arbitrarily large error body, consuming memory.

**Approach:** Use `io.LimitReader(resp.Body, 4096)` to cap error body reads at 4KB.

**Files:**
- `pkg/signal/client.go` — Replace `io.ReadAll(resp.Body)` with `io.ReadAll(io.LimitReader(resp.Body, 4096))` on line 144

**Verify:** `go test ./pkg/signal/...`

---

## Out of Scope (with rationale)

- **API token for GET endpoints**: Feature request, not a security fix. The endpoints expose operational data (health, metrics, session status) but are typically behind a reverse proxy or firewall in production. Can be a separate PR.
- **Log injection** (newlines/ANSI in user values): logrus in JSON mode (production default) properly escapes these characters. Not exploitable in standard deployment.
- **OpenTelemetry `WithInsecure()`**: Standard practice for sidecar/local OTLP collectors in Docker Compose. The traces go to the configured collector, not exposed publicly.
- **Webhook replay within timestamp window**: Adding idempotency requires persistent storage (DB/cache) for seen message IDs. The 5-minute skew window combined with HMAC verification already provides strong protection. Low ROI for the complexity.
- **Integer overflow in config**: `ValidateTimeout()` checks `< 1`, catching negative values. `time.Duration` fields checked with `> 0` guard mean negative durations skip validation but behave identically to zero (no timeout). Not exploitable.
- **`.env` with secrets on disk**: In `.gitignore`, local-only. User should rotate secrets independently.
- **mTLS for internal services**: Architectural change, separate effort.

---

## Verification

After all fixes:
1. `go build ./...`
2. `go test ./...`
3. `go vet ./...`
4. `make ci` — must pass clean
5. Manual: confirm `handleHealth()` no longer includes error details in JSON response
6. Review: confirm `CheckRedirect` is applied to both HTTP clients
