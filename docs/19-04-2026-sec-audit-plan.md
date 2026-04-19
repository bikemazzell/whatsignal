 # WhatSignal Security Assessment And TDD Remediation Plan

  ## Summary

  Perform an authorized, local-first security assessment of the WhatSignal service using passive/static analysis and active/dynamic testing. Every concrete vulnerability
  scenario must follow TDD: write a test that fails when the issue exists, run it and record the expected failure, implement the smallest fix, rerun the targeted test, then
  rerun broader security and regression gates.

  Baseline discovery already verified:

  - Security-heavy packages currently pass: go test ./cmd/whatsignal ./internal/config ./internal/database ./pkg/media
  - gosec, govulncheck, staticcheck, and golangci-lint are not currently on PATH, so scanner setup is part of the assessment gate.
  - Main attack surfaces are webhook authentication/replay, public status endpoints, config/secrets, encryption salts, media URL download/SSRF handling, file paths, logs/PII,
    Docker runtime, and dependency/base image exposure.

  ## Reviewed Assumptions And Corrections

  Initial plan risks corrected in this final plan:

  - Do not active-test real WhatsApp, Signal, or third-party infrastructure. Active tests run only against local httptest, local Docker Compose, or mock WAHA/Signal services.
  - Do not assume scanners are installed. First gate checks availability and installs only if explicitly allowed during execution.
  - Do not treat static findings as real vulnerabilities until confirmed with a failing test or a clearly documented non-testable control gap.
  - Do not fix before RED. For each suspected issue, add the failing unit/integration/smoke test first and verify it fails for the security-relevant reason.
  - Treat docs/deployment gaps as security findings when they can produce insecure defaults, even if runtime code is correct.

  ## Uniform Task Template

  For every scenario below, use this exact task shape:

  1. Passive review: Identify the expected security invariant and current behavior.
  2. RED test: Add the smallest test that fails if the vulnerability is present.
  3. Run RED: Run only the targeted test and confirm the failure is meaningful, not a fixture mistake.
  4. Fix plan: Apply the minimal remediation for the violated invariant.
  5. GREEN test: Rerun the targeted test and confirm it passes.
  6. Regression gate: Run the package test, then relevant integration/smoke/security commands.
  7. Evidence: Record command, result, files touched, residual risk, and whether docs/config need updates.

  ## Assessment And Remediation Tasks

  ### 1. Baseline Inventory And Tooling

  - Review routes, config loading, crypto, media handling, Docker, migrations, logs, and external clients.
  - Run baseline gates:
      - go test ./cmd/whatsignal ./internal/config ./internal/database ./pkg/media
      - go test ./...
      - go test -race ./cmd/whatsignal ./internal/service ./internal/database
      - go vet ./...
  - Add scanner gate:
      - If tools are available: run gosec ./..., govulncheck ./..., staticcheck ./..., golangci-lint run ./....
      - If missing: document as blocked until tools are installed, then rerun.
  - Any scanner finding must be converted into a failing test before production code changes unless it is purely dependency/image metadata.

  ### 2. Webhook Authentication, Replay, And Request Hardening

  Target invariants:

  - WhatsApp webhook POST requires valid HMAC, required timestamp, bounded skew, JSON content type, bounded body size, and no raw PII in error logs.
  - Missing secrets must fail closed in production.
  - Replayed/stale/future requests must be rejected.
  - Rate limiting must not be bypassable through spoofed client IP headers unless the proxy trust model is explicit.

  TDD scenarios:

  - Invalid or missing X-Webhook-Hmac returns 401.
  - Missing X-Webhook-Timestamp returns 401.
  - Timestamp outside configured skew returns 401.
  - Same body with changed timestamp is rejected if the final design binds timestamp into the MAC.
  - Oversized body returns 413 and does not call message service.
  - Non-JSON content type returns 400.
  - Production startup with empty or short webhook secret fails.
  - Log capture test confirms invalid JSON does not log raw body, phone numbers, message content, or secrets.

  Default fixes if tests fail:

  - Bind HMAC to timestamp plus body using a stable canonical input.
  - Keep backward compatibility only if WAHA’s actual signature format requires it; otherwise fail closed.
  - Add explicit trusted proxy configuration before honoring forwarded IP headers.
  - Keep webhook error responses generic.

  ### 3. Public Endpoint Exposure

  Target invariants:

  - /health exposes only operational state.
  - /session/status and /metrics do not expose phone numbers, session names, internal URLs, errors with credentials, or per-user metadata to unauthenticated callers.
  - Production can disable or protect sensitive diagnostics.

  TDD scenarios:

  - Unauthenticated /session/status in production returns 401 or a sanitized minimal response.
  - /metrics in production is protected or contains no PII-bearing labels.
  - Health response never includes configured secrets, API keys, phone numbers, database path, or upstream URLs.
  - Dependency error messages from WAHA/Signal are sanitized in public responses.

  Default fixes if tests fail:

  - Add WHATSIGNAL_ADMIN_TOKEN or WHATSIGNAL_PUBLIC_DIAGNOSTICS=false behavior for sensitive endpoints.
  - Keep /health public but minimal.
  - Redact dependency details from public JSON and keep detailed errors in structured logs with redaction.

  ### 4. Config, Secrets, And Secure Defaults

  Target invariants:

  - Production requires webhook secret, encryption secret, encryption salt, and lookup salt with minimum strength.
  - Example config and Compose do not encourage placeholder secrets.
  - Defaults are safe or loudly fail in production.

  TDD scenarios:

  - WHATSIGNAL_ENV=production with missing WHATSIGNAL_ENCRYPTION_SALT fails config validation.
  - WHATSIGNAL_ENV=production with missing WHATSIGNAL_ENCRYPTION_LOOKUP_SALT fails config validation.
  - Short salts fail production validation.
  - docker-compose.yml and examples include required salt variables or documented secret injection.
  - Placeholder WHATSAPP_API_KEY=your-api-key is rejected or warned as insecure in production.

  Default fixes if tests fail:

  - Extend config security validation to enforce salts in production.
  - Add salts to Compose, setup, deploy, and docs.
  - Convert repeated salt warnings into one startup warning or config validation result.

  ### 5. Encryption And Database Storage

  Target invariants:

  - Sensitive identifiers are encrypted at rest.
  - Lookup hashes use a separate HMAC key and never expose plaintext.
  - Random encryption uses unique nonces.
  - Lookup determinism is limited to lookup hashes or explicitly approved fields.
  - Database files are created with restrictive permissions.

  TDD scenarios:

  - Saved mappings do not contain plaintext chat IDs, message IDs, sender phone numbers, or media paths in SQLite.
  - Same plaintext encrypted twice with non-lookup encryption produces different ciphertext.
  - Lookup hash is deterministic but changes when lookup salt changes.
  - Decrypting tampered ciphertext fails.
  - Database file mode is 0600 on creation where the OS supports it.

  Default fixes if tests fail:

  - Prefer HMAC lookup columns over deterministic AES-GCM lookup ciphertext.
  - Ensure all query paths use lookup hashes and all stored sensitive fields use random-nonce encryption.
  - Enforce file permissions immediately after database creation/open.

  ### 6. Media Download, SSRF, Redirects, And File Handling

  Target invariants:

  - Media downloads only fetch allowed WAHA/Signal hosts.
  - Redirects cannot escape allowed hosts.
  - Internal metadata, localhost admin ports, and arbitrary private IPs are blocked unless they exactly match configured service hosts.
  - File type, size, path, and extension handling cannot write outside cache directories or execute content.

  TDD scenarios:

  - URL to 169.254.169.254, loopback, private IP, IPv6 localhost, decimal/octal/encoded host variants, and userinfo host confusion are rejected.
  - Redirect from allowed host to disallowed host is rejected.
  - Allowed Docker service host remains allowed only for configured WAHA/Signal endpoints.
  - Downloaded file exceeding configured size fails before unbounded memory/disk growth.
  - Filename/path traversal payloads cannot escape media cache.
  - SVG/HTML/polyglot content is treated as document or rejected, not trusted as image.

  Default fixes if tests fail:

  - Resolve hostnames and validate final IPs before every request and redirect.
  - Use a custom HTTP client redirect policy.
  - Write downloads to generated filenames under a validated cache directory.
  - Validate by content signature plus configured extension policy.

  ### 7. Input Validation, Routing Isolation, And Abuse Cases



  - Channel/session validation prevents cross-channel message injection.
  - Message IDs, chat IDs, session names, phone numbers, reactions, and captions have bounded length and valid format.
  - Duplicate or concurrent webhooks do not create inconsistent delivery state.
  - Malformed payloads cannot panic handlers.

  TDD scenarios:

  - Webhook with unconfigured session is ignored without forwarding.
  - Invalid session name is rejected with 400.
  - Cross-channel quoted reply cannot route to another channel’s WhatsApp session.
  - Extremely long message IDs, captions, sender names, or group names are rejected or truncated safely.
  - Fuzz tests for webhook payload decoding and session validation do not panic.
  - Concurrent duplicate webhook test remains idempotent.

  Default fixes if tests fail:

  - Centralize bounds in validation helpers.
  - Enforce channel lookups on every routing path, including reactions, edits, acks, quotes, and fallback routing.
  - Add idempotency checks around message mapping writes.

  ### 8. Logging, Privacy, And Error Disclosure

  Target invariants:

  - Logs never include secrets, API keys, raw webhook bodies, encryption material, or full phone numbers.
  - Public error responses are generic.
  - Debug logging is rejected in production.

  TDD scenarios:

  - Inject known sentinel secrets into config/env and assert captured logs do not contain them.
  - Send webhook with phone numbers and body text; assert logs contain masked values only.
  - Upstream Signal/WAHA errors containing tokens or phone numbers are redacted before logging at warn/error.
  - Production config with log_level=debug fails.

  Default fixes if tests fail:

  - Route all structured logging through existing privacy masking helpers.
  - Add redaction for keys containing secret, token, password, api_key, phone, chat, and raw body fields.
  - Keep debug-only raw payload logging disabled unless test mode explicitly enables it.

  ### 9. Docker, Supply Chain, And Deployment Hardening

  Target invariants:

  - Runtime image is pinned, non-root, read-only, minimal, and drops capabilities.
  - Compose does not publish unnecessary ports by default for production profile.
  - Secrets are not baked into images or committed examples.
  - Base images and Go dependencies are scan-clean or documented with accepted risk.

  TDD/static scenarios:

  - Static test parses Dockerfile and asserts final image is digest-pinned, USER is non-root, and no shell package manager exists in final stage.
  - Static test parses Compose and asserts read_only, cap_drop: ALL, no-new-privileges, and required secret envs for WhatSignal.


  - Pin builder image digest as well as runtime image.
  - Add production Compose profile with no host-published WAHA/Signal ports unless explicitly enabled.
  ### 10. Dynamic Active Test Suite

  Use only local services.
  - Start local app with mock WAHA/Signal and temporary SQLite.
  - GET /health succeeds and is sanitized.
  - Unsigned webhook fails.
  - Bad content type fails.
  - Oversized webhook fails.
  - Valid signed webhook with current timestamp succeeds.
  - Stale signed webhook fails.
  - Media redirect SSRF fixture fails.

  Integration tests:

  - Add security-focused integration tests under the existing integration framework using mock endpoints.
  - Run make test-integration for mock mode.
  - Run Docker integration only when Docker is available and user-approved.

  Fuzz tests:

  - Add bounded fuzz targets for webhook JSON decoding, signature header parsing, media URL validation, and config path validation.
  - Run short fuzz gates such as go test ./cmd/whatsignal -run=^$ -fuzz=Fuzz -fuzztime=30s during development and longer fuzzing before release.
  - The same test passes after the fix.
  - The containing package passes.
  - Relevant integration or smoke tests pass.
  - go test ./... passes.
  - Race tests pass for touched concurrent code.
  - Static/security scanners pass or findings are documented with severity and owner.
  - Docs/config examples are updated for any changed security behavior.
  - No unrelated user changes are reverted.

  ## Reporting Format

  For each finding, produce:

  - Title, severity, affected surface, and exploit preconditions.
  - Evidence from passive review and failing RED test.
  - Fix summary and exact tests that prove the fix.
  - Residual risk and operational guidance.
  - Whether the issue affects code, docs, deployment, dependencies, or all of them.

  ## Assumptions

  - Assessment scope is this local repository and locally launched services only.
  - No real WhatsApp, Signal, WAHA, public endpoints, or third-party systems are attacked.
  - Network-dependent scanner installation and Docker-based dynamic tests require approval during execution.
  - Production means WHATSIGNAL_ENV=production.
  - If WAHA’s actual HMAC format conflicts with timestamp-bound MAC tests, verify against official WAHA behavior before changing compatibility.
