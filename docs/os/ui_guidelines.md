# Background

Before writing the Phase 4 implementation prompt, I would establish a **UI design constitution** for Solvent. The purpose is to prevent the commercial MVP from slowly turning into a conventional SaaS dashboard that contaminates the architecture, duplicates authority logic, or overwhelms the agentic use case.

# Solvent UI Design Guidelines & Principles

## 1. The UI is a client, not the product's authority

The Web UI is an **optional operational client** of Solvent.

It is not:

* the authority engine
* the policy engine
* the identity system
* the workflow engine
* the source of truth
* a prerequisite for MCP or A2A

The architecture remains:

```text
MCP ──────────┐
A2A ──────────┤
REST/API ─────┤
              ↓
        Service Layer
              ↓
        Solvent Kernel
              ↓
          CockroachDB

Optional:
              ↑
        HTTP/API Adapter
              ↑
           Web UI
```

A UI feature must never require a new kernel primitive merely because the UI needs information or behavior.

---

## 2. The kernel must remain completely UI-blind

The kernel must know nothing about:

```text
browser
HTML
CSS
forms
buttons
screens
React
JavaScript
sessions
dashboard concepts
navigation
visual state
```

The UI speaks to the service/API layer.

Never:

```text
UI requirement
→ kernel feature
```

Prefer:

```text
UI requirement
→ existing service/read model
→ existing kernel semantics
```

Only introduce a new kernel primitive when the underlying **security semantics themselves** require one.

---

## 3. The UI must never become a second authorization engine

This is probably the most important Phase 4 rule.

A UI can display:

```text
ALLOWED
DENIED
PENDING
REVOKED
```

but it must never decide those states.

Bad:

```text
browser
  ↓
if approved {
    show Execute
}
```

and especially:

```text
browser
  ↓
if approved {
    call provider
}
```

Correct:

```text
browser
  ↓
service request
  ↓
current server-side state
  ↓
kernel / service authorization
  ↓
result
```

A hidden button is not a security boundary.

A disabled button is not a security boundary.

A frontend condition is not a security boundary.

---

## 4. The UI should explain authority, not simulate authority

Solvent's differentiation is not that it has a pretty dashboard.

The UI should make the authority model **legible to humans**.

A user should be able to answer:

```text
What does the system believe?
What evidence supports it?
What remains unresolved?
Who is acting?
What action is being requested?
What target is involved?
What authority exists?
Why is it allowed?
Why is it denied?
What happened afterward?
```

The UI should expose the reasoning chain:

```text
Evidence
   ↓
Belief
   ↓
Obligations / debt
   ↓
Promotion
   ↓
Authority
   ↓
Action / Intent
   ↓
Outcome
```

It should **visualize the chain**, not recreate the chain independently.

---

## 5. "Why?" is a first-class UI primitive

Every security-relevant state should have a human-readable explanation.

Examples:

```text
DENIED
Belief has not been promoted.

DENIED
Required obligation remains open.

DENIED
No active authority exists for this target.

DENIED
Authority exists, but the requested target does not match.

DENIED
Authority was revoked.

DENIED
Actor does not match the approved principal.

ALLOWED
Current authority matches the exact actor, action, target,
and consequence parameters.
```

Avoid vague messages such as:

```text
Unauthorized
Something went wrong
Request rejected
Policy failed
Access denied
```

when Solvent can provide the actual reason.

---

## 6. Current state beats cached UI state

Never let the browser present cached information as authoritative without qualification.

For example, this is dangerous:

```text
UI says:
"Approved"
```

while the underlying authority has already been revoked.

The UI may cache data for performance, but any consequential operation must use current server-side state.

The conceptual rule remains:

```text
UI state ≠ authority state
```

and:

```text
cached state ≠ current authority
```

---

## 7. UI actions must map to explicit service operations

Every button or command should have a clear semantic operation behind it.

For example:

```text
Review
Approve
Reject
Revoke
Retry
Inspect Evidence
```

should correspond to explicit service operations.

Avoid generic mutation endpoints such as:

```text
updateState(...)
modify(...)
setStatus(...)
```

where the frontend effectively defines its own state machine.

The UI should call **meaningful domain operations**, not manipulate arbitrary state.

---

## 8. No arbitrary client-controlled security state

The browser must never be allowed to submit authoritative values such as:

```text
approved=true
authorized=true
actor_is_human=true
authority_valid=true
risk_passed=true
policy_allowed=true
```

as trusted state.

A client may request an operation.

The server determines whether the operation is permissible.

---

## 9. The UI should distinguish facts from projections

The product may display:

```text
risk
confidence
evidence quality
freshness
status
recommendation
```

but the UI must distinguish:

```text
AUTHORITATIVE FACT
```

from:

```text
ADVISORY / DERIVED / PROJECTED INFORMATION
```

For example:

```text
Evidence quality: A
```

must not visually imply:

```text
Authority: valid
```

Likewise:

```text
Risk score: 12%
```

must not imply:

```text
Execution allowed
```

The visual hierarchy should reinforce:

> **evidence and scoring support decisions; authority determines permission.**

---

## 10. Avoid "AI theater"

Do not make the UI look like an AI chatbot merely because Solvent governs AI systems.

Avoid unnecessary:

```text
chat panels
glowing AI widgets
agent avatars
conversation bubbles
AI-generated summaries everywhere
```

unless they directly improve the operational task.

The product should feel like a **security/operations system for governed autonomy**, not another AI chat application.

---

## 11. Prefer evidence-first operational UX

The most valuable screen is not a generic dashboard.

It is the screen where an operator can understand:

```text
WHAT HAPPENED
WHY IT MATTERS
WHAT SUPPORTS IT
WHAT IS MISSING
WHAT IS ALLOWED
WHAT IS NOT ALLOWED
WHAT SHOULD HAPPEN NEXT
```

The Review/Decision experience should therefore emphasize:

```text
evidence
provenance
contradictions
debt
authority
actor
action
target
decision
audit
```

rather than decorative metrics.

---

## 12. Make the distinction between intent and execution visible

Because Solvent deliberately separates:

```text
intent
```

from:

```text
execution
```

the UI should never imply that creating an intent means something happened externally.

For example:

```text
INTENT CREATED
```

is not:

```text
DEPLOYMENT EXECUTED
```

Likewise:

```text
AUTHORIZATION GRANTED
```

is not:

```text
EXECUTION SUCCEEDED
```

The UI should preserve these distinctions explicitly.

---

## 13. Design for agentic systems first

The Web UI must not assume that a human is the only or primary consumer.

The underlying service/API contracts must remain useful to:

```text
MCP agents
A2A agents
automation
REST clients
administrative tooling
future executors
```

The UI should therefore be a **reference client for the same semantic APIs**, not a proprietary workflow that other clients cannot reproduce.

A useful architectural test is:

> **Can an MCP/A2A client perform the same legitimate operation without the Web UI?**

For core Solvent capabilities, the answer should be yes.

---

## 14. Web UI dependency must remain optional

Removing the Web UI must not break:

```text
authority
evidence
belief lifecycle
authorization
MCP
A2A
service APIs
kernel
```

This should be treated as an architectural acceptance criterion.

Conceptually:

```text
DELETE /web
    ↓
Solvent still functions
```

The UI is an operational surface, not infrastructure required by the core.

---

## 15. Build the smallest useful console

The initial commercial console should stay deliberately small.

Core surfaces:

```text
Overview
Review Queue
Decision / Authority Detail
Audit / Activity
Integrations
```

Do not immediately add:

```text
analytics center
workflow builder
custom reports
notification center
chat
billing
organization management
large admin portal
generic GRC suite
policy IDE
```

Every new screen must justify its existence through a concrete operator task.

---

## 16. One screen should answer one operational question

Avoid dashboards full of disconnected cards.

Prefer questions:

### Overview

> What needs attention?

### Review Queue

> What decisions require human attention?

### Decision Detail

> Why can or can't this action happen?

### Audit

> What actually happened?

### Integrations

> What external systems are connected and what is their state?

This keeps the product understandable and prevents dashboard sprawl.

---

## 17. Optimize for "time to understanding"

A security/operator product should minimize the time required to understand a blocked or permitted action.

A useful target:

```text
request
→ identify actor
→ identify action
→ identify target
→ inspect evidence
→ inspect authority
→ understand decision
```

without navigating through multiple unrelated screens.

The UI should favor **explanatory density**, not feature density.

---

## 18. Security state should be visually unmistakable

Important states should be immediately distinguishable:

```text
ALLOWED
DENIED
PENDING
REVOKED
STALE
CONFLICT
MISSING
FAILED
```

Do not encode critical state using color alone.

Use:

```text
label
icon
status text
reason
```

so the meaning survives accessibility limitations and monochrome environments.

---

## 19. Never hide dangerous operations behind visual conventions

High-impact operations should be explicit.

For example:

```text
Revoke Authority
```

should not be buried inside a generic kebab menu simply because the UI wants to look clean.

But the UI still does not enforce authorization.

It merely makes the operation explicit; the service/kernel remains responsible for permission.

---

## 20. No "god dashboard"

Do not build one giant screen containing:

```text
system health
agents
evidence
beliefs
authority
audit
compliance
integrations
analytics
```

The goal is operational clarity.

The commercial MVP should feel like a **small, coherent security console**, not an enterprise suite assembled prematurely.

---

## 21. The UI should surface the trust boundary itself

A sophisticated user should be able to tell:

```text
THIS IS EXTERNAL INPUT
THIS IS SOLVENT EVIDENCE
THIS IS A BELIEF
THIS IS A DERIVED ASSESSMENT
THIS IS AUTHORITATIVE AUTHORITY
THIS IS AN EXECUTION RESULT
```

This is particularly important for Solvent because the product's central thesis is:

> **Retrieval is not authority.**

The UI should make that distinction visually and semantically obvious.

---

## 22. Do not obscure uncertainty

If Solvent doesn't know something, say so.

Use:

```text
UNKNOWN
MISSING
UNVERIFIED
STALE
CONFLICT
```

rather than:

```text
probably safe
high confidence
looks approved
likely authorized
```

unless the UI explicitly labels such values as advisory projections.

This is especially important for an authority product.

---

## 23. The UI should make auditability natural

For every meaningful security decision, the operator should be able to navigate naturally from:

```text
decision
→ evidence
→ authority
→ action
→ audit
```

But audit itself remains an evidence record.

It cannot become:

```text
audit
→ authority
```

The UI must preserve that conceptual direction.

---

## 24. Avoid premature multi-tenancy and enterprise infrastructure

Do not introduce:

```text
organizations
tenants
teams
RBAC matrix
SSO
SCIM
billing
quotas
enterprise administration
```

merely because the UI is being called a "commercial MVP."

Those features may eventually matter, but they should be earned by actual product requirements.

They are especially dangerous because they can pressure the kernel to absorb identity and tenancy semantics.

---

## 25. The UI must be replaceable

A future Solvent ecosystem could have:

```text
CLI
Web UI
MCP client
A2A client
IDE plugin
Security console
SOC integration
```

The UI architecture should therefore avoid any proprietary state machine that becomes necessary for using Solvent.

The test is:

> **Could another team build a different UI against the same service/API contracts without changing the kernel?**

The answer should be yes.

---

# Phase 4 Architectural Guardrails

I would make these the **non-negotiable acceptance rules**:

```text
1. Web UI does not modify kernel semantics.

2. Web UI is optional.

3. MCP and A2A remain usable without Web UI.

4. UI never determines authorization.

5. UI never stores authoritative security state.

6. UI never trusts browser state as authority.

7. Every consequential operation goes through the server/service boundary.

8. Current Solvent state, not cached UI state, determines authorization.

9. Evidence, scoring, recommendation, and authority are visually and
   semantically distinct.

10. Intent != execution is preserved in the UI.

11. No UI-specific concept enters the kernel.

12. No new kernel primitive is justified solely by UI requirements.

13. No giant SaaS subsystem is introduced as part of Phase 4.

14. No identity/IAM/GRC platform is introduced prematurely.

15. The UI consumes service/read-model APIs rather than duplicating
    domain logic.

16. Removing the Web UI does not weaken Solvent's agentic capabilities.
```

# The Phase 4 design philosophy

I would summarize the entire discipline in one sentence:

> **Build the smallest operational console that makes Solvent's authority model understandable and useful, while keeping the Web UI completely optional, replaceable, and outside the trusted core.**

And one even stronger engineering test:

> **If implementing a UI feature makes us want to modify the kernel, stop and ask why.**

That should be our Phase 4 reflex.

The commercial MVP should ultimately feel like:

```text
             SOLVENT
        ┌────────────────┐
        │ Optional Web UI│
        └───────┬────────┘
                ↓
          Service APIs
                ↓
       ┌─────────────────┐
MCP ──→│                 │←── A2A
       │ Solvent Kernel  │
REST ─→│                 │
       └────────┬────────┘
                ↓
           CockroachDB
```

**The UI makes the authority model visible. It does not become part of the authority model.**

This is the guideline set I would lock before generating the Phase 4 coding-agent prompt.
