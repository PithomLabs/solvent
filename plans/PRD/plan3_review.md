This is a **substantial improvement** over v2.

The author actually listened to the review instead of merely patching it. The plan is now aligned with the philosophy we've been driving: **prove the kernel first, add infrastructure later.** 

My verdict changes.

> **APPROVED WITH NITS**

---

# Executive Summary

| Category               | Verdict                |
| ---------------------- | ---------------------- |
| Scope Discipline       | ✅ Excellent            |
| MVP Focus              | ✅ Excellent            |
| Architecture Fidelity  | ✅ Excellent            |
| Engineering Sequencing | ✅ Excellent            |
| Deadline Realism       | ✅ Much Better          |
| Overall                | **APPROVED WITH NITS** |

---

# What Improved

## 1. This is now truly a Phase 1 plan

This was my biggest objection before.

Now the scope is exactly what I wanted:

* Normalize
* Derive
* Kernel
* Fixtures
* Tests

Everything else is explicitly deferred. 

This is a major improvement.

---

## 2. Business before infrastructure

Excellent.

The dependency graph now says

```
NormalizedEvidence

↓

BeliefDerivation

↓

Kernel

↓

Fixtures
```

Exactly.

No transport.

No polling.

No APIs.

No orchestration.

The business semantics come first. 

---

## 3. OpenAI removed

Excellent decision.

Embedding is now

Phase 2.

Correct.

It should never have been in this work package. 

---

## 4. Live GitHub removed

Excellent.

Use fixtures.

Deterministic.

Repeatable.

Hackathon-friendly. 

---

## 5. Graph deferred

Exactly what I wanted.

The graph is a projection.

The ledger is the product.

Perfect. 

---

# Remaining Nits

These are genuinely small.

---

## Nit 1 (P1)

I don't like this sentence:

```
Confidence float64
```

inside `NormalizedEvidence`. 

We've spent weeks removing stored confidence as a first-class concept.

Even though the comment says

> derived strength signal (never stored in ledger)

it still risks reintroducing the wrong mental model.

I would replace it with something like

```go
EvidenceStrength
```

or

```go
EvidenceWeight
```

or simply omit it from Phase 1.

The ledger should care about

evidence

not confidence.

---

## Nit 2 (P1)

The package

```
internal/claim
```

still carries the old terminology.

We've discussed this several times.

I still recommend

```
internal/derive
```

or

```
internal/beliefderive
```

because

you're not extracting language.

You're deriving beliefs.

Not a blocker.

Just consistency.

---

## Nit 3 (P2)

The claim rules

currently encode

```
Fixed in v3.5.14
```

as literal strings. 

I would explicitly say

> fixtures simulate these conditions

rather than making the implementation appear version-specific.

This is documentation polish, not architecture.

---

# One thing I would strengthen

This section:

```
Kernel integration
```

I would rename it

```
Kernel Proof
```

Reason:

Everything in Phase 1 exists to prove

that the kernel behaves correctly.

That's a subtle but important framing change.

---

# One thing I absolutely love

The Decision Log. 

This is excellent engineering discipline.

Future reviewers immediately understand

why things are absent.

Keep doing this.

---

# One suggestion for the implementation itself

I would implement

each wave

as a separate commit.

Example

```
W1

NormalizedEvidence

----------------

W2

BeliefDerivation

----------------

W3

Kernel wiring

----------------

W4

Tests
```

Then every commit corresponds to

one engineering proof.

If a reviewer wants to inspect

Belief Derivation,

they don't have to mentally filter

through four unrelated changes.

---

# Technical Lead Approval

```markdown
APPROVED WITH NITS
```

---

# Required Before Coding

Only three tiny changes:

### N1

Rename

```
Confidence
```

to something that cannot be mistaken for epistemic confidence, or remove it from Phase 1 altogether.

---

### N2

Consider renaming

```
internal/claim
```

to

```
internal/derive
```

to better reflect the project's logic-first semantics.

---

### N3

Clarify in the specification that the example version strings (for example, `"Fixed in v3.5.14"`) are illustrative fixture data rather than hard-coded implementation assumptions.

---

## Technical Lead Note

This is the first implementation plan that feels like it was written with the **August 18 constraint** in mind. It no longer tries to build the entire ecosystem up front. Instead, it isolates the smallest slice that proves the project's central claim: **given normalized evidence, deterministic belief derivation, and kernel enforcement, the Transactional Belief Ledger prevents actions based on stale beliefs.**

That is exactly the proof we need before layering on feeds, embeddings, bchat integration, and the eventual demo. Once this phase is complete, every subsequent enhancement becomes an incremental capability rather than a prerequisite for demonstrating the core idea.
