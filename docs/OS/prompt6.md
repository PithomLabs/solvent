## background

You're right. **Yes, we should perform one targeted adversarial code review before allowing Phase 4.**

The live tests prove the specific authority boundary works under the tested cases. They do **not** prove that the broader repository has no bypasses. The report's 12 verified claims are strong evidence, but the final architectural gate should still be independently attacked once against the exact post-fix code state.

The correct sequence is:

```text
Live DB tests
    ↓
GO on tested remediation
    ↓
TARGETED ADVERSARIAL CODE REVIEW
    ↓
GO / HOLD
    ↓
Phase 4
```

### What this review should target

Not another giant review of everything. Focus specifically on the changes made during the remediation and their surrounding call graph:

```text
service/authority/authority.go
service/executor/
service/policy/
service/audit/
cmd/solvent-mcp/tools.go
internal/wizard/refusal.go
internal/wizard/wizard.go
action_intent execution paths
all executor invocation sites
all external side-effect/provider calls
```

The reviewer should attack these invariants:

```text
1. Every consequential path reaches ExecuteAction.

2. No executor invocation is possible without:
   PrepareForAction
   → current kernel.Authorize
   → exact actor/action/target match.

3. There is no hidden direct executor/provider call.

4. Policy cannot manufacture authority.

5. Workflow cannot manufacture authority.

6. Intent creation cannot execute.

7. A previous authorization result cannot substitute for
   execution-time authorization.

8. Target/action mutation cannot reach the executor.

9. Revoked authority cannot reach the executor.

10. Client-controlled actor/authentication fields cannot
    manufacture human authority.

11. Denied paths provably do not invoke the executor.

12. The new service boundary did not accidentally create
    a second authority checker.
```

### Most important difference from the previous review

The reviewer should inspect **the actual call graph**, not just test behavior.

Search the repository for things like:

```text
ExecuteAction(
Executor.Execute(
executor(
http.Do(
client.Do(
deployment
deploy
mutation
write
Store(
IntentOnPromoted(
```

Then classify every consequential call:

```text
AUTHORIZED PATH
READ ONLY
TEST ONLY
DEAD CODE
BYPASS
```

Any real **BYPASS** is an automatic `HOLD`, regardless of the passing tests.

### Also attack the new TOCTOU implementation

The report says target/action mutation is denied via re-read in the transaction. That's excellent. But the review should verify the actual ordering:

```text
current state read
    ↓
kernel.Authorize
    ↓
no mutation-sensitive gap
    ↓
executor invocation
```

and verify that the values passed to the executor are the **same exact action/target tuple that was authorized**, rather than re-reading one thing and executing another.

### Final verdict

So I would correct my previous recommendation:

> **Do not begin Phase 4 yet.**

Run **one targeted post-remediation adversarial code review now**.

Then:

```text
TARGETED REVIEW
    ↓
GO
    ↓
Phase 4 — Web UI
```

And after Phase 4, perform a separate **UI/service-boundary adversarial review** before Phase 6.

That gives us the stronger sequence we originally intended:

```text
Kernel/security remediation
        ↓
Live integration tests
        ↓
Post-remediation adversarial review
        ↓
GO
        ↓
Phase 4 Web UI
        ↓
UI/service adversarial review
        ↓
Phase 6 demos
```

**That is the right gate.**



## prompt

Use this as the **targeted post-remediation adversarial code-review prompt**. It is intentionally narrower than the previous full review: attack the **actual code that changed**, trace its callers and callees, and look specifically for ways the newly established security boundary can still be bypassed. The review must happen against the **current repository state after the successful live CockroachDB test run**.

```text
SOLVENT — TARGETED POST-REMEDIATION ADVERSARIAL CODE REVIEW

STATUS:
Security-hardening integration tests are green and the repository-wide
verification suite passes.

DO NOT begin Phase 4.
DO NOT modify product/UI code.
DO NOT add features.

Perform a targeted adversarial code review of the CURRENT repository
state after the latest security remediation.

This is a CODE-LEVEL adversarial review, not a repetition of the test
report.

The purpose is to determine whether the remediation actually established
a robust security boundary or merely made the known tests pass.

==================================================
1. REVIEW OBJECTIVE
==================================================

Attempt to break these invariants:

    Every consequential production execution path
        →
    ExecuteAction
        →
    PrepareForAction
        →
    current policy constraints
        →
    kernel.Authorize
        →
    Executor

And:

    No successful CURRENT kernel.Authorize
    for the EXACT actor/action/target tuple
        →
    no executor invocation.

Assume the developer who implemented the remediation may have made
subtle mistakes that the current tests do not cover.

Look for bypasses, alternate paths, stale state, duplicated authority
logic, parameter substitution, confused-deputy behavior, and trust-boundary
errors.

==================================================
2. REVIEW SCOPE — START HERE
==================================================

Primary changed components:

    service/authority/authority.go
    service/policy/
    service/executor/
    service/audit/
    cmd/solvent-mcp/tools.go
    internal/wizard/refusal.go
    service/authority/authority_integration_test.go

Then inspect every caller/callee reachable from these paths.

Also inspect:

    action_intent lifecycle
    intent execution paths
    kernel.Authorize
    kernel.IntentOnPromoted
    authority target creation
    authority snapshot handling
    authority activation
    authority revocation
    direct kernel.Store calls
    external provider calls
    GitHub adapter
    MCP entry points
    wizard entry points
    pipeline entry points

Do not limit the review to the files listed above if the call graph
reveals additional execution paths.

==================================================
3. BUILD THE ACTUAL CALL GRAPH
==================================================

Before giving conclusions, produce:

    entry point
      →
    service
      →
    preparation
      →
    policy
      →
    kernel authorization
      →
    executor
      →
    provider

Do this for EVERY consequential execution-capable path.

Search the repository for all occurrences of:

    ExecuteAction
    PrepareForAction
    kernel.Authorize
    IntentOnPromoted
    Executor
    Execute(
    executor calls
    provider client calls
    client.Do(
    http.Do(
    deployment
    deploy
    mutate
    send
    write
    Store(
    external side effects

Classify every relevant site:

    AUTHORITY-GATED
    READ-ONLY
    TEST-ONLY
    DEAD
    BYPASS

Any production consequential BYPASS is a finding.

==================================================
4. ATTACK #1 — EXECUTION BYPASS
==================================================

Try to find any production path that can invoke an executor without:

    ExecuteAction
    →
    PrepareForAction
    →
    kernel.Authorize

Do not assume ExecuteAction is the only path because the code says so.

Prove it from the call graph.

Specifically inspect:

    MCP
    wizard
    pipeline
    action_intent execution
    demo/cloud/web
    adapters
    helper functions
    package-level callbacks
    function variables
    interfaces
    dependency injection

If an executor can be reached through another path:

    HOLD / CRITICAL

==================================================
5. ATTACK #2 — KERNEL.AUTHORIZE BYPASS
==================================================

Search for every place where authority could be inferred.

Look for service code that independently checks:

    target_activation
    target_revocation
    target_snapshot
    target ID
    action
    principal
    approval
    justification

Determine whether any service code computes something equivalent to:

    authority = valid
    authority = active
    authority = approved
    authorized = true

outside kernel.Authorize.

The service may gather data needed to construct AuthorityTuple.

The service MUST NOT become the authority oracle.

Required:

    service context gathering
        →
    AuthorityTuple
        →
    kernel.Authorize

Forbidden:

    service SQL
        →
    service independently decides authority
        →
    executor

Any duplicated authority decision logic is HIGH/CRITICAL depending on
whether it can affect execution.

==================================================
6. ATTACK #3 — EXACT TUPLE BINDING
==================================================

Verify that the tuple actually authorized is exactly the tuple executed.

Trace:

    actor
    action / consequence type
    target
    consequence parameters
    snapshot
    principal

Look for:

    authorize target A
    execute target B

or:

    authorize action X
    execute action Y

or:

    authorize actor A
    execute actor B

or:

    authorize params P1
    execute params P2

Pay particular attention to:

    consequenceType
    consequenceParameters

Verify that the exact values used by kernel.Authorize are the same
values passed onward to the executor.

If any execution parameter can change after authorization without
causing a fresh denial:

    CRITICAL

==================================================
7. ATTACK #4 — EXECUTION-TIME REVALIDATION
==================================================

Inspect PrepareForAction and ExecuteAction ordering.

Prove that execution-time authorization uses CURRENT state.

Required current-state checks include:

    belief
    evidence
    debt
    contradictions
    authority
    target snapshot
    activation
    revocation
    policy
    actor
    action
    target

Determine exactly which reads occur and when.

Look for:

    cached decisions
    stale structs
    stale workflow objects
    values passed from an earlier step
    copied authorization results
    token fields treated as truth
    prepared state reused as authority

The critical property is:

    authorization at request time
        ≠
    authorization at execution time

Any execution path that skips the second check is a finding.

==================================================
8. ATTACK #5 — REVOCATION RACE / STALE AUTHORITY
==================================================

Inspect the actual transaction/order semantics.

Verify:

    revoked authority
        →
    kernel.Authorize
        →
    DENIED
        →
    executor NOT called

Also inspect whether:

    preparation
        →
    revocation
        →
    execution

can cause stale authorization to reach the executor.

The review MUST NOT claim atomic coordination with an external provider.

The MVP guarantee is only:

    successful CURRENT kernel.Authorize
    for EXACT tuple
    immediately precedes executor invocation

Verify that the implementation actually provides that guarantee.

==================================================
9. ATTACK #6 — TARGET/ACTION MUTATION
==================================================

Try to identify whether any code can mutate:

    target
    action
    consequence parameters
    actor

after preparation but before execution.

Then determine whether ExecuteAction revalidates the resulting tuple.

Required behavior:

    target changes
        →
    DENIED
        →
    executor NOT called

    action changes
        →
    DENIED
        →
    executor NOT called

Do not rely only on the existing test names.
Inspect the implementation that makes those tests pass.

==================================================
10. ATTACK #7 — POLICY AS SECOND AUTHORITY ENGINE
==================================================

Inspect PolicyConstraints and every use of them.

Prove that policy can:

    constrain

but cannot:

    create
    imply
    substitute for
    or bypass

authority.

Test the code paths conceptually:

    policy ALLOW
    authority ABSENT
    → DENIED

    policy ALLOW
    authority REVOKED
    → DENIED

    policy DENY
    authority VALID
    → DENIED

Look for any code path where:

    constraints.ActorPermitted
    constraints.ToolPermitted
    constraints.StagePermitted
    RequiresAuthority

is interpreted as final authority.

If policy alone can reach an executor:

    CRITICAL

==================================================
11. ATTACK #8 — WORKFLOW TOKEN
==================================================

The locked rule is:

    workflow token = continuity only
    workflow token != authority

Verify:

    token fields cannot create authority
    token fields cannot bypass kernel.Authorize
    token state cannot substitute for authority
    token cannot change target/action authority
    token cannot turn DENIED into ALLOWED

Also verify that the execution path does not secretly begin depending
on token contents even though the architecture says tokens are
continuity-only.

Do not demand signing/HMAC/JWT merely because a token exists.

The current token is an opaque DB-backed continuity object.

==================================================
12. ATTACK #9 — ACTOR / AUTHENTICATION SPOOFING
==================================================

Trace actor identity from the external boundary into ExecuteAction.

Reject any design where:

    request.body.actor
    request.body.actor_type
    request.body.action_source
    request.body.user_typed
    HUMAN

is treated as authentication.

Verify:

    trusted authenticated principal
        →
    actor classification
        →
    service

If the current MCP surface is intentionally trusted as an administrative
boundary, verify that this is explicitly documented as a deployment
assumption rather than represented as cryptographic user authentication.

Look for privilege escalation by changing actor IDs or actor type.

==================================================
13. ATTACK #10 — EXECUTOR SUBVERSION
==================================================

Verify that ExecuteAction resolves the executor internally.

Forbidden:

    caller-supplied executorFn
    callback supplied by untrusted caller
    arbitrary function pointer
    arbitrary provider selection
    arbitrary execution implementation

The caller may provide action parameters.

The caller must NOT provide the execution mechanism.

Then inspect every Executor implementation.

Verify executors cannot:

    approve
    promote
    revoke
    create authority
    mutate authority
    manufacture authority-bearing evidence

Executor is downstream from authority.

==================================================
14. ATTACK #11 — DIRECT PROVIDER BYPASS
==================================================

Search every external provider call.

For each:

    provider API
    HTTP client
    GitHub client
    deployment client
    side-effect function

determine whether it can produce a consequential side effect.

Every production consequential provider call must originate from the
authorized executor path.

Provider output must never create authority.

If a hidden direct provider call exists:

    HIGH/CRITICAL

==================================================
15. ATTACK #12 — INTENT CREATION VS EXECUTION
==================================================

Verify:

    intent creation != execution

and:

    authorization performed while creating intent
        !=
    execution-time authorization

Trace action_intent creation and execution separately.

An old successful authorization must not be reused to execute after:

    revocation
    target change
    action change
    policy change
    actor change

==================================================
16. ATTACK #13 — AUDIT SEMANTICS
==================================================

Inspect audit events and call sites.

Verify distinct facts exist for:

    authorization_checked
    authorization_granted
    authorization_denied
    adapter_invoked
    provider_responded
    executor_completed
    executor_failed

Verify that audit records cannot falsely imply:

    authorization succeeded
    therefore execution succeeded

or:

    execution occurred
    therefore authorization existed

Audit is evidence.

Audit is not authority.

==================================================
17. ATTACK #14 — DEAD SECURITY BOUNDARIES
==================================================

Search for security logic that exists but is not actually reachable
from production.

Examples:

    unused authority service
    unused executor interface
    unused policy gate
    unused revalidation helper
    dead authorization function

A security boundary that is dead code is not a security boundary.

Identify:

    production
    test-only
    dead

for each security-critical component.

==================================================
18. ATTACK #15 — DATABASE / TRANSACTION SEMANTICS
==================================================

Inspect the actual SQL/transaction behavior used by:

    PrepareForAction
    kernel.Authorize
    ExecuteAction

Determine whether the state used for authorization is:

    current
    transactionally coherent
    exact

Look for:

    separate connection reads
    stale transaction snapshots
    read-before-write races
    mutation after authorization
    values reconstructed outside the transaction

Do not redesign the database unless a real correctness problem is found.

Do not add kernel invariants without justification.

==================================================
19. ATTACK #16 — TEST QUALITY
==================================================

Inspect the 21 integration tests themselves.

Do not merely trust their names.

For each important test determine:

    Does it actually enter the production execution path?
    Does it actually exercise kernel.Authorize?
    Does it actually prove executor was not called?
    Does it use live CockroachDB state?
    Does it mutate state at the correct point?
    Could the test pass if the security check were removed?

Identify tests that are:

    strong
    partial
    false-confidence
    redundant

A test that passes while the security boundary is bypassed is a finding.

==================================================
20. TEST-THE-TESTS / MUTATION MINDSET
==================================================

Conceptually perform mutation testing on the security boundary.

Ask:

    If kernel.Authorize were removed, which tests fail?

    If revocation checking were removed, which tests fail?

    If target matching were removed, which tests fail?

    If action matching were removed, which tests fail?

    If ExecuteAction bypassed preparation, which tests fail?

    If executor invocation occurred before authorization, which tests fail?

The important security tests should fail when their corresponding
security control is removed.

If not, identify the coverage gap.

==================================================
21. KERNEL PRESERVATION
==================================================

Verify:

    no kernel modifications were introduced
    unless explicitly justified

Confirm authority semantics remain:

    kernel.Authorize = authority oracle
    kernel.Approve = authority creation
    kernel.RevokeTarget = revocation

No service-level replacement exists.

==================================================
22. FINDINGS
==================================================

Classify findings as:

CRITICAL
    Exploitable authority bypass or direct consequential execution
    without current exact authorization.

HIGH
    Serious architectural/security weakness likely to become a bypass
    or that undermines a core trust boundary.

MEDIUM
    Meaningful weakness, incomplete defense, or misleading boundary
    without demonstrated immediate bypass.

LOW
    Documentation, maintainability, or non-security issue.

For every finding provide:

    ID
    severity
    file
    function / symbol
    attack scenario
    why it works
    expected architecture
    actual architecture
    recommended fix
    whether code change is required

Do not manufacture findings merely to avoid a GO.

Do not downgrade a real bypass because existing tests happen to pass.

==================================================
23. REQUIRED REVIEW OUTPUT
==================================================

Produce:

A. Actual production consequential execution call graph

B. Every executor invocation site

C. Every external consequential provider call

D. Every current kernel.Authorize call

E. Every independent authority decision site

F. Exact actor/authentication flow

G. Exact policy flow

H. Exact workflow-token role

I. Exact execution-time DB reads

J. Exact action/target/consequence tuple fields

K. Audit sequence

L. Test-quality assessment

M. Findings by severity

N. Remaining documented risks

O. Confirmation of kernel changes

P. Final verdict

==================================================
24. FINAL VERDICT
==================================================

End with exactly one:

GO

Only if:

    no CRITICAL findings
    no HIGH findings
    every consequential production path is authority-gated
    current kernel.Authorize is the final authority oracle
    exact tuple binding is preserved
    revocation is enforced
    policy cannot manufacture authority
    workflow cannot manufacture authority
    token cannot manufacture authority
    actor spoofing fails
    no direct provider bypass exists
    executor cannot create authority
    denied paths cannot reach executor
    live integration tests remain valid
    no dead security boundary remains
    no unexplained kernel/schema growth exists

Otherwise:

HOLD

Do not proceed to Phase 4 on HOLD.

Security correctness takes precedence over schedule.

IMPORTANT:
This is the FINAL targeted post-remediation adversarial review before
Phase 4. Review the repository as it exists NOW, after the successful
database-connected test run. Do not rely solely on previous review
reports or previous conclusions.
```

### The key distinction

This review should **not** ask, "Did the 21 tests pass?"

It should ask:

> **"Can I find a path in the code by which a consequential action can still occur without the exact, current kernel authorization?"**

That is the final question before Phase 4.

And I would keep the gate brutally simple:

```text
TARGETED CODE REVIEW
        ↓
0 Critical
0 High
        ↓
GO
        ↓
Phase 4
```

Anything less than that should remain **HOLD**.
