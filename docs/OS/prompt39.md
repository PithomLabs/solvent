Use this as the **final independent adversarial review prompt**:

```text
PHASE 4C+ — FINAL INDEPENDENT ADVERSARIAL SECURITY / ARCHITECTURE REVIEW

You are acting as an INDEPENDENT senior security architect performing the final
acceptance review of Solvent Phase 4C+.

This repository has already gone through multiple adversarial review rounds.
Previous findings have been remediated, including the final N-1 and N-2 test gaps.

Do NOT assume the implementation is correct because:
- the developer says it is complete
- all tests pass
- previous reviewers found the architecture sound
- findings are marked CLOSED
- the implementation follows the written plan

Your job is to attempt to BREAK the actual implementation.

This is a READ-ONLY review.

DO NOT:
- modify files
- fix code
- refactor
- add implementation
- weaken tests
- rewrite acceptance criteria
- create commits

You may:
- inspect all source code
- inspect git state
- grep/search the repository
- run tests
- run static analysis
- inspect SQL
- inspect concurrency behavior
- trace production call paths
- reason about adversarial inputs and races

==================================================
REPOSITORY
==================================================

Review the actual current repository:

    ~/Documents/go/solvent-main

First establish repository state:

    git status --short
    git branch --show-current
    git log -1 --oneline

Do NOT rely on screenshots or developer summaries as proof.

==================================================
CURRENT CLAIMED STATE
==================================================

The latest implementation claims:

- all prior F-1 through F-9 findings are closed
- N-1 TestExec34 is now a real audit-failure injection test
- N-2 TestExec15B now deterministically characterizes Window B
- real HTTP GitHub provider exists
- GitHub executor is production-registered when GITHUB_TOKEN is present
- FakeGitHubProvider is actively used
- 55 GitHub adapter test functions exist/pass
- full normal tests pass
- go vet passes
- go build passes
- race tests pass for the affected package
- latest full `go test ./...` passes

Independently verify every material claim.

==================================================
AUTHORITATIVE ARCHITECTURAL CONTRACT
==================================================

The intended execution path is:

    PrepareForAction
        ↓
    kernel.Authorize
        ↓
    AuthorizeResult.ConsequenceParameters
        ↓
    service-side reconstruction
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

These invariants are NON-NEGOTIABLE.

1. kernel.Authorize is the sole authority oracle.

2. TOKEN != AUTHORITY.

3. Authorize != Execute.

4. Approved snapshot consequence_parameters are the sole source of execution
   parameters.

5. Snapshot parameters flow through:

       AuthorizeResult.ConsequenceParameters

6. Caller-supplied execution parameters must not determine the actual GitHub
   repository, workflow, ref, or other consequential provider parameters.

7. Caller-supplied `tool_name`, executor name, or equivalent capability selector
   must not determine which consequential executor is invoked.

8. Executor selection is a fixed mapping from the authorized action.

9. ExecuteAction uses the exact requested intentID.

10. No arbitrary intent rediscovery via weak selection such as:
        SELECT ... LIMIT 1

11. CompleteIntent is the only newly approved kernel lifecycle primitive.

12. CompleteIntent occurs only after provider acceptance.

13. CI-2 is authoritative:

        executor returns nil error
            =>
        result.Success == true

    CompleteIntent failure must not overwrite provider success.

14. Provider rejection/failure leaves intent live.

15. Provider acceptance is NOT provider workflow completion.

16. Provider acceptance followed by Solvent persistence failure must be represented
    distinctly, including `intent_completion_failed` where appropriate.

17. Window A:
    revocation before final authorization check
    =>
    execution denied.

18. Window B:
    revocation after final authorization check but before provider invocation
    =>
    known/documented v1 TOCTOU limitation.

19. There is NO claim of atomic database authorization + external GitHub execution.

20. Exactly one additional DB read occurs after T2:
       exact intent-state verification

    Snapshot parameters are already present in AuthorizeResult.

21. There must be no second snapshot DB query.

22. No `GetSnapshotConsequenceParams` method exists or is required.

23. GitHub semantics remain entirely outside the kernel.

24. Executor code cannot mutate Solvent authority state.

25. Executor cannot approve, promote, revoke, create authority, or otherwise alter
    authority state.

26. REST/MCP/API surfaces must not become a second authorization engine.

27. CompleteIntent must not be publicly exposed as an independent mutation path.

28. The kernel remains the smallest trusted authority core.

==================================================
PART I — PREVIOUS FINDINGS
==================================================

Independently verify closure of:

    F-1  adapter executor tests
    F-2  production executor registration
    F-3  FakeGitHubProvider usage
    F-4  TestExec36
    F-5  TestExec35
    F-6  TestExec38
    F-7  TestAT01–16
    F-8  executor example
    F-9  binary removal

Also verify:

    N-1  TestExec34 audit failure injection
    N-2  TestExec15B Window B characterization

Do not merely verify that the test names exist.

Verify that each test executes meaningful production behavior and would fail if
the relevant security property were removed.

==================================================
PART II — TEST SUITE INTEGRITY
==================================================

Inspect:

    adapter/github/executor_test.go

Verify:

    TestExec01–38
    TestAT01–16

actually execute.

Check for:
- placeholder tests
- log-only tests
- tests that never reach production code
- tests that merely assert constants
- tautological assertions
- tests whose setup accidentally proves the implementation instead of the
  security invariant
- mocks that bypass the important boundary
- tests that pass even if the security control were removed

Pay special attention to:

    TestExec15B
    TestExec34
    TestExec35
    TestExec36
    TestExec38
    TestAT01–16

==================================================
PART III — TESTEXEC34 AUDIT FAILURE
==================================================

Verify that TestExec34 genuinely performs:

    provider acceptance
        ↓
    CompleteIntent
        ↓
    ActivityExecutorCompleted audit write failure

Verify:

    provider called exactly once
    result.Allowed == true
    result.Success == true
    authority state remains correct
    executor_completed is not falsely recorded as persisted
    audit failure is distinguishable from provider failure

Determine whether the failure is injected at the real audit dependency boundary.

Ensure the test would fail if CI-2 were incorrectly implemented.

==================================================
PART IV — TESTEXEC15B WINDOW B
==================================================

Verify TestExec15B genuinely forces:

    final authorization succeeds
        ↓
    intent-state verification succeeds
        ↓
    deterministic synchronization point
        ↓
    RevokeTarget(targetID)
        ↓
    executor/provider invocation

The test MUST demonstrate that the executor can proceed despite a revocation
that occurs after T2.

Check that:
- no sleep is used
- no timing-dependent race is used
- channels or equivalent deterministic synchronization are used
- revocation is genuinely performed
- revocation occurs after the final authorization/state checks
- provider is not invoked before revocation
- provider is invoked after revocation
- result reflects the documented v1 behavior

Verify specifically that the test uses:

    RevokeTarget(targetID)

or an equally narrow target-level revocation mechanism.

Do NOT accept a normal execution test as Window B characterization.

==================================================
PART V — PARAMETER SOURCE OF TRUTH
==================================================

Trace the complete path:

    caller input
        ↓
    ExecuteAction
        ↓
    kernel.Authorize
        ↓
    AuthorizeResult.ConsequenceParameters
        ↓
    executor
        ↓
    provider
        ↓
    HTTP request

Try to force caller input to control:
- repo
- workflow
- ref
- inputs
- provider URL
- executor selection

Test conflicting and malicious caller values.

Verify that the approved snapshot remains authoritative.

Pay particular attention to mutations of Go maps and conversions between:
    map[string]interface{}
    JSON
    typed provider parameters

Look for accidental reintroduction of caller params after authorization.

==================================================
PART VI — EXECUTOR SELECTION
==================================================

Attempt to inject:
- tool_name
- executor name
- provider name
- registry key
- alternate action
- arbitrary action parameter

Determine whether a caller can select an unintended executor.

Verify fixed:

    authorized action → executor

mapping.

Ensure no hidden fallback allows arbitrary registry lookup from caller data.

==================================================
PART VII — EXACT INTENT BINDING
==================================================

Trace the exact intentID from caller to:

    intent-state verification
        ↓
    CompleteIntent

Verify:
- no arbitrary live-intent discovery
- no SELECT ... LIMIT 1
- scenario binding remains enforced
- target/action/belief relationships remain correct
- cancelled intent cannot execute
- executed intent cannot execute again sequentially
- another intent cannot accidentally be marked executed

Attempt cross-intent and cross-scenario substitution.

==================================================
PART VIII — AUTHORITY / REVOCATION
==================================================

Attempt:
- wrong actor
- wrong principal
- wrong action
- wrong target
- wrong consequence parameters
- revoked target
- stale authority
- nonexistent authority
- malformed IDs

Verify denial occurs at the authority kernel rather than via duplicate
service-level authorization logic.

Check that the service does not silently recreate authority semantics.

==================================================
PART IX — COMPLETEINTENT
==================================================

Review CompleteIntent and its SQL extremely carefully.

Verify:
- exact intentID
- correct scenario binding
- state predicate
- no completion of cancelled intent
- no completion of already executed intent
- no cross-intent modification
- no authority creation
- no belief promotion
- no target modification
- no revocation
- no public API/MCP path

Search the complete repository for every CompleteIntent invocation.

==================================================
PART X — REAL GITHUB PROVIDER
==================================================

Inspect:

    adapter/github/http_provider.go
    adapter/github/executor.go
    adapter/github/provider.go

Review:

### Credentials

Verify:
- GITHUB_TOKEN enters only at provider construction
- token is not a caller parameter
- token is not stored in kernel state
- token is not returned in API responses
- token is not logged
- token is not included in audit metadata

### HTTP request

Verify:
- correct method
- correct GitHub endpoint
- correct path handling
- safe repo/workflow/ref handling
- correct request body
- correct authorization header
- correct GitHub API version/header behavior

### Errors

Inspect:
- network errors
- malformed URL
- non-2xx responses
- malformed response bodies
- lost response
- context cancellation

Ensure provider acceptance and workflow completion are not conflated.

### HTTP client

Check timeout behavior.

A missing explicit timeout is LOW unless it creates a meaningful security or
availability problem in the actual execution environment.

### Response/error leakage

Check whether GitHub response bodies or errors can leak credentials or sensitive
information into:
- logs
- audit entries
- API responses

==================================================
PART XI — EXECUTOR TRUST BOUNDARY
==================================================

Verify the executor has no access to:
- kernel mutation methods
- authority state
- promotion
- revocation
- target creation
- belief creation

Verify its inputs are already validated by the service/kernel path.

Try to find any dependency inversion where provider/executor can reach Solvent
authority state.

==================================================
PART XII — AUDIT INTEGRITY
==================================================

Review the execution audit sequence.

Expected conceptual events include:

    authorization_granted
    adapter_invoked
    executor_completed
    executor_failed
    intent_completion_failed

Check:
- correct ordering
- no contradictory events
- provider acceptance distinguished from completion
- provider failure distinguished from audit failure
- CompleteIntent failure distinguished from provider failure
- audit persistence failure cannot rewrite already-established provider facts

Pay special attention to whether the audit failure injection introduced a
production behavior change.

==================================================
PART XIII — TOCTOU / CONCURRENCY
==================================================

Analyze:

### Window A

Revocation before final authorization.

Expected:
    DENY
    provider not called

### Window B

Revocation after final authorization but before executor.

Expected:
    documented v1 race
    execution may proceed

### Concurrent ExecuteAction

Determine whether two concurrent calls can both:
    pass live-intent verification
    invoke the external provider

This is a known v1 limitation unless the code has changed.

Do not label it a defect merely because it exists.

Do flag:
- accidental stronger claims
- newly introduced race hazards
- data races
- deadlocks
- synchronization bugs
- incorrect intent state transitions

==================================================
PART XIV — RACE DETECTOR
==================================================

Run:

    go test -race ./...

Do not rely on previous results.

If there are failures, investigate them.

Pay special attention to:
- FakeGitHubProvider
- beforeExecute hook
- audit test doubles
- concurrent test state
- WaitForCalls
- shared DB state

==================================================
PART XV — FULL VERIFICATION
==================================================

Run:

    go build ./...
    go vet ./...
    go test -count=1 ./...
    go test -race ./...

Also:

    go test -count=1 ./adapter/github/...
    go test -count=1 ./service/authority/...
    go test -count=1 ./kernel/...

Verify actual test discovery.

==================================================
PART XVI — ARCHITECTURAL DRIFT
==================================================

Search for any accidental introduction of:

- second authorization engine
- hidden authority checks
- new kernel primitives
- GitHub-specific kernel fields
- public CompleteIntent
- caller-selected executor
- token-as-authority semantics
- provider-controlled authority
- new schema security semantics
- unnecessary service abstraction
- UI/MCP security state
- unauthorized external side effects

The design principle remains:

    SMALL TRUSTED AUTHORITY CORE
        +
    SERVICES / ADAPTERS / EXECUTORS AROUND IT

==================================================
PART XVII — ADVERSARIAL MUTATION ANALYSIS
==================================================

For the most important tests, perform a mental/code-level mutation analysis.

Ask:

Would this test fail if we:

1. removed the final authorization check?
2. passed caller params directly to the executor?
3. restored params["tool_name"] executor selection?
4. removed exact intentID binding?
5. removed the scenario predicate?
6. removed the revocation check?
7. invoked executor before authorization?
8. allowed provider output to create authority?
9. called CompleteIntent from a public API?
10. changed CI-2 so audit failure rewrites success to false?

Record tests that would NOT catch the regression.

==================================================
FINDING CLASSIFICATION
==================================================

Classify findings:

    CRITICAL
    HIGH
    MEDIUM
    LOW
    INFO

CRITICAL/HIGH/MEDIUM findings block GO.

LOW/INFO do not automatically block GO.

For every finding provide:

- ID
- severity
- exact file
- exact function/symbol
- technical issue
- attack/failure scenario
- violated invariant
- whether existing tests catch it
- remediation direction

Do not fix anything.

==================================================
IMPORTANT NON-FINDINGS
==================================================

Do NOT automatically treat these as defects:

- Window B itself
- concurrent duplicate execution if already documented as v1
- provider acceptance not meaning completion
- CompleteIntent persistence failure after provider acceptance
- audit post-commit durability gap
- absence of idempotency
- absence of async execution
- absence of retries
- absence of RBAC/IAM
- absence of multi-tenancy
- absence of multi-provider infrastructure

They become findings only if:
- implementation contradicts the approved contract
- documentation claims stronger guarantees
- the new changes accidentally worsen the behavior
- the limitation is handled dishonestly

==================================================
FINAL VERDICT
==================================================

Return:

    VERDICT: GO

ONLY when:

    0 CRITICAL
    0 HIGH
    0 MEDIUM

and the implementation genuinely satisfies the approved Phase 4C+ contract.

Otherwise:

    VERDICT: NO-GO

==================================================
FINAL REPORT
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

3. N-1 / N-2 closure:
       CLOSED / PARTIAL / OPEN

4. Findings table

    ID | Severity | Area | Finding

5. Detailed findings

6. Security invariant matrix

    PASS / FAIL / UNPROVEN

7. Test verification

    go build ./...
    go vet ./...
    go test -count=1 ./...
    go test -race ./...

8. Test-quality assessment

   Determine whether the 55 GitHub adapter tests actually prove the claimed
   security properties.

9. N-1 audit-failure analysis

10. N-2 Window-B analysis

11. Real GitHub HTTP-provider security analysis

12. Executor trust-boundary analysis

13. CompleteIntent analysis

14. Architectural boundary analysis

15. Remaining known limitations

16. Final recommendation

The key question is:

    Has Solvent actually demonstrated that an authorized exact action can
    reach a real external side effect while untrusted agent input remains
    non-authoritative?

Do not approve because the implementation looks good.

Try to prove it wrong.
```

This is the final gate I would use. The previous review already established that the major architecture and real GitHub path are sound; the remaining question is whether the **newly repaired tests genuinely establish the two missing guarantees**, particularly N-1 and N-2, while surviving a fresh race/security review. The previous reviewer explicitly identified those two as the remaining blockers. 
