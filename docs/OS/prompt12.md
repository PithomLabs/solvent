You are the senior API architect for Solvent.

Your task is NOT to implement Phase 4 yet.

Your task is to produce a rigorous, implementation-ready DESIGN + IMPLEMENTATION PLAN for:

PHASE 4A — CANONICAL SOLVENT API CONTRACT

The resulting plan will be submitted to an independent adversarial security/architecture reviewer before implementation begins.

Treat the existing Solvent repository as the source of truth for CURRENT behavior. Do not invent capabilities that do not exist. Clearly distinguish:

1. CURRENT behavior already implemented
2. INTENDED Phase 4A API contract
3. FUTURE/deferred capabilities
4. OPEN DESIGN QUESTIONS requiring a decision

==================================================
1. SOLVENT ARCHITECTURAL BASELINE
==================================================

Use these principles as hard constraints.

Core thesis:

> Retrieval is not authority.

The Solvent kernel is the smallest trusted authority core.

Architectural rule:

External product/protocol       → Adapter
Policy/orchestration/composition → Service / Policy
Execution/infrastructure        → Executor / Deployment
Customer-specific behavior      → Policy / configuration/data
Reporting/UI/analytics/scoring  → Product / read model / service
Impossible state                → DB invariant
New atomic security primitive   → Kernel only if unavoidable
Everything else                 → Don't add it

The kernel is intentionally small and UI-blind.

Authority truth remains in the kernel and database invariants.

`kernel.Authorize` is the final authority oracle.

`kernel.Approve` is the sole authority-creating semantic operation.

`kernel.RevokeTarget` is append-only revocation.

Exact action/target binding is mandatory.

Do not introduce a second authorization engine in the API, UI, MCP adapter, A2A adapter, or service layer.

The product is:

> Solvent is a small, portable authority kernel/layer between autonomous systems and consequential actions.

It is NOT an agent firewall.

Solvent is intended to be language-agnostic.

Canonical architectural shape:

Any Programming Language
        ↓
Language-neutral Solvent API / Protocol
        ↓
Solvent Service Layer
        ↓
Solvent Kernel
        ↓
CockroachDB

MCP, A2A, REST/API, and Web UI must map to the same service semantics rather than implementing independent security logic.

Web UI is optional and replaceable.

Deleting the Web UI must not weaken the MCP/A2A/API capabilities or authority model.

==================================================
2. CURRENT SECURITY BASELINE
==================================================

Assume the current repository has already passed the latest security remediation baseline.

Important current facts:

- `kernel.Authorize` is healthy, read-only, and performs exact field-by-field authorization matching.
- `kernel.Approve` is the authority creation boundary.
- `kernel.RevokeTarget` is append-only.
- There is currently NO real production consequential external executor.
- `ExecuteAction` is a future canonical execution boundary and has only test/recording executor support.
- No production execution path may bypass `kernel.Authorize`.
- `service/workflow` was removed as dead code.
- Workflow tokens are not part of the current execution/authority boundary.
- Do not reintroduce workflow tokens merely to satisfy an API design.
- MCP `solvent_authorize_action` requires the relevant actor/target information and fails closed.
- `action_source` distinguishes `user_typed` from `tool_output`; untrusted tool output must not silently become authority.
- Authentication is NOT currently the kernel's responsibility.
- `actor=HUMAN` in a request body is not proof of identity.
- Authentication/attestation belongs at the API/MCP/deployment/application boundary.
- The trusted `operator-review` CLI remains a documented administrative trust boundary.
- There is no authority-core schema expansion approved for Phase 4A.
- No fake production executor should be introduced.
- No compliance subsystem should be invented merely to make the product look enterprise-ready.

Treat these as constraints unless the repository demonstrably contradicts them.

==================================================
3. PRIMARY OBJECTIVE
==================================================

Design the public Solvent API before designing the Web UI.

The API must become the canonical product contract.

The Web UI, MCP adapter, A2A adapter, SDKs, and reference integrations should consume the same service semantics.

The API design must be:

- language-neutral
- explicit
- security-first
- small
- understandable
- versionable
- suitable for autonomous agents
- suitable for human-operated systems
- suitable for future consequential execution
- independent of the Web UI
- independent of Go-specific implementation details

The design must explicitly define both:

A. what Solvent DOES expose
B. what Solvent deliberately DOES NOT expose

The absence of dangerous operations is itself part of the API security model.

==================================================
4. FIRST: RECONNAISSANCE
==================================================

Before proposing the API, inspect the repository deeply.

Inspect at minimum:

- kernel package
- service layer
- MCP implementation
- existing HTTP/API code if any
- adapters
- storage/repositories
- authority/belief/evidence models
- action intent structures
- audit/activity structures
- CLI administrative paths
- tests
- fixtures
- README / architecture docs
- any existing OpenAPI/API schema

If DealForge and/or AegisFlow reference implementations are present, inspect them for API/resource-modeling, request/response, naming, ergonomics, and integration patterns only.

Use them as UX/API design references, NOT as authority-model references.

Do NOT import or reproduce their approval, workflow, token, authorization, or authority semantics where those conflict with Solvent's governing principles:

- evidence != authority
- workflow != authority
- authorize != execute
- current authority wins
- kernel is the final authority oracle

In particular, do NOT reintroduce workflow-token or client-carried approval patterns merely because they appear convenient or established in a reference implementation.

The Solvent API contract must be derived from Solvent's own domain and authority model, with the existing kernel remaining the sole authority oracle regardless of how reference systems represent approval or workflow state.

Do not assume names.

Map the actual current domain.

Produce a CURRENT DOMAIN INVENTORY showing:

Resource/entity
Purpose
Authoritative source
Current owner
Read path
Write path
Security sensitivity
Current API exposure
Recommended API exposure

Pay special attention to:

- principal
- authority_target
- target_snapshot
- target_activation
- target_revocation
- justification
- debt_discharge
- belief
- evidence
- action intent
- authorization decision
- activity/audit
- integration
- execution-related concepts

Determine which of these are true public API resources versus internal implementation concepts.

Do NOT expose kernel tables merely because they exist as database tables.


PRE-FLIGHT BASELINE VERIFICATION

Before treating the stated security baseline as authoritative, verify it against the actual repository.

In particular, verify that:

- MCP `solvent_authorize_action` requires the relevant actor/target information.
- Missing required authorization inputs fail closed rather than conditionally skipping authorization.
- malformed actor/target identifiers fail safely.
- no production consequential execution path bypasses `kernel.Authorize`.
- `service/workflow` and stale workflow-token execution paths are actually absent.
- `GetToken` and equivalent stale workflow-token helpers are absent unless demonstrably required by current architecture.

If the repository contradicts any stated baseline fact:

1. STOP assuming the baseline is current.
2. Record the discrepancy under `BASELINE DRIFT`.
3. Identify the actual current behavior.
4. Do not design Phase 4A on top of the incorrect assumption.
5. End the document as `NOT READY FOR ADVERSARIAL REVIEW` unless the discrepancy is clearly non-security-relevant and does not affect the API contract.

Do not silently "fix" the repository while performing this reconnaissance. This task is primarily a design/planning exercise.

## REFERENCE CONTAMINATION

Did any API concept, workflow state, token, approval model, or authority abstraction originate from DealForge/AegisFlow?

If yes, demonstrate that it is compatible with Solvent's governing principles.
If compatibility cannot be demonstrated, remove it from the proposed contract.

==================================================
5. DEFINE THE CANONICAL API DOMAIN MODEL
==================================================

Propose the smallest coherent public resource model.

For every proposed public resource, define:

- resource name
- meaning
- lifecycle
- authoritative source
- identifier
- immutable fields
- mutable fields
- relationships
- whether it is authoritative or derived
- whether agents may create/read/update it
- whether humans may create/read/update it
- whether it crosses the authority boundary
- whether it requires authentication
- whether it requires authorization
- whether mutation needs idempotency
- audit requirements

Explicitly distinguish:

FACTS
PROJECTIONS
ADVISORY INFORMATION
AUTHORITY

Do not allow advisory/read-model state to masquerade as authority.

Determine whether the public API should directly expose internal kernel entities or whether it should use product-level representations.

Favor stable domain semantics over leaking implementation details.

==================================================
6. DEFINE THE API OPERATIONS
==================================================

Design the canonical operations.

Examples may include concepts such as:

- inspect evidence
- submit evidence
- inspect beliefs
- create/review decisions
- create authority
- request authorization
- inspect current authorization
- revoke authority
- create/manage action intent
- retrieve audit/activity
- inspect integration status

But DO NOT automatically adopt these names.

Derive the actual operation set from the repository and architecture.

For EVERY proposed operation define:

- HTTP method / protocol operation
- canonical endpoint or RPC name
- request schema
- response schema
- authentication requirement
- authorization requirement
- actor requirements
- actor type
- whether actor identity is server-established or caller-supplied metadata
- read-only / reversible / irreversible classification
- idempotency requirements
- audit behavior
- kernel interaction
- database interaction
- external side effects
- failure modes
- retry semantics
- concurrency considerations
- whether the operation can ever create authority
- whether it can ever revoke authority
- whether it can ever cause external execution

Most importantly:

Identify which operations cross the authority boundary.

There must be NO ambiguous operation whose semantics accidentally imply authority.

==================================================
7. EXPLICIT SECURITY NEGATIVE SPACE
==================================================

Create a dedicated section titled:

"Forbidden API Surface"

Explicitly analyze dangerous API shapes such as:

- /execute-without-authority
- /set-authorized
- /approve-with-agent
- /update-authority-state
- /force-execution
- /override-authorization
- /set-actor
- /impersonate
- /approve-and-execute
- /execute-as-approved
- /trust-tool-output
- arbitrary executor selection by caller
- arbitrary authority tuple mutation
- client-supplied current authority
- client-supplied approval state
- client-supplied debt state
- client-supplied evidence verification state

Explain why each is forbidden, unsafe, or out of scope.

Design the API so that dangerous semantics are structurally difficult or impossible to express.

Do not rely on documentation alone.

==================================================
8. AUTHENTICATION / ACTOR MODEL
==================================================

Design the API boundary around:

ACTOR TYPE != IDENTITY != AUTHENTICATION

Define:

- HUMAN
- AGENT
- SYSTEM

But do NOT treat those strings as proof of identity.

Determine the API authentication abstraction.

Do not invent a specific enterprise IAM implementation unless required by the repository.

The plan should define an abstraction that allows:

- API keys
- OAuth/OIDC
- service identity
- future workload identity
- future attestation

without putting those mechanisms into the kernel.

Define:

- authenticated principal
- actor attribution
- authorization context
- impersonation handling
- service-to-service calls
- agent calls
- administrative calls

Explain exactly what evidence the API accepts as trustworthy and what is merely caller assertion.

==================================================
9. API AUTHORIZATION SEMANTICS
==================================================

Define the difference between:

AUTHENTICATION
API ACCESS CONTROL
SOLVENT AUTHORITY
POLICY
EXECUTION AUTHORIZATION

Do not collapse them into one concept.

The API/service layer may enforce:

- endpoint access
- role/capability access
- actor restrictions
- policy constraints
- request validity

But the kernel remains authoritative for Solvent authority semantics.

For operations requiring authority:

API/service
    ↓
construct authoritative tuple/context
    ↓
re-read current state
    ↓
kernel.Authorize
    ↓
allow/deny
    ↓
optional external operation

Never:

API
    ↓
cached approval
    ↓
execute

Never:

API
    ↓
caller says approved=true
    ↓
execute

==================================================
10. REQUEST / RESPONSE DESIGN
==================================================

Define conventions for:

- IDs
- timestamps
- version fields
- enums
- nullable fields
- pagination
- filtering
- sorting
- field naming
- metadata
- provenance
- errors
- validation
- correlation/request IDs
- idempotency keys
- optimistic concurrency if needed

Keep the format language-neutral.

Prefer stable JSON structures.

Do not expose Go types directly.

Do not expose CockroachDB-specific implementation details.

Do not create custom abstraction layers unless they solve a real problem.

==================================================
11. ERROR MODEL
==================================================

Design a canonical error contract.

It must distinguish at least conceptually:

- malformed request
- authentication failure
- access denied
- invalid actor
- invalid target
- stale state
- revoked authority
- missing authority
- exact tuple mismatch
- policy denial
- conflict
- invariant violation
- external provider failure
- internal server error

Determine which failures are safe to expose publicly and which should remain implementation-specific.

Design errors so agents can make safe decisions.

For consequential denial, the API should provide enough structured reason information for an agent or human to understand:

WHAT was requested
WHY it was denied
WHICH authoritative fact caused the denial

without leaking sensitive implementation details.

==================================================
12. IDEMPOTENCY / RETRY / CONCURRENCY
==================================================

Design idempotency semantics for mutating operations.

Analyze carefully:

- duplicate requests
- network retries
- client retries
- concurrent approval
- duplicate authority creation
- duplicate revocation
- duplicate evidence submissions
- duplicate execution requests in the FUTURE

Do not invent distributed transaction guarantees that Solvent does not have.

Explicitly document the boundary between:

DB atomicity

and

DB state + external provider side effects.

Preserve the existing TOCTOU limitation:

Solvent may establish valid CURRENT authorization immediately before an external executor invocation, but it does not claim an atomic DB + provider transaction.

==================================================
13. API VERSIONING
==================================================

Design API versioning strategy.

Consider:

- `/v1`
- header/version negotiation
- additive evolution
- breaking changes
- deprecation
- MCP schema evolution
- A2A compatibility
- SDK compatibility

Pick the simplest strategy that preserves long-term compatibility.

Do not create a complicated versioning framework prematurely.

==================================================
14. MCP / A2A MAPPING
==================================================

The API must remain canonical.

Map MCP and A2A semantics onto the canonical service model.

For each important API operation explain:

REST/API representation
MCP representation
A2A representation
Service operation
Kernel interaction

Do not let MCP or A2A introduce alternate authority semantics.

Identify where tool schemas may accidentally expose security-sensitive caller-controlled state.

Especially review:

- actor_id
- target_id
- action
- consequence
- action_source
- evidence
- approval claims
- authorization claims

MCP must remain an adapter into Solvent rather than becoming a separate authorization system.

==================================================
15. WEB UI BOUNDARY
==================================================

The Web UI is NOT the API definition.

Define the API so that:

MCP
A2A
REST
Web UI
future SDKs

all consume the same service semantics.

Explicitly identify which UI requirements belong in:

- API
- read model
- service
- client only

Do not introduce UI-specific concepts into the kernel.

The UI must be replaceable.

==================================================
16. SDK STRATEGY
==================================================

Solvent is language-agnostic.

Do NOT initially propose SDKs for every language.

Design:

Canonical API
    ↓
OpenAPI / protocol specification
    ↓
thin SDKs later as justified

Explain which language SDKs, if any, are justified immediately and why.

SDKs must never become architectural dependencies.

==================================================
17. API DOCUMENTATION / OPENAPI
==================================================

Produce a proposed OpenAPI structure.

Include:

- top-level tags
- resources
- operations
- schemas
- errors
- authentication
- idempotency
- pagination
- examples
- security annotations

Do NOT write a giant generated OpenAPI file yet.

Instead provide enough structure that implementation can create it deterministically.

Identify any operations that should initially remain internal rather than public.

==================================================
18. AUDIT / OBSERVABILITY CONTRACT
==================================================

Define the public activity/audit model.

Distinguish:

- request received
- authentication result
- Solvent decision
- authority creation
- authority revocation
- adapter invocation
- provider result
- future external execution
- failure

Do not conflate:

Authorize != Execute

Design an audit event shape that supports later compliance evidence without creating a compliance subsystem now.

==================================================
19. EXECUTION FUTURE-PROOFING
==================================================

Current repository has no production consequential executor.

Do NOT add one.

However, ensure the API design does not accidentally make future safe execution impossible.

Define the future conceptual boundary:

API
 → ExecutionService
 → current-state revalidation
 → kernel.Authorize
 → Executor
 → provider

The API design must prevent a future implementer from naturally creating:

API
 → provider

or:

API
 → cached approval
 → provider

Also analyze whether arbitrary caller-provided executor/capability identifiers would be dangerous.

Do not implement executor selection now.

==================================================
20. DATA MODEL / DB IMPACT
==================================================

Determine which API resources can be served from existing data.

Do NOT add authority-core schema merely for API convenience.

For every proposed migration identify:

- why it is needed
- whether it is authority-core
- whether it is product/read-model
- whether it can instead be derived
- whether it can be deferred

Strongly prefer no kernel schema changes in Phase 4A.

==================================================
21. EXTERNAL INTEGRATIONS
==================================================

Review how a future GitHub/CI/CD/deployment integration would consume the API.

Use the preferred architecture:

IntegrationService
    ↓
provider adapter
    ↓
external provider

The API must not contain provider-specific semantics.

GitHub should remain an integration, not become the Solvent domain model.

==================================================
22. REFERENCE WORKFLOWS
==================================================

Design canonical request flows for at least:

1. Evidence submission
2. Human approval / authority creation
3. Authorization check
4. Rejected authorization
5. Revoked authority
6. Wrong target
7. Agent-provided untrusted claim
8. Future execution request
9. Audit lookup

For each flow provide:

Caller
→ API operation
→ service
→ repository/read model
→ kernel
→ result
→ audit

Mark the trust boundary at every stage.

==================================================
23. ADVERSARIAL DESIGN REVIEW PREP
==================================================

Before finalizing the plan, attack your own design.

Attempt to construct:

- confused deputy
- privilege escalation
- caller-controlled authority
- stale authorization
- replay
- duplicate approval
- wrong-target execution
- wrong-actor execution
- forged approval
- forged evidence
- agent/tool output treated as authority
- cached authorization
- UI-controlled authorization
- MCP-only bypass
- A2A-only bypass
- direct REST bypass
- arbitrary executor selection
- hidden provider side effect
- authorization/execution confusion
- schema ambiguity
- type confusion
- insecure default
- missing authentication boundary
- accidental admin bypass

For each attack state:

Attack
Current design defense
Remaining gap
Whether it is Phase 4A
Whether it requires kernel change
Whether it is deferred

==================================================
24. TEST PLAN
==================================================

Produce an implementation test matrix before implementation.

At minimum include:

API contract tests
authentication tests
authorization tests
actor/identity tests
negative tests
idempotency tests
concurrency tests
stale-state tests
revocation tests
wrong-target tests
wrong-actor tests
MCP parity tests
A2A parity tests
audit tests
error-contract tests
serialization tests

Include explicit regression tests proving that:

- caller cannot set authority directly
- caller cannot bypass kernel authorization
- UI state cannot authorize
- MCP cannot create an alternate authorization path
- A2A cannot create an alternate authorization path
- agent claims cannot become authority
- stale authority is rejected
- revoked authority is rejected
- wrong target is rejected
- wrong actor is rejected
- malformed identifiers fail safely
- advisory confidence cannot create authority

==================================================
25. IMPLEMENTATION PLAN
==================================================

After designing the contract, produce a staged implementation plan.

Prefer small steps such as:

Step 1 — Freeze domain/API vocabulary
Step 2 — Define canonical schemas
Step 3 — Define service interfaces
Step 4 — Define authentication boundary
Step 5 — Define API handlers/adapters
Step 6 — Implement error model
Step 7 — Add OpenAPI contract
Step 8 — Add MCP mapping/parity
Step 9 — Add A2A mapping/parity
Step 10 — Add contract/integration tests
Step 11 — Adversarial review
Step 12 — Only then proceed to optional UI work

For every step identify:

Files/packages likely affected
New interfaces
New structs
Migrations if any
Tests
Security implications
Rollback strategy

Do not modify the kernel unless the plan identifies a genuine atomic security primitive that cannot be expressed outside it.

==================================================
26. REQUIRED OUTPUT
==================================================

Produce ONE implementation-plan document in Markdown.

Suggested structure:

# Phase 4A — Canonical Solvent API Contract

## 1. Executive Decision
## 2. Repository Reconnaissance
## 3. Current Domain Inventory
## 4. Proposed Public Resource Model
## 5. Canonical Operations
## 6. Authentication and Actor Model
## 7. Authorization Semantics
## 8. Forbidden API Surface
## 9. Request/Response Conventions
## 10. Error Contract
## 11. Idempotency and Concurrency
## 12. Versioning
## 13. MCP Mapping
## 14. A2A Mapping
## 15. Web UI Boundary
## 16. SDK Strategy
## 17. OpenAPI Structure
## 18. Audit Contract
## 19. Future Execution Boundary
## 20. Database Impact
## 21. Integration Model
## 22. Reference Workflows
## 23. Threat / Adversarial Analysis
## 24. Test Matrix
## 25. Implementation Plan
## 26. Deferred Decisions
## 27. Acceptance Criteria
## 28. Final Architecture Rules

==================================================
27. HARD CONSTRAINTS
==================================================

Do NOT:

- implement the Web UI
- add a production executor
- add compliance/GRC infrastructure
- enlarge the kernel merely for API convenience
- create duplicate authorization logic
- reintroduce workflow dead code
- add workflow tokens to the authority path
- invent enterprise IAM infrastructure
- invent SDKs for every language
- expose database tables as an API by default
- allow caller-controlled authority state
- allow caller-controlled approval state
- trust actor type strings
- trust tool output as authority
- create an `/execute` path that bypasses current-state authorization
- add fake functionality for demo purposes

Do:

- inspect the actual repository
- derive the design from current implementation
- make security semantics explicit
- make negative space explicit
- keep the API language-neutral
- keep the kernel small
- make REST/MCP/A2A semantics converge
- make the Web UI a client
- make future execution safe by construction
- minimize migrations
- identify unresolved questions instead of guessing
- optimize for a small, durable public contract

==================================================
28. FINAL SELF-CHECK
==================================================

End the document with a checklist:

[KERNEL]
Does Phase 4A require any kernel change?
If yes, why is it unavoidable?

[AUTHORITY]
Is there exactly one source of truth for authority?

[API]
Can every consequential API operation explain exactly how current authority is checked?

[MCP]
Can MCP do anything REST cannot?

[A2A]
Can A2A do anything REST cannot?

[UI]
Can removing the UI leave the security model unchanged?

[EXECUTION]
Can a future executor be inserted without bypassing kernel.Authorize?

[AGENT SAFETY]
Can untrusted agent/tool output become authority?

[IDENTITY]
Can a caller simply claim to be HUMAN or an authorized principal?

[REPLAY]
Are retries and duplicate mutations safe?

[TOCTOU]
Are DB/external-side-effect limits honestly documented?

[SCHEMA]
Did Phase 4A unnecessarily grow the authority-core schema?

[COMPLEXITY]
Is every proposed API concept justified?

Finish with one of:

READY FOR ADVERSARIAL REVIEW

or

NOT READY FOR ADVERSARIAL REVIEW

Do NOT implement Phase 4A code unless the document itself explicitly identifies a tiny prerequisite needed solely to validate the contract. The primary deliverable is the design/implementation plan.