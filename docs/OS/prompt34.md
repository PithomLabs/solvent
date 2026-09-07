PHASE 4C+ — ADVERSARIAL SECURITY / ARCHITECTURE CODE REVIEW

You are acting as an independent senior security architect reviewing the completed
Phase 4C+ implementation of Solvent.

This is NOT an implementation task.

Do not modify any files.
Do not refactor code.
Do not fix findings.
Do not add tests unless absolutely necessary to prove an existing claim, and do not
commit any changes.

Your job is to try to BREAK the implementation and determine whether it actually
preserves the approved Phase 4C+ security and architectural contract.

Repository:
    ~/Documents/go/solvent-main

The implementation has already been reported as complete and:
    go test ./...
passes.

Treat passing tests as weak evidence. Assume the implementation may contain subtle
security, authority, concurrency, TOCTOU, attribution, or architectural flaws that
ordinary tests do not expose.

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
    JSON unmarshal in service
        ↓
    exact intentID state verification
        ↓
    fixed authorized-action → executor mapping
        ↓
    provider/executor
        ↓
    CompleteIntent(exact intentID)

Critical rules:

1. TOKEN != AUTHORITY.

2. Caller-provided execution parameters must NOT determine the actual consequential
   provider invocation.

3. Snapshot consequence_parameters captured at approval are the sole source of
   provider execution parameters.

4. Those snapshot parameters are returned through:
       AuthorizeResult.ConsequenceParameters

5. There must NOT be a kernel method such as:
       GetSnapshotConsequenceParams

6. The caller must NOT be able to select an arbitrary consequential executor by
   supplying:
       tool_name
   or an equivalent capability selector.

7. Executor selection must be determined by a fixed mapping from the already
   authorized action to the executor.

8. ExecuteAction must operate on the exact intentID supplied by the caller/service
   path. It must NOT rediscover an arbitrary live intent with:
       SELECT ... LIMIT 1
   or equivalent.

9. CompleteIntent(exact intentID) is the only newly accepted kernel primitive in
   this phase and exists solely to perform the necessary lifecycle transition after
   successful provider acceptance.

10. CI-2 is authoritative:

       executor returns nil error
           => ExecutionResult.Success == true

    CompleteIntent persistence failure must NOT overwrite this result.

11. A provider rejection/error must leave the intent live unless the existing
    contract explicitly says otherwise.

12. Provider acceptance means the provider accepted the execution request and
    returned a reference. It does NOT mean the external workflow completed.

13. A failure after provider acceptance but during CompleteIntent must be represented
    distinctly from provider invocation failure, including:
       intent_completion_failed

14. Window A:
       revocation before final authorization check
    must deny execution.

15. Window B:
       revocation after the final authorization check but before provider invocation
    remains a known v1 TOCTOU limitation.

    This must be characterized honestly.
    Do NOT claim atomic DB authorization + external provider execution.

16. There is exactly ONE additional database read after T2:
       exact intent-state verification

    Snapshot parameters are already present in AuthorizeResult and reconstruction
    after T2 is in-memory.

17. No second snapshot database query exists.

18. GitHub-specific semantics must remain outside the kernel.

19. No GitHub-specific behavior, provider schema, workflow semantics, or external
    API details should have leaked into kernel authority semantics.

20. The kernel must remain the smallest trusted authority core.

21. No unnecessary new kernel primitive, service, schema, or authorization engine
    should have been introduced.

22. Authorize != Execute.

23. The executor must not approve, promote, revoke, or create authority.

24. The executor receives an already validated authorization decision.

25. The API/UI/MCP layer must not become a second authority engine.

==================================================
REVIEW TARGET
==================================================

Inspect the actual implementation, especially:

    adapter/github/provider.go
    adapter/github/fake_provider.go
    adapter/github/executor.go
    adapter/github/executor_test.go

    kernel/sql.go
    kernel/authority.go
    kernel/contract.go

    service/authority/authority.go
    service/authority/authority_test.go
    service/authority/authority_integration_test.go

    service/audit/audit.go

Also inspect all surrounding code necessary to determine whether the above guarantees
are actually true.

Do NOT limit the review to changed files.

==================================================
ADVERSARIAL ATTACK MODEL
==================================================

Actively attempt to defeat the implementation using:

A. Parameter injection
   - conflicting caller parameters
   - extra provider-looking parameters
   - malicious repo/workflow/ref
   - mutated params after authorization
   - caller-supplied tool_name
   - caller-supplied executor name
   - caller-supplied intent ID
   - mismatched intent/target/authority combinations

B. Authorization bypass
   - stale authority
   - revoked authority
   - wrong action
   - wrong target
   - wrong consequence parameters
   - unapproved intent
   - cancelled intent
   - already executed intent
   - fabricated IDs
   - duplicate IDs
   - cross-target substitution

C. TOCTOU / concurrency
   - revoke immediately after T2
   - change intent state between checks
   - concurrent ExecuteAction calls
   - duplicate external execution
   - concurrent CompleteIntent
   - concurrent revoke/execute
   - retry after provider acceptance
   - provider acceptance followed by DB failure
   - response loss / timeout ambiguity

D. Executor isolation
   - attempt to make the service invoke an unintended executor
   - attempt to bypass fixed action→executor mapping
   - attempt to cause arbitrary provider selection
   - attempt to inject credentials through params
   - attempt to cause executor to mutate authority state

E. Kernel boundary violations
   - provider semantics leaking into kernel
   - GitHub-specific fields becoming authority semantics
   - unnecessary kernel state
   - service semantics that should belong in kernel
   - kernel primitives added without atomic/security necessity
   - duplicate authority logic outside kernel

F. Audit integrity
   - incorrect result attribution
   - provider failure reported as success
   - CompleteIntent failure reported as provider failure
   - missing event on denied execution
   - inconsistent intent/result/audit state
   - audit failure changing the meaning of an already successful authority/execution
     operation
   - ambiguity between "invocation", "accepted", and "completed"

G. Trust boundary failures
   - caller-controlled actor
   - caller-controlled identity
   - caller-controlled target
   - caller-controlled action
   - caller-controlled provider parameters
   - authentication assumptions crossing service boundaries

H. API/MCP abuse
   - execution routed through an unintended public path
   - CompleteIntent exposed directly to API/MCP
   - arbitrary execution capability exposed to clients
   - REST/MCP semantic divergence

==================================================
MANDATORY QUESTIONS
==================================================

Answer each of these explicitly:

1. Is the snapshot consequence_parameters truly the sole source of provider
   execution parameters?

2. Can any caller-controlled field influence which GitHub repository, workflow, or
   ref is actually invoked?

3. Can any caller-controlled field select an arbitrary executor?

4. Is exact intentID preserved all the way through CompleteIntent?

5. Can an intent other than the requested intent accidentally be executed or marked
   executed?

6. Does revocation before final authorization reliably deny execution?

7. Is the Window B race correctly characterized, with no false atomicity claim?

8. Are there any additional database reads between T2 and T3 beyond the one
   intent-state verification read?

9. Can concurrent ExecuteAction calls cause duplicate external side effects?

10. What happens if the provider accepts execution but CompleteIntent fails?

11. What happens if the provider definitely fails but the service receives an error?

12. What happens if the provider response is ambiguous/lost?

13. Can executor code itself alter Solvent authority state?

14. Has any GitHub-specific semantics leaked into kernel authority semantics?

15. Has any new code accidentally created a second authorization engine?

16. Can MCP/REST/API callers bypass the intended execution path?

17. Can caller-controlled actor identity or actor type be used as authority proof?

18. Are audit events semantically correct for:
       denied
       executor failure
       provider acceptance
       CompleteIntent failure

19. Does the implementation preserve:
       Authorize != Execute

20. Does the implementation preserve:
       TOKEN != AUTHORITY

==================================================
REQUIRED TEST / VERIFICATION WORK
==================================================

Do not merely read tests.

Inspect whether the tests would actually detect the important failures.

At minimum inspect:

    TestExec10
    TestExec11
    TestExec12
    TestExec13
    TestExec15A
    TestExec15B
    TestExec27
    TestExec28
    TestExec29
    TestExec30
    TestExec35
    TestExec36
    TestExec38
    TestAT01..TestAT16

Check for false-positive tests such as:

- mocks proving behavior that production code does not use
- tests that never exercise the real execution path
- assertions that are weaker than the architectural claim
- tests that accidentally permit caller parameters
- tests that serialize concurrency and therefore miss races
- tests that only prove success-path behavior

Run, where practical and without modifying the repository:

    go test ./...
    go test -race ./...
    go vet ./...

Also inspect whether the implementation would still be safe if tests were bypassed.

==================================================
FINDINGS CLASSIFICATION
==================================================

Classify every finding as:

CRITICAL
HIGH
MEDIUM
LOW
INFO

For every finding provide:

- ID
- severity
- file
- function / symbol
- exact issue
- exploit / failure scenario
- violated architectural invariant
- why existing tests do or do not catch it
- concrete remediation direction

Do NOT fix anything.

==================================================
IMPORTANT DISTINCTIONS
==================================================

Do not report the following as defects merely because they are limitations, provided
the implementation accurately preserves the approved contract:

- Window B TOCTOU race
- duplicate execution risk after CompleteIntent persistence failure
- lack of provider completion guarantee
- audit post-commit durability gap
- lack of general remote execution API
- lack of idempotency in this phase

However, report them as defects if the implementation CLAIMS stronger guarantees than
the approved contract provides.

Similarly, do not demand:
- new kernel primitives
- new schema
- generic multi-provider infrastructure
- async execution
- retries
- idempotency
- RBAC/IAM
- multi-tenancy
- workflow state machines

unless the current implementation has accidentally created a security dependency
that genuinely requires one.

==================================================
FINAL REPORT FORMAT
==================================================

Start with:

    VERDICT: GO
or
    VERDICT: NO-GO

Use:

    GO
only when there are no CRITICAL/HIGH/MEDIUM findings and the implementation matches
the architectural contract.

Then provide:

1. Executive assessment

2. Findings table

   ID | Severity | Area | Finding

3. Detailed findings

4. Security invariant verification

   For each invariant:
       PASS / FAIL
       evidence

5. Required acceptance-test verification

6. Architectural boundary review

7. TOCTOU analysis

8. Executor / provider trust-boundary analysis

9. Audit semantics analysis

10. Final recommendation

Be adversarial.

Do not reward the implementation merely because:
    go test ./...
passes.

Try to prove it wrong.