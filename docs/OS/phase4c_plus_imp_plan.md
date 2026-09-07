# Phase 4C+ Step 1 — Implementation Plan (Final Revision)

**Status:** Ready for execution
**Gate:** Kilo Code Gate 1 — GO (Test Engineering Gate Passed)
**Authoritative specification:** `.opencode/plans/phase4c_plus_test_target.md`
**Revision:** Addresses CI-2 contradiction, Scan(nil) bug, TOCTOU honesty, audit semantics

---

## Final Decisions

### A. ExecutionResult.Success Semantics

**The approved contract is authoritative:**

> CI-2: ExecutionResult.Success is true iff the executor function returned
> nil error.

This is absolute. `Success` reflects the provider/executor outcome. It does
NOT reflect Solvent's internal persistence state.

| Scenario | executor returns | result.Success | intent state |
|----------|------------------|----------------|--------------|
| Provider accepts, CompleteIntent succeeds | nil | true | executed |
| Provider accepts, CompleteIntent fails | nil | true | live |
| Provider rejects | error | false | live |
| Provider error/timeout | error | false | live |
| Executor not registered | N/A (never called) | false | live |

**CompleteIntent failure does NOT change Success.** The provider accepted.
The side effect occurred. The execution result is truthful.

### B. CompleteIntent Failure — Known v1 Limitation

If provider accepted (executor returned nil) but CompleteIntent persistence
fails:

- `result.Success = true` (CI-2 preserved)
- `result.Output` contains the run ID (the provider DID accept)
- Intent state remains `live` (the persistence failed)
- A subsequent execution MAY cause duplicate external execution
- This is a post-provider persistence failure, NOT a provider failure
- Deferred to Phase 4D idempotency work

**Test implication:** The approved test target does NOT test the
CompleteIntent-failure case. TestExec27 exercises the happy path.
This limitation is documented, not tested.

### C. Audit Semantics for Provider Acceptance + Persistence Failure

Four distinct audit event types:

| Event | When | Meaning |
|-------|------|---------|
| `authorization_granted` | kernel.Authorize allowed | Authority verified |
| `adapter_invoked` | before executor call | Executor about to run |
| `executor_completed` | executor returned nil AND CompleteIntent succeeded | Provider accepted, state persisted |
| `executor_failed` | executor returned error | Provider rejected/errored |

New event for persistence failure:

| Event | When | Meaning |
|-------|------|---------|
| `intent_completion_failed` | executor returned nil BUT CompleteIntent failed | Provider accepted, persistence failed |

**Critical distinction:** `intent_completion_failed` is NOT `executor_failed`.
The provider succeeded. The persistence failed. The audit event must not
falsely report the provider as failed.

The execution result remains truthful (`Success=true`). The audit event
records the persistence problem separately.

### D. Exact intentID Propagation

`ExecuteAction` gains `intentID string` parameter. The caller creates the
intent beforehand, receives the generated UUID, and passes it through.

```
caller creates intent → receives intentID
    ↓
ExecuteAction(..., intentID, ...)
    ↓
verifies intent is live (SELECT state FROM action_intent WHERE id = intentID)
    ↓
executor invocation
    ↓
CompleteIntent(scenarioID, intentID) — exact ID, no LIMIT 1
```

No `SELECT ... LIMIT 1` rediscovery. Exact identity preserved.

### E. Snapshot Parameter Retrieval — No Kernel Growth

**The snapshot's `consequence_parameters` are already read by the kernel
during every `Authorize` call** (`authorizeWithinTx`, line 373). The data is
in memory but discarded because `AuthorizeResult` does not carry it.

**Solution: extend `AuthorizeResult` to include the snapshot parameters.**

```go
type AuthorizeResult struct {
    Allowed               bool
    Reason                string
    IntentState           string
    ConsequenceParameters []byte  // NEW — snapshot's approved params
}
```

In `authorizeWithinTx`, after authorization succeeds, copy the already-scanned
`snapConsequenceParams` into the result. No new query. No new method. No new
kernel primitive.

**Why this is NOT kernel growth:**
1. The kernel already reads `consequence_parameters` in every `Authorize` call.
2. Returning existing data through the existing result type is information
   flow completion, not a new capability.
3. No new SQL. No new resolution logic. No new externally callable method.
4. The kernel still treats `consequence_parameters` as opaque data.

### F. Post-Authorization TOCTOU Boundary

The actual call flow between T2 (final authorization check) and T3 (executor
invocation):

```
T2: kernel.Authorize succeeds
    ↓  (AuthorizeResult.ConsequenceParameters already contains snapshot params)
intent-state read (SELECT state FROM action_intent WHERE id = $1)
    ↓  — one local DB read, no I/O
unmarshal decision.ConsequenceParameters in memory
    ↓  — in-memory, no DB
construct execParams from snapshot
    ↓
T3: executor invocation (fn(ctx, execParams))
```

**One additional local DB read** exists between T2 and T3: the exact
intent-state verification. The approved snapshot parameters are already
present in the `AuthorizeResult` returned at T2.

- Single-row SELECT (one index lookup)
- No external I/O
- Executed within the same Go goroutine
- Complete in microseconds
- Snapshot parameter reconstruction after T2 is in-memory (JSON unmarshal)

**Window B characterization remains accurate:**
- The window is bounded by local function calls, not MCP serialization
- The intent-state read re-checks revocation (if the intent was cancelled
  between T2 and the read, the state will not be 'live'). The snapshot
  parameters were already validated at T2 by kernel.Authorize.
- The window's small size is a property of the local function-call boundary
- No new architecture is needed

**Honest description for the plan:**

> Window B is the boundary between the final authorization check (T2) and
> executor invocation (T3). One additional local DB read (intent-state
> verification) occurs within this window. The approved snapshot parameters
> are already returned by Authorize at T2 and reconstructed in memory.
> A revocation arriving after T2 but before the intent-state read will be
> detected. A revocation arriving after the intent-state read but before T3
> is the documented v1 race.

---

## Revised Implementation Plan

### Summary

4 new files, 7 modified files. 54 tests.

**Track A:** Fake provider + executor (`adapter/github/`) — 3 new files
**Track B:** Kernel changes (`kernel/`) — 3 modified files
**Track C:** Service layer changes (`service/authority/`) — 4 modified files

---

### Track A: Fake Provider + Executor (`adapter/github/`)

#### File 1: `adapter/github/provider.go`

```go
type GitHubProvider interface {
    TriggerWorkflow(ctx context.Context, repo, workflow, ref string,
        inputs map[string]string) (*WorkflowResult, error)
}

type WorkflowResult struct {
    RunID string
}
```

#### File 2: `adapter/github/fake_provider.go`

As specified in test target lines 200-253. All methods, all behavioral
contracts, `callWg` synchronization primitive.

#### File 3: `adapter/github/executor.go`

```go
const ExecutorName = "github_trigger_workflow"

func NewExecutor(provider GitHubProvider) executor.ActionFunc { ... }
func RegisterExecutor(reg *executor.Registry, provider GitHubProvider) { ... }
```

Thin pass-through. Reads `repo`/`workflow`/`ref` from params (provided by
service from snapshot). Does NOT know about snapshots or authorization.

---

### Track B: Kernel Changes (`kernel/`)

#### File 4: `kernel/sql.go` — add `sqlCompleteIntent`

```go
sqlCompleteIntent = `
    UPDATE action_intent SET state = 'executed'
    WHERE id = $1::UUID AND scenario_id = $2::UUID AND state = 'live'`
```

No new snapshot query needed. The snapshot's `consequence_parameters` are
already read by `authorizeWithinTx` (line 373) during every `Authorize` call.
The data is in memory but discarded because `AuthorizeResult` does not carry it.

#### File 5: `kernel/authority.go` — add `CompleteIntent` + extend `AuthorizeResult`

**Add `CompleteIntent` method:**

```go
func (s *Store) CompleteIntent(ctx context.Context, scenarioID, intentID string) error {
    return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        _, err := tx.ExecContext(ctx, sqlCompleteIntent, intentID, scenarioID)
        return err
    })
}
```

**Extend `AuthorizeResult` to carry snapshot parameters:**

```go
type AuthorizeResult struct {
    Allowed                bool
    Reason                 string
    IntentState            string // populated by AuthorizeAndCreateIntent on success
    ConsequenceParameters  []byte // populated by Authorize — the snapshot's approved params
}
```

**Populate in `authorizeWithinTx`:**

The snapshot's `consequence_parameters` are already scanned into
`snapConsequenceParams` (line 373). After authorization succeeds, copy them
into the result:

```go
return AuthorizeResult{
    Allowed:               true,
    Reason:                "",
    ConsequenceParameters: snapConsequenceParams,
}, nil
```

**Why this is NOT kernel growth:**
1. The kernel already reads `consequence_parameters` in every `Authorize` call
   (line 373-378). The data is already in memory.
2. Returning it through the existing `AuthorizeResult` completes an information
   flow that already exists. No new query. No new resolution logic.
3. The kernel treats `consequence_parameters` as opaque data — it compares but
   does not interpret. Returning the raw bytes preserves this property.
4. No new externally callable read primitive is added. The existing
   `Authorize` method now returns what it already reads.

**`AuthorizeAndCreateIntent` also populates the field** (reuses
`authorizeWithinTx`, which already sets it).

#### File 6: `kernel/contract.go` — add `CompleteIntent` to `Contract` interface

```go
CompleteIntent(context.Context, string, string) error
```

`GetSnapshotConsequenceParams` is NOT needed on the interface. The snapshot
parameters flow through `AuthorizeResult`, not through a separate method.

Compile-time assertion `var _ Contract = (*Store)(nil)` verifies.

---

### Track C: Service Layer Changes (`service/authority/`)

#### File 7: `service/authority/authority.go`

**Change 1: Extend `ExecuteAction` signature**

```go
func (s *Service) ExecuteAction(
    ctx context.Context,
    scenarioID, beliefID, action, targetID, actorID string,
    intentID string,          // NEW — exact intent identity
    params map[string]interface{},
    consequenceType string,
    consequenceParameters []byte,
) (*ExecutionResult, error)
```

**Change 2: Action→executor mapping**

```go
var actionExecutorMap = map[string]string{
    "deploy": executor.ExecutorName,
}

func resolveExecutor(action string) (string, bool) {
    name, ok := actionExecutorMap[action]
    return name, ok
}
```

Replace `params["tool_name"]` lookup with `resolveExecutor(action)`.

**Change 3: Verify exact intent is live**

After authorization succeeds, before executor invocation:

```go
var intentState string
err = s.db.QueryRowContext(ctx, `
    SELECT state FROM action_intent
    WHERE id = $1::UUID AND scenario_id = $2::UUID
      AND belief_id = $3::UUID AND action = $4`,
    intentID, scenarioID, beliefID, action).Scan(&intentState)
if err != nil {
    result.Success = false
    result.Error = fmt.Sprintf("intent not found: %v", err)
    return result, nil
}
if intentState != "live" {
    result.Allowed = false
    result.Error = fmt.Sprintf("intent state is %q, not live", intentState)
    return result, nil
}
```

**Change 4: Snapshot parameter reconstruction**

The snapshot's `consequence_parameters` are already in
`decision.ConsequenceParameters` — returned by `kernel.Authorize` as part
of the extended `AuthorizeResult`. No second kernel call needed.

```go
var snapParams map[string]interface{}
if err := json.Unmarshal(decision.ConsequenceParameters, &snapParams); err != nil {
    result.Success = false
    result.Error = fmt.Sprintf("unmarshal snapshot params: %v", err)
    return result, nil
}

execParams := map[string]interface{}{
    "repo":     snapParams["repo"],
    "workflow": snapParams["workflow"],
    "ref":      snapParams["ref"],
}
```

Call `fn(ctx, execParams)` — NOT `fn(ctx, params)`.

**Key property:** The snapshot parameters flow through the authorization
result, not through a separate kernel call. The kernel already reads them
during `Authorize`. This completes the information flow without adding a new
kernel primitive.

**Change 5: CompleteIntent — preserves truthful execution result**

```go
output, execErr := fn(ctx, execParams)

if execErr != nil {
    // Provider rejected/errored.
    result.Success = false
    result.Error = execErr.Error()
    s.audit.Log(ctx, &audit.ActivityEntry{
        ScenarioID: scenarioID,
        Type:       audit.ActivityExecutorFailed,
        // ...
    })
    return result, nil
}

// Provider accepted. Execution result is truthful.
result.Success = true    // CI-2: executor returned nil
result.Output = output

// Attempt to persist the execution fact.
if err := s.kern.CompleteIntent(ctx, scenarioID, intentID); err != nil {
    // Persistence failure. Provider DID accept. Result remains truthful.
    // Intent may remain 'live' — known v1 duplicate-execution risk.
    s.audit.Log(ctx, &audit.ActivityEntry{
        ScenarioID: scenarioID,
        Type:       audit.ActivityIntentCompletionFailed,  // NEW event type
        ActorID:    actorID,
        SubjectID:  beliefID,
        Details: map[string]interface{}{
            "target_id": targetID,
            "action":    action,
            "intent_id": intentID,
            "error":     err.Error(),
            "note":      "provider accepted but intent state not persisted",
        },
    })
    return result, nil
}

// Persistence succeeded.
s.audit.Log(ctx, &audit.ActivityEntry{
    ScenarioID: scenarioID,
    Type:       audit.ActivityExecutorCompleted,
    ActorID:    actorID,
    SubjectID:  beliefID,
    Details: map[string]interface{}{
        "target_id": targetID,
        "action":    action,
        "intent_id": intentID,
    },
})
return result, nil
```

**Key invariant:** `result.Success` is set ONCE, immediately after the
executor returns. CompleteIntent failure does NOT overwrite it.

**Change 6: Add new audit event type**

In `service/audit/audit.go`, add:

```go
ActivityIntentCompletionFailed = "intent_completion_failed"
```

This is distinct from `ActivityExecutorFailed` (provider rejection/error).
The provider DID accept. The persistence failed.

**Test contract compatibility:** The approved test target does NOT test the
CompleteIntent-failure audit path. No existing test asserts on
`intent_completion_failed`. Adding this event type does not change any
existing test's pass/fail behavior. The new event is a documentation
addition, not a semantic change to tested behavior.

---

### Test Infrastructure Changes

#### File 8: `service/authority/authority_test.go`

The existing `mockKernelStore` only implements `Authorize`. It does NOT
implement the full `Contract` interface. This is fine because `s.kern` is
typed as `*kernel.Store` (concrete), not `kernel.Contract` (interface).

The mock's `Authorize` method must now return `ConsequenceParameters` in
the result:

```go
func (m *mockKernelStore) Authorize(ctx context.Context, targetID string, tuple kernel.AuthorityTuple) (kernel.AuthorizeResult, error) {
    m.lastTargetID = targetID
    m.lastTuple = tuple
    m.callCount++
    m.authorizeResult.ConsequenceParameters = tuple.ConsequenceParameters
    return m.authorizeResult, m.authorizeErr
}
```

If `CompleteIntent` is called in tests, add a stub method. Current tests are
all skipped (`t.Skip("requires database")`). Minimal changes.

#### File 9: `service/authority/authority_integration_test.go`

**Helper changes:**

1. `testConsequenceParams()` → GitHub params
2. `createApprovedTarget()` → unchanged (already calls `IntentOnPromoted`)
3. Add `createLiveIntent()` helper using `INSERT ... RETURNING id`
4. `newTestService()` → accept `GitHubProvider`, register executor

**Test changes:**
- All 27 `svc.ExecuteAction(...)` calls in authority_integration_test.go:
  add `intentID` parameter
  (e.g., `svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
  intentID, params, ...)`)
- DENIED tests: pass empty `intentID` (`""`) — intent verification will fail
  before executor is reached, which is correct for denial tests
- ALLOWED tests (TestP01): pass valid `intentID` from `createLiveIntent`
- Remove `tool_name` from all `params` maps
- Replace `rec.Called()` with `provider.CallCount()`

---

### File 10: `adapter/github/executor_test.go` — 54 Tests

#### Test categories

| Category | Tests | Needs DB | Key assertion |
|----------|-------|----------|---------------|
| Authorization (12.1) | TestExec01-09 | Yes | Allowed/Denied + provider call count |
| Parameter binding (12.2) | TestExec10-14, TestExec36 | 10-13: kernel; 14,36: Yes | Tuple mismatch / snapshot params |
| TOCTOU (12.3) | TestExec15A, 15B, 16-19 | Yes | Race synchronization |
| Provider failure (12.4) | TestExec20-24 | Yes | Success/Error + provider call count |
| Duplicate execution (12.5) | TestExec25-26 | Yes | Intent state |
| CompleteIntent (12.6) | TestExec27-30 | 27-29: Yes; 30: grep | Intent state transition |
| Audit (12.7) | TestExec31-35, 38 | Yes | Audit entry ordering/types |
| Adversarial (12.8) | TestAT01-16 | Yes (most) | Security property violation |

#### Call flow for execution-path tests

```
setup: create principal, promote belief, create approved target,
       create live intent → intentID
       register executor, fake provider
call:  ExecuteAction(..., intentID, params, ...)
internal:
  1. PrepareForAction → kernel.Authorize → allowed
     (AuthorizeResult.ConsequenceParameters now contains snapshot params)
  2. resolveExecutor(action) → "github_trigger_workflow"
  3. SELECT state FROM action_intent WHERE id = intentID → "live"
  4. json.Unmarshal(decision.ConsequenceParameters) → {repo, workflow, ref}
  5. execParams = {repo: snapParams["repo"], workflow: snapParams["workflow"], ref: snapParams["ref"]}
  6. fn(ctx, execParams) → provider.TriggerWorkflow(...)
  7. if accepted: CompleteIntent(scenarioID, intentID)
assert: Allowed=true, Success=true, provider.CallCount()==1
```

---

## Post-Authorization Window — Honest Description

```
T2: kernel.Authorize succeeds (authorization re-read confirms authority)
    │  AuthorizeResult.ConsequenceParameters contains snapshot params
    │
    ├─ intent-state read ─── SELECT state FROM action_intent WHERE id = $1
    │   (single-row index lookup, no I/O, revocation re-checked)
    │
    ├─ unmarshal decision.ConsequenceParameters ── in-memory, no DB
    │
    ├─ construct execParams from snapshot
    │
T3: fn(ctx, execParams) ── executor invokes provider
```

The intent-state read between T2 and T3:
- Is a single-row index lookup — microseconds
- Occurs within the same goroutine — no concurrency boundary
- Does NOT expand the TOCTOU characterization
- Snapshot parameters are already present from Authorize, not re-read from DB

**Window B = T2 to T3.** One local DB read (intent-state) and one in-memory
unmarshal are inside this window. A revocation arriving after T2 but before
the intent-state read is detected. A revocation arriving after the
intent-state read but before T3 is the documented v1 race. The window
remains bounded by local function calls.

---

## Files Changed Summary

| File | Action | Lines (est.) |
|------|--------|-------------|
| `adapter/github/provider.go` | Create | ~20 |
| `adapter/github/fake_provider.go` | Create | ~120 |
| `adapter/github/executor.go` | Create | ~40 |
| `adapter/github/executor_test.go` | Create | ~1200 |
| `kernel/sql.go` | Modify | +5 |
| `kernel/authority.go` | Modify | +25 |
| `kernel/contract.go` | Modify | +2 |
| `service/authority/authority.go` | Modify | +80 |
| `service/authority/authority_test.go` | Modify | ~15 |
| `service/authority/authority_integration_test.go` | Modify | ~60 |
| `service/audit/audit.go` | Modify | +1 |

**Total:** 4 new files, 7 modified files. ~1,570 new lines, ~85 modified lines.

---

## Verification

```bash
task lint:openapi
task test:openapi
go build ./...
go vet ./...
go test -count=1 -p 1 ./...
go test -count=1 ./...
go test -count=1 -v ./adapter/github/...
go test -count=1 -v ./service/authority/...
```

Real GitHub integration must never execute during normal `go test ./...`.

---

## Known v1 Limitations

1. **Window B TOCTOU race.** Revocation after T2 but before T3 may result
   in execution on stale authority. One local DB read (intent state) occurs
   within this window and re-checks revocation. Snapshot parameters are
   already returned by Authorize. Bounded by local function-call boundary.

2. **Concurrent duplicate execution.** Two goroutines may both pass
   authorization. Intent state check prevents sequential duplicates.
   Phase 4D adds idempotency.

3. **CompleteIntent persistence failure.** Provider accepted, execution
   result is truthful (`Success=true`), but intent state may remain `live`.
   Subsequent execution may cause duplicate external side effect. Deferred
   to Phase 4D idempotency.

4. **Audit gap on persistence failure.** `intent_completion_failed` event
   records the problem. Provider outcome remains truthful in the execution
   result.

---

## Consistency Verification

| Source | Requirement | Implementation | Status |
|--------|-------------|----------------|--------|
| CI-2 | `Success == true iff executor returned nil` | Success set once after executor, never overwritten | Consistent |
| TestExec01 | Happy path: authorize → execute → complete | Full flow with intentID | Consistent |
| TestExec10-13 | Kernel-level tuple binding | Call kernel.Authorize directly | Consistent |
| TestExec14 | Matching caller params | Service reconstructs from snapshot | Consistent |
| TestExec15A | Revocation before T2 → deny | PrepareForAction re-reads state | Consistent |
| TestExec15B | Window B characterization | One DB read + in-memory unmarshal between T2 and T3, documented | Consistent |
| TestExec25 | Sequential duplicate prevention | Intent state check | Consistent |
| TestExec27 | Provider acceptance → executed | CompleteIntent with exact intentID | Consistent |
| TestExec30 | CompleteIntent provenance | Only from ExecuteAction, no REST/MCP path | Consistent |
| TestExec35 | Provider success does not mutate authority | Only intent state changes | Consistent |
| TestExec36 | Snapshot is sole source of truth | Service reads snapshot, ignores caller params | Consistent |
| TestExec38 | Provider success + Solvent error | Executor error path, no CompleteIntent | Consistent |
| TestAT01-16 | Adversarial properties | All controls tested | Consistent |
| SI-16/17/18 | Parameter binding | Two levels: kernel + execution path | Consistent |
