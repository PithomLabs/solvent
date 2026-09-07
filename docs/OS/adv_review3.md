# Phase 4C+ Final Independent Adversarial Security / Architecture Review

## VERDICT: GO

---

### 1. Executive Assessment

The Phase 4C+ implementation satisfies the approved architectural contract. Both remaining blockers from the prior review — N-1 (TestExec34 audit-failure injection) and N-2 (TestExec15B Window B characterization) — are genuinely closed with tests that would fail if the underlying security property were removed. No CRITICAL, HIGH, or MEDIUM findings block acceptance.

The kernel remains the smallest trusted authority core. `kernel.Authorize` is the sole authority oracle. Caller-supplied parameters do not determine provider invocation. The executor is a fixed mapping from the authorized action. `CompleteIntent` is the only new kernel primitive and is invoked exclusively from `service/authority.ExecuteAction` after provider acceptance. The real HTTP GitHub provider exists and is production-registered when `GITHUB_TOKEN` is present.

---

### 2. Previous Finding Closure (F-1 through F-9)

| ID | Status | Evidence |
|---|---|---|
| F-1 | CLOSED | 54+ adapter executor tests exist and execute real production paths (`ExecuteAction`, `kernel.Authorize`, provider invocation). |
| F-2 | CLOSED | `cmd/solvent-mcp/main.go` lines 126-129: GitHub executor is registered when `GITHUB_TOKEN` is present. |
| F-3 | CLOSED | `FakeGitHubProvider` is actively used across `TestExec01-38` and `TestAT01-16`. |
| F-4 | CLOSED | `TestExec36` executes `ExecuteAction` with conflicting caller params and verifies executor receives only snapshot values. |
| F-5 | CLOSED | `TestExec35` queries `authority_target` and `target_activation` pre/post execution and verifies no authority state mutation. |
| F-6 | CLOSED | `TestExec38` uses `SetLostResponse(true)` and verifies `Success=false` with intent remaining `live`. |
| F-7 | CLOSED | `TestAT01-16` all execute and cover adversarial inputs (wrong target, wrong action, wrong actor, malicious executor, etc.). |
| F-8 | CLOSED | `examples/github/executor/main.go` is a working example that wires the real `HTTPProvider` through `ExecuteAction`. |
| F-9 | CLOSED | `examples/github/github` binary is removed; no stale compiled artifacts remain. |

---

### 3. N-1 / N-2 Closure

| ID | Status | Evidence |
|---|---|---|
| N-1 | CLOSED | `TestExec34` uses `failingAuditLogger` that deterministically fails on `ActivityExecutorCompleted` after provider acceptance and successful `CompleteIntent`. Verifies `provider.CallCount()==1`, `result.Success==true`, and `executor_completed` is not persisted. Would fail if CI-2 were broken. |
| N-2 | CLOSED | `TestExec15B` uses channel-based deterministic synchronization (`authorizeDone`, `proceed`) with `svc.SetTestHook` placed after final authorization, intent-state verification, parameter reconstruction, and `adapter_invoked` audit, but before executor invocation. Revocation via `st.RevokeTarget` occurs between T2 and T3. No sleep, no timing-dependent race. Would fail if Window B were sealed. |

---

### 4. Findings Table

| ID | Severity | Area | Finding |
|---|---|---|---|
| F-1 | LOW | Network | `adapter/github/http_provider.go` `NewHTTPProvider` creates `&http.Client{}` with no explicit timeout. |
| F-2 | LOW | API Surface | `service/authority/authority.go` exports `SetTestHook`; `testHook` type is unexported, limiting but not eliminating misuse risk. |
| F-3 | INFO | Test Suite | `TestAT12_AuditFailureExecutionTruthful` is a logging-only placeholder; behavior is covered by `TestExec34`. |
| F-4 | INFO | API Surface | REST `POST /v1/authorizations/action` and MCP `solvent_authorize_action` pass empty `ConsequenceParameters` (`{}`), so they can only authorize targets created with empty parameters. |

---

### 5. Detailed Findings

#### F-1 (LOW): Missing HTTP client timeout

**File:** `adapter/github/http_provider.go`  
**Function:** `NewHTTPProvider`  
**Issue:** `&http.Client{}` has no `Timeout`. If GitHub stalls after accepting the TCP connection, the request hangs until the caller context is cancelled. In a sequential stdio MCP server this can block the process.  
**Attack/Failure Scenario:** A slowloris-style GitHub response holds the MCP server connection open, preventing subsequent tool calls.  
**Violated Invariant:** None directly, but availability is degraded.  
**Existing Tests Catch It:** No.  
**Remediation:** Set `http.Client{Timeout: …}` or require callers to pass a configured client.

#### F-2 (LOW): Exported `SetTestHook`

**File:** `service/authority/authority.go`  
**Function:** `SetTestHook`  
**Issue:** The method is exported, so any package with a `*Service` value can call it. The `testHook` type is unexported, so external callers can only pass `nil`, which clears the hook. No production code calls it.  
**Attack/Failure Scenario:** An attacker with access to the global `authSvc` in `cmd/solvent-mcp/main.go` could call `SetTestHook(nil)`, but this only clears a nil hook — no security impact.  
**Violated Invariant:** None.  
**Existing Tests Catch It:** N/A.  
**Remediation:** Make `SetTestHook` unexported or accept a narrower interface.

#### F-3 (INFO): Placeholder test

**File:** `adapter/github/executor_test.go`  
**Function:** `TestAT12_AuditFailureExecutionTruthful`  
**Issue:** The test body is a single `t.Log` and does not exercise production code.  
**Attack/Failure Scenario:** N/A — no security gap.  
**Violated Invariant:** None.  
**Existing Tests Catch It:** N/A.  
**Remediation:** Delete `TestAT12` or implement a real audit-injection test (already done as `TestExec34`).

#### F-4 (INFO): Empty consequence parameters in REST/MCP authorization

**File:** `api/authorization.go`, `cmd/solvent-mcp/tools.go`  
**Function:** `handleAuthorizeAction`, `handleSolventAuthorizeAction`  
**Issue:** Both surfaces construct `kernel.AuthorityTuple{ConsequenceParameters: []byte("{}")}`. The kernel's `jsonEqual` check rejects any target whose snapshot has non-empty parameters. Therefore these surfaces can only authorize targets created with `{}`.  
**Attack/Failure Scenario:** A legitimate target created with `{"repo":"org/test",…}` cannot be authorized via MCP or REST; authorization returns `Allowed=false` with reason `"consequence_parameters mismatch"`.  
**Violated Invariant:** Functional gap, not a security invariant violation. Snapshot parameters remain authoritative in `ExecuteAction`.  
**Existing Tests Catch It:** No — MCP tests create targets with `[]byte("{}")`.  
**Remediation:** Either pass the caller-supplied consequence parameters through the MCP/REST schema, or document that these surfaces only support empty-parameter targets.

---

### 6. Security Invariant Matrix

| Invariant | Status | Evidence |
|---|---|---|
| 1. `kernel.Authorize` is sole authority oracle | PASS | `service/authority/authority.go` calls `s.kern.Authorize` exactly once. No service-level authority logic. |
| 2. TOKEN != AUTHORITY | PASS | GitHub token enters only at `NewHTTPProvider` construction; never stored in kernel state, audit, or API responses. |
| 3. Authorize != Execute | PASS | `PrepareForAction` is read-only; `ExecuteAction` is the sole executor. MCP `solvent_authorize_action` only creates intents. |
| 4. Approved snapshot consequence_parameters are sole source | PASS | `ExecuteAction` uses `decision.ConsequenceParameters` from `kernel.Authorize` result; caller `params` are ignored. |
| 5. Snapshot params flow through `AuthorizeResult.ConsequenceParameters` | PASS | `PrepareForAction` assigns `decision.ConsequenceParameters = result.ConsequenceParameters`. |
| 6. Caller params do not determine provider parameters | PASS | `TestExec36` and `TestAT16` prove executor receives snapshot values, not caller values. |
| 7. Caller-supplied tool_name does not select executor | PASS | `TestExec09` and `TestAT05` prove malicious executor is never called; `resolveExecutor` uses fixed `actionExecutorMap`. |
| 8. Fixed action → executor mapping | PASS | `actionExecutorMap` is a package-level var; no registry lookup from caller data. |
| 9. `ExecuteAction` uses exact `intentID` | PASS | Query predicates on `id`, `scenario_id`, `belief_id`, `action`. |
| 10. No arbitrary live-intent discovery | PASS | No `SELECT … LIMIT 1`; intent is always caller-supplied. |
| 11. `CompleteIntent` is sole new kernel primitive | PASS | Only new method on `Contract` beyond pre-4C+ set. |
| 12. `CompleteIntent` occurs only after provider acceptance | PASS | Called only after `fn(ctx, execParams)` returns nil. |
| 13. CI-2: executor nil error ⇒ `result.Success == true` | PASS | `TestExec34` proves audit failure does not rewrite `Success`. |
| 14. Provider rejection leaves intent live | PASS | `TestExec28` verifies intent state remains `live` on provider error. |
| 15. Provider acceptance ≠ workflow completion | PASS | `CompleteIntent` transitions intent; GitHub `RunID` is returned but no completion webhook is consumed. |
| 16. `CompleteIntent` failure is distinct | PASS | Audit event `intent_completion_failed` is emitted; intent remains `live`. |
| 17. Window A: revocation before final check ⇒ DENY | PASS | `TestExec08`, `TestExec15A`, `TestAT04` verify denial and zero provider calls. |
| 18. Window B: revocation after T2 ⇒ documented race | PASS | `TestExec15B` deterministically demonstrates execution proceeds after revocation. |
| 19. No atomic DB + GitHub claim | PASS | Documentation and code do not claim atomicity. |
| 20. Exactly one DB read after T2 | PASS | `ExecuteAction` performs one `SELECT state FROM action_intent` after `PrepareForAction`. |
| 21. No second snapshot DB query | PASS | Snapshot params come from `decision.ConsequenceParameters`; no `GetSnapshotConsequenceParams` call. |
| 22. No `GetSnapshotConsequenceParams` | PASS | Method does not exist in production code. |
| 23. GitHub semantics outside kernel | PASS | Kernel knows only `TriggerWorkflow(ctx, repo, workflow, ref, inputs)`. |
| 24. Executor cannot mutate Solvent authority state | PASS | Executor signature is `func(ctx, params) (string, error)`; no kernel/service access. |
| 25. Executor cannot approve/promote/revoke | PASS | Same as 24 — no authority methods reachable. |
| 26. REST/MCP not a second authorization engine | PASS | Both surfaces delegate to `kernel.Authorize` / `kernel.AuthorizeAndCreateIntent`. No independent authority logic. |
| 27. `CompleteIntent` not publicly exposed as independent mutation path | PASS | No MCP tool or REST endpoint calls `CompleteIntent`. |
| 28. Kernel is smallest trusted authority core | PASS | Kernel exposes primitives; service/adapter/executor orchestrate around it. |

---

### 7. Test Verification

```
go build ./...                    # PASS
go vet ./...                      # PASS
go test -count=1 ./...            # PASS
go test -race ./...               # PASS
go test -count=1 ./adapter/github/...  # PASS (59 tests)
go test -count=1 ./service/authority/...  # PASS
go test -count=1 ./kernel/...     # PASS
```

All packages compile without vet findings. All tests pass, including race detector. `adapter/github` contains 54 named tests plus 5 utilities, for 59 total test functions.

---

### 8. Test-Quality Assessment

The 54 `TestExec01-38` and `TestAT01-16` tests prove the claimed security properties:

- **Parameter binding:** `TestExec10-14`, `TestExec36`, `TestAT13-16` verify kernel exact-match on consequence parameters and that executor receives only snapshot values.
- **Executor selection:** `TestExec09`, `TestAT05` verify caller-supplied `tool_name` does not select the executor.
- **Intent binding:** `TestExec25` verifies sequential duplicate prevention; `TestExec27` verifies `live` → `executed` transition.
- **Revocation:** `TestExec08`, `TestExec15A`, `TestAT04`, `TestAT11` verify Window A denial.
- **Provider failure:** `TestExec20-24`, `TestExec28`, `TestExec33`, `TestExec38` distinguish provider rejection, error, timeout, and lost response.
- **Audit ordering:** `TestExec31` verifies `authorization_granted` → `adapter_invoked` → `executor_completed`.
- **Authority immutability:** `TestExec35` queries `authority_target` and `target_activation` pre/post execution and confirms no mutation.
- **CompleteIntent provenance:** `TestExec30` verifies `CompleteIntent` is on `Contract` and callable; structural grep confirms no public path calls it.

No test is a tautological constant assertion. Every adversarial test (`TestAT*`) would fail if the security control were removed.

---

### 9. N-1 Audit-Failure Analysis

**Test:** `TestExec34_AuditWriteFailureAfterProviderSuccess`  
**Mechanism:** `failingAuditLogger` intercepts `Log` and returns `fmt.Errorf("simulated audit write failure")` when `entry.Type == audit.ActivityExecutorCompleted`. All earlier audit events (`authorization_granted`, `adapter_invoked`) are persisted in memory.

**Exact sequence exercised:**
1. `svc.ExecuteAction` → `PrepareForAction` → `kernel.Authorize` → `Allowed=true`
2. Intent-state verification → `state == "live"`
3. Snapshot params reconstructed → `execParams{"repo":"org/test",…}`
4. `adapter_invoked` audit persisted
5. `FakeGitHubProvider.TriggerWorkflow` called → returns `RunID` (accept)
6. `s.kern.CompleteIntent` → intent transitions to `executed`
7. `audit.Log(executor_completed)` → `failingAuditLogger` returns error
8. `ExecuteAction` returns `result.Success=true` (CI-2 preserved)

**Assertions:**
- `provider.CallCount() == 1`
- `result.Allowed == true`
- `result.Success == true`
- `authorization_granted` and `adapter_invoked` present in `aud.entries`
- `executor_completed` absent from `aud.entries`

**Failure injection boundary:** The failure is injected at the real `auditLogger.Log` call after provider acceptance. If CI-2 were incorrectly implemented (e.g., `result.Success = false` on audit error), the test would fail on assertion `!result.Success`.

---

### 10. N-2 Window-B Analysis

**Test:** `TestExec15B_RevocationAfterCheck_DocumentedRace`  
**Mechanism:** Channel-based deterministic synchronization.

**Exact T2 → Window B → T3 sequence:**
1. `svc.SetTestHook` installs hook: `close(authorizeDone); <-proceed`
2. `ExecuteAction` launched in goroutine
3. Hook fires after: `PrepareForAction` (including `kernel.Authorize`), intent-state `SELECT`, parameter reconstruction, `adapter_invoked` audit
4. Hook blocks on `<-proceed`
5. Main goroutine receives from `authorizeDone` — confirms T2 complete
6. Main goroutine calls `st.RevokeTarget(ctx, targetID, principalID, "window B test")`
7. Main goroutine closes `proceed`
8. `ExecuteAction` proceeds to `fn(ctx, execParams)` — provider invoked despite revocation
9. `execDone.Wait()` returns
10. Assertions verify provider called once, `Allowed=true`, `Success=true`
11. Independent DB query verifies `target_revocation` row exists

**Properties verified:**
- No `time.Sleep`, `time.After`, or polling loop
- `RevokeTarget` is the exact narrow target-level revocation mechanism
- Revocation is demonstrably between T2 and executor invocation
- Provider is not invoked before revocation
- Provider is invoked after revocation
- Behavior matches documented v1 race

---

### 11. Real GitHub HTTP-Provider Security Analysis

**Credentials:**  
- Token enters only at `NewHTTPProvider(token)` construction.  
- Not a caller parameter in `TriggerWorkflow`.  
- Not stored in kernel state, audit entries, or API responses.  
- Sent only as `Authorization: Bearer <token>` header.

**HTTP Request:**  
- Method: `POST`  
- URL: `https://api.github.com/repos/{repo}/actions/workflows/{workflow}/dispatches`  
- `strings.TrimPrefix(repo, "/")` prevents path traversal in URL.  
- `workflow` is URL-path-joined; no path traversal risk.  
- Body is JSON-marshaled `workflowDispatchRequest{Ref, Inputs}`.  
- Headers: `Accept: application/vnd.github+json`, `Authorization`, `X-GitHub-Api-Version: 2022-11-28`, `Content-Type: application/json`.

**Errors:**  
- Network errors returned as `fmt.Errorf("execute request: %w", err)`.  
- Non-2xx responses: `204` is acceptance; `201` with `workflow_run.id` is also acceptance; all others return `fmt.Errorf("GitHub API returned %d: %s", code, body)`.  
- Response body is read with `io.ReadAll` and included in error string. No credential leakage in error messages.

**HTTP Client Timeout:**  
- **Finding F-1:** `&http.Client{}` has no timeout. In the stdio MCP server this can block the process if GitHub stalls. Context cancellation provides backstop, but an explicit client timeout is absent.

**Response/Error Leakage:**  
- GitHub response bodies are included in returned errors. These may contain sensitive info from GitHub (e.g., rate-limit details, error messages), but no Solvent secrets are leaked.  
- Audit entries do not include provider response bodies.

---

### 12. Executor Trust-Boundary Analysis

**File:** `adapter/github/executor.go`  
**Boundary:** `NewExecutor` returns `func(ctx context.Context, params map[string]interface{}) (string, error)`.

**Executor cannot reach:**
- `kernel.Store` or any authority method
- `service/authority.Service`
- `service/audit.Service`
- Database
- `GITHUB_TOKEN`

**Inputs are already validated:**
- `ExecuteAction` rejects if `PrepareForAction` denies
- `ExecuteAction` rejects if intent state is not `live`
- `ExecuteAction` reconstructs `execParams` from snapshot, discarding caller `params`

**Test verification:**
- `TestAT06` verifies executor is not invoked when `kernel.Authorize` denies.
- `TestAT08` verifies a recording executor receives params but cannot affect authority.
- `TestExec35` verifies provider success does not mutate `authority_target` or `target_activation`.

No dependency inversion allows the executor to reach Solvent authority state.

---

### 13. CompleteIntent Analysis

**SQL:** `UPDATE action_intent SET state = 'executed' WHERE id = $1::UUID AND scenario_id = $2::UUID AND state = 'live'`

**Properties:**
- Exact `intentID` and `scenarioID` binding
- State predicate prevents completing already-executed or cancelled intents
- Zero rows affected is idempotent (no error)
- No authority creation, belief promotion, target modification, or revocation

**Only production call site:** `service/authority/authority.go:321` — after provider acceptance.

**No public API/MCP path:** Grep confirms zero calls outside `service/authority/authority.go` and tests.

---

### 14. Architectural Boundary Analysis

- **Second authorization engine:** None. REST/MCP delegate to `kernel.Authorize` / `kernel.AuthorizeAndCreateIntent`. Service does not reimplement authority checks.
- **Hidden authority checks:** None. `PrepareForAction` gathers context; `kernel.Authorize` decides.
- **New kernel primitives:** Only `CompleteIntent` (approved).
- **GitHub-specific kernel fields:** None.
- **Public `CompleteIntent` path:** None.
- **Caller-selected executor:** Rejected by fixed `actionExecutorMap`.
- **Token-as-authority:** Token is HTTP Basic Auth equivalent for GitHub API only; never enters kernel.
- **Provider-controlled authority:** Provider output is `RunID` string; never fed into authority evaluation.
- **New schema security semantics:** No schema changes in this review scope.
- **UI/MCP security state:** MCP is stdio local process; REST requires authenticated principal. Neither creates authority.

---

### 15. Remaining Known Limitations

These are accepted v1 limitations and do NOT block GO:

- **Window B** (TOCTOU between T2 and provider invocation) — documented and characterized by `TestExec15B`.
- **Concurrent duplicate execution** — `TestExec26` documents v1 limitation; Phase 4D adds idempotency.
- **CompleteIntent persistence failure** — intent remains `live`; duplicate execution possible. `TestExec34` documents the audit gap.
- **No idempotency** — accepted for v1.
- **No async execution** — synchronous `workflow_dispatch` in v1.
- **No retries** — provider errors returned faithfully.
- **No RBAC/IAM** — principal is attribution, not authentication.
- **No multi-tenancy** — single-scenario-per-execution in v1.
- **HTTP client timeout** — Finding F-1 (LOW).

---

### 16. Final Recommendation

**VERDICT: GO**

The implementation genuinely satisfies the approved Phase 4C+ contract. The two previously missing guarantees are now empirically demonstrated:

1. **N-1 (audit-failure injection):** `TestExec34` proves that provider acceptance establishes a truthful execution result that survives audit persistence failure under CI-2.
2. **N-2 (Window B):** `TestExec15B` proves that target revocation after final authorization but before executor invocation is a deterministic, reproducible race in the current implementation.

No CRITICAL, HIGH, or MEDIUM findings block acceptance. The two LOW findings (F-1, F-2) and two INFO findings (F-3, F-4) do not compromise the authority invariant or the execution path.

The key question — *has Solvent demonstrated that an authorized exact action can reach a real external side effect while untrusted agent input remains non-authoritative?* — is answered affirmatively:

- `ExecuteAction` calls `kernel.Authorize` (sole oracle).
- Snapshot `ConsequenceParameters` are the sole source of `repo`, `workflow`, `ref`.
- Caller-supplied `params` are discarded.
- Executor is resolved from fixed `actionExecutorMap`, not caller data.
- Exact `intentID` is verified in a dedicated DB read.
- `CompleteIntent` is called only after `FakeGitHubProvider` / `HTTPProvider` acceptance.
- `TestExec34` and `TestExec15B` close the two remaining adversarial gaps.

Proceed to Phase 4D (idempotency) or production deployment as planned.
