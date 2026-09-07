# PHASE 4E — END-TO-END AGENT WORKFLOW + PRODUCTION HARDENING

## PLAN ONLY — DO NOT IMPLEMENT

Phase 4C+ and Phase 4D are COMPLETE and independently passed adversarial review.

Phase 4D final verdict: **GO**

The current Solvent authority/execution architecture is now trusted.

The purpose of Phase 4E is NOT to redesign the kernel.

The purpose is to turn the proven authority → intent → execution architecture
into a coherent, usable, end-to-end agent workflow using the real GitHub
integration, while applying a small amount of production hardening.

---

## LOCKED FOUNDATIONS

Treat these as fixed unless a genuine security defect is discovered:

- kernel.Authorize is the sole authority oracle
- TOKEN != AUTHORITY
- Authorize != Execute
- approved snapshot consequence_parameters are authoritative
- caller execution parameters do not determine consequential provider params
- fixed authorized-action → executor mapping
- exact intentID binding
- ClaimIntent is the atomic execution-ownership gate
- live → executing → executed
- definitive provider rejection → live
- ambiguous provider outcome → executing
- ambiguous outcomes require reconciliation
- executing intents survive RetractCascade
- CompleteIntent only transitions executing → executed
- provider acceptance does not mean workflow completion
- GitHub semantics remain outside kernel
- provider outcome classification remains adapter-specific
- ReconcileIntent remains internal and privileged
- no public CompleteIntent endpoint
- no public generic reconciliation endpoint
- no AuthorizeAndClaimIntent
- no GetSnapshotConsequenceParams
- no second authorization engine
- kernel remains minimal

Accepted v1 residual races remain:

    Race A: Authorize → ClaimIntent
    Race B: ClaimIntent → Provider

Do NOT attempt to eliminate these in Phase 4E unless a concrete production
requirement or security incident establishes that the current design is
insufficient.

---

## 1. Phase 4E Objective

Turn the proven authority → intent → execution architecture into a coherent,
usable, end-to-end agent workflow using the real GitHub integration, while
applying targeted production hardening.

The canonical experience becomes:

```
evidence / agent reasoning
    ↓
propose consequential action
    ↓
create/identify target
    ↓
obtain approval
    ↓
authorize (creates live intent)
    ↓
execute (claims intent → invokes GitHub → records outcome)
    ↓
audit trail explains what happened
```

---

## 2. Current End-to-End Workflow Assessment

### What exists today

| Component | Status | Gap |
|-----------|--------|-----|
| Evidence ingestion | ✅ REST + MCP | Complete |
| Belief lifecycle (enter → retire debt → promote) | ✅ REST + MCP | Complete |
| Target CRUD | ✅ REST + MCP | Complete |
| Target approval → activation → snapshot | ✅ REST + MCP | Complete |
| Authorization verification (read-only) | ✅ REST + MCP | Complete |
| Authorization + intent creation (atomic) | ✅ REST + MCP | Complete |
| **Execution (claim → invoke → complete)** | **Service only** | **No REST endpoint, no MCP tool** |
| Activity/audit query | ⚠️ REST only | `GET /v1/activity` exists; no MCP tool |
| Reconciliation | Service only | No API surface (correct for v1 — internal) |
| GitHub provider acceptance display | ❌ | RunID returned but not surfaced |

### Critical architectural gap

Neither the REST API nor the MCP server currently exposes an **execution
endpoint**. The `authority.Service.ExecuteAction` method exists and is proven,
but is not wired to any API surface. The existing MCP tool
`solvent_authorize_action` only creates a live intent — it does not claim and
execute it. The existing example (`examples/github/executor/main.go`) calls
`svc.ExecuteAction` directly, bypassing any API surface.

Phase 4E must close this gap.

### What Phase 4E must build

1. **REST execution endpoint** — `POST /v1/authorizations/execute` that calls
   `authority.Service.ExecuteAction`
2. **MCP execution tool** — `solvent_execute` that calls
   `authority.Service.ExecuteAction`
3. **Activity/audit MCP tool** — expose `GET /v1/activity` as an MCP tool
4. **Reconciliation audit fix** — record operatorID in reconciliation path
5. **Flagship example** — `examples/github/` showing the complete flow
6. **End-to-end acceptance tests** — covering happy path, security demos,
   semantic equivalence
7. **Production hardening** — credential validation, error messages, startup
   behavior

---

## 3. Flagship GitHub Deployment Workflow

**Scenario:** An AI agent proposes deploying etcd v3.5.x to production. Solvent
— not the AI — decides whether the exact consequential action is authorized.

**Target repository:** A safe test repository with a `workflow_dispatch`-triggered
workflow (e.g., `.github/workflows/solvent-demo.yml` that does
`echo "Deployed by Solvent"` and exits).

**Flow:**

```
1. Agent gathers evidence from GitHub issues
   → POST /v1/evidence (or MCP solvent_ingest_evidence)

2. Agent proposes deployment belief
   → POST /v1/beliefs (claim: "etcd v3.5.x is safe to deploy")

3. Agent retires all six debt items
   → POST /v1/beliefs/{id}/debt/retire × 6

4. Agent promotes belief
   → POST /v1/beliefs/{id}/promote

5. Agent creates authority target with frozen parameters
   → POST /v1/targets
     consequence_parameters: {"repo":"org/test","workflow":"deploy.yml","ref":"main"}

6. Agent attaches justification (the promoted belief)
   → POST /v1/targets/{id}/justifications

7. Agent requests authorization (pins snapshot)
   → POST /v1/targets/{id}/request

8. Human operator approves
   → POST /v1/targets/{id}/approve

9. Agent authorizes action (creates live intent)
   → POST /v1/authorizations/action (REST)
   → MCP solvent_authorize_action

10. Agent executes (claims intent → invokes GitHub → records outcome)
    → POST /v1/authorizations/execute (REST)  [NEW]
    → MCP solvent_execute                       [NEW]

11. Audit shows complete chain
    → GET /v1/activity (REST)
    → MCP solvent_activity                      [NEW]
```

---

## 4. REST Lifecycle

### Existing endpoints (no changes needed)

| Step | Method | Route | Handler | Status |
|------|--------|-------|---------|--------|
| Enter belief | POST | `/v1/beliefs` | `handleEnterBelief` | ✅ |
| Retire debt | POST | `/v1/beliefs/{id}/debt/retire` | `handleRetireDebt` | ✅ |
| Promote | POST | `/v1/beliefs/{id}/promote` | `handlePromoteBelief` | ✅ |
| Create target | POST | `/v1/targets` | `handleCreateTarget` | ✅ |
| Attach justification | POST | `/v1/targets/{id}/justifications` | `handleAttachJustification` | ✅ |
| Request authorization | POST | `/v1/targets/{id}/request` | `handleRequestAuthorization` | ✅ |
| Approve | POST | `/v1/targets/{id}/approve` | `handleApproveTarget` | ✅ |
| Authorize action | POST | `/v1/authorizations/action` | `handleAuthorizeAction` | ✅ |
| Activity | GET | `/v1/activity` | `handleGetActivity` | ✅ |

### New endpoints

| Step | Method | Route | Handler | Purpose |
|------|--------|-------|---------|---------|
| Execute | POST | `/v1/authorizations/execute` | `handleExecuteAction` | Claim intent → invoke provider → record outcome |

### `POST /v1/authorizations/execute` contract

**Request:**

```json
{
  "scenario_id": "uuid",
  "belief_id": "uuid",
  "action": "deploy",
  "target_id": "uuid",
  "intent_id": "uuid",
  "consequence_type": "execution"
}
```

**Response (success):**

```json
{
  "belief_id": "uuid",
  "intent_id": "uuid",
  "intent_state": "executed",
  "action": "deploy",
  "allowed": true,
  "success": true,
  "output": "RunID: 12345",
  "executed_at": "2026-09-07T12:00:00Z"
}
```

**Response (denied):**

```json
{
  "belief_id": "uuid",
  "action": "deploy",
  "allowed": false,
  "reason": "authority revoked",
  "success": false
}
```

**Response (provider rejected):**

```json
{
  "belief_id": "uuid",
  "intent_id": "uuid",
  "action": "deploy",
  "allowed": true,
  "success": false,
  "error": "403 Forbidden: insufficient permissions"
}
```

**Security:**

- `actor_id` derived from authenticated principal (same pattern as
  `handleAuthorizeAction`)
- Snapshot `consequence_parameters` read from DB, not from request body
- Executor resolved from internal registry, not caller-supplied
- `intent_id` required — must be a live intent created by
  `handleAuthorizeAction`

**Files to change:**

- `api/api.go` — add route registration, add `authSvc` to Server
- `api/authorization.go` — add `handleExecuteAction`
- `api/types.go` — add `ExecuteActionRequest` and `ExecuteActionResult`
- `docs/openapi/solvent.yaml` — add endpoint documentation

---

## 5. MCP Lifecycle

### Minimal tool sequence for flagship workflow

| Step | Tool | Inputs | State Created/Used | Read/Consequential | Approval Required | Side Effects |
|------|------|--------|-------------------|-------------------|------------------|-------------|
| 1 | `solvent_ledger` | scenario | Current state | Read-only | No | No |
| 2 | `solvent_authorize_action` | scenario, belief_id, action, action_source, target_id, actor_id | Live intent | Consequential | Yes (implicit: belief must be promoted, authority must exist) | Creates intent |
| 3 | `solvent_execute` | scenario, belief_id, action, target_id, intent_id | Executed intent | Consequential | Yes (implicit: authority must be valid) | Invokes GitHub provider |
| 4 | `solvent_activity` | scenario (optional: type) | Audit entries | Read-only | No | No |

### New MCP tools

| Tool | Purpose |
|------|---------|
| `solvent_execute` | Claim intent → invoke provider → record outcome |
| `solvent_activity` | Read audit activity entries for a scenario |

### `solvent_execute` contract

**Inputs:**

```json
{
  "scenario": "track1",
  "belief_id": "uuid",
  "action": "deploy",
  "target_id": "uuid",
  "intent_id": "uuid"
}
```

**Output envelope:**

```json
{
  "result": {
    "belief_id": "uuid",
    "intent_id": "uuid",
    "intent_state": "executed",
    "action": "deploy",
    "allowed": true,
    "success": true,
    "output": "RunID: 12345",
    "executed_at": "2026-09-07T12:00:00Z"
  },
  "audit": { "live_on_nonpromoted": 0 }
}
```

**Security invariants preserved:**

- No tool bypasses kernel authority (authority.Service.ExecuteAction calls
  kernel.Authorize)
- No tool exposes CompleteIntent
- No tool exposes ReconcileIntent
- No arbitrary executor selector (action→executor mapping is hardcoded)
- No caller-controlled provider parameters (snapshot params used)
- No duplicate authorization engine

**Files to change:**

- `cmd/solvent-mcp/main.go` — register new tools, add to toolHandler switch
- `cmd/solvent-mcp/tools.go` — add `handleSolventExecute` and
  `handleSolventActivity`

---

## 6. Proposed Service/API Contract Changes

### No changes to kernel

The kernel remains frozen. All new behavior is above the kernel boundary.

### No changes to authority.Service

`authority.Service.ExecuteAction` already has the correct signature and behavior.
The REST and MCP layers call it directly.

### REST Server changes

`api.Server` needs access to `authority.Service`. Currently `Server` holds
`*ledger.Service`. Add `*authority.Service`:

```go
type Server struct {
    db       *sql.DB
    ledger   *ledger.Service
    authSvc  *authority.Service  // NEW
    auditSvc *audit.Service
}
```

**File:** `api/api.go` — modify `NewServer` to accept `*authority.Service`

**File:** `cmd/solvent-api/main.go` — wire `authority.Service` into the server

### MCP changes

The MCP server already has `authSvc` as a package-level var. No structural
change needed — just add the new tool handler and register it.

---

## 7. Audit/Activity Improvements

### Existing audit types (complete)

The `audit.ActivityType` enum already covers the full execution lifecycle:

- `authorization_granted` / `authorization_denied` — from `PrepareForAction`
- `adapter_invoked` — from `ExecuteAction` step 7
- `executor_completed` / `executor_failed` — from `ExecuteAction` steps 9-11
- `executor_denied` — from `ExecuteAction` step 2
- `intent_completion_failed` — from `ExecuteAction` step 11 (persistence failure)

### Missing: reconciliation audit

`ReconcileIntent` currently does not log an audit entry. This is the Phase 4D
LOW finding.

**Fix:** Add an `ActivityReconciliationCompleted` audit type and log it in
`ReconcileIntent`:

```go
ActivityReconciliationCompleted ActivityType = "reconciliation_completed"
```

**Audit entry:**

```go
s.audit.Log(ctx, &audit.ActivityEntry{
    ScenarioID: scenarioID,
    Type:       audit.ActivityReconciliationCompleted,
    ActorID:    operatorID,
    SubjectID:  intentID,
    Details: map[string]interface{}{
        "intent_id":   intentID,
        "outcome":     outcome.String(),
        "scenario_id": scenarioID,
    },
})
```

**File:** `service/audit/audit.go` — add `ActivityReconciliationCompleted`
**File:** `service/authority/authority.go` — add audit logging in `ReconcileIntent`

### Missing: MCP activity tool

Add `solvent_activity` MCP tool to read audit entries. This makes the audit
trail accessible to agents.

---

## 8. Reconciliation Audit Fix

**The minimal fix:**

1. Add `ActivityReconciliationCompleted` to `service/audit/audit.go`
2. In `service/authority/authority.go` `ReconcileIntent`, log the audit entry
   before dispatching to kernel
3. The entry records: `operatorID`, `intentID`, `scenarioID`, `outcome`,
   `timestamp`

**Files:**

- `service/audit/audit.go:57` — add constant
- `service/authority/authority.go:446-457` — add audit log call

**No new tables. No new schema. No public API surface.** ReconcileIntent
remains internal and privileged.

---

## 9. GitHub Integration Experience

### Current state

The `HTTPProvider.TriggerWorkflow` already:

- Sends `POST /repos/{repo}/actions/workflows/{workflow}/dispatches`
- Returns `WorkflowResult{RunID: ""}` on 204, or parses `workflow_run.id` on 201
- Classifies errors as accepted/rejected/ambiguous

### What to improve

1. **Surface the run reference** — When GitHub returns a `workflow_run.id` in a
   201 response, the `ExecutionResult.Output` already contains it. Ensure the
   REST and MCP responses display it.

2. **Do not claim completion** — The current architecture correctly distinguishes
   "provider accepted" from "workflow completed." The response should say
   `success: true, output: "RunID: 12345"` — NOT "deployment completed."

3. **Graceful missing-GITHUB_TOKEN behavior** — Currently, if `GITHUB_TOKEN` is
   empty, the executor is not registered. The `ExecuteAction` method returns
   `no executor registered for action: deploy`. This is correct but the error
   message should be clear. Verify the message is already clear (it is:
   `"no executor registered for action: %s"`).

4. **No workflow polling** — Confirmed. The system does not poll. Provider
   acceptance is the terminal state from Solvent's perspective.

**Files to change:** None for provider hardening — the existing code is already
correct per Phase 4D review.

---

## 10. Security Demonstrations

Three explicit adversarial demonstrations using the same test architecture:

### Demo 1: Parameter substitution attack

**Attack:** Caller tries to change the approved repo/workflow/ref.
**Expected:** Snapshot wins — the execution params come from
`decision.ConsequenceParameters` (the approved snapshot), not from
caller-supplied params.

**Test:** In `ExecuteAction`, after `PrepareForAction` returns the snapshot, pass
different params. Verify the provider receives the snapshot params, not the
caller params.

**Already tested by:** The architecture in `authority.go:282-293` explicitly
constructs `execParams` from `snapParams`, ignoring caller params. But a
dedicated test makes this visible.

### Demo 2: Executor selector attack

**Attack:** Caller tries to select a different executor.
**Expected:** Fixed action→executor mapping wins. `resolveExecutor("deploy")`
always returns `"github_trigger_workflow"`.

**Already tested by:** `TestExec09`, `TestAT05` in the existing suite.

### Demo 3: Authority revoked before execution

**Attack:** Authority is revoked between authorize and execute.
**Expected:** `PrepareForAction` re-reads current state, `kernel.Authorize`
denies.

**Already tested by:** `TestEA05_RevokedAuthority`,
`TestCR_A_RevokedAfterPrepare`.

**For the flagship example:** Include these three scenarios in the example
output, showing the denial messages.

---

## 11. Developer Example

### `examples/github/` structure

```
examples/github/
├── README.md                    # Updated with full lifecycle
├── integration.go               # Existing REST API integration (update)
├── executor/
│   └── main.go                  # Existing direct-call example (update)
└── workflow/
    └── main.go                  # NEW: full end-to-end via REST API
```

### `examples/github/workflow/main.go`

A single Go program that demonstrates the complete lifecycle using only REST
API calls:

1. Create principal
2. Enter belief
3. Retire all debt items
4. Promote belief
5. Create target with frozen parameters
6. Attach justification
7. Request authorization
8. Approve (human step — simulated)
9. Authorize action (creates intent)
10. Execute action (invokes GitHub)
11. Query activity (show audit trail)
12. Attempt parameter substitution (denied)
13. Attempt unauthorized execution (denied)
14. Show audit trail with all entries

**Configuration:**

```bash
export SOLVENT_API_URL=http://localhost:8080
export SOLVENT_API_KEY=your-api-key
export GITHUB_TOKEN=ghp_xxx
export GITHUB_REPO=org/test-repo
export GITHUB_WORKFLOW=deploy.yml
export GITHUB_REF=main
```

**README.md** documents:

- Required environment variables
- Required GitHub repository/workflow setup
- What the workflow does
- What Solvent protects
- Expected success/failure behavior

---

## 12. End-to-End Acceptance Tests

### New test file: `service/authority/authority_e2e_test.go`

| Test ID | Description | Type |
|---------|-------------|------|
| E2E-01 | REST happy path: full lifecycle via HTTP | Integration |
| E2E-02 | MCP happy path: full lifecycle via MCP tools | Integration |
| E2E-03 | REST/MCP semantic equivalence: same authorization/execution behavior | Integration |
| E2E-04 | Snapshot parameter binding: caller params ignored | Security |
| E2E-05 | Unauthorized execution: no authority target | Security |
| E2E-06 | Wrong target: valid authority + wrong target ID | Security |
| E2E-07 | Revoked authority: authority revoked before execution | Security |
| E2E-08 | Malicious executor selector: hardcoded mapping wins | Security |
| E2E-09 | Audit completeness: all lifecycle events recorded | Audit |
| E2E-10 | Reconciliation audit: operator identity recorded | Audit |
| E2E-11 | Provider acceptance vs workflow completion: distinction clear | Integration |
| E2E-12 | Real GitHub provider integration (when GITHUB_TOKEN available) | Integration |

### Test infrastructure

Use existing patterns:

- `testdb.SuiteDSN("e2e")` for isolated database
- `executor.NewRecordingFunc` for fake executor
- `github.FakeGitHubProvider` for GitHub provider simulation
- Table-driven tests for parameter variations

---

## 13. Adversarial Tests

### In existing test files (extend)

**`service/authority/authority_integration_test.go`:**

- Add `TestExec11_ParameterSubstitutionAttack` — pass different
  repo/workflow/ref in caller params, verify snapshot params used
- Add `TestExec12_ExecutorSelectorAttack` — attempt to override executor via
  params
- Add `TestExec13_AuthorityRevokedMidFlight` — revoke between authorize and
  execute

**`cmd/solvent-mcp/tools_authority_test.go`:**

- Add `TestMCPHandler_ExecuteAction_HappyPath` — full MCP tool invocation
- Add `TestMCPHandler_ExecuteAction_NoAuthority` — denied by kernel
- Add `TestMCPHandler_ExecuteAction_RevokedAuthority` — revoked before execute

---

## 14. Production Hardening Items

| Item | Description | Priority | Files |
|------|-------------|----------|-------|
| Credential validation | Validate GITHUB_TOKEN format at startup (non-empty, starts with `ghp_` or `github_pat_`) | Medium | `cmd/solvent-mcp/main.go`, `cmd/solvent-api/main.go` |
| Configuration validation | Validate all required env vars at startup, fail fast with clear messages | Medium | `cmd/solvent-mcp/main.go`, `cmd/solvent-api/main.go` |
| Error messages | Ensure all error paths return actionable messages (no raw error dumps) | Low | `api/authorization.go`, `cmd/solvent-mcp/tools.go` |
| Timeout behavior | Verify 30s default timeout is appropriate; document `WithTimeout` option | Low | `adapter/github/http_provider.go` |
| Malformed provider responses | Verify `classifyGitHubError` handles unexpected status codes | Low | `adapter/github/http_provider.go` |
| Startup behavior | Log registered executors, database status, configuration summary | Low | `cmd/solvent-mcp/main.go`, `cmd/solvent-api/main.go` |
| Audit safety | Verify audit log failures don't abort the request (already: audit failures are logged but don't propagate) | Verify | `service/authority/authority.go` |
| Graceful missing token | Already handled: executor not registered, clear error on execution attempt | Verify | `cmd/solvent-mcp/main.go:126-132` |

---

## 15. UI Decision

**No UI expansion.** The API/MCP workflow is already understandable to both
agents and humans. The existing demo cloud web app (`demo/cloud/`) remains as-is.
No new UI surfaces are proposed.

**Rationale:** The flagship example demonstrates the complete flow through code.
A UI would be non-authoritative, replaceable, and unable to bypass service
semantics — adding complexity without material benefit to the product thesis.

---

## 16. A2A Decision

**Explicitly defer.** A2A (agent-to-agent) protocol is not implemented in the
codebase. The design document (`docs/OS/principles_a2a.md`) acknowledges A2A as
the horizontal peer-to-peer layer, but v0 does not model A2A task/delegation.

**Rationale:** The current product thesis is about governing a single agent's
consequential actions through Solvent. A2A adds complexity without solving the
immediate goal of proving the end-to-end GitHub workflow. The architecture is
ready for A2A when the need arises — the kernel is domain-agnostic, the authority
model is generic, and the executor registry is extensible.

---

## 17. Demo Script

### Narrative (13 steps)

```bash
# Prerequisites: CockroachDB running, Solvent API + MCP started,
# GITHUB_TOKEN set, test repo with workflow_dispatch workflow configured

# 1. Agent gathers evidence
curl -X POST localhost:8080/v1/evidence \
  -H "Authorization: Bearer $SOLVENT_API_KEY" \
  -d '{"scenario_id":"...","provenance_class":"external_feed",
       "source_url":"https://github.com/etcd-io/etcd/issues/19220",
       "content_sha256":"abc123"}'

# 2. Agent proposes deployment
curl -X POST localhost:8080/v1/beliefs \
  -H "Authorization: Bearer $SOLVENT_API_KEY" \
  -d '{"scenario_id":"...","claim":"etcd v3.5.x is safe to deploy","claim_type":"derived"}'

# 3. Agent retires all debt items (6 calls)
# 4. Agent promotes belief
# 5. Agent creates target with frozen parameters
curl -X POST localhost:8080/v1/targets \
  -H "Authorization: Bearer $SOLVENT_API_KEY" \
  -d '{"principal_id":"...","resource_type":"scenario","resource_id":"...",
       "scope":"belief:...","action_namespace":"solvent","action_name":"deploy",
       "consequence_type":"execution",
       "consequence_parameters":"{\"repo\":\"org/test\",\"workflow\":\"deploy.yml\",\"ref\":\"main\"}",
       "created_by":"..."}'

# 6. Agent attaches justification
# 7. Agent requests authorization
# 8. Human operator approves
# 9. Agent authorizes action (creates live intent)
curl -X POST localhost:8080/v1/authorizations/action \
  -H "Authorization: Bearer $SOLVENT_API_KEY" \
  -d '{"scenario_id":"...","belief_id":"...","action":"deploy",
       "action_source":"user_typed","target_id":"..."}'

# 10. Agent executes (claims intent → invokes GitHub → records outcome)
curl -X POST localhost:8080/v1/authorizations/execute \
  -H "Authorization: Bearer $SOLVENT_API_KEY" \
  -d '{"scenario_id":"...","belief_id":"...","action":"deploy",
       "target_id":"...","intent_id":"...","consequence_type":"execution"}'
# Response: {"allowed":true,"success":true,"output":"RunID: 12345",...}

# 11. Audit shows complete chain
curl "localhost:8080/v1/activity?scenario_id=..."
# Shows: authorization_granted → adapter_invoked → executor_completed

# 12. Parameter substitution attack
curl -X POST localhost:8080/v1/authorizations/execute \
  -H "Authorization: Bearer $SOLVENT_API_KEY" \
  -d '{"scenario_id":"...","belief_id":"...","action":"deploy",
       "target_id":"...","intent_id":"...","consequence_type":"execution",
       "consequence_parameters":"{\"repo\":\"evil/repo\"}"}'
# Response: snapshot params used, NOT caller params

# 13. Unauthorized execution
curl -X POST localhost:8080/v1/authorizations/execute \
  -H "Authorization: Bearer $SOLVENT_API_KEY" \
  -d '{"scenario_id":"...","belief_id":"...","action":"deploy",
       "target_id":"nonexistent","intent_id":"...","consequence_type":"execution"}'
# Response: {"allowed":false,"reason":"..."}
```

**Central thesis demonstrated:**

> "The agent can propose. Solvent decides. GitHub executes."

---

## 18. Migration/Backward-Compatibility Considerations

- **No schema changes** — no new migrations needed
- **No kernel changes** — frozen architecture preserved
- **No breaking API changes** — new endpoint added, existing endpoints unchanged
- **No breaking MCP changes** — new tools added, existing tools unchanged
- **OpenAPI spec updated** — new endpoint documented
- **Existing tests preserved** — all Phase 4C+/4D regression tests retained
- **New endpoint is additive** — clients that don't use it are unaffected
- **authority.Service already exists** — REST server just needs to accept it

---

## 19. Explicit Non-Goals

- Additional providers (Kubernetes, AWS, etc.)
- Generic workflow orchestration
- Distributed transactions
- Exactly-once external execution
- Broad retry mechanisms
- Provider polling
- Multi-tenancy
- RBAC/IAM
- Compliance platform
- Large UI
- SDK ecosystem
- Agent memory system
- Generic event sourcing
- Kernel redesign

---

## 20. Risk Analysis

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| REST execution endpoint introduces new attack surface | Low | High | Same security model as MCP: snapshot params, fixed executor mapping, kernel.Authorize |
| MCP execution tool bypasses authority | Low | High | Calls authority.Service.ExecuteAction which calls kernel.Authorize — no shortcut |
| Audit log failure causes execution failure | Low | Low | Audit failures are logged but don't propagate — already the pattern |
| GitHub provider timeout in example | Medium | Low | 30s default timeout, provider returns ProviderAmbiguous, intent stays executing |
| Example requires real GitHub token | Medium | Low | Document clearly; provide FakeGitHubProvider path for testing |
| ReconcileIntent audit adds latency | Very Low | Very Low | Single INSERT, append-only, no blocking |

---

## 21. Exact Files Expected to Change

| File | Change | Reason |
|------|--------|--------|
| `api/api.go` | Add `authSvc *authority.Service` to Server struct, update `NewServer` | Wire authority service into REST server |
| `api/authorization.go` | Add `handleExecuteAction` handler | New execution endpoint |
| `api/types.go` | Add `ExecuteActionRequest`, `ExecuteActionResult` types | Request/response DTOs |
| `cmd/solvent-api/main.go` | Wire `authority.Service` into `NewServer` | Startup wiring |
| `cmd/solvent-mcp/main.go` | Register `solvent_execute` and `solvent_activity` tools | New MCP tools |
| `cmd/solvent-mcp/tools.go` | Add `handleSolventExecute`, `handleSolventActivity` handlers | Tool implementations |
| `service/audit/audit.go` | Add `ActivityReconciliationCompleted` constant | Reconciliation audit |
| `service/authority/authority.go` | Add audit logging in `ReconcileIntent` | Record operator identity |
| `docs/openapi/solvent.yaml` | Add `POST /v1/authorizations/execute` endpoint | API documentation |
| `examples/github/README.md` | Update with full lifecycle documentation | Developer experience |
| `examples/github/workflow/main.go` | New file: full end-to-end REST example | Flagship example |
| `examples/github/integration.go` | Update to include execution step | Complete the flow |
| `service/authority/authority_integration_test.go` | Add adversarial tests (parameter substitution, executor selector, revoked authority) | Security demonstrations |
| `cmd/solvent-mcp/tools_authority_test.go` | Add MCP execution tool tests | MCP coverage |

---

## 22. Exact Files That Must NOT Change

| File | Reason |
|------|--------|
| `kernel/authority.go` | Frozen architecture — kernel is the authority oracle |
| `kernel/contract.go` | Frozen contract interface |
| `kernel/sql.go` | Frozen SQL statements |
| `kernel/errors.go` | Frozen sentinel errors |
| `kernel/kernel_test.go` | Frozen kernel tests |
| `kernel/suite_test.go` | Frozen test infrastructure |
| `adapter/github/provider_errors.go` | Frozen provider outcome classification |
| `adapter/github/http_provider.go` | Frozen HTTP provider (Phase 4D hardened) |
| `adapter/github/provider.go` | Frozen provider interface |
| `db/001_schema.sql` through `db/008_executing_state.sql` | Frozen schema |
| `service/executor/executor.go` | Frozen executor registry |
| `AGENTS.md` | Frozen project rules |

---

## 23. Implementation Order

### Wave 1: Service layer (no API surface changes)

1. Add `ActivityReconciliationCompleted` to `service/audit/audit.go`
2. Add audit logging in `service/authority/authority.go` `ReconcileIntent`

### Wave 2: REST execution endpoint

3. Add `ExecuteActionRequest`/`ExecuteActionResult` to `api/types.go`
4. Add `handleExecuteAction` to `api/authorization.go`
5. Add route to `api/api.go`
6. Update `NewServer` signature in `api/api.go`
7. Wire `authority.Service` in `cmd/solvent-api/main.go`
8. Update OpenAPI spec

### Wave 3: MCP execution tools

9. Add `handleSolventExecute` to `cmd/solvent-mcp/tools.go`
10. Add `handleSolventActivity` to `cmd/solvent-mcp/tools.go`
11. Register tools in `cmd/solvent-mcp/main.go`

### Wave 4: Tests

12. Add REST execution endpoint tests to `api/authorization_test.go`
13. Add MCP execution tool tests to `cmd/solvent-mcp/tools_authority_test.go`
14. Add adversarial tests to `service/authority/authority_integration_test.go`
15. Add E2E acceptance tests

### Wave 5: Example and documentation

16. Create `examples/github/workflow/main.go`
17. Update `examples/github/integration.go`
18. Update `examples/github/README.md`

### Wave 6: Production hardening

19. Credential validation at startup
20. Configuration validation
21. Error message review

---

## 24. Definition of Done

- [ ] `POST /v1/authorizations/execute` endpoint works end-to-end via REST
- [ ] `solvent_execute` MCP tool works end-to-end
- [ ] `solvent_activity` MCP tool reads audit entries
- [ ] REST and MCP produce semantically equivalent authorization and execution behavior
- [ ] Snapshot parameters are authoritative (caller params ignored)
- [ ] Fixed action→executor mapping enforced
- [ ] Authority revoked before execution → denied
- [ ] ReconcileIntent logs operator identity
- [ ] All Phase 4C+/4D regression tests pass
- [ ] New adversarial tests pass
- [ ] Flagship example runs with real GitHub token
- [ ] Example demonstrates parameter substitution attack → denied
- [ ] Example demonstrates unauthorized execution → denied
- [ ] Audit trail shows complete chain: authorization → adapter_invoked → executor_completed
- [ ] No kernel changes
- [ ] No schema migrations
- [ ] `task test` passes
- [ ] OpenAPI spec updated with new endpoint
- [ ] Developer can run example with minimal interpretation
