Use this as the planning prompt for **OpenCode**. I would make **Test Engineering a mandatory Phase 4C+ Step 0**, before any production implementation. The goal is exactly what you described: establish the target behavior and invariants first, then make the implementation prove itself against them.

```text
You are OpenCode, the IMPLEMENTATION agent for Solvent.

Produce an IMPLEMENTATION PLAN ONLY for the next phase:

    Phase 4C+ — First Real GitHub Executor

Do NOT modify code.
Do NOT implement anything.
Do NOT run remediation changes.
Do NOT start coding.

This task is specifically to design the implementation plan and lock the
test-engineering target before implementation begins.

==================================================
REVIEW / IMPLEMENTATION SEPARATION
==================================================

OpenCode is the implementation agent.

Kilo Code is the independent adversarial reviewer.

OpenCode and Kilo Code are separate software systems.

When implementation eventually begins:

    OpenCode
        ↓
    implement
        ↓
    verify
        ↓
    STOP
        ↓
    hand CURRENT repository to Kilo Code
        ↓
    Kilo Code independently reviews
        ↓
    GO / REQUEST CHANGES

Do NOT perform the final adversarial review yourself.
Do NOT substitute an OpenCode subagent for Kilo Code.

The Phase 4C+ plan must include the eventual Kilo Code independent review
as a mandatory completion gate.

==================================================
CURRENT VERIFIED STATE
==================================================

Phase 4C is CLOSED and FROZEN.

Kilo Code independently reviewed the final Phase 4C tree and returned:

    GO
    READY TO FREEZE

Current verified architecture:

    SMALL TRUSTED AUTHORITY KERNEL
            ↓
    STABLE CANONICAL API / OpenAPI
            ↓
    SERVICE LAYER
            ↓
    EXTENSION PLANE
            ↓
    AGENTS / APPS / WORKFLOWS / SYSTEMS

Phase 4C established:

- canonical OpenAPI 3.1 specification
- Canonical API Decision Record
- handwritten Python reference client
- GitHub reference integration
- security / API / extension documentation
- lightweight OpenAPI validation
- no kernel changes
- no schema changes
- no new services
- no production executor
- frozen API behavior

Phase 6.4 established and independently reviewed the current
authority/revocation serialization protocol:

    AuthorizeAndCreateIntent
        →
    SELECT authority_target ... FOR UPDATE
        →
    authority evaluation
        →
    intent creation

and:

    RevokeTarget
        →
    SELECT same authority_target ... FOR UPDATE
        →
    revocation

The stale-authority race was corrected and independently reviewed.

==================================================
PHASE 4C+ OBJECTIVE
==================================================

Phase 4C+ is the FIRST real consequential execution phase.

The objective is to prove that Solvent can control a real external side
effect:

    AI agent / workflow
        ↓
    Solvent authority
        ↓
    current authorization
        ↓
    real GitHub executor
        ↓
    GitHub API / Actions / deployment
        ↓
    real external side effect
        ↓
    execution result
        ↓
    audit

The phase should prove:

    authorization
        ≠
    execution

and:

    Solvent authority
        ≠
    GitHub provider semantics

Solvent must decide:

    "May this exact action happen?"

The executor performs:

    "Perform this already-authorized external action."

GitHub determines:

    "Did the external operation actually happen?"

==================================================
MOST IMPORTANT NEW REQUIREMENT:
TEST ENGINEERING FIRST
==================================================

Phase 4C+ MUST begin with a dedicated:

    STEP 0 — TEST ENGINEERING / TARGET DEFINITION

This is not merely "write some tests."

The purpose is to establish a rigorous, explicit target before production
implementation begins.

The plan must define:

    WHAT behavior are we trying to prove?
    WHAT security/correctness invariants must always hold?
    WHAT transitions are legal?
    WHAT transitions are forbidden?
    WHAT failure modes must be tested?
    WHAT external-provider behaviors must be simulated?
    WHAT evidence proves an execution happened?
    WHAT evidence proves it did NOT happen?
    HOW do we deterministically reproduce race/failure conditions?

Do NOT allow implementation to begin until the test target is explicit.

==================================================
TEST ENGINEERING PRINCIPLE
==================================================

Lock in this principle for Phase 4C+:

> First define the target and invariants.
> Then design deterministic tests against those invariants.
> Then implement the executor to satisfy those tests.
> Finally run independent adversarial review.

The test suite must not merely test what the implementation happens to do.

It must define what the implementation is REQUIRED to do.

Avoid tests whose assertions are derived from implementation details instead
of externally meaningful security/correctness properties.

==================================================
STEP 0 — TEST ENGINEERING DELIVERABLE
==================================================

The plan must produce a dedicated test-engineering specification before
production implementation.

Proposed artifact:

    docs/OS/phase4c_plus_test_target.md

The document must define:

### A. System under test

Clearly identify:

    authorization boundary
    execution boundary
    executor registry
    GitHub executor
    external provider boundary
    audit boundary

### B. Canonical execution state machine

Define the lifecycle precisely.

For example, determine whether the canonical conceptual sequence is:

    PROPOSED
        ↓
    AUTHORITY_ESTABLISHED
        ↓
    AUTHORIZED
        ↓
    EXECUTION_ATTEMPTED
        ↓
    EXECUTED
        ↓
    AUDITED

with failure / denied / cancelled / provider-failed states as appropriate.

Do not invent unnecessary workflow machinery.

This is a test model, not a new workflow engine.

### C. Security invariants

At minimum evaluate:

1. No external side effect before successful current authorization.

2. An executor cannot grant authority.

3. An executor cannot revoke authority.

4. An executor cannot reinterpret denied authorization as allowed.

5. A stale/cached authorization cannot be used.

6. Caller-controlled executor selection cannot bypass trusted registry mapping.

7. The exact authorized action/target must be the action sent to GitHub.

8. Authorization and execution are distinct events.

9. Provider success does not retroactively create authority.

10. Provider failure does not create false success.

11. Audit must distinguish:
       authorization decision
       execution attempt
       provider result

12. A failed provider operation must not be reported as successful execution.

13. A successful authorization does not prove successful external execution.

14. External execution must not occur when the authorization step fails.

15. Revoked authority must not permit a new execution attempt.

### D. TOCTOU target

Explicitly define the unavoidable boundary between:

    Solvent DB state
        ↓
    executor invocation
        ↓
    external provider

The plan must not claim an atomic transaction across CockroachDB and GitHub.

Define the actual guarantee to be proved.

At minimum evaluate:

    successful CURRENT authorization
        immediately precedes
    executor invocation

and identify what can still happen if:

    authority is revoked after authorization
        but before GitHub accepts the request.

This must become an explicit documented limitation or require a concrete
mitigation.

Do not hand-wave TOCTOU.

### E. Failure model

Define tests for:

- authorization denied
- target revoked
- wrong target
- wrong action
- wrong actor
- stale authority
- malformed parameters
- executor not registered
- executor returns error
- GitHub API returns 4xx
- GitHub API returns 5xx
- network timeout
- context cancellation
- provider accepts request but response is lost
- duplicate execution attempt
- retry after unknown external outcome
- audit write failure
- process crash between authorization and provider invocation
- process crash after provider side effect but before audit
- provider side effect succeeds but Solvent receives an error
- provider rejects action after Solvent authorization

Do not automatically promise recovery behavior that has not been designed.

The plan must explicitly classify each case:

    guaranteed safe
    detectable
    retryable
    requires operator reconciliation
    unresolved future limitation

### F. Idempotency target

Idempotency is currently a known v0 limitation.

Phase 4C+ must determine whether the first real GitHub execution can safely
operate without idempotency support.

If not, explain why.

Do not introduce a generic idempotency subsystem without concrete evidence.

If GitHub provides an operation identifier / workflow identifier / deployment
identifier that can serve as a provider-side deduplication anchor, evaluate it.

### G. Deterministic provider simulation

The test target must define a fake/test GitHub provider capable of deterministic:

    success
    rejection
    timeout
    connection failure
    ambiguous outcome
    duplicate submission
    delayed response

Do not use timing-based tests where deterministic synchronization is possible.

### H. Side-effect oracle

Define exactly how tests determine:

    external action DID happen

versus:

    external action DID NOT happen

Do not infer execution solely from:

    "executor returned nil"

Use a fake provider with observable recorded operations for tests.

For the real integration, identify GitHub's authoritative execution reference.

==================================================
TEST MATRIX
==================================================

The plan must contain a concrete matrix such as:

    Scenario
    Preconditions
    Operation
    Expected authorization
    Expected provider calls
    Expected external side effect
    Expected audit
    Expected final state

Include at least:

1. Valid authority → execution succeeds
2. No authority → zero provider calls
3. Revoked authority → zero provider calls
4. Wrong target → zero provider calls
5. Wrong action → zero provider calls
6. Wrong actor → zero provider calls
7. Executor missing → no external call
8. Provider rejection → authorization remains distinct from execution failure
9. Provider timeout → ambiguous outcome handled correctly
10. Duplicate invocation → behavior explicitly defined
11. Revocation before authorization → no execution
12. Revocation after authorization but before provider call → define guarantee
13. Provider succeeds but response is lost → define reconciliation
14. Audit failure after provider success → define accurate state/reporting
15. Attempt to inject arbitrary executor → rejected
16. Attempt to bypass current authorization → rejected

The matrix must identify which properties are:

    security invariants
    correctness invariants
    operational behavior
    known limitations

==================================================
TEST ENGINEERING QUALITY BAR
==================================================

The plan must explicitly reject:

- sleep-based race tests where synchronization is possible
- tests that only assert return values without checking provider side effects
- mocks that allow impossible provider behavior
- tests that derive expected behavior from the implementation itself
- tests that pass because authorization was accidentally denied early
- tests that cannot distinguish "not called" from "called and failed"
- tests that ignore ambiguous external outcomes
- tests that equate an executor return value with provider truth

The target test suite must be capable of detecting a regression where:

    authorization is bypassed
    wrong target is executed
    stale authority executes
    denied action reaches GitHub
    provider failure is reported as success

==================================================
ONLY AFTER STEP 0:
IMPLEMENTATION PLAN
==================================================

After the test target is defined, plan the production implementation.

Inspect the CURRENT repository first.

Determine:

1. Exact current executor abstraction.
2. Exact `ActionFunc` signature.
3. Exact `Registry` behavior.
4. Exact `ExecuteAction` path.
5. Exact `service/authority.ExecuteAction`.
6. Whether `ExecuteAction` currently receives enough validated context.
7. Whether caller-controlled action/executor names can select arbitrary capabilities.
8. Exact API/service entry point for future execution.
9. Existing audit API.
10. Existing external adapter patterns.
11. Existing GitHub adapter capabilities.
12. Existing GitHub API dependencies, if any.
13. Existing configuration/secrets conventions.
14. Existing test infrastructure for provider simulation.

Do not assume the Phase 4C design is sufficient for real execution.

Inspect reality.

==================================================
FIRST REAL EXECUTOR
==================================================

Preferred target:

    GitHub Actions / Deployment executor

Determine the narrowest useful real action.

Prefer one concrete action such as:

    trigger a specific GitHub Actions workflow
or
    create a deployment through a specific GitHub deployment endpoint

Do NOT build:

    generic GitHub automation platform
    generic CI/CD engine
    multi-provider executor framework
    generalized workflow engine

The first executor should be one narrow consequential capability.

==================================================
EXECUTOR SELECTION SECURITY
==================================================

Review the current registry model carefully.

A central security question:

    Can an API caller influence the executor/action registry key in a way
    that grants arbitrary external capability?

The canonical model should be:

    trusted internal action mapping
        ↓
    validated authorization
        ↓
    registry-resolved executor
        ↓
    external provider

Do not allow:

    caller supplies arbitrary executable capability
        ↓
    registry executes it

If the current architecture cannot safely constrain executor selection,
identify the smallest required fix.

Do NOT automatically expand the kernel.

==================================================
REAL EXECUTION PATH
==================================================

Determine the canonical eventual execution path.

Prefer:

    request
      ↓
    current-state authorization
      ↓
    validated execution intent
      ↓
    ExecutionService / existing execution boundary
      ↓
    Executor registry
      ↓
    GitHub executor
      ↓
    GitHub API
      ↓
    execution result
      ↓
    audit

But do not invent a new service if the existing boundaries are sufficient.

Explicitly evaluate whether:

    service/authority.ExecuteAction

can serve the role without becoming a second authority engine.

==================================================
AUTHORIZATION BEFORE EXECUTION
==================================================

The plan must prove:

    NO successful authorization
        →
    NO external provider call

and:

    stale authorization
        →
    NO external provider call

and:

    wrong target/action
        →
    NO external provider call

Do not trust the existence of an `action_intent` row alone.

Determine whether the execution path must re-read current authorization immediately
before execution.

If yes, specify exactly where.

==================================================
AUDIT
==================================================

Determine exactly what gets recorded.

At minimum distinguish:

    authorization requested
    authorization allowed/denied
    execution attempted
    provider accepted/rejected
    external reference
    execution result

Do not merge these into one "success" event.

Important rule:

    Authorize != Execute

A successful authorization must never be represented as proof that GitHub
actually performed the action.

==================================================
REAL GITHUB INTEGRATION
==================================================

Determine:

- GitHub API mechanism
- authentication/token strategy
- repository/environment targeting
- workflow/deployment target
- provider response
- external execution identifier
- polling or asynchronous result handling, if needed
- timeout
- retries
- provider rate limits
- error mapping
- secret handling
- local development strategy
- test strategy without real production side effects

Do not claim a real GitHub operation succeeded unless the provider confirms it.

Do not place GitHub-specific semantics in the kernel.

==================================================
SECRETS / CREDENTIALS
==================================================

Plan secure credential handling.

Do NOT:

- hardcode tokens
- commit tokens
- place provider credentials in the kernel
- put provider credentials into authorization state
- expose credentials through API responses
- log credentials

Determine the smallest configuration mechanism consistent with the existing
repository conventions.

==================================================
TOCTOU / EXECUTION BOUNDARY
==================================================

This is a REQUIRED section.

Explicitly model:

    T1:
        Solvent authorization succeeds

    T2:
        execution starts

    T3:
        provider accepts/rejects

Analyze what happens if:

    revocation occurs between T1 and T2
    target changes between T1 and T2
    provider state changes between T2 and T3

Do not claim database transaction atomicity with GitHub.

Determine the actual strongest guarantee available.

If a stronger guarantee requires new kernel semantics, apply the kernel-growth
gate rather than assuming a solution.

==================================================
KERNEL-GROWTH GATE
==================================================

For EVERY proposed production change ask:

    Is this:
      - a new durable security fact?
      - a new atomic security-critical state transition?
      - impossible to represent safely above the kernel?

If NO:

    keep it above the kernel.

If YES:

    explicitly justify the kernel change.

Default:

    executor logic → extension plane
    provider semantics → adapter
    execution orchestration → service
    provider credentials → deployment/config
    audit → audit service
    UI → product layer

Do not grow the kernel merely because execution is consequential.

==================================================
NO PREMATURE EXECUTOR ABSTRACTION
==================================================

The current executor contract is intentionally:

    ActionFunc

Do not automatically introduce:

    AuthorizationContext
    ExecutionResult
    AuditHandoff
    ProviderContext

before concrete requirements justify them.

The first real GitHub executor should reveal what information is genuinely
required.

If the current `ActionFunc` proves insufficient, document the exact failure
mode and the minimum extension required.

==================================================
OBSERVABILITY
==================================================

Determine what operational observability is needed for:

- execution attempt
- provider request
- provider response
- external execution ID
- timeout
- retry
- ambiguous outcome

Do not build a large observability platform.

Use existing audit/logging mechanisms where sufficient.

==================================================
REAL SIDE-EFFECT SAFETY
==================================================

The plan MUST distinguish:

    test executor
from
    real GitHub executor

and define explicit safety controls for development/testing.

Prefer:

- dedicated test repository
- test workflow
- dry-run only where provider actually supports it
- scoped GitHub credentials
- dedicated environment

Never make the default developer test accidentally deploy production.

==================================================
PHASE BOUNDARIES
==================================================

Do NOT silently absorb future work.

Phase 4C+ should be one real executor.

Explicitly defer:

- second provider
- Kubernetes executor
- AWS executor
- generic executor platform
- multi-cloud
- generalized workflow engine
- enterprise identity
- multi-tenancy
- RBAC
- compliance platform

The goal is:

    one deep real integration
        →
    prove extension architecture
        →
    learn what abstractions are actually needed

==================================================
ADVERSARIAL TEST ENGINEERING
==================================================

The plan must include tests designed to break the implementation.

Examples:

1. Remove/skip current authorization → test must fail.
2. Change target ID → provider call must not happen.
3. Change action → provider call must not happen.
4. Use revoked target → provider call must not happen.
5. Substitute arbitrary executor name → must fail.
6. Force executor invocation with denied authorization → must fail.
7. Return provider error as success → test must fail.
8. Return executor success without provider call → test must fail.
9. Simulate lost provider response → test must detect ambiguous state.
10. Reuse stale authorization → test must fail.
11. Revoke immediately before execution → expected result must be enforced.
12. Cause audit failure → execution result must remain truthful.

For every test ask:

    Would this test fail if the security property were accidentally removed?

If not, redesign the test.

==================================================
TEST MATRIX BEFORE CODE
==================================================

The final plan MUST include the test matrix before the implementation steps.

No production executor work should begin until:

    target behavior
    state model
    invariants
    failure model
    side-effect oracle
    test matrix

are explicit.

This is a mandatory planning gate.

==================================================
IMPLEMENTATION PLAN DELIVERABLES
==================================================

Produce a concrete Phase 4C+ plan containing:

1. Executive Decision
2. Scope
3. Non-Goals
4. Current-State Reconnaissance
5. STEP 0 — Test Engineering / Target Definition
6. State Model
7. Security Invariants
8. Correctness Invariants
9. Failure Model
10. TOCTOU Model
11. Test Matrix
12. Deterministic Provider Test Harness
13. Executor Boundary Analysis
14. Executor Selection Security
15. GitHub Action/Deployment Choice
16. Real GitHub Executor Design
17. Credential / Secret Handling
18. Authorization → Execution Boundary
19. Audit / Execution Evidence
20. Observability
21. Kernel-Growth Gate
22. Package/File Implementation Map
23. Implementation Order
24. Verification Plan
25. Adversarial Test Plan
26. Acceptance Criteria
27. Rollback Strategy
28. Risks / Open Questions
29. Kilo Code Independent Review Gate
30. Explicit next phase

==================================================
PACKAGE / FILE MAP
==================================================

For every proposed file, specify:

- path
- purpose
- production/test/docs
- dependency
- new abstraction?
- why it belongs there
- security relevance

Do not invent files merely for symmetry.

==================================================
ACCEPTANCE CRITERIA
==================================================

The plan should define completion gates including:

    go build ./...
    go vet ./...
    go test -count=1 -p 1 ./...
    go test -count=1 ./...

plus:

    deterministic executor tests
    denied-action side-effect tests
    wrong-target side-effect tests
    revoked-authority side-effect tests
    provider failure tests
    ambiguous outcome tests
    executor-selection security tests
    audit/result consistency tests
    real GitHub integration test using a safely scoped environment

and:

    independent Kilo Code adversarial review → GO

Do not call the phase complete merely because the GitHub executor returns
success.

The test oracle must establish what actually happened at the provider.

==================================================
FINAL SECURITY QUESTION
==================================================

The plan MUST explicitly answer:

> What is the strongest security guarantee Solvent can honestly make about
> a real external side effect?

Do not overclaim.

For example, distinguish:

    "Solvent authorized the action"

from:

    "Solvent invoked the executor"

from:

    "GitHub accepted the request"

from:

    "GitHub completed the deployment"

These are four different facts.

The system must preserve that distinction.

==================================================
FINAL ARCHITECTURAL QUESTION
==================================================

The plan MUST explicitly answer:

> How does adding the first real executor strengthen Solvent's role as the
> universal authorization layer without turning the kernel into an execution
> engine?

The answer should preserve:

    SMALL TRUSTED AUTHORITY KERNEL
                ↓
        STABLE CANONICAL API
                ↓
         SERVICE / POLICY
                ↓
        ADAPTER / EXECUTOR
                ↓
        EXTERNAL PROVIDER

The kernel answers:

    MAY THIS HAPPEN?

The extension plane determines:

    HOW DO WE ACT ON THAT DECISION?

==================================================
OUTPUT FORMAT
==================================================

Return exactly:

# Phase 4C+ Implementation Plan

## 1. Executive Decision
## 2. Scope
## 3. Non-Goals
## 4. Current-State Reconnaissance
## 5. Test Engineering Gate — Step 0
## 6. Canonical State Model
## 7. Security Invariants
## 8. Correctness Invariants
## 9. Failure Model
## 10. TOCTOU / External Side-Effect Model
## 11. Test Matrix
## 12. Deterministic Provider Test Harness
## 13. Executor Boundary
## 14. Executor Selection Security
## 15. GitHub Execution Choice
## 16. Real GitHub Executor
## 17. Credential / Secret Handling
## 18. Authorization → Execution
## 19. Audit / Evidence
## 20. Observability
## 21. Kernel-Growth Gate
## 22. Package / File Map
## 23. Implementation Order
## 24. Verification
## 25. Adversarial Testing
## 26. Acceptance Criteria
## 27. Rollback
## 28. Risks / Open Questions
## 29. Kilo Code Independent Review Gate
## 30. Next Phase
## 31. Final Recommendation

==================================================
FINAL RULE
==================================================

Do not write code.

Do not modify the repository.

Do not assume the existing executor abstraction is sufficient.

Do not invent a large execution framework.

Do not add kernel features merely because external execution is consequential.

MOST IMPORTANT:

    TEST ENGINEERING COMES FIRST.

Before implementation begins, the plan must define exactly:

    TARGET
    INVARIANTS
    STATE MODEL
    FAILURE MODEL
    SIDE-EFFECT ORACLE
    TEST MATRIX

so that the implementation can later be judged against a rigorously defined
target rather than against its own behavior.

The ultimate objective remains:

    Solvent becomes the authorization layer for
    AI agents, applications, workflows, and consequential systems

while maintaining:

    a small trusted kernel
    +
    a powerful extension plane
```

The key addition is the **Step 0 test-engineering gate**. I would treat that as a permanent process rule for future consequential phases too:

```text
Target definition
      ↓
Invariants
      ↓
State/failure model
      ↓
Deterministic tests
      ↓
Implementation
      ↓
Fresh verification
      ↓
Kilo Code adversarial review
```

That prevents the executor phase from falling into the same trap we encountered earlier with the concurrency testing: **a passing test is not useful unless we first know exactly what property the test is supposed to prove.**
