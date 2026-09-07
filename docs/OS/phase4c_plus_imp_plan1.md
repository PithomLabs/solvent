# Phase 4C+ Step 1 — Implementation Plan (Revised)

**Status:** Ready for execution
**Gate:** Kilo Code Gate 1 — GO (Test Engineering Gate Passed)
**Authoritative specification:** `.opencode/plans/phase4c_plus_test_target.md`
**Revision:** Addresses 8 design issues from adversarial review

---

## Design Decisions

### Decision A: CompleteIntent Failure Semantics

**Current behavior (plan v1):** CompleteIntent failure is ignored. Execution
reports `Success=true`. Intent remains `live`.

**Problem:** Provider accepted deployment. CompleteIntent fails. Intent stays
`live`. Same intent may be executed again → duplicate external side effect.

**Revised v1 semantics:**

| Scenario | `result.Success` | Intent state | Subsequent execution |
|----------|-------------------|--------------|----------------------|
| Provider accepts, CompleteIntent succeeds | `true` | `executed` | Denied (intent state check) |
| Provider accepts, CompleteIntent fails | `false` | `live` | Permitted (known v1 duplicate risk) |
| Provider rejects | `false` | `live` | Permitted |
| Provider error/timeout | `false` | `live` | Permitted |

**Why this is acceptable for v1:**

1. CompleteIntent is a single-row `UPDATE ... SET state = 'executed'`. Failure
   is extremely unlikely (requires DB failure in the same transaction window).
2. The provider acceptance is synchronous in Phase 4C+ (workflow_dispatch
   returns immediately). The duplicate window is bounded.
3. MCP sequential processing means the same intent is unlikely to be resubmitted
   before the first completion persists.
4. The test target does NOT test the CompleteIntent-failure case. It is outside
   the approved test matrix.
5. Phase 4D will add idempotency infrastructure if needed.

**Implementation:**

```go
// After executor returns nil error (provider accepted):
if err := s.kern.CompleteIntent(ctx, scenarioID, intentID); err != nil {
    result.Success = false
    result.Error = fmt.Sprintf("intent completion failed (provider accepted): %v", err)
    s.audit.Log(ctx, &audit.ActivityEntry{
        ScenarioID: scenarioID,
        Type:       audit.ActivityExecutorFailed,
        ActorID:    actorID,
        SubjectID:  beliefID,
        Details: map[string]interface{}{
            "target_id": targetID,
            "action":    action,
            "error":     result.Error,
        },
    })
    return result, nil
}
```

**Test implication:** TestExec27 expects `state == 'executed'` after success.
This test exercises the happy path (CompleteIntent succeeds). The failure path
is a documented v1 limitation, not a test matrix entry.

**Known v1 limitation:** If CompleteIntent fails after provider acceptance,
the intent remains `live` and a subsequent execution may trigger a duplicate
external side effect. This is a persistence failure, not a security invariant
violation.

---

### Decision B: Exact intent_id Propagation

**Current state:** `ExecuteAction` does not receive or carry an `intent_id`.
The `action_intent` table has `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`.
The INSERT (`sqlIntentOnPromoted`) does not `RETURNING id`. The application
never captures the generated UUID.

**Problem:** The plan used `SELECT id FROM action_intent WHERE ... LIMIT 1`
to rediscover the intent. This is too weak — it does not preserve exact
identity.

**Smallest architectural change:**

1. **Extend `ExecuteAction` signature** to accept `intentID string`:

```go
func (s *Service) ExecuteAction(
    ctx context.Context,
    scenarioID, beliefID, action, targetID, actorID string,
    intentID string,          // NEW — exact intent to execute
    params map[string]interface{},
    consequenceType string,
    consequenceParameters []byte,
) (*ExecutionResult, error)
```

2. **Extend `AuthorizationDecision`** to carry intent identity:

```go
type AuthorizationDecision struct {
    Allowed      bool   `json:"allowed"`
    Reason       string `json:"reason,omitempty"`
    BeliefID     string `json:"belief_id"`
    BeliefStatus string `json:"belief_status"`
    TargetID     string `json:"target_id"`
    Action       string `json:"action"`
    IntentID     string `json:"intent_id"`    // NEW
    CheckedAt    time.Time `json:"checked_at"`
}
```

3. **Verify intent exists and is `live`** inside `ExecuteAction` before
   proceeding to execution:

```go
// After authorization succeeds, verify the exact intent is live.
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

4. **Use exact `intentID`** for CompleteIntent:

```go
if err := s.kern.CompleteIntent(ctx, scenarioID, intentID); err != nil {
    // ... (Decision A semantics)
}
```

**Why not query during PrepareForAction:** PrepareForAction is a read-only
authorization check. It should not bind to a specific intent. The intent
binding happens in ExecuteAction, after authorization succeeds.

**Why not use LIMIT 1:** The test target requires "the exact intent identity
that is being executed." A broad query + LIMIT 1 does not provide that
guarantee. The caller must supply the exact ID.

**Caller responsibility:** The MCP tool handler (or test helper) must create
the intent before calling ExecuteAction and pass the returned `intent_id`.
This is consistent with the test target's setup pattern: "Create approved
target with live intent."

**Test infrastructure impact:**
- `createApprovedTarget` already calls `IntentOnPromoted` (from plan v1).
- Add a helper `createLiveIntent` that uses `INSERT ... RETURNING id`:
  ```go
  func createLiveIntent(t *testing.T, ctx context.Context, st *kernel.Store,
      scenarioID, beliefID, action string) string {
      t.Helper()
      var id string
      err := st.DB().QueryRowContext(ctx, `
          INSERT INTO action_intent (scenario_id, belief_id, action)
          VALUES ($1::UUID, $2::UUID, $3) RETURNING id`,
          scenarioID, beliefID, action).Scan(&id)
      if err != nil {
          t.Fatalf("setup (create intent): %v", err)
      }
      return id
  }
  ```
- All test calls to `ExecuteAction` must pass the `intentID`.

---

### Decision C: GetSnapshotConsequenceParams — Kernel Growth Justification

**Current plan:** Adds `kernel.GetSnapshotConsequenceParams(targetID)`.

**Question:** Can the service layer obtain the same data without expanding
the kernel?

**Analysis:**

The service has `*sql.DB` and could execute:
```sql
SELECT ts.consequence_parameters
FROM target_activation ta
JOIN target_snapshot ts ON ts.target_id = ta.target_id
    AND ts.snapshot_id = ta.snapshot_id
WHERE ta.target_id = $1
  AND NOT EXISTS (
    SELECT 1 FROM target_revocation WHERE target_id = $1
  )
```

**But this duplicates kernel logic.** The snapshot resolution (activation +
snapshot + no revocation) is already implemented in `authorizeWithinTx`
(kernel/authority.go:367-408). The service duplicating this query would:
- Violate DRY (two implementations of the same resolution logic)
- Risk drift if the kernel's resolution logic changes
- Undermine the architectural principle that the kernel is the authority oracle

**Decision: Add the kernel method.** Justification under kernel-growth gate:

1. **Read-only.** The method performs zero writes. It reads
   `consequence_parameters` from the same snapshot that `authorizeWithinTx`
   already reads.
2. **No new semantics.** It does not introduce new authority logic. It exposes
   data that the kernel already reads internally.
3. **Existing query pattern.** The SQL is identical to `sqlAuthorizeResolve`
   (the kernel's own snapshot resolution query), selecting only
   `consequence_parameters`.
4. **Provider-specific reconstruction stays above the kernel.** The method
   returns the raw JSONB `consequence_parameters`. The service layer
   interprets it (extracting `repo`/`workflow`/`ref`). The kernel does NOT
   understand GitHub semantics.
5. **Minimal surface.** One method, ~20 lines. No new types, no new SQL
   patterns, no new invariants.

**Revised implementation:**

```go
// GetSnapshotConsequenceParams reads the consequence_parameters from the
// approved, non-revoked snapshot for the given target.
//
// This is a read-only operation. It reuses the same snapshot resolution
// logic as authorizeWithinTx. The kernel treats consequence_parameters
// as opaque data — provider-specific interpretation happens in the service
// layer.
func (s *Store) GetSnapshotConsequenceParams(ctx context.Context, targetID string) (map[string]interface{}, error) {
    var paramsJSON []byte
    err := s.db.QueryRowContext(ctx, sqlAuthorizeResolve, targetID).Scan(
        nil, nil, nil, nil, nil, nil, nil, &paramsJSON, nil,
    )
    if err != nil {
        return nil, fmt.Errorf("snapshot not found for target %s: %w", targetID, err)
    }
    var params map[string]interface{}
    if err := json.Unmarshal(paramsJSON, &params); err != nil {
        return nil, fmt.Errorf("unmarshal snapshot params: %w", err)
    }
    return params, nil
}
```

Note: Reuses `sqlAuthorizeResolve` (the kernel's own query). Scans only
`consequence_parameters` (other columns scanned into `nil`).

---

## Revised Implementation Plan

### Summary

Step 1 requires creating 4 new files and modifying 7 existing files. The work
decomposes into 3 parallel tracks that converge:

**Track A:** Fake provider + executor infrastructure (`adapter/github/`)
**Track B:** Kernel changes — CompleteIntent + GetSnapshotConsequenceParams
**Track C:** Service layer changes — mapping, snapshot reconstruction,
intent_id binding, CompleteIntent call

All three tracks must complete before `executor_test.go` can pass.

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

As specified in test target lines 200-253. Fields: `mu`, `calls`, `accept`,
`err`, `runID`, `lostResponse`, `callWg`. Methods: `NewFakeGitHubProvider`,
`TriggerWorkflow`, `Calls`, `CallCount`, `LastCall`, `SetAccept`, `SetErr`,
`SetLostResponse`, `WaitForCalls`.

Behavioral contract:
- `accept=true`: returns `&WorkflowResult{RunID}`, nil
- `accept=false`: returns `nil, f.err`
- `lostResponse=true && accept=true`: records call, returns
  `nil, fmt.Errorf("response lost: provider accepted but response not received")`
- `callWg.Done()` on every call

#### File 3: `adapter/github/executor.go`

```go
const ExecutorName = "github_trigger_workflow"

func NewExecutor(provider GitHubProvider) executor.ActionFunc {
    return func(ctx context.Context, params map[string]interface{}) (string, error) {
        repo, _ := params["repo"].(string)
        workflow, _ := params["workflow"].(string)
        ref, _ := params["ref"].(string)
        result, err := provider.TriggerWorkflow(ctx, repo, workflow, ref, nil)
        if err != nil {
            return "", err
        }
        return result.RunID, nil
    }
}

func RegisterExecutor(reg *executor.Registry, provider GitHubProvider) {
    reg.Register(ExecutorName, NewExecutor(provider))
}
```

Executor is a thin pass-through. Service layer provides snapshot values as
`params`. Executor does NOT know about snapshots or authorization.

---

### Track B: Kernel Changes (`kernel/`)

#### File 4: `kernel/sql.go` — add `sqlCompleteIntent`

```sql
sqlCompleteIntent = `
    UPDATE action_intent SET state = 'executed'
    WHERE id = $1::UUID AND scenario_id = $2::UUID AND state = 'live'`
```

Transitions `live` → `executed`. Idempotent (no-op if already executed).
Scoped to `scenario_id` for safety.

#### File 5: `kernel/authority.go` — add `CompleteIntent`

```go
func (s *Store) CompleteIntent(ctx context.Context, scenarioID, intentID string) error {
    return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        _, err := tx.ExecContext(ctx, sqlCompleteIntent, intentID, scenarioID)
        return err
    })
}
```

#### File 6: `kernel/contract.go` — add to `Contract` interface

```go
CompleteIntent(context.Context, string, string) error
```

Compile-time assertion `var _ Contract = (*Store)(nil)` verifies.

#### File 7: `kernel/authority.go` — add `GetSnapshotConsequenceParams`

Reuses `sqlAuthorizeResolve` (the kernel's own snapshot resolution query).
Scans only `consequence_parameters` into `paramsJSON`. Returns parsed
`map[string]interface{}`.

**Kernel-growth justification:** Read-only. No new semantics. Exposes data
the kernel already reads. Provider-specific interpretation stays in the
service layer. See Decision C above.

---

### Track C: Service Layer Changes (`service/authority/`)

#### File 8: `service/authority/authority.go`

**Change 1: Extend `ExecuteAction` signature**

```go
func (s *Service) ExecuteAction(
    ctx context.Context,
    scenarioID, beliefID, action, targetID, actorID string,
    intentID string,          // NEW
    params map[string]interface{},
    consequenceType string,
    consequenceParameters []byte,
) (*ExecutionResult, error)
```

**Change 2: Action→executor mapping (replace lines 196-208)**

```go
var actionExecutorMap = map[string]string{
    "deploy": executor.ExecutorName,
}

func resolveExecutor(action string) (string, bool) {
    name, ok := actionExecutorMap[action]
    return name, ok
}
```

Replace `params["tool_name"]` lookup with:
```go
execName, ok := resolveExecutor(action)
if !ok {
    result.Success = false
    result.Error = fmt.Sprintf("no executor registered for action: %s", action)
    return result, nil
}
fn, ok := s.execReg.Get(execName)
```

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

After authorization succeeds:
```go
snapParams, err := s.kern.GetSnapshotConsequenceParams(ctx, targetID)
if err != nil {
    result.Success = false
    result.Error = fmt.Sprintf("snapshot read failed: %v", err)
    return result, nil
}

execParams := map[string]interface{}{
    "repo":     snapParams["repo"],
    "workflow": snapParams["workflow"],
    "ref":      snapParams["ref"],
}
```

Call `fn(ctx, execParams)` instead of `fn(ctx, params)`.

**Change 5: CompleteIntent after provider acceptance**

After executor returns nil error:
```go
if err := s.kern.CompleteIntent(ctx, scenarioID, intentID); err != nil {
    result.Success = false
    result.Error = fmt.Sprintf("intent completion failed (provider accepted): %v", err)
    s.audit.Log(ctx, &audit.ActivityEntry{
        ScenarioID: scenarioID,
        Type:       audit.ActivityExecutorFailed,
        ActorID:    actorID,
        SubjectID:  beliefID,
        Details: map[string]interface{}{
            "target_id": targetID,
            "action":    action,
            "error":     result.Error,
        },
    })
    return result, nil
}
```

**Change 6: Log audit on CompleteIntent success**

After successful CompleteIntent:
```go
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
```

---

### Test Infrastructure Changes

#### File 9: `service/authority/authority_test.go`

Add `GetSnapshotConsequenceParams` to `mockKernelStore` (returns error or
mock data). Most tests are skipped — minimal changes.

#### File 10: `service/authority/authority_integration_test.go`

**Helper changes:**

1. `testConsequenceParams()` → GitHub params:
```go
func testConsequenceParams() []byte {
    p, _ := json.Marshal(map[string]string{
        "repo": "org/test", "workflow": "deploy.yml", "ref": "main",
    })
    return p
}
```

2. `createApprovedTarget()` → no change (already calls `IntentOnPromoted`)

3. Add `createLiveIntent()` helper:
```go
func createLiveIntent(t *testing.T, ctx context.Context, db *sql.DB,
    scenarioID, beliefID, action string) string {
    t.Helper()
    var id string
    err := db.QueryRowContext(ctx, `
        INSERT INTO action_intent (scenario_id, belief_id, action)
        VALUES ($1::UUID, $2::UUID, $3) RETURNING id`,
        scenarioID, beliefID, action).Scan(&id)
    if err != nil {
        t.Fatalf("setup (create intent): %v", err)
    }
    return id
}
```

4. `newTestService()` → accept `GitHubProvider`, register executor:
```go
func newTestService(t *testing.T, provider github.GitHubProvider) *Service {
    t.Helper()
    pol := policy.New(shared)
    aud := audit.New(shared)
    reg := executor.NewRegistry()
    github.RegisterExecutor(reg, provider)
    return New(shared, pol, aud, reg)
}
```

**Test changes:**

- All `ExecuteAction` calls: add `intentID` parameter
- Remove `tool_name` from all `params` maps
- Update `newTestService` calls to pass `FakeGitHubProvider`
- TestP01: `fp := github.NewFakeGitHubProvider(true, "run-123")`

---

### File 11: `adapter/github/executor_test.go` — Test Matrix

#### Test categories

| Category | Tests | Needs DB | Key assertion |
|----------|-------|----------|---------------|
| Authorization (12.1) | TestExec01-09 | Yes | Allowed/Denied + provider call count |
| Parameter binding (12.2) | TestExec10-14, TestExec36 | 10-13: Yes; 14,36: Yes | Tuple mismatch / snapshot params |
| TOCTOU (12.3) | TestExec15A, 15B, 16-19 | Yes | Race synchronization |
| Provider failure (12.4) | TestExec20-24 | Yes | Success/Error + provider call count |
| Duplicate execution (12.5) | TestExec25-26 | Yes | Intent state |
| CompleteIntent (12.6) | TestExec27-30 | 27-29: Yes; 30: No (grep) | Intent state transition |
| Audit (12.7) | TestExec31-35, 38 | Yes | Audit entry ordering/types |
| Adversarial (12.8) | TestAT01-16 | Yes (most) | Security property violation |

#### Test helpers

- `TestMain` — database setup (same pattern as authority_integration_test.go)
- `newTestService` — create Service with FakeGitHubProvider
- `createPrincipal`, `createAndPromoteBelief` — same as authority helpers
- `createGitHubApprovedTarget` — with `{"repo":"org/test","workflow":"deploy.yml","ref":"main"}`
- `createLiveIntent` — INSERT ... RETURNING id
- `readIntentState` — query `action_intent.state`
- `readAuditEntries` — query audit log
- `readAuthorityState` — capture pre-execution state (for TestExec35)

#### Key implementations

**TestExec01 (happy path):**
```
setup: create principal, promote belief, create approved target,
       create live intent (intentID), register executor, fake provider accepts
call: ExecuteAction(..., intentID, params, ...)
assert: Allowed=true, Success=true, provider.CallCount()==1,
        intent state == 'executed'
```

**TestExec10-13 (kernel-level binding):**
Call `kernel.Authorize` directly with substituted tuples. Assert
`Allowed == false`. These do NOT go through ExecuteAction.

**TestExec25 (sequential duplicate):**
```
call 1: ExecuteAction(..., intentID, ...) → Success=true, state='executed'
call 2: ExecuteAction(..., intentID, ...) → Allowed=false (intent state check)
assert: provider.CallCount()==1
```

**TestExec27 (CompleteIntent success):**
```
setup: create live intent
call: ExecuteAction(..., intentID, ...) with provider accepting
assert: intent state == 'executed', Success=true
```

**TestExec30 (structural — grep):**
Verify CompleteIntent is on Contract interface, not in REST/MCP handlers.
Only call site is service/authority/authority.go.

**TestExec35 (authority state):**
Capture pre-execution state of target, activation, snapshot, belief,
principal rows. Execute successfully. Re-query all. Assert byte-for-byte
equality (except intent state).

**TestExec36 (snapshot binding):**
Three variants with a custom executor that records exact params:
- a) matching + extra → executor receives ONLY snapshot fields
- b) conflicting → executor receives snapshot values (not caller's)
- c) extra provider-looking → executor receives ONLY snapshot fields

**TestExec15B (TOCTOU Window B):**
Channel synchronization:
1. Goroutine A: passes T2, signals `afterT2`, waits on `revoked`,
   invokes executor
2. Goroutine B: waits on `afterT2`, revokes via separate DB connection,
   signals `revoked`
3. Assert: `Allowed == true`, provider call count == 1

---

## Call Flow (ExecuteAction)

```
ExecuteAction(ctx, scenarioID, beliefID, action, targetID, actorID,
              intentID, params, consequenceType, consequenceParameters)
    │
    ├─ PrepareForAction(...)
    │   ├─ getBeliefStatus(...)
    │   ├─ Construct AuthorityTuple (from context, NOT from params)
    │   ├─ kern.Authorize(targetID, tuple)
    │   │   └─ authorizeWithinTx (read-only: activation + snapshot + no revocation)
    │   │       └─ sqlAuthorizeResolve (reads consequence_parameters internally)
    │   └─ audit.Log(authorization_granted/denied)
    │
    ├─ If denied: audit.Log(executor_denied), return
    │
    ├─ resolveExecutor(action) → "github_trigger_workflow"
    │   └─ execReg.Get(execName) → ActionFunc
    │
    ├─ Verify exact intent is live:
    │   SELECT state FROM action_intent WHERE id = intentID
    │   └─ If not found or not live: return error
    │
    ├─ kern.GetSnapshotConsequenceParams(targetID)
    │   └─ sqlAuthorizeResolve (reuses kernel's snapshot query)
    │   └─ Returns map[repo, workflow, ref]
    │
    ├─ execParams = {repo: snapParams["repo"], workflow: snapParams["workflow"], ref: snapParams["ref"]}
    │
    ├─ audit.Log(adapter_invoked)
    │
    ├─ fn(ctx, execParams)  [executor calls provider.TriggerWorkflow]
    │   └─ Provider accepts or rejects
    │
    ├─ If provider error:
    │   ├─ result.Success = false
    │   └─ audit.Log(executor_failed)
    │
    ├─ If provider accepted:
    │   ├─ kern.CompleteIntent(scenarioID, intentID)
    │   │   └─ UPDATE action_intent SET state = 'executed' WHERE id = intentID
    │   │
    │   ├─ If CompleteIntent fails:
    │   │   ├─ result.Success = false
    │   │   └─ audit.Log(executor_failed)
    │   │
    │   └─ If CompleteIntent succeeds:
    │       ├─ result.Success = true
    │       ├─ result.Output = runID
    │       └─ audit.Log(executor_completed)
    │
    └─ return result
```

---

## Test-to-Implementation Traceability

| Test | Implementation feature | File |
|------|----------------------|------|
| TestExec01 | Happy path: authorize → execute → complete | executor_test.go |
| TestExec02-06 | Authorization denial paths | executor_test.go |
| TestExec07 | Executor not registered | executor_test.go |
| TestExec08, 15A | Window A: revocation before check → deny | executor_test.go |
| TestExec09 | Action→executor mapping (not caller params) | executor_test.go |
| TestExec10-13 | Kernel-level tuple binding | executor_test.go (calls kernel.Authorize directly) |
| TestExec14 | Matching caller params (compatibility) | executor_test.go |
| TestExec15B | Window B: deterministic characterization | executor_test.go |
| TestExec16-19 | TOCTOU: state changes during execution | executor_test.go |
| TestExec20-24 | Provider failure modes | executor_test.go |
| TestExec25-26 | Duplicate execution | executor_test.go |
| TestExec27-29 | CompleteIntent state transitions | executor_test.go |
| TestExec30 | Structural: CompleteIntent provenance | executor_test.go (grep) |
| TestExec31-34 | Audit ordering and distinct types | executor_test.go |
| TestExec35 | Provider success does not mutate authority | executor_test.go |
| TestExec36 | Snapshot is sole source of truth | executor_test.go |
| TestExec38 | Provider success + Solvent error | executor_test.go |
| TestAT01-16 | Adversarial property violations | executor_test.go |

---

## Dependency Graph

```
Track A (adapter/github/)     Track B (kernel/)              Track C (service/)
       │                           │                              │
  fake_provider.go          sql.go (CompleteIntent)         authority.go
  provider.go               authority.go                     (signature change,
  executor.go                 (CompleteIntent +               mapping, snapshot,
       │                       GetSnapshotParams)             intent verify,
       │                   contract.go (interface)            CompleteIntent call)
       │                           │                              │
       └───────────────┬───────────┘──────────────────────────────┘
                       │
              executor_test.go
                       │
              authority_integration_test.go (updates)
```

---

## Execution Order

1. **Phase 1 (parallel):** Track A + Track B
2. **Phase 2:** Track C (service changes)
3. **Phase 3:** Update authority_integration_test.go helpers
4. **Phase 4:** Write executor_test.go (all 54 tests)
5. **Phase 5:** Verification

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

## Files Changed Summary

| File | Action | Lines (est.) |
|------|--------|-------------|
| `adapter/github/provider.go` | Create | ~20 |
| `adapter/github/fake_provider.go` | Create | ~120 |
| `adapter/github/executor.go` | Create | ~40 |
| `adapter/github/executor_test.go` | Create | ~1200 |
| `kernel/sql.go` | Modify | +5 |
| `kernel/authority.go` | Modify | +55 |
| `kernel/contract.go` | Modify | +2 |
| `service/authority/authority.go` | Modify | +80 |
| `service/authority/authority_test.go` | Modify | ~15 |
| `service/authority/authority_integration_test.go` | Modify | ~60 |

**Total:** 4 new files, 6 modified files. ~1,580 new lines, ~75 modified lines.

---

## Known v1 Limitations

1. **Window B TOCTOU race.** Revocation after final authorization check but
   before executor invocation may result in execution on stale authority.
   Bounded by local function call. Documented in TestExec15B.

2. **Concurrent duplicate execution.** Two goroutines may both pass
   authorization and both reach the provider. Intent state check prevents
   sequential duplicates but not concurrent races. Phase 4D will add
   idempotency. Documented in TestExec26.

3. **CompleteIntent persistence failure.** If CompleteIntent fails after
   provider acceptance, intent remains `live` and a subsequent execution
   may trigger a duplicate external side effect. Extremely unlikely
   (requires DB failure in same transaction window). Documented in
   Decision A.

4. **Audit gap on write failure.** If audit write fails after provider
   acceptance, execution result remains truthful but audit entry may be
   missing. Documented in TestExec34.

---

## Consistency Verification

Verified against approved test target:

- **TestExec10-13:** Kernel-level tuple binding — called directly, not via
  ExecuteAction. Implementation matches.
- **TestExec36:** Execution-path snapshot source-of-truth — service reads
  snapshot, ignores caller params, executor receives snapshot values.
  Implementation matches.
- **TestExec35:** Provider success does not mutate authority — service calls
  CompleteIntent only (no authority mutation). Implementation matches.
- **TestExec15A:** Revocation before final check must deny — PrepareForAction
  re-reads state, kernel.Authorize detects revocation. Implementation matches.
- **TestExec15B:** Deterministic Window B characterization — channel
  synchronization, not a security gate. Implementation matches.
- **TestExec27-30:** CompleteIntent semantics and provenance — intent state
  transitions, structural test for call path. Implementation matches.
- **TestExec38:** Provider succeeds but Solvent sees error — executor error
  path does not call CompleteIntent. Implementation matches.
- **TestAT01-16:** Adversarial properties — all adversarial tests exercise
  specific security controls. Implementation does not weaken any.
