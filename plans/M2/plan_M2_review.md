# Review — Work Package 04 (M2 Implementation Plan)

## Verdict

❌ REWORK REQUIRED

This is the strongest implementation plan we've reviewed so far.

Ironically, that is why I'm asking for one more revision.

I found **zero architectural problems**.

I found **three execution problems**.

All three are fixable without changing the architecture.

---

# Strengths

## S1. This is no longer a test plan.

It is an executable specification.

That is exactly what M2 should become.

Excellent.

---

## S2. M0 and M1 are used correctly.

You never re-prove anything already retired.

Instead, every M2 case closes an open behavioral question.

Excellent milestone discipline. :contentReference[oaicite:0]{index=0}

---

## S3. The behavioral matrix is outstanding.

This is probably the best section.

Every case has

- purpose
- expected behavior
- receipt

Nothing is vague.

Excellent. :contentReference[oaicite:1]{index=1}

---

## S4. The CockroachDB dependency table is excellent.

Especially this:

```
Assumption

↓

discharged by B-xx
```

That's exactly the engineering vocabulary we've been trying to establish.

Excellent. :contentReference[oaicite:2]{index=2}

---

# Blocking Issues

---

## B1 — 24 tests is too large for one milestone

This is my biggest concern.

Originally

M2 existed to

> prove kernel behavior.

It has now grown into

- kernel behavior
- retry machinery
- cycle termination
- cross-scenario semantics
- audit semantics
- DDL drift
- retry injection
- second-run determinism

Technically

all of these belong.

Practically

this is too much risk in one implementation.

I would split

M2

internally.

Not into milestones.

Into **implementation waves.**

Required change:

Document implementation waves.

Example

```
Wave 1

B01–B12

Core kernel

----------------

Wave 2

B13–B20

Behavior

----------------

Wave 3

B21–B24

Edge cases
```

Acceptance stays identical.

The implementation becomes far less risky.

---

## B2 — B18 should not be Stage 14

Retry classification is one of the highest-risk behaviors.

I don't want discovering

```
inject_retry_errors_enabled
```

problems

after thirteen stages.

Move

B18

immediately after

Promote.

Reason:

If retry assumptions are wrong,

we should know on Day 1 of M2,

not Day 4.

---

## B3 — Failure reporting

Current acceptance criteria only define

PASS.

They don't define

FAIL.

I want every failed case to produce

```
Expected

Observed

Receipt

Likely invariant

Decision

STOP
```

Not because CI needs it.

Because

architectural review needs it.

Add one paragraph describing the failure artifact.

---

# Non-blocking Nits

---

## N1

Excellent safety guard.

I would additionally print

the target DSN

(with password removed)

before

dropping anything.

Pure operator safety.

---

## N2

B21

30-second timeout

is safe.

I'd also log

elapsed time

for successful runs.

Future regressions become easier to spot.

---

## N3

For B24

I would explicitly mention

```
rollback verified
```

inside the transcript.

Don't force future reviewers to infer it.

---

# Scope Check

✅ No architectural drift detected.

No schema changes.

No API changes.

No demo work.

No M3.

Exactly what we wanted.

---

# Risk Assessment

Architectural risk:

**0**

Implementation risk:

**Moderate**

Reason:

The milestone is becoming large enough that implementation order now matters almost as much as implementation correctness.

That is why I want execution waves.

---

# Authorization

Implementation may not begin yet.

Revise the plan to include:

- execution waves
- earlier retry validation
- standardized failure artifact

No architectural redesign is required.

I expect this to be the final planning revision.

---

# Decision Log

- D-047: M2 remains a single milestone, but implementation shall be explicitly staged into execution waves.
- D-048: Retry validation is promoted to an early-stage behavioral gate rather than a late-stage edge case.
- D-049: Every failing behavioral test must emit a standardized architectural failure artifact.
