# Phase 6.2 — Canonical Full-Suite Verification

## Invariant

```
CLEAN COCKROACHDB
      ↓
go test -count1 ./...
      ↓
all packages pass
      ↓
no package relies on another package's TestMain
      ↓
no package can reset another package's live test database
```

## Gate

```
Diagnostic (serial):
  go test -count=1 -p 1 ./...            PASS

Final (parallel):
  go test -count=1 ./...                 PASS
  go test -count=1 ./...                 PASS    (second run for confidence)
```

Zero panics. Zero skipped security tests. Zero package failures.

---

## Root Cause: Shared fable_test Under Parallel Execution

Every TestMain follows this pattern:

```
AcquireResetLock("fable_test")
    ↓
DROP + CREATE fable_test
    ↓
ReleaseResetLock("fable_test")
    ↓
run package tests
```

The lock serializes the **reset operation** only. It does not protect the
package's test execution afterward. Under `go test ./...` (parallel), two
package processes can overlap:

```
Package A:  reset fable_test → release lock → run tests ──────────────>
Package B:                      reset fable_test → release lock → run tests
                                         ↑ destroys A's live state
```

This is the architectural defect. The fix is per-package database isolation.

---

## Design: Per-Package Test Databases

Each package gets its own database:

```
fable_test_api
fable_test_kernel
fable_test_belief
fable_test_intent
fable_test_pipeline
fable_test_corpus
fable_test_wizard
fable_test_agentjacking
fable_test_view
fable_test_mcp
fable_test_ingest
fable_test_demoseed
fable_test_authority
```

Dropping `fable_test_kernel` does not touch `fable_test_api`. No cross-package
destruction is possible.

### Implementation

**Step 1: Add `SuiteDSN` to `internal/testdb/testdb.go`**

```go
// SuiteDSN returns a DSN for an isolated per-package test database.
// The name fable_test_<suite> ends in _test, so the Reset safety guard accepts it.
func SuiteDSN(suiteName string) string {
    base := DSN()
    u, err := url.Parse(base)
    if err != nil {
        return "postgresql://root@localhost:26260/fable_test_" + suiteName + "?sslmode=disable"
    }
    u.Path = "/fable_test_" + suiteName
    return u.String()
}
```

This is the only addition to `testdb`. Everything else is caller-side.

**Step 2: Update all 12 existing TestMains**

Each TestMain changes from:

```go
dsn = testdb.DSN()
```

to:

```go
dsn = testdb.SuiteDSN("kernel")  // or "belief", "intent", etc.
```

The rest of the TestMain pattern (AcquireResetLock, Reset, Open, m.Run,
ReleaseResetLock) is unchanged. The lock name is derived from the DSN via
`DBNameFromDSN`, so each package gets its own lock file automatically.

**Step 3: Add TestMain to api/**

`api/` is the only package with DB tests that lacks a TestMain. Add
`api/suite_test.go` with the standard pattern, using
`testdb.SuiteDSN("api")`.

Replace the hardcoded `testDSN` constant in `api/helpers_test.go` with
`testdb.DSN()` via `testdb.SuiteDSN("api")`.

**Step 4: Normalize schema files**

All 12 TestMains currently apply different subsets of 001-007. This creates
subtle schema-mismatch bugs. Normalize: every TestMain applies all 001-007.

Verified safe: the schema files are additive (CREATE TABLE, not CREATE TABLE
IF NOT EXISTS, but Reset drops and recreates the database first, so the
schema is always clean).

---

## Package-to-SuiteName Mapping

| Package | SuiteName | Schema | File |
|---|---|---|---|
| `kernel` | `kernel` | 001-007 | `kernel/suite_test.go` |
| `internal/belief` | `belief` | 001-007 | `internal/belief/belief_test.go` |
| `internal/intent` | `intent` | 001-007 | `internal/intent/intent_test.go` |
| `internal/pipeline` | `pipeline` | 001-007 | `internal/pipeline/pipeline_test.go` |
| `internal/corpus` | `corpus` | 001-007 | `internal/corpus/corpus_test.go` |
| `internal/wizard` | `wizard` | 001-007 | `internal/wizard/seed_test.go` |
| `internal/agentjacking` | `agentjacking` | 001-007 | `internal/agentjacking/ingest_test.go` |
| `internal/view` | `view` | 001-007 | `internal/view/explain_test.go` |
| `cmd/solvent-mcp` | `mcp` | 001-007 | `cmd/solvent-mcp/tools_authority_test.go` |
| `cmd/corpus-ingest` | `ingest` | 001-007 | `cmd/corpus-ingest/embed_test.go` |
| `internal/demoseed` | `demoseed` | 001-007 | `internal/demoseed/demoseed_test.go` |
| `service/authority` | `authority` | 001-007 | `service/authority/authority_integration_test.go` |
| `api` | `api` | 001-007 | `api/suite_test.go` (NEW) |

---

## MCP Test Fixes (unchanged from previous plan)

### Fix A: Initialize ledgerSvc in MCP TestMain

**File:** `cmd/solvent-mcp/tools_authority_test.go`

`handleSolventAuthorizeAction` calls `ledgerSvc.AuthorizeAndCreateIntent`
unconditionally. `ledgerSvc` is only initialized in `main()`, never in tests.
Tests that pass non-empty target_id/actor_id reach this call and panic.

Add to TestMain after authSvc init:

```go
auditSvc := audit.New(sharedDB)
ledgerSvc = ledger.New(sharedDB, auditSvc)
```

**Security:** `ledgerSvc` is a passthrough to the kernel. Initializing it in
tests does not weaken authority checks.

### Fix B: Restructure TestAJ_UserTypedOnUnpromotedBelief

**File:** `cmd/solvent-mcp/tools_agentjacking_test.go`

The test's original invariant (23503 gate on unpromoted belief) is obsolete
in the MCP handler. The atomic authority path rejects unpromoted beliefs
before intent creation. The 23503 gate is tested at the kernel level.

Rename to `TestAJ_UnpromotedBelief_AuthorityDenied` and assert:
1. Create principal, target, unpromoted belief
2. Do NOT activate the target
3. Provide valid target_id and actor_id
4. Authority evaluation → denied (target not activated)
5. Response has audit envelope (proving DB path was taken)
6. Response does NOT contain 23503
7. Response contains authority denial

### Fix C: Restructure TestAJ_UserTypedWithUnknownBeliefReachDB

**File:** `cmd/solvent-mcp/tools_agentjacking_test.go`

Same pattern: provide valid target_id and actor_id. The unknown belief fails
the cross-scenario guard. Verify error contains "not found".

### Fix D: Replace all text[:300] with truncate(text, 300)

**File:** `cmd/solvent-mcp/tools_agentjacking_test.go`

The package already has a safe `truncate` function at `main.go:730`. Replace
all five `text[:300]` in `t.Errorf` calls.

---

## Implementation Order

### Step 1: testdb.SuiteDSN

Add `SuiteDSN` to `internal/testdb/testdb.go`. One function, ~10 lines.

### Step 2: Update all 12 existing TestMains

For each TestMain:
1. Change `dsn = testdb.DSN()` → `dsn = testdb.SuiteDSN("<suite>")`
2. Normalize schema to 001-007
3. No other changes to the TestMain pattern

This is mechanical. Each file changes 2-3 lines.

### Step 3: Add api/ TestMain

New file `api/suite_test.go` with standard pattern. Update
`api/helpers_test.go` to use `testdb.SuiteDSN("api")`.

### Step 4: MCP TestMain — initialize ledgerSvc

Add `ledgerSvc` initialization to `cmd/solvent-mcp/tools_authority_test.go`.

### Step 5: MCP test restructuring

Fix tests in `cmd/solvent-mcp/tools_agentjacking_test.go`:
- Restructure TestAJ_UserTypedOnUnpromotedBelief
- Restructure TestAJ_UserTypedWithUnknownBeliefReachDB
- Replace text[:300] with truncate(text, 300) in all error messages

### Step 6: Clean stale lock files

```bash
rm -f /tmp/fable_test*.reset.lock
```

### Step 7: Diagnostic verification

```bash
go test -count=1 -p 1 ./...
```

### Step 8: Final verification

```bash
go test -count=1 ./...
go test -count=1 ./...
```

Two consecutive parallel green runs.

### Step 9: Key test spot-checks

```bash
go test -count=1 -v -run TestIntegration_AuthorizeAction_Atomicity ./api/...
go test -count=1 -v -run TestIntegration_AuthorizeAction_ActorIDMismatch ./api/...
go test -count=1 -v -run TestIntegration_ConcurrentRevokeTarget ./api/...
go test -count=1 -v -run TestAJ_UnpromotedBelief_AuthorityDenied ./cmd/solvent-mcp/...
go test -count=1 -v -run TestAuthorizeAction_ValidArgs ./cmd/solvent-mcp/...
```

---

## Security Checklist

| Check | Status |
|---|---|
| No kernel change | ✓ |
| No authority redesign | ✓ |
| No service redesign | ✓ |
| No API contract redesign | ✓ |
| No schema change (production) | ✓ |
| No test weakening | ✓ |
| No package skipping | ✓ |
| Actor-binding fix intact | ✓ (api/auth.go unchanged) |
| Atomic AuthorizeAndCreateIntent intact | ✓ (kernel/service unchanged) |

## Test Isolation Checklist

| Check | Status |
|---|---|
| Each package has isolated database | ✓ (per-package SuiteDSN) |
| Package A cannot DROP Package B's database | ✓ (different DB names) |
| No dependency on package execution order | ✓ (each TestMain self-contained) |
| No manually pre-created database required | ✓ |
| No production database mutation | ✓ (only *_test_* databases) |
| go test -count=1 ./... safe under parallelism | ✓ (per-package isolation) |
| go test -count=1 -p 1 ./... also passes | ✓ (diagnostic gate) |
