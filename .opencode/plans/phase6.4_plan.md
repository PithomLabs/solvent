# Phase 6.4 — Explicit Authority Serialization

## Finding F-NEW

```
F-NEW: SERIALIZABLE predicate-read assumption is invalid

Severity: CRITICAL
Classification: Phase 4B security correctness

Finding:
AuthorizeAndCreateIntent relies on a NOT EXISTS predicate over
target_revocation to serialize against a concurrent revocation.
Actual CockroachDB behavior allows the revocation INSERT to commit
without aborting the authorization transaction.

Impact:
A live action_intent can be committed after target authority has
already been revoked.

Status: CONFIRMED by TestTC9_PartA_SerializablePredicateConflictEvidence
```

## Root cause

Both `authorizeWithinTx` and `RevokeTarget` operate on `authority_target` but neither acquires a lock on it:

- `authorizeWithinTx`: plain SELECT from `target_activation`/`target_snapshot` with `NOT EXISTS target_revocation` — no lock
- `RevokeTarget`: `SELECT count(*)` on `target_activation`/`target_revocation` then INSERT into `target_revocation` — no lock

CockroachDB SERIALIZABLE does not detect NOT EXISTS predicate conflicts on concurrent row inserts. Both transactions commit.

## Solution

Extend the existing `FOR UPDATE` locking pattern (already used by `AttachJustification` and `Approve`) to both `AuthorizeAndCreateIntent` and `RevokeTarget`. Both operations lock `authority_target.target_id` (PRIMARY KEY) before reading authority state.

### Existing pattern (already in codebase)

```sql
-- sqlAttachJustificationLock (sql.go:120-125)
SELECT target_id FROM authority_target
WHERE target_id = $1::UUID FOR UPDATE
```

Used by `AttachJustification` (authority.go:118) to serialize against `Approve`.

```sql
-- sqlApproveReadTarget (sql.go:140-146)
SELECT principal_id, resource_type, ...
FROM authority_target
WHERE target_id = $1::UUID FOR UPDATE
```

Used by `Approve` (authority.go:253) to serialize against `AttachJustification`.

### New serialization

**Both operations acquire the same lock:**

```
AuthorizeAndCreateIntent
    ↓
SELECT target_id FROM authority_target WHERE target_id = $1 FOR UPDATE
    ↓
read activation/snapshot/revocation (authority evaluation)
    ↓
insert action_intent
    ↓
commit (releases lock)
```

```
RevokeTarget
    ↓
SELECT target_id FROM authority_target WHERE target_id = $1 FOR UPDATE
    ↓
check activation exists
    ↓
check not already revoked
    ↓
insert target_revocation
    ↓
commit (releases lock)
```

### Ordering guarantees

**Case A — authorize wins:**
1. `AuthorizeAndCreateIntent` acquires lock
2. Evaluates authority (no revocation exists)
3. Creates intent
4. Commits → releases lock
5. `RevokeTarget` acquires lock
6. Inserts revocation
7. Commits

Result: intent created before revocation → **valid**

**Case B — revoke wins:**
1. `RevokeTarget` acquires lock
2. Checks activation (exists), checks revocation (none)
3. Inserts revocation
4. Commits → releases lock
5. `AuthorizeAndCreateIntent` acquires lock
6. Evaluates authority → sees revocation
7. DENY

Result: no intent → **correct**

**Forbidden state (stale intent) is impossible** because both operations must serialize on the same row lock.

## Implementation

### 1. No new SQL constant needed

Reuse `sqlAttachJustificationLock` (identical SQL):
```sql
SELECT target_id FROM authority_target WHERE target_id = $1::UUID FOR UPDATE
```

### 2. Modify `AuthorizeAndCreateIntent` (kernel/authority.go:466-496)

Add lock acquisition at the start of the transaction, before `authorizeWithinTx`:

```go
func (s *Store) AuthorizeAndCreateIntent(
    ctx context.Context,
    targetID string,
    tuple AuthorityTuple,
    scenarioID, beliefID, action string,
) (AuthorizeResult, error) {
    var result AuthorizeResult
    err := crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        // LOCK: serialize against concurrent RevokeTarget.
        var locked string
        if err := tx.QueryRowContext(ctx,
            sqlAttachJustificationLock, targetID).Scan(&locked); err != nil {
            if errors.Is(err, sql.ErrNoRows) {
                return ErrTargetNotFound
            }
            return err
        }

        // 1. Evaluate authority (unchanged).
        var err error
        result, err = authorizeWithinTx(ctx, tx, targetID, tuple)
        // ... rest unchanged
    })
}
```

### 3. Modify `RevokeTarget` (kernel/authority.go:500-526)

Add lock acquisition at the start of the transaction, before the count checks:

```go
func (s *Store) RevokeTarget(ctx context.Context, targetID, revokedBy, reason string) error {
    return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        // LOCK: serialize against concurrent AuthorizeAndCreateIntent.
        var locked string
        if err := tx.QueryRowContext(ctx,
            sqlAttachJustificationLock, targetID).Scan(&locked); err != nil {
            if errors.Is(err, sql.ErrNoRows) {
                return ErrTargetNotFound
            }
            return err
        }

        // Verify target has an activation (unchanged).
        // Verify not already revoked (unchanged).
        // Insert revocation (unchanged).
    })
}
```

### 4. Update `AuthorizeAndCreateIntent` doc comment

Remove the incorrect claim about SERIALIZABLE isolation. Replace with:

```go
// Security guarantee: both the authority evaluation and the intent creation
// are protected by a FOR UPDATE lock on authority_target, which serializes
// against a concurrent RevokeTarget. If a revocation is committed before the
// lock is acquired, the authority evaluation sees it and denies. If the lock
// is acquired first, the revocation blocks until the intent is committed.
```

### 5. Rewrite T-C9 Part A (kernel/authority_test.go)

Replace the invalid predicate-conflict test with a lock-based mechanism proof.

**Timeout policy for lock-contention tests:** Raw SQL lock tests use bounded contexts (e.g., `context.WithTimeout(ctx, 5*time.Second)`) so a broken lock implementation cannot hang the suite indefinitely. If the lock is not acquired within the deadline, the test fails with a clear diagnostic rather than hanging.

```
T-C9 Part A — Lock-based serialization proof (authorize-first)

Setup:
1. Create principal, promote belief, create target, attach justification,
   request authorization, approve — full authority chain
2. Open two raw database connections
3. Create bounded contexts (5s timeout) for lock-sensitive operations

Execute with controlled ordering:
3. Conn1: BEGIN (SERIALIZABLE)
4. Conn1: SELECT ... FOR UPDATE on authority_target (acquires lock)
5. Conn1: Execute sqlAuthorizeResolve (reads authority — valid)
6. Conn2: BEGIN (SERIALIZABLE)
7. Conn2: SELECT ... FOR UPDATE on authority_target → BLOCKS (waits for Conn1)
8. Conn1: Execute sqlIntentOnPromoted (creates intent)
9. Conn1: COMMIT (releases lock)
10. Conn2: lock acquired, Execute sqlRevokeTarget (inserts revocation)
11. Conn2: COMMIT

Assert:
12. Intent exists (created by Conn1 before lock release)
13. Revocation exists (created by Conn2 after lock acquisition)
14. Intent was created while authority was valid → valid pre-revocation intent
```

And add a revoke-first test:

```
T-C9 Part A2 — Lock-based serialization proof (revoke-first)

Setup: same as above, with bounded contexts

Execute with reverse ordering:
3. Conn1: BEGIN (SERIALIZABLE)
4. Conn1: SELECT ... FOR UPDATE on authority_target (acquires lock)
5. Conn1: Execute sqlRevokeTarget (inserts revocation)
6. Conn1: COMMIT (releases lock)
7. Conn2: BEGIN (SERIALIZABLE)
8. Conn2: SELECT ... FOR UPDATE on authority_target (acquires lock)
9. Conn2: Execute sqlAuthorizeResolve → finds revocation → DENY
10. Conn2: COMMIT (no intent created)

Assert:
11. No live intent exists
12. Revocation exists
13. Authority correctly denied after revocation
```

### 6. Rewrite T-C9 Part B (kernel/authority_test.go)

Black-box production invariant test, NOT the mechanism proof. The mechanism proof is T-C9 Part A (deterministic lock-based ordering). Part B verifies that the production `AuthorizeAndCreateIntent` + `RevokeTarget` implementation obeys the same serialization invariant established by Part A, without assuming any specific CockroachDB isolation behavior.

Keep as black-box smoke test with MVCC timestamp ordering verification. The test already works correctly — it verifies `stale=false` via MVCC timestamps. No changes needed beyond the documentation update.

### 7. Keep T-C10 unchanged

T-C10 proves the old vulnerability (two-step sequence without lock). It remains the deterministic negative control.

## Files modified

| File | Change |
|---|---|
| `kernel/authority.go` | Add `FOR UPDATE` lock to `AuthorizeAndCreateIntent` and `RevokeTarget`; update doc comment |
| `kernel/authority_test.go` | Rewrite T-C9 Part A with lock-based mechanism proof; add revoke-first test |

## Files NOT modified

- No schema changes (authority_target already has PRIMARY KEY)
- No API contract changes
- No service layer changes
- No MCP changes

## Verification

```
go build ./...
go vet ./...
go test -count=1 -p 1 ./...
go test -count=1 ./...
go test -count=1 ./...
```

Key spot-checks:
```
go test -count=1 -v -run TestTC9_PartA ./kernel/...
go test -count=1 -v -run TestTC9_PartA2 ./kernel/...
go test -count=1 -v -run TestTC9_PartB ./kernel/...
go test -count=1 -v -run TestTC10 ./kernel/...
```
