We are at a clean architectural transition point.

## Where we are

The foundation is now frozen:

```text
Phase 4A — Canonical API contract          ✅
Phase 4B — Service/API implementation      ✅
Phase 6.1–6.4 — Security hardening         ✅
Phase 4C — OpenAPI + references + docs     ✅
             Kilo Code independent GO
```

Phase 4C established the consumable external boundary:

```text
                ┌─────────────────────┐
                │   SOLVENT KERNEL    │
                │  small + trusted    │
                └──────────┬──────────┘
                           │
                    canonical API
                           │
                 OpenAPI / REST / MCP
                           │
                    extension plane
```

Kilo's final review confirmed the kernel/API/schema/service boundaries remained intact and the extension plane sits above the kernel. 

---

# Now: Phase 4C+

This is the first **real-world side-effect phase**.

The purpose is not to add more authorization semantics. It is to prove that Solvent can sit **directly in front of a consequential external operation**.

The target is:

> **One real GitHub executor, end to end.**

The intended chain is:

```text
AI agent / workflow
       ↓
GitHub evidence
       ↓
Solvent evidence + belief
       ↓
target / approval
       ↓
current-state authorization
       ↓
Executor
       ↓
GitHub Actions / deployment API
       ↓
external side effect
       ↓
execution result
       ↓
audit
```

The current Phase 4C plan explicitly defines Phase 4C+ as the point where the real GitHub executor is implemented and authorization → execution → audit is demonstrated. 

## Why 4C+ matters

Until now, Solvent proves:

```text
"May this happen?"
```

4C+ proves:

```text
"Only after Solvent says yes does the real thing happen."
```

That is a much more consequential claim.

It also forces us to validate the extension architecture under real conditions:

```text
Solvent
  ↓
executor contract
  ↓
GitHub
```

rather than assuming the abstraction works because the interfaces compile.

---

# What 4C+ should NOT become

This is where discipline matters most.

Do **not** turn the first executor into:

```text
generic workflow engine
generic plugin system
universal execution framework
multi-cloud platform
enterprise IAM
RBAC
multi-tenancy
dozens of providers
```

The first integration should be **one excellent proof**, not a platform.

The kernel still should not learn anything about GitHub.

The executor learns GitHub.

---

# The critical design question for 4C+

The biggest thing the planning phase needs to resolve is:

```text
What exactly is the trusted boundary between
Solvent authorization and external execution?
```

The eventual path should remain conceptually:

```text
request
  ↓
current authorization
  ↓
executor
  ↓
provider
```

and never:

```text
request
  ↓
executor
  ↓
"trust me, it was authorized"
```

We also already know there is **no atomic database + external-provider transaction**.

So the planner needs to be explicit about the unavoidable boundary:

```text
Solvent can guarantee:
    the executor was invoked only after
    successful current authorization

Solvent cannot guarantee:
    DB authorization and GitHub side effect
    commit atomically as one transaction
```

That distinction will be central to the adversarial review.

---

# The existing executor is intentionally minimal

The current contract is still:

```go
ActionFunc(ctx, params) (string, error)
```

The Phase 4C plan deliberately kept it that way. The richer interface was deferred until a real executor reveals what it actually needs. 

4C+ is therefore where we **earn** any executor abstraction changes.

The rule should be:

> Do not design `AuthorizationContext`, `ExecutionResult`, `AuditHandoff`, etc. in advance unless the real GitHub executor demonstrates that they are necessary.

This is exactly the extension philosophy we've locked in.

---

# What comes immediately after 4C+

Assuming the real executor succeeds:

```text
Phase 4C+
    ↓
real GitHub execution
    ↓
adversarial review
    ↓
Phase 4D — optional Web UI
```

The Web UI remains a **client of the canonical API**, never a new authority layer.

Conceptually:

```text
MCP ─────────┐
REST ────────┤
A2A ─────────┤
Web UI ──────┤
             ↓
        Service Layer
             ↓
        Kernel
```

The UI exists to make the system operationally understandable, not to become security infrastructure.

---

# After the UI

Then:

### Phase 4E — API/UI adversarial review

This is where we test the whole consumable product surface for:

```text
UI bypass
API bypass
cached authorization
misleading security state
REST/MCP/UI divergence
unsafe execution affordances
```

Again, the UI gets reviewed as a client, not as an authority engine.

---

# Then Phase 5

**Governance / Compliance is deliberately later.**

The architectural position remains:

> Solvent produces authoritative evidence of governed AI-driven decisions and actions.

Not:

> Solvent makes you compliant.

Only after real usage justifies it should we add:

```text
control mapping
evidence packages
compliance exports
exception management
governance workflows
```

These remain product/service capabilities above the kernel.

---

# The long-term picture

The roadmap is ultimately converging on:

```text
                       SOLVENT
                          │
                SMALL TRUSTED KERNEL
                          │
                 CANONICAL API
                          │
                 EXTENSION PLANE
           ┌──────────────┼──────────────┐
           │              │              │
      Protocols       Integrations    Executors
      MCP/A2A/REST    GitHub/K8s/AWS   real side effects
           │              │              │
           └──────────────┴──────────────┘
                          │
                    AI / Apps / Workflows
```

And the ultimate product thesis is:

```text
AI agent / app / workflow
        ↓
    "I want to do X"
        ↓
      Solvent
        ↓
"Is THIS principal allowed to perform
 THIS exact action on THIS exact target
 under CURRENT authority?"
        ↓
       YES/NO
        ↓
   external system
```

That is how Solvent can become an authorization layer for **agents, applications, workflows, and everything in between** without turning the kernel into a giant monolith.

## Immediate roadmap

So, in practical terms:

```text
NOW
Phase 4C ✅ CLOSED
        ↓
NEXT
Phase 4C+ — plan first
        ↓
        real GitHub executor
        ↓
        auth → execution → audit
        ↓
        independent Kilo review
        ↓
Phase 4D — optional Web UI
        ↓
Phase 4E — API/UI adversarial review
        ↓
Phase 5 — earned governance/compliance
        ↓
Phase 6 — demo platform
        ↓
Phase 7 — OSS polish
```

**The next task is therefore not coding yet. It is planning Phase 4C+ carefully around the real execution boundary and TOCTOU semantics.** That is the next place where the architecture gets tested against reality.
