# Phase 4C+ Implementation Plan

**First Real GitHub Executor — Test Engineering First**

---

## 1. Executive Decision

Phase 4C+ proves that Solvent can control a real external side effect through
its authorization kernel. The first executor triggers a GitHub Actions workflow
on a safely scoped test repository.

The phase follows the test-engineering-first principle:

> Target definition → Invariants → State/failure model → Deterministic tests →
> Implementation → Fresh verification → Kilo Code adversarial review

No production executor work begins until the test target is explicit.

The kernel remains frozen except for one additive method (`CompleteIntent`).
The executor lives entirely in the extension plane. The authorization
boundary is unchanged. The execution boundary is new and consequential.

---

## 2. Scope

**In scope:**

- Test engineering specification (Step 0)
- Deterministic GitHub provider test harness (fake)
- Real GitHub Actions executor (one narrow action: trigger a workflow)
- Credential handling for GitHub API
- Audit separation: authorization vs. execution vs. provider result
- Integration test against a safely scoped test repository
- Kilo Code independent adversarial review

**Out of scope:**

- Kernel redesign
- Schema migrations (except one additive column/constraint)
- New services
- Generic executor framework
- Multi-provider support
- Kubernetes, AWS, or other cloud executors
- RBAC, multi-tenancy, enterprise identity
- Workflow engine
- Async polling / retry orchestration
- Generic GitHub automation platform

---

## 3. Non-Goals

- Do not build a generic CI/CD engine
- Do not generalize the executor abstraction beyond what the first real
  executor requires
- Do not add kernel primitives for execution beyond `CompleteIntent`
- Do not introduce authorization-context or execution-result types beyond
  what the existing `ExecutionResult` provides
- Do not create a GitHub SDK wrapper
- Do not support multiple GitHub repositories in this phase
- Do not implement workflow status polling (synchronous trigger only)

---

## 4. Current-State Reconnaissance

### Executor abstraction (verified in codebase)

**`service/executor/executor.go`** (52 lines):
```go
type ActionFunc func(ctx context.Context, params map[string]interface{}) (string, error)

type Registry struct { actions map[string]ActionFunc }
func NewRegistry() *Registry
func (r *Registry) Register(name string, fn ActionFunc)
func (r *Registry) Get(name string) (ActionFunc, bool)
func ExecuteAction(ctx, registry, actionName, params) (string, error)
```

**`service/authority/authority.go:148-256`** — the ONE production execution path:
1. `PrepareForAction` — re-reads current state, calls `kernel.Authorize`
2. `execReg.Get(tool_name)` — lookup from trusted registry
3. `fn(ctx, params)` — invoke ActionFunc
4. Audit logging at each step

**Critical finding (line 202):**
```go
toolName, _ := params["tool_name"].(string)
fn, ok := s.execReg.Get(toolName)
```
The executor name comes from caller-supplied `params`. This is the executor
selection security issue that must be fixed.

**Registry state:** Empty in all production entry points:
- `cmd/solvent-mcp/main.go:126`: `execReg := executor.NewRegistry()`
- `demo/cloud/web/main.go:86`: `execReg := executor.NewRegistry()`
- `cmd/solvent-api/main.go`: does not use executor system at all

### Existing test infrastructure

- Real CockroachDB at `localhost:26260`
- Per-package database isolation via `testdb.SuiteDSN()`
- `testdb.Reset()` drops/creates DB, applies 7 DDL files
- File-based lock prevents concurrent test database resets
- `RecordingFunc` test double records calls and params
- 21 integration tests in `service/authority/` already cover execution authorization
- Kernel tests use `Case` receipt system with failure artifacts

### Existing GitHub adapter

`adapter/github/github.go` (377 lines): webhook event normalizer only.
No GitHub API client. No `go-github` or similar library in `go.mod`.
Produces `NormalizedEvidence` structs from webhook payloads.

### Audit system

`service/audit/audit.go`: append-only `audit_activity` table. Activity types
already include: `adapter_invoked`, `provider_responded`, `executor_completed`,
`executor_failed`, `executor_denied`.

### Kernel state

- `action_intent.state` CHECK: `state IN ('live','cancelled','executed')`
- `authorizeWithinTx`: read-only authority evaluation
- `AuthorizeAndCreateIntent`: atomic authority + intent creation with FOR UPDATE lock
- No code currently transitions intent to `'executed'`

---

## 5. Test Engineering Gate — Step 0

### Purpose

Before any production code, define exactly:

1. **What behavior are we trying to prove?**
   Solvent authorizes an action, then an executor invokes a real GitHub API,
   and the three facts (authorized / invoked / provider-accepted) are
   distinguishable.

2. **What security/correctness invariants must always hold?**
   See Section 7.

3. **What transitions are legal?**
   See Section 6.

4. **What transitions are forbidden?**
   No side effect without current authorization. No authority grant from
   executor. No stale authority execution.

5. **What failure modes must be tested?**
   See Section 9.

6. **What external-provider behaviors must be simulated?**
   See Section 12.

7. **What evidence proves an execution happened?**
   Provider returns a workflow run ID. Test harness records the API call.

8. **What evidence proves it did NOT happen?**
   Fake provider records zero calls.

9. **How do we deterministically reproduce race conditions?**
   Same patterns as existing T-C1 through T-C10: multiple connections,
  `SERIALIZABLE` isolation, deliberate interleaving.

### Deliverable

`docs/OS/phase4c_plus_test_target.md` — the test engineering specification.
Written before any implementation code. Reviewed by Kilo Code before
implementation begins.

---

## 6. Canonical State Model

### Intent lifecycle (existing, unchanged)

```
PROPOSED (intent_on_promoted)
    ↓
LIVE (state = 'live', belief_status = 'promoted')
    ↓
CANCELLED (state = 'cancelled') — belief retracted or explicit cancel
    ↓
EXECUTED (state = 'executed') — execution completed (NEW: Phase 4C+)
```

### Execution lifecycle (new for Phase 4C+)

```
AUTHORIZED (kernel.Authorize returns Allowed=true)
    ↓
EXECUTION_ATTEMPTED (executor invoked)
    ↓
PROVIDER_ACCEPTED (GitHub API returns 2xx)
    ↓  or
PROVIDER_REJECTED (GitHub API returns 4xx/5xx)
    ↓  or
PROVIDER_AMBIGUOUS (timeout, network error, lost response)
```

### Key distinction

```
Authorization event ≠ Execution event ≠ Provider result
```

These are three separate facts recorded in three separate audit entries.
A successful authorization does not prove execution. A successful execution
invocation does not prove provider acceptance.

---

## 7. Security Invariants

| # | Invariant | Classification |
|---|-----------|---------------|
| SI-1 | No external side effect before successful current authorization | Security |
| SI-2 | An executor cannot grant authority | Security |
| SI-3 | An executor cannot revoke authority | Security |
| SI-4 | An executor cannot reinterpret denied authorization as allowed | Security |
| SI-5 | A stale/cached authorization cannot be used | Security |
| SI-6 | Caller-controlled executor selection cannot bypass trusted registry mapping | Security |
| SI-7 | The exact authorized action/target must be the action sent to GitHub | Security |
| SI-8 | Authorization and execution are distinct events | Security |
| SI-9 | Provider success does not retroactively create authority | Security |
| SI-10 | Provider failure does not create false success | Security |
| SI-11 | Audit must distinguish authorization decision, execution attempt, provider result | Correctness |
| SI-12 | A failed provider operation must not be reported as successful execution | Security |
| SI-13 | A successful authorization does not prove successful external execution | Security |
| SI-14 | External execution must not occur when the authorization step fails | Security |
| SI-15 | Revoked authority must not permit a new execution attempt | Security |

---

## 8. Correctness Invariants

| # | Invariant | Classification |
|---|-----------|---------------|
| CI-1 | ExecutionResult.Allowed is true if and only if kernel.Authorize returned Allowed=true | Correctness |
| CI-2 | ExecutionResult.Success is true if and only if the executor function returned nil error | Correctness |
| CI-3 | Audit entries are ordered: authorization → invocation → result | Correctness |
| CI-4 | The intent state transitions live → executed only on provider acceptance | Correctness |
| CI-5 | Executor params match the authorized action/target exactly | Correctness |

---

## 9. Failure Model

| # | Failure | Classification | Behavior |
|---|---------|---------------|----------|
| F-1 | Authorization denied | Security | No provider call. `executor_denied` audit. |
| F-2 | Target revoked | Security | No provider call. `executor_denied` audit. |
| F-3 | Wrong target | Security | No provider call. `executor_denied` audit. |
| F-4 | Wrong action | Security | No provider call. `executor_denied` audit. |
| F-5 | Wrong actor | Security | No provider call. `executor_denied` audit. |
| F-6 | Stale authority | Security | Re-read at execution time catches stale state. No provider call. |
| F-7 | Malformed parameters | Correctness | Validation rejects. No provider call. |
| F-8 | Executor not registered | Correctness | `executor_failed` audit. No provider call. |
| F-9 | Executor returns error | Operational | `executor_failed` audit. No provider call. |
| F-10 | GitHub API returns 4xx | Operational | Provider rejected. `executor_failed` audit. |
| F-11 | GitHub API returns 5xx | Operational | Provider error. `executor_failed` audit. |
| F-12 | Network timeout | Ambiguous | Provider state unknown. `executor_failed` audit with ambiguous flag. |
| F-13 | Context cancellation | Operational | Executor ctx cancelled. No provider call. |
| F-14 | Provider accepts but response lost | Ambiguous | Provider state unknown. Requires operator reconciliation. |
| F-15 | Duplicate execution attempt | Ambiguous | Provider may have accepted. Requires idempotency key or reconciliation. |
| F-16 | Retry after unknown outcome | Ambiguous | Must not duplicate side effect. Requires idempotency key. |
| F-17 | Audit write failure | Operational | Execution result remains truthful. Audit gap is detectable. |
| F-18 | Process crash between authorization and provider | Ambiguous | No side effect (if crash before provider call) or unknown (if crash after). |
| F-19 | Process crash after provider side effect but before audit | Ambiguous | Side effect happened. Audit gap. Requires operator reconciliation. |
| F-20 | Provider succeeds but Solvent receives error | Ambiguous | Side effect happened. Solvent reports failure. Requires reconciliation. |
| F-21 | Provider rejects after Solvent authorization | Operational | Authorization was valid at time of check. Provider rejection is terminal. |

---

## 10. TOCTOU / External Side-Effect Model

### The unavoidable boundary

```
T1: kernel.Authorize succeeds (CockroachDB SERIALIZABLE transaction)
    ↓
T2: executor invoked (same goroutine, same process)
    ↓
T3: GitHub API accepts/rejects (network boundary)
```

### What can happen between T1 and T2

- Authority is revoked after T1 but before T2
- Target changes after T1 but before T2
- Belief is retracted after T1 but before T2

**Current guarantee:** `PrepareForAction` re-reads current state at T2
(the same method as T1 — it calls `kernel.Authorize` again). This
eliminates the T1→T2 TOCTOU for authority revocation. The existing
integration tests EA08, EA09, CR-A, CR-B, CR-C already prove this.

### What can happen between T2 and T3

- GitHub state changes (repository deleted, workflow removed, permissions revoked)
- Network partition
- GitHub accepts but response is lost

**Honest guarantee:** Solvent guarantees that authorization was current at T2
and that the executor was invoked with the authorized parameters. Solvent
does NOT guarantee that GitHub completed the operation. This is an inherent
limitation of any system that authorizes external side effects.

### Mitigation

- Synchronous trigger (no async polling in this phase)
- Provider response is the authoritative execution reference
- Audit records the provider response (workflow run ID or error)
- Ambiguous outcomes are classified as `requires operator reconciliation`

### Documented limitation

The TOCTOU between executor invocation and provider acceptance is an
unresolved v1 limitation. A future phase may add idempotency keys,
confirmation polling, or rollback mechanisms. Phase 4C+ does not solve this.

---

## 11. Test Matrix

| # | Scenario | Preconditions | Operation | Expected Auth | Expected Provider Calls | Expected Side Effect | Expected Audit | Classification |
|---|----------|--------------|-----------|--------------|------------------------|---------------------|---------------|---------------|
| T-01 | Valid authority → success | Principal, promoted belief, approved target, registered executor | ExecuteAction | Allowed=true, Success=true | 1 call with correct params | Workflow triggered | adapter_invoked + executor_completed | Security + Correctness |
| T-02 | No authority → zero calls | Promoted belief, no authority target | ExecuteAction | Allowed=false | 0 | None | executor_denied | Security |
| T-03 | Revoked authority → zero calls | Approved target, then revoked | ExecuteAction | Allowed=false | 0 | None | executor_denied | Security |
| T-04 | Wrong target → zero calls | Authority for target A | Execute with target B | Allowed=false | 0 | None | executor_denied | Security |
| T-05 | Wrong action → zero calls | Authority for "deploy" | Execute with "rollback" | Allowed=false | 0 | None | executor_denied | Security |
| T-06 | Wrong actor → zero calls | Authority for principal A | Execute with principal B | Allowed=false | 0 | None | executor_denied | Security |
| T-07 | Executor missing → no call | Valid authority, unregistered tool | ExecuteAction | Allowed=true, Success=false | 0 | None | executor_failed | Correctness |
| T-08 | Provider rejection → distinct | Valid authority | Execute, provider 4xx | Allowed=true, Success=false | 1 call | None | executor_failed | Operational |
| T-09 | Provider timeout → ambiguous | Valid authority | Execute, timeout | Allowed=true, Success=false | 1 call | Unknown | executor_failed | Ambiguous |
| T-10 | Duplicate invocation → defined | Valid authority, first succeeded | Execute again | Allowed=true | 1 or 2 calls | Depends | Both audited | Ambiguous |
| T-11 | Revocation before auth → no exec | Revoked target | Attempt authorize | Allowed=false | 0 | None | executor_denied | Security |
| T-12 | Revocation after auth before provider | Approve, authorize, revoke, execute | ExecuteAction | Allowed=false (re-read) | 0 | None | executor_denied | Security |
| T-13 | Provider succeeds but response lost | Valid authority | Execute, response lost | Allowed=true, Success=false | 1 call | Unknown | executor_failed | Ambiguous |
| T-14 | Audit failure after provider success | Valid authority | Execute, audit fails | Execution result truthful | 1 call | Side effect happened | Audit gap | Operational |
| T-15 | Inject arbitrary executor | Valid authority, malicious tool_name | ExecuteAction | Allowed=true, Success=false | 0 | None | executor_failed | Security |
| T-16 | Bypass authorization | No authority | Direct executor call | Should not reach executor | 0 | None | N/A | Security |

---

## 12. Deterministic Provider Test Harness

### Design

A fake GitHub provider implementing the minimal interface the real executor
will use:

```go
type FakeGitHubProvider struct {
    mu           sync.Mutex
    calls        []WorkflowCall
    accept       bool
    err          error
    delay        time.Duration
    lostResponse bool
}

type WorkflowCall struct {
    Repo     string
    Workflow string
    Ref      string
    Inputs   map[string]string
    CalledAt time.Time
}
```

Capabilities:
- `SetAccept(bool)` — control whether provider accepts or rejects
- `SetError(error)` — inject errors (network timeout, 5xx, etc.)
- `SetDelay(time.Duration)` — artificial delay for race testing
- `SetLostResponse(bool)` — simulate lost response after acceptance
- `Calls() []WorkflowCall` — observe recorded calls (side-effect oracle)
- `Reset()` — clear state between tests

### Deterministic synchronization

No sleep-based tests. Use:
- `sync.WaitGroup` for provider call confirmation
- `context.WithCancel` for cancellation testing
- CockroachDB `SERIALIZABLE` for authority race testing
- Provider call recording for side-effect verification

---

## 13. Executor Boundary

### Current architecture

```
service/authority.ExecuteAction
    → PrepareForAction (re-reads state, calls kernel.Authorize)
    → execReg.Get(tool_name) (lookup from trusted registry)
    → fn(ctx, params) (invoke ActionFunc)
    → audit logging
```

### What the first real executor adds

```
service/authority.ExecuteAction (modified: derive tool_name from action)
    → PrepareForAction (unchanged)
    → action→executor mapping (new: fixed mapping, not caller-controlled)
    → githubExecutor(ctx, params) (new ActionFunc implementation)
        → validate params (repo, workflow, ref)
        → call GitHub API
        → return workflow run reference or error
    → audit logging (unchanged)
```

### What ExecuteAction already provides

- Current-state revalidation via `PrepareForAction`
- Audit logging at authorization, invocation, and result
- `ExecutionResult` with `Allowed`, `Success`, `Output`, `Error`

### What ExecuteAction needs

The `tool_name` derivation must change from caller-supplied to
action-derived. The narrowest fix: a fixed mapping in the service layer
that translates authorized action to executor name.

---

## 14. Executor Selection Security

### Current mechanism (vulnerability)

`params["tool_name"]` is extracted from caller-supplied params (line 202):
```go
toolName, _ := params["tool_name"].(string)
fn, ok := s.execReg.Get(toolName)
```

A caller could supply `tool_name = "malicious_action"` to attempt
invocation of any registered executor.

### Required fix

After `PrepareForAction` succeeds, derive `tool_name` from `action`
using a fixed mapping. Remove reliance on `params["tool_name"]`.

### Mapping for Phase 4C+

| Authorized action pattern | Executor name | Provider |
|--------------------------|---------------|----------|
| `"deploy"` | `"github_trigger_workflow"` | GitHub Actions |

The mapping is a `map[string]string` in the service layer. Callers cannot
influence it.

---

## 15. GitHub Execution Choice

### Narrowest useful real action

**Trigger a GitHub Actions workflow via `workflow_dispatch`.**

GitHub API endpoint:
```
POST /repos/{owner}/{repo}/actions/workflows/{workflow}/dispatches
```

Request body:
```json
{
  "ref": "main",
  "inputs": {}
}
```

This is:
- Synchronous (returns 204 on acceptance, or error)
- Observable (workflow run appears in GitHub Actions UI)
- Safely scoped (test repository, test workflow)
- Non-destructive (test workflow can be a no-op)

---

## 16. Real GitHub Executor

### Implementation location

`adapter/github/` — new files in the existing GitHub adapter package.

### New files

| File | Purpose |
|------|---------|
| `adapter/github/provider.go` | `GitHubProvider` interface + real implementation |
| `adapter/github/executor.go` | `ActionFunc` wrapping `GitHubProvider` |
| `adapter/github/fake_provider.go` | Deterministic test double |
| `adapter/github/executor_test.go` | Unit tests against fake provider |
| `adapter/github/provider_test.go` | Integration tests against real GitHub API |

### ActionFunc implementation

```go
func NewWorkflowTriggerExecutor(provider GitHubProvider) executor.ActionFunc {
    return func(ctx context.Context, params map[string]interface{}) (string, error) {
        repo, _ := params["repo"].(string)
        workflow, _ := params["workflow"].(string)
        ref, _ := params["ref"].(string)
        // validate params
        result, err := provider.TriggerWorkflow(ctx, repo, workflow, ref, nil)
        if err != nil {
            return "", err
        }
        return result.RunID, nil
    }
}
```

### Registration

In `cmd/solvent-mcp/main.go`:
```go
if token := os.Getenv("GITHUB_TOKEN"); token != "" {
    ghProvider := github.NewProvider(token)
    execReg.Register("github_trigger_workflow", github.NewWorkflowTriggerExecutor(ghProvider))
}
```

---

## 17. Credential / Secret Handling

### Requirements

- GitHub token loaded from environment variable `GITHUB_TOKEN`
- Never hardcoded, never committed, never logged
- Never placed in kernel state, authorization state, or API responses
- Never passed through `ActionFunc` params (injected at construction)

### Implementation

- `github.NewProvider(token string)` receives the token at construction time
- Token is stored in the provider struct, not in params
- `ActionFunc` closure captures the provider, not the token
- Audit logs never include the token
- `GITHUB_TOKEN` is read in `main.go`, validated as non-empty, passed to provider

### Local development

- `GITHUB_TOKEN` env var must be set for real integration tests
- Test repository: a dedicated no-op repository
- Test workflow: a simple `workflow_dispatch` that exits 0
- If `GITHUB_TOKEN` is not set, executor registration is skipped (not an error)

---

## 18. Authorization → Execution

### The boundary

```
kernel.Authorize (read-only, SERIALIZABLE)
    ↓ returns Allowed=true
PrepareForAction logs ActivityAuthorizationGranted
    ↓
Derive tool_name from action (fixed mapping, not caller-controlled)
    ↓
execReg.Get(tool_name) — trusted registry lookup
    ↓
fn(ctx, params) — executor invocation
    ↓
audit log: ActivityAdapterInvoked
    ↓
GitHub API call
    ↓
audit log: ActivityExecutorCompleted or ActivityExecutorFailed
```

### What this guarantees

- Authorization was current at the moment of the kernel check
- The executor was the one mapped to the authorized action
- The parameters were validated
- The provider was invoked
- The provider response was recorded

### What this does NOT guarantee

- GitHub completed the workflow (only that it accepted the dispatch request)
- The workflow succeeded (only that GitHub accepted it)
- The side effect is permanent (GitHub could roll back)

---

## 19. Audit / Evidence

### Audit entries per execution

| Step | ActivityType | Key Fields |
|------|-------------|------------|
| Authorization check | `authorization_granted` or `authorization_denied` | target_id, action, reason |
| Executor denied | `executor_denied` | target_id, action, reason |
| Pre-execution | `adapter_invoked` | target_id, action, tool |
| Execution success | `executor_completed` | target_id, action, output, provider_ref |
| Execution failure | `executor_failed` | target_id, action, error |

### Authoritative execution reference

For the real GitHub integration, the authoritative reference is:
- GitHub workflow run ID (from the API response)
- GitHub workflow run URL (constructed from run ID)

---

## 20. Observability

### What to observe

- Execution attempts (count, success rate)
- Provider response times
- Provider errors (4xx, 5xx, timeout)
- Ambiguous outcomes
- Authorization denied before execution (security signal)

### Implementation

Use existing `audit.ActivityEntry` system. No new observability platform.
The `GetActivities` query method supports filtering by activity type and
scenario.

---

## 21. Kernel-Growth Gate

### Analysis

| Proposed change | Kernel? | Decision |
|----------------|---------|----------|
| Executor registry | No | Extension plane (`service/executor/`) |
| GitHub provider | No | Adapter (`adapter/github/`) |
| Action-to-executor mapping | No | Service layer (`service/authority/`) |
| Credential handling | No | Deployment config |
| Audit logging | No | Audit service |
| Intent state transition live→executed | Yes | Kernel — see justification |

### Intent state transition justification

The `action_intent.state` column has CHECK: `state IN ('live','cancelled','executed')`.
No code currently transitions intent to `'executed'`. Phase 4C+ needs this.

**This IS a kernel change** because:
1. It is a new atomic security-critical state transition
2. It must be serialized against concurrent retract/cancel
3. It must respect the CHECK constraint
4. Application code cannot safely guarantee the constraint is respected

**The change is minimal:** one new SQL statement (`UPDATE action_intent SET state='executed' WHERE id=$1 AND state='live'`) and one new `Contract` method (`CompleteIntent`).

### What does NOT go in the kernel

- GitHub API calls
- Provider response handling
- Executor registry
- Credential management
- Workflow dispatch logic

---

## 22. Package / File Map

| File | Purpose | Prod/Test/Docs | New Abstraction? | Security Relevance | Why It Belongs Here |
|------|---------|---------------|-----------------|-------------------|-------------------|
| `docs/OS/phase4c_plus_test_target.md` | Test engineering spec | Docs | No | No | Target definition before code |
| `adapter/github/provider.go` | GitHubProvider interface + real impl | Prod | Yes (interface) | High | Adapter pattern |
| `adapter/github/executor.go` | ActionFunc wrapping provider | Prod | No | High | Execution boundary |
| `adapter/github/fake_provider.go` | Deterministic test double | Test | Yes (fake) | No | Enables deterministic testing |
| `adapter/github/executor_test.go` | Unit tests for executor | Test | No | No | Proves behavior |
| `adapter/github/provider_test.go` | Integration tests | Test | No | No | Proves real API works |
| `service/authority/authority.go` | Fix executor selection, add action mapping | Prod | No | High — selection security | Existing boundary |
| `service/authority/execution_test.go` | Integration tests for execution | Test | No | No | Proves invariants |
| `kernel/authority.go` | Add `CompleteIntent` method | Prod | Yes (method) | High — state transition | Atomic change belongs in kernel |
| `kernel/sql.go` | Add `sqlCompleteIntent` statement | Prod | No | Medium | Kernel SQL boundary |
| `kernel/contract.go` | Add `CompleteIntent` to Contract | Prod | No | Medium | Interface completeness |
| `cmd/solvent-mcp/main.go` | Register GitHub executor | Prod | No | Medium — startup wiring | Existing wiring point |
| `examples/github/executor_example.go` | Runnable example | Docs | No | No | Demonstrates integration |

---

## 23. Implementation Order

### Step 0: Test Engineering (no code changes)

1. Write `docs/OS/phase4c_plus_test_target.md`
2. Define the test matrix (Section 11) as the acceptance target
3. Define the fake provider interface (Section 12)
4. **Kilo Code reviews test target before implementation begins**

### Step 1: Fake Provider + Executor Tests (test infrastructure)

1. `adapter/github/fake_provider.go`
2. `adapter/github/provider.go` (interface only)
3. `adapter/github/executor.go` (ActionFunc wrapping provider)
4. `adapter/github/executor_test.go` — all 16 test matrix scenarios
5. All tests pass against fake provider

### Step 2: Executor Selection Security Fix

1. Add action→executor mapping in `service/authority/authority.go`
2. Remove reliance on `params["tool_name"]` for executor selection
3. Update existing integration tests
4. Verify no caller can select arbitrary executor

### Step 3: Kernel CompleteIntent

1. `kernel/sql.go` — add `sqlCompleteIntent`
2. `kernel/authority.go` — add `CompleteIntent` method
3. `kernel/contract.go` — add to `Contract` interface
4. Kernel tests for intent state transition

### Step 4: Real GitHub Provider

1. `adapter/github/provider.go` (real GitHub API client using `net/http`)
2. `adapter/github/provider_test.go` (test with real token, skip if absent)
3. Credential handling: `GITHUB_TOKEN` env var

### Step 5: Wiring + Integration

1. Register executor in `cmd/solvent-mcp/main.go`
2. `examples/github/executor_example.go`
3. Update `docs/api/extensions.md` with execution documentation

### Step 6: Full Verification

```bash
task lint:openapi
task test:openapi
go build ./...
go vet ./...
go test -count=1 -p 1 ./...
go test -count=1 ./...
go test -count=1 -v ./adapter/github/...
```

### Step 7: Handoff to Kilo Code

1. Produce implementation report
2. Hand current tree to Kilo Code
3. Wait for GO / REQUEST CHANGES

---

## 24. Verification

### Mandatory gates

```bash
task lint:openapi
task test:openapi
go build ./...
go vet ./...
go test -count=1 -p 1 ./...
go test -count=1 ./...
```

### Executor-specific gates

```bash
go test -count=1 -v ./adapter/github/...
go test -count=1 -v ./service/authority/...
```

### Real integration (if environment permits)

```bash
GITHUB_TOKEN=... go test -count=1 -v -run TestRealGitHub ./adapter/github/...
```

---

## 25. Adversarial Testing

| # | Test | Property | Would fail if security removed? |
|---|------|----------|-------------------------------|
| AT-1 | Remove authorization check → executor never invoked | SI-1, SI-14 | Yes |
| AT-2 | Change target ID after authorization → provider not called | SI-7 | Yes |
| AT-3 | Change action after authorization → provider not called | SI-7 | Yes |
| AT-4 | Revoke target immediately before execution → no provider call | SI-15 | Yes |
| AT-5 | Substitute arbitrary executor name → rejected | SI-6 | Yes |
| AT-6 | Force executor invocation with denied authorization → fails | SI-1, SI-4 | Yes |
| AT-7 | Return provider error as success → test fails | SI-10, SI-12 | Yes |
| AT-8 | Return executor success without provider call → test fails | SI-14 | Yes |
| AT-9 | Simulate lost provider response → ambiguous state detected | SI-11 | Yes |
| AT-10 | Reuse stale authorization → test fails | SI-5 | Yes |
| AT-11 | Revoke immediately before execution → expected result enforced | SI-15 | Yes |
| AT-12 | Cause audit failure → execution result remains truthful | CI-3 | Yes |

---

## 26. Acceptance Criteria

### Build and test

- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `go test -count=1 -p 1 ./...` passes
- [ ] `go test -count=1 ./...` passes
- [ ] `task lint:openapi` passes (0 errors)
- [ ] `task test:openapi` passes (7/7)

### Executor tests

- [ ] All 16 test matrix scenarios pass against fake provider
- [ ] All 12 adversarial tests pass
- [ ] Executor selection: caller cannot select arbitrary executor
- [ ] TOCTOU: revocation between authorize and execute is caught
- [ ] Audit: authorization, invocation, and result are separate entries

### Real integration

- [ ] GitHub workflow dispatch succeeds against test repository
- [ ] Workflow run ID is recorded in audit
- [ ] Provider rejection is correctly classified

### Independent review

- [ ] Kilo Code independent adversarial review → GO

---

## 27. Rollback

### If the executor introduces regressions

1. Remove executor registration from `main.go` (one line per entry point)
2. The executor code remains but is unreachable
3. `CompleteIntent` is additive — no existing behavior changes

### If the GitHub integration fails

1. Fake provider tests continue to pass (provider is independent)
2. Real provider is optional (gated on `GITHUB_TOKEN`)
3. Test repository can be recreated

---

## 28. Risks / Open Questions

| # | Risk | Mitigation | Status |
|---|------|-----------|--------|
| R-1 | TOCTOU between executor invocation and GitHub acceptance | Documented limitation. Synchronous trigger minimizes window. | Accepted |
| R-2 | GitHub API rate limits | Scope to test repository. Provider concern, not kernel concern. | Accepted |
| R-3 | GitHub token rotation | Token is deployment config, not kernel state. | Accepted |
| R-4 | Test repository availability | Must exist before integration tests. Document setup. | Open |
| R-5 | Intent state transition requires kernel change | Justified: atomic state change is a durable security fact. Minimal change. | Approved |
| R-6 | `params["tool_name"]` caller control | Fix: derive from authorized action, not caller. | Approved |
| R-7 | Ambiguous provider outcomes | Classified as `requires operator reconciliation`. | Accepted |
| R-8 | Duplicate execution (no idempotency) | Documented v1 limitation. Deferred to future phase. | Deferred |

---

## 29. Kilo Code Independent Review Gate

### Mandatory completion gate

```
OpenCode
    ↓
implement
    ↓
verify
    ↓
STOP
    ↓
hand CURRENT repository to Kilo Code
    ↓
Kilo Code independently reviews
    ↓
GO / REQUEST CHANGES
```

### Review scope for Kilo Code

Kilo Code must independently verify:

1. All 16 test matrix scenarios pass
2. All 12 adversarial tests pass
3. Executor selection security is enforced
4. TOCTOU boundaries are correct
5. Audit separation is accurate
6. Credential handling is secure
7. No kernel overreach
8. Test engineering target matches implementation
9. No forbidden endpoints added
10. No API behavior changes

### Phase closure

Phase 4C+ is CLOSED only when:

- All verification gates pass
- Kilo Code independent review returns GO
- Real GitHub integration succeeds (if environment permits)

---

## 30. Next Phase

Phase 4D: Idempotency and async execution

Phase 4C+ proves synchronous trigger. Future phases may add:

- Idempotency keys for duplicate prevention
- Async execution with polling
- Workflow status tracking
- Rollback on failure
- Multiple provider support

These are explicitly deferred.

---

## 31. Final Recommendation

### Strongest honest security guarantee

> Solvent authorized the action based on current authority state, invoked the
> executor with the authorized parameters, and the provider accepted the
> request. Solvent does not guarantee that the provider completed the
> operation — that is the provider's responsibility.

Four distinct facts:
1. **Authorization** — Solvent determined the action is allowed
2. **Invocation** — The executor was called with authorized parameters
3. **Provider acceptance** — GitHub accepted the dispatch request
4. **Provider completion** — GitHub completed the workflow (NOT guaranteed)

### Architectural preservation

The kernel does not grow into an execution engine. The executor is registered
in the extension plane. The kernel gains one additive method (`CompleteIntent`)
for an atomic state transition that cannot safely be done above the kernel.
Everything else — GitHub API calls, credential handling, provider response
parsing, workflow dispatch — lives in `adapter/github/`.

```
SMALL TRUSTED AUTHORITY KERNEL
        ↓
STABLE CANONICAL API
        ↓
SERVICE / POLICY
        ↓
ADAPTER / EXECUTOR
        ↓
EXTERNAL PROVIDER
```

The kernel answers: **MAY THIS HAPPEN?**
The extension plane determines: **HOW DO WE ACT ON THAT DECISION?**
