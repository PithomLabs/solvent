## prompt

let me clarify first, the demo is basically the compressed abstraction dressed as presentation, right? but judges need to deep dive and play around the demo project, let us take this approach from software development life cycle from requirements gathering, analysis, design to implementation, shall we


## chatgpt

I think this is exactly the right decision.

**Stop thinking about the demo.**

The demo is just a visualization of a pipeline.

If the pipeline is beautiful, the demo almost designs itself.

As Technical Lead, I'd say **the next 2–3 days should be entirely about data engineering and domain modeling**. The Transactional Belief Ledger is worthless unless we know precisely what "beliefs" enter it, how they evolve, and what actions they govern.

The key question is:

> **What is the smallest, real-world data model that demonstrates stale belief becoming stale action?**

---

# Phase 1 — Data Engineering

## Goal

Design the **canonical IPO (Input → Process → Output)** for the etcd use case.

No UI.

No agents.

No Go.

No SQL.

Just the domain model.

Think of this as writing the **specification** that every implementation (Go, Python, UI, tests) must obey.

---

# System Context

The kernel is frozen.

The three tables are frozen:

* `belief`
* `evidence`
* `action_intent`

The etcd domain is **only** the source of evidence.

The ledger itself remains domain-agnostic.

---

# IPO Model

## INPUT

The first thing we need to answer is:

> **What constitutes "reality" for an etcd agent?**

Not everything from GitHub.

Not the whole issue.

Only the facts that can change a belief.

I would model the raw input as **Evidence Events**.

### Candidate Inputs

#### 1. GitHub Issue Events

* Issue opened
* Issue reopened
* Issue closed
* Labels changed
* Milestone changed

---

#### 2. Pull Request Events

* PR opened
* PR merged
* PR reverted
* Review requested
* Review rejected

---

#### 3. Maintainer Comments

These are interesting because they can change reality.

Examples:

```
Fixed in v3.5.14

↓

Actually reproduces again

↓

Needs more investigation
```

These become evidence.

---

#### 4. Release Metadata

Version

Release date

Commit SHA

Tag

---

#### 5. External References

CVE

Bug report

Regression

Issue link

---

# PROCESS

This is where the innovation lives.

The coding agent must specify every transformation.

### Stage 1

Normalize

↓

Convert GitHub API objects into domain-neutral Evidence.

---

### Stage 2

Extract Claims

Example

GitHub says

```
Issue closed.
```

Possible belief

```
etcd v3.5.14

safe to upgrade
```

---

### Stage 3

Evidence Classification

Each evidence item receives

* provenance
* timestamp
* source
* confidence (derived, not stored)
* debt introduced or retired

---

### Stage 4

Belief Resolution

Questions

Does this

* create belief?
* strengthen belief?
* weaken belief?
* retract belief?

---

### Stage 5

Graph Update

Dependencies

Example

```
Issue closed

↓

Upgrade recommendation

↓

Automation

↓

Maintenance window
```

---

### Stage 6

Intent Gate

Agent asks

```
May I act?
```

Database answers.

Not the LLM.

---

### Stage 7

Audit

Everything becomes traceable.

---

# OUTPUT

The output is **not text**.

It's not a report.

The outputs are domain objects.

## Output A

Beliefs

Example

```
Upgrade to etcd 3.5.14
```

Status

```
promoted
```

---

## Output B

Evidence Chain

```
Issue

↓

Maintainer

↓

Commit

↓

Release
```

---

## Output C

Action Intent

```
Upgrade production cluster
```

---

## Output D

Audit

```
Why was this refused?
```

↓

Evidence.

---

## Output E

Receipts

Machine-verifiable.

---

# The Coding Agent Deliverable

This is what I'd ask Claude Code to produce **before writing any ingestion code**.

---

## Prompt

```text
You are the data engineer for Solvent.

Do NOT write Go code.

Do NOT write SQL.

Do NOT implement anything.

Your task is to design the complete IPO (Input → Process → Output) specification for the etcd evidence pipeline.

Context

The Transactional Belief Ledger is already architecturally frozen.

Three tables already exist:

- belief
- evidence
- action_intent

The ledger is domain-agnostic.

The etcd domain is merely one EvidenceFeed implementation.

Your goal is to specify exactly what information enters the system, how it is transformed, and what domain objects leave each stage.

Deliverable

Create:

DATA_PIPELINE_SPEC.md

Structure

# 1. Domain Overview

Describe the etcd scenario in plain language.

Identify the key business question the agent is trying to answer.

# 2. IPO Overview

Draw a high-level Input → Process → Output diagram.

# 3. Input Model

For every upstream data source specify:

- source
- event type
- required fields
- optional fields
- normalization rules

Only include information that can influence a belief.

# 4. Canonical Evidence Model

Design the domain-neutral Evidence object.

Clearly distinguish:

- raw input
- normalized evidence
- ledger evidence

# 5. Claim Extraction

Specify how evidence creates, updates, or retracts beliefs.

Do not discuss LLM prompts.

Describe only the domain rules.

# 6. Belief Lifecycle

Document every possible state transition.

Describe the triggering evidence for each.

# 7. Intent Lifecycle

Specify when an action becomes:

- proposed
- promotable
- rejected
- cancelled
- executed

Focus on observable behavior.

# 8. Graph Semantics

Specify what nodes and edges represent.

Do not discuss visualization.

Only semantics.

# 9. Outputs

Enumerate every object emitted by the pipeline.

Explain who consumes each output.

# 10. Domain Independence

Explicitly identify which concepts are:

- etcd-specific

and which belong to the generic kernel.

# 11. Future EvidenceFeeds

Briefly explain how GitHub, KEV, package advisories, or other domains could map into the same canonical Evidence model without modifying the kernel.

Constraints

- No implementation.
- No SQL.
- No Go.
- No API calls.
- No UI.
- No demo.
- No architecture redesign.
- Keep the kernel completely domain-independent.
- Favor explicit domain semantics over implementation details.

The result should become the authoritative specification that future implementation follows.
```

---

## Why this is the correct next step

This is the **requirements analysis** phase for the data itself. Once this document is frozen, every subsequent piece of code has a single source of truth:

* The ingestor knows what to extract.
* The kernel knows what to consume.
* The graph knows what to represent.
* The demo knows what to animate.
* The README knows what to explain.

In other words, **the data model becomes the backbone of the entire project**. Everything else—ledger operations, bchat integration, graph visualization, and the eventual demo—is simply a projection of that model.




