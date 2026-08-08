After reading Claude's cGh analogy, I think it's fundamentally correct, but I would make one important shift.

Claude's three primitives are:

* **Provenance**
* **Revision**
* **Transaction**

Those are excellent **architectural primitives**. 

However, they are **not** what judges will remember.

Judges don't remember architecture.

They remember **systems**.

So I'd keep Claude's Venn diagram for the README and technical write-up, but I'd build the **demo** around a different set of three overlapping systems.

---

# The Three Systems (Judge Mental Model)

Instead of talking about internals, I'd organize the demo around these three visible systems:

```
                 Agent
          (reasoning & planning)

             /           \

      Evidence        Database
   (changing reality) (correctness)

             \           /

         Transactional
         Belief Ledger
```

or, more explicitly,

```
            AGENTS

     "I want to act"

          ∩

        EVIDENCE

   "Reality changed"

          ∩

     TRANSACTIONS

"The database decides"

          ↓

     TRUSTED ACTION
```

This is much easier to internalize than "provenance."

---

# The Demo Story

I think the etcd pivot actually simplifies everything.

We are **not demonstrating etcd**.

We are demonstrating **decision correctness using etcd's history as reality.**

The story becomes:

---

## Opening (10 seconds)

> Distributed databases learned years ago not to trust stale state.

> AI agents still do.

---

## Show bchat

This is your existing UI.

Nothing flashy.

Just clean.

```
Chat

Graph

Evidence

Timeline
```

The judge immediately recognizes

> "Okay, this is an agent."

---

## Agent receives belief

```
Belief

etcd v3.5

Production ready.
```

Graph node appears.

---

## Evidence arrives

Real etcd issue.

Real maintainer.

Real timestamp.

Real reopening.

No synthetic data.

The evidence node appears.

---

## Graph changes

This is important.

Don't dump text.

Animate.

```
Belief

↓

yellow

↓

red

↓

retracted
```

Edges move.

Dependents highlight.

---

## Agent still wants to act

```
Recommend upgrade?
```

YES

---

## CockroachDB refuses

Boom.

The graph changes again.

Intent disappears.

Receipt appears.

```
Rejected

Belief no longer promotable.
```

---

## Split screen

Left

naive memory

Right

Solvent

Left

acts.

Right

refuses.

---

## Final screen

```
Wrong action

1

vs

0

```

or

```
Unsafe upgrade

Allowed

vs

Prevented
```

---

# Then...

This is where everyone else stops.

We don't.

---

# Playground

This is where I think bchat becomes a huge advantage.

Imagine the judge watches the demo.

Then

```
task playground
```

opens bchat.

Now they can

change reality.

---

Example

```
> reopen issue #13766
```

Graph changes.

---

```
> add evidence
```

Graph changes.

---

```
> ask agent

Should we recommend this release?
```

Agent reasons.

Ledger checks.

Database decides.

---

The judge now understands

> "This isn't scripted."

---

# Taskfile Experience

This becomes the front door.

```
task doctor

task setup

task playground

task demo

task verify

task clean
```

Nothing else.

---

# What exactly are we shipping?

I think we should think of it as **five products**, not one.

## Product A

Kernel

Already done.

---

## Product B

Demo

Three minutes.

---

## Product C

Playground

Interactive.

Change evidence.

Watch beliefs change.

---

## Product D

Proof

M0

M1

M2

Receipts.

---

## Product E

Developer Experience

Taskfile

README

Architecture

One-command install.

---

# Revised Phases (Aug 18 Deadline)

Given your deadline, I would compress everything into **four execution phases**.

## Phase 1 (Today–Tomorrow)

**Freeze the experience**

Goal:

Nobody writes code.

Deliverables:

* Demo storyboard
* Playground storyboard
* README outline
* Taskfile command map
* Architecture diagrams
* Final Venn diagram

Output:

Everything frozen.

---

## Phase 2 (Core Demo)

Goal:

Add the **Transactional Belief Ledger** into bchat.

Deliverables:

* Ledger integration
* etcd evidence adapter
* Graph
* Intent cancellation
* Split-screen comparison

Nothing more.

At the end

```
task demo
```

must work.

---

## Phase 3 (Judge Experience)

Goal:

Make it delightful.

Deliverables:

* `task setup`
* `task doctor`
* `task playground`
* `task verify`
* README
* Screenshots
* Animations
* Architecture diagram

This phase is about polish, not new features.

---

## Phase 4 (Submission)

Deliverables:

* 3-minute video
* Technical write-up
* Devpost
* Final repository cleanup
* Buffer for bugs

---

# I would slightly change our north star

Until now we said:

> **Memory is refusing to act on what is no longer true.**

I think the etcd story lets us make it even stronger.

The project is really about this:

> **Distributed systems learned decades ago that stale state corrupts decisions. Solvent brings that same discipline to AI agents.**

That sentence immediately explains:

* why CockroachDB matters,
* why the belief ledger exists,
* why etcd is the demo corpus,
* why bchat is the UI,
* and why this is different from yet another RAG chatbot.

If we can make a judge walk away with **that** mental model—and then let them run `task playground` to convince themselves—it will be far more memorable than any amount of additional engineering sophistication.

