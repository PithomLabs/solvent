# Phase 4C+ Implementation Plan (Revised)

**First Real GitHub Executor — Test Engineering First**

**Revision:** Addresses two CRITICAL design gaps and plan issues from
`docs/OS/plan_4c_plus_review.md`.

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

**Execution entry point:** Phase 4C+ uses the trusted local MCP execution
surface as its execution entry point. It does not yet establish a general
remote execution API.

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
- Schema migrations
- New services
- Generic executor framework
- Multi-provider support
- Kubernetes, AWS, or other cloud executors
- RBAC, multi-tenancy, enterprise identity
- Workflow engine
- Async polling / retry orchestration
- Generic GitHub automation platform
- General remote execution API

---

## 3. Non-Goals

- Do not build a generic CI/CD engine
- Do not generalize the executor abstraction beyond what the first real
  executor requires
- Do not add kernel primitives for execution beyond `CompleteIntent`
- Do not create a GitHub SDK wrapper
- Do not support multiple GitHub repositories in this phase
- Do not implement workflow status polling (synchronous trigger only)
- Do not establish a general remote execution API

---

## 4. Current-State Reconnaissance

### Executor abstraction (verified)

`service/executor/executor.go`:
- `ActionFunc = func(ctx context.Context, params map[string]interface{}) (string, error)`
- `Registry` maps string names to `ActionFunc`
- `ExecuteAction` does lookup-and-call
- Registry is empty in all production entry points

`service/authority/authority.go:148-256`:
- `ExecuteAction` is the ONE production execution path
- Calls `PrepareForAction` → `execReg.Get(tool_name)` → `fn(ctx, params)`
- `params["tool_name"]` is caller-controlled (security issue to fix)
- Audit logs at each step

### consequence_parameters (the binding mechanism)

- `authority_target.consequence_parameters` — JSONB NOT NULL, set at target creation
- `target_snapshot.consequence_parameters` — JSONB NOT NULL, frozen at approval, immutable
- `kernel.Authorize` compares tuple's `ConsequenceParameters` against snapshot via `jsonEqual` (semantic JSON comparison)
- `handleAuthorizeAction` hardcodes `ConsequenceParameters: []byte("{}")` — this is the Phase 4C design
- The `consequence_parameters` field is the canonical execution binding

### ActionIntent state machine

- `action_intent.state` CHECK: `state IN ('live','cancelled','executed')`
- No code currently transitions to `'executed'`
- `CompleteIntent` would be the first code to set `state = 'executed'`

### Existing test infrastructure

- Real CockroachDB at `localhost:26260`
- Per-package database isolation via `testdb.SuiteDSN()`
- 21 integration tests in `service/authority/` cover execution authorization
- `RecordingFunc` test double records calls and params

---

## 5. Test Engineering Gate — Step 0

### Purpose

Before any production code, define exactly:

1. **What behavior are we trying to prove?**
   Solvent authorizes an action, binds it to exact provider-specific
   parameters, and an executor invokes a real GitHub API with those
   exact parameters. The three facts (authorized / invoked /
   provider-accepted) are distinguishable.

2. **What security/correctness invariants must always hold?**
   See Section 7.

3. **What transitions are legal?**
   See Section 6.

4. **What transitions are forbidden?**
   No side effect without current authorization. No authority grant from
   executor. No stale authority execution. No parameter substitution.

5. **What failure modes must be tested?**
   See Section 9.

6. **What external-provider behaviors must be simulated?**
   See Section 12.

7. **What evidence proves an execution happened?**
   Provider returns a workflow run ID (via `return_run_details=true`).
   Test harness records the API call.

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

### Intent lifecycle (existing, extended)

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
INVOKED (executor function called)
    ↓
PROVIDER_ACCEPTED (GitHub API returns 2xx with run reference)
    ↓  or
PROVIDER_REJECTED (GitHub API returns 4xx/5xx)
    ↓  or
PROVIDER_AMBIGUOUS (timeout, network error, lost response)
```

### Four distinct facts

```
1. Authorization  — Solvent determined the action is allowed
2. Invocation     — The executor was called with authorized parameters
3. Provider acceptance — GitHub accepted the dispatch request
4. Provider completion — GitHub completed the workflow (NOT guaranteed by Solvent)
```

These are four separate facts recorded in separate audit entries.
A successful authorization does not prove execution. A successful execution
invocation does not prove provider acceptance. Provider acceptance does not
prove provider completion.

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
| SI-16 | Provider-specific execution parameters must be derived from the approved snapshot, not caller-supplied | Security |
| SI-17 | Caller-supplied execution parameters must not override approved values | Security |
| SI-18 | The executor may execute only the provider-specific parameters bound to the approved target | Security |

---

## 8. Correctness Invariants

| # | Invariant | Classification |
|---|-----------|---------------|
| CI-1 | ExecutionResult.Allowed is true if and only if kernel.Authorize returned Allowed=true | Correctness |
| CI-2 | ExecutionResult.Success is true if and only if the executor function returned nil error | Correctness |
| CI-3 | Audit entries are ordered: authorization → invocation → result | Correctness |
| CI-4 | The intent state transitions live → executed only on provider acceptance | Correctness |
| CI-5 | Executor params match the authorized action/target exactly | Correctness |
| CI-6 | Provider-specific parameters (repo/workflow/ref) match the approved snapshot exactly | Correctness |

---

## 9. Failure Model

### Executor-internal failures (zero provider calls)

| # | Failure | Behavior |
|---|---------|----------|
| F-1 | Authorization denied | No provider call. `executor_denied` audit. |
| F-2 | Target revoked | No provider call. `executor_denied` audit. |
| F-3 | Wrong target | No provider call. `executor_denied` audit. |
| F-4 | Wrong action | No provider call. `executor_denied` audit. |
| F-5 | Wrong actor | No provider call. `executor_denied` audit. |
| F-6 | Stale authority | Re-read catches stale state. No provider call. |
| F-7 | Malformed parameters | Validation rejects. No provider call. |
| F-8 | Executor not registered | `executor_failed` audit. No provider call. |
| F-9 | Executor internal validation failure | `executor_failed` audit. No provider call. |
| F-10 | Parameter mismatch (repo/workflow/ref differs from approved) | No provider call. `executor_failed` audit. |

### Provider-call failures (one provider call made)

| # | Failure | Behavior |
|---|---------|----------|
| F-11 | GitHub API returns 4xx | Provider rejected. `executor_failed` audit. |
| F-12 | GitHub API returns 5xx | Provider error. `executor_failed` audit. |
| F-13 | Network error after request transmission | Provider state unknown. `executor_failed` audit. |

### Ambiguous outcomes (provider state unknown)

| # | Failure | Behavior |
|---|---------|----------|
| F-14 | Network timeout | Provider state unknown. `executor_failed` audit. |
| F-15 | Provider accepts but response lost | Provider state unknown. Requires reconciliation. |
| F-16 | Process crash between authorization and provider | No side effect (if crash before provider). |
| F-17 | Process crash after provider but before audit | Side effect happened. Audit gap. |
| F-18 | Provider succeeds but Solvent receives error | Side effect happened. Solvent reports failure. |

### Operational failures

| # | Failure | Behavior |
|---|---------|----------|
| F-19 | Context cancellation | Executor ctx cancelled. No provider call. |
| F-20 | Audit write failure | Execution result remains truthful. Audit gap. |

---

## 10. TOCTOU / External Side-Effect Model

### The unavoidable boundary

```
T1: kernel.Authorize succeeds (SERIALIZABLE read)
    ↓
    ** window where authority may change **
    ↓
T2: executor invoked (trusted executor selection from approved snapshot)
    ↓
    ** window where GitHub state may change **
    ↓
T3: GitHub API accepts/rejects
```

### What re-authorization provides

A fresh `kernel.Authorize` call immediately before executor invocation
catches stale authority that changed *before* the check. This is the
strongest guarantee available without blocking on the provider.

### What re-authorization does NOT provide

Atomicity with the provider call. There is always a window between the
authorization check and the provider invocation in which authority may
change. This is an inherent limitation of any system that authorizes
external side effects.

### The honest guarantee

Solvent guarantees that authorization was current at the moment of the
final check and that the executor was invoked with the approved
parameters. Solvent does NOT guarantee that the provider completed the
operation. The T1→T2 window is a documented v1 limitation.

### Serialization against concurrent execution

For Phase 4C+, the execution entry point is the trusted local MCP surface
(single process, sequential execution). Concurrent duplicate execution is
a known v1 limitation. Sequential duplicate execution is prevented by the
intent state check (`live` → `executed`).

A future phase may add a claim/reservation mechanism (e.g.,
`ClaimForExecution` with `FOR UPDATE`) if concurrent execution surfaces.

---

## 11. Test Matrix

### Authorization tests

| # | Scenario | Expected Auth | Expected Provider Calls | Expected Side Effect | Classification |
|---|----------|--------------|------------------------|---------------------|---------------|
| T-01 | Valid authority → execution succeeds | Allowed=true, Success=true | 1 call | Workflow triggered | Security + Correctness |
| T-02 | No authority → zero calls | Allowed=false | 0 | None | Security |
| T-03 | Revoked authority → zero calls | Allowed=false | 0 | None | Security |
| T-04 | Wrong target → zero calls | Allowed=false | 0 | None | Security |
| T-05 | Wrong action → zero calls | Allowed=false | 0 | None | Security |
| T-06 | Wrong actor → zero calls | Allowed=false | 0 | None | Security |
| T-07 | Executor missing → no call | Allowed=true, Success=false | 0 | None | Correctness |
| T-08 | Revocation before auth → no exec | Allowed=false | 0 | None | Security |
| T-09 | Revocation after auth but before provider | Allowed=false (re-read) | 0 | None | Security |
| T-10 | Attempt to inject arbitrary executor | Allowed=true, Success=false | 0 | None | Security |

### Parameter binding tests (CRITICAL 1)

| # | Scenario | Expected Auth | Expected Provider Calls | Expected Side Effect | Classification |
|---|----------|--------------|------------------------|---------------------|---------------|
| T-11 | Same target, same action, substitute repo | Allowed=false (param mismatch) | 0 | None | Security |
| T-12 | Same target, same action, substitute workflow | Allowed=false (param mismatch) | 0 | None | Security |
| T-13 | Same target, same action, substitute ref | Allowed=false (param mismatch) | 0 | None | Security |
| T-14 | Same target, same action, substitute any parameter | Allowed=false (param mismatch) | 0 | None | Security |
| T-15 | Same target, same action, caller supplies params that match approved | Allowed=true, Success=true | 1 call | Workflow triggered | Correctness |

### TOCTOU tests (CRITICAL 2)

| # | Scenario | Expected Auth | Expected Provider Calls | Expected Side Effect | Classification |
|---|----------|--------------|------------------------|---------------------|---------------|
| T-16 | Authority revoked immediately before execution | Allowed=false (re-read) | 0 | None | Security |
| T-17 | Authority revoked concurrently with execution start | Allowed=false (re-read) | 0 | None | Security |
| T-18 | Target changed before execution | Allowed=false (re-read) | 0 | None | Security |
| T-19 | Binding changed before execution | Allowed=false (param mismatch) | 0 | None | Security |
| T-20 | Stale execution parameters | Allowed=false (param mismatch) | 0 | None | Security |
| T-21 | Stale authorization | Allowed=false (re-read) | 0 | None | Security |

### Provider failure tests

| # | Scenario | Expected Auth | Expected Provider Calls | Expected Side Effect | Classification |
|---|----------|--------------|------------------------|---------------------|---------------|
| T-22 | Provider rejection (4xx) | Allowed=true | 1 call | None | Operational |
| T-23 | Provider error (5xx) | Allowed=true | 1 call | None | Operational |
| T-24 | Provider timeout | Allowed=true | 1 call | Unknown | Ambiguous |
| T-25 | Provider accepts but response lost | Allowed=true | 1 call | Unknown | Ambiguous |
| T-26 | Network error before transmission | Allowed=true | 0 or unknown | None or unknown | Ambiguous |

### Duplicate execution tests

| # | Scenario | Expected Auth | Expected Provider Calls | Expected Side Effect | Classification |
|---|----------|--------------|------------------------|---------------------|---------------|
| T-27 | Sequential duplicate: second call after first completes | Intent state = 'executed', refused | 0 | None prevented by state check | Correctness |
| T-28 | Concurrent duplicate: two calls race | Both may reach provider | 0, 1, or 2 calls | Known v1 limitation | Known limitation |

### Audit tests

| # | Scenario | Expected Audit Entries | Classification |
|---|----------|----------------------|---------------|
| T-29 | Successful execution | authorization_granted, adapter_invoked, executor_completed | Correctness |
| T-30 | Denied execution | authorization_denied or executor_denied | Correctness |
| T-31 | Provider failure | authorization_granted, adapter_invoked, executor_failed | Correctness |
| T-32 | Audit failure after provider success | Execution result remains truthful | Operational |

---

## 12. Deterministic Provider Test Harness

### Design

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
    Repo      string
    Workflow  string
    Ref       string
    Inputs    map[string]string
    CalledAt  time.Time
    Accepted  bool
    RunID     string
}
```

### Provider side-effect oracle (5 levels)

```
1. Invocation      — executor function was called
2. Request sent    — provider received the HTTP request
3. Provider accepted — provider returned 2xx
4. Run created     — workflow run ID returned (requires return_run_details=true)
5. Run completed   — workflow finished (NOT guaranteed in 4C+)
```

For Phase 4C+, the oracle stops at level 4 (provider accepted + run reference).

### Deterministic synchronization

No sleep-based tests. Use:
- `sync.WaitGroup` for provider call confirmation
- `context.WithCancel` for cancellation testing
- CockroachDB `SERIALIZABLE` for authority race testing
- Provider call recording for side-effect verification

---

## 13. Execution Parameter Binding (CRITICAL 1)

### Architecture

```
Target creation
    consequence_parameters = {"repo":"org/repo","workflow":"deploy.yml","ref":"main"}
        ↓
Approval (kernel.Approve)
    frozen into target_snapshot (immutable, JSONB)
        ↓
Execution (service/authority.ExecuteAction)
    1. Read approved snapshot's consequence_parameters
    2. Construct AuthorityTuple with those values
    3. kernel.Authorize checks tuple against snapshot (jsonEqual)
    4. Executor receives ONLY the approved values (not caller-supplied)
        ↓
GitHub
    EXACTLY the approved repo/workflow/ref
```

### Binding source

The approved `target_snapshot.consequence_parameters` is the single source
of truth for provider-specific execution parameters. The service layer
reads it from the snapshot. Caller-supplied execution parameters are
ignored by the executor.

### Component ownership

| Component | Responsibility |
|-----------|---------------|
| Kernel | Stores `consequence_parameters` in snapshot. Compares tuple against snapshot during `Authorize`. |
| Service layer | Reads approved snapshot's `consequence_parameters`. Passes to executor as canonical values. |
| Executor | Receives approved values. Validates and calls provider. Does NOT read caller-supplied params for execution parameters. |
| API/MCP | Creates targets with `consequence_parameters`. Does NOT supply execution parameters at execution time. |

### Kernel knowledge

The kernel does NOT know what `repo`, `workflow`, or `ref` mean. It stores
`consequence_parameters` as opaque JSONB and compares it semantically.
Provider-specific semantics remain entirely in the adapter/executor layer.

---

## 14. Executor Selection Security

### Current mechanism (vulnerability)

```go
toolName, _ := params["tool_name"].(string)
fn, ok := s.execReg.Get(toolName)
```

### Fix

After `PrepareForAction` succeeds, derive `tool_name` from `action`
using a fixed mapping in the service layer. Remove reliance on
`params["tool_name"]`.

### Mapping for Phase 4C+

| Authorized action | Executor name | Provider |
|------------------|---------------|----------|
| `"deploy"` | `"github_trigger_workflow"` | GitHub Actions |

---

## 15. GitHub Execution Choice

### API endpoint

```
POST /repos/{owner}/{repo}/actions/workflows/{workflow}/dispatches?return_run_details=true
```

The `return_run_details=true` query parameter ensures the response includes
`workflow_run_id` and `run_url`, which serve as the authoritative execution
reference.

### Response model (verified)

- **200 OK** with JSON body containing `workflow_run_id`, `run_url`, `html_url`
- **204 No Content** when `return_run_details=false` (NOT used)
- **4xx/5xx** for errors

---

## 16. Real GitHub Executor

### Implementation location

`adapter/github/` — new files in the existing adapter package.

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

### GitHubProvider interface

```go
type GitHubProvider interface {
    TriggerWorkflow(ctx context.Context, repo, workflow, ref string,
        inputs map[string]string) (*WorkflowResult, error)
}

type WorkflowResult struct {
    RunID    string
    RunURL   string
    Accepted bool
}
```

### Real implementation

`adapter/github/provider.go`:
- Uses `net/http` with Bearer token authentication
- POST to GitHub REST API with `return_run_details=true`
- Returns structured result
- No `go-github` dependency (keep minimal)

---

## 17. Credential / Secret Handling

- GitHub token loaded from `GITHUB_TOKEN` env var
- Never hardcoded, never committed, never logged
- Never placed in kernel state or API responses
- `github.NewProvider(token)` captures token at construction
- Audit logs never include the token
- If `GITHUB_TOKEN` not set, executor registration is skipped

---

## 18. Authorization → Execution

### The boundary

```
kernel.Authorize (SERIALIZABLE read)
    ↓ returns Allowed=true
Derive tool_name from action (fixed mapping)
    ↓
execReg.Get(tool_name) — trusted registry lookup
    ↓
Read approved snapshot consequence_parameters
    ↓
Validate execution params match approved values
    ↓
fn(ctx, approvedParams) — executor invocation
    ↓
GitHub API call
    ↓
audit log: result
```

### What this guarantees

- Authorization was current at the moment of the kernel check
- The executor was the one mapped to the authorized action
- The parameters were derived from the approved snapshot
- The provider was invoked with exactly those parameters
- The provider response was recorded

### What this does NOT guarantee

- GitHub completed the workflow (only that it accepted the dispatch)
- The workflow succeeded (only that GitHub accepted it)
- No race between authorization check and provider call

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

GitHub workflow run ID and URL (from API response with `return_run_details=true`).

---

## 20. Observability

Use existing `audit.ActivityEntry` system. No new observability platform.

---

## 21. Kernel-Growth Gate

### CompleteIntent

| Question | Answer |
|----------|--------|
| New durable security fact? | YES — records that execution occurred |
| New atomic state transition? | YES — `live` → `executed` with CHECK constraint |
| Impossible above kernel? | YES — `action_intent` is kernel-owned state machine |

**Caller constraint:** `CompleteIntent` is NOT exposed through API/MCP.
Callable only by the trusted execution service path. Only a
provider-accepted execution may cause that call.

### What does NOT go in the kernel

- GitHub API calls
- Provider response handling
- Executor registry
- Credential management
- Workflow dispatch logic
- Execution parameter binding (above kernel, in service layer)

---

## 22. Package / File Map

| File | Purpose | Prod/Test/Docs | Security Relevance |
|------|---------|---------------|-------------------|
| `docs/OS/phase4c_plus_test_target.md` | Test engineering spec | Docs | No |
| `adapter/github/provider.go` | GitHubProvider interface + real impl | Prod | High — external side effect boundary |
| `adapter/github/executor.go` | ActionFunc wrapping provider | Prod | High — execution boundary |
| `adapter/github/fake_provider.go` | Deterministic test double | Test | No |
| `adapter/github/executor_test.go` | Unit tests | Test | No |
| `adapter/github/provider_test.go` | Integration tests (opt-in) | Test | No |
| `service/authority/authority.go` | Fix executor selection, add action mapping | Prod | High — selection security |
| `service/authority/execution_test.go` | Integration tests | Test | No |
| `kernel/authority.go` | Add `CompleteIntent` method | Prod | High — state transition |
| `kernel/sql.go` | Add `sqlCompleteIntent` statement | Prod | Medium |
| `kernel/contract.go` | Add `CompleteIntent` to Contract | Prod | Medium |
| `cmd/solvent-mcp/main.go` | Register GitHub executor | Prod | Medium |
| `examples/github/executor_example.go` | Runnable example | Docs | No |

---

## 23. Implementation Order

### Step 0: Test Engineering (no code changes)

1. Write `docs/OS/phase4c_plus_test_target.md`
2. Define the test matrix as the acceptance target
3. Define the fake provider interface
4. **Kilo Code reviews test target before implementation begins**

### Step 1: Fake Provider + Executor Tests

1. `adapter/github/fake_provider.go`
2. `adapter/github/provider.go` (interface only)
3. `adapter/github/executor.go` (ActionFunc wrapping provider)
4. `adapter/github/executor_test.go` — all test matrix scenarios
5. All tests pass against fake provider

### Step 2: Executor Selection Security Fix

1. Add action→executor mapping in `service/authority/authority.go`
2. Remove reliance on `params["tool_name"]`
3. Read approved snapshot's `consequence_parameters` as canonical params
4. Update existing integration tests

### Step 3: Kernel CompleteIntent

1. `kernel/sql.go` — add `sqlCompleteIntent`
2. `kernel/authority.go` — add `CompleteIntent` method
3. `kernel/contract.go` — add to `Contract` interface
4. Kernel tests for intent state transition

### Step 4: Real GitHub Provider

1. `adapter/github/provider.go` (real API client, `net/http`)
2. `adapter/github/provider_test.go` (opt-in via build tag)
3. Credential handling: `GITHUB_TOKEN` env var

### Step 5: Wiring + Integration

1. Register executor in `cmd/solvent-mcp/main.go`
2. `examples/github/executor_example.go`
3. Update `docs/api/extensions.md`

### Step 6: Full Verification

```bash
task lint:openapi
task test:openapi
go build ./...
go vet ./...
go test -count=1 -p 1 ./...
go test -count=1 ./...
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

### Real integration (explicit opt-in, never accidental)

```bash
go test -count=1 -v -tags github_integration -run TestRealGitHub ./adapter/github/...
```

Normal `go test ./...` must NEVER trigger real GitHub side effects.

---

## 25. Adversarial Testing

| # | Test | Property | Would fail if security removed? |
|---|------|----------|-------------------------------|
| AT-1 | Remove authorization check → executor never invoked | SI-1, SI-14 | Yes |
| AT-2 | Change target ID → provider not called | SI-7 | Yes |
| AT-3 | Change action → provider not called | SI-7 | Yes |
| AT-4 | Revoke target before execution → no provider call | SI-15 | Yes |
| AT-5 | Substitute arbitrary executor name → rejected | SI-6 | Yes |
| AT-6 | Force executor with denied authorization → fails | SI-1, SI-4 | Yes |
| AT-7 | Return provider error as success → test fails | SI-10, SI-12 | Yes |
| AT-8 | Return executor success without provider call → test fails | SI-14 | Yes |
| AT-9 | Simulate lost response → ambiguous state detected | SI-11 | Yes |
| AT-10 | Reuse stale authorization → test fails | SI-5 | Yes |
| AT-11 | Revoke before execution → result enforced | SI-15 | Yes |
| AT-12 | Audit failure → execution result truthful | CI-3 | Yes |
| AT-13 | Substitute repo → zero provider calls | SI-16, SI-17 | Yes |
| AT-14 | Substitute workflow → zero provider calls | SI-16, SI-17 | Yes |
| AT-15 | Substitute ref → zero provider calls | SI-16, SI-17 | Yes |

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

- [ ] All 32 test matrix scenarios pass against fake provider
- [ ] All 15 adversarial tests pass
- [ ] Executor selection: caller cannot select arbitrary executor
- [ ] Parameter binding: caller cannot override approved values
- [ ] TOCTOU: revocation caught by re-authorization
- [ ] Audit: authorization, invocation, and result are separate entries

### Real integration

- [ ] GitHub workflow dispatch succeeds against test repository
- [ ] Workflow run ID recorded in audit
- [ ] Provider rejection correctly classified

### Independent review

- [ ] Kilo Code independent adversarial review → GO

---

## 27. Rollback

1. Remove executor registration from `main.go` (one line per entry point)
2. `CompleteIntent` is additive — no existing behavior changes
3. Fake provider tests continue to pass (provider is independent)
4. Real provider is optional (gated on build tag + `GITHUB_TOKEN`)

---

## 28. Risks / Open Questions

| # | Risk | Mitigation | Status |
|---|------|-----------|--------|
| R-1 | TOCTOU between authorization and provider call | Documented limitation. Minimized by re-authorization. | Accepted |
| R-2 | GitHub API rate limits | Scope to test repository. | Accepted |
| R-3 | GitHub token rotation | Deployment config. | Accepted |
| R-4 | Test repository availability | Must exist before integration tests. | Open |
| R-5 | CompleteIntent kernel change | Justified by Kernel Growth Gate. Minimal. | Approved |
| R-6 | `params["tool_name"]` caller control | Fix: derive from authorized action. | Approved |
| R-7 | Ambiguous provider outcomes | Classified as operator reconciliation. | Accepted |
| R-8 | Concurrent duplicate execution | Known v1 limitation. | Deferred |
| R-9 | `return_run_details=true` behavior | Verified against GitHub API docs. | Resolved |

---

## 29. Kilo Code Independent Review Gate

### First gate: test target review

```
OpenCode writes test target
    ↓
Kilo Code reviews test target
    ↓
Kilo GO → OpenCode implements
Kilo REQUEST CHANGES → OpenCode revises test target
```

### Second gate: implementation review

```
OpenCode implements
    ↓
OpenCode verifies
    ↓
STOP
    ↓
hand CURRENT repository to Kilo Code
    ↓
Kilo Code independently reviews
    ↓
GO / REQUEST CHANGES
```

---

## 30. Next Phase

Phase 4D: Idempotency and async execution

- Idempotency keys for duplicate prevention
- Async execution with polling
- Workflow status tracking
- Rollback on failure
- Multiple provider support

Explicitly deferred.

---

## 31. Final Recommendation

### Strongest honest security guarantee

> Solvent authorized the action based on current authority state, bound it
> to exact provider-specific parameters from the approved snapshot, invoked
> the executor with those parameters, and the provider accepted the request.
> Solvent does not guarantee that the provider completed the operation.

### Architectural preservation

The kernel does not grow into an execution engine. The executor is registered
in the extension plane. The kernel gains one additive method (`CompleteIntent`)
for an atomic state transition. Provider-specific parameters are stored in
`consequence_parameters` (existing JSONB field) and compared semantically
during authorization. Everything else — GitHub API calls, credential handling,
parameter reconstruction — lives above the kernel.

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
