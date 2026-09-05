# Phase 6.3 — Targeted Verification Hardening

## Scope

```
No kernel changes.
No schema changes.
No authority-model changes.
No API contract changes.
Targeted fail-closed API handler hardening + test/infrastructure changes.
```

## Invariant

The concurrency tests must satisfy:
- T-C10: old two-step sequence creates stale intent (deterministic negative control)
- T-C9: SERIALIZABLE conflict detection prevents stale intent (mechanism proof)
- T-C9: production code passes the same invariant (black-box smoke test)

---

## Item 1: API actor_id mismatch regression test

**File:** `api/integration_test.go` (new test)

Add `TestIntegration_AuthorizeAction_ActorIDMismatch`:

1. Create a principal via `createTestPrincipal`
2. Create a target via `createTestTarget`
3. POST to `/v1/authorizations/action` with:
   - `action_source: "user_typed"`
   - valid `scenario_id`, `belief_id`, `action`, `target_id`
   - `actor_id` set to a DIFFERENT principal UUID (not the authenticated one)
4. Expect HTTP 403 with `"actor_id_mismatch"` in the response body

---

## Item 2: API handler fail-closed nil checks

**File:** `api/target.go` (four handlers)

The auth middleware guarantees `AuthFromContext` returns non-nil for
authenticated requests. But defense-in-depth requires fail-closed nil checks
in security-sensitive handlers that dereference the result.

### handleAttachJustification (line 116-117)

Current:
```go
AuthFromContext(r.Context()).PrincipalID
```

Replace with:
```go
principal := AuthFromContext(r.Context())
if principal == nil {
    writeError(w, http.StatusUnauthorized, "missing_principal", "Authentication required", nil)
    return
}
```
Use `principal.PrincipalID` in the kernel call.

### handleRequestAuthorization (line 140-141)

Add nil check after line 140:
```go
if principal == nil {
    writeError(w, http.StatusUnauthorized, "missing_principal", "Authentication required", nil)
    return
}
```

### handleApproveTarget (line 174-175)

Add nil check after line 174:
```go
if principal == nil {
    writeError(w, http.StatusUnauthorized, "missing_principal", "Authentication required", nil)
    return
}
```

### handleRevokeTarget (line 204-205)

Add nil check after line 204:
```go
if principal == nil {
    writeError(w, http.StatusUnauthorized, "missing_principal", "Authentication required", nil)
    return
}
```

All four checks are identical and consistent with the existing nil check in
`handleAuthorizeAction` (api/authorization.go:96-101).

---

## Item 3: Concurrency tests — two proofs

### T-C10: Historical vulnerability (deterministic negative control)

**File:** `kernel/authority_test.go` (new test)

This test reproduces the exact old vulnerable sequence and proves it creates
a stale intent. It is deterministic because each step is a separate
transaction that fully commits before the next starts.

**Setup:**
1. Create principal, promote belief, create target, attach justification,
   request authorization, approve — full authority chain

**Execute the old two-step sequence:**
2. `kernel.Authorize(ctx, targetID, tuple)` → returns `Allowed: true`,
   transaction COMMITS
3. `kernel.RevokeTarget(ctx, targetID, principalID, "test")` → transaction
   COMMITS — revocation now exists
4. `kernel.IntentOnPromoted(ctx, scenarioID, beliefID, action)` → transaction
   COMMITS — intent INSERT succeeds because gate FK only checks belief
   status (still promoted), NOT authority (revoked)

**Assert:**
5. `Authorize` returned `Allowed: true` (authority was valid at step 2)
6. `IntentOnPromoted` returned nil (insert succeeded)
7. Query `action_intent` directly: **one live intent exists** for this
   belief_id — this is the stale-authority intent

**Why this is deterministic:** Each kernel call is an independent
`crdb.ExecuteTx` transaction. They execute sequentially. The revocation
commits before IntentOnPromoted starts. The gate FK only checks
`belief.status = 'promoted'`, not `target_revocation`. The INSERT succeeds.

**Why the new code does NOT have this vulnerability:**
`AuthorizeAndCreateIntent` wraps both authority evaluation and intent
creation in ONE SERIALIZABLE transaction. If revocation commits during
the transaction, CockroachDB detects the write-read conflict and returns
40001, triggering retry. On fresh state, the revocation exists and
authority is denied.

### T-C9: Atomicity proof (mechanism + black-box)

**File:** `kernel/authority_test.go` (new test)

#### Part A — Mechanism proof (raw SQL, deterministic)

Uses two raw database connections to prove that SERIALIZABLE isolation
prevents committing a stale intent after a conflicting revocation. This
is the same mechanism that `AuthorizeAndCreateIntent` relies on.

**Setup:**
1. Create principal, promote belief, create target, attach justification,
   request authorization, approve — full authority chain
2. Open two separate `*sql.DB` connections to the test database

**Execute with controlled ordering:**
3. Conn1: `BEGIN` (SERIALIZABLE)
4. Conn1: Execute `sqlAuthorizeResolve` with targetID — reads authority,
   finds it valid (target activated, no revocation, snapshot matches)
5. Conn2: Execute `sqlRevokeTarget` with targetID — inserts revocation
6. Conn2: `COMMIT` — revocation is now committed
7. Conn1: Execute `sqlIntentOnPromoted` with scenarioID, beliefID, action
8. Conn1: `COMMIT`

**Assert (the final invariant, not a specific SQLSTATE):**
9. The transaction containing the authority read cannot successfully commit
   a stale intent after the revocation has committed. Acceptable
   implementation-level outcomes include:
   - SQLSTATE `40001` on the conflicting statement or COMMIT (serialization
     conflict);
   - transaction abort requiring retry;
   - equivalent serialization failure recognized by the driver.
10. No committed live intent exists for this belief_id on superseded authority

**Why this is deterministic:** SERIALIZABLE isolation in CockroachDB
detects write-read conflicts. Conn1's transaction reads the absence of
`target_revocation` for this target (in `sqlAuthorizeResolve`). Conn2
writes to `target_revocation`. When Conn1 tries to commit, CockroachDB
detects the conflict. The exact error (40001, abort, retry) depends on
driver/protocol details, but the outcome is guaranteed: a stale intent
cannot be committed.

**Why this proves AuthorizeAndCreateIntent's atomicity:**
`AuthorizeAndCreateIntent` wraps `authorizeWithinTx` (which executes
`sqlAuthorizeResolve`) and `createIntentWithinTx` (which executes
`sqlIntentOnPromoted`) in ONE `crdb.ExecuteTx` transaction. Part A proves
that if a revocation commits between these two SQL statements, the
transaction cannot commit a stale intent. On retry with fresh state, the
revocation exists and authority is denied.

#### Part B — Black-box smoke test

**Setup:**
1. Create principal, promote belief, create target, activate target

**Race:**
2. Goroutine A: `st.AuthorizeAndCreateIntent(ctx, targetID, tuple, scenarioID, beliefID, action)`
3. Goroutine B: `st.RevokeTarget(ctx, targetID, principalID, "concurrent revoke")`
4. `sync.WaitGroup` + buffered `chan error` (cap 2) for synchronization
5. Record the results from both goroutines (return values + errors)

**Assert (the ordering-aware invariant):**

The test must capture enough state to distinguish valid from invalid intents.
A valid intent is one created while authority was still active. An invalid
(stale) intent is one created after the revocation became authoritative.

Record:
- `authorizeResult`: the `AuthorizeResult` from `AuthorizeAndCreateIntent`
- `revokeErr`: the error from `RevokeTarget`
- `intentCount`: `SELECT count(*) FROM action_intent WHERE belief_id = $1 AND state = 'live'`
- `revocationExists`: `SELECT count(*) FROM target_revocation WHERE target_id = $1`
- `targetState`: query the target's current lifecycle state

Valid outcomes (exactly one must hold):
- **A. Authorize wins race:** `authorizeResult.Allowed == true`, intent
  exists, revocation also exists (committed after intent). This is valid
  because the intent was created while authority was active.
- **B. Revoke wins / serialization retry:** `authorizeResult.Allowed == false`
  or an error, no intent, revocation exists. This is valid because the
  atomic transaction detected the conflict and denied.

Invalid outcome (must never occur):
- **C. Stale intent:** revocation exists AND an intent was created
  AFTER the revocation. The test captures ordering via the goroutine
  results to detect this.

The test asserts: outcome C never occurs. Either A or B must hold.
This is the exact invariant that distinguishes the atomic implementation
from the old two-transaction sequence (where outcome C is possible).

This test runs as a smoke test. Under SERIALIZABLE, the conflict is
always detected, so the invariant holds on every run. But the formal
proof is in Part A (mechanism) and T-C10 (negative control).

### API-level smoke test

**File:** `api/integration_test.go` (rewrite `TestIntegration_ConcurrentRevokeTarget`)

Simpler HTTP-level concurrency test. Does NOT claim to prove atomicity.
Proves the API does not crash or corrupt state under concurrency.

1. Activate target (full authority chain)
2. Goroutine 1: POST `/v1/authorizations/action`
3. Goroutine 2: POST `/v1/targets/{id}/revoke`
4. Post-race: GET target → state is `"revoked"` or `"activated"` (consistent)
5. Post-race: query `action_intent` → count is 0 or 1 (consistent)

---

## Item 4: Service/ledger behavioral tests

**File:** `service/ledger/ledger_test.go` (new file)

Two focused tests verifying service-level observable behavior. Code review
establishes that the service delegates to the kernel atomic primitive.

### `TestLedger_AuthorizeAndCreateIntent_Allowed`

1. Create principal, target, promote belief, activate target (full chain)
2. Call `ledgerSvc.AuthorizeAndCreateIntent` with valid tuple
3. Assert: `decision.Allowed == true`
4. Assert: `decision.IntentState == "live"`
5. Query `action_intent` directly — one row exists for this belief_id

### `TestLedger_AuthorizeAndCreateIntent_Denied`

1. Create principal, target (NOT activated)
2. Call `ledgerSvc.AuthorizeAndCreateIntent` with the tuple
3. Assert: `decision.Allowed == false`
4. Assert: `decision.Reason` is non-empty
5. Query `action_intent` directly — zero rows for this belief_id

---

## Files modified

| File | Change |
|---|---|
| `api/integration_test.go` | Add actor_id mismatch test, rewrite concurrency smoke test |
| `api/target.go` | Add four nil checks (fail-closed 401) |
| `kernel/authority_test.go` | Add T-C9 (Part A mechanism + Part B smoke) and T-C10 (negative control) |
| `service/ledger/ledger_test.go` | NEW — two behavioral tests |

## Files NOT modified

- No kernel production code
- No schema
- No API contract
- No MCP handlers

---

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
go test -count=1 -v -run TestIntegration_AuthorizeAction_ActorIDMismatch ./api/...
go test -count=1 -v -run TestIntegration_ConcurrentRevokeTarget ./api/...
go test -count=1 -v -run TestTC9_AuthorizeAndCreateIntent_X_Revoke ./kernel/...
go test -count=1 -v -run TestTC10_TwoTransactionStaleAuthority ./kernel/...
go test -count=1 -v -run TestLedger_AuthorizeAndCreateIntent ./service/ledger/...
```
