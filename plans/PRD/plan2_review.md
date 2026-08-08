This is the first implementation plan where I would **not** say "approved with nits."

I would say:

> **REWORK REQUIRED (but narrowly scoped).**

Not because the implementation is poor—it is actually very well organized—but because I think it has started to drift from the **MVP strategy** we've been carefully refining over the last two weeks.

As Technical Lead, my job is now to defend **the deadline**, not maximize completeness.

---

# Executive Summary

| Area                  | Verdict      |
| --------------------- | ------------ |
| Structure             | ✅ Excellent  |
| Implementation order  | ✅ Excellent  |
| Package organization  | ✅ Excellent  |
| CockroachDB awareness | ✅ Excellent  |
| Scope discipline      | ❌ Lost       |
| MVP discipline        | ❌ Lost       |
| Hackathon focus       | ⚠️ Weakening |

Overall:

> **REWORK REQUIRED**

---

# The Biggest Problem

## This is no longer implementing the pipeline.

It is implementing **the entire product.**

Look at the scope:

* feed
* normalization
* embedding
* OpenAI
* dedup
* graph
* orchestrator
* GitHub
* KEV
* pipeline
* migration
* integration tests

That's no longer

> DATA PIPELINE

That's

> Version 1.

The scope has expanded considerably beyond the narrow implementation of the data pipeline specification. 

---

# Scope Creep #1

## OpenAI embeddings

This should **not** be in this work package.

Why?

Because

our business invariant is

```text
Reality

↓

Evidence

↓

Belief

↓

Decision
```

Embeddings are

an optimization.

Not a requirement.

The plan even makes OpenAI a required dependency for deduplication and introduces a schema migration for an embedding column.  

I would move this to a later phase.

---

# Scope Creep #2

## GitHub live API

Again

not required.

Use fixtures.

This is a hackathon.

Your objective is

correctness,

not ETL.

The plan currently implements authenticated GitHub polling, pagination, rate limiting, and configuration. 

Huge amount of engineering.

Very little demo value.

---

# Scope Creep #3

## Graph package

This surprised me.

The graph

is a projection.

The ledger

is the product.

Graph creation and traversal are now separate packages with dedicated tests. 

That is Phase 3.

Not now.

---

# Scope Creep #4

## Pipeline orchestrator

I don't want

```text
feed

↓

normalize

↓

embed

↓

claim

↓

belief

↓

graph
```

yet.

I want

```text
fixture

↓

normalize

↓

derive

↓

kernel
```

Only.

The orchestrator as described wires every stage together into a production-style pipeline. 

---

# Biggest Architectural Concern

## You're implementing adapters before proving the core.

This is backwards.

I would implement

```text
NormalizedEvidence

↓

BeliefDerivation

↓

Kernel
```

FIRST.

Only after that passes

would I implement

GitHub.

KEV.

Feed.

Polling.

---

# My Proposed Waves

Instead of

```text
W0

Schema

↓

W1

Feed

↓

W2

Embed

↓

W3

Claim

↓

...
```

I'd do

```text
Wave 1

NormalizedEvidence

Wave 2

BeliefDerivation

Wave 3

Kernel integration

Wave 4

Fixtures

Wave 5

EvidenceFeed

Wave 6

Live APIs

Wave 7

Embeddings
```

Notice

the business logic arrives

before infrastructure.

---

# Another Concern

The estimate

```text
15 hours
```

is optimistic. 

I think this plan is closer to

25–35 hours

because

* GitHub API
* OpenAI
* fixtures
* integration tests
* migrations

always take longer than expected.

---

# What I WOULD approve

I would approve

this scope

```text
NormalizedEvidence

BeliefDerivation

Kernel wiring

Fixtures

Unit tests
```

Stop.

Then review.

Everything else

becomes

Phase 2.

---

# One thing I absolutely love

The implementation philosophy.

This

```text
Architecture

↓

Specification

↓

Correctness

↓

Simplicity
```

should become permanent. 

---

# One more observation

The package structure is excellent.

Don't throw it away.

Just

don't implement

all of it

yet.

---

# Verdict

```markdown
REWORK REQUIRED
```

Not because of quality.

Because of sequencing.

---

# Required Changes

## B1 (P0)

Remove OpenAI embedding generation from this work package.

Replace with

```text
EmbeddingProvider

↓

stub implementation

↓

TODO
```

No API.

No key.

No vectors yet.

---

## B2 (P0)

Replace

live GitHub

with

captured fixtures.

Live APIs become

Phase 2.

---

## B3 (P1)

Delay

graph package

until

the ledger behavior is proven.

---

## B4 (P1)

Split

business logic

from

transport.

Business first.

Infrastructure second.

---

# Decision Log

* **D-071**: The MVP must implement the smallest pipeline that proves the transactional belief ledger. Infrastructure that does not materially strengthen that proof (live polling, embeddings, graph projection) is deferred.
* **D-072**: Business semantics precede transport semantics. Normalize → Belief Derivation → Kernel Integration is the required implementation order.
* **D-073**: Live external integrations are Phase 2 capabilities unless they are strictly necessary to validate the kernel.

## Technical Lead Note

This review is driven by one constraint: **August 18**.

If you had six weeks, I'd approve this plan almost unchanged.

With less than two weeks, I want every hour spent on code that directly advances the core claim:

> **A transactional belief ledger prevents agents from acting on stale beliefs.**

Everything else—even good engineering—must earn its place by making that claim stronger. Right now, several parts of the plan make the system broader rather than making that central claim more convincing. That's why I'm asking for a narrower first implementation.
