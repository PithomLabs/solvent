# Adversarial Review 9 — Remediation Plan

**Scope:** Close Finding 1 (MEDIUM verification gap) + Finding 2 (LOW plumbing gap) + Finding 3 (dead reads) + Finding 4 (doc drift) from `docs/OS/adv_review9.md`.

**Principle:** Tests, service/API plumbing, and documentation only. No kernel or schema changes.

---

## Finding 1 — T1→T2 confused-deputy integration test

**The gap:** No test creates two valid approved targets T1/S1 and T2/S2, binds an intent to T1/S1, and then calls `ExecuteAction` against T2/S2. Existing "wrong target" tests use nonexistent targets, so `Authorize` rejects before `ClaimIntent`.

### 1a. Service integration test (canonical security regression)

**File:** `service/authority/authority_integration_test.go`

**Test:** `TestEA03b_ConfusedDeputy`

Purpose: Proves that `ClaimIntent` rejects a T1-bound intent when `ExecuteAction` is called with T2/S2.

Steps:
1. Create scenario, principal, promoted belief (standard setup).
2. Create two approved targets: T1/S1 and T2/S2, both with identical belief/action (`"deploy"`).
3. Create intent I1 bound to T1/S1 via `createLiveIntent(..., t1, snap_t1)`.
4. Call `ExecuteAction(ctx, sid, beliefID, "deploy", t2, principalID, i1, ...)` — target is T2, but intent is bound to T1.
5. Assert:
   - `result.Allowed == false`
   - `rec.Called() == false` (executor never invoked)
   - `result.Error` contains `ErrIntentNotLive` message
   - Intent I1 remains in `live` state (not moved to `executing`)

**Naming:** `TestEA03b_ConfusedDeputy` (sibling of `TestEA03_WrongTarget` which tests nonexistent target).

### 1b. API integration test (plumbing regression)

**File:** `api/authorization_test.go`

**Test:** `TestExecute_ConfusedDeputy`

Purpose: Proves the REST path propagates target/snapshotID correctly and the same rejection surfaces at the HTTP layer.

Prerequisite: Update `newTestServerWithAuth` to register a recording executor (see Finding 2).

Steps:
1. Create scenario, promoted belief, two approved targets T1/S1 and T2/S2.
2. Create intent I1 bound to T1/S1 via direct SQL.
3. POST `/v1/authorizations/execute` with `target_id=T2, intent_id=I1, ...`.
4. Assert:
   - HTTP 200
   - `"allowed": false`
   - Executor recording function not called

---

## Finding 2 — `setupExecutionScenario` creates unbound intents

**File:** `api/authorization_test.go`, `api/helpers_test.go`

**Problem:** `setupExecutionScenario` inserts intents via raw SQL without `target_id`/`snapshot_id`. The empty executor registry causes tests to return before `ClaimIntent` is reached, so the bound-intent path is never exercised at the API layer.

### Changes

1. **`api/helpers_test.go`**: Add `newTestServerWithRecordingExecutor` — like `newTestServerWithAuth` but registers a `executor.RecordingFunc` in the registry. Returns both the `*httptest.Server` and the `*executor.RecordingFunc` so tests can assert on invocation count.

2. **`api/authorization_test.go`**: Add `setupBoundExecutionScenario` alongside `setupExecutionScenario`. It should:
   - Create scenario, belief, approved target (same as current).
   - Create intent via `AuthorizeAndCreateIntent` (the kernel path) or via direct SQL with `target_id`/`snapshot_id` columns, so the intent is genuinely bound.
   - Return the same tuple plus the snapshot_id.

3. **`TestExecute_ConfusedDeputy`** (from Finding 1b) uses the new bound helper and recording executor.

4. **Do not break existing tests.** `setupExecutionScenario` stays as-is for backward compatibility. The new helper is additive.

---

## Finding 3 — Remove dead `snapshot_id` reads

Four locations SELECT `ts.snapshot_id` but never use the variable. The kernel re-derives `snapshot_id` independently inside `Authorize`. Remove the dead reads to enforce one clear rule:

```
REST/MCP → request target → kernel.Authorize → kernel derives authoritative snapshot → ClaimIntent
```

### 3a. `api/authorization.go` — `handleAuthorizeAction` (lines 113–123)

Change the SQL from:
```sql
SELECT ts.consequence_parameters, ts.snapshot_id
FROM target_activation ta
JOIN target_snapshot ts ON ts.target_id = ta.target_id AND ts.snapshot_id = ta.snapshot_id
WHERE ta.target_id = $1::UUID
```
To:
```sql
SELECT ts.consequence_parameters
FROM target_activation ta
JOIN target_snapshot ts ON ts.target_id = ta.target_id AND ts.snapshot_id = ta.snapshot_id
WHERE ta.target_id = $1::UUID
```

Remove `var snapID string` and `&snapID` from the scan. Update the error fallback to only reset `snapParams`. Update the comment to say the kernel derives the authoritative snapshot.

### 3b. `api/authorization.go` — `handleExecuteAction` (lines 226–236)

Same change as 3a.

### 3c. `cmd/solvent-mcp/tools.go` — `handleSolventAuthorizeAction` (lines 247–257)

Same change as 3a.

### 3d. `cmd/solvent-mcp/tools.go` — `handleSolventExecute` (lines 691–701)

Same change as 3a.

### Verification

After removal, `go build ./...` and `go vet ./...` must pass. The handler no longer reads `snapshot_id` — the kernel is the sole authority for snapshot resolution.

---

## Finding 4 — Reconcile plan10.1.md with actual migration

**File:** `docs/OS/plan10.1.md` (lines 399–408)

The plan specifies a 5-column unique index:
```sql
CREATE UNIQUE INDEX IF NOT EXISTS intent_authority_unique
  ON action_intent (scenario_id, belief_id, action, target_id, snapshot_id)
  WHERE state = 'live';
```

The actual migration (`db/009_exact_authority_binding.sql`, lines 33–35) creates:
```sql
CREATE UNIQUE INDEX IF NOT EXISTS live_intent_per_snapshot
    ON action_intent (target_id, snapshot_id)
    WHERE state = 'live' AND target_id IS NOT NULL AND snapshot_id IS NOT NULL;
```

### Decision: keep the actual migration as source of truth

The 2-column index is the cleaner invariant: **one live intent per exact authority instance**. This aligns with `Authority = (target_id, snapshot_id)`. The plan should be updated to reflect this.

### Changes to `docs/OS/plan10.1.md`

1. **Section 11 (Schema / DB Design)**, lines 399–408: Replace the 5-column index SQL with the actual migration SQL. Update the surrounding comment to explain why `(target_id, snapshot_id)` is sufficient:
   - A target is unique to one scenario/belief/action tuple in normal operation.
   - The 2-column index enforces the stronger invariant: one live intent per authority instance.
   - Reference `target_activation`'s `UNIQUE(target_id)` as the upstream guarantee.

2. **Update constraint name** in the plan from `intent_authority_snapshot_fk` to `intent_authority_binding_fk` to match the actual migration.

3. **Add note** about `ON DELETE CASCADE` on the composite FK (the plan didn't mention this; the migration adds it as defense-in-depth since `target_snapshot` is append-only).

4. **Update any references** to the 5-column index throughout the rest of `plan10.1.md` if they exist.

---

## Execution order

| Step | Files | Depends on |
|------|-------|-----------|
| 1 | `api/helpers_test.go` — add `newTestServerWithRecordingExecutor` | — |
| 2 | `api/authorization_test.go` — add `setupBoundExecutionScenario`, `TestExecute_ConfusedDeputy` | Step 1 |
| 3 | `service/authority/authority_integration_test.go` — add `TestEA03b_ConfusedDeputy` | — |
| 4 | `api/authorization.go` — remove dead `snapID` reads (2 locations) | — |
| 5 | `cmd/solvent-mcp/tools.go` — remove dead `snapID` reads (2 locations) | — |
| 6 | `docs/OS/plan10.1.md` — reconcile index documentation | — |
| 7 | Full verification: `go build`, `go vet`, `go test -count=1 -p 1 ./...`, `check_i7.sh`, `mcp_verify.sh` | Steps 1–6 |

Steps 1–3 are test additions (Finding 1 + 2). Steps 4–5 are dead-code removal (Finding 3). Step 6 is doc reconciliation (Finding 4). Step 7 is verification.

---

## Verification checklist

- [ ] `go build ./...` — clean
- [ ] `go vet ./...` — clean
- [ ] `gofmt -l cmd internal kernel api service adapter` — no output
- [ ] `go test -count=1 -p 1 ./...` — all packages green
- [ ] `go test -race -count=1 -p 1 ./kernel ./service/authority ./api ./cmd/solvent-mcp ./adapter/github` — no races
- [ ] `bash scripts/check_i7.sh` — PASS
- [ ] `bash scripts/mcp_verify.sh` — GREEN
- [ ] `task db:reset` — clean
- [ ] Adversarial spot-review of only the changes in this plan
