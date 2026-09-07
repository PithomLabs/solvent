# Phase 4C+ Test Engineering Target

**Executable Acceptance Contract for Implementation**

This document is the test engineering specification for Phase 4C+ (First Real
GitHub Executor). It defines the behavior Solvent must prove, the invariants
it must hold, and the exact tests that demonstrate compliance.

No production implementation may begin until Kilo Code approves this target.

**Process:** Documentation only. No production code. No schema changes. No
new services. No kernel redesign. No async polling, retries, multiple
providers, RBAC, or remote execution APIs.

---

## 1. Scope and Behavioral Objective

### What we are proving

Solvent authorizes a consequential external action, binds it to exact
provider-specific parameters frozen at approval time, invokes a real GitHub
Actions workflow dispatch with those exact parameters, and records that the
provider accepted the request. The three facts — authorized / invoked /
provider-accepted — are distinguishable and independently auditable.

### What we are NOT proving

- That the GitHub workflow completed successfully
- That the external side effect succeeded
- That Solvent can handle arbitrary providers
- That Solvent can handle concurrent execution safely

### Execution entry point

Phase 4C+ uses the trusted local MCP execution surface as its execution
entry point. It does not establish a general remote execution API.

---

## 2. Canonical State Model

### Intent lifecycle (existing, extended)

```
PROPOSED (intent_on_promoted)
    ↓
LIVE (state = 'live', belief_status = 'promoted')
    ↓
CANCELLED (state = 'cancelled') — belief retracted or explicit cancel
    ↓
EXECUTED (state = 'executed') — provider accepted request (NEW: Phase 4C+)
```

### Semantic definition of `executed`

`action_intent.state = 'executed'` means:

> The external provider accepted the execution request and returned a
> reference (e.g., workflow run ID).

It does **NOT** mean:

> The provider completed the workflow or the external side effect succeeded.

Provider completion is a separate fact that Solvent does not possess in
Phase 4C+.

### Execution lifecycle

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
1. Authorization       — Solvent determined the action is allowed
2. Invocation          — The executor was called with authorized parameters
3. Provider acceptance — GitHub accepted the dispatch request
4. Provider completion — GitHub completed the workflow (NOT guaranteed by Solvent)
```

These are four separate facts recorded in separate audit entries. A
successful authorization does not prove execution. A successful execution
invocation does not prove provider acceptance. Provider acceptance does not
prove provider completion.

---

## 3. Security Invariants

| # | Invariant |
|---|-----------|
| SI-1 | No external side effect before successful current authorization |
| SI-2 | An executor cannot grant authority |
| SI-3 | An executor cannot revoke authority |
| SI-4 | An executor cannot reinterpret denied authorization as allowed |
| SI-5 | A stale/cached authorization cannot be used |
| SI-6 | Caller-controlled executor selection cannot bypass trusted registry mapping |
| SI-7 | The exact authorized action/target must be the action sent to GitHub |
| SI-8 | Authorization and execution are distinct events |
| SI-9 | Provider success does not retroactively create authority |
| SI-10 | Provider failure does not create false success |
| SI-11 | Audit must distinguish authorization decision, execution attempt, provider result |
| SI-12 | A failed provider operation must not be reported as successful execution |
| SI-13 | A successful authorization does not prove successful external execution |
| SI-14 | External execution must not occur when the authorization step fails |
| SI-15 | Revoked authority must not permit a new execution attempt |
| SI-16 | Provider-specific execution parameters must be derived from the approved snapshot, not caller-supplied |
| SI-17 | Caller-supplied execution parameters must not override approved values |
| SI-18 | The executor may execute only the provider-specific parameters bound to the approved target |

---

## 4. Correctness Invariants

| # | Invariant |
|---|-----------|
| CI-1 | ExecutionResult.Allowed is true if and only if kernel.Authorize returned Allowed=true |
| CI-2 | ExecutionResult.Success is true if and only if the executor function returned nil error |
| CI-3 | Audit entries are ordered: authorization → invocation → result |
| CI-4 | The intent state transitions live → executed only when the provider accepts the request (NOT on provider completion) |
| CI-5 | Executor params match the authorized action/target exactly |
| CI-6 | Provider-specific parameters (repo/workflow/ref) match the approved snapshot exactly |

---

## 5. Failure Model

### Executor-internal failures (zero provider calls)

| # | Failure | Behavior |
|---|---------|----------|
| F-1 | Authorization denied | No provider call. `executor_denied` audit. |
| F-2 | Target revoked | No provider call. `executor_denied` audit. |
| F-3 | Wrong target | No provider call. `executor_denied` audit. |
| F-4 | Wrong action | No provider call. `executor_denied` audit. |
| F-5 | Wrong actor | No provider call. `executor_denied` audit. |
| F-6 | Stale authority (revocation detected before final check) | No provider call. `executor_denied` audit. |
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

## 6. Deterministic Fake-Provider Contract

### Interface

```go
// GitHubProvider is the interface that both fake and real providers implement.
type GitHubProvider interface {
    TriggerWorkflow(ctx context.Context, repo, workflow, ref string,
        inputs map[string]string) (*WorkflowResult, error)
}

type WorkflowResult struct {
    RunID  string
    RunURL string
}
```

### FakeGitHubProvider

```go
type FakeGitHubProvider struct {
    mu            sync.Mutex
    calls         []WorkflowCall
    accept        bool        // true → return 200 + run ID; false → return error
    err           error       // returned when accept=false
    runID         string      // the run ID to return on acceptance
    callWg        sync.WaitGroup // signalled on each call (for test synchronization)
}

type WorkflowCall struct {
    Repo     string
    Workflow string
    Ref      string
    Inputs   map[string]string
    CalledAt time.Time
}
```

### Required methods

```go
func NewFakeGitHubProvider(accept bool, runID string) *FakeGitHubProvider
func (f *FakeGitHubProvider) TriggerWorkflow(ctx, repo, workflow, ref, inputs) (*WorkflowResult, error)
func (f *FakeGitHubProvider) Calls() []WorkflowCall       // returns copy of recorded calls
func (f *FakeGitHubProvider) CallCount() int              // returns len(calls)
func (f *FakeGitHubProvider) LastCall() *WorkflowCall     // returns last call or nil
func (f *FakeGitHubProvider) SetAccept(accept bool)       // change behavior mid-test
func (f *FakeGitHubProvider) SetErr(err error)            // set error response
func (f *FakeGitHubProvider) WaitForCalls(n int, timeout time.Duration) error // sync primitive
```

### Provider side-effect oracle (5 levels)

```
1. Invocation        — executor function was called
2. Request sent      — provider received the HTTP request
3. Provider accepted — provider returned 2xx
4. Run created       — workflow run ID returned
5. Run completed     — workflow finished (NOT guaranteed in 4C+)
```

Phase 4C+ stops at level 4. Tests inspect `Calls()` and `CallCount()` to
verify levels 1-3. Level 4 is verified by the returned `RunID`.

### Deterministic synchronization

No sleep-based tests. Use:
- `FakeGitHubProvider.WaitForCalls(n, timeout)` for provider call confirmation
- `context.WithCancel` for cancellation testing
- CockroachDB `SERIALIZABLE` for authority race testing
- Provider call recording for side-effect verification

---

## 7. Execution Parameter Binding

### Binding source

The approved `target_snapshot.consequence_parameters` is the single source
of truth for provider-specific execution parameters (repo/workflow/ref).
The service layer reads it from the snapshot. Caller-supplied execution
parameters are ignored by the executor.

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

## 8. Executor Selection Security

### Current mechanism (vulnerability)

```go
toolName, _ := params["tool_name"].(string)
fn, ok := s.execReg.Get(toolName)
```

### Required fix

After `PrepareForAction` succeeds, derive `tool_name` from `action`
using a fixed mapping in the service layer. Remove reliance on
`params["tool_name"]`.

### Mapping for Phase 4C+

| Authorized action | Executor name | Provider |
|------------------|---------------|----------|
| `"deploy"` | `"github_trigger_workflow"` | GitHub Actions |

---

## 9. TOCTOU / External Side-Effect Model

### The unavoidable boundary

```
T1: kernel.Authorize succeeds (SERIALIZABLE read)
    ↓
    ** window A: authority may change before final check **
    ↓
T2: final authorization check (PrepareForAction re-reads current state)
    ↓
    ** window B: authority may change after check, before provider call **
    ↓
T3: executor invoked (local function call, no I/O)
    ↓
    ** window C: GitHub state may change **
    ↓
T4: GitHub API accepts/rejects
```

### Window A: authority revoked before final check

If authority is revoked before the final `kernel.Authorize` call (T2),
the re-read catches it. The execution is denied. Zero provider calls.

**Test requirement:** Revocation before T2 → MUST deny, zero provider calls.

### Window B: authority revoked after final check, before provider call

If authority is revoked after the final `kernel.Authorize` succeeds (T2)
but before the executor invokes GitHub (T3), the revocation is not
detected. The provider call proceeds on technically stale authority.

**Why this window exists:** The final authorization check and the executor
invocation are not in the same database transaction. There is no CockroachDB
mechanism that can atomically coordinate "authorization is current" with
"GitHub API call is made."

**Why this window is acceptable for v1:** The window is bounded by a single
local function call with no intervening I/O or database round-trip. Between
the successful authorization check and the executor invocation, there is
no network call, no file write, and no context switch to another
transaction. The window is materially narrower than the multi-transaction
stale-authority race identified in Phase 6.4. The v1 design accepts this
residual TOCTOU window as a documented limitation.

**This does NOT rely on MCP request serialization.** The MCP surface
processes requests sequentially, but revocations can arrive through the
REST API on an independent connection. The window's small size is a
property of the local function-call boundary, not of MCP concurrency.

**Test requirement:** Revocation after T2, before T3 → accepted as
documented v1 race. Test must characterize the behavior, not falsely
require prevention.

### Window C: GitHub state changes after provider call

GitHub state changes (repository deleted, workflow removed) after the
provider call are outside Solvent's control. Documented, not tested.

---

## 10. CompleteIntent

### Semantic definition

`CompleteIntent` transitions `action_intent.state` from `live` to `executed`.
This records that the external provider accepted the execution request. It
does NOT record that the provider completed the workflow.

### Caller constraint (explicit call provenance)

`CompleteIntent` is NOT a general-purpose internal convenience method. It is
NOT reachable through REST, MCP, or caller-controlled input. The call
provenance is strictly:

```
ExecuteAction
    → successful current authorization (kernel.Authorize)
    → approved-parameter reconstruction (from target_snapshot)
    → executor invocation (ActionFunc)
    → provider acceptance (GitHub API returns 2xx)
    → CompleteIntent(intentID)
```

No other code path may call `CompleteIntent`. The method is callable only
by the trusted execution service path (`service/authority.ExecuteAction`)
and only after provider acceptance is confirmed.

### Verification requirement

The test suite must verify that no public or caller-controlled path can
invoke `CompleteIntent` directly. This is a structural invariant enforced
by the call graph, not by runtime checks.

---

## 11. Audit Model

### Audit entries per execution

| Step | ActivityType | Key Fields |
|------|-------------|------------|
| Authorization check | `authorization_granted` or `authorization_denied` | target_id, action, reason |
| Executor denied | `executor_denied` | target_id, action, reason |
| Pre-execution | `adapter_invoked` | target_id, action, tool |
| Execution success | `executor_completed` | target_id, action, output, provider_ref |
| Execution failure | `executor_failed` | target_id, action, error |

### Semantic note

`executor_completed` means the provider accepted the request and returned
a reference. It does NOT mean the provider completed the workflow.

---

## 12. Test Specifications

### Notation

Each test entry specifies:
- **Function:** exact Go test function name
- **Setup:** database state, provider configuration, service wiring
- **Inputs:** parameters passed to ExecuteAction
- **Synchronization:** how races/interleavings are controlled
- **Assertions:** exact conditions checked
- **Provider calls:** expected call count on FakeGitHubProvider
- **Side effect:** expected workflow dispatch behavior
- **Property:** which SI/CI invariant is proved

---

### 12.1 Authorization Tests

#### Test `TestExec01_ValidAuthority Executes`

- **Setup:** Create principal, promote belief, create approved target with
  `consequence_parameters = {"repo":"org/test","workflow":"deploy.yml","ref":"main"}`.
  Register `github_trigger_workflow` executor in registry. Fake provider
  accepts.
- **Inputs:** scenarioID, beliefID, action="deploy", targetID, actorID,
  params=`{"tool_name":"github_trigger_workflow"}`.
- **Synchronization:** None (single goroutine).
- **Assertions:**
  - `result.Allowed == true`
  - `result.Success == true`
  - `result.Output` is non-empty (the run ID)
  - `result.Error == ""`
- **Provider calls:** Exactly 1. `Calls()[0].Repo == "org/test"`,
  `Calls()[0].Workflow == "deploy.yml"`, `Calls()[0].Ref == "main"`.
- **Side effect:** Workflow triggered (via fake).
- **Property:** CI-1, CI-2, SI-1.

#### Test `TestExec02_NoAuthority`

- **Setup:** Create principal, promote belief, create target but do NOT
  approve. Fake provider accepts.
- **Inputs:** scenarioID, beliefID, action="deploy", targetID, actorID,
  params=`{"tool_name":"github_trigger_workflow"}`.
- **Assertions:**
  - `result.Allowed == false`
  - `result.Success == false`
- **Provider calls:** Exactly 0.
- **Side effect:** None.
- **Property:** SI-1, SI-14.

#### Test `TestExec03_RevokedAuthority`

- **Setup:** Create principal, promote belief, create approved target, then
  revoke the target. Fake provider accepts.
- **Inputs:** scenarioID, beliefID, action="deploy", targetID, actorID,
  params=`{"tool_name":"github_trigger_workflow"}`.
- **Assertions:**
  - `result.Allowed == false`
  - `result.Success == false`
- **Provider calls:** Exactly 0.
- **Side effect:** None.
- **Property:** SI-15.

#### Test `TestExec04_WrongTarget`

- **Setup:** Create principal, promote belief, create approved target A.
  Create a second target B (approved). Execute against target B with
  target A's parameters. Fake provider accepts.
- **Inputs:** targetID=targetB_ID, but tuple constructed with targetA's
  dimensions.
- **Assertions:**
  - `result.Allowed == false`
- **Provider calls:** Exactly 0.
- **Side effect:** None.
- **Property:** SI-7.

#### Test `TestExec05_WrongAction`

- **Setup:** Create approved target with action="deploy". Execute with
  action="rollback" (different action).
- **Assertions:**
  - `result.Allowed == false`
- **Provider calls:** Exactly 0.
- **Side effect:** None.
- **Property:** SI-7.

#### Test `TestExec06_WrongActor`

- **Setup:** Create approved target bound to principal A. Execute with
  actorID=principalB.
- **Assertions:**
  - `result.Allowed == false`
- **Provider calls:** Exactly 0.
- **Side effect:** None.
- **Property:** SI-7.

#### Test `TestExec07_ExecutorNotRegistered`

- **Setup:** Create approved target. Do NOT register any executor in the
  registry. Fake provider accepts.
- **Inputs:** scenarioID, beliefID, action="deploy", targetID, actorID,
  params=`{}`.
- **Assertions:**
  - `result.Allowed == true` (authorization succeeds)
  - `result.Success == false` (executor not found)
- **Provider calls:** Exactly 0.
- **Side effect:** None.
- **Property:** CI-1, CI-2.

#### Test `TestExec08_RevocationBeforeFinalCheck`

- **Setup:** Create approved target. Start execution in goroutine A.
  In goroutine B, revoke the target before goroutine A's
  `PrepareForAction` completes its re-read. Use CockroachDB
  `SERIALIZABLE` and deliberate interleaving via channels.
- **Synchronization:**
  1. Goroutine A: begins `ExecuteAction`, blocks at T1 (after initial
     authorization, before re-read).
  2. Goroutine B: revokes target.
  3. Goroutine A: continues re-read (T2), detects revocation.
- **Assertions:**
  - `result.Allowed == false`
  - `result.Success == false`
- **Provider calls:** Exactly 0.
- **Side effect:** None.
- **Property:** SI-5, SI-15.

#### Test `TestExec09_ExecutorSelectionFromCallerParamsRejected`

- **Setup:** Register two executors: `github_trigger_workflow` and
  `malicious_executor`. Create approved target with action="deploy".
- **Inputs:** params=`{"tool_name":"malicious_executor"}` (caller tries
  to select arbitrary executor).
- **Assertions:**
  - Executor invoked is `github_trigger_workflow` (derived from action),
    NOT `malicious_executor`.
  - `malicious_executor` was never called.
- **Provider calls:** Exactly 1 (via correct executor).
- **Side effect:** Workflow triggered via correct executor.
- **Property:** SI-6.

---

### 12.2 Parameter Binding Tests

These tests are CRITICAL. They prove that the executor cannot be tricked
into executing against a different repository, workflow, or ref than what
was approved.

#### Test `TestExec10_SubstituteRepo`

- **Setup:** Create approved target with `consequence_parameters =
  {"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`.
  Register executor. Fake provider accepts.
- **Inputs:** Construct authority tuple with repo="org/malicious" (different
  from approved). The service layer reads the approved snapshot and
  passes those values to the executor.
- **Assertions:**
  - `result.Allowed == false` (parameter mismatch detected by
    kernel.Authorize comparing tuple against snapshot)
- **Provider calls:** Exactly 0.
- **Side effect:** None.
- **Property:** SI-16, SI-17, SI-18, CI-6.

#### Test `TestExec11_SubstituteWorkflow`

- **Setup:** Same as TestExec10 but substitute workflow="malicious.yml".
- **Assertions:** `result.Allowed == false`.
- **Provider calls:** Exactly 0.
- **Property:** SI-16, SI-17, SI-18, CI-6.

#### Test `TestExec12_SubstituteRef`

- **Setup:** Same as TestExec10 but substitute ref="refs/heads/exploit".
- **Assertions:** `result.Allowed == false`.
- **Provider calls:** Exactly 0.
- **Property:** SI-16, SI-17, SI-18, CI-6.

#### Test `TestExec13_SubstituteAnyParameter`

- **Setup:** Same as TestExec10 but add an extra key
  `{"repo":"org/approved","workflow":"deploy.yml","ref":"main","evil":"true"}`.
  The extra key makes the JSONB semantically different.
- **Assertions:** `result.Allowed == false`.
- **Provider calls:** Exactly 0.
- **Property:** SI-16, SI-17, SI-18, CI-6.

#### Test `TestExec14_CallerSuppliedMatchingParams`

- **Setup:** Create approved target with `consequence_parameters =
  {"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`.
- **Inputs:** Caller supplies params that MATCH the approved values:
  `{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`.
- **Assertions:**
  - `result.Allowed == true`
  - `result.Success == true`
  - Fake provider received EXACTLY `repo="org/approved"`,
    `workflow="deploy.yml"`, `ref="main"`.
- **Provider calls:** Exactly 1.
- **Side effect:** Workflow triggered with correct parameters.
- **Property:** CI-5, CI-6.

---

### 12.3 TOCTOU Tests

#### Test `TestExec15A_RevocationBeforeCheck_MustDeny`

- **Setup:** Create approved target. Begin execution. Before the final
  authorization re-read (T2), revoke the target on a separate connection.
- **Synchronization:**
  1. Execution goroutine: calls `PrepareForAction`, blocks before the
     re-read (use a hook or channel).
  2. Revocation goroutine: revokes target via `RevokeTarget`.
  3. Execution goroutine: proceeds with re-read.
- **Assertions:**
  - `result.Allowed == false`
  - Zero provider calls.
- **Property:** SI-5, SI-15.

#### Test `TestExec15B_RevocationAfterCheck_DocumentedRace`

- **Setup:** Create approved target. Begin execution. After the final
  authorization check succeeds (T2) but before the executor invokes the
  provider (T3), revoke the target on a separate connection.
- **Synchronization:**
  1. Execution goroutine: passes `PrepareForAction` (T2 succeeds).
  2. Between T2 and T3: revocation goroutine revokes target.
  3. Execution goroutine: invokes executor (T3).
- **Assertions:**
  - `result.Allowed == true` (check already passed)
  - Provider MAY be called (0 or 1 calls).
  - This is a documented v1 race. Test MUST characterize, not prevent.
  - Test MUST document: "Window B exists. The revocation arrived after
    the final authorization check but before the provider call. The
    provider call proceeded on technically stale authority."
- **Property:** Documented v1 limitation. NOT a security invariant violation.

#### Test `TestExec16_TargetChangedBeforeExecution`

- **Setup:** Create approved target. Modify the target's action between
  authorization check and execution.
- **Synchronization:** Same pattern as TestExec15A but modify the target
  row instead of revoking.
- **Assertions:** `result.Allowed == false`. Zero provider calls.
- **Property:** SI-5.

#### Test `TestExec17_BindingChangedBeforeExecution`

- **Setup:** Create approved target with `consequence_parameters =
  {"repo":"org/approved",...}`. Between T2 and T3, modify the snapshot's
  `consequence_parameters` to `{"repo":"org/changed",...}`.
- **Assertions:** `result.Allowed == false` (parameter mismatch on
  re-check). Zero provider calls.
- **Property:** SI-16, SI-17.

#### Test `TestExec18_StaleExecutionParameters`

- **Setup:** Create approved target. Execute with parameters that matched
  at authorization time but the snapshot was changed before execution.
- **Assertions:** `result.Allowed == false`. Zero provider calls.
- **Property:** SI-5, SI-16.

#### Test `TestExec19_StaleAuthorization`

- **Setup:** Create approved target. Authorize at T1. At T2 (re-read),
  the authority activation has expired or been revoked.
- **Assertions:** `result.Allowed == false`. Zero provider calls.
- **Property:** SI-5.

---

### 12.4 Provider Failure Tests

#### Test `TestExec20_ProviderRejection4xx`

- **Setup:** Create approved target. Fake provider configured to reject
  (accept=false, err=fmt.Errorf("403 Forbidden")).
- **Assertions:**
  - `result.Allowed == true`
  - `result.Success == false`
  - `result.Error` contains the error message.
- **Provider calls:** Exactly 1.
- **Side effect:** Provider was called but rejected.
- **Property:** SI-10, SI-12.

#### Test `TestExec21_ProviderError5xx`

- **Setup:** Fake provider configured with err=fmt.Errorf("500 Internal
  Server Error").
- **Assertions:**
  - `result.Allowed == true`
  - `result.Success == false`
- **Provider calls:** Exactly 1.
- **Property:** SI-10, SI-12.

#### Test `TestExec22_ProviderTimeout`

- **Setup:** Fake provider configured with a delay longer than the test
  context timeout.
- **Assertions:**
  - `result.Allowed == true`
  - `result.Success == false`
  - Error indicates timeout.
- **Provider calls:** Exactly 1 (request was sent).
- **Side effect:** Unknown (provider may or may not have received it).
- **Property:** F-14 classification.

#### Test `TestExec23_ProviderAcceptsButResponseLost`

- **Setup:** Fake provider configured to accept but then simulate a
  response loss (e.g., close connection before reading response).
- **Assertions:**
  - `result.Allowed == true`
  - `result.Success == false` (or ambiguous)
- **Provider calls:** Exactly 1.
- **Side effect:** Unknown. Requires operator reconciliation.
- **Property:** F-15 classification.

#### Test `TestExec24_NetworkErrorBeforeTransmission`

- **Setup:** Fake provider configured to fail before any bytes are sent
  (e.g., connection refused).
- **Assertions:**
  - `result.Allowed == true`
  - `result.Success == false`
- **Provider calls:** 0 or unknown (depending on transport semantics).
- **Property:** F-13 classification.

---

### 12.5 Duplicate Execution Tests

#### Test `TestExec25_SequentialDuplicatePrevention`

- **Setup:** Create approved target. Execute successfully (first call).
  Intent state is now `executed`. Attempt a second execution.
- **Assertions:**
  - First call: `result.Allowed == true`, `result.Success == true`.
  - Second call: `result.Allowed == false` (intent state is `executed`,
    not `live`). The intent state check prevents re-execution.
- **Provider calls:** Exactly 1 (first call only).
- **Side effect:** Workflow triggered once.
- **Property:** CI-4.

#### Test `TestExec26_ConcurrentDuplicateKnownLimitation`

- **Setup:** Create approved target. Launch two execution goroutines
  simultaneously.
- **Assertions:**
  - Both goroutines may proceed past authorization.
  - Both may reach the provider.
  - Provider call count is 0, 1, or 2.
  - Test MUST document: "Concurrent duplicate execution is a known v1
    limitation. The intent state check prevents sequential duplicates
    but not concurrent races. Phase 4D will add idempotency."
- **Property:** Known v1 limitation (documented, not a security invariant).

---

### 12.6 CompleteIntent Tests

#### Test `TestExec27_ProviderAcceptanceCausesExecuted`

- **Setup:** Create approved target with live intent. Execute with fake
  provider accepting.
- **Assertions:**
  - After execution, `action_intent.state == 'executed'`.
  - The intent transitioned from `live` to `executed`.
- **Provider calls:** Exactly 1.
- **Property:** CI-4.

#### Test `TestExec28_ProviderRejectionDoesNotCauseExecuted`

- **Setup:** Create approved target with live intent. Execute with fake
  provider rejecting.
- **Assertions:**
  - After execution, `action_intent.state == 'live'` (NOT `executed`).
  - The intent did NOT transition to `executed`.
- **Provider calls:** Exactly 1.
- **Property:** CI-4, SI-12.

#### Test `TestExec29_AmbiguousProviderDoesNotFalselyEstablishAcceptance`

- **Setup:** Create approved target with live intent. Execute with fake
  provider timing out (ambiguous outcome).
- **Assertions:**
  - After execution, `action_intent.state == 'live'` (NOT `executed`).
  - The intent did NOT transition to `executed`.
- **Provider calls:** Exactly 1.
- **Property:** CI-4, SI-12.

#### Test `TestExec30_CompleteIntentNotCallableFromPublicPath`

- **Setup:** This is a structural/static analysis test. Verify that:
  1. `CompleteIntent` is NOT in the `Contract` interface (or if it is,
     it is documented as internal-only).
  2. No REST handler calls `CompleteIntent`.
  3. No MCP tool calls `CompleteIntent`.
  4. The only call site is in `service/authority.ExecuteAction` (or its
     internal helper), guarded by provider acceptance.
- **Assertions:**
  - grep/AST analysis confirms no public/caller-controlled path invokes
    `CompleteIntent`.
  - The call graph from REST/MCP → `CompleteIntent` does not exist.
- **Property:** SI-2, SI-3, SI-4 (executor cannot manufacture facts).

---

### 12.7 Audit Tests

#### Test `TestExec31_AuditOrdering`

- **Setup:** Create approved target. Execute successfully with fake
  provider.
- **Assertions:**
  - Audit log contains entries in order:
    1. `authorization_granted` (target_id, action)
    2. `adapter_invoked` (target_id, action, tool="github_trigger_workflow")
    3. `executor_completed` (target_id, action, output=runID)
  - Each entry has a distinct timestamp (or sequence number).
- **Property:** CI-3, SI-11.

#### Test `TestExec32_AuditDeniedExecution`

- **Setup:** Create target but do NOT approve. Execute.
- **Assertions:**
  - Audit log contains `authorization_denied` or `executor_denied`.
  - No `adapter_invoked` entry.
  - No `executor_completed` entry.
- **Property:** SI-11.

#### Test `TestExec33_AuditProviderFailure`

- **Setup:** Create approved target. Fake provider rejects (4xx).
- **Assertions:**
  - Audit log contains:
    1. `authorization_granted`
    2. `adapter_invoked`
    3. `executor_failed` (with error message)
  - No `executor_completed`.
- **Property:** SI-11, SI-12.

#### Test `TestExec34_AuditWriteFailureAfterProviderSuccess`

- **Setup:** Create approved target. Fake provider accepts. Simulate
  audit write failure after provider acceptance (e.g., inject error into
  audit service).
- **Assertions:**
  - Execution result remains truthful (`result.Success == true`).
  - Audit entry may be missing (gap), but the execution did happen.
  - Test MUST document: "Audit gap is a known operational limitation.
    The execution result is truthful even when audit fails."
- **Property:** F-20 classification.

---

### 12.8 Adversarial Tests

Each test removes or bypasses a specific security control and verifies that
the relevant test fails.

#### Test `TestAT01_RemoveAuthorizationCheck`

- **Mechanism:** Patch `PrepareForAction` to always return
  `Allowed=true`.
- **Verification:** Execute against a revoked target. The executor IS
  invoked (test fails if authorization check is present).
- **Property proved:** SI-1, SI-14.

#### Test `TestAT02_ChangeTargetID`

- **Mechanism:** After authorization, substitute targetID before
  execution.
- **Verification:** Provider not called (test fails if target binding
  is present).
- **Property proved:** SI-7.

#### Test `TestAT03_ChangeAction`

- **Mechanism:** After authorization, substitute action before execution.
- **Verification:** Provider not called.
- **Property proved:** SI-7.

#### Test `TestAT04_RevokeBeforeExecution`

- **Mechanism:** Revoke target, then attempt execution.
- **Verification:** Zero provider calls.
- **Property proved:** SI-15.

#### Test `TestAT05_SubstituteArbitraryExecutor`

- **Mechanism:** Pass `{"tool_name":"malicious_executor"}` in params.
- **Verification:** Executor invoked is the one derived from action, not
  caller-supplied name.
- **Property proved:** SI-6.

#### Test `TestAT06_ForceExecutorWithDeniedAuth`

- **Mechanism:** Attempt to bypass authorization and force executor
  invocation.
- **Verification:** Executor is never called.
- **Property proved:** SI-1, SI-4.

#### Test `TestAT07_ProviderErrorReportedAsSuccess`

- **Mechanism:** Fake provider returns error.
- **Verification:** `result.Success == false` (test fails if error is
  swallowed).
- **Property proved:** SI-10, SI-12.

#### Test `TestAT08_ExecutorSuccessWithoutProviderCall`

- **Mechanism:** Fake provider records zero calls but executor reports
  success.
- **Verification:** `result.Success == false` or audit shows
  `executor_failed` (test fails if success is claimed without provider
  call).
- **Property proved:** SI-14.

#### Test `TestAT09_LostResponseDetected`

- **Mechanism:** Fake provider accepts but response is lost (ambiguous).
- **Verification:** Intent state remains `live` (NOT `executed`).
- **Property proved:** SI-11, CI-4.

#### Test `TestAT10_StaleAuthorizationReuse`

- **Mechanism:** Capture authorization token/result from a previous
  execution. Attempt to reuse it.
- **Verification:** Second execution denied (re-read catches stale
  state).
- **Property proved:** SI-5.

#### Test `TestAT11_RevokeBeforeExecution_Enforced`

- **Mechanism:** Revocation arrives during the execution window.
- **Verification:** If revocation is before T2 → zero provider calls.
  If after T2 → documented race (Window B).
- **Property proved:** SI-15 (for T-A).

#### Test `TestAT12_AuditFailureExecutionTruthful`

- **Mechanism:** Inject audit write failure after provider acceptance.
- **Verification:** `result.Success == true` (execution happened, audit
  gap exists).
- **Property proved:** CI-3 (audit gap is documented, not hidden).

#### Test `TestAT13_SubstituteRepoZeroCalls`

- **Mechanism:** Same target, same action, different repo in tuple.
- **Verification:** Zero provider calls.
- **Property proved:** SI-16, SI-17.

#### Test `TestAT14_SubstituteWorkflowZeroCalls`

- **Mechanism:** Same target, same action, different workflow in tuple.
- **Verification:** Zero provider calls.
- **Property proved:** SI-16, SI-17.

#### Test `TestAT15_SubstituteRefZeroCalls`

- **Mechanism:** Same target, same action, different ref in tuple.
- **Verification:** Zero provider calls.
- **Property proved:** SI-16, SI-17.

---

## 13. Traceability Matrix

Every material security/correctness invariant maps to one or more concrete
tests.

| Invariant | Test Function(s) | Exact Assertion | What Breaks If Removed |
|-----------|------------------|-----------------|----------------------|
| SI-1 | TestExec02, TestExec03, TestAT01 | `result.Allowed == false`, provider calls == 0 | Executor invoked without authorization |
| SI-2 | TestAT06, TestExec30 | Executor not invoked when auth denied; CompleteIntent not callable from public path | Executor grants authority |
| SI-3 | TestAT06, TestExec30 | Executor cannot revoke; CompleteIntent not callable from public path | Executor revokes authority |
| SI-4 | TestAT06 | Executor not invoked when auth denied | Executor reinterprets denial as allowed |
| SI-5 | TestExec08, TestExec15A, TestExec18, TestExec19, TestAT10 | `result.Allowed == false` on stale state | Stale authorization used |
| SI-6 | TestExec09, TestAT05 | Caller-supplied `tool_name` ignored; correct executor derived from action | Arbitrary executor selected |
| SI-7 | TestExec04, TestExec05, TestExec06, TestAT02, TestAT03 | Wrong target/action/actor → zero provider calls | Wrong operation sent to GitHub |
| SI-8 | TestExec31 | Audit entries distinguish authorization, invocation, result | Authorization and execution conflated |
| SI-9 | TestExec28 | Provider rejection does not create authority | Provider success retroactively grants authority |
| SI-10 | TestExec20, TestExec21, TestAT07 | `result.Success == false` on provider error | Provider error reported as success |
| SI-11 | TestExec31, TestExec34, TestAT09 | Audit entries are distinct; ambiguous detected | Audit cannot distinguish events |
| SI-12 | TestExec20, TestExec28, TestExec29, TestAT07 | `result.Success == false` on provider failure; intent stays `live` | Failed operation reported as success |
| SI-13 | TestExec02, TestExec03 | Authorization alone does not prove execution | Authorization proves execution |
| SI-14 | TestExec02, TestExec03, TestAT01, TestAT08 | Zero provider calls when auth fails | Execution without authorization |
| SI-15 | TestExec03, TestExec08, TestExec15A, TestAT04, TestAT11 | Revoked target → zero provider calls | Revoked authority permits execution |
| SI-16 | TestExec10-13, TestAT13-15 | Substituted params → zero provider calls | Caller overrides approved params |
| SI-17 | TestExec10-13, TestExec17, TestAT13-15 | Substituted params → zero provider calls | Caller overrides approved params |
| SI-18 | TestExec10-14 | Executor uses only approved params | Executor uses unapproved params |
| CI-1 | TestExec01, TestExec02, TestExec07 | `Allowed == true` iff `kernel.Authorize` returned true | Result inconsistent with kernel |
| CI-2 | TestExec01, TestExec07, TestExec20 | `Success == true` iff executor returned nil | Result inconsistent with executor |
| CI-3 | TestExec31, TestExec34, TestAT12 | Audit entries ordered: auth → invoke → result | Audit ordering violated |
| CI-4 | TestExec25, TestExec27, TestExec28, TestExec29, TestAT09 | Intent `live→executed` only on provider acceptance | Intent transitions without acceptance |
| CI-5 | TestExec14 | Executor params match authorized action/target | Params diverge from authorization |
| CI-6 | TestExec10-14 | Provider params match approved snapshot exactly | Provider params diverge from snapshot |

---

## 14. Acceptance Criteria

### Build and test

- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `go test -count=1 -p 1 ./...` passes
- [ ] `go test -count=1 ./...` passes
- [ ] `task lint:openapi` passes (0 errors)
- [ ] `task test:openapi` passes (7/7)

### Executor tests (fake provider)

- [ ] TestExec01 through TestExec34 pass
- [ ] TestAT01 through TestAT15 pass
- [ ] All parameter binding tests (TestExec10-14) verify provider calls == 0
    for rejected substitutions
- [ ] TOCTOU tests distinguish Window A (must deny) from Window B
    (documented race)
- [ ] CompleteIntent tests verify intent state transitions
- [ ] Audit tests verify ordering and distinct entry types
- [ ] Duplicate execution tests distinguish sequential (prevented) from
    concurrent (known limitation)

### Structural verification

- [ ] `CompleteIntent` is not callable from REST or MCP handlers
- [ ] Executor selection is derived from action, not caller params
- [ ] `consequence_parameters` are read from approved snapshot, not
    caller-supplied

### Real integration (explicit opt-in)

- [ ] GitHub workflow dispatch succeeds against test repository
- [ ] Workflow run ID recorded in audit
- [ ] Provider rejection correctly classified
- [ ] Real tests gated behind build tag (`-tags github_integration`) or
    explicit `-run` flag
- [ ] Normal `go test ./...` NEVER triggers real GitHub side effects

### Independent review

- [ ] Kilo Code independent adversarial review → GO

---

## 15. Implementation Notes

### Test infrastructure extensions

The following new test infrastructure is required (documentation only, no
production code):

1. **`adapter/github/fake_provider.go`** — `FakeGitHubProvider` as
   specified in Section 6.
2. **`adapter/github/provider.go`** — `GitHubProvider` interface
   (exported, shared by fake and real).
3. **`adapter/github/executor.go`** — `ActionFunc` wrapping provider
   (reads params, calls `TriggerWorkflow`).
4. **`adapter/github/executor_test.go`** — TestExec01-34 and TestAT01-15.

### Existing test infrastructure reused

- `executor.RecordingFunc` — for service-level tests that need a test
  executor without GitHub semantics.
- `testdb.SuiteDSN` / `testdb.Reset` — for per-package DB isolation.
- `kernel/suite_test.go` `recorder` — for behavioral receipt recording.
- CockroachDB `SERIALIZABLE` — for authority race testing.

### Test file locations

| File | Contents |
|------|----------|
| `adapter/github/fake_provider.go` | FakeGitHubProvider, WorkflowCall |
| `adapter/github/provider.go` | GitHubProvider interface, WorkflowResult |
| `adapter/github/executor.go` | NewWorkflowTriggerExecutor (ActionFunc) |
| `adapter/github/executor_test.go` | TestExec01-34, TestAT01-15 |
| `adapter/github/provider_test.go` | Real integration tests (opt-in) |
| `service/authority/authority_integration_test.go` | Extended with execution tests |

---

## 16. Process Rules

- Documentation only. No production implementation.
- Do not modify kernel/service/adapter behavior.
- Do not add schema changes.
- Do not create a new service.
- Do not generalize the executor architecture.
- Do not add async polling, retries, multiple providers, RBAC, or remote
  execution APIs.
- Do not soften the known TOCTOU limitation.
- Do not replace provider acceptance with provider completion.
- Do not merely copy the plan sections. Turn them into an executable test
  specification with concrete test names, assertions, synchronization, and
  traceability.
