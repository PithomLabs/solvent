# Solvent Commercial MVP — Consolidated Implementation Plan

**Date:** 2026-09-04  
**Status:** PLANNING  
**Scope:** Consolidated implementation contract for the next coding-agent pass.

---

## 0. Product Thesis

Solvent is:

```
A portable authority layer for autonomous systems.
```

Core distinction:

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

The market should see a practical product for governing consequential AI-driven actions. The security engineer should see a small, deterministic, auditable authority kernel. The developer should see clean extension points that normally require no kernel changes. The compliance team should see traceable evidence connecting policy/control → evidence → decision → authority → action → execution outcome.

---

## 1. Non-Negotiable Architectural Principle

**KEEP THE KERNEL SMALL.**

The kernel is the smallest trusted authority core. Default: **DO NOT MODIFY THE KERNEL.**

Decision tree for every requested capability:

```
Does it understand an external product/protocol?
    → ADAPTER

Is it workflow, orchestration, composition, or policy?
    → SERVICE / POLICY

Is it execution or infrastructure?
    → EXECUTOR / DEPLOYMENT

Is it customer-specific behavior?
    → POLICY / CONFIGURATION / DATA

Is it reporting, UI, analytics, scoring, audit presentation?
    → PRODUCT / READ MODEL / SERVICE

Does it require a new security-critical durable fact
or atomic state transition that cannot be expressed using
existing kernel primitives?
    → POSSIBLE KERNEL CHANGE

Must a previously possible invalid state become
structurally impossible?
    → POSSIBLE DB INVARIANT

Otherwise:
    → DO NOT ADD IT
```

Examples that MUST remain outside the kernel: Sentry, Agentjacking, GitHub, MCP-specific behavior, REST, A2A, procurement, incident response, compliance frameworks, risk scoring, document generation, e-signature, Kubernetes, AWS, Cloudflare, Slack, Jira, customer-specific workflows.

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

Never collapse authorization, execution, workflow continuity, actor identity, or evidence into one concept.

---

## 3. Reconnaissance Before Coding

Before changing code:

1. Inspect the complete repository
2. Determine the actual current architecture from code
3. Identify: kernel packages, service/application packages, belief lifecycle, authority lifecycle, action_intent lifecycle, DB schema/migrations, MCP tools, HTTP/web server, wizard/demo, adapters, views/projections, Lean model, audit/refusal mechanisms, test harness, task commands, deployment configuration
4. Run all existing test and verification gates
5. Do not assume documentation matches implementation
6. Produce a short implementation map before editing
7. Identify existing functionality that should be wrapped/reused rather than duplicated

---

## 4. Preserve Current Solvent Authority Semantics

Preserve the existing authority model and its DB semantics:

- `principal`, `authority_target`, `target_snapshot`, `target_activation`, `target_revocation`, `justification`, `debt_discharge`
- `Approve`, `Authorize`, `RevokeTarget`
- Snapshot immutability, exact target binding, activation uniqueness
- Debt/promotion gates, transactional authority creation
- Read-only authorization verification
- DB-enforced invariants, formal verification

Do not create a competing authority engine in the service layer.

---

## 5. Kernel Growth Gate

Before ANY kernel or authority-core schema change, create an ADR with:

- Problem
- Why adapter cannot solve it
- Why service/policy cannot solve it
- Why configuration cannot solve it
- New durable security fact
- New atomic state transition
- Required DB invariant
- Impact on current authority semantics
- Impact on Lean verification
- Migration strategy
- Backward compatibility
- Security argument

Default decision: **REJECT KERNEL CHANGE** unless the ADR demonstrates that current kernel primitives are fundamentally insufficient.

---

## 6. Product MVP Workflow

```
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
```

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

---

## 7. Service Layer

Create or consolidate a clean service boundary:

```
WorkflowService
PolicyService
EvidenceService
AuthorityService
AuditService
ComplianceService
IntegrationService
ExecutionService
```

Use interfaces where they improve testability and substitution. Do not create interfaces merely for abstraction theater.

The service layer owns: composition, orchestration, policy evaluation, actor/tool restrictions, workflow transitions, evidence projections, risk/decision projections, audit projections, compliance mappings, executor dispatch.

The service layer does NOT become a second security kernel.

---

## 8. Actor Model

Introduce explicit actor classification outside the kernel:

```
HUMAN
AGENT
SYSTEM
```

Important distinction: **ActorType ≠ authenticated identity**

ActorType answers: what kind of actor is this?  
Principal identity answers: who is this?  
Authentication answers: how do we know?

Current Solvent does not need to become an identity provider. Authentication belongs at HTTP/API/MCP/deployment boundary. The service layer consumes trusted principal context from that boundary.

**NEVER treat `actor = HUMAN` provided by an untrusted request body as proof of human identity.**

---

## 9. Tool / Action Policy Registry

Implement a centralized tool/action policy registry:

```go
ToolPolicy {
    ID
    RiskClass          // READ_ONLY, REVERSIBLE, IRREVERSIBLE
    AllowedActors      // []ActorType
    RequiredStage      // WorkflowState
    RequiresHumanConfirm bool
    RequiresAuthority  bool
}
```

Use a central registry, not scattered checks. Policy must be enforced ON THE PATH TO THE OPERATION. Do not rely on UI hiding buttons.

Initial direction:
- AGENT may: retrieve, inspect, submit evidence, propose, justify, request action
- HUMAN may: review, approve, reject, revoke, cross explicitly human-only boundaries
- SYSTEM may: perform deterministic orchestration

---

## 10. Workflow Service

Create a small workflow abstraction ABOVE the kernel. Do not replace belief lifecycle, authority lifecycle, or action_intent lifecycle with workflow state. Workflow is process state. Kernel state remains authority truth.

Initial workflow states:

```
INVESTIGATING → EVIDENCE_REVIEW → HUMAN_REVIEW → APPROVED
    → AUTHORIZATION_READY → EXECUTION → COMPLETED

REJECTED (from HUMAN_REVIEW)
CANCELLED (from any non-terminal state)
```

Use a centralized transition table: `CanTransition(request) → TransitionResult`

Enforce: actor restrictions, stage requirements, policy, current authoritative state.

---

## 11. CRITICAL RULE — TOKEN ≠ AUTHORITY

**THIS IS NON-NEGOTIABLE.**

A workflow token is workflow continuity only. It is NOT authority.

DO NOT place authoritative state such as `AuthorityRef`, `SnapshotID`, or approval state into the workflow token as a source of truth.

Workflow token should contain only continuity information:

```go
WorkflowToken {
    Version    int
    ScenarioID string
    WorkflowID string
    SubjectID  string
    Stage      WorkflowState
    IssuedAt   time.Time
    ExpiresAt  *time.Time
}
```

Do NOT put Authority, current debt, evidence state, or approval status into the token as authoritative state.

If the UI needs to display such information: **read current state from Solvent.**  
If an action needs authorization: **re-read/revalidate current Solvent state.**

A valid HMAC proves token integrity. It does NOT prove current authority validity.

```
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
```

Never:

```
token
  ↓
authority
  ↓
execute
```

---

## 12. Typed Token Transitions

**NEVER implement** `Refresh(token, map[string]interface{})` or any arbitrary field-overwrite API.

Use typed operations:

```go
AdvanceStage(oldToken, newStage, actor) → newToken
AttachEvidence(oldToken, evidenceID) → newToken
CompleteReview(oldToken, decision, actor) → newToken
```

Each transition must: validate preconditions, enforce allowed actor, evaluate policy where relevant, create a newly sealed token.

The token service must not become a generic signed mutable-state store.

If a workflow operation changes authoritative state, that state change must happen through the appropriate service/kernel path.

---

## 13. Preparation / Revalidation Boundary

Create a preparation boundary for consequential operations:

```go
PrepareForAction(scenarioID, beliefID, action, target) → PreparationResult
```

It should:
1. Read current belief
2. Evaluate current evidence projection
3. Read current debt
4. Evaluate contradictions
5. Read current authority
6. Evaluate policy
7. Verify actor/action/target relationship
8. Return a preparation result

Most importantly: **PREPARATION MUST RE-READ CURRENT AUTHORITATIVE STATE.**

Do not trust: browser state, workflow token contents, agent-provided approval, cached authority, stale API state.

---

## 14. Evidence Quality / Decision Support

Implement evidence quality as a service/read-model projection:

```go
EvidenceStatus: VERIFIED | UNVERIFIED | CONFLICT | STALE | MISSING
```

Only derive these where underlying evidence supports them. Do not fabricate confidence.

Provide: provenance, debt, contradictions, freshness where available, decision findings.

Add a transparent risk/confidence score only at the product layer. **The score is advisory. It can NEVER create or substitute for authority.**

```
confidence = 98, authority = absent  → DENIED
confidence = 42, authority = valid   → AUTHORIZED (subject to policy)
```

---

## 15. Audit / Activity Ledger

Create an append-only product audit/activity layer:

```go
Event {
    EventID, ScenarioID, Timestamp
    Actor, ActorType, Operation, Service
    BeliefID, IntentID, Target
    Result
    AuthorityRef, EvidenceRef, ExternalRef
    Metadata
}
```

Keep authority events distinct from external execution events:

```
Solvent AUTHORIZED deployment
GitHub adapter CALLED API
GitHub returned 200
executor REPORTS success
```

These are different facts. Never represent "authorization succeeded" as equivalent to "execution succeeded."

---

## 16. Database Changes

Treat authority-core schema changes with a very high bar. Prefer: service projections, read models, product tables, configuration, policy data.

**There should be NO default workflow migration.** Do NOT create `007_workflow.sql` unless implementation proves persistent workflow state is genuinely required.

If a workflow/activity table is needed, clearly classify it as product-layer persistence and keep it separate from authority-core tables.

An Activity Ledger table is a product audit table, not an authority-core table. Do not duplicate kernel truth.

---

## 17. Executor Boundary

There must be one clear executor abstraction. Avoid defining two unrelated Executor interfaces.

Preferred conceptual split:

```
ExecutionService
    ↓
Executor port
    ↓
provider-specific executor adapter
```

Executor receives a current validated authorization decision. Executor does NOT: approve, promote, revoke, create authority, create evidence authority. The executor only executes an already authorized action.

---

## 18. Real Integration

Do not build seven shallow integrations. Build ONE excellent commercial integration first.

Preferred initial use case: **GitHub / CI/CD / deployment workflow** — because it demonstrates: agent, evidence, human review, authority, exact target, external execution, audit, compliance evidence.

Additional integrations (future): Sentry, Datadog, Slack, Jira, AWS, Kubernetes, A2A, REST. All belong outside the kernel.

---

## 19. Adapter Framework

Create clean ports/adapters:

```go
EvidenceSource
Executor
DocumentGenerator    // FUTURE / NOT MVP
SignatureProvider    // FUTURE / NOT MVP
ActivityLedger
```

Adapters may know external semantics. Kernel may not.

Every adapter should support clear: success, provider failure, malformed response, timeout, degraded/demo mode where appropriate.

Use an adapter registry only where it simplifies dependency wiring. Do not build a plugin framework for its own sake.

---

## 20. Document / Signature Scope

Document and signature adapters are **OPTIONAL / FUTURE**. Do not implement complete document/signature infrastructure in this MVP.

If only documenting extension boundaries: define the port conceptually but clearly mark it FUTURE / NOT MVP.

If eventually implemented: `generation ≠ send`, `draft ≠ send`, `external provider result ≠ Solvent authority`. A signature/send action must pass the same authorization boundary as other consequential actions.

---

## 21. Compliance / Governance Layer

Compliance is part of the commercial architecture. BUT: **compliance semantics do NOT belong in the kernel.**

Create a product/service-level ComplianceService. The compliance layer consumes Solvent's authoritative facts. It does not create authority.

Core capability flow:

```
CONTROL → POLICY → REQUIRED EVIDENCE → SOLVENT DECISION
    → AUTHORITY → ACTION → EXECUTION → AUDIT
```

Initial compliance capabilities:
1. Control-to-evidence mapping
2. Evidence package generation
3. Immutable audit export
4. Exception management

Compliance exports should be derived from authoritative records. Formats: JSON, CSV, PDF.

Do not market the product as "Solvent makes you compliant." The correct positioning is: "Solvent produces authoritative evidence of governed AI-driven decisions and actions."

---

## 22. Compliance Control Model

Support generic controls, not dozens of standards initially:

```go
Control {
    ControlID
    Description
    RequiredPolicy
    EvidenceRequirements
    ResponsibleRole
    CurrentStatus
    Exceptions
    LastEvaluation
    EvidenceReferences
}
```

Framework mappings (SOC 2, ISO 27001, NIST, internal controls) should initially live in configuration/product data. Do NOT encode framework semantics inside the kernel.

---

## 23. Exception Management

Exceptions are governed decisions. An exception is NOT a bypass.

Support conceptual states: `OPEN`, `APPROVED`, `REJECTED`, `EXPIRED`, `REQUIRES_REVIEW`

An exception must: identify affected control, identify scope, identify approver, have explicit validity, be auditable.

**Never let "exception=true" bypass a hard kernel invariant.**

---

## 24. Web Product

Build a small operational console, not a giant SaaS platform.

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

6. COMPLIANCE (optional)
   controls, evidence, exceptions, export
```

The UI must expose WHY:

```
DENIED — belief not promoted
DENIED — required obligation remains open
DENIED — actor not permitted
DENIED — target mismatch
DENIED — authority revoked
ALLOWED — current authority matches exact target/action
```

---

## 25. Web UI Must Not Become Authorization Engine

Buttons are not security. A UI action (Approve, Execute, Revoke) must call the service layer. The service layer must revalidate. The kernel remains authoritative.

Do not trust: hidden buttons, disabled buttons, browser state, frontend workflow state, cached approval as security controls.

---

## 26. Scenario / Demo Framework

Create a scenario framework. Every scenario must use the SAME service/API path. Do not write five custom mini-security systems.

Required scenarios:
- A. Agentjacking
- B. Lying Agent
- C. Stale Authorization
- D. Confused Deputy
- E. Legitimate Workflow

Every scenario must be: deterministic, resettable, independently seeded, independently testable.

Reset only scenario-owned data. **NEVER drop the entire database as a scenario reset.**

---

## 27. Demo A — Agentjacking

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
audit trail records result
```

Also preserve the existing host-level restriction: untrusted telemetry agents must not be given direct mutation tools that would let them manually bypass the intended workflow. This is a deployment/control-surface concern, not a reason to expand the kernel.

---

## 28. Demo B — Lying Agent

```
agent claims approval exists
    ↓
no evidence / authority
    ↓
attempt to proceed
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

## 29. Demo C — Stale Authorization

```
authority legitimately exists
    ↓
authority revoked/invalidated
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

The demo must prove: **current authoritative state wins** over previously valid workflow state.

---

## 30. Demo D — Confused Deputy

```
authority for Target A
    ↓
action requested for Target B
    ↓
DENIED
```

**NEVER mutate the original immutable authority.** The correct sequence is:

```
wrong-target request → DENIED

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
```

Do NOT implement "target correction" on an existing authority.

---

## 31. Demo E — Legitimate Workflow

Show the complete happy path:

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
audit/compliance evidence package available
```

This is the commercial proof.

---

## 32. Open-Source "Break Solvent" Suite

Create reproducible challenge tests:

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

Each challenge should contain: attack, expected decision, reason, test.

The repository should invite engineers to try to defeat the authority model.

---

## 33. Security Testing

Test at four levels: KERNEL, SERVICE, ADAPTER, END-TO-END.

Security tests MUST include bypass attempts:

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

## 34. Token Security Tests

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

Most importantly:

```
valid signed stale token + revoked current authority = DENIED
```

---

## 35. Compliance Testing

Test:

```
control-to-evidence mapping
audit export completeness
authority traceability
actor traceability
approval traceability
exception lifecycle
revoked authority appears in audit evidence
execution failure remains distinct from authorization success
```

Do not test only UI rendering. Test the underlying records and service outputs.

---

## 36. Formal Verification

Do NOT enlarge the Lean model merely because the product expands. Lean should continue to represent the security-critical kernel state machine. Only modify Lean when kernel semantics change.

Prefer: small formal model, broad service/product layer.

---

## 37. Observability

Make security and operational facts visible:

```
authorization allowed
authorization denied
human approval
revocation
stale authority attempts
policy denial
adapter failure
executor failure
```

Maintain the distinction:
- authorization succeeded + execution failed
- authorization denied + execution never happened

These are different operational facts.

---

## 38. Commercial UX

The product must look like an operational system, not a research demo.

A buyer should be able to see: current workflow, pending decision, evidence, authority, action, target, actor, policy, result, audit trail, compliance evidence.

A technical demo should be capable of explaining the same event at three levels:

```
EXECUTIVE:    "Blocked because no authorized approval exists."
SECURITY:     "Agent principal lacked a valid authority tuple for target X."
ENGINEERING:  "Current kernel authorization check returned DENIED."
```

---

## 39. Commercial Positioning

Do not turn Solvent into a generic AI firewall, GRC platform, IAM replacement, policy engine, workflow SaaS, or agent framework.

The product should demonstrate **governed autonomy** through: authoritative evidence, explicit human decisions, machine-enforced authority, exact action/target binding, external execution boundaries, compliance evidence.

Key commercial message: **"Solvent provides the authority layer between autonomous systems and consequential actions."**

---

## 40. What Not to Build Now

Explicitly defer:

- Full kernel multi-tenancy
- Generalized policy language
- Universal workflow DSL
- Dozens of integrations
- Custom identity provider
- Deep IAM platform
- Cryptographic attestation infrastructure
- Global replay-prevention machinery
- Execution-effect verification framework
- Complex distributed scheduler
- Elaborate document/signature platform
- Large analytics platform
- Generic AI risk-scoring laboratory
- Complicated compliance-framework automation

---

## 41. Implementation Order

### PHASE 0 — Reconnaissance
- Repository inspection
- Baseline tests
- Architecture confirmation

### PHASE 1 — Service Boundaries
- Workflow service
- Actor/tool policy registry
- Evidence projection
- Audit/activity service
- Authority service

### PHASE 2 — Security Boundaries
- Preparation/revalidation boundary
- Human authorization boundary
- Typed workflow token
- Current-authority revalidation
- Executor contract

### PHASE 3 — One Real Integration
- GitHub/CI/CD adapter
- Executor
- Activity recording

### PHASE 4 — Product UI
- Overview
- Review queue
- Authority detail
- Audit
- Integrations
- Compliance evidence view

### PHASE 5 — Compliance
- Control mapping
- Evidence package
- Audit export
- Exceptions

### PHASE 6 — Demo Platform
- Agentjacking
- Lying Agent
- Stale Authorization
- Confused Deputy
- Legitimate Workflow

### PHASE 7 — Open Source
- Challenge suite
- Adapter examples
- Architecture docs
- Security docs
- Quickstart
- Developer guide

---

## 42. Acceptance Criteria

### Architecture
- [ ] Kernel remains small and domain-generic
- [ ] Existing kernel semantics are preserved
- [ ] MCP remains thin
- [ ] External systems are adapters
- [ ] Execution remains outside kernel
- [ ] Service layer owns orchestration/policy
- [ ] Compliance remains outside kernel

### Security
- [ ] Actor classification exists
- [ ] Authentication is separate from actor type
- [ ] Human-only operations are mechanically gated
- [ ] Agent claims cannot become authority
- [ ] Workflow tokens cannot become authority
- [ ] Signed stale tokens cannot bypass current authority state
- [ ] Wrong targets are denied
- [ ] Revoked authority is denied
- [ ] Browser state cannot become authority
- [ ] Executors cannot create authority

### Product
- [ ] Operator can inspect pending decisions
- [ ] Operator can inspect evidence
- [ ] Operator can understand why action is blocked
- [ ] Operator can review/approve legitimate work
- [ ] Operator can inspect exact authority
- [ ] Operator can inspect audit trail
- [ ] Operator can inspect execution outcome
- [ ] Compliance evidence can be exported

### Compliance
- [ ] Controls can map to evidence
- [ ] Evidence traces to decisions
- [ ] Decisions trace to authority
- [ ] Authority traces to action/target
- [ ] Actions trace to external execution
- [ ] Exceptions are explicit and auditable
- [ ] Exceptions cannot bypass hard kernel invariants

### Demos
- [ ] Agentjacking works
- [ ] Lying Agent works
- [ ] Stale Authorization works
- [ ] Confused Deputy works
- [ ] Legitimate Workflow works
- [ ] All scenarios reset independently

### Open Source
- [ ] New adapter does not require kernel modification
- [ ] New executor does not require kernel modification
- [ ] New customer policy does not require kernel modification
- [ ] New demo scenario uses common services
- [ ] Security challenges are reproducible

### Quality
- [ ] Existing tests pass
- [ ] Service tests pass
- [ ] Security bypass tests pass
- [ ] End-to-end tests pass
- [ ] Lean verification remains clean
- [ ] No unexplained kernel growth
- [ ] No unexplained authority-core schema growth
- [ ] Documentation reflects actual implementation

---

## 43. Deliverables

1. Implementation (code)
2. Architecture documentation
3. Kernel/schema ADRs where necessary
4. Commercial MVP README
5. Security model
6. Workflow documentation
7. Policy/tool registry documentation
8. Adapter development guide
9. Executor guide
10. Compliance/evidence guide
11. Demo walkthrough
12. Challenge-suite documentation
13. Test report
14. Buyer-oriented "What is Solvent?"
15. Security-engineer-oriented "Why is Solvent different?"
16. Final implementation summary

Final implementation summary MUST include: files added, files changed, migrations added, kernel changes, reason for every kernel change, service additions, adapter additions, executor additions, UI additions, compliance additions, demo additions, test coverage, remaining limitations.

---

## 44. Final Design Principles

1. **SMALL TRUSTED CORE** — Keep the kernel minimal.
2. **CURRENT AUTHORITY WINS** — Never let cached or signed workflow state substitute for current authority verification.
3. **AUTHORIZE ≠ EXECUTE** — The authorization layer and execution layer are separate.
4. **ACTOR ≠ IDENTITY** — Actor classification does not authenticate callers.
5. **EVIDENCE ≠ AUTHORITY** — Evidence can support authority; evidence does not become authority.
6. **POLICY ≠ KERNEL** — Customer policy belongs outside the kernel.
7. **WORKFLOW ≠ AUTHORITY** — Workflow coordinates the process; Solvent establishes authority.
8. **COMPLIANCE ≠ KERNEL** — Compliance consumes Solvent facts; it does not redefine them.
9. **ADAPTERS KNOW EXTERNAL SYSTEMS** — The kernel does not.
10. **ONE SOURCE OF AUTHORITY TRUTH** — Do not duplicate kernel authority in tokens, browser state, workflow objects, or external providers.
11. **MAKE SECURITY VISIBLE** — The product must explain WHY an action was allowed or denied.
12. **BUILD ONE EXCELLENT REAL WORKFLOW** — Prefer one production-quality integration over many superficial ones.
13. **DEMOS MUST SHARE THE SAME CORE** — Different attacks, same Solvent semantics.
14. **OPEN SOURCE SHOULD BE EXTENSIBLE** — A developer should be able to add an adapter, executor, policy, or scenario without touching the kernel.
15. **COMPLEXITY MUST BE EARNED** — Only real customer demand, operational pain, observed security failure, or a clear security requirement justifies expanding the trusted core.

---

## Final Objective

The finished system should create this impression:

**TO A BUYER:** "This governs what our AI systems are actually allowed to do."

**TO A SECURITY ENGINEER:** "The critical authority decision is backed by a small, transactionally enforced core rather than LLM behavior."

**TO A DEVELOPER:** "I can integrate my system without modifying that core."

**TO A COMPLIANCE TEAM:** "I can trace controls and evidence to actual governed actions."

**TO AN OPEN-SOURCE CONTRIBUTOR:** "I can extend Solvent without turning every integration into another security mechanism."

Do not optimize for maximum code. Optimize for: small trusted core, clear boundaries, current-state authority verification, commercially useful workflow, visible security guarantees, compliance traceability, one real integration, excellent demonstrations, easy extension, maintainability, open-source credibility.

**Build the product around the kernel. Do not turn the product into the kernel.**
