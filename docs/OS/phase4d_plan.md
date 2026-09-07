# Phase 4D — Idempotent Consequential Execution + API Completion

**Status:** Implementation plan (no production code changes yet)
**Depends on:** Phase 4C+ (GO)
**Scope:** Close accepted v1 limitations: concurrent duplicate execution, post-acceptance persistence failure, consequence parameter mismatch, HTTP timeout

---

## 1. Objective

Make one logical authorized intent result in **at most one accepted external execution request** under Solvent-controlled concurrency, without claiming impossible distributed atomicity with external providers.

Phase 4C+ proved the authority-to-real-side-effect path. Phase 4D solves the most important operational weakness: the gap between an external side effect and Solvent's ability to know whether it has already happened.

---

## 2. Current v1 Failure Model

| Limitation | Status | Phase 4D Target |
|---|---|---|
| Concurrent duplicate execution | Known v1 | **Closed** |
| CompleteIntent persistence failure after provider acceptance | Known v1 | **Closed** |
| REST/MCP ConsequenceParameters = `{}` | Known v1 | **Closed** |
| HTTP provider has no timeout | Known v1 | **Closed** |
| SetTestHook in production code | Known v1 | **Closed** (removed) |
| Ambiguous provider result after crash | Accepted | **Characterized** (no auto-retry) |

---

## 3. Proposed Idempotency Model

### Core property

```
one logical authorized intent
    =>
at most one accepted external execution request
    under Solvent-controlled concurrency
```

### What this means

- Two concurrent `ExecuteAction` calls for the same intent: exactly one proceeds to the provider, the other is refused.
- Process crash after provider acceptance but before `CompleteIntent`: intent stays `executing`. No automatic retry. Reconciliation required.
- Process crash before provider invocation: intent stays `live`. Retry allowed (no side effect occurred).
- Provider rejection: intent stays `live`. Retry allowed.

### What this does NOT mean

- **Not exactly-once external execution.** GitHub does not guarantee idempotency on `workflow_dispatch`. A crash after provider acceptance may result in a duplicate workflow that Solvent cannot detect locally.
- **Not atomic coordination with the provider.** The database transaction and the HTTP call are separate operations.

---

## 4. Execution State Machine

### States

```
live → executing → executed
          ↓
      recovery / reconciliation
```

| State | Meaning | Belief constraint |
|---|---|---|
| `live` | Intent is active; belief must be promoted | `live_requires_promoted` CHECK enforced |
| `executing` | Solvent has claimed the intent for execution; provider call is in progress or ambiguous | No belief constraint (survives retraction) |
| `executed` | Provider accepted AND `CompleteIntent` succeeded | Terminal |
| `cancelled` | Intent was cancelled (retraction cascade) | Terminal |

### Transitions

| From | To | Trigger | Mechanism |
|---|---|---|---|
| `live` | `executing` | `ClaimIntent` (atomic CAS) | `UPDATE ... SET state = 'executing' WHERE id = $1 AND state = 'live'` |
| `executing` | `executed` | `CompleteIntent` (after provider acceptance) | `UPDATE ... SET state = 'executed' WHERE id = $1 AND state = 'executing'` |
| `executing` | `live` | Executor error (provider rejection) | `UPDATE ... SET state = 'live' WHERE id = $1 AND state = 'executing'` |
| `live` | `cancelled` | RetractCascade | `UPDATE ... SET state = 'cancelled' WHERE state = 'live' AND belief_id IN (...)` |
| `executing` | `cancelled` | RetractCascade | **New:** `UPDATE ... SET state = 'cancelled' WHERE state = 'executing' AND belief_id IN (...)` |

### Crash recovery semantics

| Crash point | Intent state | Provider state | Retry allowed? |
|---|---|---|---|
| Before `ClaimIntent` | `live` | Not invoked | Yes |
| After `ClaimIntent`, before provider call | `executing` | Not invoked | **No** — reconcile first |
| After provider call, before `CompleteIntent` | `executing` | Ambiguous | **No** — reconcile first |
| After `CompleteIntent` | `executed` | Accepted | N/A (already complete) |

The key insight: an `executing` intent after crash is **ambiguous**. We cannot know whether the provider received the request. Automatic rollback to `live` would risk duplicate execution. The honest position is: no retry until external state is reconciled.

---

## 5. Concurrency Control Mechanism

### Approach: Atomic CAS on intent state

The intent-state verification step in `ExecuteAction` (current step 4) is replaced with an atomic `ClaimIntent` operation:

```sql
UPDATE action_intent
SET state = 'executing'
WHERE id = $1::UUID
  AND scenario_id = $2::UUID
  AND belief_id = $3::UUID
  AND action = $4
  AND state = 'live'
RETURNING id
```

**Properties:**
- `RowsAffected == 1`: intent claimed, proceed to provider
- `RowsAffected == 0`: intent already claimed (executing/executed/cancelled), refuse
- Executed within a `crdb.ExecuteTx` transaction for CockroachDB retry safety
- Atomic: two concurrent CAS attempts — exactly one succeeds

### Why this works

1. **Concurrent duplicates:** Two goroutines both attempt CAS. One gets `RowsAffected=1`, the other gets `RowsAffected=0`. The loser is refused.
2. **Sequential duplicates:** Intent is already `executed`. CAS finds `state != 'live'`, refuses.
3. **Crash before provider:** Intent is `executing`. CAS refuses any retry. Reconciliation needed.
4. **Crash after provider:** Intent is `executing`. Same as above — honest ambiguity.

### Why not row locking alone

`SELECT ... FOR UPDATE` prevents concurrent reads but does not persist state across process crashes. If the process dies after the lock is acquired but before the provider call, the lock is released and the intent remains `live` — a duplicate execution window.

The `executing` state is durable and survives crashes. It is the minimum necessary state to prevent duplicates under the most common failure modes.

---

## 6. Kernel Changes

### 6a. New `ClaimIntent` method

```go
// ClaimIntent transitions a live intent to 'executing'. It is atomic:
// only one caller can successfully claim a given intent.
//
// Returns nil on success (state is now 'executing').
// Returns ErrIntentNotLive if the intent is not in 'live' state.
func (s *Store) ClaimIntent(ctx context.Context, scenarioID, intentID string) error
```

Implementation:
```go
func (s *Store) ClaimIntent(ctx context.Context, scenarioID, intentID string) error {
    return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        result, err := tx.ExecContext(ctx, sqlClaimIntent, intentID, scenarioID)
        if err != nil {
            return err
        }
        n, err := result.RowsAffected()
        if err != nil {
            return err
        }
        if n == 0 {
            return ErrIntentNotLive
        }
        return nil
    })
}
```

New SQL constant in `kernel/sql.go`:
```go
sqlClaimIntent = `
    UPDATE action_intent SET state = 'executing'
    WHERE id = $1::UUID AND scenario_id = $2::UUID AND state = 'live'`
```

New sentinel error in `kernel/errors.go`:
```go
var ErrIntentNotLive = errors.New("intent is not in live state")
```

### 6b. Update `CompleteIntent` SQL

Change `WHERE state = 'live'` to `WHERE state = 'executing'`:

```go
sqlCompleteIntent = `
    UPDATE action_intent SET state = 'executed'
    WHERE id = $1::UUID AND scenario_id = $2::UUID AND state = 'executing'`
```

This is safe because `ClaimIntent` already transitioned the intent to `executing`. If `CompleteIntent` is called on a non-executing intent, it affects zero rows (idempotent).

### 6c. New `RollbackClaim` method (for executor error)

```go
// RollbackClaim transitions an executing intent back to 'live' after
// executor failure. Only valid when state is 'executing'.
func (s *Store) RollbackClaim(ctx context.Context, scenarioID, intentID string) error
```

New SQL constant:
```go
sqlRollbackClaim = `
    UPDATE action_intent SET state = 'live'
    WHERE id = $1::UUID AND scenario_id = $2::UUID AND state = 'executing'`
```

### 6d. Update `RetractCascade` to handle `executing` intents

The cancel step in `RetractCascade` currently only targets `WHERE state = 'live'`. Phase 4D adds a second cancel pass for `executing` intents:

```sql
UPDATE action_intent SET state = 'cancelled'
WHERE state = 'executing'
  AND scenario_id = $2::UUID
  AND belief_id IN (SELECT id FROM d)
```

**Design question for review:** Should `RetractCascade` cancel `executing` intents? The `executing` state was introduced to prevent duplicate execution. If we cancel an `executing` intent during retraction, we lose the "attempt in progress" marker. However, if the belief is retracted, the authority foundation is gone — executing an intent based on a retracted belief would be incorrect.

**Decision:** Cancel `executing` intents during `RetractCascade`. The retraction is a higher-priority authority change. If the provider was already invoked, the external side effect already happened — cancelling the intent in Solvent does not undo it, but it prevents future Solvent-driven execution on a retracted belief.

---

## 7. Service Layer Changes

### 7a. `ExecuteAction` — new step ordering

```
1. PrepareForAction (kernel.Authorize)           — unchanged
2. If not allowed: refuse, return                 — unchanged
3. Resolve executor                               — unchanged
4. ClaimIntent (atomic CAS live→executing)        — NEW (replaces read-only state check)
5. Reconstruct execution params from snapshot      — unchanged
6. Log adapter_invoked                            — unchanged
7. Execute provider                               — unchanged
8a. On provider error:
        RollbackClaim (executing→live)
        Log executor_failed
        Return result
8b. On provider success:
        CompleteIntent (executing→executed)       — updated WHERE clause
        Log executor_completed
        Return result
```

### 7b. Service layer also needs a `ReconcileIntent` method

For operator use after crash recovery:

```go
// ReconcileIntent resolves an ambiguous 'executing' intent.
// The caller must determine the provider state externally before calling.
func (s *Service) ReconcileIntent(ctx context.Context, scenarioID, intentID string, outcome IntentOutcome) error
```

Where `IntentOutcome` is:
```go
type IntentOutcome int
const (
    IntentOutcomeCompleted IntentOutcome = iota  // provider accepted, transition to 'executed'
    IntentOutcomeFailed                          // provider rejected, transition to 'live'
    IntentOutcomeCancelled                       // intent should be cancelled
)
```

This is a service-layer method, not a kernel primitive, because reconciliation is execution coordination, not authority semantics.

---

## 8. API/MCP Consequence Parameters

### Problem

`handleAuthorizeAction` (REST) and `handleSolventAuthorizeAction` (MCP) hardcode `ConsequenceParameters: []byte("{}")`. Targets with real parameters (e.g., GitHub `repo`, `workflow`, `ref`) cannot be authorized through these surfaces.

### Solution: Read from target snapshot

The authorization endpoint reads the target's approved snapshot `consequence_parameters` and uses those as the tuple's `ConsequenceParameters`. The caller does not supply them.

### REST changes

**`api/authorization.go` `handleAuthorizeAction`:**

Replace the hardcoded `[]byte("{}")` with a snapshot read:

```go
// Read the target's approved snapshot consequence_parameters.
var snapParams []byte
err := s.db.QueryRowContext(ctx, `
    SELECT ts.consequence_parameters
    FROM target_activation ta
    JOIN target_snapshot ts ON ts.target_id = ta.target_id AND ts.snapshot_id = ta.snapshot_id
    WHERE ta.target_id = $1::UUID`, req.TargetID).Scan(&snapParams)
if err != nil {
    // target not activated or snapshot missing
    writeError(w, http.StatusBadRequest, "target not activated or snapshot missing")
    return
}

tuple := kernel.AuthorityTuple{
    // ... existing fields ...
    ConsequenceParameters: snapParams,  // from approved snapshot
}
```

The `AuthorizeActionRequest` type does NOT gain a `ConsequenceParameters` field. The approved snapshot is the sole source.

### MCP changes

**`cmd/solvent-mcp/tools.go` `handleSolventAuthorizeAction`:**

Same pattern — read from snapshot instead of hardcoding `{}`:

```go
var snapParams []byte
err = db.QueryRowContext(ctx, `
    SELECT ts.consequence_parameters
    FROM target_activation ta
    JOIN target_snapshot ts ON ts.target_id = ta.target_id AND ts.snapshot_id = ta.snapshot_id
    WHERE ta.target_id = $1::UUID`, targetID).Scan(&snapParams)
if err != nil {
    return errorResult(fmt.Errorf("target not activated: %v", err)), nil
}

tuple := kernel.AuthorityTuple{
    // ... existing fields ...
    ConsequenceParameters: snapParams,  // from approved snapshot
}
```

### Why this is safe

- The snapshot is immutable after creation (INSERT-only, no UPDATE)
- The snapshot was created during `Approve` with the exact parameters the approver authorized
- `authorizeWithinTx` already compares `tuple.ConsequenceParameters` against the snapshot via `jsonEqual` — this check is now trivially satisfied (same data), which is correct
- No caller-controlled parameter substitution is possible
- The kernel's authority semantics are unchanged

---

## 9. HTTP Provider Hardening

### 9a. Configurable timeout

**`adapter/github/http_provider.go`:**

```go
type HTTPProvider struct {
    token      string
    httpClient *http.Client
}

type HTTPProviderOption func(*HTTPProvider)

func WithTimeout(d time.Duration) HTTPProviderOption {
    return func(p *HTTPProvider) {
        p.httpClient.Timeout = d
    }
}

func NewHTTPProvider(token string, opts ...HTTPProviderOption) *HTTPProvider {
    p := &HTTPProvider{
        token:      token,
        httpClient: &http.Client{Timeout: 30 * time.Second},  // default
    }
    for _, opt := range opts {
        opt(p)
    }
    return p
}
```

Context cancellation remains supported. The client timeout is a safety bound, not the primary mechanism.

### 9b. Error response truncation

GitHub error responses can be large (rate limit pages, HTML error pages). Truncate before propagation into audit-visible errors:

```go
const maxErrorBodyLen = 1024

func truncateError(body []byte) string {
    if len(body) > maxErrorBodyLen {
        return string(body[:maxErrorBodyLen]) + "... (truncated)"
    }
    return string(body)
}
```

### 9c. Wrap API errors with `%w`

Currently `http_provider.go` line 103 uses `fmt.Errorf("GitHub API returned %d: %s", ...)` without `%w`. Add `%w` wrapping with a sentinel for API errors to enable `errors.Is` classification:

```go
var ErrGitHubAPI = errors.New("GitHub API error")

// In error path:
return nil, fmt.Errorf("%w %d: %s", ErrGitHubAPI, resp.StatusCode, truncateError(respBody))
```

---

## 10. Remove `SetTestHook` from Production

### Current state

`SetTestHook` is an exported method on `authority.Service` that installs a `beforeExecute` callback. The callback fires between adapter_invoked audit and executor invocation. The hook type is unexported, but the method is exported.

### Problem

A production mutable hook on a security-sensitive service. Even though the hook type is unexported, any code holding the service reference can install arbitrary behavior before consequential execution.

### Solution: Remove from production, use executor/provider test double

Remove from `service/authority/authority.go`:
- `testHook` type
- `beforeExecute` field on `Service`
- `SetTestHook` method
- The hook call in `ExecuteAction` (lines 290-293)

Replace with a test-only synchronization seam through the executor infrastructure. The `FakeGitHubProvider` already has synchronization primitives (`callWg`, `WaitForCalls`). Extend this pattern:

**New test helper in `adapter/github/executor_test.go`:**

```go
// syncExecutor is a test double that signals a channel before calling the provider,
// allowing the test to inject state changes between authorization and execution.
type syncExecutor struct {
    provider *FakeGitHubProvider
    before   chan struct{}  // test signals when to proceed
    signal   chan struct{}  // executor signals when it's ready
}

func (e *syncExecutor) Func() executor.ActionFunc {
    return func(ctx context.Context, params map[string]interface{}) (string, error) {
        close(e.signal)      // signal: executor is about to run
        <-e.before           // block: wait for test to inject changes
        return e.provider.TriggerWorkflow(ctx, params["repo"].(string), params["workflow"].(string), params["ref"].(string), nil)
    }
}
```

**Rewritten TestExec15B:**

```go
func TestExec15B_RevocationAfterCheck_DocumentedRace(t *testing.T) {
    ctx := context.Background()
    st := kernel.New(shared)

    sync := &syncExecutor{
        provider: NewFakeGitHubProvider(true, "run-15b"),
        before:   make(chan struct{}),
        signal:   make(chan struct{}),
    }
    svc := newTestServiceWithRecording(t, sync)  // registers sync.Func() as executor

    sid := testScenario(151)
    // ... setup ...

    go func() {
        result, execErr = svc.ExecuteAction(...)
    }()

    <-sync.signal    // wait for executor to be invoked

    // Window B: revocation AFTER authorization, BEFORE provider call
    st.RevokeTarget(ctx, targetID, principalID, "window B test")

    close(sync.before)  // release provider
    execDone.Wait()

    // Assert: provider called, Allowed=true, Success=true
}
```

**No production code changes needed** — the synchronization lives entirely in the test executor infrastructure.

---

## 11. Schema Changes

### New migration file: `db/008_executing_state.sql`

```sql
-- Phase 4D: Add 'executing' state to action_intent for idempotent execution.
-- Idempotent DDL: safe to apply multiple times.

-- Update the state CHECK constraint.
ALTER TABLE action_intent
  DROP CONSTRAINT IF EXISTS action_intent_state_check;

ALTER TABLE action_intent
  ADD CONSTRAINT action_intent_state_check
  CHECK (state IN ('live','cancelled','executing','executed'));

-- Index for finding executing intents (reconciliation queries).
CREATE INDEX IF NOT EXISTS executing_intents
  ON action_intent (scenario_id) WHERE state = 'executing';
```

### Migration consumers to update

| File | Change |
|---|---|
| `internal/testdb/testdb.go` schemaPaths | Add `../../db/008_executing_state.sql` |
| `adapter/github/executor_test.go` schemaPaths | Add `../../db/008_executing_state.sql` |
| `demo/cloud/init/main.go` | Add file 008 to the apply list |
| `internal/m0/schema.go` | **No change** — M0 is frozen; table count unchanged, constraint name unchanged |

---

## 12. Proof Updates

### `proof/02_lifecycle_and_invariants.sql`

Add invariant **I-7**: Executing intent survives retraction (cancelled, not left executing):

```sql
-- I-7: RetractCascade cancels executing intents
-- Create intent, claim it (executing), then retract the belief.
-- The intent must be cancelled, not left in 'executing' state.
```

### `internal/m0/gate.go`

**No changes** — M0 is frozen. The D-series probes test `live` and `cancelled` states, which remain valid. The new `executing` state does not affect existing probes.

### `proof/harness/cells.sh` and `proof/harness/cells_crdb.sh`

Update the naive_intent strawman CHECK constraint to include `executing` if the harnesses are to remain in sync.

---

## 13. Test Strategy

### Acceptance tests

| Test | What it proves |
|---|---|
| `TestExec39_ConcurrentDuplicatePrevention` | Two goroutines call ExecuteAction for the same intent concurrently — exactly one reaches the provider |
| `TestExec40_SequentialDuplicatePrevention` | ExecuteAction on an already-executed intent is refused |
| `TestExec41_ClaimIntentAtomicity` | ClaimIntent CAS succeeds exactly once for a given intent |
| `TestExec42_ExecutorErrorRollbackToLive` | Provider rejection rolls back executing→live, allowing retry |
| `TestExec43_CrashRecoveryNoAutoRetry` | After ClaimIntent, simulated crash leaves intent in `executing` — retry refused |
| `TestExec44_RetractCascadeCancelsExecuting` | RetractCascade cancels an `executing` intent |
| `TestExec45_ConsequenceParametersFromSnapshot` | REST/MCP authorization reads consequence_parameters from approved snapshot |
| `TestExec46_HTTPProviderTimeout` | HTTPProvider respects configurable timeout |
| `TestExec47_NoSetTestHookInProduction` | authority.Service does not expose SetTestHook |

### Adversarial tests

| Test | Attack | Expected |
|---|---|---|
| `TestExec48_Adv_ConcurrentRace` | 10 goroutines race on same intent | Exactly 1 succeeds to provider |
| `TestExec49_Adv_RetryAfterExecuting` | Retry ExecuteAction while intent is `executing` | Refused |
| `TestExec50_Adv_RetractDuringExecution` | RetractCascade during `executing` state | Intent cancelled, provider result is truth |
| `TestExec51_Adv_ParamSubstitution` | Caller tries to supply different consequence_parameters | Rejected (snapshot is source) |
| `TestExec52_Adv_ExecuteAfterRevocation` | Execute after target revocation | Refused by kernel.Authorize |

---

## 14. Non-Goals (Explicit)

- Generic workflow engine
- Distributed transaction system
- Exactly-once external execution claims
- Generic orchestration platform
- Arbitrary retries without state semantics
- Kubernetes/AWS executor
- Multi-tenancy, RBAC/IAM
- UI redesign
- Agent memory/state platform
- Generic event-sourcing framework

---

## 15. Risk Analysis

| Risk | Severity | Mitigation |
|---|---|---|
| `executing` state survives indefinitely after crash | Medium | Operator reconciliation required; documented |
| RetractCascade cancelling `executing` intent may lose "attempt in progress" marker | Low | External side effect already happened; Solvent state is advisory |
| Schema migration on running cluster | Low | Idempotent DDL (`DROP CONSTRAINT IF EXISTS`, `ADD CONSTRAINT`) |
| API consequence_parameters change breaks existing callers | Low | Callers currently get `{}` anyway; snapshot read is strictly more correct |
| M0 frozen probes become stale | None | M0 probes test `live`/`cancelled` which are unchanged |

---

## 16. Implementation Order

1. **Schema migration** (`db/008_executing_state.sql`) + update test schema paths
2. **Kernel** (`ClaimIntent`, `RollbackClaim`, update `CompleteIntent` SQL, update `RetractCascade`)
3. **Service** (`ExecuteAction` with atomic CAS, `ReconcileIntent`)
4. **API/MCP** (consequence_parameters from snapshot)
5. **HTTP provider** (timeout, error truncation, `%w` wrapping)
6. **Remove `SetTestHook`** from production, rewrite TestExec15B with sync executor
7. **Tests** (acceptance + adversarial)
8. **Proofs** (I-7 invariant)
9. **Verification** (`go build`, `go vet`, `go test -count=1 ./...`, `go test -race ./...`)

---

## 17. Security Invariants Added by Phase 4D

| ID | Invariant | Enforcement |
|---|---|---|
| CI-4 | At most one provider invocation per intent under Solvent-controlled concurrency | `ClaimIntent` atomic CAS on `state = 'live'` → `state = 'executing'` |
| CI-5 | Ambiguous execution requires reconciliation before retry | `executing` state is durable; `ClaimIntent` refuses non-`live` intents |
| CI-6 | Consequence parameters are always from the approved snapshot | API/MCP read snapshot; caller cannot supply substitute parameters |
| CI-7 | Executing intents are cancelled by RetractCascade | `RetractCascade` cancels both `live` and `executing` intents |
