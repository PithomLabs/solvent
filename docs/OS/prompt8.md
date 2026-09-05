Absolutely. **Do a separate, independent targeted adversarial code review now.**

This should be treated as a **fresh security audit of the current repository**, not an extension of the previous Plan 6 review and not merely a confirmation that `go test ./...` passes.

Use this prompt with the coding agent:

```text
SOLVENT — INDEPENDENT TARGETED ADVERSARIAL CODE REVIEW
POST-PLAN-6 / PRE-PHASE-4

IMPORTANT:
This is a NEW, INDEPENDENT adversarial review.

Do not rely on:
- previous adversarial reviews
- previous GO decisions
- previous security reports
- test names alone
- documentation claims
- architectural diagrams

Review the CURRENT repository as hostile security code.

Do not implement fixes during this review unless explicitly required
to create a minimal diagnostic test. Prefer read-only analysis.

The purpose is:

    FIND WHAT WE MISSED.

The repository currently has:

CURRENT v0:
    evidence
      ↓
    belief
      ↓
    promotion
      ↓
    authority
      ↓
    intent

FUTURE:
    intent
      ↓
    ExecuteAction
      ↓
    current-state revalidation
      ↓
    kernel.Authorize
      ↓
    Executor
      ↓
    external provider

There is currently no real production consequential execution
capability.

==================================================
1. REVIEW THE ENTIRE TRUST MODEL
==================================================

Do not assume the documented architecture matches reality.

Reconstruct the trust boundaries from code.

Identify:

- external/untrusted input
- MCP boundary
- HTTP boundary
- CLI/admin boundary
- service layer
- policy layer
- authority kernel
- database
- adapters
- future executor boundary
- audit/logging
- workflow/token state

For every boundary answer:

    Who controls this input?
    Who authenticates it?
    Who validates it?
    Who can mutate authoritative state?
    Who can trigger consequential behavior?

Flag any place where an untrusted component crosses a boundary
without an explicit validation step.

==================================================
2. SEARCH FOR HIDDEN CONSEQUENTIAL SIDE EFFECTS
==================================================

Search the ENTIRE repository for every operation that could cause
a real-world side effect.

Search for:

    exec.Command
    os/exec
    client.Do
    http.Do
    grpc calls
    SDK client methods
    POST
    PUT
    PATCH
    DELETE
    deployment
    deploy
    send
    publish
    create
    update
    delete
    mutate
    write
    shell
    subprocess
    webhook
    callback
    queue publish
    cloud SDK operations
    GitHub mutations
    filesystem mutations that have security significance

Also inspect:

    initialization hooks
    background goroutines
    callbacks
    deferred functions
    event handlers
    cron/schedulers
    package init()
    test fixtures that may accidentally be production code

Classify every side effect:

    READ-ONLY
    INTERNAL STATE MUTATION
    AUTHORITY CREATION
    CONSEQUENTIAL EXTERNAL SIDE EFFECT
    TEST-ONLY
    CLI-ONLY
    FUTURE
    DEAD
    BYPASS

Rule:

    intent creation
    belief promotion
    evidence ingestion
    authority creation

MUST NOT directly invoke consequential external side effects.

Any existing production consequential side effect outside the
canonical execution boundary is HIGH/CRITICAL.

==================================================
3. SEARCH FOR ALL AUTHORITY CREATION
==================================================

Find every place that can create or establish authority.

Search for:

    Approve
    target_activation
    target_snapshot
    authority_target
    authority writes
    inserts into authority tables
    direct SQL against authority tables
    authority constructors
    approval handlers
    admin commands

Produce:

    authority creation site
    caller
    actor
    validation
    kernel/service boundary

Expected:

    kernel.Approve

is the sole authority-creating semantic operation.

If another path can create equivalent authority:

    CRITICAL

==================================================
4. SEARCH FOR ALL AUTHORITY CHECKS
==================================================

Find every location that decides:

    allowed
    authorized
    approved
    permitted
    active authority
    valid authority

Search for:

    Authorize
    authorization
    allowed
    permitted
    approved
    revoked
    target activation
    target snapshot
    principal match

Classify each:

    kernel authority oracle
    policy constraint
    workflow condition
    UI condition
    duplicated authority logic
    informational/read-only

The ONLY final authority oracle must remain:

    kernel.Authorize

Service code may prepare context.

Service code must NOT independently decide authority validity.

==================================================
5. ATTACK THE SERVICE LAYER
==================================================

Inspect:

    service/authority
    service/policy
    service/audit
    service/executor
    all remaining services

Ask:

    Can service code create authority?
    Can service code bypass kernel.Authorize?
    Can service code convert policy into authority?
    Can service code convert evidence into authority?
    Can service code convert workflow state into authority?
    Can service code execute an action directly?

Trace both exported and unexported functions.

Look for:

    helper functions
    convenience methods
    package-level functions
    callbacks
    function values
    interface implementations
    test-only hooks accidentally available in production

==================================================
6. ATTACK THE KERNEL API SURFACE
==================================================

The kernel is considered healthy.

Do not change it.

Instead attack its CALLERS.

Search for ways callers can:

    bypass Authorize
    call lower-level DB operations
    directly construct kernel state
    invoke Store in ways that create consequential state
    bypass tuple construction
    bypass revocation
    bypass exact target/action binding

Determine whether the public kernel API exposes any lower-level
operation that makes the service boundary optional.

If so, document the risk.

Do not automatically fix it by expanding the kernel.

==================================================
7. ATTACK ACTION_INTENT
==================================================

This deserves special attention.

Trace:

    creation
    storage
    retrieval
    status changes
    execution
    cancellation
    expiration
    replay
    re-submission

Determine exactly what makes an intent:

    proposed
    live
    executable
    completed

Ask:

    Can an intent become executable without current authority?
    Can an old intent bypass a new authorization decision?
    Can an intent be modified after authority is established?
    Can actor/action/target be changed after authorization?
    Can an intent be replayed?
    Can a caller fabricate an intent directly?
    Can an intent reference authority that no longer exists?

Intent must never substitute for authority.

==================================================
8. ATTACK TARGET / SNAPSHOT IMMUTABILITY
==================================================

Inspect:

    authority_target
    target_snapshot
    target_activation
    target_revocation

Verify:

    approved snapshot is immutable
    exact tuple is preserved
    wrong target fails
    wrong action fails
    consequence parameters fail closed
    revocation wins
    current state wins

Look specifically for:

    mutable target rows
    mutable snapshot rows
    indirect target aliases
    string normalization
    case normalization
    default parameters
    missing parameter handling
    nil/empty-value equivalence
    wildcard target matching
    partial tuple matching

Any way to turn:

    target A

into:

    target B

after authorization is CRITICAL.

==================================================
9. ATTACK CONSEQUENCE PARAMETERS
==================================================

Audit every field introduced for:

    consequenceType
    consequenceParameters

Verify these fields are:

    constructed consistently
    included in AuthorityTuple
    checked by kernel.Authorize
    preserved through PrepareForAction
    preserved through ExecuteAction

Look for a subtle bug where:

    authority approves P1

but execution actually uses:

    P2

Test:

    identical target/action
    different consequence parameters
    → DENIED

Missing consequence parameters must not silently become wildcard
authorization.

==================================================
10. ATTACK ACTOR IDENTITY
==================================================

Trace actor from:

    external input
      ↓
    authentication/trust boundary
      ↓
    actor classification
      ↓
    AuthorityTuple
      ↓
    kernel.Authorize

Attempt:

    actor substitution
    actor spoofing
    actor type substitution
    principal ID substitution
    HUMAN assertion
    AGENT assertion
    SYSTEM assertion

Never trust:

    request.body.actor
    request.body.actor_type
    action_source
    user_typed

as authentication.

Verify trusted administrative tooling is explicitly separated from
untrusted runtime callers.

==================================================
11. ATTACK POLICY
==================================================

Audit PolicyConstraints end-to-end.

Determine whether any policy result can:

    create authority
    imply authority
    bypass authority
    skip kernel.Authorize
    invoke an executor

Test conceptually:

    policy ALLOW
    authority ABSENT
    → DENIED

    policy DENY
    authority VALID
    → DENIED

    policy ALLOW
    authority VALID
    wrong target
    → DENIED

Look for semantic confusion such as:

    Allowed
    Authorized
    Permitted
    Approved

being treated as interchangeable.

==================================================
12. ATTACK WORKFLOW/TOKEN
==================================================

Current architecture:

    workflow token = continuity only

Verify this remains true after Plan 6.

The workflow package was removed.

Now search the ENTIRE repository for:

    workflow token references
    token tables
    token IDs
    workflow state
    token payload
    old workflow imports
    compatibility code
    stale migrations
    unused token fields

Verify there is no residual code that treats workflow state as
authority.

Any stale/dead token implementation that can affect security is a
finding.

==================================================
13. ATTACK MCP
==================================================

Review EVERY MCP tool.

For each tool classify:

    READ_ONLY
    BELIEF/EVIDENCE MUTATION
    AUTHORITY CREATION
    INTENT CREATION
    CONSEQUENTIAL EXECUTION
    ADMINISTRATIVE

For every mutation tool ask:

    Does it authenticate the caller?
    Does it trust client actor IDs?
    Does it trust client action source?
    Can it bypass policy?
    Can it bypass kernel authority?
    Can it manufacture approval?
    Can it cause a side effect?

Verify missing authorization context fails closed.

Verify no tool creates a hidden execution capability.

==================================================
14. ATTACK WIZARD / HTTP
==================================================

Review every route and handler.

For each:

    input
    validation
    actor
    service call
    kernel call
    DB write
    external side effect

Look specifically for:

    browser state
    hidden form fields
    client-controlled authority references
    direct kernel calls
    direct DB mutations
    direct provider calls

Verify the browser cannot manufacture:

    approval
    authority
    execution permission

==================================================
15. ATTACK ADMIN / CLI TOOLS
==================================================

Review:

    cmd/operator-review
    corpus tooling
    all commands under cmd/

Classify each as:

    production service
    trusted administrative tooling
    development tooling
    ingestion tooling
    test tooling

A trusted admin CLI may have elevated authority, but this must be
explicitly documented.

However:

    trusted CLI != excuse for accidental external execution

Verify no CLI unexpectedly performs consequential operations as part
of inspection, review, import, or reporting.

==================================================
16. ATTACK ADAPTERS
==================================================

Review all adapters, especially GitHub.

Verify:

    external input
      →
    normalized generic evidence

and NOT:

    external input
      →
    authority

Search for provider-specific semantics leaking inward.

Look for:

    provider says approved
    provider says authorized
    provider says trusted
    provider says success

being translated into authority or execution permission.

Providers cannot create authority.

==================================================
17. ATTACK AUDIT
==================================================

Audit is evidence.

Verify that audit data cannot become authority.

Verify audit events accurately distinguish:

    evidence
    belief
    authority
    authorization
    intent
    execution

Do not allow an audit record such as:

    authorization_granted

to itself become a cached authorization source.

Search for consumers of audit records.

==================================================
18. ATTACK DATABASE BOUNDARIES
==================================================

Search for all direct SQL writes to:

    authority_target
    target_snapshot
    target_activation
    target_revocation
    principal
    justification
    debt_discharge
    belief
    action_intent

Classify every writer:

    kernel-approved
    legitimate product mutation
    migration
    test
    dangerous bypass

Verify product services cannot accidentally write authority-core
state directly.

Verify DB invariants remain intact.

Verify no new authority-core migration appeared unexpectedly.

==================================================
19. ATTACK TRANSACTION BOUNDARIES
==================================================

Inspect transactions around:

    Promote
    Approve
    Authorize
    RevokeTarget
    IntentOnPromoted
    PrepareForAction
    ExecuteAction

Verify that code does not:

    read authority on connection A
    release transaction
    perform execution based on stale result

where the intended security claim requires current state.

Look for:

    cached SQL results
    stale structs
    transaction snapshots
    read-before-write races
    authorization before mutation
    execution after transaction ends

Remember:

    Solvent cannot atomically transact with an external provider.

Do not demand impossible distributed atomicity.

==================================================
20. ATTACK REPLAY
==================================================

Even though global replay prevention is deferred, inspect current code
for obvious accidental replay vulnerabilities.

Ask:

    Can the same approved intent be executed repeatedly?
    Can the same authority be reused for multiple unintended actions?
    Can the same request be replayed against another target?
    Can stale request data become current execution input?

Do not introduce global replay-prevention machinery.

Report only concrete current risks.

==================================================
21. ATTACK ERROR HANDLING
==================================================

Inspect every authority/execution boundary for:

    ignored errors
    default-allow behavior
    nil handling
    empty strings
    zero UUIDs
    malformed parameters
    missing fields
    fallback values
    panic recovery
    partial failures

Security failures MUST fail closed.

Search for patterns such as:

    if err != nil { ... continue ... }
    default true
    missing → allow
    empty → wildcard
    fallback authority
    best effort authorization

Any default-allow security behavior is HIGH/CRITICAL.

==================================================
22. ATTACK TEST HARNESS
==================================================

Inspect tests for false confidence.

Ask:

    Could the test pass if the authority check were removed?

    Could the test pass if executor were never reached?

    Is the recording executor actually asserting invocation?

    Are mutations occurring at the correct point?

    Are tests operating against real CockroachDB?

    Are fixtures accidentally encoding the answer?

Inspect the three known critical tests:

    revoke after prepare
    target mutation
    action mutation

Verify they genuinely test execution-time revalidation.

==================================================
23. ATTACK COMPILATION / BUILD SURFACE
==================================================

Run:

    go test -count=1 -p 1 ./...
    go build ./...
    go vet ./...

Search for:

    build tags
    alternate binaries
    hidden packages
    generated code
    platform-specific files

Ensure a security boundary is not bypassed under another build target.

==================================================
24. ARCHITECTURE DRIFT REVIEW
==================================================

Compare CURRENT CODE against the locked principles:

    small trusted kernel
    current authority wins
    authorize != execute
    actor != identity
    evidence != authority
    policy != kernel
    workflow != authority
    adapters know external systems
    one source of authority truth
    complexity must be earned

Identify any code that violates these principles even if it is not
currently exploitable.

Classify:

    SECURITY
    ARCHITECTURE
    MAINTAINABILITY
    DOCUMENTATION

Do not turn style disagreements into security findings.

==================================================
25. REQUIRED FINDINGS FORMAT
==================================================

For every finding provide:

    ID
    SEVERITY
    FILE
    FUNCTION/SYMBOL
    ATTACK
    ACTUAL BEHAVIOR
    EXPECTED BEHAVIOR
    EXPLOITABILITY
    SECURITY IMPACT
    RECOMMENDED FIX
    CODE CHANGE REQUIRED? YES/NO

Severity:

CRITICAL
    Existing exploitable authority or consequential-execution bypass.

HIGH
    Serious security boundary failure likely to produce bypass.

MEDIUM
    Meaningful weakness or architectural flaw without demonstrated
    immediate bypass.

LOW
    Documentation/maintainability/minor issue.

Do not manufacture findings.

Do not downgrade a concrete bypass merely because current tests pass.

==================================================
26. REQUIRED SUMMARY TABLES
==================================================

Provide these tables:

TABLE A — CONSEQUENTIAL SIDE EFFECTS

    site | operation | classification | authority path

TABLE B — AUTHORITY CREATION

    site | mechanism | kernel? | validation

TABLE C — AUTHORITY CHECKS

    site | type | final oracle? | notes

TABLE D — EXECUTION ENTRY POINTS

    entry point | execution capability | path | status

TABLE E — DIRECT PROVIDER CALLS

    site | provider | side effect? | classification

TABLE F — DIRECT KERNEL CALLS

    site | kernel method | purpose | risk

TABLE G — SERVICE BOUNDARIES

    service | production caller | security role | status

==================================================
27. REQUIRED SECURITY QUESTIONS
==================================================

Answer explicitly:

1. Can any existing production operation cause a consequential
   external side effect?

2. If yes, does it use the canonical authority boundary?

3. Can anything create authority without kernel.Approve?

4. Can anything decide authority without kernel.Authorize?

5. Can policy manufacture authority?

6. Can workflow/token state manufacture authority?

7. Can intent state manufacture authority?

8. Can evidence manufacture authority?

9. Can provider output manufacture authority?

10. Can actor identity be spoofed?

11. Can target/action/consequence parameters diverge after authorization?

12. Can revocation be bypassed?

13. Can stale state reach execution?

14. Can any denied path reach an executor?

15. Can the browser/client manufacture authority?

16. Can a CLI or admin tool accidentally perform consequential
    operations?

17. Are there hidden external side effects?

18. Is any security boundary dead code?

19. Is any security-sensitive default allow?

20. Is there any unexplained kernel or authority-schema growth?

==================================================
28. REQUIRED FINAL VERDICT
==================================================

End with exactly one:

GO

only if:

- no CRITICAL findings
- no HIGH findings
- no existing consequential production bypass
- authority creation remains controlled
- kernel.Authorize remains the final authority oracle
- exact actor/action/target/consequence tuple is preserved
- revocation remains effective
- policy cannot manufacture authority
- workflow cannot manufacture authority
- intent cannot manufacture authority
- evidence cannot manufacture authority
- providers cannot manufacture authority
- browser/client cannot manufacture authority
- no hidden external side effects exist
- no direct executor/provider bypass exists
- denied paths cannot reach an executor
- tests provide real evidence rather than false confidence
- architecture matches the documented current/future state

OR:

HOLD

if ANY CRITICAL or HIGH finding remains.

==================================================
29. IMPORTANT SCOPE RULE
==================================================

Do NOT create production execution capability merely to satisfy this
review.

The absence of a production consequential executor is an explicitly
documented v0 product-scope fact.

The correct question is:

    "Does anything that CAN execute today bypass authority?"

not:

    "Why can't we find an executor?"

Future execution MUST use:

    ExecuteAction
      →
    current-state revalidation
      →
    kernel.Authorize
      →
    Executor
      →
    external provider

==================================================
30. FINAL OUTPUT
==================================================

Produce a self-contained adversarial report.

Do not merely say "all tests pass."

The report must explain:

    what was searched
    what was traced
    what was attacked
    what was proven
    what remains unproven
    what risks remain
    whether the current repository is safe to advance to Phase 4

This is the FINAL PRE-PHASE-4 SECURITY REVIEW.

Be skeptical.

Assume something important was missed.

Try to find it.
```

This is the review I would run **now**, independently of the earlier "GO" reports.

One important improvement over the previous review is that it explicitly attacks **`action_intent`**, consequence parameters, direct kernel API exposure, hidden side effects, error/default-allow behavior, replay, build surfaces, and whether the tests would fail if the security control were removed. Those are exactly the areas where a superficially correct authority architecture can still leak.

And the final standard should remain:

```text
0 Critical
0 High
→ GO
```

Anything else stays **HOLD** before Phase 4.
