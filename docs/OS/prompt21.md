Use this prompt with the coding agent. It should produce the **implementation plan only**, not modify code.

```text
You are the senior architect responsible for planning the NEXT implementation phase of Solvent.

Do NOT modify code.
Do NOT implement anything.
Do NOT create migrations.
Do NOT redesign the kernel.
Produce an implementation plan only.

==================================================
CURRENT VERIFIED STATE
==================================================

Solvent has now completed the Phase 4B security/API foundation.

The following has been completed and independently adversarially reviewed:

- Phase 4A — canonical, language-neutral API contract
- Phase 4B — REST/API implementation and real Service Layer
- Phase 6.1 — authenticated-principal identity binding
- Phase 6.2 — per-package test database isolation / full-suite verification
- Phase 6.3 — targeted verification hardening
- Phase 6.4 — explicit authority serialization

Phase 6.4 replaced the invalid assumption that CockroachDB SERIALIZABLE +
NOT EXISTS(target_revocation) was sufficient to prevent the stale-authority
race.

The current canonical security path is:

    REST
      ↓
    Service
      ↓
    Kernel
      ↓
    CockroachDB

and:

    MCP
      ↓
    same Service
      ↓
    same Kernel
      ↓
    CockroachDB

The critical authorize/revoke invariant is now enforced by a shared
FOR UPDATE lock on authority_target.target_id.

The final Phase 6.4 adversarial review returned:

    GO
    READY TO FREEZE

Fresh full-suite verification is green:

    go build ./...
    go vet ./...
    go test -count=1 -p 1 ./...
    go test -count=1 ./...

and the final parallel go test ./... also passes.

Treat Phase 4B as FROZEN.
Do not reopen or redesign it unless the investigation discovers a concrete
security/correctness defect that makes the next phase impossible.

==================================================
STRATEGIC GOAL
==================================================

The ultimate product goal is:

> Make Solvent the authorization layer for AI agents, applications,
> workflows, and consequential systems.

The architecture must achieve this without turning the kernel into a giant
platform.

The enduring architectural principle is:

    SMALL TRUSTED KERNEL
            +
    EXTENSION PLANE ABOVE IT

The governing extension rule is:

    External product / protocol
        → Adapter

    Policy / orchestration / composition
        → Service / Policy

    Execution / infrastructure
        → Executor / Deployment

    Customer-specific behavior
        → Policy / configuration / data

    Reporting / UI / analytics
        → Product / read model / service

    New durable security fact or atomic security transition
        → Kernel ONLY if unavoidable

    Otherwise
        → Don't add it

The kernel should remain generic, protocol-agnostic, product-agnostic,
and unaware of GitHub, Kubernetes, AWS, MCP-specific business semantics,
A2A-specific semantics, UI concepts, compliance frameworks, or individual
AI providers.

==================================================
WHAT COMES NEXT
==================================================

The next planned phase is:

    PHASE 4C — OPENAPI + REFERENCE INTEGRATION CONTRACT

This phase is primarily about CONSUMABILITY, not adding new authority
semantics.

The intent is to turn the now-verified REST/API boundary into a stable,
language-neutral integration surface that other systems can actually use.

After that, the roadmap should move toward ONE serious real integration
rather than many shallow integrations.

Preferred direction:

    one consequential GitHub / CI-CD style integration

to prove the complete chain:

    untrusted / AI-generated information
        ↓
    evidence
        ↓
    review
        ↓
    explicit authority
        ↓
    exact current-state authorization
        ↓
    external execution
        ↓
    audit

The integration should validate the extension architecture in reality.

==================================================
PLANNING OBJECTIVE
==================================================

Produce a rigorous implementation plan for Phase 4C and the immediate
follow-on work needed to validate the extension model.

Do not jump ahead into full enterprise SaaS architecture.

Do not prematurely add:

- RBAC
- multi-tenancy
- enterprise IAM
- OAuth platform
- policy DSL
- workflow engine
- compliance subsystem
- many executors
- dozens of SDKs
- large Web UI
- generalized marketplace/plugin platform

These may become future product capabilities, but they are not the immediate
goal.

==================================================
FIRST: RECONNAISSANCE
==================================================

Before proposing work, inspect the repository and determine:

1. Exact current REST/API implementation.
2. Exact current Phase 4A contract.
3. Whether any OpenAPI/spec-generation infrastructure already exists.
4. Whether request/response types already map cleanly to OpenAPI.
5. Existing API validation/error semantics.
6. Authentication semantics exposed by the API.
7. Existing MCP semantics that must remain convergent.
8. Existing integration/adapter interfaces.
9. Existing executor interfaces.
10. Existing service boundaries.
11. Existing documentation structure.
12. Existing examples or clients.
13. Existing demo paths that could be mistaken for canonical API.
14. Current repository/package conventions for documentation and examples.
15. Existing CI checks relevant to API contract validation.

Do not assume something does or does not exist.
Inspect first.

==================================================
PHASE 4C DESIGN PRINCIPLES
==================================================

The Phase 4C plan must preserve these invariants:

1. OpenAPI describes the canonical Phase 4A API.
2. OpenAPI does NOT redefine authority semantics.
3. REST, MCP, and future A2A use the same underlying service semantics.
4. API schemas must not expose kernel tables merely because they exist.
5. Internal kernel concepts that are intentionally hidden remain hidden.
6. Error semantics remain stable and machine-readable.
7. Authentication semantics are explicit.
8. Current-state authorization semantics remain server-side.
9. No SDK becomes a second semantic contract.
10. Reference integrations must call the canonical API.
11. No language-specific client gets privileged semantics.
12. Examples must demonstrate correct security usage, not shortcuts.

==================================================
API CONTRACT RECONCILIATION
==================================================

Compare:

    Phase 4A contract
        ↕
    current Phase 4B implementation
        ↕
    intended OpenAPI document

Identify:

- exact mismatches
- missing schemas
- undocumented status codes
- inconsistent field requirements
- inconsistent nullable/optional semantics
- inconsistent enum definitions
- inconsistent error codes
- authentication ambiguities
- pagination semantics
- idempotency semantics
- timestamp representation
- UUID representation
- raw JSON field semantics
- actor/principal semantics
- authorization result semantics

Do NOT silently "fix" Phase 4A.
Any discrepancy must be classified as:

    implementation bug
    documentation gap
    contract mismatch requiring explicit decision
    acceptable v0 limitation

The Phase 4A contract remains the canonical semantic source of truth.

==================================================
OPENAPI DESIGN
==================================================

Plan the canonical OpenAPI surface.

Determine:

- OpenAPI version
- document location
- reusable schemas
- reusable error schema
- security scheme
- response envelopes
- path parameters
- request bodies
- enum declarations
- pagination structure
- common headers
- request ID behavior
- authentication description
- authorization semantics
- concurrency/conflict behavior
- retryable error representation

The generated specification should accurately describe the actual API.

Do not invent endpoints.

Do not expose:

- direct authority-table mutation
- direct target activation
- direct snapshot mutation
- direct intent insertion
- "execute without authorization"
- force authorization
- approve-and-execute
- override authorization
- arbitrary authority state mutation

==================================================
REFERENCE INTEGRATIONS
==================================================

Design a SMALL number of reference examples.

The purpose is to prove that Solvent is truly language-neutral.

Choose examples that provide maximum architectural value with minimum code.

At minimum consider:

- one simple non-Go HTTP client
- one realistic agent/workflow integration

Do NOT automatically create official SDKs.

A reference client should be a thin wrapper over the canonical API.

The contract remains:

    OpenAPI / HTTP API = canonical
    client library = convenience

Do not allow reference clients to grow their own security semantics.

==================================================
EXTENSION-MECHANISM VALIDATION
==================================================

The plan must explicitly explain how Phase 4C prepares for the extension plane.

Evaluate whether the current interfaces cleanly support:

    Protocol Adapter
        ↓
    Service
        ↓
    Kernel

and later:

    Service
        ↓
    Executor Adapter
        ↓
    External system

Also determine what is currently missing to support a first real integration.

Examples:

    GitHub Adapter
    CI/CD Adapter
    Kubernetes Adapter
    Cloud Adapter
    Agent-runtime Adapter

But do NOT implement all of them.

Identify the minimum adapter contract necessary to support one serious
integration while keeping the kernel unchanged.

==================================================
ONE REAL INTEGRATION STRATEGY
==================================================

After Phase 4C, plan the first serious integration.

Preferred candidate:

    GitHub / CI-CD / deployment authorization

Example conceptual flow:

    AI agent proposes:
        deploy commit X to production

    Solvent receives:
        actor
        resource
        target
        action
        consequence
        evidence

    reviewer/policy establishes authority

    Solvent authorizes:
        EXACT principal
        EXACT action
        EXACT target
        CURRENT authority

    executor invokes GitHub/CI/CD provider

    Solvent records:
        authorization
        execution reference
        result
        audit evidence

The plan must clearly separate:

    authorization
from
    execution

and:

    Solvent authority
from
    external provider semantics

==================================================
EXECUTOR BOUNDARY
==================================================

Inspect the existing executor abstraction.

Determine exactly what is missing for the first real executor.

The design must preserve:

    Executor is NOT an authority engine.

Executor must not:

- approve authority
- promote beliefs
- revoke authority
- redefine authorization
- bypass current-state checks

The canonical future execution path should be:

    request
      ↓
    current authorization validation
      ↓
    Executor
      ↓
    provider

Do not claim atomic coordination between CockroachDB and the external
provider.

Explicitly document the remaining external side-effect boundary.

==================================================
SERVICE / POLICY EXTENSION MODEL
==================================================

Determine whether current service boundaries are sufficient for:

- evidence ingestion
- policy evaluation
- integration orchestration
- execution coordination
- audit
- future governance

Do not create placeholder services just for architectural neatness.

Only propose a new boundary where there is a concrete responsibility that
cannot cleanly live in an existing service.

Remember:

    Policy ≠ Authority

Advisory policy output cannot substitute for kernel authority.

==================================================
WEB UI POSITION
==================================================

Treat Web UI as optional and replaceable.

Do not make UI a dependency of:

- MCP
- REST
- A2A
- agents
- workflows
- the kernel

Do not include major UI implementation in Phase 4C unless reconnaissance
shows something necessary for API/integration validation.

==================================================
DOCUMENTATION
==================================================

Plan the smallest documentation set needed to make the API usable by another
engineer without reading the kernel source.

At minimum evaluate:

- API getting started
- authentication example
- authorization example
- error handling example
- authorize-action example
- reference integration
- security model overview
- extension model overview

Avoid documentation drift.

Prefer generating portions of the documentation from the canonical
OpenAPI contract where practical.

==================================================
CI / CONTRACT VALIDATION
==================================================

Plan how the canonical API contract becomes continuously verifiable.

Possible mechanisms:

- OpenAPI linting
- schema validation
- generated artifact verification
- contract tests
- API integration tests against OpenAPI examples

Do not add heavyweight tooling unless justified.

The important invariant is:

    implementation
        ↔
    canonical OpenAPI
        ↔
    reference client

must not silently drift.

==================================================
SECURITY REVIEW REQUIREMENTS
==================================================

The implementation plan must include adversarial verification.

At minimum review:

1. API contract drift
2. accidental exposure of kernel semantics
3. authentication ambiguity
4. caller-controlled security fields
5. error leakage
6. unauthorized resource access
7. accidental second authorization engine
8. API/MCP semantic divergence
9. executor bypass
10. stale/cached authorization behavior
11. arbitrary provider capability selection
12. reference-client security shortcuts

No "looks good" review.

Require concrete code-path evidence.

==================================================
KERNEL-GROWTH GATE
==================================================

Explicitly evaluate every proposed Phase 4C/extension change against:

    Is this:
      - a new durable security fact?
      - an atomic security-critical state transition?
      - impossible to represent safely above the kernel?

If NO:
    do not modify the kernel.

If YES:
    document why.

The default answer should remain:

    extend above the kernel.

==================================================
DELIVERABLES
==================================================

Produce a concrete implementation plan containing:

1. Executive decision
2. Current-state reconnaissance
3. Phase 4C goals
4. Phase 4C non-goals
5. API/contract reconciliation
6. OpenAPI structure
7. Schema strategy
8. Error/security scheme
9. Reference integration examples
10. Contract-validation strategy
11. Extension-interface assessment
12. First-real-integration proposal
13. Executor boundary requirements
14. Service/policy requirements
15. Documentation
16. CI/testing
17. Security/adversarial review
18. Kernel-growth gate
19. Package/file-level implementation map
20. Implementation order
21. Acceptance criteria
22. Rollback strategy
23. Risks / unresolved decisions
24. Explicit next phase after 4C

==================================================
IMPLEMENTATION DETAIL
==================================================

For every proposed implementation item identify:

- exact package/file
- responsibility
- why it belongs there
- dependency
- whether it is production or test/documentation code
- whether it introduces a new abstraction
- whether it modifies existing public semantics

Avoid estimates based solely on line count.

Prefer the smallest change that satisfies the goal.

==================================================
IMPORTANT ARCHITECTURAL QUESTION
==================================================

The plan must explicitly answer:

> How do we make Solvent universal without making the Solvent kernel universal?

The answer should be expressed concretely in terms of:

    stable kernel semantics
    canonical API
    service/policy composition
    protocol adapters
    integration adapters
    executor adapters
    product/read models

Show where a future:

    GitHub integration
    Kubernetes integration
    MCP integration
    A2A integration
    autonomous agent runtime
    enterprise workflow

would plug in WITHOUT changing the kernel.

==================================================
IMPORTANT PRODUCT QUESTION
==================================================

The plan must also answer:

> What is the smallest next implementation that makes Solvent more useful
> as an authorization layer for AI agents and real software systems?

Do not optimize for number of features.

Optimize for:

    one clear integration contract
    one strong reference integration
    one excellent end-to-end security story
    minimal trusted core
    maximum reuse

==================================================
FINAL ACCEPTANCE GATE
==================================================

The plan must define a gate for completion.

At minimum:

    go build ./...
    go vet ./...
    go test -count=1 ./...

plus:

    OpenAPI contract validation
    API integration validation
    reference-client validation
    adversarial security review

Do not declare Phase 4C complete merely because the API compiles.

==================================================
OUTPUT FORMAT
==================================================

Return:

# Phase 4C Implementation Plan

## 1. Executive Decision

## 2. Current-State Reconnaissance

## 3. Architecture

Include a diagram.

## 4. Phase 4C Scope

## 5. Phase 4C Non-Goals

## 6. API Contract Reconciliation

## 7. OpenAPI Design

## 8. Reference Integrations

## 9. Extension Mechanism Validation

## 10. First Real Integration

## 11. Executor Boundary

## 12. Service / Policy Boundary

## 13. Documentation

## 14. CI / Contract Verification

## 15. Security / Adversarial Review

## 16. Kernel-Growth Gate

## 17. Package / File Implementation Map

## 18. Implementation Order

## 19. Acceptance Criteria

## 20. Rollback Strategy

## 21. Risks / Open Questions

## 22. Next Phase

## 23. Final Recommendation

Use evidence from the actual repository.

Do not modify code.
Do not implement.
Do not silently reconcile contradictions.
Do not invent missing infrastructure.
Do not expand the kernel for convenience.

The goal is not simply to document an API.

The goal is to establish the next layer of Solvent's long-term architecture:

    SMALL TRUSTED AUTHORITY KERNEL
                ↓
        STABLE LANGUAGE-NEUTRAL API
                ↓
         EXTENSION PLANE
                ↓
    AGENTS / APPS / WORKFLOWS / SYSTEMS

where Solvent can eventually sit between autonomous systems and consequential
actions without becoming a giant, monolithic security platform.
```

The key thing this prompt does is make the agent plan **Phase 4C as the bridge between the frozen kernel and the extension ecosystem**, rather than treating OpenAPI as a documentation exercise. That keeps the roadmap aligned with the larger Solvent objective.
