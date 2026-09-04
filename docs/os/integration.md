Yes. **At the current Solvent v0 state, we are still aligned with the “small kernel, extensions at the edge” philosophy—but with an important refinement: adapters are only one extension mechanism.** The architecture has evolved into a layered model where the kernel remains small, while different kinds of extension sit at different boundaries.

The v0 technical material explicitly says the product decision was to **reduce the v0 boundary rather than continually expand the kernel**. It also identifies infrastructure/deployment concerns, future analytical capabilities, and other deferred capabilities as things that do not automatically belong in the kernel. 

## The Linux-kernel analogy is directionally right

I would formulate the Solvent philosophy as:

> **Keep the kernel small, deterministic, transactionally authoritative, and domain-generic. Push ecosystem-specific behavior outward.**

That is very much what the Agentjacking work validated.

We deliberately refused to put:

```text
Sentry
Agentjacking
Sentry source types
Sentry debt rules
Sentry detection
```

into the generic core.

Instead:

```text
Sentry
   ↓
internal/agentjacking
   ↓
generic Solvent evidence/belief types
   ↓
kernel
```

That is exactly the kind of boundary discipline you want.

The current Agentjacking implementation demonstrates this particularly well: the adapter owns Sentry parsing and command observation, while the generic Solvent machinery remains unaware of Sentry. 

But **“everything is an adapter” would be too narrow**.

---

# I would define five extension mechanisms

## 1. Adapters — the ecosystem ingress/egress mechanism

This is the most obvious one.

Examples:

```text
Sentry adapter
GitHub adapter
Datadog adapter
MCP adapter
REST adapter
A2A adapter
executor adapter
```

Their job is translation:

```text
external protocol/domain
        ↓
generic Solvent contract
```

The v0 docs explicitly describe MCP as a thin adapter rather than an identity provider. 

And the authority architecture explicitly separates:

```text
Service
Kernel
External Adapters
```

with MCP and future REST sitting outside the kernel. 

This is the **primary ecosystem extension mechanism**.

### Rule

An adapter may know about:

* Sentry;
* HTTP;
* MCP;
* GitHub;
* a cloud provider;
* a particular executor.

The kernel should not.

---

# 2. Service layer — policy/composition extension

This is the important mechanism beyond adapters.

The v0 architecture explicitly introduced `internal/service/` to handle higher-level authority semantics such as:

```text
set-level AND over justifications
policy floors
four-role enforcement
authorization-target matching
```

while leaving atomic transactional transitions to the kernel. 

The intended architecture is:

```text
          SERVICE LAYER
       policy / composition
              │
        ┌─────┴─────┐
        ▼           ▼
     KERNEL      ADAPTERS
 atomic facts    protocols
        │
        ▼
       DB
```

This is **not kernel bloat**.

The key distinction is:

### Kernel

Answers:

> What state transitions are transactionally valid?

### Service

Answers:

> How do we compose those primitive operations into a meaningful authority workflow?

For example:

```text
Approve()
```

may involve:

```text
lock target
→ validate target
→ validate justifications
→ evaluate policy floor
→ create snapshot
→ activate authority
```

The service coordinates that workflow, while the kernel supplies the transactional primitives and DB guarantees.

This is very similar to the Linux ecosystem distinction between a small core primitive and higher-level subsystems/services.

---

# 3. Database constraints — extension by invariants

This is a particularly Solvent-specific mechanism.

Some behavior should not be implemented in Go at all.

Instead:

```text
schema
+
FKs
+
CHECK constraints
+
UNIQUE constraints
```

make invalid states structurally impossible.

For example, v0's:

```text
UNIQUE(target_id)
```

means a target can be activated only once. 

And the authority model uses constraints such as:

```text
belief/status relationship
ON UPDATE CASCADE
approval/target relationships
activation uniqueness
```

The v0 design explicitly says the goal is to derive durable facts, relationships, impossible states, transaction boundaries, and then relational constraints. 

This is an **extension mechanism in the sense that behavior is extended declaratively**, without making the kernel larger.

But I would not call arbitrary schema growth an "extension API." It should remain tightly controlled.

The rule should be:

> **Add a DB invariant only when a new security property genuinely requires a new durable fact or impossible state.**

That is exactly the discipline the v0 work established.

---

# 4. Policy/configuration — extension by data rather than code

This is the next important mechanism.

Your v0 architecture already separates:

```text
minimum safety floor
```

from:

```text
customer policy
```

The planning material says policy floors and obligation sets are evaluated by the service layer, while customer policy can define obligation sets within Solvent's minimum safety floor. 

That suggests a powerful principle:

```text
kernel semantics
        +
policy data
        =
different customer behavior
```

without modifying kernel code.

This is fundamentally different from an adapter.

An adapter changes **how the outside world speaks to Solvent**.

A policy changes **what Solvent accepts within its generic authority model**.

For example:

```text
Customer A:
high-risk deployment requires 3 obligations

Customer B:
high-risk deployment requires 5 obligations
```

Potentially the kernel remains unchanged.

That is an important scalability mechanism.

---

# 5. Deployment/executor integrations — extension outside Solvent

Another category is deliberately **outside** the core.

The v0 documentation is explicit that concerns such as:

* VM isolation;
* network controls;
* credential rotation;
* authentication infrastructure;
* executor isolation;

are infrastructure/deployment concerns, not automatically kernel responsibilities. 

Likewise:

```text
Authorize ≠ Execute
```

is a foundational boundary.

The v0 architecture's lifecycle explicitly ends with:

```text
Authorize
    ↓
Execute outside kernel
```

and the authority model treats authorization as read-only verification rather than an execution mechanism. 

So you can have:

```text
Solvent
   ↓
ALLOW
   ↓
executor adapter
   ↓
Kubernetes
AWS
Cloudflare
GitHub Actions
internal deployment system
```

without making the kernel understand Kubernetes, Cloudflare, etc.

That is another major extension mechanism.

---

# Put together, the architecture is actually this

I would now draw Solvent this way:

```text
                  EXTERNAL ECOSYSTEM
 ┌─────────────────────────────────────────────┐
 │                                             │
 │ Sentry   GitHub   MCP   REST   A2A          │
 │    │       │       │     │     │            │
 └────┼───────┼───────┼─────┼─────┼────────────┘
      │       │       │     │     │
      └───────┴───────┴─────┴─────┘
                    │
               ADAPTERS
                    │
                    ▼
          ┌─────────────────────┐
          │   SERVICE LAYER     │
          │                     │
          │ policy composition  │
          │ role enforcement    │
          │ authority matching  │
          └──────────┬──────────┘
                     │
                     ▼
          ┌─────────────────────┐
          │    SOLVENT KERNEL   │
          │                     │
          │ atomic transitions │
          │ transactions        │
          │ durable facts       │
          │ generic primitives  │
          └──────────┬──────────┘
                     │
                     ▼
          ┌─────────────────────┐
          │   COCKROACHDB       │
          │                     │
          │ FKs / CHECK / UNIQUE│
          │ impossible states   │
          └─────────────────────┘
```

And then beside it:

```text
          DEPLOYMENT / EXECUTOR WORLD
                     ▲
                     │
              execution adapter
                     │
                     │
                real systems
```

This is cleaner than saying "Solvent extends via adapters."

---

# The critical distinction: what is allowed to grow?

I would establish an explicit **kernel-growth hierarchy**.

### Easy to extend — no kernel change

```text
New Sentry integration
New MCP client
New REST API
New cloud executor
New A2A integration
New customer policy
New reporting UI
New deployment tooling
```

These should normally be implemented outside the kernel.

### Moderate threshold — service layer

```text
new authority composition
new policy evaluation
new multi-step governance workflow
new role orchestration
```

These belong in a service/policy layer first.

### High threshold — kernel

Only add a kernel primitive when you can say:

> **A new security-critical durable fact or atomic state transition cannot be correctly represented using the existing kernel primitives.**

That's a very high bar.

### Highest threshold — schema/invariant

A schema change should require an even stronger argument:

> **The system must make a previously possible invalid state structurally impossible, and application logic is insufficient.**

That is why the v0 schema remained deliberately small. The docs explicitly describe the seven-object model as intentionally small and emphasize reducing the v0 boundary rather than expanding the kernel. 

---

# This gives you a much better "Linux kernel" analogy

I wouldn't literally try to clone Linux's architecture; the domains are very different.

But the **engineering philosophy** is extremely useful:

### Linux-like principle

> Keep the privileged core minimal and stable; place hardware/vendor/filesystem/network-specific behavior at well-defined interfaces.

### Solvent equivalent

> Keep the authority kernel minimal and generic; place vendor/protocol/domain-specific behavior behind adapters, policy/service layers, and executor integrations.

So:

```text
Linux
kernel
  +
drivers
  +
filesystems
  +
network stacks
  +
userspace

Solvent
kernel
  +
adapters
  +
service/policy
  +
executors
  +
clients
```

The analogy becomes particularly strong because both systems are trying to establish a **small trusted computing base**.

---

# The Agentjacking work is actually a good architectural test

The Agentjacking feature could easily have produced:

```text
normalize.Sentry
derive.Sentry
kernel.Sentry
Sentry debt semantics
Sentry schema
```

Instead it became:

```text
internal/agentjacking
        ↓
generic evidence
        ↓
existing kernel
```

That is evidence that the architecture is working.

The current technical writeup explicitly captures the principle:

> “Sentry is an input format to an adapter, not a concept that Solvent's kernel needs to understand.” 

That's exactly the behavior you want to preserve as Solvent grows.

---

# One thing I would formalize now

I think Solvent would benefit from an explicit **extension rule** in its engineering guide:

> **Before adding functionality to the kernel, classify the requirement as one of: adapter, service/policy, executor/deployment, or invariant/kernel primitive. Kernel changes require evidence that the behavior cannot be expressed correctly at a lower-trust boundary.**

Then enforce a rough decision tree:

```text
Does it understand an external product/protocol?
        │
        ├── YES → Adapter
        │
        NO
        ↓
Is it orchestration/policy/composition?
        │
        ├── YES → Service / Policy
        │
        NO
        ↓
Is it execution/infrastructure?
        │
        ├── YES → Executor / Deployment
        │
        NO
        ↓
Does it require a new durable security fact
or atomic state transition?
        │
        ├── YES → Kernel
        │
        NO
        ↓
Do we need to make an invalid state impossible?
        │
        ├── YES → Schema / invariant
        │
        NO
        ↓
Don't add it.
```

That last line is important.

The v0 documents repeatedly emphasize that deferred features should be earned by real customer demand, operational pain, an observed security failure, or a clear product requirement—not theoretical completeness. 

## Bottom line

**Yes: Solvent v0 is still very much on the small-kernel trajectory.**

But the mature formulation is not:

> “Everything extends through adapters.”

It is:

> **“The kernel is the smallest trusted authority core; everything that does not require a new durable security fact or atomic state transition belongs outside it.”**

The primary extension mechanisms are:

**adapters → service/policy → executor/deployment → declarative DB invariants → kernel primitives only when absolutely necessary.**

That is actually stronger than the original “adapters only” idea, because it gives you multiple controlled escape valves for growth while maintaining a very high bar for kernel expansion. The v0 roadmap itself reinforces this: future identity, multi-tenancy, policy versioning, cryptographic attestation, global replay prevention, and execution-effect verification are explicitly deferred until real evidence earns their complexity. 

