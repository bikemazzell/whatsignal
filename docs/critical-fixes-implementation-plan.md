# Critical Fixes Implementation Plan

## Summary

This plan addresses the 5 most critical issues in the WhatSignal codebase, sequenced by priority, dependencies, and risk. Issues 1+3 are the most likely root cause of intermittent Signal → WhatsApp forwarding failures.

| Phase | Issue | Severity | Fix | Effort | Risk |
|-------|-------|----------|-----|--------|------|
| 1 | HealthCheck consumes messages + trips poll breaker | P0+P1 | Switch to `/v1/about`, use send breaker | Low | Low |
| 2 | Circuit breaker data race on `requestCount` | P2 | Consistent mutex-based access | Low | Low |
| 3 | Silent message processing failures | P1 | Per-message retry with metrics | Medium | Medium |
| 4 | No message persistence before processing | P1 | Pending message queue table | High | High |

---

## Phase 1: Fix HealthCheck (Issues 1 + 3)

**Priority:** P0 — Fixes both message consumption and circuit breaker interference.

### Root Cause

`SignalClient.HealthCheck()` calls the destructive `/v1/receive/{phoneNumber}` endpoint via `doPollRequestWithCircuitBreaker()`. This:
1. Permanently consumes all pending Signal messages (discards response body)
2. Uses the poll circuit breaker, so health check failures can block message polling

### Files to Modify

| File | Symbol | Change |
|------|--------|--------|
| `pkg/signal/client.go` | `HealthCheck()` | Switch endpoint to `/v1/about`, use `doRequestWithCircuitBreaker` |
| `pkg/signal/client_edge_cases_test.go` | `TestHealthCheck*` | Update test assertions for new endpoint |

### Implementation

**Step 1:** Replace the `HealthCheck` method in `pkg/signal/client.go`:

```go
func (c *SignalClient) HealthCheck(ctx context.Context) error {
    endpoint := fmt.Sprintf("%s/v1/about", c.baseURL)

    req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
    if err != nil {
        return fmt.Errorf("failed to create Signal health check request: %w", err)
    }

    req.Header.Set("Accept", "application/json")

    resp, err := c.doRequestWithCircuitBreaker(ctx, req)
    if err != nil {
        return fmt.Errorf("signal API health check failed: %w", err)
    }
    defer func() { _ = resp.Body.Close() }()

    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
        return nil
    }

    body, readErr := io.ReadAll(resp.Body)
    if readErr != nil {
        return fmt.Errorf("signal API health check returned status %d (failed to read body: %v)", resp.StatusCode, readErr)
    }
    return fmt.Errorf("signal API health check returned status %d: %s", resp.StatusCode, string(body))
}
```

**Step 2:** Update health check tests to verify the `/v1/about` endpoint is called and `/v1/receive` is NOT called.

### Dependencies

None — this is the first phase.

### Quality Gates

- [ ] `go build ./...` — zero errors
- [ ] `go test ./...` — 100% pass rate
- [ ] `go vet ./...` — clean output
- [ ] `go test -race ./...` — no race conditions
- [ ] Verify test server receives requests to `/v1/about`, not `/v1/receive`
- [ ] Verify health check does NOT consume messages (test with mock server that tracks calls)

### Rollback Strategy

- **Verify:** Health endpoint returns 200, Signal messages continue to be polled and forwarded
- **Monitor:** `signal_poll_messages_received` metric should show consistent message flow
- **Rollback:** Revert the two-line change in `HealthCheck()` — no schema or state changes involved

---

## Phase 2: Fix Circuit Breaker Data Race (Issue 5)

**Priority:** P2 — Low severity but trivial to fix and eliminates a `-race` detector violation.

### Root Cause

`requestCount` is incremented with `atomic.AddUint32` in `Execute()` (no mutex held) but read as a plain field in `GetStats()` (under `RLock`). Similarly, `failures` and `successCount` use `atomic.AddUint32` inside mutex-protected methods, creating an inconsistent access pattern.

### Files to Modify

| File | Symbol | Change |
|------|--------|--------|
| `pkg/circuitbreaker/circuit_breaker.go` | `Execute()` | Increment `requestCount` under write lock |
| `pkg/circuitbreaker/circuit_breaker.go` | `onFailure()` | Use plain increment for `failures` (already under lock) |
| `pkg/circuitbreaker/circuit_breaker.go` | `onSuccess()` | Use plain increment for `successCount` (already under lock) |

### Implementation

**Step 1:** In `Execute()`, move the `requestCount` increment under the mutex:

```go
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
    if !cb.allowRequest() {
        return &CircuitBreakerError{
            Name:  cb.name,
            State: cb.GetState(),
        }
    }

    cb.mu.Lock()
    cb.requestCount++
    cb.mu.Unlock()

    err := fn(ctx)

    if err != nil {
        cb.onFailure()
        return err
    }

    cb.onSuccess()
    return nil
}
```

**Step 2:** In `onFailure()`, replace `atomic.AddUint32(&cb.failures, 1)` with `cb.failures++` (already under `Lock`).

**Step 3:** In `onSuccess()`, replace `atomic.AddUint32(&cb.successCount, 1)` with `cb.successCount++` in the `StateClosed` case (already under `Lock`).

**Step 4:** Remove the `"sync/atomic"` import if no longer used.

### Dependencies

None — independent of other phases.

### Quality Gates

- [ ] `go build ./...` — zero errors
- [ ] `go test ./...` — 100% pass rate
- [ ] `go vet ./...` — clean output
- [ ] `go test -race ./...` — **critical**: must pass with zero race conditions
- [ ] `TestConcurrentAccess` passes under race detector
- [ ] `TestGetStats` validates correct counts after mixed success/failure operations

### Rollback Strategy

- **Verify:** `go test -race ./pkg/circuitbreaker/...` passes cleanly
- **Monitor:** Circuit breaker stats in `/metrics` endpoint show accurate counts
- **Rollback:** Revert to atomic operations — the race is benign in practice (stats only)

---

## Phase 3: Add Per-Message Retry (Issue 2)

**Priority:** P1 — Prevents permanent message loss from transient processing failures.

### Root Cause

In `PollSignalMessages()`, worker goroutines call `ProcessIncomingSignalMessageWithDestination()` and only log errors. Since `/v1/receive` is destructive, failed messages are permanently lost.

### Files to Modify

| File | Symbol | Change |
|------|--------|--------|
| `internal/service/message_service.go` | `PollSignalMessages()` | Add per-message retry loop in worker goroutines |
| `internal/constants/defaults.go` | (new constant) | Add `DefaultMessageProcessRetryAttempts` |
| `internal/service/message_service_test.go` | (existing tests) | Add test for retry behavior on transient failures |

### Implementation

**Step 1:** Add constant in `internal/constants/defaults.go`:

```go
DefaultMessageProcessRetryAttempts = 3
DefaultMessageProcessRetryBackoffMs = 500
```

**Step 2:** Replace the worker goroutine body in `PollSignalMessages()`:

```go
go func(m signaltypes.SignalMessage, dest string) {
    defer wg.Done()
    defer func() { <-sem }()

    chatKey := m.Sender + ":" + dest
    chatLock := s.chatLockManager.getLock(chatKey)
    chatLock.Lock()
    defer chatLock.Unlock()

    var lastErr error
    maxAttempts := constants.DefaultMessageProcessRetryAttempts
    backoff := time.Duration(constants.DefaultMessageProcessRetryBackoffMs) * time.Millisecond

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        if err := s.ProcessIncomingSignalMessageWithDestination(ctx, &m, dest); err != nil {
            lastErr = err
            if attempt < maxAttempts {
                s.logger.WithFields(logrus.Fields{
                    "messageID": m.MessageID,
                    "attempt":   attempt,
                }).WithError(err).Warn("Message processing failed, retrying")
                select {
                case <-ctx.Done():
                    s.logger.WithField("messageID", m.MessageID).Error("Context cancelled during message retry")
                    metrics.IncrementCounter("signal_message_process_failures", map[string]string{
                        "reason": "context_cancelled",
                    }, "Signal message processing failures")
                    return
                case <-time.After(backoff):
                    backoff *= 2
                }
            }
        } else {
            if attempt > 1 {
                metrics.IncrementCounter("signal_message_process_retries_succeeded", nil,
                    "Signal messages that succeeded after retry")
            }
            return
        }
    }

    s.logger.WithFields(logrus.Fields{
        "messageID": m.MessageID,
        "attempts":  maxAttempts,
    }).WithError(lastErr).Error("Message processing failed after all retry attempts")
    metrics.IncrementCounter("signal_message_process_failures", map[string]string{
        "reason": "retries_exhausted",
    }, "Signal message processing failures")
}(msg, destination)
```

### Dependencies

- Phase 1 should be completed first (reduces the frequency of messages needing retry by eliminating health-check-induced losses)

### Quality Gates

- [ ] `go build ./...` — zero errors
- [ ] `go test ./...` — 100% pass rate
- [ ] `go vet ./...` — clean output
- [ ] `go test -race ./...` — no race conditions
- [ ] Unit test: mock `ProcessIncomingSignalMessageWithDestination` to fail N times then succeed — verify retry count
- [ ] Unit test: mock to fail permanently — verify all attempts exhausted and metric incremented
- [ ] Unit test: context cancellation during retry — verify clean exit
- [ ] Verify `signal_message_process_failures` and `signal_message_process_retries_succeeded` metrics appear

### Rollback Strategy

- **Verify:** `signal_message_process_retries_succeeded` metric shows retries recovering messages
- **Monitor:** `signal_message_process_failures{reason="retries_exhausted"}` should be near zero
- **Rollback:** Set `DefaultMessageProcessRetryAttempts = 1` to disable retry without code changes

---

## Phase 4: Add Message Persistence Before Processing (Issue 4)

**Priority:** P1 — Prevents message loss on crash between receive and process.

### Root Cause

After `ReceiveMessages()` returns, messages exist only in memory. If the process crashes before `ProcessIncomingSignalMessageWithDestination()` completes, messages are permanently lost because `/v1/receive` already consumed them.

### Files to Modify

| File | Symbol | Change |
|------|--------|--------|
| `internal/database/queries.go` | (new queries) | Add pending_signal_messages table queries |
| `internal/database/database.go` | (new methods) | `SavePendingMessages`, `GetPendingMessages`, `DeletePendingMessage` |
| `internal/database/migrations.go` | (new migration) | Create `pending_signal_messages` table |
| `internal/service/message_service.go` | `PollSignalMessages()` | Persist before processing, delete after success |
| `internal/service/message_service.go` | `Database` interface | Add new methods |

### Implementation

**Step 1:** Add migration for the `pending_signal_messages` table:

```sql
CREATE TABLE IF NOT EXISTS pending_signal_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id TEXT NOT NULL,
    sender TEXT NOT NULL,
    message TEXT,
    group_id TEXT,
    timestamp INTEGER NOT NULL,
    raw_json TEXT NOT NULL,
    destination TEXT NOT NULL,
    retry_count INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(message_id)
);
CREATE INDEX IF NOT EXISTS idx_pending_signal_created_at ON pending_signal_messages(created_at);
```

**Step 2:** Add database methods:

```go
func (d *Database) SavePendingMessages(ctx context.Context, messages []PendingSignalMessage) error
func (d *Database) GetPendingMessages(ctx context.Context, limit int) ([]PendingSignalMessage, error)
func (d *Database) DeletePendingMessage(ctx context.Context, messageID string) error
func (d *Database) IncrementPendingRetryCount(ctx context.Context, messageID string) error
```

**Step 3:** Modify `PollSignalMessages()` flow:

```
1. ReceiveMessages() → get messages from Signal API
2. SavePendingMessages() → persist to database (crash-safe)
3. For each pending message:
   a. Process message (with retry from Phase 3)
   b. On success: DeletePendingMessage()
   c. On failure: IncrementPendingRetryCount()
4. On startup: GetPendingMessages() → reprocess any orphaned messages
```

**Step 4:** Add startup recovery in the `SignalPoller.Start()` method to reprocess pending messages.

### Dependencies

- Phase 1 must be completed (reduces message loss surface)
- Phase 3 should be completed (retry logic is used within the persistence flow)

### Quality Gates

- [ ] `go build ./...` — zero errors
- [ ] `go test ./...` — 100% pass rate
- [ ] `go vet ./...` — clean output
- [ ] `go test -race ./...` — no race conditions
- [ ] Unit test: save pending messages, simulate crash (skip processing), verify messages recoverable
- [ ] Unit test: full flow — receive → persist → process → delete
- [ ] Unit test: partial failure — some messages processed, crash, verify remaining messages recovered
- [ ] Integration test: verify migration creates table correctly
- [ ] Verify `pending_signal_messages` table is empty during normal operation (all messages processed)

### Rollback Strategy

- **Verify:** `pending_signal_messages` table stays near-empty during normal operation
- **Monitor:** Query `SELECT COUNT(*) FROM pending_signal_messages` — should be 0 or near 0
- **Rollback:** Remove the persistence calls from `PollSignalMessages()` — table remains but is unused. Drop table in a subsequent migration if needed.

---

## Implementation Order

```
Phase 1 (Issues 1+3) ──→ Phase 2 (Issue 5) ──→ Phase 3 (Issue 2) ──→ Phase 4 (Issue 4)
     P0, Low risk            P2, Low risk          P1, Medium risk       P1, High risk
     ~2 hours                ~1 hour               ~4 hours              ~2 days
```

Phases 1 and 2 are independent and could be done in parallel, but sequential execution is recommended to keep each change isolated and verifiable.

## Global Quality Gates (After All Phases)

After all phases are complete, run the full quality gate suite:

```bash
go build ./...
go test ./...
go vet ./...
go test -race ./...
```

Additionally:
- Manual test: send 10 Signal messages while health checks are running — verify all 10 arrive on WhatsApp
- Manual test: kill the process mid-poll, restart — verify pending messages are recovered (Phase 4)
- Monitor `signal_poll_messages_received` for 24 hours — verify no drops compared to pre-fix baseline


---

## Internal Review Notes

The following edge cases and corrections were identified during review:

### Phase 3: Retry Lock Duration

The per-message retry holds the per-chat lock for the entire retry duration (up to ~3.5 seconds with 3 attempts and exponential backoff). This is **intentional** — it preserves message ordering within a chat. However, it means other messages for the same sender+destination pair are blocked during retries. This is acceptable because:
- Retry only triggers on transient failures (rare)
- Message ordering is more important than latency
- The backoff is bounded (max ~3.5s total)

### Phase 4: Encryption of Pending Messages

The `pending_signal_messages` table must encrypt sensitive fields (`sender`, `message`, `raw_json`) using the existing `encryptor` pattern from `internal/database/database.go`. Store lookup hashes for `message_id` to enable efficient queries without decryption.

### Phase 4: Unique Constraint

The `UNIQUE(message_id)` constraint should be `UNIQUE(message_id, destination)` to handle the edge case where the same Signal message ID could appear in different routing contexts.

### Phase 4: Graceful Degradation on Persistence Failure

If `SavePendingMessages()` fails (e.g., disk full, database locked), `PollSignalMessages()` should fall back to in-memory processing (current behavior) rather than dropping messages. Log a warning and increment a `signal_pending_save_failures` metric.
