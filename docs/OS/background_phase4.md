Yes. That is actually the better sequencing.

The roadmap we locked was broadly:

```text
Phase 0  Reconnaissance
Phase 1  Service boundaries
Phase 2  Security boundaries
Phase 3  One real integration
Phase 4  Product/UI
Phase 5  Compliance — deferred / earned
Phase 6  Demo platform
Phase 7  Open-source polish
```

But after our language-agnostic decision, **Phase 4 should be refined**. We should not jump directly into screens.

## Phase 4 should become API-first Product Surface

I would structure it as:

```text
Phase 4A — API Contract
        ↓
Phase 4B — Service/API implementation
        ↓
Phase 4C — Reference clients / integration examples
        ↓
Phase 4D — Web UI
        ↓
Phase 4E — UI/API adversarial review
```

The critical idea is:

> **Design the API surface before designing the UI.**

The UI then becomes the first reference client of a stable API rather than the thing that dictates the API.

### Why this fits Solvent

We want:

```text
Python ─────┐
Go ─────────┤
TypeScript ─┤
Rust ───────┤
MCP ────────┤
A2A ────────┤
Web UI ─────┘
       ↓
  Solvent API
       ↓
Service Layer
       ↓
Kernel
```

The **API contract becomes the language-neutral boundary**.

That means Phase 4 should first define things such as:

```text
authority
evidence
belief
review
intent
decision
audit
integration
```

as stable API resources/operations.

Not UI screens.

---

# What API design should accomplish

The API should expose the Solvent concepts without leaking implementation details of the Go kernel.

For example, a client should think in terms of:

```text
Create evidence
Get belief
Review belief
Request authorization
Check authorization
Revoke authority
Get activity
```

rather than:

```text
call kernel.Store
call sqlAuthorizeResolve
construct AuthorityTuple manually
```

The kernel remains behind the API.

So:

```text
API semantics
    ≠
kernel implementation details
```

This is especially important for language-agnostic integration.

---

# DealForge and AegisFlow fit here

This is where I would use your existing codebases.

Treat them as **reference implementations for API ergonomics**, particularly:

```text
workflow/resource modeling
review flows
activity/audit presentation
integration patterns
request/response ergonomics
```

But extract only what helps Solvent's API.

Do not import their conceptual authority models if they conflict with:

```text
evidence != authority
workflow != authority
authorize != execute
current authority wins
kernel is final authority oracle
```

A useful approach would be:

```text
DealForge
     \
      → API design study → Solvent API
     /
AegisFlow
```

rather than:

```text
DealForge → port architecture
AegisFlow → port architecture
Solvent → becomes combination of both
```

---

# What I would make canonical

For Phase 4A, I would define **three layers of API**.

### 1. Core HTTP/JSON API

This is the fundamental language-neutral integration surface.

Something conceptually like:

```text
/v1/beliefs
/v1/evidence
/v1/reviews
/v1/authorizations
/v1/intents
/v1/activity
/v1/integrations
```

The exact resources should come from the existing domain, not be invented for REST aesthetics.

### 2. OpenAPI specification

This becomes the machine-readable contract.

That gives us:

```text
Python client
Go client
TypeScript client
Rust client
Java client
```

without implementing Solvent semantics separately in each language.

### 3. Agent protocols

Keep:

```text
MCP
A2A
```

as first-class integration surfaces that map into the same service semantics.

The important rule:

```text
REST/API
MCP
A2A
Web UI

        ↓
same service semantics
        ↓
same kernel authority
```

No protocol gets its own authorization engine.

---

# SDKs fit after this

This sequencing also solves your earlier SDK question.

First:

```text
API contract
```

Then:

```text
OpenAPI
```

Then, where demand justifies it:

```text
Go SDK
Python SDK
TypeScript SDK
Rust SDK
...
```

The SDK is just a convenient façade over the canonical API.

So we're not designing five different Solvent products.

We're designing **one protocol/API with multiple clients**.

---

# Where the Web UI comes in

Once the API is solid:

```text
Solvent API
      ↓
Web UI
```

The UI should expose the API's semantics.

For example:

```text
GET /v1/reviews
```

becomes:

```text
Review Queue
```

and:

```text
GET /v1/authorizations/{id}
```

becomes:

```text
Authority / Decision Detail
```

and:

```text
GET /v1/activity
```

becomes:

```text
Audit / Activity
```

That gives us a very useful test:

> **If the Web UI needs an endpoint that has no corresponding legitimate API concept, ask whether the UI is inventing product semantics.**

That is precisely the discipline we want.

---

# Revised roadmap

I would now treat the roadmap as:

```text
PHASE 0
Reconnaissance
        ↓
PHASE 1
Service boundaries
        ↓
PHASE 2
Security boundaries
        ↓
PHASE 3
Integration architecture / security validation
        ↓
PHASE 4A
Canonical API/domain contract
        ↓
PHASE 4B
API/service implementation
        ↓
PHASE 4C
OpenAPI + reference integration examples
        ↓
PHASE 4D
Optional Web UI
        ↓
PHASE 4E
API/UI adversarial review
        ↓
PHASE 5
Compliance — deferred / earned
        ↓
PHASE 6
Demo platform
        ↓
PHASE 7
OSS polish
```

And I'd make **Phase 4A the next thing we do**, not UI.

## Phase 4A should answer

Before writing UI code, we should be able to answer:

```text
What are Solvent's public resources?

What operations exist?

What are the request/response schemas?

What errors exist?

What constitutes authentication?

What constitutes authorization?

Which operations are read-only?

Which operations mutate Solvent state?

Which operations create authority?

Which operations only request authorization?

Which operations revoke authority?

What is idempotent?

What is not idempotent?

What is the audit/event contract?

How do MCP and A2A map onto the same semantics?

How does a foreign-language client integrate without knowing Go?

Which operations are safe for agents?

Which require human approval?

What is explicitly NOT part of the API?
```

That last question is important.

A disciplined API needs a **negative space**.

For example:

```text
No API endpoint:
    /execute-without-authority
    /set-authorized
    /approve-with-agent
    /update-authority-state
    /force-execution
```

We should make the safe architecture hard to misuse.

---

# And this protects the kernel

This approach is particularly good for your original concern.

Instead of:

```text
UI requirement
→ kernel change
```

we get:

```text
client requirement
→ API design
→ service implementation
→ existing kernel semantics
```

If API design exposes something the kernel cannot safely represent, **that becomes an architectural warning**, not an automatic reason to modify the kernel.

That is the exact "complexity must be earned" discipline we established.

## My recommendation

**Do not start the Phase 4 UI prompt yet.**

First do **Phase 4A: Canonical Solvent API Design**, using DealForge and AegisFlow as reference material.

Once the API contract is solid, the UI becomes much easier—and, more importantly, we will know whether we're actually building a **language-agnostic authority platform** or accidentally building a UI-centric Go application.



Locked in.

The current Solvent architecture, security baseline, product philosophy, demo portfolio, and roadmap above are now the working baseline for subsequent decisions.

Most importantly, the roadmap now treats **Phase 4 as API-first**:

**4A — Canonical API Contract → 4B — API/Service Implementation → 4C — OpenAPI + Reference Integrations → 4D — Optional Web UI → 4E — API/UI Adversarial Review**

The governing architectural rule remains:

> **Keep the kernel as the smallest trusted authority core. Everything else earns its place outside it.**

And the API is now treated as a **language-neutral product contract**, not a thin transport layer around Go internals.
