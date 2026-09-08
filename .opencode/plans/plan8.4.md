# Plan 8.4 — Kernel Growth Gate + Discharge Atomicity + Caller Inventory

**Date:** 2026-09-08
**Repository:** HEAD `5933f44 ✨ phase 4e`
**Predecessor:** Plan 8.3 (remediation design) + Plan 8.3_review (review feedback)
**Mode:** Design + ADR — no code changes

---

## Purpose

Plan 8.3 identified four findings (M-01, M-02, L-01, L-05). The review of Plan 8.3 raised three structural issues:

1. M-01 proposes kernel SQL changes without a Kernel Growth Gate ADR
2. M-01's discharge design dismisses `sqlDischargeInsert` as "just a record" — creating a partial-write risk
3. `cmd/operator-review` is not in the caller inventory

This plan resolves all three, then produces the final implementation-ready specification.

---

## 1. Kernel Growth Gate ADR — M-01

### Question

Is `(scenario_id, belief_id)` a durable security invariant that genuinely requires kernel/DB-level enforcement, or is the existing service/handler-layer authorization model sufficient?

### Options

**Option A — Kernel/DB enforcement**

Add `scenario_id` to the kernel SQL for `RetireDebt`, `Promote`, and `Discharge`. The kernel functions gain a `scenarioID` parameter. The DB enforces that the belief belongs to the scenario for every mutation.

**Option B — Service/handler enforcement**

Add `view.GetSnapshot()` guard at every handler/service layer. The kernel functions remain unchanged — they mutate by `belief_id` alone.

**Option C — Hybrid: kernel enforcement + handler defense-in-depth**

Kernel SQL gets `scenario_id` predicates. Handler guards remain as defense-in-depth and for clearer error messages.

### Analysis

The kernel already enforces `scenario_id` on most state mutations:

| Kernel function | Has `scenario_id` in SQL? |
|----------------|--------------------------|
| `EnterBelief` | Yes — `VALUES ($1::UUID, ...)` where $1 is `scenario_id` |
| `AddEvidence` | Yes — `VALUES ($1::UUID, $2::UUID, ...)` where $1 is `scenario_id` |
| `IntentOnPromoted` | Yes — `VALUES ($1::UUID, $2::UUID, ...)` where $1 is `scenario_id` |
| `RetractCascade` | Yes — `WHERE scenario_id = $2::UUID` in both CTEs |
| `ClaimIntent` | Yes — `WHERE id = $1 AND scenario_id = $2` |
| `CompleteIntent` | Yes — `WHERE id = $1 AND scenario_id = $2` |
| `RollbackClaim` | Yes — `WHERE id = $1 AND scenario_id = $2` |
| `CancelIntent` | Yes — `WHERE id = $1 AND scenario_id = $2` |
| **`RetireDebt`** | **No** — `WHERE id = $1::UUID` only |
| **`Promote`** | **No** — `WHERE id = $1::UUID` only |
| **`Discharge`** | **No** — `WHERE id = $1::UUID` only (UPDATE) |

`RetireDebt`, `Promote`, and `Discharge` are the exceptions. These three mutate `belief.debt` or `belief.status` — the same durable state that `RetractCascade` and `ClaimIntent` also modify, and those ARE scenario-bound.

The asymmetry is inconsistent: `RetractCascade` cancels intents and retracts beliefs within a scenario, and the kernel enforces `scenario_id`. But `RetireDebt` (which modifies `belief.debt`) and `Promote` (which modifies `belief.status`) do not enforce `scenario_id`.

### Decision

**Option A (kernel enforcement) + Option B (handler defense-in-depth) = Option C.**

Rationale:

1. **Consistency**: The kernel already enforces `scenario_id` on 8 of 11 state-mutation functions. `RetireDebt`, `Promote`, `Discharge` should follow the same pattern.
2. **New caller safety**: If a future caller is added without a handler guard, the kernel still enforces the invariant. With Option B alone, every new handler must remember to add a guard.
3. **Atomicity**: The kernel already uses `crdb.ExecuteTx`. Adding `scenario_id` to the SQL is a minimal change within the existing transaction boundary.
4. **Existing pattern**: The kernel already performs transaction-scoped state validation before security-sensitive mutations (for example, scenario/state checks in intent operations). `RetireDebt` and `Discharge` will use the same transaction-scoped validation pattern to distinguish a valid same-scenario operation from an invalid scenario binding.

### Contract change: `RetireDebt` and `Promote` are no longer idempotent for cross-scenario calls

Currently:
- `RetireDebt(ctx, beliefID, item)` with a non-existent belief: returns success (0 rows, idempotent)
- `RetireDebt(ctx, beliefID, item)` with a belief in a different scenario: returns success (0 rows, idempotent) — **this is the bug**

After the fix:
- `RetireDebt(ctx, scenarioID, beliefID, item)` with a belief in the correct scenario: returns success
- `RetireDebt(ctx, scenarioID, beliefID, item)` with a belief in a different scenario: returns error (`ErrBeliefNotFound`)
- `RetireDebt(ctx, scenarioID, beliefID, item)` with a non-existent belief: returns error (`ErrBeliefNotFound`)

### Concurrency safety

`belief.scenario_id` is immutable — set at `EnterBelief`, never updated by any kernel mutation. The `SELECT EXISTS` + mutation in one `SERIALIZABLE` transaction has no TOCTOU window: the existence check and the mutation execute atomically, and no concurrent mutation can change the belief's scenario identity between them. This is verified by CockroachDB's serializable isolation guarantee.

### Idempotency regression tests (CockroachDB)

Three test cases proving exact behavior:

| Test | Setup | Call | Expected |
|------|-------|------|----------|
| Same-scenario, item already retired | Belief in scenario A, debt item already removed | `RetireDebt(ctx, scenarioA, beliefID, item)` | Success (idempotent) |
| Same-scenario, item present | Belief in scenario A, debt item present | `RetireDebt(ctx, scenarioA, beliefID, item)` | Success + debt removed |
| Wrong scenario | Belief in scenario A, call with scenario B | `RetireDebt(ctx, scenarioB, beliefID, item)` | `ErrBeliefNotFound` + zero mutation |

---

## 2. Complete Caller Inventory

### 2a. RetireDebt

| Caller | File:Line | Has scenario context? | Has scenario guard? | Must update? |
|--------|-----------|----------------------|-------------------|------------|
| MCP handler | `cmd/solvent-mcp/tools.go:143` | Yes | Yes (view.GetSnapshot) | Yes — pass scenarioID |
| REST handler | `api/belief.go:115` | Yes (query param) | No | Yes — pass scenarioID + add guard |
| Ledger service | `service/ledger/ledger.go:63` | No (pass-through) | No | Yes — add scenarioID param |
| Operator CLI | `cmd/operator-review/main.go:166` | Yes | Yes (beliefScenario != scenarioID) | Yes — pass scenarioID |
| Wizard discharge | `internal/wizard/discharge.go:111,145` | Yes | No explicit guard | Yes — pass scenarioID |
| Wizard seed | `internal/wizard/seed.go:127` | Yes | No explicit guard | Yes — pass scenarioID |
| Internal belief | `internal/belief/belief.go:81` | Yes | No explicit guard | Yes — pass scenarioID |
| Kernel tests | `kernel/kernel_test.go:175,203,903` | Yes | Test setup | Yes — update signatures |
| Kernel example | `kernel/example_test.go:69` | Yes | Test setup | Yes — update signatures |
| Ledger tests | `service/ledger/ledger_test.go:114` | Yes | Test setup | Yes — update signatures |
| View tests | `internal/view/explain_test.go:141,197` | Yes | Test setup | Yes — update signatures |

### 2b. Promote

| Caller | File:Line | Has scenario context? | Has scenario guard? | Must update? |
|--------|-----------|----------------------|-------------------|------------|
| MCP handler | `cmd/solvent-mcp/tools.go:189` | Yes | Yes (view.GetSnapshot) | Yes — pass scenarioID |
| REST handler | `api/belief.go:143` | Yes (query param) | No | Yes — pass scenarioID + add guard |
| Ledger service | `service/ledger/ledger.go:68` | No (pass-through) | No | Yes — add scenarioID param |
| Operator CLI | `cmd/operator-review/main.go:172` | Yes | Yes (beliefScenario != scenarioID) | Yes — pass scenarioID |
| Wizard refusal | `internal/wizard/refusal.go:121` | Yes | No explicit guard | Yes — pass scenarioID |
| Wizard seed | `internal/wizard/seed.go:131` | Yes | No explicit guard | Yes — pass scenarioID |
| Internal belief | `internal/belief/belief.go:88` | Yes | No explicit guard | Yes — pass scenarioID |
| Kernel tests | `kernel/kernel_test.go:235,261,298,361,918` | Yes | Test setup | Yes — update signatures |
| Kernel example | `kernel/example_test.go:52,74` | Yes | Test setup | Yes — update signatures |
| Kernel dogfood | `kernel/authority_dogfood_test.go:266` | Yes | Test setup | Yes — update signatures |
| Ledger tests | `service/ledger/ledger_test.go:118` | Yes | Test setup | Yes — update signatures |
| View tests | `internal/view/explain_test.go:122,156,199` | Yes | Test setup | Yes — update signatures |
| Agentjacking tests | `internal/agentjacking/ingest_test.go:103` | Yes | Test setup | Yes — update signatures |
| Wizard tests | `internal/wizard/seed_test.go`, `flow_test.go` | Yes | Test setup | Yes — update signatures |

### 2c. Discharge

| Caller | File:Line | Has scenario context? | Has scenario guard? | Must update? |
|--------|-----------|----------------------|-------------------|------------|
| MCP handler | `cmd/solvent-mcp/tools.go:622` | No | No | Yes — add scenarioID + guard |
| REST handler | `api/discharge.go:33` | No | No | Yes — add scenarioID + guard |
| Ledger service | `service/ledger/ledger.go:114` | No (pass-through) | No | Yes — add scenarioID param |
| Wizard discharge | `internal/wizard/discharge.go:51-70` | Yes | No explicit guard | Yes — pass scenarioID |
| Kernel tests | `kernel/authority_dogfood_test.go:354-355` | Yes | Test setup | Yes — update signatures |

### 2d. Kernel interface

`kernel/contract.go` defines the `Contract` interface. The following signatures change:

```go
// Before:
RetireDebt(context.Context, string, string) error
Promote(context.Context, string) error
Discharge(context.Context, string, string, string, string) error

// After:
RetireDebt(context.Context, string, string, string) error    // scenarioID, beliefID, item
Promote(context.Context, string, string) error               // scenarioID, beliefID
Discharge(context.Context, string, string, string, string, string) error  // scenarioID, beliefID, ...
```

The `Contract` interface assertion at the bottom of `contract.go` (`_ Contract = (*Store)(nil)`) will fail to compile if the implementation doesn't match. This is the mechanical guard.

---

## 3. Discharge Atomicity Design

### Problem

The current `Discharge` function has two writes in one transaction:

```go
func (s *Store) Discharge(ctx context.Context, beliefID, obligationKey, instrumentRef, dischargedBy string) error {
    return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        // Write 1: INSERT discharge record
        if _, err := tx.ExecContext(ctx, sqlDischargeInsert,
            beliefID, obligationKey, instrumentRef, dischargedBy); err != nil {
            return wrapIf(sqlStateUniqueViolation, ErrDuplicateDischarge, err)
        }
        // Write 2: UPDATE belief.debt
        _, err := tx.ExecContext(ctx, sqlDischargeRetireDebt, beliefID, obligationKey)
        return err
    })
}
```

`sqlDischargeInsert` inserts a `debt_discharge` row without checking `scenario_id`. `sqlDischargeRetireDebt` updates `belief.debt` without checking `scenario_id`.

Cross-scenario attack:
1. `sqlDischargeInsert` succeeds — creates a `debt_discharge` row asserting "obligation was discharged"
2. `sqlDischargeRetireDebt` returns 0 rows — `belief.debt` is unchanged

Result: `debt_discharge` says "discharged", `belief.debt` says "still owed". This is a partial-write failure creating an integrity violation.

### Solution

Make both writes scenario-bound and check `RowsAffected()` for the UPDATE:

**Step 1**: Change `sqlDischargeInsert` to verify the belief belongs to the scenario:

```sql
INSERT INTO debt_discharge (belief_id, obligation_key, instrument_ref, discharged_by)
SELECT $1::UUID, $2::STRING, $3::STRING, $4::UUID
FROM belief b
WHERE b.id = $1::UUID AND b.scenario_id = $5::UUID
```

If the belief doesn't exist in the scenario, the `SELECT` returns 0 rows, and the `INSERT ... SELECT` inserts 0 rows. CockroachDB treats this as success (0 rows inserted). We need to check `RowsAffected()` on the INSERT result.

**Step 2**: Change `sqlDischargeRetireDebt` to check `scenario_id`:

```sql
UPDATE belief SET debt = array_remove(debt, $2::STRING)
WHERE id = $1::UUID AND scenario_id = $3::UUID
```

**Step 3**: Add `RowsAffected()` checks in the kernel function:

```go
func (s *Store) Discharge(ctx context.Context, scenarioID, beliefID, obligationKey, instrumentRef, dischargedBy string) error {
    return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        // Write 1: INSERT discharge record (scenario-bound via subquery)
        result, err := tx.ExecContext(ctx, sqlDischargeInsert,
            beliefID, obligationKey, instrumentRef, dischargedBy, scenarioID)
        if err != nil {
            return wrapIf(sqlStateUniqueViolation, ErrDuplicateDischarge, err)
        }
        n, err := result.RowsAffected()
        if err != nil {
            return err
        }
        if n == 0 {
            return ErrBeliefNotFound  // belief not in this scenario
        }

        // Write 2: UPDATE belief.debt (scenario-bound)
        result, err = tx.ExecContext(ctx, sqlDischargeRetireDebt, beliefID, obligationKey, scenarioID)
        if err != nil {
            return err
        }
        n, err = result.RowsAffected()
        if err != nil {
            return err
        }
        if n == 0 {
            return ErrBeliefNotFound  // defensive: should never happen if INSERT succeeded
        }
        return nil
    })
}
```

**Step 4**: Handle the idempotent case for `sqlDischargeRetireDebt`. When the item is already retired, `array_remove` returns 0 rows (no change). This is different from a cross-scenario failure. We need to distinguish:

- `RowsAffected() == 0` because the item was already retired: success (idempotent)
- `RowsAffected() == 0` because the belief is in a different scenario: error

The solution: the UPDATE already has `AND scenario_id = $3::UUID`. If the belief exists in the scenario but the item is already retired, `array_remove` on an absent item changes nothing, and `RowsAffected()` returns 0. This is the idempotent case — it should succeed.

If the belief doesn't exist in the scenario, the WHERE clause doesn't match, and `RowsAffected()` also returns 0. This is the cross-scenario case — it should fail.

We can distinguish these by checking whether the belief exists in the scenario first (via `SELECT EXISTS`), then performing the UPDATE. This is the same pattern as the `RetireDebt` fix above.

**Revised Discharge implementation**:

```go
func (s *Store) Discharge(ctx context.Context, scenarioID, beliefID, obligationKey, instrumentRef, dischargedBy string) error {
    return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        // Verify belief exists in this scenario.
        var exists bool
        if err := tx.QueryRowContext(ctx,
            `SELECT EXISTS(SELECT 1 FROM belief WHERE id = $1::UUID AND scenario_id = $2::UUID)`,
            beliefID, scenarioID).Scan(&exists); err != nil {
            return err
        }
        if !exists {
            return ErrBeliefNotFound
        }

        // Write 1: INSERT discharge record.
        if _, err := tx.ExecContext(ctx, sqlDischargeInsert,
            beliefID, obligationKey, instrumentRef, dischargedBy); err != nil {
            return wrapIf(sqlStateUniqueViolation, ErrDuplicateDischarge, err)
        }

        // Write 2: UPDATE belief.debt.
        _, err := tx.ExecContext(ctx, sqlDischargeRetireDebt, beliefID, obligationKey, scenarioID)
        return err
    })
}
```

This approach:
1. Verifies belief exists in scenario (single query)
2. INSERT discharge record (no scenario check needed — belief existence already verified)
3. UPDATE belief.debt with scenario binding (defense-in-depth)

The INSERT doesn't need a subquery because we've already verified the belief exists in the scenario. The UPDATE still gets `scenario_id` as defense-in-depth.

### Discharge atomicity regression tests (CockroachDB)

| Test | Setup | Call | Expected |
|------|-------|------|----------|
| Cross-scenario discharge | Belief in scenario A, call with scenario B | `Discharge(ctx, scenarioB, beliefID, ...)` | `ErrBeliefNotFound` + zero `debt_discharge` rows + `belief.debt` unchanged |
| Same-scenario discharge | Belief in scenario A with outstanding debt | `Discharge(ctx, scenarioA, beliefID, ...)` | Success + one `debt_discharge` row + `belief.debt` updated |
| Forced failure after INSERT | Mock UPDATE to fail | `Discharge(ctx, scenarioA, beliefID, ...)` | Transaction rolls back + zero `debt_discharge` rows + `belief.debt` unchanged |

The forced-failure test proves CockroachDB transaction rollback leaves zero discharge rows when the second write fails.

### New error

Add `ErrBeliefNotFound` to the kernel:

```go
var ErrBeliefNotFound = errors.New("belief not found in scenario")
```

A nonexistent belief and a belief in a wrong scenario are intentionally indistinguishable externally. This avoids leaking information across scenario boundaries — a caller in scenario A must not be able to determine whether a belief exists in scenario B.

---

## 4. L-01: MCP `handleSolventDischarge` Cross-Scenario Guard

### Current state

`handleSolventDischarge` at `cmd/solvent-mcp/tools.go:611-632`:
- No `scenario` argument in the tool schema
- No cross-scenario guard
- Accepts `belief_id` directly and calls `kernel.Discharge()` without verifying the belief belongs to any scenario

### Fix

1. Add `scenario` to the tool schema in `main.go` (matching other handlers)
2. Add `view.GetSnapshot()` cross-scenario guard before `kernel.Discharge()`
3. Update `kernel.Discharge()` to accept `scenarioID` (part of M-01)

The tool schema change is in `main.go` where `solvent_discharge` is registered (around line 547-572). The handler is in `tools.go:611-632`.

After the kernel change, the MCP handler becomes:

```go
func handleSolventDischarge(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
    beliefID, _ := args["belief_id"].(string)
    obligationKey, _ := args["obligation_key"].(string)
    instrumentRef, _ := args["instrument_ref"].(string)
    dischargedBy, _ := args["discharged_by"].(string)
    scenario, _ := args["scenario"].(string)

    if beliefID == "" || obligationKey == "" || instrumentRef == "" || dischargedBy == "" {
        return errorResult(fmt.Errorf("all fields are required (belief_id, obligation_key, instrument_ref, discharged_by)")), nil
    }

    scenarioID, ok := lookupScenario(scenario)
    if !ok {
        return errorResult(fmt.Errorf("unknown scenario: %q (valid: %s)", scenario, strings.Join(scenarioNames(), ", "))), nil
    }

    // Cross-scenario guard: verify the belief belongs to this scenario.
    snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
    if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
        return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
    }

    st := kernel.New(db)
    if err := st.Discharge(ctx, scenarioID, beliefID, obligationKey, instrumentRef, dischargedBy); err != nil {
        return toolErrorResult(err), nil
    }

    // ... response
}
```

---

## 5. M-02: ExecuteAction intentID Precondition

No change from Plan 8.3. Add one line at the top of `ExecuteAction`:

```go
if intentID == "" {
    return nil, errors.New("intentID is required for ExecuteAction")
}
```

Place it after the parameter validation, before `PrepareForAction`. One line, one regression test.

---

## 6. L-05: ReconcileIntent Audit Ordering

No change from Plan 8.3. Move audit log after kernel transition. Add `ActivityReconciliationFailed` to audit constants. Log failure on error, success on success. Ensure audit failure doesn't obscure the kernel error.

---

## 7. New Kernel Errors

Add to `kernel/`:

```go
var ErrBeliefNotFound = errors.New("belief not found in scenario")
```

This is returned when `RetireDebt`, `Promote`, or `Discharge` is called with a `scenarioID` that doesn't match the belief's scenario.

---

## 8. New Audit Constant

Add to `service/audit/audit.go`:

```go
ActivityReconciliationFailed ActivityType = "reconciliation_failed"
```

---

## 9. Implementation Sequence

1. **Add `ErrBeliefNotFound` to kernel** (`kernel/kernel.go`)
2. **Add `ActivityReconciliationFailed` to audit** (`service/audit/audit.go`)
3. **Update `kernel/contract.go`** — change `RetireDebt`, `Promote`, `Discharge` signatures
4. **Update `kernel/sql.go`** — add `scenario_id` to `sqlRetireDebt`, `sqlPromote`, `sqlDischargeRetireDebt`
5. **Update `kernel/kernel.go`** — add `scenarioID` param to `RetireDebt`, `Promote`; add existence check + `RowsAffected` logic
6. **Update `kernel/authority.go`** — add `scenarioID` param to `Discharge`; add existence check + transactional atomicity
7. **Update `service/ledger/ledger.go`** — add `scenarioID` param to `RetireDebt`, `Promote`, `Discharge`
8. **Update `api/belief.go`** — add scenario validation + guard to `handleRetireDebt`, `handlePromoteBelief`; pass scenarioID
9. **Update `api/discharge.go`** — add `scenario_id` to request; add scenario validation; pass scenarioID
10. **Update `cmd/solvent-mcp/main.go`** — add `scenario` to `solvent_discharge` tool schema
11. **Update `cmd/solvent-mcp/tools.go`** — update `handleSolventRetireDebt`, `handleSolventPromote`, `handleSolventDischarge` to pass scenarioID; add cross-scenario guard to `handleSolventDischarge`
12. **Update `cmd/operator-review/main.go`** — pass scenarioID to `RetireDebt`, `Promote`
13. **Update `internal/wizard/discharge.go`** — pass scenarioID to `RetireDebt`
14. **Update `internal/wizard/refusal.go`** — pass scenarioID to `Promote`
15. **Update `internal/wizard/seed.go`** — pass scenarioID to `RetireDebt`, `Promote`
16. **Update `internal/belief/belief.go`** — pass scenarioID to `RetireDebt`, `Promote`
17. **Update `service/authority/authority.go`** — add `intentID` precondition to `ExecuteAction`; fix `ReconcileIntent` audit ordering
18. **Update all tests** — update kernel test signatures, ledger test signatures, wizard test signatures, view test signatures
19. **Add regression tests**:
    - Cross-scenario retire_debt → error + zero mutation
    - Cross-scenario promote → error + zero mutation
    - Cross-scenario discharge → error + zero mutation + no debt_discharge row
    - Same-scenario discharge → success + discharge record + debt removed atomically
    - Empty intentID → error
    - ReconcileIntent logs failure event on kernel error
20. **Run verification suite**

---

## 10. Files Modified

| File | Change |
|------|--------|
| `kernel/kernel.go` | Add `ErrBeliefNotFound`; update `RetireDebt`, `Promote` signatures + implementation |
| `kernel/contract.go` | Update `RetireDebt`, `Promote`, `Discharge` interface signatures |
| `kernel/sql.go` | Add `scenario_id` to `sqlRetireDebt`, `sqlPromote`, `sqlDischargeRetireDebt` |
| `kernel/authority.go` | Update `Discharge` signature + implementation (existence check + atomicity) |
| `service/audit/audit.go` | Add `ActivityReconciliationFailed` |
| `service/ledger/ledger.go` | Update `RetireDebt`, `Promote`, `Discharge` signatures (add scenarioID) |
| `service/authority/authority.go` | Add `intentID` precondition; fix `ReconcileIntent` audit ordering |
| `api/belief.go` | Add scenario validation + guard to `handleRetireDebt`, `handlePromoteBelief` |
| `api/discharge.go` | Add `scenario_id` to request; add scenario validation |
| `cmd/solvent-mcp/main.go` | Add `scenario` to `solvent_discharge` tool schema |
| `cmd/solvent-mcp/tools.go` | Update `handleSolventRetireDebt`, `handleSolventPromote`, `handleSolventDischarge` |
| `cmd/operator-review/main.go` | Pass scenarioID to `RetireDebt`, `Promote` |
| `internal/wizard/discharge.go` | Pass scenarioID to `RetireDebt` |
| `internal/wizard/refusal.go` | Pass scenarioID to `Promote` |
| `internal/wizard/seed.go` | Pass scenarioID to `RetireDebt`, `Promote` |
| `internal/belief/belief.go` | Pass scenarioID to `RetireDebt`, `Promote` |
| `kernel/kernel_test.go` | Update `RetireDebt`, `Promote` test signatures |
| `kernel/authority_dogfood_test.go` | Update `Promote`, `Discharge` test signatures |
| `kernel/example_test.go` | Update `RetireDebt`, `Promote` test signatures |
| `service/ledger/ledger_test.go` | Update `RetireDebt`, `Promote` test signatures |
| `internal/view/explain_test.go` | Update `RetireDebt`, `Promote` test signatures |
| `internal/agentjacking/ingest_test.go` | Update `Promote` test signature |
| `internal/wizard/seed_test.go` | Update `Promote` test signature |
| `internal/wizard/flow_test.go` | Update `Discharge` test signatures |

---

## 11. Acceptance Criteria

- [ ] `go build ./...` succeeds (compiler catches missed callers)
- [ ] `go vet ./...` succeeds
- [ ] `go test -count=1 -p 1 ./...` passes (or DB-dependent tests skip gracefully)
- [ ] `go test -race -count=1 -p 1 ./internal/derive ./internal/normalize ./service/executor ./service/policy ./api/openapi` passes
- [ ] `task test` passes (or DB-dependent tests skip gracefully)
- [ ] `bash scripts/check_i7.sh` passes
- [ ] `bash scripts/mcp_verify.sh` passes (or DB-dependent tests skip gracefully)
- [ ] Regression: cross-scenario retire_debt → error + zero mutation
- [ ] Regression: cross-scenario promote → error + zero mutation
- [ ] Regression: cross-scenario discharge → error + zero mutation + no debt_discharge row
- [ ] Regression: same-scenario discharge → success + discharge record + debt removed atomically
- [ ] Regression: empty intentID → error
- [ ] Regression: ReconcileIntent logs failure event on kernel error
- [ ] `MCP_TRANSPORT=sse` still exits with error
- [ ] All existing tests still pass (no regressions)

---

## 12. Invariants Preserved

- `kernel.Authorize` remains the sole authority oracle
- `TOKEN != AUTHORITY`
- `Authorize != Execute`
- Snapshot parameters are authoritative, caller params are ignored
- Fixed action → executor mapping
- Exact intentID binding (now enforced by `ExecuteAction` precondition)
- `ClaimIntent` CAS (now enforced by `ExecuteAction` precondition)
- Ambiguous outcome → executing (no retry)
- `executing` survives `RetractCascade`
- `CompleteIntent` source-state guard
- REST authenticated principal
- MCP trusted-local stdio model
- GitHub semantics outside kernel
- No `AuthorizeAndClaimIntent`
- No second authorization engine
- **NEW: `(scenario_id, belief_id)` is enforced at the kernel/DB level for all state mutations**

---

## 13. Key Architectural Decision

> **`(scenario_id, belief_id)` is a durable security invariant enforced at the kernel/DB level.** Every kernel mutation that modifies `belief.debt` or `belief.status` requires the caller to provide `scenarioID`, and the kernel verifies the belief belongs to the scenario before executing the mutation. This is consistent with the existing kernel pattern (`ClaimIntent`, `RetractCascade`, etc.) and prevents cross-scenario mutations from any caller, including future ones that may not add handler-level guards.

**ADR accepted; implementation authorized.**
