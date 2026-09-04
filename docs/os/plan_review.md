I reviewed the uploaded **Solvent Commercial MVP Implementation Plan**. It is broadly aligned with the architecture we locked in, but I would **not hand it to the coding agent unchanged**. There are several places where the plan risks accidentally turning the product shell into a second kernel.

The plan correctly preserves the central boundary: small kernel, service/policy around it, adapters at the edge, and execution outside Solvent.  It also correctly puts workflow, actor/tool policy, evidence quality, audit, and executors into the service layer rather than `kernel/`. 

## The five changes I would make before implementation

### 1. Do not freeze `kernel/` merely as a directory; freeze its **responsibility**

The plan says:

> `kernel/ # FROZEN — invariant enforcement`

That's directionally right, but too absolute.

The better rule is:

> **Kernel API and semantics are frozen by default; kernel changes require the Kernel Growth Gate.**

Otherwise the coding agent may avoid a genuinely necessary security primitive simply because "kernel is frozen."

Your actual architectural rule is the stronger one:

```text
new external behavior       → adapter
new workflow/policy          → service
new execution behavior      → executor
new presentation/analytics  → product layer
new impossible state         → DB invariant
new atomic security fact     → kernel, only when unavoidable
```

That is the principle we should preserve. 

### 2. Remove the proposed `workflow` database migration unless implementation proves it necessary

The plan currently says:

```text
007_workflow.sql # NEW — workflow tables (if needed)
```

I would change that to:

```text
db/
    existing authority/core migrations

# NO workflow migration by default.
# Introduce workflow persistence only after determining that
# workflow state cannot be represented as a service/read model.
```

This is important because the plan already has:

```text
service/workflow/
```

and an HMAC workflow token.

You do not want:

```text
workflow state
+
workflow token
+
kernel state
+
DB workflow state
```

all becoming competing sources of truth.

DealForge's useful lesson is that workflow continuity can exist outside the authoritative domain state, while consequential mutations revalidate trusted state before proceeding. 

So I would establish:

```text
Kernel DB state
      = authority truth

Workflow service
      = process state

Token/session
      = workflow continuity

UI
      = presentation

Executor
      = execution result
```

No ambiguity.

### 3. Do not put evidence references and debt into the authority token as authoritative data

The proposed token contains:

```go
Evidence []EvidenceRef
Debt     []string
Authority *AuthorityRef
```

I would change the design.

Those fields can become **stale copies of authoritative state**.

Instead, a workflow token should contain only enough information to identify and continue the workflow:

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

Then:

```text
token
   ↓
identify workflow
   ↓
read current state
   ↓
re-evaluate policy
   ↓
read current evidence/debt/authority
   ↓
perform operation
```

This follows the strongest lesson in the DealForge material: workflow state is continuity, not execution authority, and current trusted state is revalidated before consequential mutation. 

This is one of the places I would be particularly strict with the coding agent.

---

# 4. Actor classification must be separated from authentication

The plan is correct to introduce:

```go
HUMAN
AGENT
SYSTEM
```

and a central tool policy registry. 

But the implementation instructions should explicitly say:

```text
ActorType != Identity

ActorType:
    WHAT KIND OF ACTOR IS THIS?

Authenticated Principal:
    WHO IS THIS?

Authentication:
    HOW DO WE KNOW?
```

The existing Solvent design deliberately does not claim caller authentication at the kernel boundary.

Therefore:

```text
HTTP / MCP / deployment boundary
        ↓
authenticated principal context
        ↓
ActorType + principal ID
        ↓
service policy
        ↓
kernel
```

Do not let:

```json
{"actor":"HUMAN"}
```

be sufficient proof that the caller is human.

The security model must say this explicitly.

---

# 5. Risk scoring needs a harder boundary

The plan proposes:

```go
EvidenceQuality {
    Confidence float64
}
```

and scoring.

That is fine **as a product-layer projection**, but this sentence needs to be non-negotiable:

> **A score is advisory; it can never create, extend, or substitute for authority.**

For example:

```text
Risk Score = 98
Authority  = absent

RESULT = DENIED
```

and:

```text
Risk Score = 42
Authority  = valid

RESULT = AUTHORIZED
```

subject to policy.

This distinction is crucial because otherwise the product gradually turns "confidence scoring" into an implicit authorization system.

The competitive analysis specifically identifies risk scoring as something AegisFlow does above its workflow, while Solvent's hard promotion/authority gates remain separate. 

---

# I would also change the implementation order

The uploaded plan currently goes:

```text
Service
→ adapters
→ UI
→ human boundary
→ executor
→ demos
```

I would make one architectural change:

```text
1. Boundary audit
2. Service interfaces
3. Actor/tool policy
4. Workflow orchestration
5. Preparation/revalidation boundary
6. Audit projection
7. Executor contract
8. ONE real adapter/executor
9. UI
10. Demo platform
```

Why?

Because **UI before the authorization/service boundary is dangerous**.

We explicitly decided that the UI must not become a parallel authorization engine. The service layer has to exist first.

The uploaded plan's own UI requirements make clear that the dashboard is supposed to expose why actions are allowed or denied.  Therefore the UI should consume the service's decision model rather than invent one.

---

# One more important correction: don't build all five demos as five implementations

The plan currently gives each scenario its own Go file:

```text
agentjacking.go
lying_agent.go
stale_auth.go
confused_deputy.go
legitimate.go
```

That's fine for scenario definitions, but they must all exercise the **same product APIs and policy machinery**.

The architecture should be:

```text
              Scenario Framework
                     |
       ┌─────────────┼─────────────┐
       ↓             ↓             ↓
 Agentjacking     Lying Agent    Wrong Target
       \             |             /
        \            |            /
         └────── common services ─┘
                       |
                  Solvent kernel
```

Not:

```text
scenario A → custom logic
scenario B → custom logic
scenario C → custom logic
```

Otherwise the demos become five bespoke security applications.

---

# The MVP should remain narrower than this plan suggests

The uploaded plan is already much better than trying to copy AegisFlow wholesale. It explicitly defers multi-tenancy, a generalized policy language, dozens of integrations, deep IAM, cryptographic attestation, global replay prevention, and other large features. 

I would keep that list.

But I'd also change one thing:

### Don't build document/signature infrastructure yet.

The first commercial proof should be:

```text
Agent
 ↓
Evidence
 ↓
Review
 ↓
Authority
 ↓
GitHub/real executor
 ↓
Audit
```

That is enough to demonstrate that Solvent governs **real consequential work**.

Document generation and e-signature are useful later, and the DealForge integration model gives us a good adapter pattern when they become commercially justified. 

---

# The commercial MVP I would actually lock

```text
                     SOLVENT
        Portable Authority Layer
                     │
        ┌────────────┴────────────┐
        │                         │
   AGENT / TOOLS             HUMAN OPERATOR
        │                         │
        └────────────┬────────────┘
                     ↓
              SERVICE LAYER
        ┌────────────┼────────────┐
        │            │            │
     Workflow      Policy      Evidence
        │            │            │
        └────────────┼────────────┘
                     ↓
              SOLVENT KERNEL
                     ↓
                COCKROACHDB
                     │
             authority decision
                /          \
             DENY           ALLOW
              │               │
              │          EXECUTOR
              │               │
              │          GitHub/etc.
              │
          audit reason
```

The open-source story then becomes:

```text
Add a new adapter      → no kernel change
Add a new executor      → no kernel change
Add customer policy     → no kernel change
Add new workflow        → no kernel change
Add new UI              → no kernel change

New security primitive?
→ prove why existing kernel cannot express it
→ ADR
→ DB/Lean/security review
→ only then modify kernel
```

That is the architecture I would now treat as the **actual implementation contract**, rather than simply following the uploaded plan literally.

The plan is therefore **approved in direction, but I would revise those five areas before the coding agent starts**. The uploaded plan itself confirms the central objective: commercially useful workflow, visible security guarantees, clean extension points, five deterministic demos, and open-source extensibility while keeping the trusted core small. 

