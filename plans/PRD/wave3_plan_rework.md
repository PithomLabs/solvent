## Technical Lead Review

**Verdict: REWORK REQUIRED**

This is a **very strong plan**, but I think it makes one architectural mistake that we've been carefully avoiding since Wave 1.

The mistake is subtle:

> **Wave 3 is becoming an application service instead of a thin kernel wiring layer.** 

If we fix that, I think the implementation will become both smaller and cleaner.

---

# Overall

| Area                  | Verdict                   |
| --------------------- | ------------------------- |
| Scope discipline      | ✅ Excellent               |
| Roadmap alignment     | ✅ Excellent               |
| CockroachDB awareness | ✅ Excellent               |
| Test strategy         | ✅ Excellent               |
| Kernel separation     | ⚠️ Too much orchestration |
| Overall               | **REWORK REQUIRED**       |

---

# P0 Blockers

## B1 — `Process()` is doing too much

This is my biggest concern.

Right now `belief.Process()` owns:

* dedup
* enter
* attach evidence
* retire debt
* promote
* contradiction handling

That's no longer "wiring."

That's orchestration. 

Remember our layering:

```
Wave 1
Normalize

↓

Wave 2
Derive

↓

Wave 3
Wire

↓

Kernel
```

Wave 3 should translate semantic objects into kernel operations.

It should not become another business layer.

### Required change

Split responsibilities conceptually:

```
Process()

↓

Translate

↓

Kernel API
```

Keep the orchestration extremely thin.

---

## B2 — Dedup belongs in the kernel

The plan performs reads outside kernel transactions:

* claim dedup
* evidence dedup

before mutation.  

That creates a classic TOCTOU boundary.

Even with SERIALIZABLE, the abstraction leaks.

I'd much rather see:

```
Kernel

↓

Find-or-enter

↓

Attach-if-new
```

rather than

```
Wiring

↓

SELECT

↓

Kernel mutation
```

If the kernel already owns transactions, it should own uniqueness semantics as well.

---

## B3 — Sequential transactions worry me

This section is the only place where I became uncomfortable. 

The plan says:

```
Enter

↓

AddEvidence

↓

RetireDebt

↓

Promote
```

each in independent transactions.

I understand why (retry ownership).

But I want us to verify one thing before implementation:

> Can an observer see an intermediate state that violates our intended lifecycle?

For example

```
belief entered

↓

(no evidence yet)

↓

crash
```

Is that acceptable?

Maybe.

Maybe not.

I want this explicitly justified.

---

# P1

## C1 — Exact claim dedup

Acceptable for MVP.

But I want one sentence explaining

why

exact string equality

is sufficient.

The current explanation is only in Risks. 

---

## C2 — Evidence→Debt mapping

Excellent.

But move it into a compile-time table,

not scattered logic.

The plan implies this already, but I'd make it explicit. 

---

# Excellent

## Kernel responsibilities

This section is excellent.

Exactly the right abstraction. 

---

## Database invariants

Excellent.

The project finally feels like

a CockroachDB project,

not a Go project. 

---

## Integration tests

These are exactly the demo scenarios.

Keep them.

Especially

```
Vulnerability

↓

Promote

↓

Intent

↓

Contradiction

↓

Cascade

↓

Audit = 0
```

That's your hackathon demo in executable form. 

---

# One architectural suggestion

I would add one design principle to AGENTS.md after Wave 3.

> **The kernel owns correctness. Every higher layer exists only to translate domain semantics into kernel operations.**

That single sentence will prevent a lot of future architectural drift.

---

# Decision

```markdown
REWORK REQUIRED
```

Required changes before implementation:

### P0

1. Reduce `belief.Process()` into a thin translation layer.
2. Reconsider whether dedup belongs in the kernel rather than the wiring layer.
3. Explicitly justify sequential kernel transactions and document the visibility model of intermediate states.

### P1

4. Make the evidence→debt mapping an explicit compile-time table.
5. Add a short rationale for exact-claim dedup as the MVP trade-off.

---

## Technical Lead Note

One broader observation.

We're now entering the part of the project where **architecture matters more than code**.

Wave 1 and Wave 2 were mostly pure functions.

Wave 3 introduces **state**.

Every line that manipulates state should trigger the question:

> **"Could this responsibility move one layer lower?"**

If the answer is yes, it probably belongs in the kernel.

I would rather spend another hour refining this boundary than spend two days later untangling orchestration from correctness. This is the phase where the Transactional Belief Ledger either stays elegant or gradually turns into an application service with a database underneath—and we want the former.
