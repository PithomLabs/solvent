Yes. **Proceed with remaining work items 1 and 2, but make the adversarial code review a mandatory gate after implementation and before any Phase 4/6 work.**

The current status says the hardening rules are satisfied, but explicitly acknowledges that the **real execution path still needs to be wired through `authority.ExecuteAction`** and that database-connected integration tests remain. That means the security architecture is not yet proven end-to-end.

Use this as the next coding-agent prompt:

```text
SOLVENT — FINAL REMEDIATION EXECUTION + ADVERSARIAL GATE

The previous security-hardening remediation is complete according
to the implementation report.

Do NOT begin Phase 4 or Phase 6 yet.

There are two remaining work items:

1. Wire the REAL CONSEQUENTIAL EXECUTION PATH through
   service/authority.ExecuteAction.
2. Implement and run DATABASE-CONNECTED INTEGRATION TESTS.

After both are complete, perform a fresh ADVERSARIAL CODE REVIEW
against the entire repository and issue a final GO/HOLD gate.

==================================================
CURRENT STATUS
==================================================

The following hardening rules have already been implemented:

- ExecuteAction uses internally-resolved executor; no caller injection.
- kernel.Authorize remains the final authority oracle.
- MCP handleSolventAuthorizeAction is wired through authority.PrepareForAction.
- Intent creation is explicitly separated from execution.
- Policy returns PolicyConstraints rather than a final authorization boolean.
- service/authority, service/policy, service/audit, service/executor are now
  on production paths.
- Workflow tokens remain non-authoritative.
- Promote remains belief lifecycle and must NOT be coupled to authority.

Do not undo these properties.

==================================================
WORK ITEM 1 — REAL EXECUTION PATH
==================================================

The critical requirement is:

    NO successful current kernel.Authorize
        →
    NO executor invocation.

The current remediation may have wired intent creation correctly while
the actual consequential execution path still bypasses ExecuteAction.

Find EVERY production path capable of causing an external consequential
action.

At minimum inspect:

- MCP
- wizard
- pipeline
- demo/cloud/web paths
- any executor/provider invocation
- any action_intent execution path
- any direct external API call related to consequential actions

Produce a complete inventory before modifying code:

entry point → service → authorization → executor → provider

--------------------------------------------------
1A. ONE EXECUTION PATH
--------------------------------------------------

Enforce exactly one production consequential execution path:

    ExecuteAction
        ↓
    current-state preparation
        ↓
    policy constraints
        ↓
    kernel.Authorize
        ↓
    ExecutionService
        ↓
    internally resolved Executor
        ↓
    provider adapter

No production path may:

- call an executor directly
- call a provider consequential API directly
- treat workflow state as authorization
- treat intent creation as execution
- treat previous authorization as current authorization

If any bypass exists, remove it or route it through ExecuteAction.

--------------------------------------------------
1B. EXECUTION-TIME REVALIDATION
--------------------------------------------------

ExecuteAction must independently revalidate current state immediately
before executor invocation.

Verify current:

- belief state
- evidence
- debt
- contradictions
- policy
- actor
- action
- target
- authority
- target snapshot
- target activation
- target revocation

Do not reuse an authorization decision made when the intent was created.

Do not trust:

- browser state
- workflow token
- prepared state
- cached authority
- previous policy result
- agent-provided approval
- external provider claims

--------------------------------------------------
1C. KERNEL AUTHORITY ORACLE
--------------------------------------------------

The service may gather the context required to construct the
AuthorityTuple.

The service MUST NOT independently determine authority validity.

Forbidden:

    service SQL
      ↓
    service decides authority=true
      ↓
    execute

Required:

    service gathers tuple/context
      ↓
    kernel.Authorize(tuple)
      ↓
    authoritative decision

Do not reimplement:

- activation logic
- revocation logic
- snapshot matching
- exact authority matching
- authority lifecycle semantics

inside service/authority.

--------------------------------------------------
1D. EXECUTOR BOUNDARY
--------------------------------------------------

ExecuteAction MUST resolve the executor internally.

Forbidden:

    ExecuteAction(..., executorFn)

Required:

    ExecuteAction(...)
      ↓
    ExecutionService
      ↓
    internal executor registry
      ↓
    Executor
      ↓
    provider adapter

Caller may provide action parameters.

Caller may NOT provide the execution implementation.

--------------------------------------------------
1E. EXECUTOR REJECTION INVARIANT
--------------------------------------------------

Instrument or test the executor so that the test suite can prove:

    DENIED authorization
        →
    executor was NOT called

This must be observable in tests.

==================================================
WORK ITEM 2 — DATABASE-CONNECTED INTEGRATION TESTS
==================================================

Use the real CockroachDB-backed repository behavior.

Do not replace database integration with mocks where the purpose is
to validate authority and DB invariants.

Run the complete test suite with CockroachDB available.

Create production-path integration tests covering:

1. promoted belief + no authority → DENIED
2. promoted belief + policy allow + no authority → DENIED
3. valid authority + wrong target → DENIED
4. valid authority + wrong action → DENIED
5. revoked authority → DENIED
6. stale workflow token + revoked authority → DENIED
7. fake approval → DENIED
8. agent spoofing HUMAN → DENIED
9. workflow state cannot create authority
10. executor cannot create authority
11. provider output cannot create authority
12. malformed provider output → DENIED / rejected
13. expired workflow token → DENIED where configured
14. invalid workflow transition → DENIED
15. policy ALLOW + authority ABSENT → DENIED
16. policy ALLOW + authority REVOKED → DENIED
17. target mutates between preparation and execution → DENIED
18. action mutates between preparation and execution → DENIED

All 18 MUST exercise the ACTUAL production execution path where
applicable.

Do not satisfy the requirement with isolated helper tests.

==================================================
CRITICAL REGRESSION TESTS
==================================================

These are mandatory and must prove the executor is not called.

TEST A:

    valid authority
        ↓
    preparation succeeds
        ↓
    authority revoked
        ↓
    ExecuteAction
        ↓
    kernel.Authorize
        ↓
    DENIED
        ↓
    executor NOT called

TEST B:

    valid authority
        ↓
    preparation succeeds
        ↓
    target changes
        ↓
    ExecuteAction
        ↓
    DENIED
        ↓
    executor NOT called

TEST C:

    valid authority
        ↓
    preparation succeeds
        ↓
    action changes
        ↓
    ExecuteAction
        ↓
    DENIED
        ↓
    executor NOT called

==================================================
TOKEN TEST
==================================================

Preserve:

    opaque DB-backed workflow token
        ≠
    self-authenticating authority token

Do NOT add HMAC/JWT merely to satisfy testing.

The required property is:

    valid stale workflow token
    +
    revoked current authority
    =
    DENIED

The production execution path must prove this.

==================================================
ACTOR / AUTHENTICATION
==================================================

Verify that no client-controlled value becomes authentication.

Never trust:

- request.body.actor
- request.body.actor_type
- request.body.action_source
- request.body.user_typed
- HUMAN supplied by caller

Required concept:

    trusted authenticated principal
        ↓
    actor classification
        ↓
    service

If trusted authentication is unavailable, human-only consequential
operations must fail closed.

Do not create an identity provider as part of this task.

==================================================
POLICY
==================================================

Policy is a constraint layer.

It must NOT become a second authority engine.

Prove:

    policy ALLOW
    +
    authority ABSENT
    =
    DENIED

Prove:

    policy ALLOW
    +
    authority REVOKED
    =
    DENIED

Prove:

    policy ALLOW
    +
    authority VALID
    +
    WRONG TARGET
    =
    DENIED

Final authorization must require:

    policy constraints satisfied
    +
    kernel.Authorize(current authority)

==================================================
INTENT VS EXECUTION
==================================================

Intent creation is NOT execution.

An earlier authorization check during intent creation must NOT be
treated as sufficient for later execution.

Required:

    request
      ↓
    intent creation

and later:

    ExecuteAction
      ↓
    current-state revalidation
      ↓
    kernel.Authorize
      ↓
    executor

Never reuse stale authorization results.

==================================================
AUDIT
==================================================

Ensure the real production path records separate events for:

- authorization_checked
- authorization_granted
- authorization_denied
- adapter_invoked
- provider_responded
- executor_completed
- executor_failed

Preserve:

    authorization succeeded + execution failed

as different from:

    authorization denied + execution never happened

Do not move authority truth into audit records.

==================================================
PRODUCTION PATH DISCOVERY
==================================================

Search the entire repository for:

- executor invocations
- provider API calls
- external side effects
- action execution
- deployment
- send
- mutate
- write operations
- direct kernel.Store use
- direct provider-client use

For every result classify:

    SAFE
    AUTHORITY-GATED
    READ-ONLY
    DEAD CODE
    BYPASS

Any consequential BYPASS must be fixed before GO.

==================================================
AFTER IMPLEMENTATION — ADVERSARIAL CODE REVIEW
==================================================

Once Work Items 1 and 2 are complete, STOP implementation.

Perform a fresh adversarial review of the entire repository.

Review:

- kernel
- authority-core schema
- service layer
- policy
- workflow
- workflow_token
- MCP
- wizard
- pipeline
- adapter
- executor
- audit
- integration tests
- production entry points

Attack the architecture rather than merely checking that tests pass.

Specifically attempt to find:

1. Any path around ExecuteAction.
2. Any path around kernel.Authorize.
3. Any way policy can create authority.
4. Any way workflow state can create authority.
5. Any way tokens can substitute for authority.
6. Any stale-state execution.
7. Any wrong-target execution.
8. Any wrong-action execution.
9. Any actor spoofing.
10. Any client-controlled authentication.
11. Any provider-created authority.
12. Any executor-created authority.
13. Any hidden direct provider execution path.
14. Any audit path that falsely conflates authorization with execution.
15. Any second authority engine.
16. Any new kernel/schema expansion not justified by the plan.
17. Any dead security boundary.
18. Any test that passes without exercising the real security boundary.

==================================================
ADVERSARIAL REVIEW OUTPUT
==================================================

Produce:

A. Current execution-path diagram

B. Previously bypassing paths and whether each is fixed

C. List of ALL consequential-action entry points

D. List of ALL executor invocation sites

E. List of ALL external consequential provider calls

F. Exact location of current kernel.Authorize call

G. Exact authority tuple fields checked

H. Exact revocation check

I. Exact actor/authentication boundary

J. Exact policy constraint boundary

K. Exact workflow-token role

L. Authority-related DB reads immediately before execution

M. Audit event sequence

N. Integration-test matrix with ACTUAL results

O. All files changed

P. All migrations changed/added

Q. Kernel changes

If none:

    No kernel changes.

R. Remaining risks

==================================================
IMPORTANT: EXTERNAL EXECUTION TOCTOU
==================================================

Do NOT claim that Solvent can atomically coordinate its database
authorization transaction with an external provider side effect.

The guaranteed property for this MVP is:

    successful current kernel authorization
    immediately precedes executor invocation

There may be an unavoidable distributed race between authorization
and an external provider side effect.

Do not simulate an atomic external transaction.

Do not introduce execution-effect verification machinery.

Document this as a known limitation if applicable.

==================================================
FINAL SECURITY GATE
==================================================

After implementation and adversarial review, end with exactly one:

GO

only if ALL of the following are true:

- every consequential production execution path uses ExecuteAction
- no executor is reachable without successful kernel.Authorize
- current authority is re-read immediately before execution
- exact target/action binding is verified
- revocation is checked
- policy cannot manufacture authority
- workflow tokens cannot manufacture authority
- actor spoofing fails
- authentication is trusted or human-only actions fail closed
- provider output cannot create authority
- executor cannot create authority
- stale authority is rejected
- wrong target/action is rejected
- target/action mutation tests pass
- stale-token revocation test passes
- denied paths prove executor was not called
- database-connected integration tests pass
- audit distinguishes authorization from execution
- no second authority engine exists
- no production bypass exists
- service boundaries are actually on the critical path
- no unexplained kernel/schema growth exists

Otherwise:

HOLD

Do not proceed to Phase 4 or Phase 6.

Security correctness takes precedence over schedule.
```

### One important sequencing decision

I would have the agent do:

```text
Work Item 1
    ↓
Work Item 2
    ↓
FULL ADVERSARIAL CODE REVIEW
    ↓
GO / HOLD
    ↓
only then Phase 4
```

Not:

```text
Work Item 1
→ Work Item 2
→ Phase 4
→ later review
```

The current implementation has already demonstrated why this matters: the previous service architecture looked complete while the actual execution path bypassed it. The remediation's own acceptance criterion now correctly requires that **no executor is reachable without successful current `kernel.Authorize`**. 

One other point: **do not let the agent interpret "database-connected integration tests" as simply running the existing unit suite against CockroachDB.** The valuable tests are those that begin at MCP/wizard/pipeline entry points and demonstrate the actual deny/allow boundary all the way to the executor. The prior adversarial review specifically found that isolated kernel tests passed while the production execution path remained vulnerable. 

This is the right final security gate before we let the implementation move into the UI and demo phases.
