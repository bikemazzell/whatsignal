# WhatSignal — Unified Code Review & Remediation Plan

**Date:** 2026-06-17 (revised — second `qwen` pass merged)
**Inputs:** independent reviews (`codex`, `glm`, two `qwen` passes; the first `qwen` was byte-identical to `deepseek`). `codex` and `glm` shipped reproducible tests; the second, more detailed `qwen` pass added findings merged below.
**Method:** every unique claim was re-validated against the actual code on disk (file:line evidence), not taken on the reviews' word. Claims that failed scrutiny are listed in the Rejected section with the reason. Items added in the second `qwen` merge are tagged **[qwen-2]**.

---

## How to read this

Each item carries: the originating review IDs, the verified location, a one-line verdict, the fix, effort (S/M/L), and the test that proves it. Items are ordered by ROI within phases, not by original severity label.

Verdicts use: **CONFIRMED** (reproduced against code), **PARTIAL** (true with caveats), **REJECTED** (claim does not hold).

---

## Execution plan

The findings below are the verified reference. This section makes them executable: decisions to settle first, a sequenced PR breakdown, the items that are spikes rather than tasks, and a per-PR exit gate. Each PR cites finding IDs (`P1.1` …) defined later.

### Decisions applied (defaults — override before starting)

Sensible defaults are chosen so work can begin; the two consequential ones are flagged. Change here, not mid-PR.

| # | Decision | Default chosen | Notes |
|---|---|---|---|
| D1 | Secure-mode model | Keep `WHATSIGNAL_ENV`; treat **unset as production (fail-closed)**. Insecure/dev behavior requires explicit `WHATSIGNAL_ENV=development`. | Replaces every `!= "production"` fail-open check (P1.2, P1.3) with one shared `isSecureMode()` helper. |
| D2 | Trusted-proxy config | New `WHATSIGNAL_TRUSTED_PROXIES` = comma-separated CIDRs; empty ⇒ trust no forwarded headers, use `RemoteAddr`. | Honor `X-Forwarded-For`/`X-Real-IP` only when `RemoteAddr` ∈ trusted set (P1.1). |
| D3 | Admin token | Enforce `MinAdminTokenLength = 32`, validated at startup when set; required in secure mode. | Mirrors `MinWebhookSecretLength` (P3.10 bullet). |
| D4 | Webhook skew + replay | Reduce `DefaultWebhookMaxSkewSec` 600 → 120; replay-cache TTL = skew + buffer. | P2.1. |
| D5 | **Dead code: delete** | Remove `internal/features`, `internal/versioning`, `models.Encryptor`, unused `errors/helpers.go` constructors, the no-op `webhook.go` API, the legacy `timestamp.body` signature path. | **Consequential but reversible** (git). Project favors simplicity. Veto here if you want any kept. (P3.1, P3.6) |
| D6 | **`EncryptForLookup` migration** | **Defer.** Ship the fail-closed-salt fix now (P2.5a); the `LookupHash` data migration (P2.5b) is a separate spike. | **Consequential**: P2.5b rewrites indexed encrypted columns. Don't block the security PR on it. |
| D7 | `messageService.mu` narrowing (H3) | **Spike first** — measure with a throughput benchmark before changing locking. | Behavior-changing; concurrency regressions have prod history (CLAUDE.md). |
| D8 | Outbound media size | Enforce a hard cap (`MaxRecommendedFileSizeBytes` becomes a real limit) now; streaming refactor deferred to a spike. | P1.10 immediate vs S4. |

### PR breakdown (sequenced)

Order matters where noted. PR-1 lands the guardrails (-race gate, honest mocks) before any concurrency code is touched.

| PR | Theme | Items | Depends on | Blast radius |
|---|---|---|---|---|
| **PR-1** | Test & CI foundation | P4.1, P4.2, P4.9 | — | none (test/CI only) |
| **PR-2** | Secure-mode fail-closed (D1, D3) | P1.2, P1.3, P3.10·admin-token, P2.5·salt (fail-closed only, D6) | PR-1 | config/startup behavior |
| **PR-3** | Network-edge hardening (D2) | P1.1, P1.8, P2.16, P1.9, P2.8, P2.6 | PR-1 | external request handling — **canary** |
| **PR-4** | Toolchain & deps | P1.4, P3.10·compose-pin | — | build/deps |
| **PR-5** | Concurrency & lifecycle | P1.5, P1.6, P1.7, P2.3, P2.11, P2.19, P2.4·minimal | PR-1 | runtime — **canary**; reproducer-first |
| **PR-6** | Webhook integrity & resource bounds (D4) | P2.1, P2.2, P2.12, P2.10, P2.13, P3.10·readall-caps, P1.10·cap (D8) | PR-1 | webhook/data lifecycle |
| **PR-7** | Small correctness fixes | P2.7, P2.9, P2.14, P2.15, P2.17, P2.18, P3.5, P3.9, P4.7, P4.8, P4.11 | — | localized |
| **PR-8** | Test-quality sweep (mechanical) | P4.3, P4.4, P4.5, P4.6 | PR-1 | tests only |
| **PR-9** | Consolidation & dead-code (D5) | P3.1, P3.6, P3.2 (absorbs P2.4), P3.8, P3.3, P3.4, P3.11 | PR-5 | wide refactor — land last |

**Remaining P3.10 bullets** (WS-frame logging, query-param logging, `sessionName` escaping, `att.ID` rejection, empty-string encryption, CSP/HSTS/XSS header cleanup, generic signature-error response, body draining, `generateUniqueID` hardening, raw-debug gating): attach each to whichever PR above already touches that file; sweep any leftovers into a final **PR-10 hygiene** pass. They are intentionally low-priority and must not block PR-1…PR-9.

### Spikes (investigation tasks, not yet implementable)

Each produces a short design note + decision before it becomes a PR. Do **not** fold into the mechanical PRs. **Full design notes (code-grounded) are in [Appendix A](#appendix-a--spike-design-notes-s1-s6); summary and post-investigation status below.**

- **S1 — P2.5b** `EncryptForLookup` migration. **Scope grew on investigation: L, not the simple swap first assumed** — all 5 target columns are also decrypted, so they need parallel hash columns + random-nonce re-encryption, not a drop-in `LookupHash`. Phase per column.
- **S2 — H3** `messageService.mu` narrowing. **M, well-bounded** — `mu` guards only DB calls; 8/10 sites drop safely, the one check-then-act (`UpdateDeliveryStatus`) becomes an atomic conditional `UPDATE`. Do P2.17 first; benchmark before/after.
- **S3 — P3.7** O(sessions) routing. **M, low-risk** — collapse N per-session history queries into one `SELECT DISTINCT session_name … IN (…)`.
- **S4 — P1.10** media streaming. **Gate resolved (web):** WAHA's send endpoints accept a remote `url` *or* base64 `data` (no multipart); signal-cli `/v2/send` is base64-only. So the WhatsApp direction can hand WAHA a `url` and skip loading the file entirely (when the media is HTTP-reachable from WAHA's network); the Signal direction must base64, so stream the *encode* via a custom body `io.Reader` to cut peak ~7.7× → ~1×. Defer behind P1.10's hard cap.
- **S5 — P4.10** drop CGO via `modernc.org/sqlite`. **M, low-risk** — sqlite is the only C dep, only side-effect import, all PRAGMAs supported; swap driver name + flip `CGO_ENABLED=0`, then benchmark both drivers.
- **S6 — ~~timestamp precision~~ RESOLVED, no action** — the skew/replay check uses the millisecond `X-Webhook-Timestamp` header, not the truncated `FlexibleTimestamp`. Closed.

### Per-PR exit gate (Definition of Done)

Every PR above must satisfy all of:

1. **Reproducer-first.** A failing test (or `-race` reproducer for concurrency PRs) is committed before the fix; it goes green with the fix.
2. **`make ci` zero failures**, and `go test -race ./...` clean (trustworthy once PR-1 lands).
3. **Named regression tests** listed in the PR description, asserting the behavior — not just "no error" (CLAUDE.md: assert the thing you care about).
4. **No unrelated working-tree changes** swept in (release-hygiene lesson); version bump only at release, as the last commit.
5. For **canary** PRs (PR-3, PR-5): a stated rollback (env/flag or revert) and a monitoring note before deploy.

---

## Rejected / downgraded claims (validated as not actionable)

These were checked and deliberately excluded so effort is not spent on non-issues:

| Claim (source) | Why rejected |
|---|---|
| `SelectLatestMessageMappingQuery` is dead (qwen/deepseek L4) | **FALSE** — used at `internal/database/database.go:534`. |
| SQLite `busy_timeout` set on only one pooled connection (asserted by 3 reviewers) | **DISPROVED** by glm's own test: `mattn/go-sqlite3` applies `busy_timeout=5000` to every connection by default; a 4-writer contention test produced 0 `SQLITE_BUSY`. Switching to a DSN param is nicer but not a bug. |
| "Remove `messageService.mu` entirely" (qwen/deepseek H3, glm L1, codex #7) | **Downgraded.** The mutex *is* a real throughput bottleneck (confirmed — it wraps `db.*` calls), but blind deletion is unsafe: any check-then-write dedup sequence may rely on it. Plan narrows it to in-memory state behind a throughput regression test, not "delete it." |
| "Flip the default-true retryable classifier" (qwen/deepseek M5) | **Downgraded.** Defaulting unknown errors to *retry* is a defensible fail-safe. The real defect is fragile substring matching duplicated 5×; addressed under A7 consolidation, not by flipping the default. |
| Dead-code line counts (~1,200 / ~3,800 LOC) | **Corrected.** Actual non-test dead LOC: `internal/features` = 591, `internal/versioning` = 704. Real but smaller than claimed. |
| Pending-persistence layer has "zero" test coverage (qwen/deepseek L11) | **PARTIAL** — the DB functions are exercised via service-layer mocks, but the real `database.go` implementations lack direct DB-level tests. Kept as a Phase-3 test gap, not "zero coverage." |
| Phone number logged in plaintext at `bridge.go:1493` (qwen-2 M13) | **FALSE** — every phone log in `bridge.go` routes through `SanitizePhoneNumber` (e.g. `:569,:654,:678`). |
| Migration 001 data loss on crash re-run (qwen-2 M3) | **FALSE** — `001_initial_schema.sql:26-57` creates the temp table first, copies with `INSERT OR IGNORE ... WHERE EXISTS`, then `DROP TABLE IF EXISTS` + rename; all steps idempotent. |
| Context-key mismatch dead code (qwen-2 H10) | **FALSE** — `errors/helpers.go` and `tracing/tracing.go` use the same string values; no value is set under one key and missed by the other. Poor API factoring, not a bug. |
| `math/rand` without seed (qwen-2 L2) | **FALSE** — Go 1.20+ auto-seeds the global source; project is on 1.26.x. Non-issue. |
| Unused `Encryptor` struct is a *defect* (qwen-2 L6) | **Reclassified** — it is genuine dead code (`internal/models/encryption.go:15`, zero refs) but harmless; folded into P3.6, not a correctness finding. |
| Webhook handler stubs silently drop messages (qwen-2 L19) | **Reclassified** — `handleTextMessage`/`handleImageMessage` (`pkg/whatsapp/webhook.go:63-70`) are no-ops, but the `RegisterEventHandler` API is referenced only by `interfaces.go` and is **not on the live webhook route** (production handling is in `server.go`). Dead/unused API, not a live silent-drop. Folded into P3.6. |
| Legacy WAHA signature fallback "weakens security" (qwen-2 H9) | **Downgraded** — the `timestamp.body` fallback (`security.go:69-78`) was the *broken verifier* from v1.2.47; real WAHA signs raw body (proven by the contract test), so it matches nothing and still requires the secret. Dead complexity to remove (P3.6), not a security weakening. |

---

## Phase 1 — Production safety (do first)

### ✅ P1.1 Rate-limit IP spoofing via forwarded headers
**Sources:** qwen/deepseek C1, codex #2, glm S3 · **CONFIRMED**
**Where:** `internal/httputil/clientip.go:14-26` (trusts `X-Forwarded-For` then `X-Real-IP` before `RemoteAddr`, no trusted-proxy gate); consumed at `cmd/whatsignal/server.go:189`.
**Impact:** default `docker-compose` publishes the port directly with no proxy, so headers are fully attacker-controlled → rate-limit bypass (rotate IPs) and victim-IP DoS.
**Fix:** add a `TRUSTED_PROXIES` (CIDR list) config. Only honor `X-Forwarded-For`/`X-Real-IP` when `RemoteAddr` is in the trusted set; otherwise use the socket peer. **Effort: S.**
**Tests:** direct client with spoofed XFF → uses `RemoteAddr`; trusted proxy → uses first forwarded IP; untrusted client `X-Real-IP` cannot override.

### ✅ P1.2 Webhook HMAC fails open outside production
**Sources:** glm S2 · **CONFIRMED**
**Where:** `cmd/whatsignal/security.go:30-34` — empty `secretKey` + `WHATSIGNAL_ENV != "production"` returns the body with **no signature check**. `WHATSIGNAL_ENV` defaults to unset.
**Impact:** an operator who skips the env step silently accepts unsigned webhook traffic.
**Fix:** fail closed. Require an explicit opt-in (`WHATSIGNAL_ENV=development`) to run without a secret; otherwise refuse to start. **Effort: S.**
**Test:** unset env + empty secret → startup error (or 401 on webhook), not pass-through.

### ✅ P1.3 Admin endpoints unauthenticated outside production
**Sources:** qwen/deepseek M12, glm S6 · **CONFIRMED**
**Where:** `cmd/whatsignal/security.go:99-102` — `requireProductionAdminToken` returns `true` whenever `WHATSIGNAL_ENV != "production"`. Guards `/metrics` (`server.go:122`) and `/session/status` (`server.go:341`).
**Impact:** `/metrics` (session names, counts, timings) and `/session/status` exposed in any non-prod (incl. unset, "staging", "prod") env.
**Fix:** same fail-closed model as P1.2 — require the admin token whenever it is set, and treat unset-in-prod as a hard error. Tie env semantics to a single helper so P1.2/P1.3 share one definition of "secure mode." **Effort: S.**

### ✅ P1.4 Go toolchain has reachable stdlib vulnerabilities
**Sources:** codex #1 · **CONFIRMED (version)**
**Where:** `go.mod:3` and `Dockerfile:2` pin `go 1.26.3`. `govulncheck` reported `GO-2026-5039` (`net/textproto`) and `GO-2026-5037` (`crypto/x509`), reachable via Signal client HTTP/TLS paths, fixed in 1.26.4.
**Fix:** per the project release checklist, bump `go`/`toolchain` in `go.mod`, `Dockerfile` base image, and every `GO_VERSION` in `.github/workflows/*.yml` to the latest patch; `go mod tidy`; rebuild; rerun `go test ./...`, `go test -race ./...`, `govulncheck ./...`. **Effort: S.**
> **Verified (web):** Go **1.26.4** (released **2026-06-02**) fixes both — GO-2026-5039 / CVE-2026-42507 (`net/textproto`) and GO-2026-5037 / CVE-2026-27145 (`crypto/x509` quadratic `VerifyHostname`), plus CVE-2026-42504 (`mime`). Exact edit sites: `go.mod:3`, `Dockerfile:2` (`golang:1.26.4-alpine` + refresh digest), `.github/workflows/security.yml:15`, `.github/workflows/integration-tests.yml:26`. Re-confirm latest patch at implementation time.

### ✅ P1.5 `/metrics` data race (deterministic under `-race`)
**Sources:** glm H2 · **CONFIRMED** (glm reproduced under `-race`)
**Where:** `internal/metrics/metrics.go:169-196` — `GetAllMetrics` copies map *references*; `result.Timers[key] = timer` (≈line 188) hands out pointers to live `TimerMetric` structs that `RecordTimer` (line 106) mutates while `/metrics` JSON-encodes them.
**Impact:** any `/metrics` scrape concurrent with one webhook races; worst case a torn read aborts the response mid-write (recovery middleware does not cover the encoder).
**Fix:** deep-copy each `Metric`/`TimerMetric` (scalar struct copy + explicit `samples` slice copy) under the read lock before returning. **Effort: S.**
**Test:** keep glm's `-race` reproducer.

### ✅ P1.6 `Stop()` panics on second call; no goroutine wait
**Sources:** qwen/deepseek C3+C5, glm H1 · **CONFIRMED**
**Where:** `internal/service/delivery_monitor.go:57` and `internal/service/scheduler.go:57` — `close(stopCh)` with no `sync.Once` (panic on double-close) and no `WaitGroup` (returns before the loop exits). The correct pattern already exists at `cmd/whatsignal/security.go:180-187` (RateLimiter: `sync.Once` + `WaitGroup`).
**Impact:** double-`Stop()` → "close of closed channel" panic; on shutdown, `db.Close()` (LIFO defer in `main.go`) can run before an in-flight `CleanupOldRecords`, yielding `sql: database is closed`.
**Fix:** copy the RateLimiter pattern into both monitors. **Effort: S.**
**Tests:** `Stop()` twice → no panic; `Stop()` blocks until the loop goroutine returns.

### ✅ P1.7 `SessionMonitor.Stop()` does not wait for its goroutine
**Sources:** qwen/deepseek C4 · **CONFIRMED**
**Where:** `internal/service/session_monitor.go:87-101` closes `stopCh` (and nils it) but never waits for `monitorLoop`. A `Stop()`→`Start()` sequence can leave two `monitorLoop`s running.
**Fix:** add `sync.WaitGroup`; `Start` does `wg.Add(1)`, `monitorLoop` defers `wg.Done()`, `Stop` calls `wg.Wait()`. **Effort: S.**

### ✅ P1.8 Missing `ReadHeaderTimeout` (slowloris)
**Sources:** glm S7 · **CONFIRMED**
**Where:** `cmd/whatsignal/server.go:157-163` sets Read/Write/Idle timeouts but not `ReadHeaderTimeout`. The integration env already sets it (`integration_test/environment.go:910`).
**Fix:** add `ReadHeaderTimeout` (e.g. 10s) to the production `http.Server`. **Effort: S.** **Test:** static assert the field is set (glm has one).

### ✅ P1.9 Unbounded `io.Copy` on media download → disk-exhaustion DoS
**Sources:** codex #3, glm S9, qwen/deepseek M2(adjacent) · **CONFIRMED**
**Where:** `pkg/media/handler.go:254` — `io.Copy(tempFile, resp.Body)` with no cap; size check runs only after the full body is on disk. The Signal in-memory path does this correctly with `io.LimitReader` (`pkg/signal/client.go:831`). `docker-compose.yml` mounts a 100 MB tmpfs on `/tmp`.
**Fix:** `io.Copy(tempFile, io.LimitReader(resp.Body, maxBytes+1))`, then reject and `os.Remove` if the copied count exceeds the limit. Use `Content-Length` only as an early reject. **Effort: S.**
**Test:** stream a response larger than the limit; assert the temp file never exceeds `maxBytes+1` and is removed.

### ✅ P1.10 Media send: full-file read + base64 + JSON in memory
**Sources:** qwen/deepseek M2, codex #5, glm R5 · **CONFIRMED** (glm benchmarked 383 MB peak for a 50 MB file)
**Where:** `pkg/whatsapp/client.go:423-441` — warns above `MaxRecommendedFileSizeBytes` but continues, then `os.ReadFile` → `base64.StdEncoding.EncodeToString` → JSON marshal. Mirror at `pkg/signal/client.go:894-901`.
**Impact:** ~7.7× file-size peak heap; 5 concurrent 50 MB sends ≈ 1.9 GB RSS → OOM in a default container.
**Fix (tiered):** (a) immediate — turn the warning into an *enforced* hard limit before `os.ReadFile`, failing fast; (b) better — stream via `io.Pipe` + `base64.NewEncoder` + `json.NewEncoder`, or multipart upload if WAHA supports it. **Effort: M.**
**Test:** allocation benchmark asserting peak ≈ file size, not 7×.

---

## Phase 2 — Correctness & resource hygiene

### ✅ P2.1 Webhook replay protection + over-wide skew window
**Sources:** glm S1, qwen/deepseek M9 · **CONFIRMED**
**Where:** no nonce/jti/seen-timestamp dedup anywhere in webhook handling (grep confirms only AES-GCM nonces exist); `DefaultWebhookMaxSkewSec = 600` (`internal/constants/defaults.go:44`).
**Impact:** a captured webhook can be replayed verbatim for 10 min — re-forwarding messages or corrupting `message.ack` delivery state.
**Fix:** short-TTL cache of seen `(hash(body), timestamp)` or `(messageID, eventTime)`, reject repeats with 409; reduce skew to ~60-120s (TTL ≥ skew). **Effort: M.**
**Test:** identical signed webhook twice → second returns 409.

### ✅ P2.2 `pending_signal_messages` grows unbounded
**Sources:** glm R3, qwen/deepseek L8 · **CONFIRMED**
**Where:** only deletion is `DELETE ... WHERE message_id_hash=? AND destination=?` after success (`queries.go:172`); `IncrementPendingRetryCount` (`queries.go:177`) has no ceiling; `CleanupOldRecords` (`database.go:730`) only touches `message_mappings`.
**Fix:** add `DELETE FROM pending_signal_messages WHERE retry_count > ? OR created_at < ?` to the daily scheduler, and enforce a max retry count (drop/dead-letter past it). **Effort: S.** **Test:** rows past retry cap / age are purged by cleanup.

### ✅ P2.3 Unsynchronized cached fields read from HTTP/poller goroutines
**Sources:** qwen/deepseek H1+H2, glm H3-H5 · **CONFIRMED**
**Where:**
- `pkg/whatsapp/client.go:900-932` — `supportsVideo *bool` read/written in `checkVideoSupport` with no lock.
- `pkg/signal/client.go:52-54` — `initialized`/`initError`/`detectedMode` written in `InitializeDevice` (762-764), read lock-free by `IsInitialized`/`InitializationError`/`DetectedMode` (HTTP handlers).
- `internal/service/contact_service.go:136` and `group_service.go:115` — `degradedMode bool` written from webhook goroutines without sync.
**Fix:** one `sync.RWMutex` per struct, or `atomic.Pointer`/`atomic.Bool` for the scalars. **Effort: S.** **Test:** `-race` test crossing writer/reader for each.

### ✅ P2.4 Circuit breaker mixed atomic/non-atomic access (data race)
**Sources:** qwen/deepseek C2, glm A5 · **CONFIRMED**
**Where:** `internal/service/circuit_breaker.go` — `atomic.AddUint32(&cb.failures,1)` at line 122 vs non-atomic reads/writes under the mutex (128, 132, 157, 165) and an atomic read with **no lock** at line 76 (`Execute`). Mixed atomic/non-atomic on one variable is a race per the Go memory model.
**Fix:** pick one discipline. Since every other access is already under `mu`, drop the atomics and use plain field access under the lock (and stop the lock-free read at line 76 — take the lock or expose a locked getter). **Effort: S.** Folds into P3.2 (CB consolidation).

### ✅ P2.5 Deterministic lookup encryption + public default salt, fail-open
**Sources:** qwen/deepseek M13, glm S4, codex #12 · **CONFIRMED**
**Where:** `internal/database/encryption.go:133-153` `EncryptForLookup` derives the nonce from `sha256(plaintext+salt)` → identical plaintext yields identical ciphertext (equality leak). Default `EncryptionLookupSalt = "whatsignal-lookup-salt-v1"` (`constants/defaults.go:209`) is public and used silently when the env salt is unset (only enforced in production). A keyed `LookupHash` (HMAC-SHA256) already exists (`encryption.go:66`) and is the correct searchable-encryption primitive.
**Fix (tiered):** (a) immediate — fail closed when the lookup salt is unset (don't silently use the repo default); (b) longer-term — migrate indexed columns from `EncryptForLookup` to `LookupHash` and keep random-nonce ciphertext only where plaintext recovery is needed. Note (b) is a data migration across 5 call sites → schedule deliberately. **Effort: S now / L migration.**
Also fix the misleading `#nosec` comment at `encryption.go:86` ("Deterministic nonce required for searchable encryption") — it sits on the **random-nonce** `Encrypt` function and is false there (codex #12).

### ✅ P2.6 Lenient path validator on the media file boundary
**Sources:** glm S5 · **CONFIRMED (PARTIAL threat)**
**Where:** `internal/security/path.go:10-35` `ValidateFilePath` accepts absolute paths (`/etc/passwd`); the strict variants `ValidateFilePathStrict` (`:68-79`) and `ValidateFilePathWithBase` (`:38-65`) exist with **zero production callers**. `pkg/media/handler.go:120` uses the lenient one, and a `Media.URL` with no scheme/host (or `file://`) routes to a local file read.
**Threat model:** the field is WAHA-server-controlled today, so this is defense-in-depth — but it is exactly what the unused strict validators were written for.
**Fix:** wire `ValidateFilePathWithBase` (constrained to `cacheDir`/`signalAttachmentsDir`) at the media boundary; fix its prefix-match bug first (glm's `TestZZReview_ValidateFilePathWithBasePrefixBug`: `/app/cache_evil` accepted under `/app/cache`). This also consumes the "unused strict validators" dead-code finding (qwen/deepseek A4). **Effort: S.**

### ✅ P2.7 `/health` returns 200 when degraded
**Sources:** glm S8, codex #10 · **CONFIRMED**
**Where:** `cmd/whatsignal/server.go:324-328` — `case "degraded": WriteHeader(StatusOK)`. The Dockerfile `HEALTHCHECK` calls `/health`, so a half-down bridge (Signal down) never gets restarted. The same endpoint also runs dependency checks, so public polling generates upstream traffic (codex #10).
**Fix:** split cheap liveness (`/healthz` → 200 if process up) from readiness (`/readyz` → 503 when any dependency is down, incl. "degraded"); point the container/k8s healthcheck at readiness. **Effort: S.**

### ✅ P2.8 Signal attachment streaming-to-disk is uncapped
**Sources:** codex #4 · **CONFIRMED**
**Where:** `pkg/signal/client.go:875` `DownloadAttachmentToFile` uses `io.Copy(file, resp.Body)` with no cap; normal attachment saving routes through it (≈line 687). (The in-memory `DownloadAttachment` path is correctly capped.)
**Fix:** wrap with `io.LimitReader` at the same per-media limit, remove the partial file on overflow. **Effort: S.** Pairs with P1.9.

### ✅ P2.9 `context.Background()` in the reply-routing hot path
**Sources:** qwen/deepseek H6 · **CONFIRMED**
**Where:** `internal/service/bridge.go:990` — `ctx := context.Background()` for `GetContactByName` inside `extractMappingFromQuotedText`, ignoring the caller's deadline/cancellation.
**Fix:** thread the caller's context (or derive a short-deadline child). **Effort: S.**

### ✅ P2.10 `RecordTimer` sorts the full sample slice on every call under the global lock
**Sources:** glm R1 · **CONFIRMED** (glm: 13.5 µs/op)
**Where:** `internal/metrics/metrics.go:106-150` holds `r.mu.Lock()` and calls `calculatePercentile` twice (P95+P99), each `sort.Float64s` (`:230`); `metricKey` also sorts labels + `fmt.Sprintf` per call.
**Impact:** ~10 calls/webhook on one shared mutex; meaningful CPU at load.
**Fix:** maintain min/max/sum/count online; compute percentiles lazily at snapshot time. **Effort: M.**

### ✅ P2.11 `chatLockManager` unbounded below threshold + can evict a held lock
**Sources:** qwen/deepseek M7, glm M1 · **CONFIRMED (PARTIAL on eviction race)**
**Where:** `internal/service/message_service.go:78-103` — `cleanup()` returns early when `len(locks) <= MaxChatLocks`, so the map grows unbounded under the cap; and `getLock` returns `&cl.mu`, so a cleanup that deletes an entry whose mutex is still held breaks per-chat serialization (narrow: needs a >5 min hold while map >1000).
**Fix:** run periodic time-based eviction regardless of size, and skip entries with a non-zero in-use refcount (or guard deletion against currently-held locks). **Effort: M.**

### ✅ P2.12 Media download temp-file leak on success/timeout race
**Sources:** qwen/deepseek M6 · **CONFIRMED (narrow)** — re-verified after an initial agent mis-call
**Where:** `pkg/signal/client.go:586-621`. `downloadChan` is buffered(1): on success the goroutine's send never blocks, so `filePath` lands in the buffer; if the outer `select` happens to pick `<-downloadCtx.Done()` over the now-ready `<-downloadChan`, the written file is orphaned (nobody reads it). (The pure-error path *is* cleaned up by `DownloadAttachmentToFile`'s `os.Remove`.)
**Fix:** in the `downloadCtx.Done()` arm, drain `downloadChan` non-blocking and `os.Remove` any path found. **Effort: S.**

### ✅ P2.13 `http_requests_active` counter used as a gauge
**Sources:** glm R2 · **CONFIRMED**
**Where:** `internal/middleware/observability.go:78-81` increments then `AddToCounter(-1)`; `SetGauge` exists (`metrics.go:153`). Decrementing counters breaks Prometheus-style consumers.
**Fix:** use `SetGauge` (or a dedicated inc/dec gauge). **Effort: S.**

### ✅ P2.14 Reaction fallback can target the wrong chat
**Sources:** qwen/deepseek H7 · **CONFIRMED**
**Where:** `internal/service/bridge.go:1366-1376` — on Signal-ID miss, falls back to `GetLatestMessageMappingBySession` regardless of chat, so a reaction can apply to a message in a different active chat.
**Fix:** at minimum log a warning when the fallback is ambiguous; ideally scope the fallback to the reaction's originating chat. **Effort: S-M.**

### ✅ P2.15 `log_level` config silently capped at Info
**Sources:** qwen/deepseek H8 · **CONFIRMED**
**Where:** `cmd/whatsignal/main.go:395-398` — `if level > logrus.InfoLevel { level = logrus.InfoLevel }`, so `config.json` `log_level: "debug"` is ignored unless `-verbose`.
**Fix:** respect the configured level (it's already validated elsewhere), or remove the field and document `-verbose` as the only switch. Don't ship a dead config knob. **Effort: S.**

### ✅ P2.16 SSRF: fail-open validation gaps + DNS-rebinding **[qwen-2]**
**Sources:** qwen-2 C1/C2/C3/M19, glm S10, codex (adjacent) · **CONFIRMED**
**Where:** `pkg/media/validate_url.go`:
- `:13-15` — `if h.wahaBaseURL == "" { return nil }` skips **all** URL validation (loopback/private/link-local blocking) when the WAHA base URL is unset. **Fail-open.**
- `:81-83` — on `net.LookupHost` error the function returns `nil` (allowed) instead of rejecting. **Fail-open.**
- `:42` vs the fetch in `handler.go:65-73` — IPs are checked at validation time but the `http.Client` has no `net.Dialer.Control`, so DNS is re-resolved at connect time (rebinding TOCTOU).
- `:115-126` (M19) — any dot-less hostname is treated as "Docker-internal" and allowed; tighten to a known-hosts allowlist.
**Fix:** (a) always block private/loopback/link-local regardless of `wahaBaseURL`; (b) reject on DNS-lookup error; (c) resolve once and pin the IP via a `net.Dialer.Control`/`DialContext` that re-checks the connect-time IP, preserving the `Host` header; (d) constrain the dot-less-hostname allowance. **Effort: M.**
**Tests:** empty base URL still blocks `127.0.0.1`/`169.254.169.254`; DNS failure → rejected; rebinding (public-then-internal) → blocked at dial.

### ✅ P2.17 Undefined delivery status `"received"` **[qwen-2]**
**Sources:** qwen-2 M14 · **CONFIRMED**
**Where:** `internal/service/message_service.go:209` stores `DeliveryStatus: "received"`, which is **not** among the defined constants (`internal/models/message_mapping.go:8-12`: pending/sent/delivered/read/failed). `deliveryStatusRank` returns `-1` for unknown values, so status-progression comparisons (`shouldUpdateDeliveryStatus`) treat it as below everything.
**Fix:** add a `DeliveryStatusReceived` constant with a defined rank, or use an existing constant; assert ranks cover every stored status. **Effort: S.** **Test:** a "received" mapping correctly progresses to delivered/read.

### ✅ P2.18 `ValidateTimeout` applied to a retry count **[qwen-2]**
**Sources:** qwen-2 M8 · **CONFIRMED**
**Where:** `internal/config/config.go:247` calls `validation.ValidateTimeout(c.WhatsApp.RetryCount, ...)`; `ValidateTimeout` (`internal/validation/validation.go:156`) enforces a 1-3600 **seconds** range. A retry count of 0 is rejected with a "must be at least 1 second" error, and an absurd count (e.g. 3000) passes as a "valid timeout."
**Fix:** validate `RetryCount` with a numeric-range validator using retry-appropriate bounds. **Effort: S.** **Test:** `RetryCount=0` accepted (or rejected with a sensible message), out-of-range count rejected.

### ✅ P2.19 Config watcher fires unbounded callback goroutines on reload
**Sources:** qwen/deepseek H4, qwen-2 M10 · **CONFIRMED**
**Where:** `internal/config/watcher.go:114-122` launches one `go func` per callback on every reload with no context, timeout, or tracking; a blocked callback leaks a goroutine permanently.
**Fix:** run callbacks with a bounded `context.WithTimeout` and a `WaitGroup`; log (don't leak) on timeout. **Effort: S.** **Test:** a callback that blocks does not leak past the timeout.

---

## Phase 3 — Hygiene, dead code, consolidation

### ✅ P3.1 Delete dead packages
**Sources:** qwen/deepseek L2+L3, glm A1+A2 · **CONFIRMED (sizes corrected)**
`internal/features/` (591 non-test LOC) and `internal/versioning/` (704) have zero production importers. Remove both (and their tests). **Effort: S.**

### ✅ P3.2 Consolidate the two circuit breakers
**Sources:** qwen/deepseek H5+M1, glm A5 · **CONFIRMED**
Two implementations: `pkg/circuitbreaker` (used by signal/whatsapp clients) and `internal/service/circuit_breaker.go` (used by contact/group services), with divergent state machines and the P2.4 race. Also `pkg/circuitbreaker/circuit_breaker.go:184-200` `GetState()` mutates state (OPEN→HALF_OPEN) — a getter with side effects that surprises health/metrics callers. **Fix:** keep one implementation; split `GetState()` (pure read) from an explicit `MaybeTransition()`. **Effort: M.**

### ✅ P3.3 Consolidate retry/backoff and retryable-error classifiers
**Sources:** glm A6+A7, qwen/deepseek M5 · **CONFIRMED**
Four backoff impls (`internal/retry/backoff.go`, `internal/database/retry.go`, manual loops in `signal_poller.go:340-430` and `message_service.go:484-519`) and five substring-matching "is retryable" classifiers (`bridge.go:55-69`, `bridge.go:74-128`, `signal_poller.go:271-314`, `database/retry.go:60-100`, `errors/types.go:103-109`) with inconsistent verdicts for the same error string. **Fix:** one backoff helper; one classifier driven by typed/sentinel errors rather than message substrings. Also drop the `crypto/rand` jitter in `internal/retry/backoff.go` — jitter does not need a CSPRNG. **Effort: M-L.**

### ✅ P3.4 Unify phone masking and phone validation
**Sources:** glm A8+A9 · **CONFIRMED**
Three maskers with different output (`privacy.MaskPhoneNumber` → `+******7890`, `service.SanitizePhoneNumber` → `***7890`, `signal.maskPhone` → `***7890`) defeat cross-service log correlation; two validators disagree (`internal/validation/validation.go:14-42` strict ≤20 digits vs `internal/service/logging.go:98-175` permissive ≤25 + `@g.us`/`@lid`). **Fix:** single masking helper; one validator (or two clearly-named ones: `ValidateE164` vs `ValidateChatID`). **Effort: M.**

### ✅ P3.5 Resolve no-op `DeleteMessage` — **possible latent bug, needs decision**
**Sources:** qwen/deepseek M4 · **CONFIRMED — reclassified on verification**
`internal/service/message_service.go:283-288` locks the mutex and returns nil. **Verification changed this from "dead code" to "decide first":** `DeleteMessage` is declared on the service interface (`:51`) and a real deletion exists at the DB/WhatsApp-client layer (bridge deletion path `bridge.go:1058`). So the service method silently swallowing a delete may be a **latent bug** (a requested delete that does nothing), not safe-to-remove dead code. **Action:** confirm whether anything calls `messageService.DeleteMessage`; if yes → implement it (forward to `s.db.DeleteMessage`) and add a test; if nothing calls it → remove from the interface. Do **not** blind-delete. **Effort: S.** **Moved out of PR-9's delete list into PR-7 (decide+fix).**

### ✅ P3.6 Remove unused types/wrappers and unused error constructors
**Sources:** qwen/deepseek L5, glm A3 · **CONFIRMED (PARTIAL)**
Verified unused: `MessageMetadata` (`internal/models/message_mapping.go:33`); non-session send wrappers `SendImage/SendFile/SendVoice/SendVideo/SendDocument` (bridge calls only the `*WithSession` variants). `internal/errors/helpers.go` constructors have zero external callers — either adopt them in `server.go` error responses or delete. (`AudioMessage`/`VideoMessage` constants *are* referenced in logging/metrics — keep.) **Effort: S.**
Also remove **[qwen-2]**: the abandoned `models.Encryptor` struct (`internal/models/encryption.go:15`, zero refs; the real impl is `database.encryptor`); the unused `pkg/whatsapp/webhook.go` event-handler API with its no-op `handleTextMessage`/`handleImageMessage` stubs (`:63-70`, referenced only by `interfaces.go`, not on the live route — remove the dead API or wire+implement it); and the dead legacy `timestamp.body` signature fallback (`cmd/whatsignal/security.go:69-78`) — real WAHA signs raw body (contract-test-proven), so the fallback matches nothing and just adds attack-surface complexity.

### P3.7 `O(sessions)` DB queries per inbound Signal message
**Sources:** glm A11 · **CONFIRMED**
`internal/service/message_service.go:629-684` `determineDestinationForSender` calls `HasMessageHistoryBetween` once per session inside a nested loop. **Fix:** use `ChannelManager`'s reverse maps for the common O(1) case; fall back to the scan only when needed. **Effort: M.**

### ✅ P3.8 Inject the configured logger
**Sources:** qwen/deepseek L7, glm R7 · **CONFIRMED**
`logrus.New()` created fresh in `message_service.go:120`, `pkg/whatsapp/client.go:60`, both circuit breakers — ignoring the app's JSON formatter/level. **Fix:** pass the configured logger into constructors. **Effort: S.** (Naturally pairs with P3.2.)

### ✅ P3.9 Fix inverted `Tracing.UseStdout` default
**Sources:** qwen/deepseek M8 · **CONFIRMED (harmless today)**
`internal/config/config.go:148-150` sets `UseStdout=true` when tracing is *disabled*. Latent (no effect while tracing is off) but wrong. **Fix:** set stdout when tracing is enabled and no exporter endpoint is configured. **Effort: S.**

### P3.10 Lower-value hardening (do as touched)
- ✅ `io.ReadAll` without `LimitReader` on hot response bodies — `pkg/signal/client.go:248`, `pkg/whatsapp/client.go:620,664,801,862,967` (glm R4, qwen/deepseek M3). Wrap with `io.LimitReader(resp.Body, max+1)`; convert the two `ReadAll`→`Unmarshal` sites to `json.NewDecoder`. **CONFIRMED.** **Effort: S.**
- Gate raw Signal-envelope debug logging behind an explicit flag and redact PII (`pkg/signal/client.go:284-339`, codex #8). **CONFIRMED.** **Effort: S.**
- **[qwen-2]** WebSocket receiver logs raw frames (message PII) at **Error/Warn** — visible in production even with debug off (`pkg/signal/ws_receiver.go:67,85,87`, qwen-2 M17). **CONFIRMED.** Note the CLAUDE.md lesson that this path deliberately added envelope diagnostics for upstream issue #818 — so *don't delete it*: gate behind the same raw-debug flag as above and redact phone/text, rather than removing the diagnostic. **Effort: S.**
- **[qwen-2]** Request logging records the full URL incl. query string (`internal/middleware/detailed_logging.go:95`, qwen-2 M9) — log path only to avoid leaking any query-param secrets. **CONFIRMED, low** (WAHA puts the HMAC in headers, not query). **Effort: S.**
- **[qwen-2]** `sessionName` not URL-escaped in several API URLs (`pkg/whatsapp/session.go:84,123,156,188`, `client.go:781,938`; note `client.go:143` already escapes) — apply `url.PathEscape`/`QueryEscape` consistently (qwen-2 H11). Operator-controlled config, so **low**, but the inconsistency is a real bug. **Effort: S.**
- **[qwen-2]** Reject path separators / `..` in `att.ID` at construction (`pkg/signal/client.go:679-682`, qwen-2 H12) — already mitigated downstream by `ValidateFilePath` before write, so **low** defense-in-depth. **Effort: S.**
- **[qwen-2]** Empty strings short-circuit encryption and are stored as plaintext in "encrypted" columns (`internal/database/encryption.go:77,93,138`, qwen-2 M1) — reveals only emptiness; **low**. Encrypt to a sentinel if the equality leak matters (overlaps P2.5). **Effort: S.**
- ✅ Add a min-length/entropy check on `WHATSIGNAL_ADMIN_TOKEN` (`security.go:104`), matching the 32-char webhook-secret rule (qwen/deepseek M14). **CONFIRMED.** **Effort: S.**
- Drop deprecated `X-XSS-Protection`; add `Strict-Transport-Security` (and `Content-Security-Policy` if any HTML is ever served) at `server.go:209-212` (qwen/deepseek M11). **CONFIRMED, low** (server-to-server). **Effort: S.**
- Genericize the *client-facing* signature-verification error to a single 401 string while logging the specific cause server-side (`security.go:39-92`, qwen/deepseek L9). **CONFIRMED, low.** **Effort: S.**
- Drain non-2xx response bodies (`io.Copy(io.Discard, resp.Body)`) before `Close` to keep keep-alive (glm R6). **PARTIAL/low** — only matters under sustained 5xx. **Effort: S.**
- Strengthen `generateUniqueID` crypto/rand fallback (`message_service.go:132-139`) with a monotonic counter (qwen/deepseek L6). **CONFIRMED, low.** **Effort: S.**
- Pin `docker-compose.yml` images by digest instead of `:latest` (`:7,:34,:72`, codex #11). **CONFIRMED.** **Effort: S.**

### ✅ P3.11 OTLP exporter hardcoded insecure; tracing shutdown not concurrent-safe **[qwen-2]**
**Sources:** qwen-2 H8 + L12 · **CONFIRMED**
**Where:** `internal/tracing/opentelemetry.go:185` always creates the OTLP exporter with `WithInsecure()` (plaintext) — fine for local dev, but this path runs whenever `use_stdout=false`, including production over an untrusted network. Separately, `TracingManager` has no mutex around `tracerProvider` (`:231,:249`), so `Shutdown()` racing `Initialize()`/`GetTracer()` is unsafe.
**Fix:** make transport security configurable (TLS by default, opt-in insecure for local); guard the provider with a mutex or `sync.Once`. **Effort: S.** **Severity:** medium, but only when tracing is enabled.

---

## Phase 4 — Tests, build, CI

### ✅ P4.1 Fix the mock that ignores testify expectations
**Sources:** glm T3 · **CONFIRMED** — highest-value test fix
**Where:** `internal/service/mocks_test.go:14-58` embeds `testify/mock.Mock` but `SendImageWithSession`/`SendVideoWithSession`/`SendDocumentWithSession`/`SendVoiceWithSession` return fixed fields instead of calling `m.Called(...)`, so `.On(...).Return(...)` setups are silently ignored (only `SendReactionWithSession` honors them). This is the "mock mirrors the implementation" anti-pattern called out in CLAUDE.md. **Fix:** route all methods through `m.Called(...)`. **Effort: S.**

### ✅ P4.2 Make `go test -race` a reliable gate (fixed-port isolation)
**Sources:** codex #6, glm T4 · **CONFIRMED**
`cmd/whatsignal/main_test.go:28-57` starts `run(ctx)` without `PORT=0`, binding the default `:8082` (`server.go:135`) and colliding under `-race`; `main_test.go:620-629` shows the correct `PORT=0` pattern. The listener-close-then-rebind in `server_test.go:1706-1729` also has a TOCTOU window. **Fix:** set `PORT=0` in shared setup for every test that calls `run(ctx)`, or inject the listener into `http.Server.Serve`. Then add `go test -race ./...` to CI. **Effort: M.**

### ✅ P4.3 Add tests for uncovered webhook handlers
**Sources:** qwen/deepseek L10 · **CONFIRMED**
`handleWhatsAppReaction` (`server.go:561`) and `handleWhatsAppWaitingMessage` (`server.go:818`) have no test referencing them. Add handler tests (valid + malformed payloads). **Effort: M.**

### ✅ P4.4 Add direct DB-layer tests for pending persistence
**Sources:** qwen/deepseek L11 (re-scoped) · **PARTIAL**
The persistence functions are exercised only via service-layer mocks; `database.go:1078-1208` (`SavePendingSignalMessage`, `GetPendingMessages`, `DeletePendingSignalMessage`, `IncrementPendingRetryCount`) need round-trip tests against a real temp DB — this is the crash-recovery path. Combine with P2.2 (retry-cap/age cleanup). **Effort: M.**

### ✅ P4.5 Replace `time.Sleep` syncs with polling helpers
**Sources:** qwen/deepseek L1, glm T1 · **CONFIRMED** (55 instances)
Worst: `pkg/media/handler_test.go:665` (3 s), `internal/service/signal_poller_test.go:227` (2.5 s), 8× in `session_monitor_test.go`. Per CLAUDE.md, use poll-until-state helpers. **Effort: M** (do opportunistically per file).

### ✅ P4.6 Migrate `os.Setenv`→`t.Setenv` in tests
**Sources:** glm T2 · **CONFIRMED** (245 instances)
`t.Setenv` auto-cleans and is `t.Parallel`-safe. Mechanical sweep. **Effort: M.**

### ✅ P4.7 Make migration 004 idempotent
**Sources:** glm B1 · **CONFIRMED**
`scripts/migrations/004_add_contact_name_hashes.sql:2-4` uses bare `ALTER TABLE ADD COLUMN` (SQLite has no `IF NOT EXISTS` for columns). **Fix:** guard each add (check `pragma_table_info` before `ALTER`, or split into separately-tracked migration steps). **Effort: S.**

### ✅ P4.8 Add missing indexes for hot/cleanup queries
**Sources:** glm B2, qwen-2 M5 · **CONFIRMED (PARTIAL)**
`SelectRecentMessageMappingsBySessionQuery` (`queries.go:80-88`) filters `session_name` and orders by `forwarded_at DESC LIMIT` (hit on every fallback route); existing indexes don't cover it — add a `(session_name, forwarded_at)` composite. **[qwen-2]** Also add an index on `message_mappings.created_at` for the cleanup `DELETE ... WHERE created_at < ?` (currently a scan). (The `pending_signal_messages` table already has `created_at` and `message_id_hash` indexes — no change needed there.) Add in a new migration; verify each with `EXPLAIN QUERY PLAN`. **Effort: S.**

### ✅ P4.11 Small DB/middleware correctness cleanups **[qwen-2]**
**Sources:** qwen-2 M4/M6/M7/L4 · **CONFIRMED (low)**
- `SELECT COUNT(*) ... LIMIT 1` used as an existence check (`database.go:1039`, M6) — the `LIMIT 1` is a no-op; switch to `SELECT EXISTS(SELECT 1 ...)` to short-circuit. **Effort: S.**
- `INSERT OR REPLACE` on contacts/groups (`queries.go:105-111,137-140`, M4) deletes+reinserts on UNIQUE conflict, churning the AUTOINCREMENT PK. No FK references exist today so impact is nil, but switch to `INSERT ... ON CONFLICT(...) DO UPDATE` (true upsert) to keep it safe as the schema grows. **Effort: S.**
- `responseCaptureWrapper` does not implement `http.Flusher` (`internal/middleware/detailed_logging.go:176-203`, M7) — add a `Flush()` delegating to the underlying writer (and assert `http.Flusher` where wrapped) so streaming/flush isn't silently swallowed. **Effort: S.**
- `FlexibleTimestamp.UnmarshalJSON` truncates float timestamps to whole seconds (`internal/models/webhooks.go:23`, L4) — confirm WAHA timestamps don't need sub-second precision for dedup/ordering; if they do, preserve milliseconds. **Effort: S, verify-first.**

### ✅ P4.9 Exclude `.worktrees` from CI scanners
**Sources:** codex #13, glm (implied) · **CONFIRMED**
`.worktrees/` exists and `.github/workflows/security.yml` + `Makefile` run gosec/staticcheck/vet on `./...` with no exclusion (only manual greps exclude it). **Fix:** add `-exclude-dir=.worktrees` (gosec) and equivalent ignores, or keep worktrees outside the repo. **Effort: S.**

### P4.10 (Optional) Evaluate `modernc.org/sqlite` to drop CGO
**Sources:** glm B3 · **OBSERVATION, not a defect**
`Dockerfile:39-44` builds `CGO_ENABLED=1` static into distroless — works today (sqlite is the only C dep) but brittle if another C dep is added. Pure-Go sqlite would remove the CGO constraint. Track as a future option, not a required change. **Effort: L.**

---

## Suggested execution order

1. **Phase 1** as one security/stability PR (P1.1-P1.10) — small, high-impact, mostly S. **Pull P2.16's two fail-open SSRF fixes (empty-base-URL bypass, DNS-failure-allows) forward into this PR** — they are one-line guards with direct external exposure.
2. **Phase 2** in two PRs: security/resource (P2.1, P2.2, P2.5, P2.6, P2.7, P2.8, rest of P2.16) then concurrency/correctness (P2.3, P2.4, P2.9-P2.15, P2.17, P2.18).
3. **Phase 3** consolidation PRs, smallest-blast-radius first (P3.1 dead-code delete + P3.6 incl. the qwen-2 dead API/struct/legacy-sig removals, then the consolidations P3.2-P3.4, P3.7; P3.11 with any tracing work). Note P3.5 moved to PR-7 (it's a decide-then-fix, not a dead-code delete — see V1).
4. **Phase 4** alongside or before each code PR — land P4.1 and P4.2 early so `-race` and honest mocks guard the rest; P4.8/P4.11 ride with the relevant DB changes.

Per project CLAUDE.md: run `make ci` (and `go test -race ./...` once P4.2 lands) to zero failures before each commit; version bump is the last commit of any release.

---

## Appendix A — Spike design notes (S1-S6)

Each spike was code-investigated to ground the design. Two changed scope on investigation: **S1 is larger** than first framed (the target columns are not lookup-only), and **S6 is effectively a no-op** (the precision loss doesn't reach the security check). Facts below are cited to file:line.

### S1 — `EncryptForLookup` → searchable-hash migration

**Problem.** `EncryptForLookup` (`internal/database/encryption.go:137-153`) uses a deterministic nonce, so equal plaintext → equal ciphertext (equality leak), and it silently falls back to the public default lookup salt. P2.5a (fail-closed salt) is the immediate mitigation; this spike removes the equality leak.

**Key finding that changes scope.** All five `EncryptForLookup` columns are **also decrypted for plaintext recovery** — they are *not* lookup-only:
- `contacts.contact_id` (write `database.go:746`, WHERE `:801`, decrypt `:823`) — `UNIQUE` on the ciphertext column.
- `groups.group_id` (write `:936`, WHERE `:965`, decrypt `:987`) — `UNIQUE(group_id, session_name)` on the ciphertext.
- `message_mappings.signal_msg_id` (write `:439`, decrypt `:253`).
So you cannot just replace the column with a hash (the proven `contacts.name_hash` pattern works only because names are never decrypted). `LookupHash` is HMAC-SHA256 with a dedicated key (`encryption.go:67-74`); the random-nonce `Encrypt` is the recoverable primitive.

**Design (per-column, phased).** For each column: (1) add a `<col>_hash` column + index, move any `UNIQUE` constraint onto the hash; (2) backfill the hash from existing rows; (3) switch all `WHERE <col> = ?` to `WHERE <col>_hash = ?`; (4) re-encrypt the value column from deterministic `EncryptForLookup` to random-nonce `Encrypt` (kills the equality leak while keeping recovery); (5) drop the old index. Run as new migrations `006+` (runner: `internal/migrations/migrations.go`, tracked in `schema_migrations`; highest current is `005`). Use a dual-read window (read hash, fall back to old ciphertext) so a half-applied deploy keeps working.

**Risks.** Backfill requires the encryption key at migration time; `UNIQUE`-constraint relocation on `groups`/`contacts` is the trickiest step (do it via table-rebuild like migration 001, idempotently per P4.7's lesson). Random-nonce re-encryption means the value column can no longer be queried directly — every reader must go through the hash.

**Acceptance.** Equal plaintext → distinct ciphertext (test); lookups still hit an index (`EXPLAIN QUERY PLAN`); round-trip decrypt unchanged; migration idempotent on re-run; full suite + `-race` green.
**Effort: L.** Recommend doing the highest-cardinality column first (`signal_msg_id`) as a vertical slice, then `contact_id`/`group_id`.

### S2 — Narrow/remove `messageService.mu`

**Problem.** `s.mu` wraps DB calls, serializing all of them across sessions/chats (H3).

**Key findings.** `s.mu` protects **only `s.db` calls** — no in-memory state (struct `message_service.go:105-116`; dedup is handled separately by `inProgressMessages sync.Map` and per-chat `chatLockManager`, both orthogonal to `s.mu`). Of 10 critical sections, 8 are single-op DB calls (safe to drop — WAL + pool handle concurrency, confirmed by glm's disproved busy-timeout test). **Exactly one** is a check-then-act that needs atomicity: `UpdateDeliveryStatus` (`:360-373`) reads the mapping, applies the monotonic `shouldUpdateDeliveryStatus` rank guard, then writes — dropping the lock here allows a `sent→read` / `sent→delivered` race to land out of order.

**Design.** Remove `s.mu` from the 8 single-op sites. For `UpdateDeliveryStatus`, make the guard atomic at the DB instead of via a process lock: a single conditional `UPDATE ... SET delivery_status = ? WHERE whatsapp_msg_id_hash = ? AND <new-rank > current-rank>` (encode rank via a `CASE` over the status string, or store a numeric rank column). This is correct across processes too, which a `sync.Mutex` never was.

**Risks.** The monotonic CASE must enumerate every status — including the undefined `"received"` (P2.17), so **do P2.17 first**. Build the throughput benchmark before/after on the `TestChatLockManager_Concurrent*` / `database_bench_test.go` templates (no message-service benchmark exists yet) to prove the win and catch regressions.

**Acceptance.** Concurrent `sent→delivered`/`sent→read` always converges to the higher rank (race test); benchmark shows throughput improvement; `-race` clean.
**Effort: M.**

### S3 — O(sessions) routing fast path

**Problem.** `determineDestinationForSender` (`message_service.go:629-684`) issues one `HasMessageHistoryBetween` query **per WhatsApp session** for an inbound Signal message when the sender isn't itself a configured destination.

**Key findings.** A fast path already exists: single-destination configs skip the function entirely (`:414-415`), and a sender that matches a configured destination returns via the O(1) `ChannelManager.reverse` map (`:631-645`, `channel_manager.go:72`). The remaining cost is the multi-session fallback. `HasMessageHistoryBetween` (`database.go:1018-1052`) genuinely needs the DB — it depends on actual message history, not static config — so the `ChannelManager` maps can't replace it. The query is `SELECT COUNT(*) ... WHERE session_name = ? AND chat_id_hash = ? LIMIT 1`.

**Design.** Collapse the N per-session queries into **one**: `SELECT DISTINCT session_name FROM message_mappings WHERE chat_id_hash = ? AND session_name IN (<sessions>)`, then pick among the returned sessions. This turns O(sessions) round-trips into a single indexed query (covered by `idx_chat_hash_time`). Fold in P4.11's `COUNT(*)`→`EXISTS` note since the per-session COUNT goes away entirely.

**Risks.** Low — same data, fewer queries. Preserve current tie-break/selection order when multiple sessions match.
**Acceptance.** One query per inbound message regardless of session count (assert via a query-counting DB wrapper or `EXPLAIN`); routing decision identical to today on a fixture with 2+ sessions.
**Effort: M.**

### S4 — Outbound media streaming (constrained)

**Problem.** Media send reads the whole file + base64 + JSON into memory (~7.7× peak; P1.10/R5).

**Key finding — protocol (gate resolved, web-verified).** The repo only ever sends base64-in-JSON (WAHA `MediaMessageRequest.File.Data` string, `pkg/whatsapp/types/models.go:71-79`; Signal `Base64Attachments[]`, `pkg/signal/types/models.go:37-43`). But the **WAHA API itself accepts a remote `url` *or* base64 `data`** in the `file` object (no multipart), per its docs (`files.dto.ts`/`chatting.controller.ts`). **signal-cli `/v2/send` is base64-only.** Caveat: the `url` must be HTTP-reachable from WAHA's container network.

**Design (per direction).**
1. **WhatsApp direction — prefer URL passthrough (removes base64 entirely).** Send `file.url` instead of `file.data` when the source media is reachable as a URL from WAHA — either the original WhatsApp media URL (if still valid) or a URL whatsignal serves from its cache. This is the biggest win and needs no streaming machinery. Validate the URL with the same SSRF guard. Fall back to base64 when no reachable URL exists.
2. **Signal direction (and WAHA base64 fallback) — stream the encode.** Build the request body as a custom `io.Reader` that emits `{… "base64_attachments":["` + a `base64.NewEncoder` wrapping the file reader + `"]}`, passed to `http.NewRequest`, so neither the full base64 string nor a `json.Marshal` copy is held. Peak drops from ~7.7× to roughly the file size. Pair with the P1.10 hard cap.

**Risks.** Hand-built JSON body must exactly match the current contract — existing tests only assert the `"file"` key is present (`client_media_test.go:195-201`) and the POST path (`client_test.go:396`), so **add a test that decodes the body and verifies the base64 round-trips** before refactoring.
**Effort: M** (track 2) / **S** (gate check). Defer until P1.10's hard cap (which removes the acute OOM risk) is shipped.

### S5 — Drop CGO via `modernc.org/sqlite`

**Problem.** `CGO_ENABLED=1` + static libsqlite + distroless is fragile (B3); the only C dependency is sqlite.

**Key findings (low-risk).** `mattn/go-sqlite3` is imported **only as a side-effect** (`database.go:17`; no mattn-specific APIs), all DB access is stdlib `database/sql`, and the only PRAGMAs used — `journal_mode=WAL` (`:92`), `synchronous=NORMAL` (`:100`), `busy_timeout=5000` (`:108`) — are all supported by modernc. No `RegisterFunc`, FTS, JSON1, loadable extensions, custom DSN params, `import "C"`, or `.s` files anywhere. Switch points: 6 × `sql.Open("sqlite3", …)` → `"sqlite"`; `CGO_ENABLED 1→0` (Makefile `:38`); drop `sqlite-dev` + `-extldflags '-static'` from the Dockerfile (`:17,:39-40`).

**Design (web-verified specifics).** modernc registers driver name **`"sqlite"`** (not `"sqlite3"`) — update the single `sql.Open` (`database.go:46`) and the 5 side-effect imports (`_ "modernc.org/sqlite"`). modernc has **no dedicated `_busy_timeout` DSN param**; set PRAGMAs via the DSN `_pragma=` form: `file:…?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)` (or keep the existing `PRAGMA` exec statements). Important hardening modernc enables: use **`_txlock=immediate`** for write transactions — under WAL, a read-then-write transaction can still hit `SQLITE_BUSY` that `busy_timeout` won't retry (true of both drivers); `BEGIN IMMEDIATE` avoids the lock-upgrade deadlock. Flip `CGO_ENABLED=1→0` (Makefile `:38` and build/test targets), drop `sqlite-dev` + `-extldflags '-static'` from the Dockerfile, and update the CLAUDE.md "CGO_ENABLED=1 required" note. Run the **full suite + `-race` + `database_bench_test.go`** against both drivers and compare latency (modernc is pure-Go and directionally somewhat slower under heavy write load — immaterial for this low-write workload, but measure).

**Risks.** Behavioral/perf differences between C-sqlite and modernc on edge SQL; verify the migration runner and concurrency tests pass identically. Reversible (driver name + import swap).
**Acceptance.** All tests + `-race` green on modernc; benchmark delta acceptable; image builds with `CGO_ENABLED=0` and no sqlite-dev.
**Effort: M.**

### S6 — `FlexibleTimestamp` precision — RESOLVED (no action)

**Investigated and closed.** The concern was that `FlexibleTimestamp.UnmarshalJSON` (`internal/models/webhooks.go:12-25`) truncates float seconds to `int64`, dropping sub-second precision. Tracing usage: it feeds storage (`message_mappings.signal_timestamp`), ACK tracking, and logs — all second-granularity. The **webhook skew/replay check does *not* use it** — that path parses the `X-Webhook-Timestamp` *header* (milliseconds) separately in `security.go:48-59`. Signal message IDs derive from Signal envelope timestamps (`pkg/signal/client.go:435,507`), independent of this type. **No functional impact; close S6.** Optional only: preserve milliseconds if a future feature needs sub-second ordering. Removed from the active spike list.

---

## Appendix B — Verification pass: per-task implementation notes

Every item below was re-checked against the code (deep agents, June 2026). This captures the non-obvious facts an implementer needs that aren't in the finding body: exact change surface, hidden call sites, **tests that will break**, and any sub-decision still open. Locations confirmed unless noted.

### Decisions discovered during verification (settle before the owning PR)

These are *new* open decisions surfaced by the deep pass (distinct from the D1-D8 defaults):

| # | Decision | Owning PR | Recommendation |
|---|---|---|---|
| V1 | `messageService.DeleteMessage`: implement (forward to `s.db.DeleteMessage`) or remove from interface? It's a silent no-op today and may drop real deletes. | PR-7 (P3.5) | Grep callers first; implement if called, else remove. |
| V2 | Rank value for the new `DeliveryStatusReceived` constant (pre-sent? = sent? between sent/delivered?). Blocks S2's atomic UPDATE. | PR-7 (P2.17) | Rank it at/just-above `sent`; confirm against the ACK lifecycle. |
| V3 | Valid `WhatsApp.RetryCount` range for the new numeric validator (e.g. 0-100). | PR-7 (P2.18) | 0-100; 0 = retries disabled. |
| V4 | `log_level` config: respect it, or remove the field and keep `-verbose` only? | PR-7 (P2.15) | Respect the config value; keep `-verbose` as an override. |
| V5 | Which circuit-breaker impl is the survivor for P3.2? | PR-9 (P3.2) | Keep `pkg/circuitbreaker` (more polished, typed `Stats`); delete `internal/service` copy; split its side-effecting `GetState`. |
| V6 | P2.14 reaction fallback: does the inbound Signal reaction payload carry a source chat/group to scope the fallback? | PR-7 (P2.14) | Confirm in the envelope; if absent, log-warn only (don't silently mis-target). |

### Shared-helper / dependency notes (avoid rework & merge conflicts)

- **`isSecureMode()` is shared by P1.2, P1.3, P3.10-admin-token, and P2.5a.** All four currently branch on `os.Getenv("WHATSIGNAL_ENV")` independently (`security.go:31,100`, `config.go:200`, `encryption.go:168-175`). Introduce one helper (unset = secure/production) in PR-2 and route all of them through it.
- **`config.go:200-223` already fail-closes salts/secret in production** at the *config* layer; P2.5a only needs to make the *runtime* `getEncryptionSalt`/`getEncryptionLookupSalt` (`encryption.go:168-175`) stop silently using the repo default — i.e., match the layer that already validates.
- **P2.17 must land before S2** — the monotonic `UPDATE … CASE` must enumerate `received`, else it ranks `-1` and breaks transitions.
- **P4.2 (-race gate) must land before PR-5** so the concurrency reproducers are trustworthy.

### Tests that will BREAK and must be updated in the same PR

- **P1.2/P1.3 (fail-closed):** `cmd/whatsignal/server_test.go:343-347` (`TestVerifySignature` "empty secret key (skip verification)" expects no error with env unset) and several `internal/config/config_test.go` cases assume fail-open — update to set `WHATSIGNAL_ENV=development` explicitly.
- **P2.17 (`received` rank):** any test asserting current rank behavior of an unknown status.
- **P3.1/P3.6 (deletes):** delete the corresponding `*_test.go` (features, versioning, errors/helpers, webhook).

### Per-item confirmations (locations re-verified)

| Item | Confirmed location | Implementation note from verification |
|---|---|---|
| P1.1 | `httputil/clientip.go:14-26`; consumers `security.go:317`, `server.go:189` | Change `GetClientIP` signature to take trusted-proxy CIDRs; add `WHATSIGNAL_TRUSTED_PROXIES` to config + plumb to rate-limit middleware. |
| P1.5 | `metrics.go:169-196`; scraped at `metrics_handler.go:29` then `json.Encode` | Deep-copy `Metric`/`TimerMetric` by value **and** `samples := make+copy` under the RLock. |
| P1.6 | `delivery_monitor.go:57`, `scheduler.go:57` | Copy RateLimiter pattern verbatim (`security.go:180-187`): `stopOnce`+`stopWg`; `Add(1)` in Start, `defer Done()` in loop, `Once.Do{close; Wait}`. |
| P1.7 | `session_monitor.go:82` (go launch), `:87-101` Stop, `:103` loop | Add `monitorWg`; `Add(1)` before `go monitorLoop`, `defer Done()` in loop, `Wait()` in Stop. |
| P1.8 | `server.go:157-163` | Copy `ReadHeaderTimeout` value from `integration_test/environment.go:910` (30s) or use a new `DefaultServerReadHeaderTimeoutSec`. |
| P2.3 | `whatsapp/client.go:42,900-932`; `signal/client.go:52-54,762-776,990-996`; `contact_service.go:136`, `group_service.go:115` | `supportsVideo`→`atomic.Pointer[bool]`; signal init trio→one `sync.RWMutex` (3 correlated fields); `degradedMode`→`atomic.Bool`. |
| P2.4 | `internal/service/circuit_breaker.go:76,122,128,132,157,165` | Minimal fix if P3.2 deferred: drop the `atomic.*` calls, use plain field access under `mu` (incl. the lock-free read in `Execute:76`). |
| P2.7 | `server.go:324-328`; Dockerfile HEALTHCHECK `:70` | Split `/healthz` (200 if up) vs `/readyz` (503 on degraded); point HEALTHCHECK at `/readyz`. |
| P2.9 | `bridge.go:990`; sole caller `:931` | Add `ctx` param to `extractMappingFromQuotedText`; thread from `resolveMessageMapping(ctx,…)`. |
| P2.10 | `metrics.go:106-150,220-238` | Move P95/P99 sort out of `RecordTimer`; keep online min/max/sum/count; compute percentiles in `GetAllMetrics` (folds with P1.5 deep-copy). |
| P2.12 | `signal/client.go:586-621` | In the `downloadCtx.Done()` arm: `select { case fp := <-downloadChan: os.Remove(fp); default: }`. |
| P2.13 | `observability.go:78-80`; `SetGauge` at `metrics.go:153` | Replace counter inc/dec with a gauge (atomic active-count + `SetGauge`). |
| P2.16 | `validate_url.go:12-15,81-83,115-126`; download `handler.go:233,254` | `validateDownloadURL` returns the resolved IP; pin it via `http.Transport`/`net.Dialer.Control` at dial; fix both fail-open returns; tighten dot-less host allowlist. |
| P3.2 | `pkg/circuitbreaker` (Stats struct, private `reset`, side-effecting `GetState:184`) vs `internal/service/circuit_breaker.go` (map stats, public `Reset`, atomic race) | Keep `pkg`; consumers to migrate: `contact_service.go`, `group_service.go`. Reconcile: typed `Stats`, split `GetState`/`MaybeTransition`, inject logger (folds P3.8). |
| P3.3 | backoff: `retry/backoff.go`, `database/retry.go`, `signal_poller.go:340-430`, `message_service.go:484-519`; classifiers: `bridge.go:55-69,74-128`, `signal_poller.go:271-314`, `database/retry.go:60-100`, `errors/types.go:104-109` | Unify on `retry/backoff.go` (drop crypto/rand jitter); one typed-error classifier; substring matching only as legacy fallback. |
| P3.4 | maskers `privacy/masking.go:9`, `service/logging.go:105`, `signal/client.go` `maskPhone`; validators `validation/validation.go:14-42` vs `service/logging.go:98-175` | `SanitizePhoneNumber` has 20+ call sites — biggest sweep; settle V-decision on whether the two validators are intentionally different (`ValidateE164` vs `ValidateChatID`). |
| P3.8 | `message_service.go:120`, `whatsapp/client.go:60`, both CBs | Constructors need a new `logger` param + caller updates (main.go, tests). |
| P3.11 | `tracing/opentelemetry.go:185` (`WithInsecure`), `:231-249` Shutdown | Make transport TLS configurable; guard `tracerProvider` with `sync.RWMutex` (Init/Get/Shutdown). |
| P4.7 | `004_add_contact_name_hashes.sql:2-4` | Guard each `ALTER ADD COLUMN` with a `pragma_table_info` existence check (or split per column); runner wraps each file in a txn (`migrations.go:157-174`). |
| P4.8 | `queries.go:80-88` + cleanup `DELETE … created_at` | New migration `006`: `idx … (session_name, forwarded_at DESC)` and `idx … (created_at)`; verify with `EXPLAIN QUERY PLAN`. |
| P4.11 | `database.go:1039` (COUNT→EXISTS), `queries.go:106-111,137-140` (→`ON CONFLICT DO UPDATE`), `detailed_logging.go:176-203` (+`Flush()` + `var _ http.Flusher` assert) | All independent, low-risk. |
| P4.3 | handlers `server.go:561` (`handleWhatsAppReaction`), `:818` (`handleWhatsAppWaitingMessage`) | Zero tests today — confirmed; add valid + malformed-payload cases. |
| P4.5 / P4.6 | mechanical sweeps | Counts to recount at implementation time (~59 `time.Sleep`, ~150-245 `os.Setenv`); use poll-helpers / `t.Setenv`. |

**Net status:** all 9 PRs + 6 spikes are now implementation-grade. Remaining true unknowns are the six V-decisions above (owner calls, not code gaps) and the WAHA-`url` network-reachability check for S4 track 1 (operational, verify in the target deployment).
