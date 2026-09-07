# PHASE 4C+ — N-1 / N-2 Implementation Plan

**Status:** Ready for execution
**Scope:** Close final two MEDIUM review findings (N-1, N-2)
**Constraint:** No architecture redesign, no new kernel primitives, no schema changes

---

## N-1: TestExec34 — Deterministic Audit Failure Injection

### Problem
`TestExec34_AuditWriteFailureAfterProviderSuccess` (line 1474) is a stub that logs messages. It does not exercise the failure scenario.

### Audit call order in ExecuteAction

```
1. authorization_granted      (line 148, inside PrepareForAction)
2. executor_denied            (line 199, if !decision.Allowed — not reached in happy path)
3. adapter_invoked            (line 260)
4. executor                   (line 273)
5. executor_failed            (line 279, if exec error — not reached in happy path)
6. intent_completion_failed   (line 301, if CompleteIntent error)
7. executor_completed         (line 318, on full success)
```

For the happy path (provider accepts, CompleteIntent succeeds), the ordering is:
```
authorization_granted → adapter_invoked → executor_completed
```

### Implementation

#### Step 1: Add `auditLogger` interface (`service/authority/authority.go`)

```go
type auditLogger interface {
    Log(ctx context.Context, entry *audit.ActivityEntry) error
}
```

Matches `(*audit.Service).Log` signature exactly.

#### Step 2: Change `Service.audit` field type

```go
type Service struct {
    db           *sql.DB
    kern         *kernel.Store
    policy       *policy.Service
    audit        auditLogger    // was: *audit.Service
    execReg      *executor.Registry
    beforeExecute func()        // N-2: test hook, nil in production
}
```

#### Step 3: Update `New` constructor parameter

```go
func New(db *sql.DB, pol *policy.Service, aud auditLogger, reg *executor.Registry) *Service {
```

`*audit.Service` already satisfies `auditLogger`. All callers pass `*audit.Service`.

#### Step 4: Add `SetTestHook` (unexported setter per reviewer guidance)

```go
type testHook func()

func (s *Service) SetTestHook(hook testHook) {
    s.beforeExecute = hook
}
```

#### Step 5: Insert hook call in `ExecuteAction` (line 270)

```go
// After adapter_invoked log, before executor invocation:
if s.beforeExecute != nil {
    s.beforeExecute()
}

// 7. Execute.
output, execErr := fn(ctx, execParams)
```

#### Step 6: Add `failingAuditLogger` test double

```go
type failingAuditLogger struct {
    mu      sync.Mutex
    failOn  audit.ActivityType
    entries []*audit.ActivityEntry
    failErr error
}

func (f *failingAuditLogger) Log(ctx context.Context, entry *audit.ActivityEntry) error {
    f.mu.Lock()
    defer f.mu.Unlock()
    if entry.Type == f.failOn {
        return f.failErr
    }
    f.entries = append(f.entries, entry)
    return nil
}
```

#### Step 7: Add `newTestServiceWithAudit` helper

```go
func newTestServiceWithAudit(t *testing.T, provider *FakeGitHubProvider, aud auditLogger) *authority.Service {
    t.Helper()
    pol := policy.New(shared)
    reg := executor.NewRegistry()
    RegisterExecutor(reg, provider)
    return authority.New(shared, pol, aud, reg)
}
```

#### Step 8: Rewrite `TestExec34`

```go
func TestExec34_AuditWriteFailureAfterProviderSuccess(t *testing.T) {
    ctx := context.Background()
    st := kernel.New(shared)
    provider := NewFakeGitHubProvider(true, "run-34")
    aud := &failingAuditLogger{
        failOn:  audit.ActivityExecutorCompleted,
        failErr: fmt.Errorf("simulated audit write failure"),
    }
    svc := newTestServiceWithAudit(t, provider, aud)

    sid := testScenario(34)
    principalID := createPrincipal(t, ctx, st, "agent", "exec34-issuer")
    beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
    targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
    intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

    result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
        intentID, map[string]interface{}{}, "execution", testConsequenceParams())
    if err != nil {
        t.Fatalf("ExecuteAction returned error: %v", err)
    }

    // Provider was called once.
    if provider.CallCount() != 1 {
        t.Errorf("expected 1 provider call, got %d", provider.CallCount())
    }

    // Execution result remains truthful despite audit failure.
    if !result.Allowed {
        t.Errorf("expected Allowed=true")
    }
    if !result.Success {
        t.Errorf("expected Success=true (provider accepted, audit failure must not rewrite)")
    }

    // Verify audit entries persisted before the failure point.
    aud.mu.Lock()
    defer aud.mu.Unlock()
    var hasGrant, hasInvoke bool
    for _, e := range aud.entries {
        switch e.Type {
        case audit.ActivityAuthorizationGranted:
            hasGrant = true
        case audit.ActivityAdapterInvoked:
            hasInvoke = true
        }
    }
    if !hasGrant {
        t.Errorf("expected authorization_granted entry")
    }
    if !hasInvoke {
        t.Errorf("expected adapter_invoked entry")
    }
    // executor_completed never persisted (failOn intercepted it).
    for _, e := range aud.entries {
        if e.Type == audit.ActivityExecutorCompleted {
            t.Errorf("executor_completed must not be persisted when audit write fails")
        }
    }
}
```

**Assertions:**
- `provider.CallCount() == 1`
- `result.Allowed == true`
- `result.Success == true`
- `authorization_granted` present in audit entries
- `adapter_invoked` present in audit entries
- `executor_completed` absent from audit entries

---

## N-2: TestExec15B — Deterministic Window B Characterization

### Problem
`TestExec15B_RevocationAfterCheck_DocumentedRace` (line 729) performs normal execution without any revocation. It has no adversarial value.

### Window B location

```
PrepareForAction (kernel.Authorize)     ← T2
    ↓
intent-state SELECT                     ← step 4
    ↓
param reconstruction                    ← step 5
    ↓
adapter_invoked log                     ← step 6
    ↓
    ╔══ beforeExecute hook ══╗          ← deterministic pause
    ║  (test revokes here)   ║
    ╚════════════════════════╝
    ↓
executor invocation                     ← T3
    ↓
CompleteIntent
    ↓
executor_completed log
```

### Implementation

Uses the same `beforeExecute` hook from N-1 (Step 4-5 above).

#### Rewrite `TestExec15B`

```go
func TestExec15B_RevocationAfterCheck_DocumentedRace(t *testing.T) {
    ctx := context.Background()
    st := kernel.New(shared)
    provider := NewFakeGitHubProvider(true, "run-15b")

    // Channel synchronization: deterministic, no sleep.
    authorizeDone := make(chan struct{})
    proceed := make(chan struct{})

    svc := newTestService(t, provider)
    svc.SetTestHook(func() {
        close(authorizeDone)  // signal: authorization + state verification passed
        <-proceed            // block: wait for test to inject revocation
    })

    sid := testScenario(151)
    principalID := createPrincipal(t, ctx, st, "agent", "exec15b-issuer")
    beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
    targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
    intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

    // Start ExecuteAction — blocks at hook.
    var execDone sync.WaitGroup
    execDone.Add(1)
    var result *authority.ExecutionResult
    var execErr error
    go func() {
        defer execDone.Done()
        result, execErr = svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
            intentID, map[string]interface{}{}, "execution", testConsequenceParams())
    }()

    // Wait for T2 (authorization) to complete.
    <-authorizeDone

    // Window B: revocation AFTER T2, BEFORE executor invocation.
    retracted, retractErr := st.RetractCascade(ctx, sid, beliefID)
    if retractErr != nil {
        t.Fatalf("RetractCascade failed: %v", retractErr)
    }
    if retracted == 0 {
        t.Fatal("expected at least one belief retracted")
    }

    // Release execution — executor runs despite revoked belief.
    close(proceed)
    execDone.Wait()

    if execErr != nil {
        t.Fatalf("ExecuteAction returned error: %v", execErr)
    }

    // Window B v1 behavior: execution proceeds.
    if provider.CallCount() != 1 {
        t.Errorf("expected 1 provider call (execution proceeded despite revocation), got %d", provider.CallCount())
    }
    if !result.Allowed {
        t.Errorf("expected Allowed=true (authorization was already granted before revocation)")
    }
    if !result.Success {
        t.Errorf("expected Success=true (executor ran in Window B)")
    }

    // Verify revocation was real.
    // After RetractCascade, the belief status is no longer "promoted".
    var status string
    err := shared.QueryRowContext(ctx,
        `SELECT status FROM belief WHERE scenario_id = $1::UUID AND id = $2::UUID`,
        sid, beliefID).Scan(&status)
    if err != nil {
        t.Fatalf("belief status query failed: %v", err)
    }
    if status == "promoted" {
        t.Errorf("belief should not be promoted after RetractCascade, got %q", status)
    }
}
```

**Assertions:**
- `provider.CallCount() == 1` (execution proceeded despite revocation)
- `result.Allowed == true` (authorization was already granted before T3)
- `result.Success == true` (executor ran in Window B)
- Belief status is no longer "promoted" (revocation was real)
- Provider invoked exactly once, after revocation

---

## Files Modified

| File | Changes |
|------|---------|
| `service/authority/authority.go` | Add `auditLogger` interface, `testHook` type, `beforeExecute` field, `SetTestHook` method, update `Service.audit` type, update `New` signature, insert hook call |
| `adapter/github/executor_test.go` | Add `failingAuditLogger`, `newTestServiceWithAudit`, rewrite `TestExec34`, rewrite `TestExec15B` |

## Verification

```bash
go build ./...
go vet ./...
go test -count=1 ./...
go test -race ./...
go test -v ./adapter/github/... -run "TestExec34|TestExec15B"
```

## Architectural Preservation Checklist

- [x] No new kernel primitive
- [x] No schema change
- [x] No new service
- [x] No TOCTOU guarantee strengthened
- [x] No deferred feature introduced
- [x] `kernel.Authorize` remains sole authority oracle
- [x] `CompleteIntent` occurs only after provider acceptance
- [x] CI-2 remains authoritative (`Success = true` iff executor returned nil)
- [x] Window B remains documented v1 limitation
- [x] `beforeExecute` hook is nil in production, set only in tests
- [x] `auditLogger` interface is unexported (test seam, not production API)
- [x] `*audit.Service` satisfies `auditLogger` — no production callers change
