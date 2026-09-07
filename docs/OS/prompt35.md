The adversarial review is a legitimate **NO-GO**, but there is one important discrepancy that should be resolved before changing code:

The implementation report you showed says `adapter/github/executor_test.go` was created, while the independent review says that file **does not exist**. The review also says `FakeGitHubProvider` has zero test references. That means the first task should be to reconcile the actual repository state, rather than blindly implementing the review findings. 

The review otherwise gives a clean diagnosis: the kernel/service implementation is largely correct, but the executable acceptance layer is incomplete—especially adapter-level proof and production executor wiring. 

### Prompt for the coding agent

```text
PHASE 4C+ — CLOSE ADVERSARIAL REVIEW FINDINGS

You are implementing fixes for the independent Phase 4C+ adversarial review.

The review verdict was:

    NO-GO

Do NOT redesign Solvent.
Do NOT enlarge the kernel.
Do NOT add new services.
Do NOT change the approved authority model.
Do NOT introduce idempotency, async execution, retries, RBAC, multi-tenancy,
or other deferred features.

The goal is strictly to close the identified acceptance/wiring gaps while
preserving the already-approved architecture.

==================================================
FIRST: RECONCILE REPOSITORY STATE
==================================================

There is a discrepancy between the implementation report and the independent
review:

Implementation report claimed:
    adapter/github/executor_test.go exists

Independent review found:
    adapter/github/executor_test.go does not exist

Before modifying anything, inspect the actual working tree and git state.

Run:

    git status --short
    git ls-files adapter/github/executor_test.go
    ls -la adapter/github/
    grep -R "FakeGitHubProvider" -n adapter/github . --exclude-dir=.git
    grep -R "RegisterExecutor" -n cmd adapter examples --exclude-dir=.git

Determine whether:
- executor_test.go is genuinely absent
- it exists but is untracked
- it exists on disk but is not the repository version
- the reviewer inspected a different state

Do NOT fabricate a test suite that already exists.

==================================================
AUTHORITATIVE ARCHITECTURE — DO NOT CHANGE
==================================================

Preserve all of the following:

1. kernel.Authorize remains the sole authority oracle.

2. Snapshot consequence_parameters remain the sole source of provider execution
   parameters.

3. Execution parameters flow through:

       kernel.Authorize
           ↓
       AuthorizeResult.ConsequenceParameters
           ↓
       service-side JSON reconstruction
           ↓
       executor

4. Caller-supplied provider parameters are ignored.

5. Caller-supplied tool_name/executor selection is ignored.

6. Executor selection remains a fixed mapping from authorized action:

       authorized action → fixed executor

7. ExecuteAction uses the exact intentID.

8. CompleteIntent(exact intentID) remains the only new kernel lifecycle primitive.

9. CompleteIntent is called only after provider acceptance.

10. CI-2 remains authoritative:

       executor returns nil error
           => result.Success == true

    CompleteIntent persistence failure must not rewrite that result.

11. Provider rejection/failure leaves intent live.

12. Provider acceptance does not mean external workflow completion.

13. Provider acceptance followed by Solvent persistence failure remains distinct via:

       intent_completion_failed

14. Window B remains a documented v1 TOCTOU limitation.

15. No claim of atomic database authorization + external provider execution.

16. There is exactly one DB read between T2 and T3:
       exact intent-state verification

17. No GetSnapshotConsequenceParams kernel method.

18. No GitHub-specific semantics in the kernel.

19. Executor code remains in adapter/github.

20. Executor cannot manipulate Solvent authority state.

21. API/MCP must not expose CompleteIntent directly.

22. Do not create a second authorization engine.

==================================================
FINDINGS TO CLOSE
==================================================

F-1 CRITICAL
Adapter-level executor test suite missing.

F-2 CRITICAL
GitHub executor not registered in production MCP entry point.

F-3 CRITICAL
FakeGitHubProvider not used by tests.

F-4 HIGH
TestExec36 missing.

F-5 HIGH
TestExec35 missing.

F-6 HIGH
TestExec38 missing.

F-7 HIGH
TestAT01–16 missing.

F-8 MEDIUM
examples/github/executor_example.go missing.

F-9 LOW
8.6MB compiled binary committed under examples/github/.

==================================================
TASK 1 — ADAPTER TEST SUITE
==================================================

If executor_test.go is genuinely absent, create:

    adapter/github/executor_test.go

Implement the acceptance tests specified by:

    docs/OS/phase4c_plus_test_target.md

Do NOT invent new security semantics.

The adapter tests must use FakeGitHubProvider rather than a generic executor
double whenever the test is intended to prove GitHub executor behavior.

At minimum ensure the suite covers the approved:

    TestExec01–38
    TestAT01–16

Pay particular attention to:

    TestExec35
    TestExec36
    TestExec38

and the adversarial cases:

    TestAT01–16

The tests must genuinely exercise production execution code.

Do not write tests that merely duplicate implementation logic.

==================================================
TASK 2 — TEST THE APPROVED SNAPSHOT SOURCE OF TRUTH
==================================================

TestExec36 must establish that caller parameters cannot control execution.

Use at least these variants:

A. Caller supplies matching parameters plus extra fields.

Expected:
    executor receives only approved snapshot parameters.

B. Caller supplies conflicting repo/workflow/ref.

Expected:
    executor receives snapshot values.

C. Caller supplies arbitrary provider-looking extra fields.

Expected:
    those fields do not reach the executor/provider.

The source of truth must be:

    AuthorizeResult.ConsequenceParameters

Never caller params.

==================================================
TASK 3 — TEST AUTHORITY INTEGRITY
==================================================

Implement TestExec35.

Capture pre-execution state for the authority-related rows required by the
test target, including:

    target
    activation
    snapshot
    belief
    principal

Execute with provider acceptance.

Re-read the rows.

Assert byte-for-byte equality except for the permitted action-intent state
transition.

The test must fail if provider success causes creation/modification of authority
state.

==================================================
TASK 4 — TEST PROVIDER-SUCCESS / SOLVENT-ERROR
==================================================

Implement TestExec38.

Use FakeGitHubProvider's lost-response/error behavior as specified by the test
target.

Verify:

    provider called exactly once
    execution result is unsuccessful from Solvent's perspective
    intent remains live
    executor_failed is recorded appropriately
    this is distinguishable from CompleteIntent success/failure semantics

Do not reinterpret provider acceptance as workflow completion.

==================================================
TASK 5 — ADVERSARIAL TESTS
==================================================

Implement TestAT01–16 exactly according to the approved test target.

The tests should demonstrate that removing/bypassing a particular security
control would cause the corresponding security property to fail.

In particular verify:

- authorization cannot be skipped
- arbitrary executor selection cannot be injected
- caller parameters cannot override snapshot parameters
- wrong target is rejected
- wrong action is rejected
- wrong actor/principal is rejected
- revoked authority is rejected
- exact intentID binding is preserved
- CompleteIntent cannot be used as a public bypass
- provider output cannot become authority

Do not weaken tests merely to make the current implementation pass.

==================================================
TASK 6 — PRODUCTION EXECUTOR REGISTRATION
==================================================

Inspect:

    cmd/solvent-mcp/main.go

Register the GitHub executor in the production registry, using the architecture
specified by the implementation plan.

Expected conceptual wiring:

    registry := executor.NewRegistry()

    github provider
        ↓
    github.RegisterExecutor(registry, provider)

Registration must be gated appropriately on GITHUB_TOKEN as defined by the
approved Phase 4C+ design.

Do not hard-code credentials.

Do not expose credentials through params.

Do not log GITHUB_TOKEN.

If no GITHUB_TOKEN is present, preserve the intended behavior that the real
integration is optional rather than inventing fake production credentials.

Also inspect other actual production entry points that can invoke ExecuteAction.
Register the executor only where the approved architecture requires it.

Do NOT introduce a generic multi-provider registration framework.

==================================================
TASK 7 — EXAMPLE
==================================================

Create:

    examples/github/executor_example.go

Keep it small and runnable.

It should demonstrate the intended GitHub executor wiring and execution path.

It must not introduce a second authority engine.

It must not bypass service/authority.ExecuteAction.

It must not call CompleteIntent directly.

It must not treat provider output as authority.

It must not embed credentials.

==================================================
TASK 8 — REPOSITORY HYGIENE
==================================================

Remove:

    examples/github/github

if it is confirmed to be a committed/generated ELF binary.

Update .gitignore only if appropriate for the existing repository conventions.

Do not make unrelated cleanup changes.

==================================================
STRICT SCOPE
==================================================

Do NOT:

- modify authority semantics
- add kernel primitives beyond those already approved
- add schema changes
- add services
- add workflow/token machinery
- add idempotency
- add retries
- add asynchronous execution
- add provider polling
- add multi-provider abstractions
- add RBAC/IAM
- add authentication redesign
- change Window B semantics
- change CI-2
- change CompleteIntent semantics
- move GitHub semantics into the kernel
- expose a public CompleteIntent endpoint

==================================================
VERIFICATION
==================================================

After implementation run:

    go test ./...
    go test -race ./...
    go vet ./...

Also run focused tests:

    go test ./adapter/github/...
    go test ./service/authority/...
    go test ./kernel/...

Verify the exact required acceptance tests exist and execute.

Confirm:

    TestExec01–38
    TestAT01–16

are actually discovered by Go's test runner rather than merely existing as
unused helpers.

Also verify:

    grep -R "FakeGitHubProvider" -n adapter/github

shows real test usage.

Verify executor registration with source inspection.

Verify there is no:

    GetSnapshotConsequenceParams

and no execution path using:

    params["tool_name"]

for executor selection.

Verify ExecuteAction uses the exact intentID all the way to:

    CompleteIntent(intentID)

==================================================
FINAL REPORT
==================================================

Report only after the implementation and tests are complete.

Provide:

1. Reconciliation result for executor_test.go discrepancy.

2. Files created/modified.

3. Findings closed:
   F-1 through F-9.

4. Exact test counts discovered/executed.

5. Results of:
       go test ./...
       go test -race ./...
       go vet ./...

6. Confirmation that FakeGitHubProvider is actively used.

7. Confirmation that GitHub executor is registered in production.

8. Confirmation that snapshot parameters remain the sole execution source.

9. Confirmation that no new kernel/service/schema architecture was introduced.

10. Any remaining limitations.

Do not claim GO merely because tests pass.
State whether the implementation is actually ready for another independent
adversarial review.
```

The important architectural point is that this review does **not** call for changing the kernel. The reviewer explicitly found the kernel/service design structurally sound; the NO-GO is primarily because the security proof and real executor wiring were not actually connected. 

Also, I would **not** let the coding agent simply implement F-1 through F-9 without first reconciling the `executor_test.go` contradiction. That discrepancy is significant enough that it could indicate the review was run against a different checkout, branch, or working-tree state.
