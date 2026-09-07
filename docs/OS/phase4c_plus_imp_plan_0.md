# Phase 4C+ Step 1 — Implementation Plan

**Status:** Ready for execution
**Gate:** Kilo Code Gate 1 — GO (Test Engineering Gate Passed)
**Authoritative specification:** `.opencode/plans/phase4c_plus_test_target.md`

---

## Summary

Step 1 requires creating 4 new files and modifying 6 existing files. The work
decomposes into 3 parallel tracks that converge:

**Track A:** Fake provider + executor infrastructure (`adapter/github/`)
**Track B:** Kernel CompleteIntent (`kernel/`)
**Track C:** Service layer changes (`service/authority/`)

All three tracks must complete before `executor_test.go` can pass.

---

## Track A: Fake Provider + Executor (`adapter/github/`)

### File 1: `adapter/github/provider.go`

Define the `GitHubProvider` interface:

```go
type GitHubProvider interface {
    TriggerWorkflow(ctx context.Context, repo, workflow, ref string,
        inputs map[string]string) (*WorkflowResult, error)
}

type WorkflowResult struct {
    RunID string
}
```

Narrow interface. Fake and real providers both satisfy it.

### File 2: `adapter/github/fake_provider.go`

Implement `FakeGitHubProvider` exactly as specified in test target lines 200-253.

**Fields:**
- `mu sync.Mutex`
- `calls []WorkflowCall`
- `accept bool`
- `err error`
- `runID string`
- `lostResponse bool`
- `callWg sync.WaitGroup`

**Methods:**
- `NewFakeGitHubProvider(accept bool, runID string) *FakeGitHubProvider`
- `TriggerWorkflow(ctx, repo, workflow, ref, inputs) (*WorkflowResult, error)`
- `Calls() []WorkflowCall`
- `CallCount() int`
- `LastCall() *WorkflowCall`
- `SetAccept(accept bool)`
- `SetErr(err error)`
- `SetLostResponse(lost bool)`
- `WaitForCalls(n int, timeout time.Duration) error`

**Behavioral contract:**
- `accept=true`: records call, returns `&WorkflowResult{RunID: f.runID}`, nil
- `accept=false`: records call, returns `nil, f.err`
- `lostResponse=true` AND `accept=true`: records call, returns
  `nil, fmt.Errorf("response lost: provider accepted but response not received")`
- `callWg.Done()` on every call (for synchronization)

### File 3: `adapter/github/executor.go`

Executor wraps a `GitHubProvider` into an `executor.ActionFunc`:

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

**Critical:** The executor reads `repo`/`workflow`/`ref` from `params`. The
service layer puts the snapshot's values into `params`. The executor is a thin
pass-through.

---

## Track B: Kernel CompleteIntent (`kernel/`)

### File 4: `kernel/sql.go` — add `sqlCompleteIntent`

```sql
UPDATE action_intent SET state = 'executed'
WHERE id = $1::UUID AND scenario_id = $2::UUID AND state = 'live'
```

Only transitions `live` → `executed`. Idempotent (no-op if already executed).

### File 5: `kernel/authority.go` — add `CompleteIntent` method

```go
func (s *Store) CompleteIntent(ctx context.Context, scenarioID, intentID string) error {
    return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        _, err := tx.ExecContext(ctx, sqlCompleteIntent, intentID, scenarioID)
        return err
    })
}
```

### File 6: `kernel/contract.go` — add to `Contract` interface

```go
CompleteIntent(context.Context, string, string) error
```

Compile-time assertion `var _ Contract = (*Store)(nil)` already exists and will
verify.

### File 7: `kernel/authority.go` — add `GetSnapshotConsequenceParams`

```go
func (s *Store) GetSnapshotConsequenceParams(ctx context.Context, targetID string) (map[string]interface{}, error) {
    var paramsJSON []byte
    err := s.db.QueryRowContext(ctx, `
        SELECT ts.consequence_parameters
        FROM target_activation ta
        JOIN target_snapshot ts ON ts.target_id = ta.target_id
            AND ts.snapshot_id = ta.snapshot_id
        WHERE ta.target_id = $1::UUID
          AND NOT EXISTS (
            SELECT 1 FROM target_revocation WHERE target_id = $1::UUID
          )`, targetID).Scan(&paramsJSON)
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

---

## Track C: Service Layer Changes (`service/authority/`)

### File 8: `service/authority/authority.go` — three changes

#### Change 1: Action→executor mapping (replace lines 196-208)

```go
var actionExecutorMap = map[string]string{
    "deploy": executor.ExecutorName,
}

func resolveExecutor(action string) (string, bool) {
    name, ok := actionExecutorMap[action]
    return name, ok
}
```

In `ExecuteAction`, replace:
```go
toolName, _ := params["tool_name"].(string)
fn, ok := s.execReg.Get(toolName)
```

With:
```go
execName, ok := resolveExecutor(action)
if !ok {
    result.Success = false
    result.Error = fmt.Sprintf("no executor registered for action: %s", action)
    return result, nil
}
fn, ok := s.execReg.Get(execName)
```

#### Change 2: Snapshot parameter reconstruction (after PrepareForAction)

After authorization succeeds, read snapshot and reconstruct params:

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

Then call `fn(ctx, execParams)` instead of `fn(ctx, params)`.

#### Change 3: CompleteIntent after provider acceptance

After executor returns nil error:

```go
var intentID string
err = s.db.QueryRowContext(ctx, `
    SELECT id FROM action_intent
    WHERE scenario_id = $1::UUID AND belief_id = $2::UUID
      AND action = $3 AND state = 'live'
    LIMIT 1`, scenarioID, beliefID, action).Scan(&intentID)
if err == nil {
    _ = s.kern.CompleteIntent(ctx, scenarioID, intentID)
}
```

CompleteIntent failure is logged but does not fail the execution result.

---

## Test Infrastructure Changes

### File 9: `service/authority/authority_test.go`

Add `GetSnapshotConsequenceParams` to `mockKernelStore` if needed. Most tests
are skipped — minimal changes.

### File 10: `service/authority/authority_integration_test.go`

#### Helper changes:

1. `testConsequenceParams()` → GitHub params:
```go
func testConsequenceParams() []byte {
    p, _ := json.Marshal(map[string]string{
        "repo": "org/test", "workflow": "deploy.yml", "ref": "main",
    })
    return p
}
```

2. `createApprovedTarget()` → add `IntentOnPromoted` at end:
```go
if err := st.IntentOnPromoted(ctx, scenarioID, beliefID, action); err != nil {
    t.Fatalf("setup (create intent): %v", err)
}
```

3. `newTestService()` → accept `GitHubProvider`, register executor:
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

#### Test changes:

- Remove `tool_name` from all `params` maps
- Update `newTestService` calls to pass `FakeGitHubProvider`
- Replace `rec.Called()` with `provider.CallCount()`
- TestP01 uses `fp := github.NewFakeGitHubProvider(true, "run-123")`

---

## File 11: `adapter/github/executor_test.go` — Test Matrix

### Test categories and requirements

| Category | Tests | Needs DB | Key assertion |
|----------|-------|----------|---------------|
| Authorization (12.1) | TestExec01-09 | Yes | Allowed/Denied + provider call count |
| Parameter binding (12.2) | TestExec10-14, TestExec36 | 10-13: Yes (kernel only); 14,36: Yes | Tuple mismatch / snapshot params |
| TOCTOU (12.3) | TestExec15A, 15B, 16-19 | Yes | Race synchronization |
| Provider failure (12.4) | TestExec20-24 | Yes | Success/Error + provider call count |
| Duplicate execution (12.5) | TestExec25-26 | Yes | Intent state |
| CompleteIntent (12.6) | TestExec27-30 | 27-29: Yes; 30: No (grep) | Intent state transition |
| Audit (12.7) | TestExec31-35, 38 | Yes | Audit entry ordering/types |
| Adversarial (12.8) | TestAT01-16 | Yes (most) | Security property violation |

### Test helpers in executor_test.go

- `TestMain` — database setup (same pattern as authority_integration_test.go)
- `newTestService` — create Service with FakeGitHubProvider
- `createPrincipal`, `createAndPromoteBelief` — same as authority helpers
- `createGitHubApprovedTarget` — with `{"repo":"org/test","workflow":"deploy.yml","ref":"main"}` + `IntentOnPromoted`
- `readIntentState` — query `action_intent.state`
- `readAuditEntries` — query audit log
- `readAuthorityState` — capture pre-execution state (for TestExec35)

### Key test implementations

**TestExec10-13 (kernel-level):** Call `kernel.Authorize` directly with
substituted tuples. Assert `Allowed == false`.

**TestExec30 (structural):** Grep/AST analysis. Verify CompleteIntent is on
Contract interface, not in REST/MCP handlers.

**TestExec15B (TOCTOU):** Channel synchronization:
1. Goroutine A: passes T2, signals `afterT2`, waits on `revoked`, invokes executor
2. Goroutine B: waits on `afterT2`, revokes via separate DB connection, signals `revoked`
3. Assert: `Allowed == true`, provider call count == 1

**TestExec35 (authority state):** Capture pre-execution state of target,
activation, snapshot, belief, principal rows. Execute. Re-query. Assert
byte-for-byte equality (except intent state).

**TestExec36 (snapshot binding):** Three variants:
- a) matching + extra fields → success, executor receives only snapshot fields
- b) conflicting values → success, executor receives snapshot values
- c) extra provider-looking fields → success, executor receives only snapshot fields

---

## Dependency Graph

```
Track A (adapter/github/)     Track B (kernel/)         Track C (service/)
       │                           │                          │
  fake_provider.go          sql.go (CompleteIntent)    authority.go
  provider.go               authority.go (CompleteIntent)  (mapping + snapshot
  executor.go               contract.go (interface)         + CompleteIntent)
       │                           │                          │
       └───────────────┬───────────┘──────────────────────────┘
                       │
              executor_test.go
                       │
              authority_integration_test.go (updates)
```

Track A and Track B are independent. Track C depends on both. Test files depend
on all three tracks.

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
| `kernel/authority.go` | Modify | +40 |
| `kernel/contract.go` | Modify | +2 |
| `service/authority/authority.go` | Modify | +60 |
| `service/authority/authority_test.go` | Modify | ~10 |
| `service/authority/authority_integration_test.go` | Modify | ~50 |

**Total:** 4 new files, 6 modified files. ~1,540 new lines, ~60 modified lines.

---

## Security Properties Demonstrated

1. Authorization precedes external execution (TestExec01-06)
2. Executor derived from action, not caller params (TestExec09, TestAT05)
3. Snapshot is sole source of execution params (TestExec36)
4. Revocation before final check prevents execution (TestExec08, TestExec15A)
5. Window B is documented v1 limitation (TestExec15B)
6. `executed` means provider acceptance (TestExec27-29)
7. CompleteIntent not callable from public path (TestExec30)
8. Provider success does not create authority (TestExec35)
9. Audit facts are distinct and ordered (TestExec31-34)
10. Duplicate execution prevented sequentially (TestExec25)

---

## Known v1 Limitations

- Window B TOCTOU race (documented, bounded by local function call)
- Concurrent duplicate execution (TestExec26)
- Audit gap on write failure (TestExec34)
- CompleteIntent failure is best-effort (not transactional with provider call)
