# Architecture Review Fixes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all verified CRITICAL and HIGH findings from the architecture review, plus high-value MEDIUM fixes.

**Architecture:** Fixes are grouped into independent tasks that can be parallelized. Each task targets a single concern: database integrity, poller deadlock, PII logging, and security hardening.

**Tech Stack:** Go, SQLite, logrus, HMAC-SHA256 for lookup hashing

**Quality gate:** `make ci` must pass after each task. All existing tests must continue to pass.

---

## Task 1: Fix broken `GetContactByName` + encrypt `short_name` [CRITICAL C1] -- DONE

**Problem:** `SaveContact` encrypts `name` and `push_name` with random-nonce AES-GCM, but `GetContactByName` queries those columns with plaintext `WHERE name = ?`. The query never matches. Also, `short_name` is stored in plaintext while all other PII is encrypted.

**Fix:** Add `name_hash`, `push_name_hash`, `short_name_hash` columns to `contacts`. Encrypt `short_name`. Query by hash columns.

**Files:**
- Create: `scripts/migrations/004_add_contact_name_hashes.sql`
- Modify: `internal/database/queries.go:119-125` (SelectContactByNameQuery)
- Modify: `internal/database/queries.go:105-110` (InsertOrReplaceContactQuery)
- Modify: `internal/database/database.go:727-759` (SaveContact)
- Modify: `internal/database/database.go:821-841` (GetContactByName)
- Test: `internal/database/database_test.go` (add test for GetContactByName round-trip)

- [ ] **Step 1: Write failing test for `GetContactByName` round-trip**

Add to `internal/database/database_test.go`:

```go
func TestGetContactByName_RoundTrip(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	contact := &models.Contact{
		ContactID:   "12345@c.us",
		PhoneNumber: "12345",
		Name:        "Alice",
		PushName:    "Alice Push",
		ShortName:   "Ali",
		IsBlocked:   false,
		IsGroup:     false,
		IsMyContact: true,
	}
	err := db.SaveContact(context.Background(), contact)
	require.NoError(t, err)

	// Look up by Name
	found, err := db.GetContactByName(context.Background(), "Alice")
	require.NoError(t, err)
	require.NotNil(t, found, "GetContactByName should find contact by name")
	assert.Equal(t, "12345@c.us", found.ContactID)

	// Look up by PushName
	found2, err := db.GetContactByName(context.Background(), "Alice Push")
	require.NoError(t, err)
	require.NotNil(t, found2, "GetContactByName should find contact by push_name")

	// Look up by ShortName
	found3, err := db.GetContactByName(context.Background(), "Ali")
	require.NoError(t, err)
	require.NotNil(t, found3, "GetContactByName should find contact by short_name")

	// Miss
	notFound, err := db.GetContactByName(context.Background(), "Nobody")
	require.NoError(t, err)
	assert.Nil(t, notFound)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=1 go test ./internal/database/ -run TestGetContactByName_RoundTrip -v -count=1`
Expected: FAIL — nil result because query hits encrypted columns with plaintext

- [ ] **Step 3: Create migration `004_add_contact_name_hashes.sql`**

Create `scripts/migrations/004_add_contact_name_hashes.sql`:

```sql
-- Add hash columns for encrypted name lookups in contacts table
ALTER TABLE contacts ADD COLUMN name_hash TEXT;
ALTER TABLE contacts ADD COLUMN push_name_hash TEXT;
ALTER TABLE contacts ADD COLUMN short_name_hash TEXT;

CREATE INDEX IF NOT EXISTS idx_contact_name_hash ON contacts(name_hash);
CREATE INDEX IF NOT EXISTS idx_contact_push_name_hash ON contacts(push_name_hash);
CREATE INDEX IF NOT EXISTS idx_contact_short_name_hash ON contacts(short_name_hash);
```

- [ ] **Step 4: Update `InsertOrReplaceContactQuery` to include hash columns**

In `internal/database/queries.go`, replace existing query:

```go
InsertOrReplaceContactQuery = `
	INSERT OR REPLACE INTO contacts (
		contact_id, phone_number, name, push_name, short_name,
		is_blocked, is_group, is_my_contact, cached_at,
		name_hash, push_name_hash, short_name_hash
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?)
`
```

- [ ] **Step 5: Update `SelectContactByNameQuery` to use hash columns**

In `internal/database/queries.go`, replace existing query:

```go
SelectContactByNameQuery = `
	SELECT contact_id, phone_number, name, push_name, short_name,
		   is_blocked, is_group, is_my_contact, cached_at
	FROM contacts
	WHERE name_hash = ? OR push_name_hash = ? OR short_name_hash = ?
	LIMIT 1
`
```

- [ ] **Step 6: Update `SaveContact` to hash name fields and encrypt `short_name`**

In `internal/database/database.go`, update `SaveContact` (around line 727) to:
- Encrypt `short_name` (currently stored plaintext)
- Compute HMAC hashes for `name`, `push_name`, `short_name`
- Pass all new values to the updated query

Key changes:
```go
encryptedShortName, err := d.encryptor.EncryptIfEnabled(contact.ShortName)
// ... compute nameHash, pushNameHash, shortNameHash via d.encryptor.LookupHash()
_, err = d.db.ExecContext(ctx, query,
    encryptedContactID, encryptedPhone, encryptedName, encryptedPushName, encryptedShortName,
    contact.IsBlocked, contact.IsGroup, contact.IsMyContact,
    nameHash, pushNameHash, shortNameHash)
```

- [ ] **Step 7: Rewrite `GetContactByName` to hash the search term and decrypt results**

In `internal/database/database.go`, replace `GetContactByName` to:
- Compute HMAC hash of the search name
- Pass hash to all three WHERE conditions
- Scan encrypted fields and decrypt them
- Use `errors.Is(err, sql.ErrNoRows)` instead of string comparison (fixes M11)

- [ ] **Step 8: Run test to verify it passes**

Run: `CGO_ENABLED=1 go test ./internal/database/ -run TestGetContactByName_RoundTrip -v -count=1`
Expected: PASS

- [ ] **Step 9: Run full test suite and fix any broken tests**

Run: `CGO_ENABLED=1 go test ./... -count=1 -timeout 120s`
Expected: All pass. Existing tests mocking `SaveContact` may need arg count updates.

- [ ] **Step 10: Commit**

---

## Task 2: Fix `SignalPoller.Stop()` deadlock [CRITICAL C2] -- DONE

**Problem:** `Stop()` holds `sp.mu.Lock()` while calling `sp.wg.Wait()`. Goroutines running `pollWithRetry()` acquire `sp.mu.Lock()` at lines 373 and 442 before returning, which triggers `defer sp.wg.Done()` in `pollLoop`. Deadlock.

**Fix:** Release the mutex before `wg.Wait()`, re-acquire to update state.

**Verified:** Regression test `TestSignalPoller_Stop_NoDeadlock` passes in 0.1s.

- [x] **Step 1:** Deadlock regression test added
- [x] **Step 2:** `Stop()` fixed — mutex released before `wg.Wait()`
- [x] **Step 3:** Tests pass

---

## Task 3: Fix PII logging [CRITICAL C3 + MEDIUM C4] -- DONE

**Problem:** Raw message envelopes logged at WARN on normal code path (C3). Intermediary phone number logged unmasked at INFO (C4).

**Verified:** All three changes confirmed at exact line numbers. Tests pass.

- [x] **Step 1:** Envelope log split into WARN (no PII) + DEBUG (with envelope)
- [x] **Step 2:** Phone masked via `privacy.MaskPhoneNumber()` in `logFields()`
- [x] **Step 3:** WebSocket URL replaced with `"mode", "websocket"`

---

## Task 4: Fix non-atomic migrations + add `busy_timeout` [HIGH H1 + H3] -- DONE

**Verified:** `applyMigration` now uses `BeginTx`/`Commit`/`Rollback`. `PRAGMA busy_timeout=5000` added at line 105.

- [x] **Step 1:** Migration DDL + tracking wrapped in transaction
- [x] **Step 2:** `PRAGMA busy_timeout=5000` added after WAL mode

---

## Task 5: Fix `updateDeliveryStatusInternal` error masking [HIGH H4] -- DONE

**Verified:** Real DB errors propagated; only "no message found" triggers Signal ID fallback.

- [x] **Step 1:** Error type checked before fallthrough

---

## Task 6: Add `io.LimitReader` to attachment download [MEDIUM M5] -- DONE

**Verified:** `io.ReadAll(io.LimitReader(resp.Body, int64(constants.MaxRecommendedFileSizeBytes)))` at line 769.

- [x] **Step 1:** Read bounded to `MaxRecommendedFileSizeBytes`

---

## Task 7: Add missing `rows.Err()` check [MEDIUM M10] -- DONE

**Verified:** `rows.Err()` check added at line 719.

- [x] **Step 1:** Check added after for loop

---

## Task 8: Drop dead indexes + add delivery monitor index [MEDIUM M7+M8+M9] -- DONE

**Verified:** Migration 005 drops 3 dead indexes, adds 2 composite indexes.

- [x] **Step 1:** Migration created

---

## Task 9: Final quality gate -- DONE

- [x] **Step 1:** Full test suite: 26/26 packages pass, 0 failures
- [x] **Step 2:** All agent changes verified at exact line numbers
- [x] **Step 3:** No regressions — all existing tests still pass
