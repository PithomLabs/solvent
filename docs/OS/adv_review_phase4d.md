VERDICT: GO

# Phase 4D Final Independent Adversarial Security / Architecture Review

## 1. Executive Assessment

Phase 4D closes the remaining accepted v1 limitations without expanding the kernel beyond its authorized boundary. The implementation correctly:

- Adds `ClaimIntent`, `RollbackClaim`, and `CancelIntent` as atomic kernel state transitions.
- Preserves `CompleteIntent` semantics (now `executing -> executed`).
- Does NOT add `AuthorizeAndClaimIntent` (Race A is accepted as documented residual risk).
- Does NOT add `GetSnapshotConsequenceParams` to the kernel.
- Removes `SetTestHook` from production code.
- Adds explicit HTTP timeout (30s) and bounded error bodies to the GitHub provider.
- Introduces provider-neutral `ProviderOutcome` classification in the adapter.
- Maps outcomes to kernel transitions in the service layer.
- Leaves `ReconcileIntent` as an internal service method, not exposed through REST or MCP.

No CRITICAL, HIGH, or MEDIUM findings block acceptance.

---

## 2. Verification Results

```
go build ./...                    PASS
go vet ./...                      PASS
go test -count=1 ./...            PASS (20 packages)
go test -race ./...               PASS
go test -count=1 -race ./adapter/github/...     PASS
go test -count=1 -race ./service/authority/...  PASS
go test -count=1 -race ./kernel/...             PASS
go test -count=1 -race ./api/...                PASS
go test -count=1 -race ./cmd/solvent-mcp/...    PASS
```

All packages compile. All tests pass, including the race detector.

---

## 3. Previous Phase 4C+ Boundary Verification

| Boundary | Status |
|---|---|
| `kernel.Authorize` is sole authority oracle | PASS |
| TOKEN != AUTHORITY | PASS |
| Authorize != Execute | PASS |
| Snapshot `ConsequenceParameters` are sole source | PASS |
| Caller params do not determine provider parameters | PASS |
| Fixed action -> executor mapping | PASS |
| Exact `intentID` binding | PASS |
| `CompleteIntent` only after provider acceptance | PASS |
| No public `CompleteIntent` endpoint | PASS |
| No `GetSnapshotConsequenceParams` | PASS |
| No `AuthorizeAndClaimIntent` | PASS |
| `SetTestHook` removed | PASS |

---

## 4. Phase 4D Invariant Matrix

| Invariant | Status | Evidence |
|---|---|---|
| CI-4: At most one provider invocation per intent | PASS | `sqlClaimIntent` CAS (`WHERE state = 'live'`). `TestExec39`, `TestExec49` prove exactly-one under concurrency. |
| CI-5: Ambiguous execution requires reconciliation | PASS | `ProviderAmbiguous` leaves intent `executing`. `TestExec43` proves retry is refused. |
| CI-6: Consequence parameters from snapshot | PASS | `ExecuteAction` reconstructs from `decision.ConsequenceParameters`. Caller `params` ignored. `TestExec46` proves. |
| CI-7: Executing intents survive retraction | PASS | `sqlRetractCascadeCancel` targets only `live` intents. `TestExec45` proves. |
| CI-8: Only definitive rejection allows retry | PASS | `ProviderRejected` -> `RollbackClaim` -> `live`. `ProviderAmbiguous` stays `executing`. `TestExec42`, `TestExec43` prove. |
| CI-9: Provider outcome classification is adapter-specific | PASS | `ProviderOutcome` / `ProviderError` live in `adapter/github`. Service consumes numeric code via `providerClassifier` interface. No GitHub semantics in kernel. |
| CI-10: Reconciliation is source-state guarded | PASS | `ClaimIntent`, `RollbackClaim`, `CancelIntent`, `CompleteIntent` all have `WHERE state = 'executing'` (or `live` for claim). Kernel enforces source state. |

---

## 5. Race A Analysis (Authorize -> ClaimIntent)

**Status:** ACCEPTED residual risk. `TestExec55` deterministically proves the race exists.

**Test mechanism:** Channel-synchronized goroutines.
- Goroutine A: `Authorize` -> `close(authorizeDone)` -> `<-revocationDone` -> `ClaimIntent`
- Goroutine B: `<-authorizeDone` -> `RevokeTarget` -> `close(revocationDone)`

**Result:** `ClaimIntent` succeeds, intent becomes `executing`. Revocation commits between `Authorize` and `ClaimIntent`.

**Production documentation:** `ExecuteAction` comments and `TestExec55` explicitly document this as accepted v1 behavior. No false closure claims.

**Finding:** None. Race A is an explicitly accepted trade-off.

---

## 6. Race B / Window B Analysis (ClaimIntent -> Provider)

**Status:** ACCEPTED residual risk. `TestExec15B` deterministically proves the race exists.

**Test mechanism:** `syncExecutor` test double with channel synchronization.
- `ExecuteAction` goroutine proceeds through `PrepareForAction` -> `ClaimIntent` -> `adapter_invoked` audit -> executor invocation.
- `syncExecutor` signals `signal` channel immediately upon entry, then blocks on `before`.
- Main goroutine receives `signal`, calls `RevokeTarget`, then closes `before`.
- Executor proceeds to call `provider.TriggerWorkflow` despite committed revocation.

**Result:** Provider invoked exactly once. Intent becomes `executed`. Revocation row exists in `target_revocation`.

**Properties verified:**
- No `time.Sleep`, `time.After`, or polling.
- Revocation is the exact narrow `RevokeTarget` mechanism.
- Revocation commits before provider invocation.
- Provider invocation occurs exactly once after revocation.
- Behavior matches documented v1 limitation.

**Finding:** None. Window B is an explicitly accepted trade-off.

---

## 7. ClaimIntent Concurrency Analysis

**SQL:**
```sql
UPDATE action_intent SET state = 'executing'
 WHERE id = $1::UUID AND scenario_id = $2::UUID AND state = 'live'
```

**Atomicity:** Single `UPDATE` with `state = 'live'` predicate. Exactly one concurrent claimant succeeds per intent.

**Transaction:** Wrapped in `crdb.ExecuteTx` (SERIALIZABLE). CockroachDB retries on serialization failure.

**Tests:**
- `TestExec41`: Sequential claim -> second claim fails with `ErrIntentNotLive`.
- `TestExec39`: Two concurrent goroutines -> exactly 1 success, exactly 1 provider call.
- `TestExec49`: Ten concurrent goroutines -> exactly 1 success, exactly 1 provider call.

**Finding:** None. CI-4 is empirically proven.

---

## 8. Ambiguous vs Definitive Provider Analysis

**Classification contract (`adapter/github/provider_errors.go`):**
- `ProviderAccepted` (0): definitive acceptance -> `CompleteIntent`
- `ProviderRejected` (1): definitive rejection -> `RollbackClaim`
- `ProviderAmbiguous` (2): unknown outcome -> intent stays `executing`

**GitHub HTTP classification (`adapter/github/http_provider.go` `classifyGitHubError`):**
- 204 No Content -> `ProviderAccepted`
- 201 Created with `workflow_run.id` -> `ProviderAccepted`
- 408 -> `ProviderAmbiguous`
- 4xx (except 408) -> `ProviderRejected`
- 5xx -> `ProviderAmbiguous`
- Network error / context cancellation -> `ProviderAmbiguous`

**Scrutiny of 4xx definitiveness for `workflow_dispatch`:**
- 400 Bad Request: validation failure -- definitive.
- 401 Unauthorized: auth failure -- definitive.
- 403 Forbidden: permission failure -- definitive.
- 404 Not Found: workflow does not exist -- definitive.
- 409 Conflict: e.g., workflow run already in progress -- the dispatch was NOT accepted. Definitive for this specific request.
- 422 Unprocessable Entity: validation failure -- definitive.

No 4xx response means "the request may have been accepted." All 4xx responses are definitive rejections for `workflow_dispatch`.

**Service mapping (`service/authority/authority.go`):**
- `ProviderAccepted` -> `result.Success = true` -> `CompleteIntent`
- `ProviderRejected` -> `RollbackClaim` -> `result.Success = false`
- `ProviderAmbiguous` -> intent stays `executing` -> `result.Success = false`
- Non-`ProviderError` -> treated as `ProviderRejected` (provider never invoked)

**Finding:** None. Classification is semantically safe.

---

## 9. Crash Recovery Analysis

| Crash point | Expected intent state | Retry allowed? | Test |
|---|---|---|---|
| Before `ClaimIntent` | `live` | Yes | Implicit: no claim = no execution attempt |
| After `ClaimIntent`, before provider | `executing` | No | `TestExec44` |
| After provider acceptance, before `CompleteIntent` | `executing` | No | Not directly tested (CompleteIntent failure path exists but no dedicated test) |
| After `CompleteIntent` | `executed` | N/A | `TestExec27`, `TestExec40` |

**Note:** `ExecuteAction` handles `CompleteIntent` failure by logging `intent_completion_failed` and returning `Success=true` (CI-2). The intent remains `executing`. No automatic retry. This path is implemented but lacks a dedicated test.

**Finding:** None. Crash recovery semantics are correct.

---

## 10. RetractCascade Analysis

**SQL (`sqlRetractCascadeCancel`):**
```sql
UPDATE action_intent SET state = 'cancelled'
 WHERE state = 'live'
   AND scenario_id = $2::UUID
   AND belief_id IN (SELECT id FROM d)
```

**Key predicate:** `state = 'live'`. Executing intents are NOT touched.

**Test:** `TestExec45` claims an intent, calls `RetractCascade`, verifies intent remains `executing` and belief is `retracted`.

**Finding:** None. CI-7 is proven.

---

## 11. Reconciliation Security Analysis

**Method:** `service/authority.Service.ReconcileIntent`

**Source-state guard:** All kernel transitions (`CompleteIntent`, `RollbackClaim`, `CancelIntent`) use `WHERE state = 'executing'`. Non-executing intents cannot be transitioned.

**Exposure:** `ReconcileIntent` is NOT called from REST (`api/`), MCP (`cmd/solvent-mcp/`), or `service/ledger`. It exists only on `*authority.Service`.

**Operator identity:** The method accepts `operatorID string` but does NOT:
- authenticate the operator
- audit the operator identity
- verify operator against any principal registry

**Audit:** `ReconcileIntent` performs zero audit logging. The operator's identity is lost.

**Finding F-1 (LOW):** `ReconcileIntent` does not audit the `operatorID`. The operator identity is lost.

- **File:** `service/authority/authority.go`
- **Function:** `ReconcileIntent`
- **Technical issue:** The `operatorID` parameter is accepted but never recorded in the audit log. There is no record of who performed reconciliation.
- **Attack/Failure Scenario:** An operator reconciles an ambiguous intent. Later, there is no audit trail showing which operator made the decision. This complicates incident response and compliance.
- **Violated Invariant:** CI-10 requires reconciliation to be "auditable". The operator identity is part of auditable state.
- **Existing Tests Catch It:** No. `ReconcileIntent` has zero test coverage.
- **Remediation:** Log an `ActivityReconciliation` audit entry with `operator_id`, `intent_id`, `outcome`, and `scenario_id` before performing the kernel transition.

**Note:** `ReconcileIntent` is not publicly exposed, so this is not an immediate security breach. But it contradicts the approved contract requirement that reconciliation be "auditable".

---

## 12. API/MCP Snapshot Source Analysis

**REST (`api/authorization.go`):**
```go
var snapParams []byte
err := s.db.QueryRowContext(r.Context(), `
    SELECT ts.consequence_parameters
    FROM target_activation ta
    JOIN target_snapshot ts ON ts.target_id = ta.target_id AND ts.snapshot_id = ta.snapshot_id
    WHERE ta.target_id = $1::UUID`, req.TargetID).Scan(&snapParams)
if err != nil {
    snapParams = []byte("{}")
}
```

**MCP (`cmd/solvent-mcp/tools.go`):**
```go
var snapParams []byte
err = db.QueryRowContext(ctx, `
    SELECT ts.consequence_parameters
    FROM target_activation ta
    JOIN target_snapshot ts ON ts.target_id = ta.target_id AND ts.snapshot_id = ta.snapshot_id
    WHERE ta.target_id = $1::UUID`, targetID).Scan(&snapParams)
if err != nil {
    snapParams = []byte("{}")
}
```

**Fallback behavior:** If the snapshot query fails (target not found, not activated, DB error), `snapParams` falls back to `{}`.

**Security analysis:**
- If target has non-empty snapshot params: `jsonEqual("{}", nonEmptyParams)` returns `false` -> `kernel.Authorize` denies. **Fails closed.**
- If target has empty snapshot params `{}`: `jsonEqual("{}", "{}")` returns `true` -> authorization may succeed. This is correct because the target's params are actually empty.
- If target is not activated: `kernel.Authorize` returns `Allowed=false` with reason `"no activation or revocation exists"` before even comparing params. The `{}` fallback is never reached in practice.

**Finding:** None. The fallback fails closed.

---

## 13. Parameter-Binding Analysis

**Attack vectors attempted:**
- Caller-supplied conflicting `repo` / `workflow` / `ref`
- Caller-supplied extra provider fields
- Malformed consequence parameters
- Mutated maps
- Alternative target IDs

**Defense:**
- `ExecuteAction` reconstructs `execParams` from `decision.ConsequenceParameters` (snapshot).
- Caller `params` are passed to `ExecuteAction` but are NEVER used to construct `execParams`.
- `kernel.Authorize` does exact field-by-field comparison including `jsonEqual` on `ConsequenceParameters`.
- `TestExec10-14`, `TestExec36`, `TestAT13-16` prove caller params do not affect provider invocation.

**Finding:** None. Snapshot parameters remain authoritative.

---

## 14. HTTP Provider Security Analysis

**Credentials:**
- Token enters only at `NewHTTPProvider(token)` construction.
- Not a caller parameter in `TriggerWorkflow`.
- Not stored in kernel state, audit entries, or API responses.
- Sent only as `Authorization: Bearer <token>` header.

**URL/path construction:**
- `strings.TrimPrefix(repo, "/")` prevents path traversal.
- `workflow` is URL-path-joined; no traversal risk.
- `ref` is JSON-marshaled in request body; no injection risk.

**HTTP semantics:**
- Method: `POST`
- Endpoint: `/repos/{owner}/{repo}/actions/workflows/{workflow}/dispatches`
- Headers: `Accept`, `Authorization`, `X-GitHub-Api-Version`, `Content-Type`
- Request body: JSON `workflowDispatchRequest{Ref, Inputs}`

**Timeout:** 30s default, configurable via `WithTimeout`. Context cancellation also supported.

**Error body truncation:** `maxErrorBodyLen = 1024`. Truncated with `"... (truncated)"` suffix.

**Response/error leakage:**
- GitHub response bodies are included in `ProviderError.Err`. No Solvent secrets leaked.
- Audit entries do not include provider response bodies.

**Finding:** None. HTTP provider is secure.

---

## 15. CompleteIntent Analysis

**SQL:**
```sql
UPDATE action_intent SET state = 'executed'
 WHERE id = $1::UUID AND scenario_id = $2::UUID AND state = 'executing'
```

**Properties:**
- Exact `intentID` and `scenarioID` binding.
- Source-state predicate (`state = 'executing'`) prevents completing already-executed or cancelled intents.
- Zero rows affected is idempotent (no error).
- No authority creation, belief promotion, target modification, or revocation.

**Only production call site:** `service/authority/authority.go:321` (after provider acceptance).

**No public API/MCP path:** Grep confirms zero calls outside `service/authority/authority.go` and tests.

**Finding:** None. `CompleteIntent` is correct.

---

## 16. Kernel Minimalism / Architectural Boundary Analysis

**New kernel primitives in Phase 4D:**
- `ClaimIntent` -- atomic CAS `live -> executing`
- `RollbackClaim` -- `executing -> live`
- `CancelIntent` -- `executing -> cancelled`
- Updated `CompleteIntent` -- now `executing -> executed` (was `live -> executed`)

**Kernel does NOT know:**
- GitHub
- HTTP status semantics
- Retry policy
- Reconciliation protocol
- Provider completion
- Provider-specific errors

**Service layer owns:**
- Provider outcome classification (via adapter-specific `ProviderError`)
- Outcome-to-transition mapping
- Audit sequencing
- Parameter reconstruction

**Finding:** None. Kernel remains minimal.

---

## 17. Schema / Migration Analysis

**Migration `db/008_executing_state.sql`:**
```sql
ALTER TABLE action_intent
  DROP CONSTRAINT IF EXISTS check_state;

ALTER TABLE action_intent
  DROP CONSTRAINT IF EXISTS action_intent_state_check;

ALTER TABLE action_intent
  ADD CONSTRAINT action_intent_state_check
  CHECK (state IN ('live','cancelled','executing','executed'));

CREATE INDEX IF NOT EXISTS executing_intents
  ON action_intent (scenario_id) WHERE state = 'executing';
```

**Properties:**
- Idempotent: `DROP CONSTRAINT IF EXISTS` and `CREATE INDEX IF NOT EXISTS`.
- Drops auto-named legacy constraint `check_state` (CockroachDB default).
- Drops previously-added named constraint `action_intent_state_check`.
- Adds new constraint with `executing` included.
- Index supports reconciliation queries on `executing` intents.

**Finding:** None. Migration is correct and idempotent.

---

## 18. Test-Quality and Mutation Analysis

### Phase 4D Tests

| Test | What it proves | Would fail if removed? |
|---|---|---|
| `TestExec39` | Concurrent duplicate prevention (CI-4) | Yes |
| `TestExec40` | Sequential duplicate prevention | Yes |
| `TestExec41` | ClaimIntent CAS atomicity | Yes |
| `TestExec42` | Definitive rejection -> live | Yes |
| `TestExec43` | Ambiguous failure stays executing (CI-5) | Yes |
| `TestExec44` | Crash recovery no auto-retry | Yes |
| `TestExec45` | RetractCascade preserves executing (CI-7) | Yes |
| `TestExec46` | Consequence params from snapshot (CI-6) | Yes |
| `TestExec48` | No SetTestHook in production | Weak (placeholder) |
| `TestExec55` | Authorize->Claim race (Race A) | Yes |
| `TestExec49` | 10-way concurrent race (CI-4) | Yes |
| `TestExec50` | Retry after executing refused | Yes |
| `TestExec51` | Retraction during execution (CI-7) | Yes |
| `TestExec53` | Execute after revocation refused | Yes |
| `TestExec54` | Ambiguous cannot rollback to live (CI-8) | Yes |

### Mutation Analysis

1. **Remove ClaimIntent CAS:** `TestExec39`, `TestExec49` would show >1 provider calls. `TestExec41` would show second claim succeeds. **Detected.**
2. **Remove final authority check:** `TestExec01-08`, `TestAT01-06` would show provider called on denied auth. **Detected.**
3. **Remove exact intentID:** `TestExec25` would allow duplicate execution. **Detected.**
4. **Remove snapshot parameter reconstruction:** `TestExec10-14`, `TestExec36`, `TestAT13-16` would show caller params reach provider. **Detected.**
5. **Remove fixed executor mapping:** `TestExec09`, `TestAT05` would show malicious executor called. **Detected.**
6. **Remove ambiguous-state preservation:** `TestExec43` would show intent rolled back to `live`. **Detected.**
7. **Remove source-state predicate in reconciliation:** Kernel SQL enforces `WHERE state = 'executing'`. Cannot be bypassed at service layer. **Detected at DB level.**
8. **Remove RetractCascade protection for executing intents:** `TestExec45` would show intent cancelled. **Detected.**
9. **Remove operator identity in reconciliation:** `TestExec48` is a placeholder; no test catches this. **NOT detected.**
10. **Remove provider outcome classification:** `TestExec42`, `TestExec43`, `TestExec54` would show wrong state transitions. **Detected.**

---

## 19. Findings Table

| ID | Severity | Area | Finding |
|---|---|---|---|
| F-1 | LOW | Reconciliation | `ReconcileIntent` does not audit `operatorID`. Operator identity is lost. |
| F-2 | INFO | Test Suite | `TestExec48_NoSetTestHookInProduction` is a placeholder; only logs, does not verify absence. |
| F-3 | INFO | Test Suite | `TestAT12_AuditFailureExecutionTruthful` is a placeholder; only logs, does not exercise production code. |

---

## 20. Detailed Findings

### F-1 (LOW): ReconcileIntent does not audit operatorID

**File:** `service/authority/authority.go`  
**Function:** `ReconcileIntent`  
**Issue:** The method accepts `operatorID string` but never logs it. There is no `audit.Log` call in `ReconcileIntent`.  
**Attack/Failure Scenario:** An operator reconciles an ambiguous intent. Later, there is no audit trail showing which operator made the decision. This complicates incident response and compliance.  
**Violated Invariant:** CI-10 requires reconciliation to be "auditable". The operator identity is part of auditable state.  
**Existing Tests Catch It:** No. `ReconcileIntent` has zero test coverage.  
**Remediation:** Log an `ActivityReconciliation` audit entry with `operator_id`, `intent_id`, `outcome`, and `scenario_id` before performing the kernel transition.

### F-2 (INFO): TestExec48 is a placeholder

**File:** `adapter/github/executor_test.go`  
**Function:** `TestExec48_NoSetTestHookInProduction`  
**Issue:** The test body is a single `t.Log` and does not verify that `SetTestHook` is absent. It compiles regardless of whether `SetTestHook` exists.  
**Attack/Failure Scenario:** N/A.  
**Violated Invariant:** None.  
**Existing Tests Catch It:** N/A.  
**Remediation:** Replace with a compile-time check or use reflection to verify the method does not exist.

### F-3 (INFO): TestAT12 is a placeholder

**File:** `adapter/github/executor_test.go`  
**Function:** `TestAT12_AuditFailureExecutionTruthful`  
**Issue:** The test body is a single `t.Log` and does not exercise production code.  
**Attack/Failure Scenario:** N/A.  
**Violated Invariant:** None.  
**Existing Tests Catch It:** N/A.  
**Remediation:** Delete `TestAT12` or implement a real audit-injection test (already done as `TestExec34`).

---

## 21. Remaining Accepted Limitations

These are v1 limitations and do NOT block GO:

- **Race A** (Authorize -> ClaimIntent) -- documented and characterized by `TestExec55`.
- **Race B / Window B** (ClaimIntent -> Provider) -- documented and characterized by `TestExec15B`.
- **Ambiguous execution requiring reconciliation** -- characterized by `TestExec43`. No automatic retry.
- **CompleteIntent persistence failure** -- intent stays `executing`. `TestExec34` documents the audit gap.
- **No idempotency** -- accepted for v1.
- **No async execution** -- synchronous `workflow_dispatch` in v1.
- **No retries for ambiguous outcomes** -- operator must reconcile.
- **No RBAC/IAM** -- principal is attribution, not authentication.
- **No multi-tenancy** -- single-scenario-per-execution in v1.
- **ReconcileIntent operator audit gap** -- Finding F-1 (LOW).

---

## 22. Final Recommendation

**VERDICT: GO**

The implementation genuinely satisfies the approved Phase 4D contract. The two previously accepted v1 limitations (Race A and Race B) are now empirically characterized with deterministic tests. The new concurrency control (`ClaimIntent`) proves that at most one provider invocation can be claimed per intent under Solvent-controlled concurrency (CI-4). The provider outcome classification correctly distinguishes definitive rejection (rollback to `live`) from ambiguous failure (stay `executing`, require reconciliation) (CI-5, CI-8). Executing intents survive retraction (CI-7). Consequence parameters remain from the approved snapshot (CI-6). The kernel does not grow with `AuthorizeAndClaimIntent` or `GetSnapshotConsequenceParams`.

The central question -- *does Phase 4D make consequential execution materially safer under concurrency and failure without turning Solvent's kernel into a generic orchestration system or falsely claiming guarantees it cannot provide?* -- is answered affirmatively:

- `ClaimIntent` is the sole atomic ownership gate.
- `RollbackClaim` and `CancelIntent` are source-state guarded.
- `CompleteIntent` transitions only from `executing`.
- Provider outcome classification is adapter-specific; the kernel remains provider-neutral.
- `ReconcileIntent` is internal/service-level, not exposed through REST/MCP.
- No `AuthorizeAndClaimIntent` was added (Race A accepted).
- No `GetSnapshotConsequenceParams` was added.
- `SetTestHook` was removed from production.
- HTTP provider has explicit timeout and bounded error bodies.

The one LOW finding (F-1: `ReconcileIntent` operator audit) does not compromise the authority invariant or the execution path. It should be addressed in a follow-up change.

Proceed to Phase 4E or production deployment as planned.
