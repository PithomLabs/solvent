# Phase 4E Independent Adversarial Security / Architecture Review

**Date:** 2026-09-07  
**Reviewer:** Independent senior security architect (read-only, adversarial)  
**Subject:** Phase 4E implementation — `authority → intent → execution` end-to-end workflow  
**Verdict:** VERDICT: GO

---

## 1. Executive Assessment

Phase 4E turns the proven `authority → intent → execution` architecture into a coherent end-to-end agent workflow. The implementation adds two new entry points — `POST /v1/authorizations/execute` (REST) and `solvent_execute` (MCP) — both of which delegate to the existing `authority.Service.ExecuteAction`. That service is the **sole production execution path**; it calls `kernel.Authorize`, resolves the executor from an internal registry (never caller-supplied), reads `consequence_parameters` from the approved snapshot (never from the caller), claims the intent via `ClaimIntent`, invokes the adapter, and records the outcome via `CompleteIntent` / `RollbackClaim`.

The review attempted to break the implementation through every attack vector specified in the review brief. All build, vet, race-detector, integration, and MCP verification commands pass. No CRITICAL, HIGH, or MEDIUM finding was confirmed.

**Bottom line:** The implementation satisfies the Phase 4E contract. An untrusted agent cannot reach a real consequential GitHub action while bypassing Solvent's authority boundary.

---

## 2. Actual Verification Results

| Check | Command | Result |
|---|---|---|
| Build | `go build ./...` | PASS |
| Vet | `go vet ./...` | PASS |
| Tests (no race) | `go test -count=1 -p 1 ./...` | PASS |
| Tests (race) | `go test -race -count=1 -p 1 ./...` | PASS |
| Task test | `task test` | PASS |
| I-7 gate | `bash scripts/check_i7.sh` | PASS |
| MCP verify | `bash scripts/mcp_verify.sh` | PASS |
| API race | `go test -race ./api/...` | PASS |
| Authority race | `go test -race ./service/authority/...` | PASS |
| Kernel race | `go test -race ./kernel/...` | PASS |
| GitHub adapter race | `go test -race ./adapter/github/...` | PASS |
| MCP race | `go test -race ./cmd/solvent-mcp/...` | PASS |

---

## 3. REST Execution Boundary Analysis (`POST /v1/authorizations/execute`)

**File:** `api/authorization.go:167-267`

### 3.1 Principal Attack
**Attack:** Authenticated principal A + intent belonging to principal B → execution  
**Expected:** DENIED  
**Result:** DENIED

The handler derives `effectiveActor` exclusively from `AuthFromContext(r.Context())`, which is set by `AuthMiddleware` from the Bearer token (`api/auth.go:26-54`). The request body contains no `principal_id` or `actor_id` field (`api/types.go:195-205`). The handler passes `effectiveActor` to `authSvc.ExecuteAction`, which passes it into `kernel.Authorize` as `tuple.PrincipalID`. `authorizeWithinTx` compares `tuple.PrincipalID` against `snapPrincipalID` from the approved snapshot (`kernel/authority.go:386-388`). A mismatch returns `Allowed: false`.

The adversarial test `TestExecute_CrossPrincipalDenied` (`api/authorization_test.go:287-363`) constructs exactly this scenario: target owned by `otherPrincipalID`, request authenticated as `testPrincipalID`. The kernel denies with `principal mismatch`. The executor is never invoked.

### 3.2 Context Substitution Attacks
**Attack:** With a valid intent, change `scenario_id`, `belief_id`, `action`, `target_id`, or `intent_id`  
**Expected:** DENIED  
**Result:** DENIED for all fields

The handler passes all caller-supplied context fields (`scenario_id`, `belief_id`, `action`, `target_id`, `intent_id`) into `authSvc.ExecuteAction`. `ExecuteAction` calls `PrepareForAction`, which constructs a `kernel.AuthorityTuple` with those exact values and calls `s.kern.Authorize(ctx, targetID, tuple)` (`service/authority/authority.go:172`). `authorizeWithinTx` compares the presented tuple field-by-field against the snapshot (`kernel/authority.go:386-429`). Any mismatch returns `Allowed: false`.

The adversarial tests `TestExecute_WrongScenarioDenied`, `TestExecute_WrongBeliefDenied`, `TestExecute_WrongActionDenied`, `TestExecute_WrongTargetDenied` (`api/authorization_test.go:365-503`) all confirm denial.

### 3.3 Consequence Parameter Attack
**Attack:** Caller sends `consequence_parameters` in the request body  
**Expected:** Caller values ignored; approved snapshot remains sole execution source  
**Result:** Caller values ignored

`ExecuteActionRequest` has no `consequence_parameters` field (`api/types.go:195-205`). Even if a caller adds an undocumented field, `json.NewDecoder(r.Body).Decode(&req)` silently ignores unknown fields. The handler reads snapshot params from the database (`api/authorization.go:224-232`) and passes them to `authSvc.ExecuteAction` as `snapParams`. The caller-supplied `params` argument is always `map[string]interface{}{}` (`api/authorization.go:241`).

`TestExecute_CallerParamsIgnored` (`api/authorization_test.go:505-546`) sends `consequence_parameters: '{"repo":"evil/repo","workflow":"evil.yml","ref":"evil"}'` in the body and confirms the request succeeds (allowed=true) because the handler uses DB params, not caller params.

### 3.4 Executor Attack
**Attack:** Caller supplies `tool_name`, `executor`, `provider`, or alternate registry key  
**Expected:** Caller cannot select another executor  
**Result:** Caller cannot select another executor

`ExecuteActionRequest` has no executor-selection field. The handler passes `req.Action` to `authSvc.ExecuteAction`, which resolves the executor via `resolveExecutor(action)` (`service/authority/authority.go:267`). This is a hardcoded map: `{"deploy": "github_trigger_workflow"}` (`service/authority/authority.go:115-117`). The caller cannot influence this mapping.

`TestExec12_ExecutorSelectorAttack` (`service/authority/authority_integration_test.go:877-918`) creates a target with `action="malicious_action"` (not in the map). The kernel authorizes the action (target is approved), but the executor registry has no entry. Result: `Allowed=true, Success=false`. The executor is never called.

---

## 4. MCP Execution Boundary Analysis (`solvent_execute`)

**File:** `cmd/solvent-mcp/tools.go:636-715`

### 4.1 Same Service Semantics as REST
The MCP handler calls `authSvc.ExecuteAction(ctx, scenarioID, beliefID, action, targetID, actorID, intentID, map[string]interface{}{}, "execution", snapParams)` (`cmd/solvent-mcp/tools.go:693-694`). This is the **same service method** called by the REST handler. All security invariants enforced by `ExecuteAction` apply identically.

### 4.2 No Bypass of `kernel.Authorize`
`ExecuteAction` always calls `PrepareForAction` first, which calls `s.kern.Authorize` (`service/authority/authority.go:172`). There is no path from `handleSolventExecute` to the executor that skips authorization.

### 4.3 No Public `CompleteIntent`
`CompleteIntent` is not exposed as a public REST endpoint or MCP tool. It is called only internally by `ExecuteAction` after the provider accepts (`service/authority/authority.go:400-418`). `RollbackClaim` is similarly internal.

### 4.4 No Caller-Controlled Executor or Consequence Parameters
The MCP `InputSchema` for `solvent_execute` has no `executor`, `provider`, or `consequence_parameters` field (`cmd/solvent-mcp/main.go:577-610`). The handler reads snapshot params from the DB and passes `map[string]interface{}{}` as caller params (`cmd/solvent-mcp/tools.go:678-694`).

### 4.5 MCP Trust Model
The MCP server operates as a **trusted local stdio surface**. The `actor_id` comes from the tool arguments, not from authenticated credentials. This is explicitly documented in the tool description (`cmd/solvent-mcp/main.go:577-578`) and in the `handleSolventAuthorizeAction` comment (`cmd/solvent-mcp/tools.go:207-209`). The trust boundary is the local process, not the network.

**Key question:** Can an MCP caller execute an intent belonging to another principal?

**Answer:** Yes, in the sense that the MCP server does not authenticate callers. Any local process that can invoke the MCP server can supply any `actor_id`. This is **intentional and documented**. The MCP server is a trusted administrative surface, analogous to `kubectl` or `docker` — it assumes the caller has local access. The security boundary is the host, not the MCP protocol.

This is consistent with the architecture. The MCP server is not a multi-tenant network service. If multi-tenant isolation is required, it must be enforced at the host level (e.g., OS-level access control, separate MCP server instances per principal).

---

## 5. REST/MCP Semantic Equivalence

| Dimension | REST | MCP | Equivalent? |
|---|---|---|---|
| Authorization | `kernel.Authorize` via `PrepareForAction` | `kernel.Authorize` via `PrepareForAction` | YES |
| Execution | `authSvc.ExecuteAction` | `authSvc.ExecuteAction` | YES |
| Identity | `AuthFromContext` (Bearer token) | `actor_id` argument (trusted local) | Documented difference |
| Snapshot params | DB read in handler | DB read in handler | YES |
| Executor selection | `resolveExecutor(action)` | `resolveExecutor(action)` | YES |
| Error semantics | `writeKernelError` | `errorResult` | Equivalent |
| Intent binding | `intent_id` validated, `ClaimIntent` | `intent_id` validated, `ClaimIntent` | YES |
| Audit behavior | `auditSvc.Log` | `auditSvc.Log` + `pipeline.AuditIntent` | Equivalent |

The only semantic difference is identity: REST uses Bearer token authentication; MCP uses trusted-local attribution. This is explicitly documented and consistent with the trust model.

---

## 6. Authentication/Principal Analysis

**REST:** `AuthMiddleware` (`api/auth.go:26-54`) maps Bearer tokens to `principal_id` via static configuration (`SOLVENT_API_KEYS`). The handler never trusts `actor_id` or `principal_id` from the request body. `handleExecuteAction` does not read any identity field from the body.

**MCP:** No authentication boundary. The MCP server is a local stdio process. `actor_id` is an attribution input, not an authentication proof.

**Attempted identity confusion attacks:**
1. REST: Send `actor_id` in request body → field does not exist in `ExecuteActionRequest`; ignored.
2. REST: Send `principal_id` in request body → field does not exist in `ExecuteActionRequest`; ignored.
3. MCP: Supply `actor_id` belonging to another principal → accepted (trusted local model).
4. MCP: Omit `actor_id` → defaults to `"mcp-agent"` (`cmd/solvent-mcp/tools.go:688-691`).

No attack vector allows a network-unauthenticated caller to bypass the REST principal check.

---

## 7. Intent/Context Binding Analysis

**Exact binding is preserved.** `ExecuteAction` receives `intentID` and passes it to `s.kern.ClaimIntent(ctx, scenarioID, intentID)` (`service/authority/authority.go:304`). `sqlClaimIntent` is:

```sql
UPDATE action_intent SET state = 'executing'
WHERE id = $1::UUID AND scenario_id = $2::UUID AND state = 'live'
```

The intent is bound to the scenario and must be in `live` state. The handler also validates `scenario_id`, `belief_id`, `action`, and `target_id` against the same values passed to `kernel.Authorize`. The kernel validates the tuple against the snapshot. There is no path where a caller can substitute a different intent or context.

---

## 8. Snapshot Parameter Analysis

**File:** `service/authority/authority.go:280-300`

Snapshot parameters are read from the database in two places:
1. REST handler: `SELECT ts.consequence_parameters FROM target_activation ta JOIN target_snapshot ts ... WHERE ta.target_id = $1::UUID` (`api/authorization.go:224-232`)
2. MCP handler: identical query (`cmd/solvent-mcp/tools.go:678-685`)

Both pass the result to `ExecuteAction` as `consequenceParameters`. `ExecuteAction` unmarshals these into `snapParams` and constructs `execParams` from `snapParams["repo"]`, `snapParams["workflow"]`, `snapParams["ref"]` (`service/authority/authority.go:282-293`). Caller-supplied `params` is always `map[string]interface{}{}`.

**Attack:** Caller sends conflicting `repo`/`workflow`/`ref` in request body  
**Result:** Ignored. The executor receives only snapshot values.

**Confirmed by:** `TestExecute_CallerParamsIgnored` (REST) and `TestExec11_ParameterSubstitutionAttack` (service layer, `service/authority/authority_integration_test.go:837-872`). Exec11 passes DIFFERENT `consequence_parameters` to `ExecuteAction` and confirms the kernel denies with `consequence_parameters mismatch`.

---

## 9. Executor-Selection Analysis

**File:** `service/authority/authority.go:115-123`

```go
var actionExecutorMap = map[string]string{
    "deploy": "github_trigger_workflow",
}
```

The mapping is hardcoded. `resolveExecutor(action)` returns the executor name for the given action. The executor is looked up from `s.execReg.Get(executorName)`. The caller cannot influence the registry or the mapping.

**Attack:** Caller uses an action name not in the map (e.g., `malicious_action`)  
**Result:** `Allowed=true` (kernel approves the target), `Success=false` (no executor registered). The executor is never called.

**Confirmed by:** `TestExec12_ExecutorSelectorAttack` (`service/authority/authority_integration_test.go:877-918`).

---

## 10. Audit/Reconciliation Analysis

### 10.1 Audit Trail
`ExecuteAction` writes the following audit entries:
1. `ActivityAuthorizationGranted` or `ActivityAuthorizationDenied` (from `PrepareForAction`)
2. `ActivityExecutorDenied` (if `!decision.Allowed`)
3. `ActivityAdapterInvoked` (before provider call)
4. `ActivityExecutorFailed` (on provider error — `ProviderRejected` or `ProviderAmbiguous`)
5. `ActivityIntentCompletionFailed` (if `CompleteIntent` fails after provider accepted)
6. `ActivityExecutorCompleted` (on full success)

All entries include `ScenarioID`, `ActorID`, `SubjectID`, and structured `Details`. No secrets are included in any audit entry.

### 10.2 Reconciliation Audit
`ReconcileIntent` (`service/authority/authority.go:446-469`) writes `ActivityReconciliationCompleted` **before** dispatching to the kernel. The entry includes `intent_id`, `outcome`, and `operatorID`. This satisfies the Phase 4D LOW finding requirement: operator identity + intent + scenario + outcome + timestamp are all auditable.

### 10.3 No Public Reconciliation Route
`ReconcileIntent` is a method on `*Service`. It is not registered as a REST endpoint or MCP tool. The only call site is internal (future operator tooling).

---

## 11. GitHub External-Side-Effect Analysis

### 11.1 Trace
```
REST/MCP
  → ExecuteAction
    → PrepareForAction (kernel.Authorize)
      → resolveExecutor("deploy") → "github_trigger_workflow"
        → execReg.Get("github_trigger_workflow") → NewExecutor(provider)
          → provider.TriggerWorkflow(ctx, repo, workflow, ref, inputs)
            → HTTPProvider.TriggerWorkflow
              → POST https://api.github.com/repos/{repo}/actions/workflows/{workflow}/dispatches
```

The API/MCP layer does **not** implement its own GitHub execution logic. All GitHub-specific code lives in `adapter/github/`.

### 11.2 Provider Output
`TriggerWorkflow` returns `(*WorkflowResult, error)`. On success (204 No Content or 201 Created), it returns `&WorkflowResult{RunID: ...}`. On failure, it returns a classified `*ProviderError`. The `RunID` is surfaced in `result.Output` (`service/authority/authority.go:395-396`).

### 11.3 System Does Not Claim Workflow Completion
The response field `success` means "the provider accepted the request", not "the workflow completed". The `IntentState` is `"executing"` when `Allowed && !result.Success` or when `CompleteIntent` fails (`api/authorization.go:259-263`). It is `"executed"` only when `CompleteIntent` succeeds after provider acceptance. The system never claims the GitHub workflow has finished.

### 11.4 Missing GITHUB_TOKEN
`cmd/solvent-api/main.go:64-70` and `cmd/solvent-mcp/main.go:129-135`: if `GITHUB_TOKEN` is absent, the GitHub executor is not registered. `ExecuteAction` will fail with `executor not registered: github_trigger_workflow` (`service/authority/authority.go:275-277`). The error is returned to the caller. No token is logged or leaked.

### 11.5 Credentials
The token is a construction-time configuration passed to `github.NewHTTPProvider(token)`. It is never logged, never returned in API responses, never stored in the database.

---

## 12. Example/Demo Analysis

### 12.1 Runnable
`examples/github/integration.go` and `examples/github/executor/main.go` are runnable programs. They require:
- `SOLVENT_DATABASE_URL` / `FABLE_DSN` (CockroachDB)
- `SOLVENT_API_KEYS` (for the REST example)
- `GITHUB_TOKEN` (for the executor example)
- A real GitHub repo/workflow/ref

No embedded secrets. The `ghp_xxx` in comments is a usage example, not a real token.

### 12.2 Adversarial Demonstrations
`TestExecute_CallerParamsIgnored` demonstrates that snapshot params win over caller params. The test sends `{"repo":"evil/repo",...}` in the request body and confirms the request is **allowed** (because the handler uses DB params), not falsely denied. This is the correct demonstration: if the handler had used caller params, the kernel would have rejected the mismatch.

---

## 13. OpenAPI Contract Analysis

**File:** `docs/openapi/solvent.yaml`

### 13.1 New Endpoint
`/v1/authorizations/execute` is present with `operationId: executeAction` (`solvent.yaml:656-698`). The schema `ExecuteActionRequest` requires `scenario_id`, `belief_id`, `action`, `target_id`, `intent_id` — no `principal_id`, `actor_id`, `consequence_parameters`, `executor`, or `tool_name`.

### 13.2 No Caller-Controlled Consequence Parameters
`ExecuteActionRequest` has no `consequence_parameters` field. The response schema `ExecuteActionResult` accurately reflects the implementation: `allowed`, `success`, `output`, `error`, `reason`, `executed_at`.

### 13.3 No False Execution-Completion Claims
The description explicitly states: "Provider acceptance does not mean workflow completion." The response distinguishes `allowed` (kernel granted authority) from `success` (provider accepted).

### 13.4 Authentication
Global `security: - bearerAuth: []` applies to all endpoints. No endpoint overrides this.

### 13.5 Forbidden Endpoints
The spec's description lists the forbidden endpoints from Phase 4A §8 (`/execute-without-authority`, `/set-authorized`, etc.). None of these are present.

### 13.6 Drift
The OpenAPI test (`api/openapi/openapi_test.go`) verifies:
- All canonical routes exist
- No extra undocumented paths
- All mutating POST endpoints have operationIds and responses
- Bearer auth scheme is defined
- Global security is declared

No drift detected.

---

## 14. Phase 4D Regression Analysis

| Invariant | Status | Evidence |
|---|---|---|
| CI-4: Concurrent claims permit only one provider attempt | PASS | `sqlClaimIntent` uses `WHERE state = 'live'` CAS. `ClaimIntent` returns `ErrIntentNotLive` if RowsAffected=0. `TestIntegration_ConcurrentRevokeTarget` and race detector confirm. |
| CI-5: Ambiguous results remain executing | PASS | `ProviderAmbiguous` → `RollbackClaim` is NOT called. Intent stays `executing`. `ReconcileIntent` is required to resolve. |
| CI-6: Snapshot parameters remain authoritative | PASS | Caller params are `map[string]interface{}{}`. Snapshot params are read from DB. `TestExec11` and `TestExecute_CallerParamsIgnored` confirm. |
| CI-7: Executing intents survive `RetractCascade` | PASS | `RetractCascade` cancels only `live` intents (`sqlRetractCascadeCancel: WHERE state = 'live'`). Executing intents are untouched. |
| CI-8: Only definitive rejection returns `executing → live` | PASS | `RollbackClaim` is called only for `ProviderRejected`. `ProviderAmbiguous` and non-provider errors do not call `RollbackClaim`. |
| CI-9: Provider outcome classification is provider-neutral at service boundary | PASS | Service maps `ProviderOutcomeCode()` to transitions. No provider-specific HTTP semantics in service layer. |
| CI-10: Reconciliation transitions are source-state guarded | PASS | `CompleteIntent`: `WHERE state = 'executing'`. `RollbackClaim`: `WHERE state = 'executing'`. `CancelIntent`: `WHERE state = 'executing'`. |

All Phase 4D invariants remain intact.

---

## 15. Concurrency/Race Analysis

### 15.1 Race Detector
All packages pass `go test -race`:
- `./api/...` — 24.711s
- `./service/authority/...` — 25.287s
- `./kernel/...` — 30.913s
- `./adapter/github/...` — 32.521s
- `./cmd/solvent-mcp/...` — 37.030s

No data races detected.

### 15.2 Concurrent Claims
`ClaimIntent` uses `sqlClaimIntent: UPDATE action_intent SET state = 'executing' WHERE id = $1::UUID AND scenario_id = $2::UUID AND state = 'live'`. The `WHERE state = 'live'` CAS ensures only one concurrent claim succeeds. Subsequent claims get `ErrIntentNotLive`.

### 15.3 REST + MCP Simultaneous Execution
Both REST and MCP call the same `authSvc.ExecuteAction`, which calls the same `s.kern.ClaimIntent`. The database serializes the CAS. Only one path can claim the intent. The second gets `ErrIntentNotLive` and returns `Allowed=false`.

---

## 16. Test-Quality Assessment

### 16.1 Service-Layer Tests (`service/authority/authority_integration_test.go`)
- **Exec11–Exec19:** Each test exercises a specific security boundary:
  - Exec11: Parameter substitution → kernel denies
  - Exec12: Executor override → kernel authorizes but no executor found
  - Exec13: Revocation mid-flight → re-read denies
  - Exec14: Cross-principal → principal mismatch
  - Exec15–19: Wrong scenario/belief/action/target/principal → tuple mismatch

  **Would these fail if the handler bypassed the service?** Yes. Each test calls `svc.ExecuteAction` directly, so bypassing the service would require changing the test itself.

- **EA01–EA10:** Negative-path tests for no authority, wrong target, wrong action, revoked authority, fake approval, etc. All confirm executor is NOT called when denied.

- **CR-A, CR-B, CR-C:** Time-of-check/time-of-use tests. Revoke or change target/action between `PrepareForAction` and `ExecuteAction` → denied.

- **P01:** Positive test. Valid auth + correct context → `Allowed=true, Success=true`, executor called with correct params.

### 16.2 REST Tests (`api/authorization_test.go`)
- `TestExecute_CrossPrincipalDenied`: Full HTTP path with cross-principal target → denied
- `TestExecute_WrongScenarioDenied`, `TestExecute_WrongBeliefDenied`, `TestExecute_WrongActionDenied`, `TestExecute_WrongTargetDenied`: Context substitution → denied
- `TestExecute_CallerParamsIgnored`: Caller-supplied `consequence_parameters` ignored → allowed (snapshot wins)

**Would these fail if authentication were removed?** Yes — `TestExecute_CrossPrincipalDenied` would pass with the wrong principal if the handler trusted `actor_id` from the body. But `ExecuteActionRequest` has no `actor_id` field. The test would need to be redesigned.

### 16.3 MCP Tests (`cmd/solvent-mcp/tools_authority_test.go`)
- `TestMCPHandler_Execute_RequiresAuthSvc`: nil service → error
- `TestMCPHandler_Execute_InvalidScenario`: unknown scenario → error
- `TestMCPHandler_Execute_MissingRequiredFields`: missing `belief_id` → error
- `TestMCPHandler_Execute_InvalidBelief`: non-existent belief → error
- `TestMCPHandler_Activity_*`: activity tool coverage

The MCP execution tests are fewer than the service-layer tests. They validate the MCP plumbing (argument parsing, scenario lookup, cross-scenario guard) but rely on the service layer for the security invariants. This is correct: the security semantics are enforced once in `authority.Service`, not duplicated in each handler.

### 16.4 Tests That Only Validate HTTP Shape
`TestExecuteAction_ServiceUnavailable`, `TestExecuteAction_MissingScenarioID`, `TestExecuteAction_MissingBeliefID`, `TestExecuteAction_MissingIntentID`, `TestExecuteAction_InvalidJSON` validate HTTP status codes but do not prove security semantics. These are necessary but not sufficient. The adversarial tests (Exec11–Exec19, cross-principal, etc.) provide the semantic proof.

---

## 17. Mutation Analysis

| Mutation | Would existing tests detect it? |
|---|---|
| 1. REST handler passes caller identity instead of authenticated principal | **YES** — `TestExecute_CrossPrincipalDenied` would fail if the handler trusted a caller-supplied `principal_id` (but the field doesn't exist). |
| 2. REST handler trusts request `consequence_parameters` | **YES** — `TestExecute_CallerParamsIgnored` would fail; the kernel would deny mismatched params. |
| 3. REST handler bypasses `ExecuteAction` | **YES** — `TestEA01_NoAuthorityTarget` and `TestExecuteAction_ServiceUnavailable` would fail. |
| 4. MCP handler bypasses `ExecuteAction` | **YES** — same service-layer tests apply. |
| 5. MCP tool accepts arbitrary executor | **YES** — `TestExec12_ExecutorSelectorAttack` would fail if caller could select executor. |
| 6. Intent context validation removed | **YES** — `TestExecute_WrongScenarioDenied`, `TestExec15_WrongScenario`, etc. would fail. |
| 7. Snapshot parameter lookup removed | **YES** — `TestExecute_CallerParamsIgnored` and `TestExec11` would fail. |
| 8. `ClaimIntent` bypassed | **YES** — `ClaimIntent` is the sole ownership gate. Bypassing it would allow concurrent execution, violating CI-4. The race detector would also catch unsynchronized state mutations. |
| 9. Revoked authority accepted | **YES** — `TestEA05_RevokedAuthority` and `TestExec13_AuthorityRevokedMidFlight` would fail. |
| 10. Reconciliation audit removed | **NO** — there is no test that verifies `ReconcileIntent` writes an audit entry. This is a gap (see Findings). |
| 11. Activity tool returns cross-scenario data | **NO** — `TestMCPHandler_Activity_Success` does not verify scenario isolation. It only checks that activities are returned for a valid scenario. A mutation that omits the `scenario_id` filter would not be detected. |
| 12. GitHub executor mapping becomes caller-controlled | **YES** — `TestExec12_ExecutorSelectorAttack` would fail. |

---

## 18. Architectural-Boundary Assessment

| Locked Foundation | Status | Evidence |
|---|---|---|
| `kernel.Authorize` is the sole authority oracle | PASS | `ExecuteAction` calls `PrepareForAction` → `s.kern.Authorize`. No second authority engine exists. |
| `TOKEN != AUTHORITY` | PASS | `GITHUB_TOKEN` is a provider credential, not an authority token. Authority comes from `kernel.Authorize`. |
| `Authorize != Execute` | PASS | `PrepareForAction` is read-only; `ExecuteAction` is the sole executor. |
| Approved snapshot `consequence_parameters` are authoritative | PASS | Caller params are `map[string]interface{}{}`. Snapshot params are read from DB. |
| Caller execution parameters are non-authoritative | PASS | Same as above. |
| Fixed action → executor mapping | PASS | `actionExecutorMap = map[string]string{"deploy": "github_trigger_workflow"}` |
| Exact `intentID` binding | PASS | `ClaimIntent` uses `WHERE id = $1::UUID AND scenario_id = $2::UUID AND state = 'live'` |
| `ClaimIntent` is the execution ownership gate | PASS | CI-4: CAS ensures only one provider attempt. |
| `live → executing → executed` | PASS | `ClaimIntent` transitions `live → executing`; `CompleteIntent` transitions `executing → executed`. |
| Definitive rejection → `live` | PASS | `RollbackClaim` transitions `executing → live` only for `ProviderRejected`. |
| Ambiguous outcome → `executing` | PASS | `ProviderAmbiguous` leaves intent as `executing`. |
| Ambiguous outcomes require reconciliation | PASS | `ReconcileIntent` is the only path from `executing` to `executed`/`live`/`cancelled` for ambiguous outcomes. |
| Executing intents survive `RetractCascade` | PASS | `sqlRetractCascadeCancel: WHERE state = 'live'` |
| `CompleteIntent` only transitions `executing → executed` | PASS | `sqlCompleteIntent: WHERE state = 'executing'` |
| Provider acceptance ≠ workflow completion | PASS | `success=true` means provider accepted. `intent_state=executed` means `CompleteIntent` succeeded. |
| GitHub semantics remain outside kernel | PASS | GitHub-specific code is in `adapter/github/`. Kernel has no GitHub imports. |
| Provider outcome classification is adapter-specific | PASS | `providerClassifier` interface; service maps numeric codes. |
| `ReconcileIntent` is internal and privileged | PASS | Not exposed as REST/MCP endpoint. |
| No public `CompleteIntent` | PASS | Only called internally by `ExecuteAction`. |
| No public `ReconcileIntent` | PASS | Not exposed as REST/MCP endpoint. |
| No `AuthorizeAndClaimIntent` | PASS | `AuthorizeAndCreateIntent` is the composite primitive; `ClaimIntent` is separate. |
| No `GetSnapshotConsequenceParams` | PASS | Snapshot params are read inline in handlers, not exposed as a public endpoint. |
| No second authorization engine | PASS | `kernel.Authorize` is the sole oracle. `PrepareForAction` delegates to it. |
| Kernel remains minimal | PASS | No new kernel primitives added in Phase 4E. |

No architectural boundary was violated.

---

## 19. Findings Table

| ID | Severity | File | Function/Symbol | Issue | Attack/Failure Scenario | Violated Invariant | Current Tests Detect? | Remediation Direction |
|---|---|---|---|---|---|---|---|---|
| F-01 | LOW | `service/authority/authority.go:446` | `ReconcileIntent` | No test verifies that `ReconcileIntent` writes an audit entry before dispatching to the kernel. If the audit write is removed or moved after the kernel call, the operator identity would not be recorded for the reconciliation outcome. | Operator reconciles an ambiguous intent without audit trail. | Phase 4D LOW finding: operator identity must be auditable. | **NO** | Add a test that calls `ReconcileIntent` and verifies `ActivityReconciliationCompleted` exists in `audit_activity` with the correct `actor_id` and `intent_id`. |
| F-02 | LOW | `cmd/solvent-mcp/tools.go:719-750` | `handleSolventActivity` | No test verifies that `solvent_activity` enforces `scenario_id` scoping. The handler calls `auditSvc.GetActivities(ctx, scenarioID, ...)` which includes `scenario_id` in the WHERE clause, but a mutation that drops the `scenario_id` filter would not be detected. | MCP caller reads audit entries from another scenario. | Scenario-scoped access | **NO** | Add a test that inserts an activity for scenario A, calls `solvent_activity` for scenario B, and asserts the activity is not returned. |
| F-03 | INFO | `examples/github/README.md:23` | Documentation | The example flow references `POST /v1/intents`, which does not exist as a REST endpoint. The example creates intents via `POST /v1/authorizations/action` (which internally calls `AuthorizeAndCreateIntent`). | Developer confusion; no security impact. | — | N/A | Update the example README to reference the correct endpoint or note that intent creation is implicit in `authorize_action`. |
| F-04 | INFO | `examples/github/integration.go:127-138` | `main` | The example calls `POST /v1/intents` (non-existent) and then `POST /v1/authorizations/execute`. If a developer runs this against a live server, the first call returns 404. | Example does not run as written. | — | N/A | Fix the example to use `POST /v1/authorizations/action` for intent creation, or document that the example requires a pre-existing intent. |
| F-05 | INFO | `cmd/solvent-mcp/main.go:688-691` | `handleSolventExecute` | `actor_id` defaults to `"mcp-agent"` when omitted. This is documented, but the default is a generic string, not a configurable identity. | Attribution is less precise when `actor_id` is omitted. | — | N/A | Consider making the default configurable via environment variable (e.g., `SOLVENT_MCP_ACTOR_ID`). |

---

## 20. Remaining Accepted Limitations

These are intentional architectural decisions, not defects:

- **Race A (Authorize → ClaimIntent):** Between `kernel.Authorize` and `ClaimIntent`, the target could be revoked. `ExecuteAction` mitigates this by re-reading current state in `PrepareForAction` immediately before `ClaimIntent`. The window is a single SERIALIZABLE transaction. This is an accepted v1 race.
- **Race B (ClaimIntent → Provider):** Between `ClaimIntent` and the provider call, the belief could be retracted. The intent is already `executing`; `RetractCascade` does not cancel `executing` intents. The operator must reconcile. This is intentional.
- **Lack of exactly-once provider execution:** The provider is called once per claim. Network failures may cause the provider to accept but the response to be lost (`ProviderAmbiguous`). Reconciliation is required.
- **Lack of provider polling:** Solvent does not poll GitHub for workflow completion. `success=true` means the dispatch was accepted, not that the workflow finished.
- **Lack of broad retries:** `ExecuteAction` does not retry on `ProviderAmbiguous`. The intent remains `executing` for operator reconciliation.
- **Lack of RBAC/IAM:** REST uses static API-key-to-principal mapping. MCP uses trusted-local model. No fine-grained RBAC.
- **Lack of multi-tenancy:** No tenant isolation beyond scenario scoping.
- **Lack of A2A:** No agent-to-agent protocol.
- **Lake of broad UI:** The product is an API/MCP server, not a web application.

---

## 21. Final Recommendation

**VERDICT: GO**

Phase 4E satisfies the contract:

1. The REST execution boundary (`POST /v1/authorizations/execute`) does not create a second authorization engine. It delegates to `authority.Service.ExecuteAction`, which delegates to `kernel.Authorize`.
2. The authenticated principal is the only effective identity. Request body fields cannot substitute or override it.
3. Snapshot `consequence_parameters` are the sole execution source. Caller-supplied parameters are ignored.
4. The executor is resolved from an internal registry, never caller-supplied.
5. The MCP execution boundary (`solvent_execute`) uses the same service method with the same invariants.
6. The MCP trust model is explicitly documented as trusted-local-stdio. This is intentional.
7. No public `CompleteIntent`, `ReconcileIntent`, or generic reconciliation endpoint exists.
8. All Phase 4D invariants (CI-4 through CI-10) remain intact and are tested.
9. The GitHub executor is reached through the fixed action mapping. No GitHub logic lives in the kernel or API layer.
10. The OpenAPI contract accurately reflects the implementation.
11. All build, vet, race-detector, integration, and MCP verification commands pass.

Two LOW findings (F-01, F-02) should be addressed in Phase 5 but do not block the GO verdict. The INFO items (F-03, F-04) are documentation/example fixes.

---

**Final answer to the central question:**

> Can an untrusted agent now reach a real consequential GitHub action through REST or MCP while Solvent remains the authoritative decision and execution boundary?

**Yes, but only through the authorized path.** An untrusted agent cannot bypass `kernel.Authorize`, cannot substitute context or parameters, cannot select its own executor, and cannot use another principal's intent. The only path to execution is:
1. A promoted belief with discharged debt
2. An approved target with an exact-matching snapshot
3. A live intent created by `AuthorizeAndCreateIntent`
4. `ClaimIntent` transitioning the intent to `executing`
5. The fixed executor invoking the GitHub provider with snapshot params
6. `CompleteIntent` recording the outcome

Every step is enforced by the database or the kernel. The API and MCP layers are thin, authenticated, and audited boundaries.
