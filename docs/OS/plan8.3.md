# Plan 8.3 — Remediation Pass for M-01, M-02, L-01, L-05

**Date:** 2026-09-08
**Repository:** HEAD `5933f44 ✨ phase 4e`
**Predecessor:** Plan 8.1 (future-transport guard + evidence reconciliation)
**Trigger:** Fresh independent adversarial review identified 4 new findings

---

## Context

The fresh adversarial review of HEAD `5933f44` identified two medium and two low findings not covered by the historical F-01–F-07 cycle. These are real findings in the current code, not stale review claims. The findings reveal an **asymmetry between the MCP and REST surfaces**: the MCP handlers enforce cross-scenario guards before kernel mutations, but the REST handlers do not. The kernel SQL itself (`sqlRetireDebt`, `sqlPromote`, `sqlDischargeRetireDebt`, `sqlDischargeInsert`) operates on `belief_id` alone with no `scenario_id` predicate.

The remediation should be:
- Narrowly scoped (no architectural redesign)
- Preserve all existing invariants (kernel authority, snapshot binding, ClaimIntent CAS, etc.)
- Fix the specific defects while also establishing a pattern for future mutation handlers
- Add regression tests

---

## Finding Summary

| ID | Severity | Description | Location |
|----|----------|-------------|----------|
| **M-01** | MEDIUM | REST API cross-scenario write gap: `handleRetireDebt`, `handlePromoteBelief`, `handleDischarge` call kernel mutations keyed only by `belief_id`, no `scenario_id` predicate | `api/belief.go:98-169`, `api/discharge.go:9-45`, `kernel/sql.go:22-28,187-193` |
| **M-02** | MEDIUM | `ExecuteAction` silently skips `ClaimIntent`/`CompleteIntent` when `intentID == ""`. Current callers enforce, but function doesn't assert precondition | `service/authority/authority.go:303,400` |
| **L-01** | LOW | `handleSolventDischarge` (MCP) has no cross-scenario guard. No `scenario` argument in tool schema | `cmd/solvent-mcp/tools.go:611-632` |
| **L-05** | LOW→MEDIUM | `ReconcileIntent` logs `ActivityReconciliationCompleted` before kernel transition. Transient failure produces false audit record | `service/authority/authority.go:446-469` |

---

## M-01: REST API Cross-Scenario Write Gap

### Root Cause

The kernel mutations for `RetireDebt`, `Promote`, and `Discharge` operate on `belief_id` alone:

```sql
-- sqlRetireDebt (kernel/sql.go:22-24)
UPDATE belief SET debt = array_remove(debt, $2::STRING) WHERE id = $1::UUID

-- sqlPromote (kernel/sql.go:26-28)
UPDATE belief SET status = 'promoted' WHERE id = $1::UUID

-- sqlDischargeInsert (kernel/sql.go:187-189)
INSERT INTO debt_discharge (belief_id, obligation_key, instrument_ref, discharged_by) VALUES ($1::UUID, $2::STRING, $3::STRING, $4::UUID)

-- sqlDischargeRetireDebt (kernel/sql.go:191-193)
UPDATE belief SET debt = array_remove(debt, $2::STRING) WHERE id = $1::UUID
```

The MCP handlers (`handleSolventRetireDebt`, `handleSolventPromote`) enforce cross-scenario guards by calling `view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})` before the kernel call. The REST handlers (`handleRetireDebt`, `handlePromoteBelief`, `handleDischarge`) do not.

### Fix Strategy

The kernel SQL should enforce scenario binding. Two approaches:

**Option A: Add `AND scenario_id = $2::UUID` to the kernel SQL**

This moves the invariant to the database layer where it belongs. The kernel already has `scenario_id` on the `belief` table. Adding it to the UPDATE/INSERT WHERE clause is a schema-level enforcement that cannot be bypassed by any caller.

**Option B: Add cross-scenario guard at the handler layer (like MCP)**

This is what the MCP handlers do. It's simpler but duplicates logic.

**Recommendation: Option A for kernel SQL, with Option B as a handler-level guard for the REST API.**

The kernel SQL is the authoritative enforcement. The handler-level guard provides defense-in-depth and a clear error message (rather than relying on zero-rows-affected semantics).

### Implementation

**Step 1: Add `scenario_id` to kernel SQL**

Change `sqlRetireDebt`:
```sql
UPDATE belief SET debt = array_remove(debt, $2::STRING)
WHERE id = $1::UUID AND scenario_id = $3::UUID
```

Change `sqlPromote`:
```sql
UPDATE belief SET status = 'promoted'
WHERE id = $1::UUID AND scenario_id = $2::UUID
```

Change `sqlDischargeRetireDebt`:
```sql
UPDATE belief SET debt = array_remove(debt, $2::STRING)
WHERE id = $1::UUID AND scenario_id = $3::UUID
```

Change `sqlDischargeInsert`:
```sql
INSERT INTO debt_discharge (belief_id, obligation_key, instrument_ref, discharged_by)
VALUES ($1::UUID, $2::STRING, $3::STRING, $4::UUID)
```
(This one is trickier — the belief table has scenario_id, but debt_discharge doesn't. The FK is on belief_id. Adding a WHERE clause on belief.id with scenario_id requires a subquery or a different approach. Actually, we can add the scenario check to `sqlDischargeRetireDebt` which is the actual mutation of belief.debt. The INSERT into debt_discharge is just a record; the mutation is the belief.debt update.)

**Step 2: Update kernel function signatures**

- `RetireDebt(ctx, beliefID, item string)` → `RetireDebt(ctx, scenarioID, beliefID, item string)`
- `Promote(ctx, beliefID string)` → `Promote(ctx, scenarioID, beliefID string)`
- `Discharge(ctx, beliefID, obligationKey, instrumentRef, dischargedBy string)` → `Discharge(ctx, scenarioID, beliefID, obligationKey, instrumentRef, dischargedBy string)`

**Step 3: Update all callers**

- `api/belief.go`: `handleRetireDebt` → add scenario_id from query param, pass to `s.ledger.RetireDebt()`
- `api/belief.go`: `handlePromoteBelief` → add scenario_id from query param, pass to `s.ledger.Promote()`
- `api/discharge.go`: `handleDischarge` → add scenario_id from request body, pass to `s.ledger.Discharge()`
- `cmd/solvent-mcp/tools.go`: `handleSolventRetireDebt` → update to pass scenario_id to kernel
- `cmd/solvent-mcp/tools.go`: `handleSolventPromote` → update to pass scenario_id to kernel
- `cmd/solvent-mcp/tools.go`: `handleSolventDischarge` → add scenario_id, pass to kernel

**Step 4: Add cross-scenario guard to REST handlers**

Add the same `view.GetSnapshot()` guard that MCP handlers use. This provides defense-in-depth and clear error messages.

**Step 5: Add cross-scenario guard to `handleSolventDischarge` (MCP)**

This is L-01. The MCP discharge handler has no scenario argument and no cross-scenario guard.

**Step 6: Add regression tests**

- Test: scenario A + belief B (from scenario C) + retire_debt → error + zero mutation
- Test: scenario A + belief B (from scenario C) + promote → error + zero mutation
- Test: scenario A + belief B (from scenario C) + discharge → error + zero mutation
- Test: scenario A + valid belief A → existing behavior unchanged

---

## M-02: ExecuteAction intentID precondition

### Root Cause

`ExecuteAction` at `service/authority/authority.go:227-433` has `if intentID != ""` guards at lines 303 and 400. Both current callers (MCP and REST) enforce non-empty `intentID`, but the function itself doesn't assert it as a precondition.

### Fix

Add a precondition check at the top of `ExecuteAction`:

```go
if intentID == "" {
    return nil, errors.New("intentID is required for ExecuteAction")
}
```

This is a one-line change. Place it after the parameter validation, before `PrepareForAction`.

Also add a regression test that verifies empty `intentID` is rejected.

---

## L-01: handleSolventDischarge cross-scenario guard

### Root Cause

`handleSolventDischarge` at `cmd/solvent-mcp/tools.go:611-632` has no `scenario` argument in the tool schema and no cross-scenario guard. It accepts `belief_id` directly and calls `kernel.Discharge()` without verifying the belief belongs to any scenario.

### Fix

1. Add `scenario` to the tool schema (matching other handlers)
2. Add `view.GetSnapshot()` cross-scenario guard before `kernel.Discharge()`
3. Update `kernel.Discharge()` to accept `scenarioID` (this is part of M-01)

The tool schema change is in `main.go` where `solvent_discharge` is registered (around line 547-572). The handler is in `tools.go:611-632`.

---

## L-05: ReconcileIntent audit ordering

### Root Cause

`ReconcileIntent` at `service/authority/authority.go:446-469` logs `ActivityReconciliationCompleted` BEFORE calling the kernel transition (`CompleteIntent`, `RollbackClaim`, `CancelIntent`). If the kernel transition fails (transient DB error), the audit says "completed" but the intent is still `executing`. This is a false-positive audit record.

### Fix

Follow the same pattern as `CompleteIntent` in `ExecuteAction` (lines 399-418):
- Log the audit AFTER the kernel transition succeeds
- On failure, log a truthful failure event (like `ActivityIntentCompletionFailed`)

The fix:

```go
func (s *Service) ReconcileIntent(ctx context.Context, scenarioID, intentID string, outcome IntentOutcome, operatorID string) error {
    var err error
    switch outcome {
    case IntentOutcomeCompleted:
        err = s.kern.CompleteIntent(ctx, scenarioID, intentID)
    case IntentOutcomeFailed:
        err = s.kern.RollbackClaim(ctx, scenarioID, intentID)
    case IntentOutcomeCancelled:
        err = s.kern.CancelIntent(ctx, scenarioID, intentID)
    default:
        return fmt.Errorf("unknown outcome: %d", outcome)
    }
    
    if err != nil {
        s.audit.Log(ctx, &audit.ActivityEntry{
            ScenarioID: scenarioID,
            Type:       audit.ActivityReconciliationFailed,
            ActorID:    operatorID,
            SubjectID:  intentID,
            Details: map[string]interface{}{
                "intent_id": intentID,
                "outcome":   outcome.String(),
                "error":     err.Error(),
            },
        })
        return err
    }
    
    s.audit.Log(ctx, &audit.ActivityEntry{
        ScenarioID: scenarioID,
        Type:       audit.ActivityReconciliationCompleted,
        ActorID:    operatorID,
        SubjectID:  intentID,
        Details: map[string]interface{}{
            "intent_id": intentID,
            "outcome":   outcome.String(),
        },
    })
    return nil
}
```

This requires adding `ActivityReconciliationFailed` to the audit constants if it doesn't exist. Need to check.

---

## Implementation Sequence

1. **Add `scenario_id` to kernel SQL and function signatures** (kernel/sql.go, kernel/kernel.go, kernel/authority.go)
2. **Update all callers** (api/belief.go, api/discharge.go, cmd/solvent-mcp/tools.go)
3. **Add cross-scenario guards to REST handlers** (api/belief.go, api/discharge.go)
4. **Add cross-scenario guard to `handleSolventDischarge`** (cmd/solvent-mcp/tools.go)
5. **Add `intentID` precondition to `ExecuteAction`** (service/authority/authority.go)
6. **Fix `ReconcileIntent` audit ordering** (service/authority/authority.go)
7. **Add regression tests**
8. **Run verification suite** (go build, go vet, go test, task test, check_i7.sh, mcp_verify.sh)

---

## Files Modified

| File | Change |
|------|--------|
| `kernel/sql.go` | Add `scenario_id` to `sqlRetireDebt`, `sqlPromote`, `sqlDischargeRetireDebt` |
| `kernel/kernel.go` | Update `RetireDebt`, `Promote` signatures to accept `scenarioID` |
| `kernel/authority.go` | Update `Discharge` signature to accept `scenarioID` |
| `api/belief.go` | Add scenario validation + guard to `handleRetireDebt`, `handlePromoteBelief` |
| `api/discharge.go` | Add `scenario_id` to request, add scenario validation |
| `cmd/solvent-mcp/tools.go` | Update `handleSolventRetireDebt`, `handleSolventPromote`, `handleSolventDischarge` to pass `scenarioID`; add cross-scenario guard to `handleSolventDischarge` |
| `cmd/solvent-mcp/main.go` | Update `solvent_discharge` tool schema to include `scenario` |
| `service/authority/authority.go` | Add `intentID` precondition to `ExecuteAction`; fix `ReconcileIntent` audit ordering |

---

## Acceptance Criteria

- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` succeeds
- [ ] `go test -count=1 -p 1 ./...` passes (or DB-dependent tests skip gracefully)
- [ ] `go test -race -count=1 -p 1 ./internal/derive ./internal/normalize ./service/executor ./service/policy ./api/openapi` passes
- [ ] `task test` passes (or DB-dependent tests skip gracefully)
- [ ] `bash scripts/check_i7.sh` passes
- [ ] `bash scripts/mcp_verify.sh` passes (or DB-dependent tests skip gracefully)
- [ ] Regression test: cross-scenario retire_debt → error + zero mutation
- [ ] Regression test: cross-scenario promote → error + zero mutation
- [ ] Regression test: cross-scenario discharge → error + zero mutation
- [ ] Regression test: empty intentID → error
- [ ] Regression test: ReconcileIntent logs failure event on kernel error
- [ ] `MCP_TRANSPORT=sse` still exits with error
- [ ] All existing tests still pass (no regressions)

---

## Invariants Preserved

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

---

## Risk Assessment

| Change | Risk | Mitigation |
|--------|------|------------|
| Kernel SQL change (scenario_id) | Low — adding WHERE predicate to existing UPDATE | Tests prove zero-row mutation on cross-scenario |
| Kernel function signature change | Medium — all callers must update | Go compiler catches missed callers |
| REST handler changes | Low — adding guard before existing call | Tests prove cross-scenario rejection |
| ExecuteAction precondition | Very low — one line, no behavior change for correct callers | Regression test for empty intentID |
| ReconcileIntent audit ordering | Low — moving audit log after kernel call | Regression test for audit truthfulness |