Excellent.

I read it carefully.

**This is approved.**

Not "approved with edits."

Approved.

The only thing I would do is make one small process change before we hand work to Claude Code.

---

# Technical Lead Review

## Verdict

**PASS**

Score: **9.8/10**

This is no longer an architecture document.

It is exactly what we wanted:

> **an implementation contract.**

It has:

* frozen authority hierarchy
* frozen API
* frozen invariants
* frozen milestone ordering
* frozen acceptance criteria
* frozen escalation rules

Most importantly,

it explicitly tells Claude Code

> **STOP instead of redesigning.**

That is the biggest win in the document. 

---

# I found only ONE thing missing.

Not technical.

Project management.

---

## I would add

At the very top.

```text
Implementation Rule

Exactly one milestone may be in progress.

Claude Code is forbidden to implement future milestones.

When a milestone satisfies its acceptance criteria,
stop and wait for architectural review before beginning
the next milestone.
```

Why?

Because otherwise Claude Code will happily implement

```
M1

↓

M2

↓

M3

↓

M4
```

in one giant commit.

That's exactly what we don't want.

Small verified increments win hackathons.

---

# Therefore

I'm changing our workflow slightly.

Instead of giving Claude Code

the whole contract,

I'll give it

the contract

*

exactly ONE milestone.

---

Example.

Today

Claude Code receives

```
IMPLEMENTATION_CONTRACT.md

Only implement M0.

Ignore everything after M0.

Do not anticipate M1.

When M0 passes,

stop.
```

This is how experienced teams prevent accidental scope creep.

---

# Therefore...

## Work Package 02

**Owner**

Claude Code

---

### Objective

Complete **M0 only.**

Nothing else.

---

### Prompt

```
You are implementing a frozen architecture.

Read IMPLEMENTATION_CONTRACT.md.

Implement ONLY Milestone M0.

Objective

- Bring up CockroachDB.
- Apply db/001_schema.sql.
- Verify every acceptance criterion in M0.

Do NOT begin M1.

Do NOT write kernel code.

Do NOT redesign anything.

If any acceptance criterion fails,
stop immediately and report:

- exact criterion
- observed behavior
- expected behavior

Deliverables

- schema applied
- verification transcript
- SQLSTATE receipts
- CockroachDB version
- confirmation of READ COMMITTED support
- blocker report (if any)

When M0 passes,

STOP.
```

---

# Acceptance

Exactly those listed under M0.

Nothing more.

---

# Estimated effort

2–4 hours.

---

# Why M0?

Because M0 retires the **last unknown architectural risk**.

Specifically this line:

> Verify the composite FK + ON UPDATE CASCADE + CHECK interaction on the actual CockroachDB version.

That is the only remaining place where PostgreSQL reasoning might diverge from CockroachDB behavior, and your own escalation policy correctly identifies it as the highest-risk implementation question.  

If M0 passes,

I will personally declare the architecture **production-ready** and we never revisit it again.

---

## New standing rule (effective immediately)

From now on, whenever you type **"next"**, I will:

1. Review the completed milestone.
2. Either approve it or reject it.
3. Issue **exactly one** new work package.
4. Never allow parallel milestone implementation.

This is how we'll maximize the probability of finishing a polished, winning demo instead of a half-finished ambitious project.






I **love this addition**, with one refinement.

From now on, **Claude Code never codes first. It plans first.**

That mirrors how senior engineering teams work: implementation plans are cheaper to review than code, and architectural mistakes are much cheaper to catch before they're compiled.

I reviewed the attached M0 plan, and it's exactly the level of rigor I want. It is implementation-focused, explicitly bounded to M0, uses the CockroachDB documentation to distinguish documented behavior from behavior that must be empirically verified, and treats discrepancies as escalation events rather than redesign opportunities.  

## I want to formalize our workflow

Every work package will now have **four phases**.

### Phase 1 — Me (Technical Lead)

I issue the work package.

---

### Phase 2 — Claude Code

Before writing code, Claude Code must produce an implementation plan.

Requirements:

* Read the implementation contract.
* Read the relevant CockroachDB documentation (via MCP where applicable).
* Identify assumptions.
* Identify risks.
* Identify blockers.
* Produce a milestone-scoped implementation plan.
* **Do not write code yet.**

---

### Phase 3 — Me

I review the implementation plan.

I will check:

* Is it still within scope?
* Does it violate the frozen architecture?
* Did it misunderstand CockroachDB behavior?
* Is there unnecessary engineering?
* Is there hidden scope creep?
* Are the acceptance criteria sufficient?

Only after approval do we proceed.

---

### Phase 4 — Claude Code

Implement exactly the approved plan.

Nothing else.

---

## One more rule

I want Claude Code to classify every uncertainty.

Every uncertainty must be labeled as one of:

* **Verified** — confirmed by CockroachDB documentation or experiment.
* **Assumption** — explicitly stated because it isn't yet verified.
* **Risk** — could affect implementation.
* **Blocker** — must stop implementation.
* **Out of scope** — intentionally deferred.

Your attached M0 plan already does this well—for example, it clearly separates documented CockroachDB support from the empirical D3/D4 verification that M0 exists to perform. 

That vocabulary prevents people from accidentally treating guesses as facts.

---

## New standing instruction to Claude Code

Every future prompt I give Claude Code will begin with something like:

> **Before writing code, produce an implementation plan. Consult the CockroachDB documentation (via MCP where relevant) to verify behavior instead of assuming it. Distinguish verified facts, assumptions, risks, blockers, and out-of-scope items. Wait for approval before implementing.**

---

## This changes my role slightly

From now on I will review **two artifacts** for every milestone:

1. **Implementation Plan** (architecture/scope review)
2. **Implementation** (correctness review)

That gives us two gates instead of one, which is exactly what you want under a 12-day deadline.

## I think we've now reached a very healthy workflow

* **You** own the product and final decisions.
* **I** act as Technical Lead and control scope, sequencing, and reviews.
* **Claude Fable** owns architecture and implementation contracts.
* **Claude Code** owns planning, coding, compilation, and testing—but only after its plan is approved.

This minimizes architectural drift while letting Claude Code do what it's best at: turning a well-defined contract into working Go code.






Got it. I actually think this is the last piece we were missing.

I accept this as part of our operating procedure.

## Final Workflow (Frozen)

### 1. I issue the work package.

I decide:

* Owner (Fable or Claude Code)
* Objective
* Acceptance criteria
* Scope boundaries

---

### 2. Claude Code **does not code**.

It first produces an **Implementation Plan**.

Requirements:

* Read `IMPLEMENTATION_CONTRACT.md`
* Consult CockroachDB docs via MCP where relevant
* State verified facts
* State assumptions
* State risks
* State blockers
* State out-of-scope items
* Produce a milestone-scoped implementation plan
* Wait

---

### 3. I review the plan.

This review will **always** be in **Markdown**.

No exceptions.

The review has only two outcomes.

---

# APPROVED (with optional nits)

```markdown
# Review — Work Package XX

## Verdict

✅ APPROVED

## Strengths

- ...
- ...

## Nits (non-blocking)

1.
2.

## Scope Check

No architectural drift detected.

## Authorization

Proceed with implementation exactly as planned.
Address nits only if they are essentially free.
Do not expand scope.
```

---

# REWORK REQUIRED

```markdown
# Review — Work Package XX

## Verdict

❌ REWORK REQUIRED

## Blocking Issues

### B1

...

Reason

...

Required Change

...

---

### B2

...

Reason

...

Required Change

...

## Scope Check

Implementation may not begin.

Resubmit an updated implementation plan after addressing
all blocking issues.
```

---

Only these two outcomes exist.

There is no

> "Looks good."

There is no

> "Mostly okay."

There is no

> "Proceed but..."

Engineering deserves binary gates.

---

### 4. Claude Code updates the plan.

If rework is requested,

the cycle repeats.

No code.

---

### 5. Once approved

Claude Code implements

**exactly**

the approved plan.

No deviations.

No anticipatory work.

No redesign.

---

### 6. I review the implementation.

Again.

Markdown.

Binary.

```text
PASS

or

REWORK
```

---

# New rule

Implementation cannot exceed the approved plan.

If Claude Code wants to add

* caching
* helper packages
* abstractions
* refactors
* optimizations

that were not in the approved plan,

it must stop

and ask.

---

# Why I like this

This is essentially a lightweight architecture review board.

```
Work Package

↓

Implementation Plan

↓

Architecture Review

↓

Approval

↓

Implementation

↓

Implementation Review

↓

Next Package
```

Every architectural decision is made before code exists.

That's exactly how you keep a 12-day project from drifting.

---

## One addition I'd make

Every review I produce will end with a **Decision Log**.

For example:

```markdown
## Decision Log

- D-014: pgx/v5 accepted as the sole SQL driver.
- D-015: No kernel code permitted during M0.
- D-016: Composite FK behavior must be empirically verified.
```

This creates a running record of architectural decisions without reopening old discussions. If a later plan contradicts one of these decisions, I can reject it immediately by reference rather than re-arguing the point.

I think that closes the loop. We now have a disciplined workflow where architecture, implementation planning, coding, and review each have a clear owner and a clear gate.

