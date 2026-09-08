# Plan 8.5 — CockroachDB-Backed Behavioral Verification

**Date:** 2026-09-08
**Repository:** HEAD `5933f44 ✨ phase 4e` (with Plan 8.4 changes applied)
**Predecessor:** Plan 8.4 (kernel growth gate + implementation)
**Mode:** Test-only — no code changes beyond test files

---

## Purpose

Plan 8.4 implementation is complete and compiles cleanly. The remaining gap is behavioral verification: the regression tests that prove the security invariants actually hold against a running CockroachDB. This plan adds those tests and runs the full verification suite.

---

## 1. Start CockroachDB

The container `solvent-crdb` exists but is stopped.

```bash
docker start solvent-crdb
```

Then reset the database:

```bash
task db:reset
```

This runs the schema migrations against a fresh `fable` database.

---

## 2. Add Regression Tests

### 2a. Kernel cross-scenario tests — `kernel/kernel_test.go`

Add after `TestW2_B19_CrossScenarioIsolation` (line 629):

**Test: RetireDebt cross-scenario → error + zero mutation**
```
Create belief in scenario A
Attempt RetireDebt(scenarioB, beliefID, item)
→ expect ErrBeliefNotFound
→ verify belief.debt unchanged in scenario A
```

**Test: RetireDebt same-scenario idempotent → success**
```
Create belief in scenario A with item present
RetireDebt(scenarioA, beliefID, item)  → success
RetireDebt(scenarioA, beliefID, item)  → success (idempotent)
→ verify belief.debt has item removed
```

**Test: Promote cross-scenario → error + zero mutation**
```
Create belief in scenario A with outstanding debt
Attempt Promote(scenarioB, beliefID)
→ expect ErrBeliefNotFound
→ verify belief.status still 'entered' in scenario A
```

**Test: Discharge cross-scenario → error + zero writes**
```
Create belief in scenario A with outstanding debt
Create a principal for discharged_by
Attempt Discharge(scenarioB, beliefID, ...)
→ expect ErrBeliefNotFound
→ verify zero debt_discharge rows for this belief
→ verify belief.debt unchanged
```

**Test: Discharge same-scenario → atomic success**
```
Create belief in scenario A with outstanding debt
Discharge(scenarioA, beliefID, item, instrument, principal)
→ verify debt_discharge row exists
→ verify belief.debt has item removed
```

### 2b. ExecuteAction empty intentID test — `service/authority/authority_integration_test.go`

**Test: ExecuteAction with empty intentID → rejected**
```
Call ExecuteAction with intentID = ""
→ expect error containing "intentID is required"
→ verify no executor invocation
```

### 2c. ReconcileIntent audit ordering test — `service/authority/authority_integration_test.go`

**Test: ReconcileIntent on executing intent → completed audit**
```
Create promoted belief + live intent + claim intent
ReconcileIntent(scenarioID, intentID, IntentOutcomeCompleted, operatorID)
→ verify reconciliation_completed audit entry exists
→ verify intent state is 'executed'
```

**Test: ReconcileIntent on non-executing intent → failed audit + original error**
```
Create promoted belief + live intent (not claimed)
ReconcileIntent(scenarioID, intentID, IntentOutcomeCompleted, operatorID)
→ expect error (intent not executing)
→ verify reconciliation_failed audit entry exists
→ verify intent state is still 'live'
→ verify audit entry contains the error message
```

---

## 3. Run Full Verification Suite

After adding tests:

```bash
# Full test suite
go test -count=1 -p 1 ./...

# Race-tested non-DB packages
go test -race -count=1 -p 1 ./internal/derive ./internal/normalize ./service/executor ./service/policy ./api/openapi

# Taskfile tests
task test

# I-7 gate
bash scripts/check_i7.sh

# MCP verification
bash scripts/mcp_verify.sh
```

---

## 4. Expected Results

| Check | Expected |
|-------|----------|
| Cross-scenario RetireDebt | `ErrBeliefNotFound` + zero mutation |
| Cross-scenario Promote | `ErrBeliefNotFound` + zero mutation |
| Cross-scenario Discharge | `ErrBeliefNotFound` + zero `debt_discharge` rows + zero debt mutation |
| Same-scenario Discharge | Success + discharge row + debt removed atomically |
| Same-scenario RetireDebt idempotent | Success on second call |
| Empty intentID | Rejected before executor |
| ReconcileIntent success | `reconciliation_completed` audit + intent state updated |
| ReconcileIntent failure | `reconciliation_failed` audit + original error + no false completion |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `check_i7.sh` | PASS |
| `mcp_verify.sh` | PASS (if CockroachDB available) |

---

## 5. Implementation Sequence

1. Start CockroachDB: `docker start solvent-crdb`
2. Reset database: `task db:reset`
3. Add kernel cross-scenario tests to `kernel/kernel_test.go`
4. Add ExecuteAction empty intentID test to `service/authority/authority_integration_test.go`
5. Add ReconcileIntent audit ordering tests to `service/authority/authority_integration_test.go`
6. Run `go test -count=1 -p 1 ./kernel/...` — verify kernel tests pass
7. Run `go test -count=1 -p 1 ./service/authority/...` — verify authority tests pass
8. Run full suite: `go test -count=1 -p 1 ./...`
9. Run `task test`
10. Run `bash scripts/check_i7.sh`
11. Run `bash scripts/mcp_verify.sh`
12. Report results

---

## 6. Files Modified

| File | Change |
|------|--------|
| `kernel/kernel_test.go` | Add 5 cross-scenario regression tests |
| `service/authority/authority_integration_test.go` | Add empty intentID test + ReconcileIntent audit tests |

---

## 7. Acceptance Criteria

- [ ] All 5 kernel cross-scenario tests pass
- [ ] Empty intentID test passes
- [ ] Both ReconcileIntent audit tests pass
- [ ] `go test -count=1 -p 1 ./...` passes
- [ ] `task test` passes
- [ ] `bash scripts/check_i7.sh` passes
- [ ] `bash scripts/mcp_verify.sh` passes
