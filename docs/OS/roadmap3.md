Yes. With the three documents considered together, I would **not turn the gaps list into a shopping list for Solvent**. The central architectural conclusion is that Solvent should remain the small authority kernel, while the harness supplies the controls around it.

The swarm article gives the strategic reason: oversight is moving toward the harness—sandbox, credentials, network, action logs, and monitoring—because agents can increasingly evade reasoning-based oversight.  The harness review then maps Solvent into that picture: Solvent is the database-enforced authority boundary, not the sandbox, credential broker, CoT monitor, or swarm orchestrator. 

So the roadmap should split into **what Solvent must strengthen** and **what the surrounding harness must supply**.

# Where we are now

The current Solvent kernel has reached a meaningful maturity point.

The important authority path is:

```text
AI / coding agent
        ↓
MCP
        ↓
Solvent kernel
        ↓
CockroachDB
```

The repository explicitly places transaction discipline and invariants at the kernel/database boundary, while the agent owns reasoning. 

And the recent remediation cycle was valuable because it exposed the importance of enforcing relationships such as:

```text
scenario ↔ belief
scenario ↔ intent
scenario + belief + action ↔ execution
```

rather than trusting callers.

The next gap is therefore **not “make Solvent bigger.”**

It is:

> **Make the small authority primitive more precise, then build the minimum harness contract around it.**

---

# Priority roadmap

## Priority 1 — Exact action/target authorization

This is the most important remaining **Solvent-side** gap in `gaps.md`.

Today the documented limitation is that a promoted belief can support a stated action without the approval itself being bound to that exact action and target. 

That is the logical next step after everything we just fixed.

The desired invariant becomes:

```text
belief
   ↓
approval
   ↓
EXACTLY:
   action
   target
   consequence parameters
```

Then execution becomes:

```text
requested action
requested target
        ↓
must exactly match
        ↓
approved authority
```

This is the natural evolution of Solvent's confused-deputy defense.

**Priority: P0**

And because this is another durable authority invariant, I would route it through the same **Kernel Growth Gate ADR** discipline before touching the kernel.

This should probably be the next major Solvent phase.

---

# Priority 2 — Define the harness trust contract

The biggest gap for real agent adoption is not actually MCP transport.

It is:

> **Who is allowed to invoke Solvent, and what is that caller allowed to do?**

`gaps.md` explicitly identifies three related gaps:

* no caller identity/authentication
* no built-in role restriction on tools
* tool restrictions delegated to the host/harness 

But the architectural documents also explicitly say authentication/authorization was intentionally rejected for the local stdio demo. 

So I would **not** solve this by stuffing authentication into the kernel.

Instead create a **Harness Contract**:

```text
Harness
 ├── identifies agent/session
 ├── decides which MCP tools are exposed
 ├── limits credentials
 ├── controls filesystem/network
 ├── records every tool invocation
 └── connects agent to Solvent
```

while:

```text
Solvent
 └── determines whether consequential state transition is authorized
```

This is precisely the division suggested by the swarm article and harness review.  

**Priority: P0 for the harness architecture, not a kernel feature.**

---

# Priority 3 — Tool capability profiles

Closely related, but distinct from authentication.

A swarm shouldn't necessarily receive all Solvent capabilities.

For example:

```text
Research agent
  → ledger
  → ingest_evidence

Review agent
  → ledger
  → ingest_evidence
  → perhaps propose/review-related operations

Operator / approval agent
  → promotion
  → authority creation

Execution agent
  → authorize/execute
  → no belief promotion
```

`gaps.md` explicitly says Solvent currently does not restrict which agents can call state-mutating tools. 

But this belongs **above the kernel**.

The harness should expose only the capabilities an agent needs.

The important principle:

> **Don't make Solvent answer “may this agent call this MCP method?” unless/ until Solvent becomes a remotely shared authenticated service.**

Today the harness answers that question.

**Priority: P0/P1 harness work.**

---

# Priority 4 — Make Solvent “swarm-safe” experimentally

The current documents are unusually honest here: concurrency has been tested through selected races, not through a real swarm-scale workload. 

This should be the next **evidence** phase.

Not:

> “support 1,000 agents.”

Instead:

> **prove that multiple independent agents can safely share the same authority substrate.**

Build a small adversarial/concurrency harness that creates:

```text
Agent A ─┐
Agent B ─┼──→ Solvent → CockroachDB
Agent C ─┤
Agent D ─┘
```

and exercises:

```text
same belief
same intent
different intents
promotion races
retraction races
evidence races
competing execution attempts
```

The goal is characterization, not a marketing claim of unlimited scalability.

This fits the existing engineering discipline: the repository already says the current concurrency evidence is only a few specific interleavings. 

**Priority: P1**

---

# Priority 5 — Harness escape-path demonstration

This is arguably the most important conceptual gap exposed by the swarm story.

Solvent cannot stop:

```text
Agent
  ↓
shell
  ↓
curl
  ↓
production API
```

without calling Solvent.

The documents are explicit about this. Solvent does not provide sandboxing, network isolation, or credential isolation. 

And the harness review explicitly says Solvent assumes the MCP/tool surface is already the path through which consequential actions occur. 

So don't try to solve this inside Solvent.

Instead, build a **reference harness demonstration** showing:

```text
WITHOUT harness restriction

agent
 ├── Solvent → safe gate
 └── shell/API → escape

WITH harness restriction

agent
 ├── allowed tools
 ├── controlled credentials
 ├── controlled egress
 └── Solvent → consequential authority
```

That would make Solvent's actual role incredibly clear.

**Priority: P1, but outside the kernel.**

---

# Priority 6 — Durable agent/action activity trail

Solvent already has a strong refusal/audit story. The documents emphasize that refusals and SQLSTATE evidence matter because they explain not only what happened but why. 

The missing piece for swarms is **cross-agent activity attribution**.

You eventually want:

```text
agent-17
session-abc
tool=solvent_authorize_action
belief=...
action=deploy
decision=refused
reason=...
```

and:

```text
agent-42
session-xyz
tool=...
...
```

That does **not necessarily mean identity belongs inside the kernel**.

The harness can attach agent/session identity to the activity envelope while Solvent continues to enforce authority independently.

The useful invariant is:

> **Attribution may identify who attempted something; attribution must never itself confer authority.**

That is consistent with the existing design's treatment of caller-declared metadata. `gaps.md` already recognizes that self-reported provenance is not proof. 

**Priority: P1.**

---

# Priority 7 — Evidence/proof packaging

There is another gap that matters more for enterprise adoption than for the kernel itself:

> some checks exist without a permanent reviewable transcript.

The documents explicitly acknowledge that some verification is code-backed but does not ship with a durable transcript. 

For a swarm, this becomes:

```text
100 agents
   ↓
10,000 proposals
   ↓
1,200 refusals
   ↓
300 approvals
   ↓
20 executions
```

A human should be able to reconstruct what happened.

This could be a **harness evidence recorder**, not necessarily a Solvent kernel feature.

**Priority: P1/P2.**

---

# Lower priority / explicitly defer

Several gaps in `gaps.md` should **not** pull Solvent off course yet.

### Remote MCP / authentication

Useful eventually, especially if Solvent becomes a shared service, but the current architecture deliberately keeps MCP local stdio. 

Don't build it just because the word “swarm” appeared.

### Packaging

Useful adoption work, but not a security primitive.

### Multi-region / large-scale swarm claims

Do the smaller concurrency characterization first.

### Multi-hop belief graphs

The existing engineering documentation deliberately treats this as future work rather than a current claim. 

### Client compatibility matrix

Explicitly rejected as unbounded work in the current design. 

### Web UI

Still unnecessary.

---

# So the roadmap I'd use now

```text
COMPLETED
─────────
Phase 4E
    End-to-end agent workflow

Plan 8
    Review reconciliation

Plan 8.1
    MCP trust-boundary clarification

Plan 8.4
    Scenario-binding kernel invariant

Plan 8.5
    DB behavioral verification

Plan 9.0
    Intent ownership + scenario isolation remediation

Current
    Fresh adversarial review → GREEN
```

Then:

```text
NEXT 1 — Exact Authority Binding
    approval → exact action + exact target
    Kernel Growth Gate
    confused-deputy elimination

NEXT 2 — Harness Contract
    caller identity
    tool capability profiles
    session/activity attribution
    explicit trust boundary

NEXT 3 — Swarm Concurrency Lab
    many agents
    adversarial interleavings
    shared-state stress
    measured limits

NEXT 4 — Escape-Path Harness
    sandbox
    credentials
    network/egress
    demonstrate Solvent + harness together

NEXT 5 — Evidence / Forensics
    durable agent activity
    reproducible transcripts
    investigation tooling

LATER
    remote MCP
    cloud verification
    packaging
    broader belief networks
    large-scale deployment characterization
```

## The key strategic shift

The three attached documents point toward a very clean positioning:

> **Solvent should not become the thing that watches the swarm. Solvent should be the thing the swarm cannot legitimately bypass when it wants to turn shared belief into consequential authority.**

The swarm article says the harness is the last controllable layer.  The Solvent material says the kernel/database is the final invariant boundary.  Put those together and the architecture becomes:

```text
┌─────────────────────────────────────────────┐
│                AGENT SWARM                  │
│ reasoning / collaboration / planning        │
└──────────────────────┬──────────────────────┘
                       │
┌──────────────────────▼──────────────────────┐
│                  HARNESS                    │
│ identity / tools / sandbox / credentials    │
│ network / activity / monitoring             │
└──────────────────────┬──────────────────────┘
                       │
┌──────────────────────▼──────────────────────┐
│                 SOLVENT                     │
│ belief → authority → exact action           │
│ deterministic state transitions             │
└──────────────────────┬──────────────────────┘
                       │
┌──────────────────────▼──────────────────────┐
│               COCKROACHDB                   │
│ durable invariants / concurrency / state    │
└─────────────────────────────────────────────┘
```

That is probably the most important roadmap insight: **the next major Solvent kernel feature is exact authority binding; the next major ecosystem feature is a reference harness contract.** Everything else should be judged against those two tracks rather than allowed to turn the kernel into a general-purpose agent security platform.



---

Yes—with an important distinction.

### What I mean by “harness”

I mean the **agent runtime/orchestration environment around Solvent**, not Solvent itself.

That can absolutely include things like:

* OpenClaw
* Hermes
* Claude Code
* Codex
* Antigravity
* Kilo / swarm orchestrators
* your own custom agent runner

The terminology gets confusing because some of these products are simultaneously **agent + harness + tool host**.

Conceptually:

```text
Agent/model
  = reasoning, planning, deciding what to attempt

Harness/runtime
  = what the agent is actually allowed to touch
  = tools exposed
  = credentials
  = filesystem
  = network/egress
  = sandbox
  = process execution
  = session identity
  = tool-call/activity recording

Solvent
  = durable authority gate
  = belief/review state
  = authorization
  = exact consequential-action authorization

CockroachDB
  = final durable invariant/concurrency boundary
```

So Claude Code, Codex, OpenClaw, Hermes, etc. can be **the environment hosting Solvent**, but they aren't necessarily identical in architecture. The useful abstraction for us is simply:

> **Whatever sits between the model and the outside world and controls its capabilities is the harness.**

That is consistent with the harness analysis: Solvent is deliberately not the sandbox, credential broker, network monitor, CoT monitor, or swarm orchestrator. 

The current `gaps.md` makes the same division: identity, tool restrictions, sandboxing, hosting, and scale-testing are largely responsibilities of the host/harness rather than the kernel. 

---

# Is exact authority binding the last kernel primitive?

**I think it should be the last *planned* kernel expansion for the current architecture.**

Not “the kernel can never grow again,” but:

> **After exact authority binding, freeze the kernel unless a new concrete security invariant proves that an additional atomic primitive is unavoidable.**

That is consistent with the philosophy we've been enforcing throughout this review cycle.

Today the progression is roughly:

```text
belief
  ↓
review debt
  ↓
promotion
  ↓
intent
  ↓
claim
  ↓
execution
```

We've progressively tightened each boundary.

The remaining conceptual gap identified in `gaps.md` is that authorization can still be too broad: a promoted belief can potentially support an action different from the one actually reviewed. 

So exact authority binding would make:

```text
Approved Authority
    =
    (scenario,
     belief,
     action,
     target,
     frozen consequence parameters)
```

rather than merely:

```text
Approved Authority
    =
    promoted belief
```

Then the execution invariant becomes:

```text
requested action/target
        ==
approved action/target
        ==
intent action/target
        ==
frozen consequence parameters
```

That closes the **confused-deputy** class that was explicitly identified as still outstanding in the earlier agentjacking material. The older demo documentation described exact approved target/action binding as the forthcoming strengthening of the pre-v0 gate. 

## And then I would freeze the kernel

After that, the architecture should become deliberately asymmetric:

```text
              ┌─────────────────────────┐
              │     Agent / Swarm       │
              │ reasoning, planning     │
              └───────────┬─────────────┘
                          │
              ┌───────────▼─────────────┐
              │        Harness          │
              │ identity                │
              │ tool permissions        │
              │ sandbox                 │
              │ credentials             │
              │ network/egress          │
              │ activity                │
              └───────────┬─────────────┘
                          │
                    MCP / adapter
                          │
              ┌───────────▼─────────────┐
              │        Solvent          │
              │ belief                  │
              │ review                  │
              │ authority               │
              │ exact action binding    │
              │ transactional rules     │
              └───────────┬─────────────┘
                          │
              ┌───────────▼─────────────┐
              │      CockroachDB        │
              │ invariants / concurrency│
              └─────────────────────────┘
```

The next work then moves **outward**, not inward:

### Kernel

Finish exact authority binding → freeze.

### Harness contract

Define how OpenClaw / Hermes / Claude Code / Codex / your own runner should expose tools, credentials, identity, sandboxing, and network access.

### Swarm evidence

Test many agents concurrently against the same Solvent substrate.

### Reference harness

Build one exemplary integration showing:

```text
agent swarm
   ↓
restricted capabilities
   ↓
Solvent authority gate
   ↓
real external action
```

That would directly answer the “who watches the swarm?” problem without turning Solvent into the thing that watches everything.

The reason I favor this boundary is precisely because the source material says Solvent's strength is its **narrow, database-enforced authority primitive**, while sandboxing, network isolation, credential scoping, and agent monitoring belong to the harness. 

So my recommendation is:

> **Yes: exact authority binding should be the final kernel feature we deliberately plan now. After that, institute a kernel freeze and spend the roadmap on making Solvent the authority substrate that arbitrary agent harnesses can plug into.**

That gives you a very clean story: **small kernel, powerful harness ecosystem.**
