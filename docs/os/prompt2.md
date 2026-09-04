I read all four attached materials. The consistent conclusion is that **Solvent should not try to become AegisFlow or DealForge internally**. Those systems are useful reference implementations for the *product shell*: workflow visibility, actor/tool gating, auditability, decision support, integrations, human approval, and operational UX. Solvent's advantage remains its transactional authority kernel, database-enforced invariants, formal verification, and MCP surface. 

The most important design constraint is therefore:

> **Build the minimum viable commercial product around the kernel, not by turning the kernel into the product.**

The prompt below is the one I would give the coding agent.

---

# Coding-Agent Prompt — Solvent Commercial MVP

```text
You are the senior staff/principal engineer responsible for extending the Pithom Labs Solvent repository from a technically strong v0 kernel/demo into a minimum viable commercial product.

Your primary objective is NOT to maximize feature count.

Your objective is:

    Make Solvent commercially understandable, operationally usable,
    demonstrably secure, and extensible without materially enlarging
    the trusted kernel.

The central product thesis is:

    SOLVENT
    A portable authority layer for autonomous systems.

The core distinction is:

    Evidence is not authority.
    Agent output is not authority.
    Retrieval is not authority.
    An action is allowed only when Solvent can establish the required authority.

The existing Solvent kernel is the differentiator and must remain small.

======================================================================
CATEGORY 0 — NON-NEGOTIABLE ARCHITECTURAL PRINCIPLE
======================================================================

Treat the current kernel as a small trusted authority core.

The design rule is:

    Keep the kernel small, deterministic, transactionally authoritative,
    and domain-generic.

Push everything else outward.

Use this decision tree for EVERY proposed feature:

    External product/protocol?
        -> Adapter

    Workflow / orchestration / policy / composition?
        -> Service / Policy layer

    Execution / infrastructure / provider interaction?
        -> Executor / Deployment adapter

    Reporting / UI / analytics / scoring / operational visibility?
        -> Product layer / read model / service

    Customer-specific rules?
        -> Policy/configuration/data

    Does the feature require a new security-critical durable fact
    or atomic state transition that cannot be expressed correctly
    with existing primitives?
        -> Only then consider kernel changes

    Does the system need to make a previously possible invalid state
    structurally impossible?
        -> Consider a DB invariant/schema change

    Otherwise:
        -> DO NOT ADD IT TO THE KERNEL

Do not introduce domain-specific concepts such as:

    Sentry
    GitHub
    MCP-specific semantics
    procurement
    incident response
    e-signature
    Kubernetes
    Cloudflare
    Salesforce
    vendor-specific risk models

into the generic authority kernel.

Agentjacking is an adapter/use-case, not a kernel concept.

======================================================================
CATEGORY 1 — FIRST: RECONNAISSANCE, NOT CODING
======================================================================

Before modifying anything:

1. Inspect the entire repository.

2. Establish the actual current architecture rather than relying on
   documentation assumptions.

3. Identify and document:

    - kernel packages
    - service layer
    - MCP layer
    - HTTP/API layer
    - UI/demo implementation
    - DB schema and migrations
    - authority lifecycle
    - belief lifecycle
    - action_intent lifecycle
    - current actor/principal handling
    - authorization path
    - audit implementation
    - adapter implementations
    - Lean model
    - tests
    - task commands
    - deployment configuration

4. Run the existing test/verification gates BEFORE changing code.

5. Produce a short internal architecture map showing:

       external systems
            |
         adapters
            |
       service/policy
            |
        SOLVENT KERNEL
            |
          DB

   and separately:

       Solvent authorization
            |
       executor/deployment adapter
            |
       real-world execution

6. Identify which capabilities already exist and MUST NOT be
   reimplemented.

7. Do not blindly follow the attached competitive analysis if the
   actual repository differs from it. The repository is the source of
   truth for implementation decisions.

======================================================================
CATEGORY 2 — DEFINE THE COMMERCIAL MVP
======================================================================

Do NOT attempt to reproduce the full breadth of AegisFlow.

The MVP should prove one coherent commercial workflow:

    UNTRUSTED / AI-GENERATED INFORMATION
                    |
                    v
              Solvent review
                    |
                    v
          evidence + obligations
                    |
                    v
             human decision
                    |
                    v
          explicit authority
                    |
                    v
          exact action authorization
                    |
                    v
           external execution
                    |
                    v
             audit ledger

The MVP should make these things visible:

    1. What does the agent believe?
    2. What evidence supports it?
    3. What remains unresolved?
    4. Who is allowed to approve it?
    5. Which action is being requested?
    6. Which target does that action affect?
    7. Why was the action allowed or denied?
    8. What authority existed at the time?
    9. What happened afterwards?

The MVP is successful when a non-technical security/operator buyer
can understand Solvent in approximately five minutes.

======================================================================
CATEGORY 3 — PRESERVE THE EXISTING KERNEL
======================================================================

Assume the existing v0 authority lifecycle is the canonical foundation.

Preserve the semantics already established for:

    principal
    authority_target
    target_snapshot
    target_activation
    target_revocation
    justification
    debt_discharge

Preserve the current authority concepts:

    Approve
    Authorize
    RevokeTarget

Preserve:

    target snapshot semantics
    activation uniqueness
    debt/promotion gates
    transactional authority creation
    read-only authorization verification
    DB-enforced invariants
    formal verification

Do NOT redesign these concepts merely to make the UI easier.

Instead, build services and projections around them.

Any proposed kernel change must first include:

    - why current primitives are insufficient
    - why service/policy cannot express the behavior
    - why an adapter cannot express it
    - why a DB/read model cannot express it
    - what new durable security fact is required
    - what atomic transition is required
    - what new invariant is required
    - how Lean verification is affected

If that case cannot be made convincingly, reject the kernel change.

======================================================================
CATEGORY 4 — ACTOR AND TOOL AUTHORIZATION LAYER
======================================================================

One of the most important commercial gaps is explicit actor/tool
governance.

Implement this OUTSIDE the kernel first.

Create a generic policy/service concept equivalent to:

    Actor:
        HUMAN
        AGENT
        SYSTEM

    Risk:
        READ_ONLY
        REVERSIBLE
        IRREVERSIBLE

    Capability:
        allowed actors
        required workflow stage
        required authority state
        optional confirmation requirement

Create a central registry for tool/action policy.

Conceptually:

    ToolPolicy {
        id
        risk_class
        allowed_actors
        required_stage
        requires_human_confirmation
        required_authority
    }

The registry must be the single place where MCP/API actions are
classified.

Do not scatter actor checks across individual handlers.

Enforcement must occur on the path to the consequential operation,
not merely in UI code.

Initial policy should establish a clear separation such as:

    AGENT:
        may retrieve
        may submit evidence
        may propose
        may justify
        may request an action

    HUMAN:
        may review
        may approve
        may revoke
        may perform explicitly human-only transitions

    SYSTEM:
        may perform deterministic orchestration

Important:

Do not claim that this authenticates callers.

Current Solvent does not establish caller authentication at the kernel
level. Keep authentication/identity at the deployment/API boundary.

The service layer should consume a normalized authenticated principal
context supplied by the deployment boundary.

======================================================================
CATEGORY 5 — WORKFLOW SERVICE
======================================================================

Add an explicit workflow abstraction ABOVE the kernel.

Do not replace the kernel lifecycle.

The workflow should be a product/service construct that composes
existing kernel primitives.

Use a small, understandable state machine.

Start with something like:

    INVESTIGATING
        ->
    EVIDENCE_REVIEW
        ->
    HUMAN_REVIEW
        ->
    APPROVED
        ->
    AUTHORIZATION_READY
        ->
    EXECUTION
        ->
    COMPLETED

And a rejection/cancellation path where appropriate.

Do not create dozens of states.

The workflow service should answer:

    What stage are we in?
    What can the agent do here?
    What can the human do here?
    What evidence is required?
    What transition is allowed?
    What transition is forbidden?

Use a centralized transition table/function.

For example:

    CanTransition(from, to, actor, context)

The workflow must call the existing Solvent primitives.

It must NOT become a second authority engine.

The kernel remains the final authority on transactional correctness.

======================================================================
CATEGORY 6 — HUMAN AUTHORIZATION BOUNDARY
======================================================================

Introduce a deliberate human authorization boundary for irreversible
operations.

Use the strongest useful lesson from the DealForge design:

    preparation != authorization
    generation != sending
    draft != execution

For Solvent:

    proposed
        !=
    approved
        !=
    authorized
        !=
    executed

Make this distinction visible in APIs and UI.

For consequential operations, support:

    explicit human action
    optional explicit confirmation
    current authority re-validation immediately before execution

Never trust a browser object, stale workflow state, or agent-provided
"approved=true" field as execution authority.

Any workflow token, session state, or UI state is workflow continuity,
NOT the authority source.

The authority source remains Solvent.

======================================================================
CATEGORY 7 — POLICY ENGINE / CONFIGURATION
======================================================================

Add configurable policy ABOVE the kernel.

Policy should be data-driven where practical.

Examples:

    deployment policy
    approval thresholds
    required review obligations
    actor restrictions
    reversible/irreversible classification
    target restrictions
    escalation requirements

Keep a distinction between:

    Solvent minimum safety floor

and:

    customer policy

Customer policy should be able to become stricter without modifying
the kernel.

Do not introduce a fully generic policy language unless actual use
cases require it.

Start with typed configuration and deterministic evaluation.

======================================================================
CATEGORY 8 — EVIDENCE AND DECISION LAYER
======================================================================

Do not modify the kernel just to implement richer presentation of
evidence quality.

Build a service/read model that derives:

    evidence completeness
    unresolved debt
    contradictions
    provenance
    freshness/staleness where available
    review status
    decision readiness

Use computed statuses such as:

    VERIFIED
    UNVERIFIED
    CONFLICT
    STALE
    MISSING

only when the underlying evidence actually supports those conclusions.

Do not fabricate confidence.

Add a transparent decision/risk score only as a PRODUCT-layer score.

The score must never override a hard Solvent authority invariant.

For example:

    score = 96
    but unresolved hard security obligation exists

must still result in:

    NOT AUTHORIZED

The scoring layer is advisory.

The authority kernel remains decisive.

Implement an initial small scoring model rather than a massive
enterprise risk platform.

Every score component should point to its supporting evidence.

======================================================================
CATEGORY 9 — AUDIT / ACTIVITY LEDGER
======================================================================

Create a product-facing append-only activity/audit view.

The user must be able to see:

    timestamp
    actor
    actor type
    workflow stage
    operation
    target
    result
    authority reference
    evidence reference
    external adapter
    execution outcome

Separate:

    AUTHORITY EVENTS

from:

    EXTERNAL ACTIVITY EVENTS

The distinction is important.

For example:

    Solvent AUTHORIZED deploy
    GitHub adapter CALLED
    GitHub returned 200
    executor REPORTS success

These are different facts.

Do not pretend that authorization proves execution.

Do not pretend that provider success proves Solvent authority.

Maintain the boundary:

    AUTHORIZE != EXECUTE

======================================================================
CATEGORY 10 — WEB PRODUCT SURFACE
======================================================================

Replace the "demo wizard" mentality with a minimal operational console.

Do NOT build 20 pages.

Build approximately five coherent views:

    1. OVERVIEW
       current workflows
       pending reviews
       blocked actions
       recent decisions

    2. REVIEW QUEUE
       beliefs / decisions requiring human attention
       evidence
       open debt
       contradictions
       requested action
       target

    3. AUTHORITY / DECISION DETAIL
       evidence
       provenance
       debt
       policy
       actor
       authority status
       exact target
       reason for allow/deny

    4. AUDIT / ACTIVITY
       immutable chronological history
       filters by actor, target, action, result

    5. INTEGRATIONS
       adapter health
       recent calls
       LIVE / LOCAL / DEMO mode
       latency/status/failure

A sixth SETTINGS view is acceptable only if necessary.

The UI must expose the kernel's guarantees rather than hide them.

The user should be able to SEE:

    "Denied because belief is not promoted."

    "Denied because required obligation remains open."

    "Denied because actor is not permitted."

    "Denied because target does not match authority."

    "Allowed because exact authority exists."

This is the productization of the kernel.

======================================================================
CATEGORY 11 — API / SERVICE BOUNDARY
======================================================================

Define stable application-facing services.

Prefer interfaces along these boundaries:

    AuthorityService
    WorkflowService
    PolicyService
    EvidenceService
    AuditService
    IntegrationService
    ExecutionService

Do not expose raw kernel/database operations throughout the UI.

The service layer should translate product concepts into existing
kernel primitives.

Keep MCP thin.

Keep HTTP/API thin.

Avoid a situation where:

    MCP
    UI
    REST
    CLI

each implement their own authorization logic.

All should converge on the same service/policy boundary.

======================================================================
CATEGORY 12 — ADAPTER FRAMEWORK
======================================================================

Create a clean adapter contract.

An adapter should translate:

    external system
        ->
    generic Solvent input/output

Adapters may understand:

    GitHub
    Sentry
    Datadog
    Slack
    Jira
    cloud providers
    MCP
    A2A
    REST

The core should not.

Every integration should have:

    provider name
    operation
    request metadata
    response metadata
    timing
    success/failure
    mode

Support:

    LIVE
    LOCAL
    DEMO

where meaningful.

The first commercial adapter should be deliberately chosen for a
high-value workflow.

Prefer a workflow that clearly demonstrates:

    agent
    decision
    human approval
    authorized action
    external execution
    audit

A GitHub deployment/change workflow is a strong candidate.

Do not build seven integrations just to match AegisFlow.

One excellent integration is better than seven shallow ones.

======================================================================
CATEGORY 13 — EXECUTION BOUNDARY
======================================================================

Create a formal executor interface outside the kernel.

Conceptually:

    AuthorizationDecision
        ->
    Executor.Execute(...)

Example executors:

    GitHubActionsExecutor
    KubernetesExecutor
    GenericHTTPExecutor
    MockExecutor

The executor receives an already validated authorization decision.

The executor MUST NOT invent authority.

The executor MUST NOT promote beliefs.

The executor MUST NOT mutate Solvent authority state.

Keep:

    authorization

and

    execution

as different contracts.

Where practical, re-check current authority immediately before
consequential execution.

======================================================================
CATEGORY 14 — WORKFLOW CONTINUITY / TOKENS
======================================================================

Use the useful lesson from DealForge carefully.

A workflow token may carry:

    workflow ID
    workflow stage
    object identity
    trusted continuity information
    version
    expiry if appropriate

If tokens are introduced, make them tamper-evident.

However:

    token != authority

Every consequential operation must re-read/revalidate current
authoritative state.

Never let browser-controlled workflow state become an authority source.

Prefer stateless or minimally stateful workflow continuity where it
reduces complexity.

Do not introduce tokens merely because DealForge has them.

Only use them if they simplify the actual Solvent application.

======================================================================
CATEGORY 15 — MULTI-TENANCY AND AUTHENTICATION
======================================================================

Do not prematurely redesign the kernel for full multi-tenancy.

For the commercial MVP:

    authentication belongs at the deployment/application boundary.

    authorization semantics belong in service/policy.

    authority facts belong in Solvent.

For self-hosted MVP:

    support one installation / one organization cleanly.

For hosted deployment:

    introduce tenant context outside the kernel first.

Do not add deep tenant semantics to kernel tables unless a genuine
security boundary requires it.

If tenant isolation eventually requires durable kernel facts,
document that as a separate architectural decision.

======================================================================
CATEGORY 16 — DOCUMENTS AND SIGNATURES
======================================================================

Do NOT make document generation/e-signature a kernel feature.

If included in MVP, implement:

    DocumentGenerationPort
    SignaturePort

as adapters.

Borrow the DealForge principle:

    generate != send

and:

    create draft != send externally

Any send/signature operation must pass the same Solvent
human-authorization boundary.

For the initial MVP, document generation may remain a secondary
integration rather than a core requirement.

Do not spend the majority of engineering time here.

======================================================================
CATEGORY 17 — COMMERCIAL DEMO MODE
======================================================================

Build a first-class demo harness.

It should support named scenarios.

At minimum implement:

    Scenario A — Agentjacking
    Scenario B — Lying Agent
    Scenario C — Stale Authorization
    Scenario D — Confused Deputy / Wrong Target
    Scenario E — Legitimate Human Approval

Every scenario should be deterministic and resettable.

Do not destroy the whole database during reset.

Reset only scenario-owned state/data.

Add controlled failure injection where useful:

    provider unavailable
    stale evidence
    conflicting evidence
    unauthorized actor
    wrong target
    no authority
    revoked authority

Every demo must display the actual Solvent reason for the decision.

======================================================================
CATEGORY 18 — DEMO PORTFOLIO DESIGN
======================================================================

The demos are NOT separate features.

They are different tests of the same authority model.

The five primary demos should communicate:

    Agentjacking
        Evidence is not authority.

    Lying Agent
        Agent claims are not authority.

    Stale Authorization
        Authority can become invalid.

    Confused Deputy
        Authority is bound to the correct actor/action/target.

    Legitimate Workflow
        Human approval creates real authority and execution proceeds.

The product UI should make these scenarios selectable so a prospect can
see the same Solvent machinery handling different failures.

======================================================================
CATEGORY 19 — OPEN-SOURCE COMMUNITY DESIGN
======================================================================

The open-source repository should be easy to understand and challenge.

Add:

    examples/
    demos/
    adapters/
    policies/
    scenarios/
    docs/architecture/
    docs/security/
    docs/getting-started/

Provide a "Break Solvent" challenge suite.

Each challenge should attempt to obtain authorization through:

    poisoned evidence
    fabricated approval
    agent hallucination
    wrong actor
    wrong target
    stale authority
    revoked authority
    malformed tool output

Every challenge should have:

    attack
    expected denial
    expected reason
    test

This makes the project useful to security engineers, not merely
potential customers.

======================================================================
CATEGORY 20 — TESTING STRATEGY
======================================================================

Preserve all existing tests.

Add tests at four levels:

    1. Kernel tests
       Existing guarantees must remain intact.

    2. Service tests
       workflow
       policy
       actor restrictions
       tool restrictions
       scoring
       orchestration

    3. Adapter tests
       translation
       malformed provider responses
       failure handling

    4. End-to-end scenario tests
       full commercial workflows

For every security-sensitive rule, test both:

    happy path

and:

    attempted bypass

Especially test:

    agent invoking human-only operation
    stale workflow state
    manipulated token
    wrong target
    revoked authority
    missing obligation
    tool output attempting to become authority
    execution attempted without authorization

======================================================================
CATEGORY 21 — LEAN / FORMAL VERIFICATION
======================================================================

Do NOT expand Lean merely because new application features exist.

Lean should track the actual security-critical kernel state machine.

Only modify the formal model if the kernel itself changes.

If service-layer behavior can be proven through ordinary deterministic
tests and the underlying kernel invariants remain unchanged, do not
expand Lean unnecessarily.

The objective is:

    small trusted kernel
    small formal model
    broad product layer

======================================================================
CATEGORY 22 — DATABASE CHANGES
======================================================================

Treat DB changes with a higher bar than ordinary application changes.

Prefer:

    read models
    projections
    service-layer derived data
    configuration
    product tables outside the authority core

before adding new kernel tables.

A new authority-core table requires justification that a new durable
security fact is necessary.

A new CHECK/FK/UNIQUE constraint requires proof that the invariant
should become structurally impossible.

Do not add schema merely because a UI would be easier to implement.

Do not duplicate existing kernel facts in competing tables.

======================================================================
CATEGORY 23 — OBSERVABILITY
======================================================================

Make operational behavior inspectable.

Expose metrics/logs for:

    authorization denied
    authorization allowed
    human approvals
    authority revocations
    stale authorization
    adapter failures
    executor failures
    policy violations

Be explicit about the boundary:

    Solvent authorization succeeded
    does NOT mean

    external execution succeeded

That distinction must survive logging, UI, and APIs.

======================================================================
CATEGORY 24 — SECURITY MODEL DOCUMENTATION
======================================================================

Produce an explicit architecture/security document explaining:

    What Solvent guarantees
    What it does not guarantee
    Who authenticates callers
    Who supplies identity
    What the kernel trusts
    What the service layer trusts
    What adapters trust
    What executors trust
    Where human authorization happens
    Where execution happens

Keep current v0 limitations honest.

Do not claim:

    caller authentication
    cryptographic attestation
    global replay prevention
    complete execution-effect verification
    multi-tenancy isolation

unless actually implemented.

======================================================================
CATEGORY 25 — COMMERCIAL PACKAGING
======================================================================

The resulting repository should look like a real product project, not
a research prototype.

Provide:

    single-command local startup
    deterministic seed/demo setup
    clean configuration
    health endpoint
    API documentation
    security model
    architecture diagram
    quickstart
    commercial demo walkthrough
    adapter development guide

A developer should be able to:

    clone
    configure
    start
    create an authority
    review it
    authorize an action
    observe denial/approval
    inspect the audit trail

without reading the entire repository.

======================================================================
CATEGORY 26 — COMMERCIAL MVP PRIORITY ORDER
======================================================================

Implement in this order.

PHASE 1 — PRODUCT SHELL

    1. architecture cleanup
    2. service boundaries
    3. actor/tool policy registry
    4. workflow service
    5. audit/activity projection
    6. operational web UI

PHASE 2 — REAL AUTHORIZATION WORKFLOW

    7. human review queue
    8. approval boundary
    9. exact action/target presentation
    10. executor interface
    11. one real external integration

PHASE 3 — DECISION SUPPORT

    12. evidence quality projection
    13. transparent scoring
    14. conflict/staleness presentation
    15. policy configuration

PHASE 4 — DEMO PLATFORM

    16. scenario framework
    17. Agentjacking
    18. Lying Agent
    19. Stale Authorization
    20. Confused Deputy
    21. legitimate approval workflow
    22. failure injection

PHASE 5 — OPEN SOURCE POLISH

    23. developer documentation
    24. adapter SDK/examples
    25. security challenge suite
    26. architecture/security docs
    27. commercial demo walkthrough

Do NOT proceed to large enterprise features before the above is coherent.

======================================================================
CATEGORY 27 — WHAT NOT TO BUILD NOW
======================================================================

Explicitly defer:

    full multi-tenancy in kernel
    generalized policy programming language
    universal workflow DSL
    dozens of integrations
    deep IAM system
    custom identity provider
    cryptographic attestation infrastructure
    global replay-prevention machinery
    execution-effect verification framework
    complex distributed scheduler
    elaborate document/signature platform
    large analytics platform
    generic AI risk-scoring laboratory

These may become future products/features.

They are not requirements for the MVP.

======================================================================
CATEGORY 28 — KERNEL GROWTH GATE
======================================================================

Before making ANY kernel/schema change, write an ADR containing:

    Problem
    Why product/service layer cannot solve it
    Why adapter cannot solve it
    Why policy/configuration cannot solve it
    New durable security fact
    New atomic transition
    Required DB invariant
    Impact on existing authority semantics
    Impact on Lean model
    Migration strategy
    Backward compatibility
    Security argument

If the ADR cannot establish a compelling need:

    DO NOT MODIFY THE KERNEL.

======================================================================
CATEGORY 29 — ACCEPTANCE CRITERIA
======================================================================

The work is complete only when all of the following are true:

ARCHITECTURE

    [ ] Kernel remains small and domain-generic.
    [ ] Product behavior lives outside the kernel.
    [ ] MCP remains thin.
    [ ] External systems are adapters.
    [ ] Execution is outside the kernel.
    [ ] Service layer owns orchestration/policy.

SECURITY

    [ ] Actor/tool restrictions are explicit.
    [ ] Human-only actions are mechanically gated.
    [ ] Agent claims cannot become authority.
    [ ] Wrong target cannot use correct authority.
    [ ] Revoked/stale authority cannot authorize.
    [ ] Browser/workflow state cannot become authority.
    [ ] External execution cannot create authority.

PRODUCT

    [ ] Operator can view pending decisions.
    [ ] Operator can inspect evidence.
    [ ] Operator can see why something is blocked.
    [ ] Operator can approve legitimate actions.
    [ ] Operator can inspect authority.
    [ ] Operator can inspect audit history.
    [ ] Operator can see external execution status.

DEMO

    [ ] Agentjacking demo works.
    [ ] Lying Agent demo works.
    [ ] Stale Authorization demo works.
    [ ] Confused Deputy demo works.
    [ ] Legitimate approval demo works.
    [ ] All scenarios reset independently.

OPEN SOURCE

    [ ] New developer can add an adapter without touching kernel code.
    [ ] New customer policy does not require kernel modification.
    [ ] New executor does not require kernel modification.
    [ ] Security challenges are reproducible.

QUALITY

    [ ] Existing tests continue to pass.
    [ ] New security invariants have bypass tests.
    [ ] Lean verification remains clean.
    [ ] No unexplained kernel/schema growth.
    [ ] Documentation reflects actual implementation.

======================================================================
CATEGORY 30 — DELIVERABLES
======================================================================

At the end provide:

    1. implementation
    2. architecture diagram
    3. ADRs for any kernel/schema changes
    4. commercial MVP README
    5. security model
    6. adapter development guide
    7. workflow documentation
    8. demo guide
    9. test report
    10. concise "what is Solvent?" explanation for a buyer
    11. concise "why is Solvent different?" explanation for a security
        engineer

Also provide a final implementation summary containing:

    - files added
    - files changed
    - migrations added
    - kernel changes, if any
    - why each kernel change was unavoidable
    - service-layer additions
    - adapter additions
    - UI additions
    - test coverage
    - remaining known limitations

IMPORTANT:

Do not optimize for architectural cleverness.

Optimize for:

    small trusted core
    clear boundaries
    commercially useful workflow
    visible security guarantees
    easy integration
    excellent demos
    maintainability
    open-source extensibility

The guiding principle throughout the implementation is:

    "The market should see a product.
     The security engineer should see a small kernel.
     The developer should see clean extension points."

Build toward that outcome.
```

## Why I would choose this design

The attached analysis identifies the critical product gaps as UI, workflow, actor/tool restrictions, risk/decision support, integrations, document workflows, scenario management, and demo controls.  The crucial point, however, is that these are **not reasons to inflate the kernel**; the same material explicitly recommends making the kernel visible, then actionable, then productive through surrounding layers. 

The DealForge material gives an especially useful implementation pattern: workflow continuity can be tamper-evident, but must be revalidated against current trusted state; and the “draft → send” boundary demonstrates how to make irreversible actions explicitly human-controlled.  Its generation/signature ports also reinforce the adapter boundary: external providers report results, but they do not become the source of authority. 

Most importantly, this preserves the extension hierarchy we already established: **adapters → service/policy → executor/deployment → controlled invariants → kernel only when unavoidable**. 

### The commercial MVP I would expect the agent to produce

The first really polished product should feel like this:

```text
                 AGENT / TOOLS / DATA
                         |
                         v
                  ┌───────────────┐
                  │   ADAPTERS    │
                  └───────┬───────┘
                          |
                          v
                 ┌─────────────────┐
                 │ SERVICE / POLICY│
                 │                 │
                 │ workflow        │
                 │ actor rules     │
                 │ risk/decision   │
                 │ review          │
                 └────────┬────────┘
                          |
                          v
                 ┌─────────────────┐
                 │ SOLVENT KERNEL  │
                 │                 │
                 │ authority       │
                 │ atomic state    │
                 │ invariants      │
                 └────────┬────────┘
                          |
                          v
                       COCKROACHDB
                          |
                          v
                    AUTHORIZED?
                      /       \
                    NO         YES
                    |           |
                  DENY      EXECUTOR
                              |
                              v
                         REAL SYSTEM
```

That is the key product architecture. The UI, scoring, workflows, integrations and demos should make that model obvious without weakening the kernel.

One strategic adjustment I would make relative to the raw competitive-analysis priority order: **do not build the web UI first in isolation**. Build the service/policy/workflow contracts first, then build the UI against those contracts. Otherwise the UI risks becoming another orchestration layer that bypasses the authority model. This is the main architectural lesson I would take from both AegisFlow and DealForge. 

