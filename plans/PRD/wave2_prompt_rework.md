## Technical Lead Review

**Verdict:** **REWORK REQUIRED** 

This is a good plan, but it crosses an important architectural boundary that we've been very disciplined about preserving.

The issue isn't implementation quality—it's **responsibility leakage**.

---

# P0 Blockers

## B1 — `derive` knows too much about the kernel

This is the biggest issue.

Right now `DeriveAction` contains kernel concepts:

* `ActionEnter`
* `ActionRetireDebt`
* `ActionRetract`
* `BeliefID`



That couples Wave 2 to Wave 3.

`derive` should answer only:

> **"Given evidence, what beliefs logically follow?"**

It should **not** prescribe kernel operations.

**Required change**

Replace command-oriented actions with domain-oriented derivation results.

The kernel decides **how** to apply them.

---

## B2 — Debt retirement belongs in Wave 3

Wave 2 should not emit

```
ActionRetireDebt
```

or know about individual debt items. 

Debt is ledger state.

Wave 2 derives beliefs.

Wave 3 mutates beliefs.

Move debt retirement logic into the belief wiring layer.

---

## B3 — ClaimID is premature

`ClaimID` exists solely because Wave 3 needs wiring. 

Wave 2 shouldn't invent identifiers for later layers.

Remove it.

Wave 3 can correlate derivations when persistence exists.

---

# P1 Changes

## C1 — Five rules are too etcd-specific

The mechanism is generic.

The examples are etcd.

Separate those.

Example:

```
Rule

↓

Evidence matches vulnerability advisory

Example

↓

KEV
```

instead of

```
Rule

↓

KEV
```

This keeps the kernel demonstrably domain-independent.

---

## C2 — Claim text should not be hardcoded

Example:

```
"etcd {version} is vulnerable..."
```



Instead derive structured facts.

Formatting belongs to presentation or adapters.

---

## C3 — Rule 5

Excellent decision.

Keep promotion entirely in Wave 3.

No changes needed.

---

# P2

Rename

```
internal/derive
```

to

```
internal/beliefderive
```

or keep `derive` if brevity is preferred.

This is optional.

---

# What I want instead

Wave 2 should produce something closer to:

```go
type DerivedBelief struct {
    Claim
    Classification
    SupportingEvidence
    Contradicts
}
```

Nothing about:

* promotion
* debt
* IDs
* kernel actions
* transactions

That keeps Wave 2 purely semantic.

---

# Revised Scope

Wave 2 owns:

* deterministic derivation
* classification
* contradiction detection
* provenance
* supporting evidence

Wave 3 owns:

* enter
* update
* retire debt
* promote
* retract
* transactions

That boundary is much cleaner.

---

## Decision

**REWORK REQUIRED**

Reason:

The current plan allows semantic derivation to leak into ledger mutation.

Maintain the separation we've established:

> **Wave 2 derives facts. Wave 3 decides what to do with them.**
