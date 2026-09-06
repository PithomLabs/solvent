## prompt

refresh me about the roadmap, the pending phase implementation and how it overlaps with extension mechanisms discussed earlier, the ultimate goal is to make Solvent the authorization layer for AI agents, apps, workflows and everything in between with a solid kernel and extension mechanisms on top of it

## answer

Yes. The roadmap has now reached an important architectural boundary: **Phase 4B is frozen and verified**, so the next work should be about making the kernel useful as a broadly consumable authorization layer without bloating the kernel itself.

The ultimate architecture we have been converging toward is:

```text
                    AI agents
                       │
                    MCP / A2A
                       │
apps ────────────── REST/API ────────────── workflows
                       │
                 extension/adapters
                       │
                 Service / Policy
                       │
              ┌──────────────────┐
              │  SOLVENT KERNEL  │
              │ authority core   │
              │ invariants       │
              │ current state    │
              └──────────────────┘
                       │
                 CockroachDB
```

The key idea is:

> **Solvent should become the authorization layer for AI agents, applications, workflows, and other consequential systems, while keeping the trusted kernel small and pushing almost everything else into extension mechanisms above it.**

## Where we are now

The roadmap we established was:

```text
Phase 0   Reconnaissance
Phase 1   Service boundaries
Phase 2   Security boundaries
Phase 3   One real integration / architecture validation
Phase 4A  Canonical API Contract
Phase 4B  API / Service implementation
Phase 4C  OpenAPI + reference integrations
Phase 4D  Optional Web UI
Phase 4E  API/UI adversarial review
Phase 5   Compliance / Governance — earned later
Phase 6   Demo platform
Phase 7   OSS polish
```

The important news is that **4A, 4B, 6.1, 6.2, 6.3, and 6.4 have now essentially established the foundation**.

Phase 4A defined the language-neutral API boundary. The contract explicitly positions MCP, A2A, Web UI, future SDKs, and other clients as consumers of the same service semantics rather than independent authorization systems. 

Phase 4B then implemented that boundary with a real service layer and the atomic authorization+intent primitive. The final security review established that REST and MCP converge on the same authority path and that there is one authority engine. 

The critical serialization discovery was actually valuable architecturally: we learned that **transactional composition by itself wasn't enough** and that the authority and revocation paths needed an explicit shared serialization point. Phase 6.4 added the `FOR UPDATE` protocol on the `authority_target` row and the adversarial review concluded that the stale-authority state was now eliminated.  

And your final fresh `go test ./...` run closed the last verification gap.

So I would regard:

```text
Phase 4B = frozen security/authorization foundation
```

not merely "API work completed."

---

# What the kernel actually is

The kernel should remain deliberately boring.

Its job is to know facts such as:

```text
Who is the principal?
What target was proposed?
What exact authority was approved?
What exact action/target tuple is authorized?
Has that authority been revoked?
Is the supporting belief in the required state?
Can this intent be created under current authority?
```

The kernel **does not need to know**:

```text
GitHub
Kubernetes
Slack
Jira
AWS
OpenAI
Anthropic
MCP-specific semantics
A2A-specific semantics
SOC 2
ISO 27001
CI/CD
customer workflows
UI concepts
agent frameworks
LLM providers
```

That is deliberate.

Phase 4A's resource model already reflects this distinction: `Target Snapshot + Target Activation` are the authority; beliefs, evidence, policy projections, and audit activity live around that core. 

That is the foundation for making Solvent universal.

---

# Where extension mechanisms fit

This connects directly to our earlier discussion about the "Linux kernel" analogy.

The principle was:

> **Do not expand the kernel merely because a new integration exists.**

Instead:

```text
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
        → Kernel
```

That remains the core architecture.

So when Solvent eventually supports:

```text
GitHub deployment
Kubernetes rollout
AWS mutation
database migration
financial transaction
agent tool invocation
A2A workflow
MCP tool call
internal enterprise workflow
```

we should **not** make the kernel understand each one.

Instead:

```text
GitHub Adapter
Kubernetes Adapter
AWS Adapter
MCP Adapter
A2A Adapter
...
        ↓
canonical Solvent service semantics
        ↓
kernel authority
```

That is how Solvent can become broad without becoming enormous.

---

# What Phase 4C is really about

The next phase is not "add more security."

The kernel security foundation is the part we just finished.

**Phase 4C is about making that foundation consumable.**

The immediate deliverables should therefore be:

```text
OpenAPI specification
        +
canonical request/response schemas
        +
error semantics
        +
authentication contract
        +
reference integration examples
```

The goal is:

> A developer in Go, Python, Rust, TypeScript, Java, or anything else should be able to integrate with Solvent without understanding the kernel internals.

That follows directly from the language-neutral API decision.

The API contract is canonical; SDKs are convenience wrappers, not separate semantics.

So the progression becomes:

```text
Kernel semantics
      ↓
Service semantics
      ↓
HTTP/API contract
      ↓
OpenAPI
      ↓
Reference clients
      ↓
SDKs later, when demand justifies them
```

---

# Why this matters to the ultimate Solvent vision

Suppose an AI agent wants to:

```text
merge PR
deploy service
delete resource
modify production DB
send payment
issue refund
change access control
send external message
```

You do **not** want Solvent to be a giant firewall with hundreds of bespoke rules for every API.

Instead the external system expresses a proposed consequential operation as a canonical Solvent authorization target:

```text
principal
resource
scope
action
consequence
parameters
```

The external system supplies evidence.

Humans or policy systems approve authority.

Then execution asks:

```text
"Is THIS exact principal
 allowed to perform THIS exact action
 against THIS exact target
 under CURRENT authority?"
```

The kernel answers.

That is much more powerful than a traditional "agent firewall" concept.

---

# The extension stack we are building toward

I would think about Solvent as six layers.

### 1. Kernel

The smallest trusted authority core.

```text
Authority
Invariants
Revocation
Current-state verification
Atomic security transitions
```

### 2. Service / Policy layer

This is where composition lives.

```text
Evidence
Policy
Workflow orchestration
Decision preparation
Audit coordination
Integration coordination
```

The service layer **does not become authority**. The final authority decision still belongs to the kernel.

### 3. Protocol adapters

These make Solvent reachable everywhere:

```text
REST
MCP
A2A
future protocol adapters
```

They translate external protocol semantics into canonical Solvent operations.

### 4. Integration adapters

These connect Solvent to the outside world:

```text
GitHub
CI/CD
Kubernetes
AWS
databases
ticketing systems
agent runtimes
etc.
```

They should live at the edge.

### 5. Executors

Eventually:

```text
Authorization
    ↓
ExecutionService
    ↓
Executor
    ↓
external system
```

The executor performs the real side effect but cannot create authority, approve itself, revoke authority, or redefine the authorization model.

That boundary is deliberately not implemented yet; the current executor registry is empty. The adversarial review explicitly verified that there is no production executor bypass today. 

### 6. Product layer

Then:

```text
Web UI
Audit
Reporting
Compliance
Governance
Analytics
Evidence packages
```

These are products **around** the kernel, not additions to it.

---

# Why the one-real-integration strategy still matters

We previously decided not to build ten integrations.

The first serious integration should demonstrate the whole thesis end-to-end:

```text
untrusted AI output
      ↓
evidence
      ↓
review
      ↓
approval
      ↓
authority
      ↓
exact authorization
      ↓
execution
      ↓
audit
```

GitHub/CI-CD was the preferred candidate because it makes the authorization story very understandable:

```text
"Agent wants to deploy commit X to production."

Solvent:
Who?
What repository?
Which commit?
Which environment?
Which action?
What evidence?
Who approved it?
Is that authority still valid?
```

Then Solvent becomes visibly useful rather than merely philosophically interesting.

That integration will also test whether our extension boundary is actually good.

---

# What should NOT happen next

This is probably the most important part of the roadmap.

Now that the core works, there will be a temptation to build:

```text
RBAC
multi-tenancy
policy DSL
workflow engine
agent identity platform
OAuth
enterprise IAM
compliance mappings
dozens of executors
huge UI
SDKs for 12 languages
```

too early.

We should resist that.

The architecture's current discipline is:

```text
Need proven by integration?
    ↓ yes → extension layer

Need proven by policy/orchestration?
    ↓ yes → service/policy layer

Need proven by deployment?
    ↓ yes → adapter/executor

Need proven by customer demand?
    ↓ yes → product feature

Cannot be represented securely without a new
durable fact or atomic security transition?
    ↓ yes → kernel

Otherwise:
    don't add it
```

That rule is what keeps "Solvent as an authorization layer for everything" from becoming "Solvent as another giant platform."

---

# The bigger picture

The end state I'd keep in mind is:

```text
                         SOLVENT
                            │
             ┌──────────────┴──────────────┐
             │                             │
        AUTHORITY CORE                EXTENSION PLANE
             │                             │
     ┌───────┼────────┐          ┌─────────┼──────────┐
     │       │        │          │         │          │
  Identity Authority Revocation  Protocol  Policy   Integrations
     │       │        │          adapters   │       adapters
     │       │        │          │          │          │
     └───────┴────────┘          MCP/A2A    │     GitHub/K8s/AWS/...
             │                              │
             └─────────── Service ──────────┘
                            │
                      Execution layer
                            │
                       External world
```

The kernel answers **"may this happen?"**

The extension plane answers:

```text
What is happening?
Where?
Through which protocol?
Under which policy?
How do we collect evidence?
How do we execute it?
How do we show it?
How do we report it?
```

That division is the heart of the product.

## Where we are headed now

So the immediate roadmap is:

```text
NOW
Phase 4B frozen
    ↓
Phase 4C
OpenAPI + canonical integration contract
    ↓
reference client / integration
    ↓
one serious external executor/integration
    ↓
validate extension architecture in reality
    ↓
Phase 4D optional Web UI
    ↓
Phase 4E adversarial API/UI review
    ↓
earned governance/compliance capabilities
```

The key strategic transition is that **Phase 4C begins turning Solvent from a well-designed authorization kernel into a platform that other systems can actually embed**.

That is exactly where your ultimate goal—**Solvent as the authorization layer between AI agents/apps/workflows and consequential actions**—starts becoming real.

And the most important architectural constraint remains: **the kernel should stay small even as everything around it grows.** The public API and extension mechanisms are how Solvent scales outward; the kernel should not scale outward with them. 
