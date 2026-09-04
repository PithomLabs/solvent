## background

Below is the revised consolidated prompt I would give the coding agent. It incorporates the original commercial-MVP design, the compliance/governance layer, the five-demo strategy, the small-kernel philosophy, and the latest plan evaluation fixes—especially the critical “token != authority” correction. The evaluation confirms the architecture is sound after these implementation-level changes. One particularly important improvement over the previous prompt is the explicit separation of workflow continuity, identity, evidence, authority, and execution. The evaluation found that the previous plan's Authority field and untyped token refresh were the one area that could have reintroduced precisely the class of authority-bypass problem the architecture was intended to prevent. I would treat this consolidated version as the implementation contract for the next coding-agent pass.


# SOLVENT COMMERCIAL MVP — CONSOLIDATED IMPLEMENTATION PROMPT

You are the senior staff/principal engineer responsible for extending
the Pithom Labs Solvent repository from its current technically strong
v0 kernel/demo into a minimum viable commercial product.

This is an implementation task, not an architectural blank slate.

The central objective is:

    MAKE SOLVENT COMMERCIAL WITHOUT TURNING THE KERNEL INTO THE PRODUCT.

The commercial MVP must make Solvent understandable and useful to:

    - security engineers
    - security operations
    - platform/infrastructure teams
    - AI/agent developers
    - compliance/governance teams
    - technical buyers

while preserving the small trusted authority kernel as Solvent's primary
technical differentiator.

======================================================================
0. PRODUCT THESIS
======================================================================

Solvent is:

    A portable authority layer for autonomous systems.

Core distinction:

    Evidence is not authority.
    Agent output is not authority.
    Retrieval is not authority.
    Workflow state is not authority.
    A workflow token is not authority.
    Authorization is not execution.

An action is allowed only when current authoritative Solvent state
establishes the required authority.

The market should see:

    a practical product for governing consequential AI-driven actions.

The security engineer should see:

    a small, deterministic, auditable authority kernel.

The developer should see:

    clean extension points that normally require no kernel changes.

The compliance team should see:

    traceable evidence connecting policy/control → evidence →
    decision → authority → action → execution outcome.

======================================================================
1. NON-NEGOTIABLE ARCHITECTURAL PRINCIPLE
======================================================================

KEEP THE KERNEL SMALL.

The kernel is the smallest trusted authority core.

Default:

    DO NOT MODIFY THE KERNEL.

Use this decision tree for every requested capability:

    Does it understand an external product/protocol?
        -> ADAPTER

    Is it workflow, orchestration, composition, or policy?
        -> SERVICE / POLICY

    Is it execution or infrastructure?
        -> EXECUTOR / DEPLOYMENT

    Is it customer-specific behavior?
        -> POLICY / CONFIGURATION / DATA

    Is it reporting, UI, analytics, scoring, audit presentation?
        -> PRODUCT / READ MODEL / SERVICE

    Does it require a new security-critical durable fact
    or atomic state transition that cannot be expressed using
    existing kernel primitives?
        -> POSSIBLE KERNEL CHANGE

    Must a previously possible invalid state become
    structurally impossible?
        -> POSSIBLE DB INVARIANT

    Otherwise:
        -> DO NOT ADD IT

This rule is mandatory.

Do not add domain concepts to the kernel merely because they are
commercially useful.

Examples that MUST remain outside the kernel:

    Sentry
    Agentjacking
    GitHub
    MCP-specific behavior
    REST
    A2A
    procurement
    incident response
    compliance frameworks
    risk scoring
    document generation
    e-signature
    Kubernetes
    AWS
    Cloudflare
    Slack
    Jira
    customer-specific workflows

======================================================================
2. TRUST BOUNDARIES
======================================================================

Maintain these boundaries:

    External systems
        ↓
    Adapters
        ↓
    Service / Policy layer
        ↓
    Solvent Kernel
        ↓
    CockroachDB

And separately:

    Solvent authorization
        ↓
    Executor / Deployment adapter
        ↓
    Real-world system

The kernel establishes authority.

The executor performs external work.

Never collapse:

    authorization
    execution
    workflow continuity
    actor identity
    evidence

into one concept.

======================================================================
3. RECONNAISSANCE BEFORE CODING
======================================================================

Before changing code:

1. Inspect the complete repository.

2. Determine the actual current architecture from code.

3. Identify:

    - kernel packages
    - current service/application packages
    - belief lifecycle
    - authority lifecycle
    - action_intent lifecycle
    - DB schema/migrations
    - MCP tools
    - current HTTP/web server
    - current wizard/demo
    - existing adapters
    - existing views/projections
    - Lean model
    - existing audit/refusal mechanisms
    - test harness
    - task commands
    - deployment configuration

4. Run all existing test and verification gates.

5. Do not assume documentation matches implementation.

6. Produce a short implementation map before editing.

7. Identify existing functionality that should be wrapped/reused rather
   than duplicated.

Do not rewrite functioning kernel functionality to fit the product shell.

======================================================================
4. PRESERVE CURRENT SOLVENT AUTHORITY SEMANTICS
======================================================================

Preserve the existing authority model and its DB semantics.

Treat the current authority lifecycle as canonical:

    principal
    authority_target
    target_snapshot
    target_activation
    target_revocation
    justification
    debt_discharge

Preserve:

    Approve
    Authorize
    RevokeTarget

Preserve:

    snapshot immutability
    exact target binding
    activation uniqueness
    debt/promotion gates
    transactional authority creation
    read-only authorization verification
    DB-enforced invariants
    formal verification

Do not create a competing authority engine in the service layer.

Service logic may orchestrate and validate.

The kernel remains authoritative for atomic security state transitions.

======================================================================
5. KERNEL GROWTH GATE
======================================================================

Before ANY kernel or authority-core schema change, create an ADR with:

    Problem
    Why adapter cannot solve it
    Why service/policy cannot solve it
    Why configuration cannot solve it
    New durable security fact
    New atomic state transition
    Required DB invariant
    Impact on current authority semantics
    Impact on Lean verification
    Migration strategy
    Backward compatibility
    Security argument

Default decision:

    REJECT KERNEL CHANGE

unless the ADR demonstrates that the current kernel primitives are
fundamentally insufficient.

Do not weaken this standard because the feature would make the product
easier to implement.

======================================================================
6. PRODUCT MVP WORKFLOW
======================================================================

The MVP should provide one coherent end-to-end model:

    untrusted / AI-generated information
                ↓
            Solvent review
                ↓
        evidence + obligations
                ↓
          human decision
                ↓
        explicit authority
                ↓
      exact action authorization
                ↓
        external execution
                ↓
           audit ledger
                ↓
      compliance evidence package

The MVP must answer:

    1. What does the agent believe?
    2. What evidence supports it?
    3. What remains unresolved?
    4. Who is allowed to approve it?
    5. Which action is requested?
    6. Which target is affected?
    7. Why was it allowed or denied?
    8. What authority existed at the time?
    9. What happened afterwards?

======================================================================
7. SERVICE LAYER
======================================================================

Create or consolidate a clean service boundary.

Preferred concepts:

    WorkflowService
    PolicyService
    EvidenceService
    AuthorityService
    AuditService
    ComplianceService
    IntegrationService
    ExecutionService

Use interfaces where they improve testability and substitution.

Do not create interfaces merely for abstraction theater.

The service layer owns:

    composition
    orchestration
    policy evaluation
    actor/tool restrictions
    workflow transitions
    evidence projections
    risk/decision projections
    audit projections
    compliance mappings
    executor dispatch

The service layer does NOT become a second security kernel.

======================================================================
8. ACTOR MODEL
======================================================================

Introduce explicit actor classification outside the kernel.

Initial actor types:

    HUMAN
    AGENT
    SYSTEM

Important distinction:

    ActorType != authenticated identity

ActorType answers:

    what kind of actor is this?

Principal identity answers:

    who is this?

Authentication answers:

    how do we know?

Current Solvent does not need to become an identity provider.

Authentication belongs at:

    HTTP/API/MCP/deployment boundary.

The service layer consumes trusted principal context from that boundary.

NEVER treat:

    actor = HUMAN

provided by an untrusted request body as proof of human identity.

======================================================================
9. TOOL / ACTION POLICY REGISTRY
======================================================================

Implement a centralized tool/action policy registry.

Conceptually:

    ToolPolicy {
        ID
        RiskClass
        AllowedActors
        RequiredStage
        RequiresHumanConfirmation
        RequiresAuthority
    }

Risk classes:

    READ_ONLY
    REVERSIBLE
    IRREVERSIBLE

Use a central registry, not scattered checks.

Policy must be enforced ON THE PATH TO THE OPERATION.

Do not rely on UI hiding buttons.

Initial direction:

    AGENT may:
        retrieve
        inspect
        submit evidence
        propose
        justify
        request action

    HUMAN may:
        review
        approve
        reject
        revoke
        cross explicitly human-only boundaries

    SYSTEM may:
        perform deterministic orchestration

Adjust only where actual product requirements justify it.

======================================================================
10. WORKFLOW SERVICE
======================================================================

Create a small workflow abstraction ABOVE the kernel.

Do not replace:

    belief lifecycle
    authority lifecycle
    action_intent lifecycle

with workflow state.

Workflow is process state.

Kernel state remains authority truth.

Initial workflow states may be:

    INVESTIGATING
    EVIDENCE_REVIEW
    HUMAN_REVIEW
    APPROVED
    AUTHORIZATION_READY
    EXECUTION
    COMPLETED
    REJECTED
    CANCELLED

Use a centralized transition table.

Conceptually:

    CanTransition(request) -> TransitionResult

Enforce:

    actor restrictions
    stage requirements
    policy
    current authoritative state

Do not create unnecessary workflow states.

======================================================================
11. CRITICAL RULE — TOKEN != AUTHORITY
======================================================================

THIS IS NON-NEGOTIABLE.

A workflow token is workflow continuity only.

It is NOT authority.

DO NOT place authoritative state such as:

    AuthorityRef
    SnapshotID
    approval state

into the workflow token as a source of truth.

Workflow token should contain only continuity information such as:

    Version
    ScenarioID
    WorkflowID
    SubjectID
    Stage
    IssuedAt
    ExpiresAt

Do NOT put:

    Authority
    current debt
    evidence state
    approval status

into the token as authoritative state.

If the UI needs to display such information:

    read current state from Solvent.

If an action needs authorization:

    re-read/revalidate current Solvent state.

A valid HMAC proves:

    token integrity

It does NOT prove:

    current authority validity.

A previously valid authority may have been revoked.

Therefore:

    token
      ↓
    identify workflow
      ↓
    read current state
      ↓
    evaluate policy
      ↓
    verify current authority
      ↓
    execute

Never:

    token
      ↓
    authority
      ↓
    execute

======================================================================
12. TYPED TOKEN TRANSITIONS
======================================================================

NEVER implement:

    Refresh(token, map[string]interface{})

or any arbitrary field-overwrite API.

Use typed operations such as:

    AdvanceStage(...)
    AttachEvidence(...)
    CompleteReview(...)

Each transition must:

    validate preconditions
    enforce allowed actor
    evaluate policy where relevant
    create a newly sealed token

The token service must not become a generic signed mutable-state store.

If a workflow operation changes authoritative state, that state change
must happen through the appropriate service/kernel path.

======================================================================
13. PREPARATION / REVALIDATION BOUNDARY
======================================================================

Create a preparation boundary for consequential operations.

Conceptually:

    PrepareForAction(...)

It should:

    1. read current belief
    2. evaluate current evidence projection
    3. read current debt
    4. evaluate contradictions
    5. read current authority
    6. evaluate policy
    7. verify actor/action/target relationship
    8. return a preparation result

Most importantly:

    PREPARATION MUST RE-READ CURRENT AUTHORITATIVE STATE.

Do not trust:

    browser state
    workflow token contents
    agent-provided approval
    cached authority
    stale API state

======================================================================
14. EVIDENCE QUALITY / DECISION SUPPORT
======================================================================

Implement evidence quality as a service/read-model projection.

Useful statuses may include:

    VERIFIED
    UNVERIFIED
    CONFLICT
    STALE
    MISSING

Only derive these where underlying evidence supports them.

Do not fabricate confidence.

Provide:

    provenance
    debt
    contradictions
    freshness where available
    decision findings

Add a transparent risk/confidence score only at the product layer.

The score is advisory.

It can NEVER create or substitute for authority.

Example:

    confidence = 98
    authority = absent

must result in:

    DENIED

And:

    confidence = 42
    authority = valid

may still result in:

    AUTHORIZED

subject to policy.

======================================================================
15. AUDIT / ACTIVITY LEDGER
======================================================================

Create an append-only product audit/activity layer.

Each event should be able to capture:

    EventID
    ScenarioID
    Timestamp
    Actor
    ActorType
    Operation
    Service
    BeliefID
    IntentID
    Target
    Result
    AuthorityRef
    EvidenceRef
    ExternalRef
    Metadata

Keep authority events distinct from external execution events.

Example:

    Solvent AUTHORIZED deployment
    GitHub adapter CALLED API
    GitHub returned 200
    executor REPORTS success

These are different facts.

Never represent:

    authorization succeeded

as equivalent to:

    execution succeeded.

======================================================================
16. DATABASE CHANGES
======================================================================

Treat authority-core schema changes with a very high bar.

Prefer:

    service projections
    read models
    product tables
    configuration
    policy data

before adding authority-core tables.

There should be NO default workflow migration.

Do NOT create:

    007_workflow.sql

unless implementation proves persistent workflow state is genuinely
required.

If a workflow/activity table is needed, clearly classify it as:

    product-layer persistence

and keep it separate from authority-core tables.

Likewise, an Activity Ledger table is a product audit table, not an
authority-core table.

Do not duplicate kernel truth.

======================================================================
17. EXECUTOR BOUNDARY
======================================================================

There must be one clear executor abstraction.

Avoid defining two unrelated Executor interfaces.

Choose a single canonical executor contract or explicitly define:

    service-level executor dispatcher

versus:

    provider adapter executor port

If both are needed, document their exact relationship.

Preferred conceptual split:

    ExecutionService
        ↓
    Executor port
        ↓
    provider-specific executor adapter

Executor receives a current validated authorization decision.

Executor does NOT:

    approve
    promote
    revoke
    create authority
    create evidence authority

The executor only executes an already authorized action.

======================================================================
18. REAL INTEGRATION
======================================================================

Do not build seven shallow integrations.

Build ONE excellent commercial integration first.

Preferred initial use case:

    GitHub / CI/CD / deployment workflow

because it demonstrates:

    agent
    evidence
    human review
    authority
    exact target
    external execution
    audit
    compliance evidence

Additional integrations can later include:

    Sentry
    Datadog
    Slack
    Jira
    AWS
    Kubernetes
    A2A
    REST

All belong outside the kernel.

======================================================================
19. ADAPTER FRAMEWORK
======================================================================

Create clean ports/adapters.

Examples:

    EvidenceSource
    Executor
    DocumentGenerator
    SignatureProvider
    ActivityLedger

Adapters may know external semantics.

Kernel may not.

Every adapter should support clear:

    success
    provider failure
    malformed response
    timeout
    degraded/demo mode where appropriate

Use an adapter registry only where it simplifies dependency wiring.

Do not build a plugin framework for its own sake.

======================================================================
20. DOCUMENT / SIGNATURE SCOPE
======================================================================

Document and signature adapters are OPTIONAL/FUTURE.

Do not implement complete document/signature infrastructure in this MVP.

If only documenting extension boundaries:

    define the port conceptually

but clearly mark it:

    FUTURE / NOT MVP

If eventually implemented:

    generation != send
    draft != send
    external provider result != Solvent authority

A signature/send action must pass the same authorization boundary as
other consequential actions.

======================================================================
21. COMPLIANCE / GOVERNANCE LAYER
======================================================================

Compliance is part of the commercial architecture.

BUT:

    compliance semantics do NOT belong in the kernel.

Create a product/service-level ComplianceService.

The compliance layer consumes Solvent's authoritative facts.

It does not create authority.

Core capability:

    CONTROL
       ↓
    POLICY
       ↓
    REQUIRED EVIDENCE
       ↓
    SOLVENT DECISION
       ↓
    AUTHORITY
       ↓
    ACTION
       ↓
    EXECUTION
       ↓
    AUDIT

Initial compliance capabilities:

    1. control-to-evidence mapping
    2. evidence package generation
    3. immutable audit export
    4. exception management

Compliance exports should be derived from authoritative records.

Formats may include:

    JSON
    CSV
    PDF

Do not market the product as:

    "Solvent makes you compliant."

The correct positioning is:

    "Solvent produces authoritative evidence of governed AI-driven
     decisions and actions."

======================================================================
22. COMPLIANCE CONTROL MODEL
======================================================================

Support generic controls, not dozens of standards initially.

A control may reference:

    control ID
    description
    required policy
    evidence requirements
    responsible role
    current status
    exceptions
    last evaluation
    evidence references

Framework mappings such as:

    SOC 2
    ISO 27001
    NIST
    internal controls

should initially live in configuration/product data.

Do NOT encode framework semantics inside the kernel.

======================================================================
23. EXCEPTION MANAGEMENT
======================================================================

Exceptions are governed decisions.

An exception is NOT a bypass.

Support conceptual states such as:

    OPEN
    APPROVED
    REJECTED
    EXPIRED
    REQUIRES_REVIEW

An exception must:

    identify affected control
    identify scope
    identify approver
    have explicit validity
    be auditable

Never let:

    "exception=true"

bypass a hard kernel invariant.

======================================================================
24. WEB PRODUCT
======================================================================

Build a small operational console, not a giant SaaS platform.

Primary views:

    1. OVERVIEW
       workflows
       pending reviews
       blocked actions
       recent decisions

    2. REVIEW QUEUE
       belief
       evidence
       debt
       contradictions
       requested action
       target
       actor

    3. AUTHORITY / DECISION DETAIL
       evidence
       provenance
       policy
       actor
       authority
       exact target
       reason for allow/deny

    4. AUDIT / ACTIVITY
       append-only timeline
       filters
       authority events
       external events

    5. INTEGRATIONS
       providers
       mode
       health
       recent calls
       failures

Optional:

    6. COMPLIANCE
       controls
       evidence
       exceptions
       export

The UI must expose WHY.

Examples:

    DENIED — belief not promoted
    DENIED — required obligation remains open
    DENIED — actor not permitted
    DENIED — target mismatch
    DENIED — authority revoked
    ALLOWED — current authority matches exact target/action

======================================================================
25. WEB UI MUST NOT BECOME AUTHORIZATION ENGINE
======================================================================

Buttons are not security.

A UI action such as:

    Approve
    Execute
    Revoke

must call the service layer.

The service layer must revalidate.

The kernel remains authoritative.

Do not trust:

    hidden buttons
    disabled buttons
    browser state
    frontend workflow state
    cached approval

as security controls.

======================================================================
26. SCENARIO / DEMO FRAMEWORK
======================================================================

Create a scenario framework.

Every scenario must use the SAME service/API path.

Do not write five custom mini-security systems.

Required scenarios:

    A. Agentjacking
    B. Lying Agent
    C. Stale Authorization
    D. Confused Deputy
    E. Legitimate Workflow

Every scenario must be:

    deterministic
    resettable
    independently seeded
    independently testable

Reset only scenario-owned data.

NEVER drop the entire database as a scenario reset.

======================================================================
27. DEMO A — AGENTJACKING
======================================================================

Show:

    poisoned external evidence
        ↓
    evidence enters Solvent
        ↓
    belief has unresolved debt
        ↓
    agent cannot promote
        ↓
    human reviews
        ↓
    poisoning identified
        ↓
    belief retracted
        ↓
    audit trail records result

Also preserve the existing host-level restriction:

    untrusted telemetry agents must not be given direct mutation tools
    that would let them manually bypass the intended workflow.

This is a deployment/control-surface concern, not a reason to expand
the kernel.

======================================================================
28. DEMO B — LYING AGENT
======================================================================

Scenario:

    agent claims approval exists
        ↓
    no evidence / authority
        ↓
    attempt to proceed
        ↓
    DENIED

Then:

    human reviews
        ↓
    legitimate approval
        ↓
    current authority exists
        ↓
    retry
        ↓
    ALLOWED

Core message:

    Agent claims are not authority.

======================================================================
29. DEMO C — STALE AUTHORIZATION
======================================================================

Scenario:

    authority legitimately exists
        ↓
    authority revoked/invalidated
        ↓
    agent attempts action
        ↓
    DENIED

Then:

    human creates fresh valid authority
        ↓
    retry
        ↓
    ALLOWED

The demo must prove:

    current authoritative state wins

over:

    previously valid workflow state.

======================================================================
30. DEMO D — CONFUSED DEPUTY
======================================================================

Scenario:

    authority for Target A
        ↓
    action requested for Target B
        ↓
    DENIED

Important:

    NEVER mutate the original immutable authority.

The correct sequence is:

    wrong-target request
        ↓
    DENIED

then:

    new requested target/action tuple
        ↓
    new authority workflow
        ↓
    human approval
        ↓
    new authority
        ↓
    execution

Do NOT implement "target correction" on an existing authority.

======================================================================
31. DEMO E — LEGITIMATE WORKFLOW
======================================================================

Show the complete happy path:

    agent submits evidence
        ↓
    evidence reviewed
        ↓
    debt retired
        ↓
    belief promoted
        ↓
    human approves authority
        ↓
    exact authorization verified
        ↓
    executor runs
        ↓
    execution result recorded
        ↓
    audit/compliance evidence package available

This is the commercial proof.

======================================================================
32. OPEN-SOURCE "BREAK SOLVENT" SUITE
======================================================================

Create reproducible challenge tests.

Attack cases:

    poisoned evidence
    fabricated approval
    agent hallucination
    wrong actor
    wrong target
    stale authority
    revoked authority
    malformed provider output
    invalid workflow token
    unauthorized execution attempt

Each challenge should contain:

    attack
    expected decision
    reason
    test

The repository should invite engineers to try to defeat the authority
model.

======================================================================
33. SECURITY TESTING
======================================================================

Test at four levels:

    KERNEL
    SERVICE
    ADAPTER
    END-TO-END

Security tests MUST include bypass attempts.

Required:

    agent cannot invoke human-only operation
    agent cannot approve itself
    fake approval denied
    no authority denied
    open debt denied
    wrong target denied
    revoked authority denied
    stale state denied
    stale workflow token rejected where applicable
    manipulated token rejected
    browser state cannot create authority
    executor cannot create authority
    external provider cannot create authority

======================================================================
34. TOKEN SECURITY TESTS
======================================================================

Explicitly test:

    tampered token rejected
    invalid signature rejected
    expired token rejected where configured
    invalid stage transition rejected
    token cannot carry authority as truth
    token cannot bypass current revocation
    token cannot bypass current target binding
    token cannot bypass current policy

Most importantly:

    valid signed stale token
        +
    revoked current authority
        =
    DENIED

======================================================================
35. COMPLIANCE TESTING
======================================================================

Test:

    control-to-evidence mapping
    audit export completeness
    authority traceability
    actor traceability
    approval traceability
    exception lifecycle
    revoked authority appears in audit evidence
    execution failure remains distinct from authorization success

Do not test only UI rendering.

Test the underlying records and service outputs.

======================================================================
36. FORMAL VERIFICATION
======================================================================

Do NOT enlarge the Lean model merely because the product expands.

Lean should continue to represent the security-critical kernel state
machine.

Only modify Lean when kernel semantics change.

Prefer:

    small formal model
    broad service/product layer

rather than:

    giant formal model
    giant kernel

======================================================================
37. OBSERVABILITY
======================================================================

Make security and operational facts visible.

Track:

    authorization allowed
    authorization denied
    human approval
    revocation
    stale authority attempts
    policy denial
    adapter failure
    executor failure

Maintain the distinction:

    authorization succeeded
    execution failed

and:

    authorization denied
    execution never happened

These are different operational facts.

======================================================================
38. COMMERCIAL UX
======================================================================

The product must look like an operational system, not a research demo.

A buyer should be able to see:

    current workflow
    pending decision
    evidence
    authority
    action
    target
    actor
    policy
    result
    audit trail
    compliance evidence

A technical demo should be capable of explaining the same event at
three levels:

    EXECUTIVE:
        "Blocked because no authorized approval exists."

    SECURITY:
        "Agent principal lacked a valid authority tuple for target X."

    ENGINEERING:
        "Current kernel authorization check returned DENIED."

======================================================================
39. COMMERCIAL POSITIONING
======================================================================

Do not turn Solvent into a generic:

    AI firewall
    GRC platform
    IAM replacement
    policy engine
    workflow SaaS
    agent framework

The product should demonstrate:

    governed autonomy

through:

    authoritative evidence
    explicit human decisions
    machine-enforced authority
    exact action/target binding
    external execution boundaries
    compliance evidence

The key commercial message:

    "Solvent provides the authority layer between autonomous systems
     and consequential actions."

======================================================================
40. WHAT NOT TO BUILD NOW
======================================================================

Explicitly defer:

    full kernel multi-tenancy
    generalized policy language
    universal workflow DSL
    dozens of integrations
    custom identity provider
    deep IAM platform
    cryptographic attestation infrastructure
    global replay-prevention machinery
    execution-effect verification framework
    complex distributed scheduler
    elaborate document/signature platform
    large analytics platform
    generic AI risk-scoring laboratory

Also defer:

    complicated compliance-framework automation

until actual customer requirements justify it.

======================================================================
41. IMPLEMENTATION ORDER
======================================================================

Implement in this order:

PHASE 0
    repository reconnaissance
    baseline tests
    architecture confirmation

PHASE 1 — SERVICE BOUNDARIES
    workflow service
    actor/tool policy registry
    evidence projection
    audit/activity service
    authority service

PHASE 2 — SECURITY BOUNDARIES
    preparation/revalidation boundary
    human authorization boundary
    typed workflow token
    current-authority revalidation
    executor contract

PHASE 3 — ONE REAL INTEGRATION
    GitHub/CI/CD adapter
    executor
    activity recording

PHASE 4 — PRODUCT UI
    overview
    review queue
    authority detail
    audit
    integrations
    compliance evidence view

PHASE 5 — COMPLIANCE
    control mapping
    evidence package
    audit export
    exceptions

PHASE 6 — DEMO PLATFORM
    Agentjacking
    Lying Agent
    Stale Authorization
    Confused Deputy
    Legitimate Workflow

PHASE 7 — OPEN SOURCE
    challenge suite
    adapter examples
    architecture docs
    security docs
    quickstart
    developer guide

Do NOT build large enterprise functionality before the above is coherent.

======================================================================
42. ACCEPTANCE CRITERIA
======================================================================

ARCHITECTURE

[ ] Kernel remains small and domain-generic.
[ ] Existing kernel semantics are preserved.
[ ] MCP remains thin.
[ ] External systems are adapters.
[ ] Execution remains outside kernel.
[ ] Service layer owns orchestration/policy.
[ ] Compliance remains outside kernel.

SECURITY

[ ] Actor classification exists.
[ ] Authentication is separate from actor type.
[ ] Human-only operations are mechanically gated.
[ ] Agent claims cannot become authority.
[ ] Workflow tokens cannot become authority.
[ ] Signed stale tokens cannot bypass current authority state.
[ ] Wrong targets are denied.
[ ] Revoked authority is denied.
[ ] Browser state cannot become authority.
[ ] Executors cannot create authority.

PRODUCT

[ ] Operator can inspect pending decisions.
[ ] Operator can inspect evidence.
[ ] Operator can understand why action is blocked.
[ ] Operator can review/approve legitimate work.
[ ] Operator can inspect exact authority.
[ ] Operator can inspect audit trail.
[ ] Operator can inspect execution outcome.
[ ] Compliance evidence can be exported.

COMPLIANCE

[ ] Controls can map to evidence.
[ ] Evidence traces to decisions.
[ ] Decisions trace to authority.
[ ] Authority traces to action/target.
[ ] Actions trace to external execution.
[ ] Exceptions are explicit and auditable.
[ ] Exceptions cannot bypass hard kernel invariants.

DEMOS

[ ] Agentjacking works.
[ ] Lying Agent works.
[ ] Stale Authorization works.
[ ] Confused Deputy works.
[ ] Legitimate Workflow works.
[ ] All scenarios reset independently.

OPEN SOURCE

[ ] New adapter does not require kernel modification.
[ ] New executor does not require kernel modification.
[ ] New customer policy does not require kernel modification.
[ ] New demo scenario uses common services.
[ ] Security challenges are reproducible.

QUALITY

[ ] Existing tests pass.
[ ] Service tests pass.
[ ] Security bypass tests pass.
[ ] End-to-end tests pass.
[ ] Lean verification remains clean.
[ ] No unexplained kernel growth.
[ ] No unexplained authority-core schema growth.
[ ] Documentation reflects actual implementation.

======================================================================
43. DELIVERABLES
======================================================================

Deliver:

    1. implementation
    2. architecture documentation
    3. kernel/schema ADRs where necessary
    4. commercial MVP README
    5. security model
    6. workflow documentation
    7. policy/tool registry documentation
    8. adapter development guide
    9. executor guide
    10. compliance/evidence guide
    11. demo walkthrough
    12. challenge-suite documentation
    13. test report
    14. buyer-oriented "What is Solvent?"
    15. security-engineer-oriented "Why is Solvent different?"
    16. final implementation summary

Final implementation summary MUST include:

    files added
    files changed
    migrations added
    kernel changes
    reason for every kernel change
    service additions
    adapter additions
    executor additions
    UI additions
    compliance additions
    demo additions
    test coverage
    remaining limitations

======================================================================
44. FINAL DESIGN PRINCIPLES
======================================================================

1. SMALL TRUSTED CORE

Keep the kernel minimal.

2. CURRENT AUTHORITY WINS

Never let cached or signed workflow state substitute for current
authority verification.

3. AUTHORIZE != EXECUTE

The authorization layer and execution layer are separate.

4. ACTOR != IDENTITY

Actor classification does not authenticate callers.

5. EVIDENCE != AUTHORITY

Evidence can support authority; evidence does not become authority.

6. POLICY != KERNEL

Customer policy belongs outside the kernel.

7. WORKFLOW != AUTHORITY

Workflow coordinates the process; Solvent establishes authority.

8. COMPLIANCE != KERNEL

Compliance consumes Solvent facts; it does not redefine them.

9. ADAPTERS KNOW EXTERNAL SYSTEMS

The kernel does not.

10. ONE SOURCE OF AUTHORITY TRUTH

Do not duplicate kernel authority in tokens, browser state,
workflow objects, or external providers.

11. MAKE SECURITY VISIBLE

The product must explain WHY an action was allowed or denied.

12. BUILD ONE EXCELLENT REAL WORKFLOW

Prefer one production-quality integration over many superficial ones.

13. DEMOS MUST SHARE THE SAME CORE

Different attacks, same Solvent semantics.

14. OPEN SOURCE SHOULD BE EXTENSIBLE

A developer should be able to add an adapter, executor, policy, or
scenario without touching the kernel.

15. COMPLEXITY MUST BE EARNED

Only real customer demand, operational pain, observed security failure,
or a clear security requirement justifies expanding the trusted core.

======================================================================
FINAL OBJECTIVE
======================================================================

The finished system should create this impression:

    TO A BUYER:

        "This governs what our AI systems are actually allowed to do."

    TO A SECURITY ENGINEER:

        "The critical authority decision is backed by a small,
         transactionally enforced core rather than LLM behavior."

    TO A DEVELOPER:

        "I can integrate my system without modifying that core."

    TO A COMPLIANCE TEAM:

        "I can trace controls and evidence to actual governed actions."

    TO AN OPEN-SOURCE CONTRIBUTOR:

        "I can extend Solvent without turning every integration
         into another security mechanism."

Do not optimize for maximum code.

Optimize for:

    small trusted core
    clear boundaries
    current-state authority verification
    commercially useful workflow
    visible security guarantees
    compliance traceability
    one real integration
    excellent demonstrations
    easy extension
    maintainability
    open-source credibility

Build the product around the kernel.

Do not turn the product into the kernel.
