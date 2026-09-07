Use this as the **fresh, independent adversarial review prompt**. It deliberately tells the reviewer to ignore the “all tasks complete” claim and verify the actual repository state.

```text
PHASE 4C+ — FINAL INDEPENDENT ADVERSARIAL SECURITY / ARCHITECTURE REVIEW

You are acting as an INDEPENDENT senior security architect performing the final
acceptance review of Solvent Phase 4C+.

The implementation has already been revised after a previous adversarial review
that returned NO-GO.

Do NOT assume the new implementation is correct merely because:
- the developer reports completion
- tests pass
- a previous reviewer found the kernel/service design sound
- a summary claims findings are closed

Your job is to attempt to BREAK the implementation.

This is a READ-ONLY review.

DO NOT:
- modify files
- fix code
- refactor
- add implementation
- rewrite tests
- weaken acceptance criteria
- create commits

You may run tests, inspect source, inspect git state, grep the repository, and
perform static/security analysis.

==================================================
REPOSITORY
==================================================

Review the actual current repository, not a developer summary.

Repository:

    ~/Documents/go/solvent-main

First establish the actual working-tree state:

    git status --short
    git branch --show-current
    git log -1 --oneline

Then inspect all relevant production and test code.

==================================================
PREVIOUS NO-GO FINDINGS
==================================================

A previous independent review identified:

    F-1  CRITICAL  Missing adapter executor test suite
    F-2  CRITICAL  GitHub executor not registered in production
    F-3  CRITICAL  FakeGitHubProvider unused
    F-4  HIGH      Missing TestExec36
    F-5  HIGH      Missing TestExec35
    F-6  HIGH      Missing TestExec38
    F-7  HIGH      Missing TestAT01–16
    F-8  MEDIUM    Missing GitHub executor example
    F-9  LOW       Committed 8.6MB ELF binary

The implementation now claims these have been fixed.

Independently verify every one.

Do NOT accept a claim such as "F-1 closed" merely because the file exists.
Verify that the tests actually execute and exercise the intended production path.

==================================================
CURRENT IMPLEMENTATION CLAIM
==================================================

The current implementation reportedly contains:

    adapter/github/executor_test.go
        55 test functions
        TestExec01–38
        TestAT01–16
        existing test(s)

    adapter/github/http_provider.go
        real HTTP GitHub Actions workflow_dispatch provider

    examples/github/executor/main.go
        runnable executor example

    cmd/solvent-mcp/main.go
        GitHub executor registration gated on GITHUB_TOKEN

    examples/github/github
        removed

It also reports:

    go build ./...       PASS
    go vet ./...         PASS
    go test -count=1 ./... PASS
    go test ./adapter/github ... 55/55 PASS

Verify all of these independently.

IMPORTANT:
The reported verification does NOT show:

    go test -race ./...

Therefore race/concurrency behavior must be explicitly investigated.

==================================================
AUTHORITATIVE PHASE 4C+ ARCHITECTURE
==================================================

The approved execution path is:

    PrepareForAction
        ↓
    kernel.Authorize
        ↓
    AuthorizeResult.ConsequenceParameters
        ↓
    service-side JSON reconstruction
        ↓
    exact intentID state verification
        ↓
    fixed authorized-action → executor mapping
        ↓
    GitHub executor
        ↓
    GitHub provider
        ↓
    CompleteIntent(exact intentID)

The following invariants are NON-NEGOTIABLE.

1. kernel.Authorize is the sole authority oracle.

2. TOKEN != AUTHORITY.

3. Authorize != Execute.

4. Snapshot consequence_parameters are the sole source of provider execution
   parameters.

5. Snapshot parameters flow through:

       AuthorizeResult.ConsequenceParameters

6. Caller-supplied execution parameters must never determine the actual provider
   repo/workflow/ref.

7. Caller-supplied tool_name/executor name must never select an arbitrary
   consequential executor.

8. Executor selection must be a fixed mapping from the authorized action.

9. ExecuteAction must use the exact supplied intentID.

10. No arbitrary live intent may be rediscovered using a weak query such as:
        SELECT ... LIMIT 1

11. CompleteIntent(exact intentID) is the only newly approved kernel primitive.

12. CompleteIntent occurs only after provider acceptance.

13. CI-2 remains authoritative:

        provider/executor returns nil error
            => result.Success == true

    CompleteIntent persistence failure must NOT overwrite this success result.

14. Provider rejection/error leaves the intent live.

15. Provider acceptance is not the same as provider workflow completion.

16. Provider acceptance followed by Solvent persistence failure must be represented
    distinctly as:
        intent_completion_failed

17. Window A:
    revocation before final authorization must deny execution.

18. Window B:
    revocation after final authorization but before provider invocation is a known
    v1 TOCTOU limitation.

19. Do NOT claim atomic DB authorization + external provider execution.

20. There is exactly one additional DB read between T2 and T3:
        exact intent-state verification

    Snapshot parameters are already returned by AuthorizeResult.

21. There must be no second snapshot lookup after Authorize.

22. No GetSnapshotConsequenceParams kernel method exists or is required.

23. GitHub-specific semantics must remain in adapter/github.

24. GitHub provider code must not become part of the kernel authority model.

25. The executor must not have access to Solvent authority mutation operations.

26. The executor must not approve, promote, revoke, create authority, or otherwise
    mutate authority state.

27. REST/MCP/API surfaces must not become a second authorization engine.

28. CompleteIntent must not be exposed as an independent public mutation endpoint.

==================================================
PART I — VERIFY PREVIOUS NO-GO FINDINGS
==================================================

For each F-1 through F-9, independently determine:

    CLOSED
    PARTIALLY CLOSED
    STILL OPEN

Do not rely on comments or summaries.

### F-1 / F-4 / F-5 / F-6 / F-7

Verify:

    adapter/github/executor_test.go

Check that:

    TestExec01–38 actually exist
    TestAT01–16 actually exist
    all required tests are discovered by `go test`
    they exercise real executor/service behavior
    they are not dead helper functions
    they do not merely duplicate implementation logic

Pay particular attention to:

    TestExec35
    TestExec36
    TestExec38
    TestExec15A
    TestExec15B

### F-3

Verify that FakeGitHubProvider is genuinely used by the tests.

Search for all references:

    grep -R "FakeGitHubProvider" -n .

Confirm that the tests actually use its:
- call recording
- parameter capture
- call count
- acceptance/rejection
- error injection
- lost-response behavior
- synchronization

### F-2

Verify that the real GitHub executor is actually registered.

Inspect:

    cmd/solvent-mcp/main.go

Confirm:
- executor registry is populated
- GitHub registration actually executes
- registration is appropriately gated on GITHUB_TOKEN
- no credentials are hard-coded
- no credential is passed through execution params
- no credential is logged
- missing GITHUB_TOKEN behaves as intended

Also inspect every production entry point that can reach ExecuteAction.

### F-8

Verify the example is genuinely runnable and demonstrates the intended execution
path.

It must not bypass authority/service logic.

### F-9

Verify the binary is genuinely removed from git tracking.

Check:

    git ls-files examples/github/github
    find examples/github -maxdepth 2 -type f

==================================================
PART II — PARAMETER BINDING ATTACK
==================================================

Try to prove that caller input can influence the external GitHub request.

Inspect the complete call chain:

    API/MCP/request params
        ↓
    ExecuteAction
        ↓
    authorization
        ↓
    parameter reconstruction
        ↓
    executor
        ↓
    provider
        ↓
    HTTP request

Verify that the actual provider receives ONLY the approved snapshot values.

Attack with:

    matching caller params + extra fields
    conflicting caller repo
    conflicting caller workflow
    conflicting caller ref
    arbitrary provider-looking fields
    tool_name
    executor name
    provider URL
    token-like parameters
    mutable maps
    extra JSON fields

The provider invocation must remain controlled exclusively by approved snapshot
data and fixed executor routing.

==================================================
PART III — EXECUTOR SELECTION ATTACK
==================================================

Attempt to cause a caller to execute:

- an arbitrary registered executor
- an unintended provider
- a fake or test executor
- an executor selected through params["tool_name"]
- an executor selected through a renamed parameter
- a provider chosen through arbitrary action input

Verify the authorized action is the sole source of executor selection.

==================================================
PART IV — AUTHORITY / INTENT ATTACKS
==================================================

Attempt:

- wrong actor
- wrong principal
- wrong target
- wrong action
- wrong consequence parameters
- revoked target
- stale authority
- nonexistent intent
- cancelled intent
- executed intent
- another user's intent
- another scenario's intent
- duplicate intentID
- mismatched target/intent
- mismatched scenario/intent

Confirm the exact intentID is propagated through the entire execution path and into:

    CompleteIntent(id, scenarioID)

Verify there is no accidental intent substitution.

==================================================
PART V — REVOCATION / TOCTOU
==================================================

Explicitly analyze:

### Window A

Revocation before final authorization.

Expected:

    DENY
    provider not invoked

### Window B

Revocation after final authorization but before provider invocation.

Expected:

    known v1 limitation
    no false atomicity claim

Determine whether the implementation accidentally claims stronger guarantees than
this.

Also inspect concurrent behavior.

Attempt:

    concurrent ExecuteAction
    concurrent revoke + execute
    concurrent CompleteIntent
    duplicate execution attempts
    retry after provider acceptance

Determine whether two concurrent calls can both cause an external GitHub side
effect.

If duplicate execution is possible, determine whether that is:
- an intended documented v1 limitation
- accidentally introduced by the implementation
- falsely described as impossible

==================================================
PART VI — COMPLETEINTENT SECURITY
==================================================

Review CompleteIntent extremely carefully.

Verify:

- exact intentID
- scenario binding
- state transition requirements
- cannot complete another intent
- cannot complete cancelled intent
- cannot complete already executed intent
- cannot be invoked directly through REST/MCP
- cannot create authority
- cannot revoke authority
- cannot promote beliefs
- cannot modify target activation/snapshot/principal state

Inspect SQL predicates and transaction behavior.

==================================================
PART VII — PROVIDER TRUST BOUNDARY
==================================================

Review:

    adapter/github/provider.go
    adapter/github/http_provider.go
    adapter/github/executor.go

Verify the provider is a dumb external side-effect adapter.

It must not:
- authorize
- approve
- promote
- revoke
- create Solvent authority
- inspect Solvent kernel state
- reinterpret authority
- accept caller-selected credentials
- become a second policy engine

Verify GitHub-specific details remain entirely outside the kernel.

==================================================
PART VIII — REAL HTTP PROVIDER REVIEW
==================================================

Treat the real provider as security-sensitive code.

Inspect:

- HTTP method
- endpoint construction
- owner/repo handling
- workflow path construction
- ref handling
- request body
- headers
- Authorization handling
- response status handling
- response body handling
- timeout behavior
- connection errors
- redirect behavior
- TLS assumptions
- token exposure
- logging
- error messages
- accidental credential leakage

Look for:
- URL/path injection
- header injection
- credential leakage
- accepting an HTTP 2xx that is not actually provider acceptance
- treating workflow completion as acceptance
- silently accepting malformed responses
- unsafe redirects
- unbounded request behavior
- ambiguous error semantics

Do not demand an entire hardened HTTP framework.
Report only materially relevant issues within this phase.

==================================================
PART IX — PROVIDER ACCEPTANCE / FAILURE SEMANTICS
==================================================

Verify all distinct states:

A. Provider accepts request.

Expected:

    executor nil error
    Success == true
    CompleteIntent attempted

B. Provider definitively rejects/fails.

Expected:

    Success == false
    intent remains live
    executor_failed audit

C. Provider accepts but Solvent receives a later persistence error.

Expected:

    Success remains true under CI-2
    intent may remain live
    intent_completion_failed audit
    no false claim that provider failed

D. Ambiguous/lost provider response.

Verify semantics are honest and do not imply completion.

==================================================
PART X — AUDIT INTEGRITY
==================================================

Inspect all execution-related audit events:

    authorization granted
    authorization denied
    executor denied
    adapter invoked
    provider accepted
    provider failed
    intent completion failed

Verify:
- correct ordering
- correct classification
- no contradictory success/failure events
- no omission of security-significant failures
- audit persistence failures do not rewrite already established provider facts
- provider acceptance is distinguishable from workflow completion

Do not accept event names alone.
Trace actual code paths.

==================================================
PART XI — ADVERSARIAL TEST QUALITY
==================================================

Do not only verify that the tests PASS.

Determine whether the tests would FAIL if each security control were removed.

For a sample of adversarial tests, mentally or structurally mutate the implementation:

- remove authorization check
- use caller params
- restore params["tool_name"] executor selection
- remove exact intentID
- skip CompleteIntent state predicate
- allow provider output to modify authority
- skip revocation check
- invoke executor before authorization
- use arbitrary snapshot parameters
- permit public CompleteIntent

Determine whether the corresponding test would actually catch the regression.

Flag tests that are:
- tautological
- too weak
- testing mocks instead of production paths
- accidentally coupled to implementation details
- unable to detect the security regression they claim to cover

==================================================
PART XII — RACE / CONCURRENCY
==================================================

Run:

    go test -race ./...

If it fails, investigate the actual cause.

Inspect:
- FakeGitHubProvider synchronization
- shared test state
- call recording
- WaitForCalls
- concurrent intent execution
- concurrent revocation
- concurrent provider invocation
- test isolation

Do not dismiss races merely because ordinary `go test` passes.

==================================================
PART XIII — STATIC / BUILD VERIFICATION
==================================================

Run:

    go build ./...
    go vet ./...
    go test -count=1 ./...
    go test -race ./...

Also run focused suites:

    go test ./kernel/...
    go test ./service/authority/...
    go test ./adapter/github/...

Verify all expected tests are actually discovered.

==================================================
PART XIV — ARCHITECTURAL DRIFT
==================================================

Search specifically for accidental architecture expansion.

Look for:

- new authorization logic outside kernel
- new authority mutation paths
- new kernel primitives
- GitHub-specific kernel knowledge
- hidden service-level authority rules
- caller-controlled executor routing
- token-as-authority assumptions
- public CompleteIntent
- provider-controlled authority
- UI/MCP acting as authorization layer
- new schema semantics
- generic executor framework added unnecessarily

The kernel philosophy remains:

    smallest trusted authority core
    everything else in extension/service/deployment plane

==================================================
FINAL VERDICT RULE
==================================================

Return:

    VERDICT: GO

only when there are:

    0 CRITICAL
    0 HIGH
    0 MEDIUM

and the implementation genuinely satisfies the approved Phase 4C+ contract.

Return:

    VERDICT: NO-GO

for any unresolved CRITICAL/HIGH/MEDIUM issue.

LOW and INFO findings do not automatically block GO, but must still be reported.

==================================================
FINAL REPORT FORMAT
==================================================

Start with:

    VERDICT: GO
or
    VERDICT: NO-GO

Then provide:

1. Executive assessment

2. Previous finding closure

    F-1 through F-9:
        CLOSED / PARTIAL / OPEN
        evidence

3. Findings table

    ID | Severity | Area | Finding

4. Detailed findings

For each:
    - exact file
    - function/symbol
    - technical issue
    - attack/failure scenario
    - violated invariant
    - why existing tests catch or fail to catch it
    - remediation direction

Do NOT modify the code.

5. Security invariant matrix

For each important invariant:

    PASS / FAIL / UNPROVEN

with concrete evidence.

6. Test verification

Report actual results of:

    go build ./...
    go vet ./...
    go test -count=1 ./...
    go test -race ./...

and focused suites.

7. Adversarial test-quality assessment

Explain whether the 54 new acceptance/adversarial tests genuinely prove the
security properties they claim to prove.

8. Real GitHub provider review

Cover credential handling, request construction, response semantics, errors,
timeouts, and external-side-effect boundaries.

9. TOCTOU/concurrency analysis

Explicitly cover Window A, Window B, and concurrent duplicate execution.

10. Architectural boundary review

Kernel / service / executor / provider / API / MCP.

11. Final recommendation

State whether Phase 4C+ is actually ready to be declared COMPLETE.

Be skeptical.

The objective is not to confirm that the developer finished the work.

The objective is to determine whether Solvent's security architecture survives
an adversarial attempt to break it.
```

This is the review I would run next. The previous reviewer found the kernel/service design sound but the acceptance layer disconnected; the new review therefore needs to verify the **actual code path from authority → snapshot → exact intent → real GitHub HTTP side effect**, not just count tests. 

One particularly important addition is the real HTTP-provider attack surface. The previous review never reached that layer because the provider was missing; now that it exists, it deserves a fresh security examination rather than inheriting a “GO” assumption from the earlier service-level review.
