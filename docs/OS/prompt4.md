Below is the **consolidated implementation prompt**, updated to incorporate the final decision that **Phase 5 Compliance/Governance is deferred and must be earned by real demand**, while preserving the rest of the approved architecture. It consolidates the implementation contract, security corrections, demo portfolio, commercial MVP, and open-source roadmap from the approved plan.  

---

# SOLVENT — CONSOLIDATED IMPLEMENTATION PROMPT

You are the principal engineer implementing the next commercial MVP of **Solvent**, a portable authority layer for autonomous systems.

Your job is to implement the product **around the existing Solvent authority kernel without turning the product into the kernel**.

The governing principle is:

> **Keep the kernel as the smallest trusted authority core.**

The implementation must optimize for:

* security correctness
* small trusted core
* explicit trust boundaries
* current authoritative state
* commercially useful workflow
* excellent demonstrations
* one production-quality integration
* clear developer extension points
* maintainability
* open-source credibility

Do **not** optimize for maximum feature count or maximum code.

---

# 1. PRODUCT THESIS

Solvent is:

> **A portable authority layer for autonomous systems.**

The core distinctions are non-negotiable:

```text
Evidence is not authority.
Agent output is not authority.
Retrieval is not authority.
Workflow state is not authority.
A workflow token is not authority.
Authorization is not execution.
```

An action is allowed only when **current authoritative Solvent state** establishes the required authority.

The intended product perception is:

```text
BUYER:
"This governs what our AI systems are actually allowed to do."

SECURITY ENGINEER:
"The critical authority decision is backed by a small,
transactionally enforced core rather than LLM behavior."

DEVELOPER:
"I can integrate my system without modifying that core."

COMPLIANCE TEAM:
"I can trace controls and evidence to actual governed actions."

OPEN-SOURCE CONTRIBUTOR:
"I can extend Solvent without turning every integration
into another security mechanism."
```

---

# 2. ABSOLUTE ARCHITECTURAL RULE

## KEEP THE KERNEL SMALL

The kernel is the smallest trusted authority core.

Default:

> **DO NOT MODIFY THE KERNEL.**

For every requested feature, apply this decision tree:

```text
External product / protocol?
    → ADAPTER

Workflow / orchestration / composition / policy?
    → SERVICE / POLICY

Execution / infrastructure?
    → EXECUTOR / DEPLOYMENT

Customer-specific behavior?
    → POLICY / CONFIGURATION / DATA

Reporting / UI / analytics / scoring / audit presentation?
    → PRODUCT / READ MODEL / SERVICE

New durable security fact or atomic security-critical transition
that cannot be expressed with existing primitives?
    → POSSIBLE KERNEL CHANGE

Previously possible invalid state must become structurally impossible?
    → POSSIBLE DB INVARIANT

Otherwise:
    → DO NOT ADD IT
```

External concepts that must remain outside the kernel include:

```text
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
```

The kernel must remain domain-generic.

---

# 3. TRUST BOUNDARIES

Maintain these boundaries:

```text
External systems
      ↓
Adapters
      ↓
Service / Policy layer
      ↓
Solvent Kernel
      ↓
CockroachDB
```

And separately:

```text
Solvent authorization
      ↓
Executor / Deployment adapter
      ↓
Real-world system
```

Never collapse:

```text
authorization
execution
workflow continuity
actor identity
authentication
evidence
```

into one concept.

---

# 4. PHASE 0 — RECONNAISSANCE

Before modifying any code:

1. Inspect the complete repository.
2. Determine the actual architecture from implementation rather than assuming documentation is accurate.
3. Identify:

   * kernel packages
   * service/application packages
   * belief lifecycle
   * authority lifecycle
   * action_intent lifecycle
   * DB schema and migrations
   * MCP tools
   * HTTP/web server
   * existing wizard/demo
   * adapters
   * views/read models
   * Lean model
   * audit/refusal mechanisms
   * tests
   * task/build commands
   * deployment configuration
4. Run all existing tests and verification gates.
5. Identify code that can be wrapped or reused rather than duplicated.
6. Produce a concise implementation map before editing.

Do not begin architectural refactoring based solely on the plan. Confirm what actually exists.

---

# 5. PRESERVE CURRENT KERNEL SEMANTICS

Preserve the current authority model and its database semantics.

Existing authority concepts include:

```text
principal
authority_target
target_snapshot
target_activation
target_revocation
justification
debt_discharge
```

Preserve the semantics of:

```text
Approve
Authorize
RevokeTarget
```

Preserve:

* immutable target snapshots
* exact target binding
* activation uniqueness
* debt/promotion gates
* transactional authority creation
* read-only authorization verification
* DB-enforced invariants
* Lean formal verification

Do not create a second authority engine in the service layer.

The service layer may orchestrate the kernel.

It may not redefine authority.

---

# 6. KERNEL GROWTH GATE

Before changing kernel code or authority-core schema, create an ADR documenting:

```text
Problem
Why adapter cannot solve it
Why service/policy cannot solve it
Why configuration cannot solve it
New durable security fact
New atomic state transition
Required DB invariant
Impact on authority semantics
Impact on Lean verification
Migration strategy
Backward compatibility
Security argument
```

Default decision:

> **REJECT THE KERNEL CHANGE**

unless the ADR demonstrates that existing kernel primitives are fundamentally insufficient.

Every unexplained kernel or authority-schema expansion is a defect.

---

# 7. SERVICE ARCHITECTURE

Build or consolidate these service boundaries:

```text
WorkflowService
PolicyService
EvidenceService
AuthorityService
AuditService
IntegrationService
ExecutionService
```

Reserve:

```text
ComplianceService
```

as a product architecture boundary, but **do not implement substantive compliance functionality during the initial MVP unless it is explicitly triggered by real demand**. See Section 18.

Use interfaces where they materially improve substitution/testability.

Do not create interfaces purely for abstraction theater.

The service layer owns:

```text
composition
orchestration
policy evaluation
actor/tool restrictions
workflow transitions
evidence projections
decision/risk projections
audit projections
executor dispatch
```

It does **not** become a second security kernel.

---

# 8. ACTOR MODEL

Introduce explicit actor classification outside the kernel:

```text
HUMAN
AGENT
SYSTEM
```

Maintain the distinction:

```text
ActorType != Identity != Authentication
```

ActorType answers:

> What kind of actor is this?

Principal identity answers:

> Who is this?

Authentication answers:

> How do we know?

Authentication belongs at the HTTP/API/MCP/deployment boundary.

Solvent does not need to become an identity provider.

**Never treat `actor = HUMAN` in an untrusted request body as proof of human identity.**

---

# 9. TOOL / ACTION POLICY REGISTRY

Implement a centralized policy registry.

Conceptually:

```go
type ToolPolicy struct {
    ID                   string
    RiskClass            RiskClass
    AllowedActors        []ActorType
    RequiredStage        WorkflowState
    RequiresHumanConfirm bool
    RequiresAuthority    bool
}
```

Risk classes:

```text
READ_ONLY
REVERSIBLE
IRREVERSIBLE
```

Policy must be enforced **on the path to the operation**.

Do not rely on:

```text
hidden UI buttons
disabled UI buttons
frontend state
client-side validation
```

as security.

Initial actor direction:

```text
AGENT:
    retrieve
    inspect
    submit evidence
    propose
    justify
    request action

HUMAN:
    review
    approve
    reject
    revoke
    cross explicit human-only boundaries

SYSTEM:
    deterministic orchestration
```

---

# 10. WORKFLOW SERVICE

Workflow exists above the kernel.

Workflow is process state.

Kernel state is authority truth.

Do not replace:

```text
belief lifecycle
authority lifecycle
action_intent lifecycle
```

with workflow state.

Initial workflow states:

```text
INVESTIGATING
    ↓
EVIDENCE_REVIEW
    ↓
HUMAN_REVIEW
    ↓
APPROVED
    ↓
AUTHORIZATION_READY
    ↓
EXECUTION
    ↓
COMPLETED
```

Alternative transitions:

```text
HUMAN_REVIEW → REJECTED

any non-terminal state → CANCELLED
```

Implement a centralized transition function/table:

```go
CanTransition(request) → TransitionResult
```

Every transition must consider:

```text
actor
stage
policy
current authoritative state
```

Workflow should remain deliberately small.

---

# 11. CRITICAL SECURITY RULE — TOKEN != AUTHORITY

This is non-negotiable.

A workflow token exists only for workflow continuity.

It is **not authority**.

Do not place authoritative state such as:

```text
AuthorityRef
SnapshotID
approval status
current debt
evidence state
current authority
```

into the token as a source of truth.

Use a token shape similar to:

```go
type WorkflowToken struct {
    Version    int
    ScenarioID string
    WorkflowID string
    SubjectID  string
    Stage      WorkflowState
    IssuedAt   time.Time
    ExpiresAt  *time.Time
}
```

A signed token proves:

> the token was not modified.

It does **not** prove:

> current authorization remains valid.

Correct flow:

```text
token
  ↓
identify workflow
  ↓
read current Solvent state
  ↓
evaluate current policy
  ↓
verify current authority
  ↓
execute
```

Never implement:

```text
token
  ↓
authority
  ↓
execute
```

---

# 12. TYPED TOKEN OPERATIONS

Never implement:

```go
Refresh(token, map[string]interface{})
```

or any generic arbitrary field-overwrite API.

Use typed operations such as:

```go
AdvanceStage(oldToken, newStage, actor) → newToken

AttachEvidence(oldToken, evidenceID) → newToken

CompleteReview(oldToken, decision, actor) → newToken
```

Each transition must:

1. validate preconditions
2. enforce actor restrictions
3. evaluate policy when relevant
4. generate a newly sealed token

The token service must never become a generic signed mutable-state store.

When an operation changes authoritative state, use the appropriate service/kernel path.

---

# 13. PREPARATION / REVALIDATION BOUNDARY

All consequential actions must pass through a preparation/revalidation boundary.

Conceptually:

```go
PrepareForAction(
    scenarioID,
    beliefID,
    action,
    target,
) → PreparationResult
```

It must re-read:

```text
current belief
current evidence projection
current debt
contradictions
current authority
current policy
actor/action/target relationship
```

Most importantly:

> **Preparation must re-read current authoritative state.**

Never trust:

```text
browser state
workflow token contents
agent-provided approval
cached authority
stale provider state
```

This is the principal defense against stale-state and workflow-token bypasses.

---

# 14. EVIDENCE / DECISION SUPPORT

Evidence quality belongs at the service/read-model layer.

Use statuses such as:

```text
VERIFIED
UNVERIFIED
CONFLICT
STALE
MISSING
```

Only derive them where the underlying records support them.

Never fabricate confidence.

Expose:

```text
provenance
debt
contradictions
freshness
decision findings
```

Risk/confidence scoring may exist at the product layer as decision support.

It must never create or substitute for authority.

Example:

```text
confidence = 98
authority  = absent
→ DENIED
```

versus:

```text
confidence = 42
authority  = valid
→ may be AUTHORIZED, subject to policy
```

The authority decision remains deterministic and authoritative.

---

# 15. AUDIT / ACTIVITY LEDGER

Create an append-only product activity/audit layer.

Conceptually:

```go
type Event struct {
    EventID      string
    ScenarioID   string
    Timestamp    time.Time
    Actor        string
    ActorType    ActorType
    Operation    string
    Service      string
    BeliefID     string
    IntentID     string
    Target       string
    Result       string
    AuthorityRef string
    EvidenceRef  string
    ExternalRef  string
    Metadata     map[string]string
}
```

Maintain separate facts for:

```text
Solvent authorization
adapter call
provider response
executor result
```

For example:

```text
Solvent AUTHORIZED deployment
GitHub adapter CALLED API
GitHub returned 200
Executor reports success
```

Never equate:

```text
authorization succeeded
```

with:

```text
execution succeeded
```

---

# 16. DATABASE RULES

Treat authority-core schema changes as exceptional.

Prefer:

```text
service projections
read models
product tables
configuration
policy data
```

There should be **no default workflow migration**.

Do not create:

```text
007_workflow.sql
```

unless implementation proves persistent workflow state is genuinely required.

If workflow/activity persistence is required:

> classify it explicitly as **product-layer persistence**, separate from authority-core tables.

Do not duplicate kernel authority truth.

---

# 17. EXECUTOR ARCHITECTURE

There must be one clear executor abstraction.

Do not create multiple competing Executor interfaces.

Use:

```text
ExecutionService
      ↓
Executor port
      ↓
provider-specific executor adapter
```

The executor receives a **current validated authorization decision**.

The executor must not:

```text
approve
promote
revoke
create authority
create authority-bearing evidence
```

It only executes an already authorized action.

---

# 18. COMPLIANCE / GOVERNANCE — DEFERRED, EARNED FEATURE

Compliance is part of the **long-term commercial architecture**, but substantive implementation is **not part of the initial MVP unless real demand earns it**.

Reserve:

```text
ComplianceService
```

as a clean boundary above the service layer.

Do not make compliance semantics part of the kernel.

When implementation is eventually justified, the conceptual chain is:

```text
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
```

Potential future capabilities:

```text
control-to-evidence mapping
evidence packages
immutable audit export
exception management
JSON / CSV / PDF export
framework mappings
```

But these are **not initial implementation requirements**.

### Trigger Phase 5 only when one or more of the following exists:

```text
actual compliance-oriented customer requirement
concrete buyer workflow
repeated operational demand
specific evidence/reporting requirement
material commercial blocker
```

Until then:

```text
Compliance architecture = RESERVED
Compliance implementation = DEFERRED
```

Do not build elaborate SOC 2 / ISO / NIST automation merely because it sounds commercially attractive.

Correct market language:

> **Solvent produces authoritative evidence of governed AI-driven decisions and actions.**

Do not claim:

> Solvent makes you compliant.

### Future exception model

When eventually implemented, exceptions must be governed decisions, not bypasses:

```text
OPEN
APPROVED
REJECTED
EXPIRED
REQUIRES_REVIEW
```

An exception must identify:

```text
affected control
scope
approver
validity
audit record
```

Never implement:

```text
exception=true
```

as a bypass for a kernel invariant.

---

# 19. REAL COMMERCIAL INTEGRATION

Do **not** build seven shallow integrations.

Build **one excellent integration**.

Preferred first integration:

> **GitHub / CI-CD / deployment workflow**

It should demonstrate:

```text
agent
  ↓
evidence
  ↓
human review
  ↓
authority
  ↓
exact target/action
  ↓
external execution
  ↓
audit
```

Provider-specific semantics remain in adapters.

Future candidates:

```text
Sentry
Datadog
Slack
Jira
AWS
Kubernetes
A2A
REST
```

Do not modify the kernel to support them.

---

# 20. ADAPTER ARCHITECTURE

Use clean external integration boundaries:

```go
EvidenceSource
Executor
ActivityLedger
```

Reserve but do not fully implement:

```go
DocumentGenerator // FUTURE
SignatureProvider // FUTURE
```

Adapters are allowed to understand external system semantics.

The kernel is not.

Each adapter must explicitly handle appropriate combinations of:

```text
success
provider failure
malformed provider response
timeout
degraded/demo mode
```

Use an adapter registry only when it actually simplifies dependency wiring.

Do not build a plugin framework for its own sake.

---

# 21. DOCUMENT / SIGNATURE FUNCTIONALITY

Do not implement a complete document/signature platform in the MVP.

Keep the conceptual boundary only:

```text
generation != send
draft != send
provider result != Solvent authority
```

Any future consequential send/signature operation must pass through the same authorization path as any other consequential action.

---

# 22. WEB PRODUCT

Build a **small operational console**, not a giant SaaS platform.

Primary views:

```text
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
```

A compliance view may be reserved for future work but must not force substantive compliance implementation now.

The UI must make security understandable.

Examples:

```text
DENIED — belief not promoted

DENIED — required obligation remains open

DENIED — actor not permitted

DENIED — target mismatch

DENIED — authority revoked

ALLOWED — current authority matches exact target/action
```

---

# 23. UI IS NOT SECURITY

Buttons are not security.

An operation such as:

```text
Approve
Execute
Revoke
```

must call the service layer.

The service layer must revalidate current state.

The kernel remains authoritative.

Never rely on:

```text
hidden buttons
disabled buttons
browser state
frontend workflow state
cached approval
```

for security.

---

# 24. SCENARIO / DEMO FRAMEWORK

Build one shared scenario framework.

All scenarios must use the **same service/API path**.

Do not build five mini-security systems.

Required scenarios:

```text
A. Agentjacking
B. Lying Agent
C. Stale Authorization
D. Confused Deputy
E. Legitimate Workflow
```

Every scenario must be:

```text
deterministic
resettable
independently seeded
independently testable
```

Reset only scenario-owned state.

**Never drop the entire database as a scenario reset.**

---

# 25. DEMO A — AGENTJACKING

Narrative:

```text
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
audit records result
```

Preserve the deployment boundary:

Untrusted telemetry agents must not receive direct mutation tools that let them bypass the intended workflow.

For example, untrusted telemetry agents must not be granted direct authority-changing capabilities equivalent to:

```text
retire debt
promote
falsify
approve
```

This is a **deployment/control-surface boundary**, not a reason to enlarge the kernel.

---

# 26. DEMO B — LYING AGENT

Show:

```text
agent claims approval exists
        ↓
no evidence / authority
        ↓
attempt proceeds
        ↓
DENIED
```

Then:

```text
human reviews
        ↓
legitimate approval
        ↓
current authority exists
        ↓
retry
        ↓
ALLOWED
```

Core message:

> **Agent claims are not authority.**

---

# 27. DEMO C — STALE AUTHORIZATION

Show:

```text
authority legitimately exists
        ↓
authority revoked / invalidated
        ↓
agent attempts action
        ↓
DENIED
```

Then:

```text
human creates fresh valid authority
        ↓
retry
        ↓
ALLOWED
```

The key security demonstration:

> **Current authoritative state wins over previously valid workflow state.**

---

# 28. DEMO D — CONFUSED DEPUTY

Show:

```text
authority for Target A
        ↓
action requested for Target B
        ↓
DENIED
```

Never mutate the existing authority to "correct" its target.

Correct behavior:

```text
wrong-target request
        ↓
DENIED
        ↓
new action/target request
        ↓
new authority workflow
        ↓
human approval
        ↓
new authority
        ↓
execution
```

Immutable authority remains immutable.

---

# 29. DEMO E — LEGITIMATE WORKFLOW

Show the complete commercial path:

```text
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
audit evidence available
```

This is the primary commercial proof.

---

# 30. OPEN-SOURCE "BREAK SOLVENT" SUITE

Create reproducible security challenges:

```text
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
```

Each challenge should define:

```text
attack
expected decision
reason
test
```

The repository should actively invite engineers to try to defeat the authority model.

---

# 31. SECURITY TESTING

Test at:

```text
KERNEL
SERVICE
ADAPTER
END-TO-END
```

Mandatory bypass tests:

```text
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
```

---

# 32. TOKEN SECURITY TESTS

Explicitly test:

```text
tampered token rejected
invalid signature rejected
expired token rejected where configured
invalid stage transition rejected
token cannot carry authority as truth
token cannot bypass current revocation
token cannot bypass current target binding
token cannot bypass current policy
```

Most important adversarial test:

```text
valid signed stale token
+
revoked current authority
=
DENIED
```

This test is mandatory.

---

# 33. FORMAL VERIFICATION

Do not enlarge the Lean model merely because the product expands.

Lean should continue to model the security-critical kernel state machine.

Only change Lean when kernel semantics change.

Desired architecture:

```text
small formal kernel model
+
broad service/product layer
```

---

# 34. OBSERVABILITY

Make important security and operational facts visible:

```text
authorization allowed
authorization denied
human approval
revocation
stale authority attempt
policy denial
adapter failure
executor failure
```

Always preserve the distinction between:

```text
authorization succeeded + execution failed
```

and:

```text
authorization denied + execution never happened
```

These are different operational facts.

---

# 35. COMMERCIAL UX

The product must look like an operational system, not a research demonstration.

A buyer/operator should be able to inspect:

```text
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
```

The same event should be explainable at three levels:

```text
EXECUTIVE:
"Blocked because no authorized approval exists."

SECURITY:
"Agent principal lacked a valid authority tuple for target X."

ENGINEERING:
"Current kernel authorization check returned DENIED."
```

---

# 36. POSITIONING

Do not turn Solvent into:

```text
generic AI firewall
GRC platform
IAM replacement
generic policy engine
workflow SaaS
agent framework
```

Instead demonstrate:

```text
governed autonomy
+
authoritative evidence
+
explicit human decisions
+
machine-enforced authority
+
exact action/target binding
+
external execution boundaries
+
auditable outcomes
```

Core commercial message:

> **Solvent provides the authority layer between autonomous systems and consequential actions.**

---

# 37. WHAT NOT TO BUILD NOW

Explicitly defer:

```text
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
complicated compliance-framework automation
```

Also defer substantive compliance implementation unless the Phase 5 trigger conditions in Section 18 are met.

---

# 38. IMPLEMENTATION ORDER

## PHASE 0 — RECONNAISSANCE

Implement:

```text
repository inspection
baseline tests
architecture confirmation
implementation map
```

No speculative refactoring.

---

## PHASE 1 — SERVICE BOUNDARIES

Implement/consolidate:

```text
WorkflowService
PolicyService
EvidenceService
AuthorityService
AuditService
```

Establish clean service boundaries without duplicating kernel semantics.

---

## PHASE 2 — SECURITY BOUNDARIES

Implement:

```text
preparation/revalidation boundary
human authorization boundary
typed workflow token
current-authority revalidation
single executor contract
```

Make the security rules mechanically enforceable.

---

## PHASE 3 — ONE REAL INTEGRATION

Implement:

```text
GitHub / CI-CD adapter
executor
activity recording
```

Prefer one production-quality workflow over integration breadth.

---

## PHASE 4 — PRODUCT UI

Implement:

```text
Overview
Review Queue
Authority / Decision Detail
Audit / Activity
Integrations
```

Do not let frontend state become a parallel authority mechanism.

Reserve the compliance view architecturally, but do not implement substantive compliance functionality solely to complete a checklist.

---

## PHASE 5 — COMPLIANCE / GOVERNANCE

### STATUS: DEFERRED — EARNED FEATURE

Do **not** implement this phase automatically.

Trigger it only when there is:

```text
real buyer demand
actual compliance-oriented workflow
repeated customer operational need
specific evidence/reporting requirement
material sales requirement
```

When triggered, implement:

```text
control mapping
evidence packages
audit exports
exception management
```

without changing the kernel.

---

## PHASE 6 — DEMO PLATFORM

Implement:

```text
Agentjacking
Lying Agent
Stale Authorization
Confused Deputy
Legitimate Workflow
```

All must share the same service/kernel path.

---

## PHASE 7 — OPEN SOURCE POLISH

Implement:

```text
challenge suite
adapter examples
architecture documentation
security documentation
quickstart
developer guide
```

A developer should be able to add:

```text
adapter
executor
policy
demo scenario
```

without modifying the kernel.

---

# 39. ACCEPTANCE CRITERIA

## Architecture

```text
[ ] kernel remains small and domain-generic
[ ] existing authority semantics preserved
[ ] MCP remains thin
[ ] external systems remain adapters
[ ] execution remains outside kernel
[ ] service layer owns orchestration/policy
[ ] compliance remains outside kernel
[ ] no competing authority engine exists
```

## Security

```text
[ ] actor classification exists
[ ] actor type is separate from authentication
[ ] human-only operations are mechanically gated
[ ] agent claims cannot become authority
[ ] workflow tokens cannot become authority
[ ] signed stale tokens cannot bypass current authority
[ ] wrong targets are denied
[ ] revoked authority is denied
[ ] browser state cannot create authority
[ ] executors cannot create authority
[ ] external providers cannot create authority
```

## Product

```text
[ ] operator can inspect pending decisions
[ ] operator can inspect evidence
[ ] operator can understand why an action is blocked
[ ] operator can review legitimate work
[ ] operator can inspect exact authority
[ ] operator can inspect audit trail
[ ] operator can inspect execution outcome
```

## Compliance architecture

```text
[ ] ComplianceService boundary exists conceptually
[ ] no compliance semantics enter kernel
[ ] substantive compliance implementation remains deferred unless earned
```

## Demos

```text
[ ] Agentjacking works
[ ] Lying Agent works
[ ] Stale Authorization works
[ ] Confused Deputy works
[ ] Legitimate Workflow works
[ ] scenarios reset independently
```

## Open Source

```text
[ ] new adapter requires no kernel modification
[ ] new executor requires no kernel modification
[ ] new policy requires no kernel modification
[ ] new scenario uses common services
[ ] security challenges are reproducible
```

## Quality

```text
[ ] existing tests pass
[ ] service tests pass
[ ] security bypass tests pass
[ ] end-to-end tests pass
[ ] Lean verification remains clean
[ ] no unexplained kernel growth
[ ] no unexplained authority-core schema growth
[ ] documentation matches actual implementation
```

---

# 40. REQUIRED DELIVERABLES

Produce:

```text
implementation code
architecture documentation
kernel/schema ADRs where genuinely required
commercial MVP README
security model
workflow documentation
tool/policy registry documentation
adapter development guide
executor guide
compliance architecture note
demo walkthrough
challenge-suite documentation
test report
"What is Solvent?" buyer document
"Why is Solvent different?" security-engineer document
final implementation summary
```

The final implementation summary must explicitly state:

```text
files added
files changed
migrations added
kernel changes
reason for every kernel change
service additions
adapter additions
executor additions
UI additions
compliance additions, if any
demo additions
test coverage
remaining limitations
```

If compliance was not implemented, explicitly state:

```text
Compliance Phase 5 deferred.
Reason:
no sufficiently concrete earned requirement yet.
Architecture boundary reserved.
```

---

# 41. FINAL ENGINEERING PRINCIPLES

These rules govern all implementation decisions:

```text
1. SMALL TRUSTED CORE
   Keep the kernel minimal.

2. CURRENT AUTHORITY WINS
   Cached or signed workflow state never substitutes for current authority.

3. AUTHORIZE != EXECUTE
   Authorization and execution are separate.

4. ACTOR != IDENTITY
   Actor classification does not authenticate callers.

5. EVIDENCE != AUTHORITY
   Evidence may support authority but never becomes authority.

6. POLICY != KERNEL
   Customer policy belongs outside the kernel.

7. WORKFLOW != AUTHORITY
   Workflow coordinates process; Solvent establishes authority.

8. COMPLIANCE != KERNEL
   Compliance consumes Solvent facts and never redefines them.

9. ADAPTERS KNOW EXTERNAL SYSTEMS
   The kernel does not.

10. ONE SOURCE OF AUTHORITY TRUTH
    Never duplicate authority in tokens, browser state,
    workflow objects, or external systems.

11. MAKE SECURITY VISIBLE
    Always explain why an action was allowed or denied.

12. BUILD ONE EXCELLENT REAL WORKFLOW
    Prefer depth over integration count.

13. DEMOS SHARE THE SAME CORE
    Different attacks, same authority semantics.

14. OPEN SOURCE SHOULD BE EXTENSIBLE
    Developers should add integrations without modifying the kernel.

15. COMPLEXITY MUST BE EARNED
    Expand the trusted core only because of real customer demand,
    operational pain, observed security failure, or a concrete
    security requirement.
```

---

# 42. FINAL OBJECTIVE

Do not ask:

> "What features can we add?"

Ask:

> "What is the smallest architecture that makes this product commercially compelling while preserving the authority kernel as the trusted core?"

The finished system must create this impression:

```text
BUYER:
"This governs what our AI systems are actually allowed to do."

SECURITY ENGINEER:
"The authority decision is deterministic, transactional,
and backed by a small trusted core."

DEVELOPER:
"I can integrate without modifying the kernel."

COMPLIANCE TEAM:
"When we need it, governed decisions can be traced
from evidence through authority to execution."

OPEN-SOURCE CONTRIBUTOR:
"I can extend the ecosystem without creating another
security mechanism."
```

Optimize for:

```text
small trusted core
clear boundaries
current-state authority verification
commercially useful workflow
visible security guarantees
one real integration
excellent demonstrations
easy extension
maintainability
open-source credibility
```

And above all:

> **Build the product around the kernel. Do not turn the product into the kernel.**

