# Plan 9.0 — Security Remediation: NEW-01 / NEW-02 / NEW-03 + Verification Cleanup

**Date:** 2026-09-08
**Repository:** HEAD `5933f44`, 62 uncommitted files (Plan 8.4/8.5 changes)
**Predecessor:** Plan 8.5 (behavioral verification)
**Mode:** Implementation — fix findings, add regression tests, verify

---

## Scope

Fix three security/integrity findings and two verification defects from the fresh adversarial review. No architectural redesign. No Plan 8.6 work.

| Finding | Severity | Summary |
|---------|----------|---------|
| NEW-01 | HIGH | `ClaimIntent` accepts any same-scenario live intent regardless of belief/action |
| NEW-02 | HIGH | `IntentOnPromoted` permits cross-scenario intent creation, DoS on RetractCascade |
| NEW-03 | MEDIUM | `AddEvidence` permits cross-scenario evidence writes |
| NEW-04 | LOW | `gofmt` regression in `kernel/kernel_test.go` |
| NEW-05 | LOW | `mcp_verify.sh` default DSN lacks migrations 005-008 |

---

## 1. NEW-01 — ClaimIntent Intent Ownership

### Root cause

`sqlClaimIntent` WHERE clause: `id = $1 AND scenario_id = $2 AND state = 'live'`. No `belief_id` or `action` binding. Any live intent in the scenario can be claimed.

### Fix

**Kernel SQL** (`kernel/sql.go`):
```sql
sqlClaimIntent = `
    UPDATE action_intent SET state = 'executing'
    WHERE id = $1::UUID AND scenario_id = $2::UUID
      AND belief_id = $3::UUID AND action = $4::STRING
      AND state = 'live'`
```

**Kernel function** (`kernel/authority.go`):
```go
func (s *Store) ClaimIntent(ctx context.Context, scenarioID, intentID, beliefID, action string) error
```

**Contract interface** (`kernel/contract.go`):
```go
ClaimIntent(context.Context, string, string, string, string) error
```

**Service caller** (`service/authority/authority.go:307-314`):
```go
if err := s.kern.ClaimIntent(ctx, scenarioID, intentID, beliefID, action); err != nil {
```

`CompleteIntent`, `RollbackClaim`, `CancelIntent` remain unchanged — they operate on already-claimed intents where `state = 'executing'` and the binding was enforced at claim time.

**Immutability prerequisite**: This design assumes `action_intent.scenario_id`, `action_intent.belief_id`, and `action_intent.action` are immutable after creation. Before implementation, verify that no reachable code path UPDATEs these columns after INSERT. If any path mutates them, claim-time binding alone is insufficient and the fix must be adjusted. Expected: the fields are effectively immutable (INSERT-only schema), but this must be established, not assumed.

### Files modified

| File | Change |
|------|--------|
| `kernel/sql.go` | Add `belief_id = $3 AND action = $4` to `sqlClaimIntent` |
| `kernel/authority.go` | Update `ClaimIntent` signature + pass new params |
| `kernel/contract.go` | Update `ClaimIntent` in interface |
| `service/authority/authority.go` | Pass `beliefID, action` to `ClaimIntent` |

### Regression tests

Add to `service/authority/authority_integration_test.go`:

1. **Happy path**: correct scenario + belief + action + live intent → claim succeeds
2. **Wrong belief**: same scenario, different belief → claim fails, state remains live, executor NOT invoked
3. **Wrong action**: same scenario, same belief, different action → claim fails, state remains live, executor NOT invoked
4. **Wrong scenario**: → claim fails, state remains live, executor NOT invoked
5. **Exact NEW-01 exploit**: Belief A (approved for deploy, no intent) + Belief B (live intent for rollback) → ExecuteAction(A, deploy, intentB) → rejected, intentB remains live, executor NOT called

---

## 2. NEW-02 — IntentOnPromoted Scenario Isolation

### Root cause

`createIntentWithinTx` INSERTs into `action_intent` with caller-supplied `scenario_id` and `belief_id` without verifying the belief belongs to the scenario. The `gate` FK references `belief(id, status)` — not `belief(scenario_id, id)`.

### Fix

**Kernel function** (`kernel/kernel.go:137-140`): Add `SELECT EXISTS` check inside `createIntentWithinTx`:

```go
func createIntentWithinTx(ctx context.Context, tx *sql.Tx, scenarioID, beliefID, action string) error {
    var exists bool
    if err := tx.QueryRowContext(ctx,
        `SELECT EXISTS(SELECT 1 FROM belief WHERE id = $1::UUID AND scenario_id = $2::UUID)`,
        beliefID, scenarioID).Scan(&exists); err != nil {
        return err
    }
    if !exists {
        return ErrBeliefNotFound
    }
    _, err := tx.ExecContext(ctx, sqlIntentOnPromoted, scenarioID, beliefID, action)
    return wrapIf(sqlStateFKViolation, ErrActionOnUnpromoted, err)
}
```

This protects both `IntentOnPromoted` and `AuthorizeAndCreateIntent` (both call `createIntentWithinTx`).

### Files modified

| File | Change |
|------|--------|
| `kernel/kernel.go` | Add `SELECT EXISTS` + `ErrBeliefNotFound` check in `createIntentWithinTx` |

### B-24 test update

The existing `TestW2_B24_B16_BlockedCascadeIsAtomic` test at `kernel/kernel_test.go:796` deliberately creates a cross-scenario intent (line 811) to demonstrate the 23514 deadlock. After the fix, `IntentOnPromoted(ctx, scB, a2, ...)` will fail with `ErrBeliefNotFound` because `a2` belongs to `scA`.

Update B-24 to verify:
- Cross-scenario `IntentOnPromoted` returns `ErrBeliefNotFound`
- `RetractCascade(ctx, scA, a1)` now succeeds (no foreign intent to block it)
- Belief `a2` becomes retracted
- No 23514 deadlock

### Additional regression tests

Add to `kernel/kernel_test.go`:

1. **Cross-scenario intent rejected**: `IntentOnPromoted(scenarioB, beliefA, action)` → `ErrBeliefNotFound`, zero `action_intent` rows
2. **RetractCascade succeeds after rejection**: after failed cross-scenario intent, `RetractCascade(scenarioA, beliefA)` → succeeds, belief retracted
3. **Same-scenario intent succeeds**: `IntentOnPromoted(scenarioA, beliefA, action)` → success, intent created
4. **AuthorizeAndCreateIntent cross-scenario rejected**: `AuthorizeAndCreateIntent(targetID, tuple, scenarioB, beliefA, action)` → `ErrBeliefNotFound`, zero `action_intent` rows. This directly exercises the second security-sensitive caller of `createIntentWithinTx`, not just the helper-level coverage from tests 1-3.

---

## 3. NEW-03 — AddEvidence Scenario Isolation

### Root cause

`kernel.AddEvidence` INSERTs evidence with caller-supplied `scenario_id` and `belief_id` without verifying the belief belongs to the scenario. The REST handler `handleAddEvidence` has no `view.GetSnapshot` guard.

### Fix

**Kernel function** (`kernel/kernel.go:80-86`): Add `SELECT EXISTS` check inside the transaction:

```go
func (s *Store) AddEvidence(ctx context.Context, scenarioID, beliefID, provenanceClass, sourceURL, contentSHA256 string) error {
    return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        var exists bool
        if err := tx.QueryRowContext(ctx,
            `SELECT EXISTS(SELECT 1 FROM belief WHERE id = $1::UUID AND scenario_id = $2::UUID)`,
            beliefID, scenarioID).Scan(&exists); err != nil {
            return err
        }
        if !exists {
            return ErrBeliefNotFound
        }
        _, err := tx.ExecContext(ctx, sqlAddEvidence,
            scenarioID, beliefID, provenanceClass, sourceURL, contentSHA256)
        return err
    })
}
```

**REST handler** (`api/evidence.go`): Add `view.GetSnapshot` defense-in-depth guard. Must use the same ownership predicate as `handleRetireDebt` and `handleDischarge` — verify both that the snapshot resolves and that the belief ID matches:

```go
snap, err := view.GetSnapshot(r.Context(), s.db, req.ScenarioID, view.SnapshotOpts{BeliefID: req.BeliefID})
if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != req.BeliefID {
    writeError(w, http.StatusNotFound, "not_found", "belief not found in scenario", nil)
    return
}
```

This matches the established guard pattern in `handleDischarge` (`api/discharge.go:40-43`). Do not use the shorter `if _, err := ...; err != nil` variant — the dual check (error + belief ID match) is the project's established defense-in-depth convention for scenario ownership.

### Files modified

| File | Change |
|------|--------|
| `kernel/kernel.go` | Add `SELECT EXISTS` + `ErrBeliefNotFound` in `AddEvidence` |
| `api/evidence.go` | Add `view.GetSnapshot` guard + import |

### Regression tests

Add to `kernel/kernel_test.go`:
1. **Cross-scenario evidence rejected**: `AddEvidence(scenarioB, beliefA, ...)` → `ErrBeliefNotFound`, zero evidence rows
2. **Same-scenario evidence succeeds**: `AddEvidence(scenarioA, beliefA, ...)` → success

Add to `api/integration_test.go`:
3. **REST cross-scenario evidence rejected**: `POST /v1/evidence` with wrong scenario → HTTP 404, zero evidence rows
4. **REST same-scenario evidence succeeds**: → HTTP 201

---

## 4. NEW-04 — gofmt Regression

Run `gofmt -w kernel/kernel_test.go`. The issues are at lines 748-782 (stray spaces in struct field alignment).

---

## 5. NEW-05 — mcp_verify.sh Migration Gap

`Taskfile.yml` `db:reset` only applies migrations 001-004. Add migrations 005-008:

```yaml
db:reset:
    cmds:
      - docker exec solvent-crdb cockroach sql --insecure -e "DROP DATABASE IF EXISTS fable CASCADE; CREATE DATABASE fable;"
      - docker exec -i solvent-crdb cockroach sql --insecure --database=fable < db/001_schema.sql
      - docker exec -i solvent-crdb cockroach sql --insecure --database=fable < db/002_corpus.sql
      - docker exec -i solvent-crdb cockroach sql --insecure --database=fable < db/003_wizard.sql
      - docker exec -i solvent-crdb cockroach sql --insecure --database=fable < db/004_debt_vocabulary.sql
      - docker exec -i solvent-crdb cockroach sql --insecure --database=fable < db/005_authority_mvp.sql
      - docker exec -i solvent-crdb cockroach sql --insecure --database=fable < db/006_authority_justification_cascade.sql
      - docker exec -i solvent-crdb cockroach sql --insecure --database=fable < db/007_service_tables.sql
      - docker exec -i solvent-crdb cockroach sql --insecure --database=fable < db/008_executing_state.sql
```

---

## 6. Caller Inventory for ClaimIntent Signature Change

All callers of `ClaimIntent` that need updating:

| File | Line | Current call | New call |
|------|------|-------------|----------|
| `service/authority/authority.go` | 309 | `ClaimIntent(ctx, scenarioID, intentID)` | `ClaimIntent(ctx, scenarioID, intentID, beliefID, action)` |
| `service/authority/authority_integration_test.go` | 1184 | `ClaimIntent(ctx, sid, intentID)` | `ClaimIntent(ctx, sid, intentID, beliefID, action)` |
| `adapter/github/executor_test.go` | ~1354, 2389, 2405, 2510, 2516, 2542, 2686, 2799 | `ClaimIntent(ctx, sid, intentID)` | `ClaimIntent(ctx, sid, intentID, beliefID, action)` |

---

## 7. Implementation Sequence

0. **Verify immutability**: grep for any UPDATE on `action_intent` that modifies `scenario_id`, `belief_id`, or `action` columns. Confirm none exist before proceeding with claim-time binding.
1. Fix `gofmt` (NEW-04) — `gofmt -w kernel/kernel_test.go`
2. Fix `Taskfile.yml` (NEW-05) — add migrations 005-008
3. Fix `kernel/sql.go` — update `sqlClaimIntent` with `belief_id` + `action`
4. Fix `kernel/authority.go` — update `ClaimIntent` signature
5. Fix `kernel/contract.go` — update interface
6. Fix `kernel/kernel.go` — add scenario guard in `createIntentWithinTx` + `AddEvidence`
7. Fix `service/authority/authority.go` — pass `beliefID, action` to `ClaimIntent`
8. Fix `api/evidence.go` — add `view.GetSnapshot` guard
9. Update all test callers of `ClaimIntent`
10. Update B-24 test for new behavior
11. Add NEW-01/02/03 regression tests
12. Run full verification suite

---

## 8. Acceptance Criteria

- [ ] `gofmt -l kernel` reports no files
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] NEW-01 regression: 5 tests pass (happy path + four rejection cases, including the exact NEW-01 exploit)
- [ ] NEW-02 regression: 4 tests pass (cross-scenario rejected + retract succeeds + same-scenario succeeds + AuthorizeAndCreateIntent cross-scenario rejected)
- [ ] NEW-03 regression: 4 tests pass (kernel + REST, same-scenario + cross-scenario)
- [ ] B-24 updated: cross-scenario intent creation rejected, RetractCascade succeeds
- [ ] `go test -count=1 -p 1 ./...` passes
- [ ] `task db:reset` applies all 8 migrations
- [ ] `bash scripts/mcp_verify.sh` passes
- [ ] `bash scripts/check_i7.sh` passes
