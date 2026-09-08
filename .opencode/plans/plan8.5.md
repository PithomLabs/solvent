# Plan 8.5 — CockroachDB-Backed Behavioral Verification

**Date:** 2026-09-08
**Repository:** HEAD `5933f44`, working tree has 24 uncommitted files (Plan 8.4 implementation)
**Predecessor:** Plan 8.4 (kernel growth gate + implementation)
**Mode:** Test-only — no code changes beyond test files
**Safety assumption:** `solvent-crdb` is the dedicated disposable test database; `task db:reset` is expected to destroy/recreate its test state.

---

## Purpose

Plan 8.4 implementation is complete and compiles cleanly. The remaining gap is behavioral verification: the regression tests that prove the security invariants actually hold against a running CockroachDB. This plan adds those tests and runs the full verification suite.

The original defects (M-01, L-01) were discovered at the REST/MCP surface. Plan 8.4 chose both kernel enforcement **and** handler-level defense-in-depth. This plan verifies both layers: kernel invariant enforcement and endpoint-level guard behavior.

---

## 1. Start CockroachDB

```bash
docker start solvent-crdb
task db:reset
```

Before execution, establish the actual tested revision:

```bash
git rev-parse HEAD
git status --short | wc -l
```

Record the output so later reviewers can distinguish "Plan 8.4 was applied" from any particular immutable commit.

---

## 2. Add Kernel Regression Tests — `kernel/kernel_test.go`

Add after `TestW2_B19_CrossScenarioIsolation` (line 629). Each test uses the existing `scenario(n)` helper and `shared` DB handle.

### 2a. RetireDebt cross-scenario → error + zero mutation

```
Create belief in scenario A via EnterBelief
Record belief.debt before attempt
Attempt RetireDebt(scenarioB, beliefID, item)
→ expect ErrBeliefNotFound
→ verify belief.debt unchanged (SELECT array_to_string(debt, ','))
```

### 2b. RetireDebt same-scenario idempotent → success

```
Create belief in scenario A with item present
RetireDebt(scenarioA, beliefID, item)  → success
RetireDebt(scenarioA, beliefID, item)  → success (idempotent)
→ verify belief.debt has item removed
```

### 2c. Promote cross-scenario → error + zero mutation

```
Create belief in scenario A with outstanding debt
Attempt Promote(scenarioB, beliefID)
→ expect ErrBeliefNotFound
→ verify belief.status still 'entered' (SELECT status FROM belief WHERE id = ...)
```

### 2d. Discharge cross-scenario → error + zero writes

```
Create belief in scenario A with outstanding debt
Create a principal for discharged_by
Attempt Discharge(scenarioB, beliefID, item, instrument, principal)
→ expect ErrBeliefNotFound
→ verify zero debt_discharge rows (SELECT count(*) FROM debt_discharge WHERE belief_id = ...)
→ verify belief.debt unchanged
```

### 2e. Discharge same-scenario → atomic success

```
Create belief in scenario A with outstanding debt
Discharge(scenarioA, beliefID, item, instrument, principal)
→ verify debt_discharge row exists
→ verify belief.debt has item removed
```

---

## 3. Add Discharge Rollback Test — `kernel/kernel_test.go`

**Goal:** Prove the two constituent writes (INSERT discharge + UPDATE belief.debt) are atomic. The current positive test only proves both succeed together. This test forces a failure after the first durable operation and asserts both writes are absent.

**Mechanism:** Use `pg_cancel_backend` from a concurrent goroutine to interrupt the connection mid-transaction. This is a standard CockroachDB testing pattern that forces a transaction rollback without adding kernel-level fault injection.

```
Create belief in scenario A with outstanding debt
Open a second DB connection (killer)
Begin a goroutine that:
  1. Waits 50ms for the Discharge transaction to start
  2. SELECT pg_backend_pid() from the Discharge connection
  3. SELECT pg_cancel_backend(pid) from the killer connection
On the main goroutine: Discharge(scenarioA, beliefID, item, instrument, principal)
→ expect error (context canceled or connection closed)
→ verify zero debt_discharge rows
→ verify belief.debt unchanged
```

**Alternative if pg_cancel_backend proves unreliable:** Use a deferred CHECK constraint approach:
1. Add a temporary CHECK constraint on belief: `CHECK (array_length(debt, 1) >= 0)` (always true)
2. Replace with a CHECK that fails: `CHECK (debt IS NOT NULL OR claim != 'forced-fail-rollback')`
3. Create a belief with claim = 'forced-fail-rollback' and non-null debt
4. The UPDATE that sets debt = array_remove(...) will violate the CHECK
5. The entire transaction rolls back — no discharge row, no debt change
6. Drop the temporary constraint

The pg_cancel_backend approach is preferred because it requires zero schema changes.

---

## 4. Add Endpoint-Level Regression Tests

These verify the defense-in-depth guards at the REST and MCP boundaries, not just the kernel invariant.

### 4a. REST cross-scenario tests — `api/integration_test.go`

Using existing test infrastructure: `testDB(t)`, `newTestServer(t, db)`, `createTestScenario(t)`, `createTestBelief(t, db, scenarioID)`, `doRequest(t, ts, method, path, body)`.

**Test: REST RetireDebt wrong scenario → 404 + zero mutation**
```
Create belief in scenario A
POST /v1/beliefs/{beliefID}/debt/retire with scenario_id = scenarioB
→ expect HTTP 404 with "not_found" code
→ verify belief.debt unchanged via direct SQL
```

**Test: REST Promote wrong scenario → 404 + zero mutation**
```
Create belief in scenario A
POST /v1/beliefs/{beliefID}/promote with scenario_id = scenarioB
→ expect HTTP 404 with "not_found" code
→ verify belief.status still 'entered' via direct SQL
```

**Test: REST Discharge wrong scenario → 404 + zero mutation**
```
Create belief in scenario A
POST /v1/discharge with scenario_id = scenarioB, belief_id = beliefID
→ expect HTTP 404 with "not_found" code
→ verify zero debt_discharge rows via direct SQL
→ verify belief.debt unchanged via direct SQL
```

### 4b. MCP cross-scenario test — `cmd/solvent-mcp/tools_test.go` (new file)

Using existing test infrastructure: `sharedDB`, `handleSolventDischarge(ctx, db, args)`.

**Test: MCP Discharge wrong scenario → tool error + zero mutation**
```
Create belief in scenario A via kernel calls
Call handleSolventDischarge(ctx, sharedDB, args) with scenario = "wrong_scenario"
→ expect result.IsError = true
→ expect error text contains "not found"
→ verify zero debt_discharge rows via direct SQL
→ verify belief.debt unchanged via direct SQL
```

Note: MCP `lookupScenario` validates against hardcoded scenario names. A "wrong scenario" name returns an error before DB access. To test the DB-level guard, use a valid scenario name that doesn't contain the belief. The test creates the belief in scenario A and calls discharge with scenario B (a valid scenario name).

---

## 5. Add ExecuteAction + ReconcileIntent Tests — `service/authority/authority_integration_test.go`

### 5a. ExecuteAction empty intentID → rejected

```
Set up promoted belief + live intent
Call ExecuteAction with intentID = ""
→ expect error containing "intentID is required"
→ verify no executor invocation
```

### 5b. ReconcileIntent success → completed audit

```
Create promoted belief + live intent + claim intent (state = 'executing')
ReconcileIntent(scenarioID, intentID, IntentOutcomeCompleted, operatorID)
→ verify reconciliation_completed audit entry exists
→ verify intent state is 'executed'
```

### 5c. ReconcileIntent failure → failed audit + original error

```
Create promoted belief + live intent (state = 'live', not claimed)
ReconcileIntent(scenarioID, intentID, IntentOutcomeCompleted, operatorID)
→ expect error (intent not executing)
→ verify reconciliation_failed audit entry exists
→ verify intent state is still 'live'
→ verify audit entry contains the error message
```

---

## 6. Run Full Verification Suite

```bash
# Individual test packages first (faster feedback)
go test -count=1 -p 1 ./kernel/...
go test -count=1 -p 1 ./api/...
go test -count=1 -p 1 ./cmd/solvent-mcp/...
go test -count=1 -p 1 ./service/authority/...

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

## 7. Expected Results

| Check | Expected |
|-------|----------|
| Cross-scenario RetireDebt (kernel) | `ErrBeliefNotFound` + zero mutation |
| Same-scenario RetireDebt idempotent (kernel) | Success on second call |
| Cross-scenario Promote (kernel) | `ErrBeliefNotFound` + zero mutation |
| Cross-scenario Discharge (kernel) | `ErrBeliefNotFound` + zero `debt_discharge` rows + zero debt mutation |
| Same-scenario Discharge (kernel) | Success + discharge row + debt removed atomically |
| Discharge rollback (kernel) | Error + zero `debt_discharge` rows + debt unchanged |
| REST RetireDebt wrong scenario | HTTP 404 + zero mutation |
| REST Promote wrong scenario | HTTP 404 + zero mutation |
| REST Discharge wrong scenario | HTTP 404 + zero mutation + no discharge row |
| MCP Discharge wrong scenario | Tool error + zero mutation + no discharge row |
| Empty intentID | Rejected before executor |
| ReconcileIntent success | `reconciliation_completed` audit + intent state updated |
| ReconcileIntent failure | `reconciliation_failed` audit + original error + no false completion |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `check_i7.sh` | PASS |
| `mcp_verify.sh` | PASS (if CockroachDB available) |

---

## 8. Implementation Sequence

1. Start CockroachDB: `docker start solvent-crdb`
2. Reset database: `task db:reset`
3. Record git state: `git rev-parse HEAD`, `git status --short | wc -l`
4. Add kernel cross-scenario tests (5 tests) to `kernel/kernel_test.go`
5. Add discharge rollback test to `kernel/kernel_test.go`
6. Add REST cross-scenario tests (3 tests) to `api/integration_test.go`
7. Add MCP cross-scenario test to `cmd/solvent-mcp/tools_test.go` (new file)
8. Add ExecuteAction empty intentID test to `service/authority/authority_integration_test.go`
9. Add ReconcileIntent audit ordering tests (2 tests) to `service/authority/authority_integration_test.go`
10. Run kernel tests: `go test -count=1 -p 1 ./kernel/...`
11. Run API tests: `go test -count=1 -p 1 ./api/...`
12. Run MCP tests: `go test -count=1 -p 1 ./cmd/solvent-mcp/...`
13. Run authority tests: `go test -count=1 -p 1 ./service/authority/...`
14. Run full suite: `go test -count=1 -p 1 ./...`
15. Run `task test`
16. Run `bash scripts/check_i7.sh`
17. Run `bash scripts/mcp_verify.sh`
18. Report results

---

## 9. Files Modified

| File | Change |
|------|--------|
| `kernel/kernel_test.go` | Add 5 cross-scenario tests + 1 discharge rollback test |
| `api/integration_test.go` | Add 3 REST cross-scenario tests |
| `cmd/solvent-mcp/tools_test.go` | **New file** — MCP cross-scenario discharge test |
| `service/authority/authority_integration_test.go` | Add empty intentID test + 2 ReconcileIntent audit tests |

---

## 10. Acceptance Criteria

- [ ] All 5 kernel cross-scenario tests pass
- [ ] Discharge rollback test passes (zero partial durable state)
- [ ] REST cross-scenario RetireDebt → error + zero mutation
- [ ] REST cross-scenario Promote → error + zero mutation
- [ ] REST cross-scenario Discharge → error + zero mutation + no discharge row
- [ ] MCP cross-scenario Discharge → error + zero mutation + no discharge row
- [ ] Empty intentID test passes
- [ ] Both ReconcileIntent audit tests pass
- [ ] `go test -count=1 -p 1 ./...` passes
- [ ] `task test` passes
- [ ] `bash scripts/check_i7.sh` passes
- [ ] `bash scripts/mcp_verify.sh` passes
