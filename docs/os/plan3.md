# Solvent — Consolidated Implementation Plan

**Date:** 2026-09-04  
**Status:** APPROVED  
**Scope:** Implementation contract for the next coding-agent pass.

---

## 0. Product Thesis

Solvent is:

```
A portable authority layer for autonomous systems.
```

Core distinctions (non-negotiable):

```
Evidence is not authority.
Agent output is not authority.
Retrieval is not authority.
Workflow state is not authority.
A workflow token is not authority.
Authorization is not execution.

An action is allowed only when current authoritative Solvent state
establishes the required authority.
```

Intended product perception:

```
BUYER:        "This governs what our AI systems are actually allowed to do."
SECURITY:     "The authority decision is backed by a small, transactionally
               enforced core rather than LLM behavior."
DEVELOPER:    "I can integrate my system without modifying that core."
COMPLIANCE:   "I can trace controls and evidence to actual governed actions."
OPEN SOURCE:  "I can extend Solvent without turning every integration
               into another security mechanism."
```

---

## 1. Absolute Architectural Rule

**KEEP THE KERNEL SMALL.**

The kernel is the smallest trusted authority core. Default: **DO NOT MODIFY THE KERNEL.**

Decision tree for every requested feature:

```
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

External concepts that must remain outside the kernel: Sentry, Agentjacking, GitHub, MCP-specific behavior, REST, A2A, procurement, incident response, compliance frameworks, risk scoring, document generation, e-signature, Kubernetes, AWS, Cloudflare, Slack, Jira, customer-specific workflows.

---

## 2. Trust Boundaries

```
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

```
Solvent authorization
      ↓
Executor / Deployment adapter
      ↓
Real-world system
```

Never collapse authorization, execution, workflow continuity, actor identity, authentication, or evidence into one concept.

---

## 3. Phase 0 — Reconnaissance

Before modifying any code:

1. Inspect the complete repository
2. Determine the actual architecture from implementation
3. Identify: kernel packages, service/application packages, belief lifecycle, authority lifecycle, action_intent lifecycle, DB schema/migrations, MCP tools, HTTP/web server, wizard/demo, adapters, views/read models, Lean model, audit/refusal mechanisms, tests, task/build commands, deployment configuration
4. Run all existing tests and verification gates
5. Identify code that can be wrapped or reused rather than duplicated
6. Produce a concise implementation map before editing

Do not begin architectural refactoring based solely on the plan. Confirm what actually exists.

---

## 4. Preserve Current Kernel Semantics

Preserve the existing authority model and its DB semantics:

```
principal
authority_target
target_snapshot
target_activation
target_revocation
justification
debt_discharge
```

Preserve: `Approve`, `Authorize`, `RevokeTarget`

Preserve: immutable target snapshots, exact target binding, activation uniqueness, debt/promotion gates, transactional authority creation, read-only authorization verification, DB-enforced invariants, Lean formal verification.

Do not create a second authority engine in the service layer.

---

## 5. Kernel Growth Gate

Before changing kernel code or authority-core schema, create an ADR documenting:

```
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

Default decision: **REJECT THE KERNEL CHANGE** unless the ADR demonstrates that existing kernel primitives are fundamentally insufficient.

Every unexplained kernel or authority-schema expansion is a defect.

---

## 6. Service Architecture

Build or consolidate these service boundaries:

```
WorkflowService
PolicyService
EvidenceService
AuthorityService
AuditService
IntegrationService
ExecutionService
```

Reserve `ComplianceService` as a product architecture boundary, but **do not implement substantive compliance functionality during the initial MVP unless it is explicitly triggered by real demand**. See Section 18.

Use interfaces where they materially improve substitution/testability. Do not create interfaces purely for abstraction theater.

The service layer owns: composition, orchestration, policy evaluation, actor/tool restrictions, workflow transitions, evidence projections, decision/risk projections, audit projections, executor dispatch.

It does **not** become a second security kernel.

---

## 7. Actor Model

Introduce explicit actor classification outside the kernel:

```
HUMAN
AGENT
SYSTEM
```

Maintain the distinction:

```
ActorType != Identity != Authentication
```

ActorType answers: What kind of actor is this?  
Principal identity answers: Who is this?  
Authentication answers: How do we know?

Authentication belongs at the HTTP/API/MCP/deployment boundary. Solvent does not need to become an identity provider.

**Never treat `actor = HUMAN` in an untrusted request body as proof of human identity.**

---

## 8. Tool / Action Policy Registry

Implement a centralized policy registry:

```go
type ToolPolicy struct {
    ID                   string
    RiskClass            RiskClass      // READ_ONLY, REVERSIBLE, IRREVERSIBLE
    AllowedActors        []ActorType
    RequiredStage        WorkflowState
    RequiresHumanConfirm bool
    RequiresAuthority    bool
}
```

Policy must be enforced **on the path to the operation**. Do not rely on hidden UI buttons, disabled UI buttons, frontend state, or client-side validation as security.

Initial actor direction:
- AGENT: retrieve, inspect, submit evidence, propose, justify, request action
- HUMAN: review, approve, reject, revoke, cross explicit human-only boundaries
- SYSTEM: deterministic orchestration

---

## 9. Workflow Service

Workflow exists above the kernel. Workflow is process state. Kernel state is authority truth.

Do not replace belief lifecycle, authority lifecycle, or action_intent lifecycle with workflow state.

Initial workflow states:

```
INVESTIGATING → EVIDENCE_REVIEW → HUMAN_REVIEW → APPROVED
    → AUTHORIZATION_READY → EXECUTION → COMPLETED

HUMAN_REVIEW → REJECTED
any non-terminal state → CANCELLED
```

Implement a centralized transition function: `CanTransition(request) → TransitionResult`

Every transition must consider: actor, stage, policy, current authoritative state.

---

## 10. CRITICAL SECURITY RULE — TOKEN ≠ AUTHORITY

**THIS IS NON-NEGOTIABLE.**

A workflow token exists only for workflow continuity. It is **not authority**.

Do not place authoritative state such as `AuthorityRef`, `SnapshotID`, approval status, current debt, evidence state, or current authority into the token as a source of truth.

Token shape:

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

A signed token proves the token was not modified. It does **not** prove current authorization remains valid.

Correct flow:

```
token → identify workflow → read current Solvent state
    → evaluate current policy → verify current authority → execute
```

Never implement:

```
token → authority → execute
```

---

## 11. Typed Token Operations

Never implement `Refresh(token, map[string]interface{})` or any generic arbitrary field-overwrite API.

Use typed operations:

```go
AdvanceStage(oldToken, newStage, actor) → newToken
AttachEvidence(oldToken, evidenceID) → newToken
CompleteReview(oldToken, decision, actor) → newToken
```

Each transition must: validate preconditions, enforce actor restrictions, evaluate policy when relevant, generate a newly sealed token.

The token service must never become a generic signed mutable-state store.

When an operation changes authoritative state, use the appropriate service/kernel path.

---

## 12. Preparation / Revalidation Boundary

All consequential actions must pass through a preparation/revalidation boundary:

```go
PrepareForAction(scenarioID, beliefID, action, target) → PreparationResult
```

It must re-read: current belief, current evidence projection, current debt, contradictions, current authority, current policy, actor/action/target relationship.

Most importantly: **Preparation must re-read current authoritative state.**

Never trust: browser state, workflow token contents, agent-provided approval, cached authority, stale provider state.

This is the principal defense against stale-state and workflow-token bypasses.

---

## 13. Evidence / Decision Support

Evidence quality belongs at the service/read-model layer.

Use statuses: `VERIFIED`, `UNVERIFIED`, `CONFLICT`, `STALE`, `MISSING`

Only derive them where the underlying records support them. Never fabricate confidence.

Expose: provenance, debt, contradictions, freshness, decision findings.

Risk/confidence scoring may exist at the product layer as decision support. It must never create or substitute for authority.

```
confidence = 98, authority = absent  → DENIED
confidence = 42, authority = valid   → may be AUTHORIZED, subject to policy
```

---

## 14. Audit / Activity Ledger

Create an append-only product activity/audit layer:

```go
type Event struct {
    EventID, ScenarioID    string
    Timestamp              time.Time
    Actor, ActorType       string
    Operation, Service     string
    BeliefID, IntentID     string
    Target, Result         string
    AuthorityRef           string
    EvidenceRef            string
    ExternalRef            string
    Metadata               map[string]string
}
```

Maintain separate facts for: Solvent authorization, adapter call, provider response, executor result.

Never equate "authorization succeeded" with "execution succeeded."

---

## 15. Database Rules

Treat authority-core schema changes as exceptional.

Prefer: service projections, read models, product tables, configuration, policy data.

**There should be no default workflow migration.** Do not create `007_workflow.sql` unless implementation proves persistent workflow state is genuinely required.

If workflow/activity persistence is required, classify it explicitly as product-layer persistence, separate from authority-core tables.

Do not duplicate kernel authority truth.

---

## 16. Executor Architecture

There must be one clear executor abstraction. Do not create multiple competing Executor interfaces.

```
ExecutionService → Executor port → provider-specific executor adapter
```

The executor receives a current validated authorization decision. The executor must not: approve, promote, revoke, create authority, create authority-bearing evidence. It only executes an already authorized action.

---

## 17. Real Commercial Integration

Do **not** build seven shallow integrations. Build **one excellent integration**.

Preferred first integration: **GitHub / CI-CD / deployment workflow**

It should demonstrate: agent → evidence → human review → authority → exact target/action → external execution → audit.

Future candidates: Sentry, Datadog, Slack, Jira, AWS, Kubernetes, A2A, REST. Do not modify the kernel to support them.

---

## 18. Adapter Architecture

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

Adapters are allowed to understand external system semantics. The kernel is not.

Each adapter must explicitly handle: success, provider failure, malformed provider response, timeout, degraded/demo mode.

Use an adapter registry only when it actually simplifies dependency wiring. Do not build a plugin framework for its own sake.

---

## 19. Document / Signature Functionality

Do not implement a complete document/signature platform in the MVP.

Keep the conceptual boundary only: `generation ≠ send`, `draft ≠ send`, `provider result ≠ Solvent authority`.

Any future consequential send/signature operation must pass through the same authorization path as any other consequential action.

---

## 20. Web Product

Build a **small operational console**, not a giant SaaS platform.

Primary views:

```
1. OVERVIEW
   workflows, pending reviews, blocked actions, recent decisions

2. REVIEW QUEUE
   belief, evidence, debt, contradictions, requested action, target, actor

3. AUTHORITY / DECISION DETAIL
   evidence, provenance, policy, actor, authority, exact target, reason for allow/deny

4. AUDIT / ACTIVITY
   append-only timeline, filters, authority events, external events

5. INTEGRATIONS
   providers, mode, health, recent calls, failures
```

A compliance view may be reserved for future work but must not force substantive compliance implementation now.

The UI must make security understandable:

```
DENIED — belief not promoted
DENIED — required obligation remains open
DENIED — actor not permitted
DENIED — target mismatch
DENIED — authority revoked
ALLOWED — current authority matches exact target/action
```

---

## 21. UI Is Not Security

Buttons are not security. An operation such as Approve, Execute, or Revoke must call the service layer. The service layer must revalidate current state. The kernel remains authoritative.

Never rely on hidden buttons, disabled buttons, browser state, frontend workflow state, or cached approval for security.

---

## 22. Scenario / Demo Framework

Build one shared scenario framework. All scenarios must use the **same service/API path**. Do not build five mini-security systems.

Required scenarios:

```
A. Agentjacking
B. Lying Agent
C. Stale Authorization
D. Confused Deputy
E. Legitimate Workflow
```

Every scenario must be: deterministic, resettable, independently seeded, independently testable.

Reset only scenario-owned state. **Never drop the entire database as a scenario reset.**

---

## 23. Demo A — Agentjacking

```
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

Preserve the deployment boundary: untrusted telemetry agents must not receive direct mutation tools that let them bypass the intended workflow. This is a deployment/control-surface boundary, not a reason to enlarge the kernel.

---

## 24. Demo B — Lying Agent

```
agent claims approval exists
    ↓
no evidence / authority
    ↓
attempt proceeds
    ↓
DENIED

then:

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

Core message: **Agent claims are not authority.**

---

## 25. Demo C — Stale Authorization

```
authority legitimately exists
    ↓
authority revoked / invalidated
    ↓
agent attempts action
    ↓
DENIED

then:

human creates fresh valid authority
    ↓
retry
    ↓
ALLOWED
```

Key security demonstration: **Current authoritative state wins over previously valid workflow state.**

---

## 26. Demo D — Confused Deputy

```
authority for Target A
    ↓
action requested for Target B
    ↓
DENIED
```

Never mutate the existing authority to "correct" its target. Correct behavior:

```
wrong-target request → DENIED

then:

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

## 27. Demo E — Legitimate Workflow

Show the complete commercial path:

```
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

## 28. Open-Source "Break Solvent" Suite

Create reproducible security challenges:

```
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

Each challenge should define: attack, expected decision, reason, test.

The repository should actively invite engineers to try to defeat the authority model.

---

## 29. Security Testing

Test at four levels: KERNEL, SERVICE, ADAPTER, END-TO-END.

Mandatory bypass tests:

```
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

## 30. Token Security Tests

Explicitly test:

```
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

```
valid signed stale token + revoked current authority = DENIED
```

This test is mandatory.

---

## 31. Compliance / Governance — DEFERRED, EARNED FEATURE

Compliance is part of the **long-term commercial architecture**, but substantive implementation is **not part of the initial MVP unless real demand earns it**.

Reserve `ComplianceService` as a clean boundary above the service layer.

Do not make compliance semantics part of the kernel.

When implementation is eventually justified, the conceptual chain is:

```
CONTROL → POLICY → REQUIRED EVIDENCE → SOLVENT DECISION
    → AUTHORITY → ACTION → EXECUTION → AUDIT
```

### Trigger Phase 5 only when one or more of the following exists:

```
actual compliance-oriented customer requirement
concrete buyer workflow
repeated operational demand
specific evidence/reporting requirement
material commercial blocker
```

Until then:

```
Compliance architecture = RESERVED
Compliance implementation = DEFERRED
```

Do not build elaborate SOC 2 / ISO / NIST automation merely because it sounds commercially attractive.

Correct market language:

> **Solvent produces authoritative evidence of governed AI-driven decisions and actions.**

Do not claim: "Solvent makes you compliant."

### Future exception model

When eventually implemented, exceptions must be governed decisions, not bypasses:

```
OPEN → APPROVED / REJECTED / EXPIRED / REQUIRES_REVIEW
```

An exception must identify: affected control, scope, approver, validity, audit record.

Never implement `exception=true` as a bypass for a kernel invariant.

---

## 32. Formal Verification

Do not enlarge the Lean model merely because the product expands. Lean should continue to model the security-critical kernel state machine. Only change Lean when kernel semantics change.

Desired architecture: small formal kernel model + broad service/product layer.

---

## 33. Observability

Make important security and operational facts visible:

```
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
- authorization succeeded + execution failed
- authorization denied + execution never happened

These are different operational facts.

---

## 34. Commercial UX

The product must look like an operational system, not a research demonstration.

A buyer/operator should be able to inspect: current workflow, pending decision, evidence, authority, action, target, actor, policy, result, audit trail.

The same event should be explainable at three levels:

```
EXECUTIVE:     "Blocked because no authorized approval exists."
SECURITY:      "Agent principal lacked a valid authority tuple for target X."
ENGINEERING:   "Current kernel authorization check returned DENIED."
```

---

## 35. Positioning

Do not turn Solvent into: generic AI firewall, GRC platform, IAM replacement, generic policy engine, workflow SaaS, agent framework.

Instead demonstrate: governed autonomy + authoritative evidence + explicit human decisions + machine-enforced authority + exact action/target binding + external execution boundaries + auditable outcomes.

Core commercial message:

> **Solvent provides the authority layer between autonomous systems and consequential actions.**

---

## 36. What Not to Build Now

Explicitly defer:

```
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

Also defer substantive compliance implementation unless the Phase 5 trigger conditions in Section 31 are met.

---

## 37. Implementation Order

### PHASE 0 — RECONNAISSANCE

```
repository inspection
baseline tests
architecture confirmation
implementation map
```

No speculative refactoring.

### PHASE 1 — SERVICE BOUNDARIES

```
WorkflowService
PolicyService
EvidenceService
AuthorityService
AuditService
```

Establish clean service boundaries without duplicating kernel semantics.

### PHASE 2 — SECURITY BOUNDARIES

```
preparation/revalidation boundary
human authorization boundary
typed workflow token
current-authority revalidation
single executor contract
```

Make the security rules mechanically enforceable.

### PHASE 3 — ONE REAL INTEGRATION

```
GitHub / CI-CD adapter
executor
activity recording
```

Prefer one production-quality workflow over integration breadth.

### PHASE 4 — PRODUCT UI

```
Overview
Review Queue
Authority / Decision Detail
Audit / Activity
Integrations
```

Do not let frontend state become a parallel authority mechanism. Reserve the compliance view architecturally, but do not implement substantive compliance functionality solely to complete a checklist.

### PHASE 5 — COMPLIANCE / GOVERNANCE

**STATUS: DEFERRED — EARNED FEATURE**

Do **not** implement this phase automatically. Trigger it only when there is: real buyer demand, actual compliance-oriented workflow, repeated customer operational need, specific evidence/reporting requirement, material sales requirement.

When triggered, implement: control mapping, evidence packages, audit exports, exception management — without changing the kernel.

### PHASE 6 — DEMO PLATFORM

```
Agentjacking
Lying Agent
Stale Authorization
Confused Deputy
Legitimate Workflow
```

All must share the same service/kernel path.

### PHASE 7 — OPEN SOURCE POLISH

```
challenge suite
adapter examples
architecture documentation
security documentation
quickstart
developer guide
```

A developer should be able to add an adapter, executor, policy, or demo scenario without modifying the kernel.

---

## 38. Acceptance Criteria

### Architecture
- [ ] Kernel remains small and domain-generic
- [ ] Existing authority semantics preserved
- [ ] MCP remains thin
- [ ] External systems remain adapters
- [ ] Execution remains outside kernel
- [ ] Service layer owns orchestration/policy
- [ ] Compliance remains outside kernel
- [ ] No competing authority engine exists

### Security
- [ ] Actor classification exists
- [ ] Actor type is separate from authentication
- [ ] Human-only operations are mechanically gated
- [ ] Agent claims cannot become authority
- [ ] Workflow tokens cannot become authority
- [ ] Signed stale tokens cannot bypass current authority
- [ ] Wrong targets are denied
- [ ] Revoked authority is denied
- [ ] Browser state cannot create authority
- [ ] Executors cannot create authority
- [ ] External providers cannot create authority

### Product
- [ ] Operator can inspect pending decisions
- [ ] Operator can inspect evidence
- [ ] Operator can understand why an action is blocked
- [ ] Operator can review legitimate work
- [ ] Operator can inspect exact authority
- [ ] Operator can inspect audit trail
- [ ] Operator can inspect execution outcome

### Compliance Architecture
- [ ] ComplianceService boundary exists conceptually
- [ ] No compliance semantics enter kernel
- [ ] Substantive compliance implementation remains deferred unless earned

### Demos
- [ ] Agentjacking works
- [ ] Lying Agent works
- [ ] Stale Authorization works
- [ ] Confused Deputy works
- [ ] Legitimate Workflow works
- [ ] Scenarios reset independently

### Open Source
- [ ] New adapter requires no kernel modification
- [ ] New executor requires no kernel modification
- [ ] New policy requires no kernel modification
- [ ] New scenario uses common services
- [ ] Security challenges are reproducible

### Quality
- [ ] Existing tests pass
- [ ] Service tests pass
- [ ] Security bypass tests pass
- [ ] End-to-end tests pass
- [ ] Lean verification remains clean
- [ ] No unexplained kernel growth
- [ ] No unexplained authority-core schema growth
- [ ] Documentation matches actual implementation

---

## 39. Required Deliverables

1. Implementation code
2. Architecture documentation
3. Kernel/schema ADRs where genuinely required
4. Commercial MVP README
5. Security model
6. Workflow documentation
7. Tool/policy registry documentation
8. Adapter development guide
9. Executor guide
10. Compliance architecture note
11. Demo walkthrough
12. Challenge-suite documentation
13. Test report
14. "What is Solvent?" buyer document
15. "Why is Solvent different?" security-engineer document
16. Final implementation summary

The final implementation summary must explicitly state: files added, files changed, migrations added, kernel changes, reason for every kernel change, service additions, adapter additions, executor additions, UI additions, compliance additions (if any), demo additions, test coverage, remaining limitations.

If compliance was not implemented, explicitly state:

```
Compliance Phase 5 deferred.
Reason: no sufficiently concrete earned requirement yet.
Architecture boundary reserved.
```

---

## 40. Final Engineering Principles

```
1.  SMALL TRUSTED CORE
    Keep the kernel minimal.

2.  CURRENT AUTHORITY WINS
    Cached or signed workflow state never substitutes for current authority.

3.  AUTHORIZE != EXECUTE
    Authorization and execution are separate.

4.  ACTOR != IDENTITY
    Actor classification does not authenticate callers.

5.  EVIDENCE != AUTHORITY
    Evidence may support authority but never becomes authority.

6.  POLICY != KERNEL
    Customer policy belongs outside the kernel.

7.  WORKFLOW != AUTHORITY
    Workflow coordinates process; Solvent establishes authority.

8.  COMPLIANCE != KERNEL
    Compliance consumes Solvent facts and never redefines them.

9.  ADAPTERS KNOW EXTERNAL SYSTEMS
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

## 41. Final Objective

Do not ask: "What features can we add?"

Ask: "What is the smallest architecture that makes this product commercially compelling while preserving the authority kernel as the trusted core?"

```
BUYER:           "This governs what our AI systems are actually allowed to do."
SECURITY:        "The authority decision is deterministic, transactional,
                  and backed by a small trusted core."
DEVELOPER:       "I can integrate without modifying the kernel."
COMPLIANCE:      "When we need it, governed decisions can be traced
                  from evidence through authority to execution."
OPEN SOURCE:     "I can extend the ecosystem without creating another
                  security mechanism."
```

Optimize for: small trusted core, clear boundaries, current-state authority verification, commercially useful workflow, visible security guarantees, one real integration, excellent demonstrations, easy extension, maintainability, open-source credibility.

**Build the product around the kernel. Do not turn the product into the kernel.**
