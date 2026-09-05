Agreed. **Before Phase 4, I would run one final implementation-specific adversarial review that ignores the 16-point GO summary and attacks the actual code changes made in the last remediation.**

The previous review established the architecture. This one should answer a narrower question:

> **Did the exact implementation we just changed introduce any subtle defect, semantic regression, inconsistent contract, or hidden bypass?**

Use this prompt:

```text id="r8final"
SOLVENT — FINAL IMPLEMENTATION-SPECIFIC ADVERSARIAL CODE REVIEW
PRE-PHASE-4 / POST-PLAN-6 CLOSEOUT

IMPORTANT:

This is an independent, implementation-specific security review of
the CURRENT repository state.

Do NOT rely on:
- previous adversarial review conclusions
- previous GO reports
- the 16-point checklist alone
- documentation claims
- test names
- statements that "Plan 6 is complete"

Inspect the actual current code.

The objective is to find anything that slipped through the previous
reviews specifically because the implementation was assumed correct.

DO NOT implement fixes during this review.

This is READ-ONLY analysis.

==================================================
1. REVIEW THE EXACT RECENT IMPLEMENTATION CHANGES
==================================================

Focus first on the actual files modified during the latest
security-remediation cycle:

    cmd/solvent-mcp/main.go
    cmd/solvent-mcp/tools.go
    cmd/operator-review/main.go
    service/authority/authority.go
    service/authority/authority_integration_test.go
    service/executor/
    service/policy/
    service/audit/
    internal/wizard/
    AGENTS.md
    docs/os/plan6.md
    docs/os/security_gate_report_v2.md

Also inspect deleted files and git history/diff where available.

Start by reconstructing:

    BEFORE
      ↓
    CHANGE
      ↓
    AFTER

For every security-relevant change, answer:

    What changed?
    Why did it change?
    What invariant was it supposed to establish?
    Is the invariant actually established?
    Did the change create a new alternate path?

==================================================
2. MCP SCHEMA ↔ RUNTIME CONTRACT
==================================================

Inspect the complete definition of:

    solvent_authorize_action

Verify:

    target_id ∈ required[]
    actor_id ∈ required[]
    action_source ∈ required[] if intended
    all required properties have correct types

Then inspect the handler.

Verify all of these independently:

    missing target_id
        → rejected

    missing actor_id
        → rejected

    empty target_id
        → rejected

    empty actor_id
        → rejected

    malformed target_id
        → rejected

    malformed actor_id
        → rejected

    valid target_id + actor_id
        → normal behavior preserved

Critical:

Verify there is NO second MCP schema definition,
compatibility schema, alias, tool registration, generated schema,
or alternate handler that still advertises these fields as optional.

Verify schema and runtime semantics are identical.

==================================================
3. MCP DOWNGRADE ATTACK
==================================================

Attempt to find every path by which:

    incomplete authorization context

can become:

    weaker authorization behavior

Search for:

    if field != ""
    if field == ""
    optional actor_id
    optional target_id
    fallback actor
    fallback target
    missing target
    missing actor
    default actor
    default target
    skip PrepareForAction
    bypass PrepareForAction

Do this across ALL MCP tools, not only
solvent_authorize_action.

The required invariant is:

    missing security context
        →
    fail closed

No silent downgrade.

==================================================
4. MCP TRUST BOUNDARY
==================================================

Trace:

    MCP request
      ↓
    request parsing
      ↓
    actor identity
      ↓
    action_source
      ↓
    service
      ↓
    kernel

Determine precisely what is trusted and what is not.

Verify:

    request.body.actor
    request.body.actor_type
    request.body.action_source
    request.body.user_typed

cannot become authentication.

If MCP is intentionally a trusted administrative surface,
verify the documentation and code agree about that assumption.

Do not accept "trusted" as a substitute for understanding exactly
what authority the MCP process has.

==================================================
5. AUTHORITY SERVICE IMPLEMENTATION
==================================================

Review:

    PrepareForAction
    ExecuteAction

line by line.

Verify:

    service gathers context
        ↓
    AuthorityTuple
        ↓
    kernel.Authorize
        ↓
    authoritative result

Confirm the service does NOT independently implement:

    activation validity
    revocation validity
    snapshot matching
    principal matching
    consequence matching

Search for duplicate SQL or comparison logic.

==================================================
6. EXACT TUPLE PRESERVATION
==================================================

Trace ALL tuple fields from external input to execution.

At minimum verify:

    principal / actor
    target
    action / consequence type
    consequence parameters
    snapshot
    relevant belief/justification context

For every field:

    input
      ↓
    PrepareForAction
      ↓
    AuthorityTuple
      ↓
    kernel.Authorize
      ↓
    ExecuteAction
      ↓
    Executor

Verify the value cannot silently change between any two stages.

Pay particular attention to:

    consequenceParameters
    JSON serialization
    empty vs nil
    omitted vs empty
    string normalization
    default values

A field that is authorized as X but executed as Y is CRITICAL.

==================================================
7. EXECUTEACTION CALL GRAPH
==================================================

Search the whole repository for:

    ExecuteAction(
    .ExecuteAction(
    executor.Execute(
    Executor
    Registry
    Register(
    Get(

Produce every caller.

Classify:

    production
    test-only
    dead
    unreachable

Verify the implementation did NOT accidentally leave an alternative
execution entry point.

Also verify an untrusted caller cannot inject:

    executor
    function
    implementation
    provider
    provider client

into ExecuteAction.

==================================================
8. EMPTY EXECUTOR REGISTRY
==================================================

Verify the empty production executor registry is genuinely intentional.

For:

    cmd/solvent-mcp/main.go
    demo/cloud/web/main.go

verify:

    no executor is registered
    no hidden default executor exists
    no fallback executor exists
    no dynamic executor loading exists
    no provider is silently substituted

Confirm documentation correctly describes this as:

    FUTURE EXECUTION WIRING

and not:

    active execution capability

==================================================
9. NO HIDDEN EXTERNAL SIDE EFFECT
==================================================

Perform a complete source search for external effects.

Search for:

    exec.Command
    os/exec
    client.Do
    http.Do
    grpc
    POST
    PUT
    PATCH
    DELETE
    SDK mutation methods
    GitHub mutation APIs
    cloud mutation APIs
    shell execution
    subprocesses
    queues
    webhooks
    callbacks
    background workers

For EACH result inspect the surrounding implementation.

Do not classify by filename alone.

Determine whether it can cause:

    external consequential side effect

If yes:

    show exact caller
    show exact authorization path

Special rule:

    intent creation
    evidence ingestion
    belief promotion
    authority creation

MUST NOT directly invoke consequential external side effects.

==================================================
10. DIRECT IntentOnPromoted REVIEW
==================================================

Find EVERY:

    IntentOnPromoted

call.

For each, classify:

    production
    admin CLI
    deployment tooling
    test
    internal library

For every production-facing one ask:

    Does it create intent?
    Does it execute?
    Does it require authority context?
    Is bypass intentional?
    Is it documented?

Pay special attention to:

    cmd/operator-review
    wizard
    MCP
    pipeline
    demo/cloud/init
    internal/intent

No intent creation path may secretly cause external execution.

==================================================
11. ACTION_INTENT MUTABILITY
==================================================

Inspect:

    CREATE
    UPDATE
    DELETE
    RETRACT
    CANCEL
    READ

for action_intent.

Verify:

    actor cannot change after creation
    target cannot change after creation
    action cannot change after creation
    consequence parameters cannot change after creation

Search direct SQL and ORM/write helpers.

Determine whether a caller can create an intent first and then mutate
its meaning without reauthorization.

==================================================
12. AUTHORITY IMMUTABILITY
==================================================

Inspect:

    authority_target
    target_snapshot
    target_activation
    target_revocation
    justification
    debt_discharge

Verify:

    snapshot cannot be modified
    activation cannot be modified
    revocation remains append-only
    old authority cannot be retargeted
    old authority cannot be repurposed
    justification cannot silently change the approved meaning

Search all UPDATE / DELETE statements touching these tables.

==================================================
13. REVOCATION
==================================================

Trace:

    Approve
    Authorize
    RevokeTarget

Verify:

    revocation is current-state authoritative
    revoked authority cannot authorize
    stale activation cannot override revocation
    service code cannot cache "authorized=true"

Inspect exact SQL used by kernel.Authorize.

Do not merely rely on test names.

==================================================
14. POLICY
==================================================

Inspect every remaining use of:

    PolicyConstraints
    EvaluateConstraints

Verify there is no semantic reintroduction of:

    policy allowed = authority allowed

Look for fields such as:

    Allowed
    Authorized
    Approved
    Permitted

being passed between layers with ambiguous meaning.

Policy may constrain.

Policy may never manufacture authority.

==================================================
15. DEAD-CODE AUDIT
==================================================

Search for:

    unreachable security functions
    unused services
    stale interfaces
    obsolete workflow references
    dead executor registrations
    dead authorization helpers

Classify each:

    ACTIVE
    TEST-ONLY
    FUTURE
    DEAD
    REMOVE

Do not call something an active security boundary if production
cannot reach it.

Do not resurrect dead components merely to eliminate the finding.

==================================================
16. OPERATOR-REVIEW TRUST EXCEPTION
==================================================

Review the documented exception around:

    cmd/operator-review

Verify:

    it is truly administrative
    it is not runtime agent execution
    it cannot invoke an external consequential executor
    it does not create hidden side effects
    its trust assumptions are explicit

Then answer this adversarial question:

    Can a stale operator session cause an unintended consequential
    action?

If the answer is "it can create an intent but cannot execute anything",
state that clearly.

Do not convert the existence of a trusted admin tool into a blanket
security exemption.

==================================================
17. AUTHORIZATION-FIRST SCOPE
==================================================

Verify documentation consistently states:

CURRENT:

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

Search documentation for contradictory claims such as:

    "Solvent executes"
    "executor available"
    "deployment supported"
    "authorization causes execution"

Any contradiction should be reported.

==================================================
18. TEST QUALITY — IMPLEMENTATION SPECIFIC
==================================================

Inspect the actual newly-added MCP regression tests.

Verify they test the real handler/schema contract rather than only
calling an internal helper.

Required:

    missing target_id
      → rejection
      → no intent

    missing actor_id
      → rejection
      → no intent

    malformed target_id
      → rejection

    malformed actor_id
      → rejection

    valid inputs
      → existing behavior preserved

Then inspect the 21 authority integration tests again.

For each critical test ask:

    Could this test still pass if the implementation had an
    accidental alternate path?

==================================================
19. MUTATION-STYLE REVIEW

Without modifying the repository, reason through these mutations:

A. Remove the MCP required-array entries.
   Which tests fail?

B. Remove runtime missing-target validation.
   Which tests fail?

C. Remove runtime missing-actor validation.
   Which tests fail?

D. Skip PrepareForAction.
   Which tests fail?

E. Replace kernel.Authorize result with policy constraints.
   Which tests fail?

F. Remove revocation checking.
   Which tests fail?

G. Stop comparing consequence parameters.
   Which tests fail?

H. Invoke executor before authorization.
   Which tests fail?

I. Call executor directly from MCP.
   Which tests fail?

J. Add a second direct provider call.
   Would the current test suite detect it?

If important mutations would not be detected, identify the coverage
gap.

==================================================
20. BUILD/CONFIGURATION SURFACE
==================================================

Inspect:

    build tags
    alternate binaries
    environment flags
    config switches
    test mode
    demo mode
    production mode

Ask:

    Can a configuration option disable authority verification?

    Can a debug flag enable execution?

    Can demo mode bypass authorization?

    Can an environment variable substitute an executor?

    Can alternate binaries bypass the service boundary?

==================================================
21. ERROR-HANDLING REVIEW
==================================================

Inspect all security-sensitive error handling.

Look for:

    ignored errors
    zero-value fallback
    nil fallback
    empty UUID fallback
    default actor
    default target
    default action
    default authorization
    "best effort" behavior
    warnings followed by execution

Required:

    authorization uncertainty
        →
    DENIED / error

Never:

    authorization uncertainty
        →
    continue

==================================================
22. DATABASE REVIEW
==================================================

Verify:

    authority-core schema unchanged
    no new authority tables
    no duplicate authority state
    no direct service writes to authority tables
    no accidental migration changes

Inspect every new/modified SQL statement.

Confirm DB constraints still protect:

    target activation uniqueness
    snapshot integrity
    belief promotion gating
    revocation semantics
    exact binding

==================================================
23. SECURITY CLAIM VS ACTUAL GUARANTEE
==================================================

For each documented claim, determine:

    PROVEN IN CODE
    PROVEN IN TEST
    DOCUMENTED ASSUMPTION
    UNPROVEN

Especially inspect:

    current authority wins
    exact tuple binding
    actor trust boundary
    policy separation
    token separation
    no consequential side effects
    future ExecuteAction boundary

Do not allow documentation to upgrade an unproven property into a
security guarantee.

==================================================
24. FINAL FINDINGS
==================================================

Classify:

CRITICAL
    Exploitable current authority bypass or consequential side effect.

HIGH
    Serious current security-boundary defect.

MEDIUM
    Meaningful weakness without demonstrated immediate bypass.

LOW
    Documentation, maintainability, or minor issue.

INFO
    Explicitly accepted design property or future limitation.

For each finding:

    ID
    severity
    exact file
    exact function/symbol
    attack
    actual behavior
    expected behavior
    exploitability
    impact
    recommended correction
    blocks Phase 4? YES/NO

Do NOT manufacture findings.

Do NOT downgrade an actual bypass because tests happen to pass.

==================================================
25. REQUIRED FINAL TABLE
==================================================

Produce:

| Area | Reviewed | Result | Findings |
|------|----------|--------|----------|
| MCP schema/runtime | | | |
| MCP downgrade paths | | | |
| Authentication | | | |
| Authority service | | | |
| ExecuteAction | | | |
| Executor registry | | | |
| External side effects | | | |
| Intent lifecycle | | | |
| Authority lifecycle | | | |
| Revocation | | | |
| Policy | | | |
| Operator CLI | | | |
| Database | | | |
| Transactions | | | |
| Error handling | | | |
| Tests | | | |
| Build/config surface | | | |
| Documentation alignment | | | |

==================================================
26. PHASE-4 RELEASE GATE
==================================================

End with exactly one:

GO

only if:

- no CRITICAL findings
- no HIGH findings
- MCP schema and runtime contract agree
- missing actor/target fail closed
- no silent authorization downgrade exists
- no existing consequential external side effect bypass exists
- no hidden consequential side effect exists
- kernel.Authorize remains final authority oracle
- exact tuple binding is preserved
- revocation is enforced
- consequence parameters are enforced
- intent cannot become authority
- evidence cannot become authority
- policy cannot become authority
- workflow/token cannot become authority
- operator-review exception is explicit and bounded
- no service-level authority oracle exists
- no second execution path exists
- no production executor exists unless intentionally introduced
- future ExecuteAction contract is preserved
- test suite is meaningful
- no unexplained kernel/schema changes exist
- documentation matches actual implementation

OR:

HOLD

if any CRITICAL/HIGH issue remains or any current production
security boundary is ambiguous.

==================================================
27. FINAL RULE

Do NOT start Phase 4 from the fact that the previous report said GO.

The previous report is evidence only.

Independently inspect the CURRENT repository.

The question is:

    "Is there anything in the actual implementation that could
     surprise us later?"

Find it now.

If nothing significant remains, say so and issue GO.

If something remains, HOLD it even if the implementation otherwise
looks clean.

Security correctness takes precedence over schedule.
```

I would use this **instead of another generic "does everything look good?" review**. It focuses on the exact implementation that just changed and, importantly, attacks the seams between schema, handler, service, kernel, intent, and future execution.

The two areas I would watch especially closely are:

```text
MCP schema
    ↔
runtime validation
    ↔
PrepareForAction

and

IntentOnPromoted
    ↔
future ExecuteAction
```

Those are where the project has repeatedly found subtle boundary inconsistencies.

**Do not move to Phase 4 until this review independently returns GO.**
