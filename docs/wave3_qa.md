My answer is **not one of the three options as written**.

I choose **Option A**, **with one important architectural refinement**.

---

# Decision

> **Approve Option A (modified).** 

Do **not** implement Option B.

Do **not** implement Option C.

---

# Why not B?

Because we've spent the entire project establishing one principle:

> **The kernel owns correctness.**

If the wiring layer performs

```text
SELECT

↓

decision

↓

kernel
```

then the wiring layer now owns correctness.

That violates our architecture.

---

# Why not C?

Because it moves in the opposite direction.

Instead of a thin wiring layer,

the kernel becomes an application service.

That is too much for the MVP. 

---

# The refinement

I would **not** call it

```go
EnterBeliefIfNew(...)
```

because the API leaks implementation.

Instead,

design the kernel around **intent**, not optimization.

Something closer to

```go
EnsureBelief(...)
```

or

```go
EnsureBeliefExists(...)
```

Semantics:

```text
Given a scenario + claim

↓

Return the belief ID

↓

Create it if necessary

↓

Otherwise return the existing one

↓

One transaction
```

Notice

the caller doesn't care whether it was inserted.

The caller needs

the belief.

Not whether an INSERT happened.

That makes the kernel API more stable.

---

# One more refinement

I also wouldn't return

```go
entered bool
```

unless a real caller needs it.

Ask:

> Does Wave 3 ever behave differently depending on whether the belief already existed?

If the answer is **no**,

don't expose it.

Good APIs expose only what callers need.

---

# The new kernel boundary

Instead of

```text
Wiring

↓

SELECT

↓

kernel.EnterBelief()
```

I want

```text
Wiring

↓

kernel.EnsureBelief()

↓

beliefID
```

Then

```text
AttachEvidence()

↓

RetireDebt()

↓

Promote()
```

That preserves our layering:

```text
Wave 1

Normalize

↓

Wave 2

Derive

↓

Wave 3

Translate

↓

Kernel

↓

CockroachDB
```

---

# Cost

This is a **small additive change**.

It slightly violates "kernel frozen,"

but it **improves** the architecture instead of expanding it.

I consider this a justified exception.

---

# Reply to the coding agent

```markdown
Technical Lead Decision

Approve **Option A**, with one refinement.

Do not keep deduplication in the wiring layer (Option B), and do not introduce a composite `ProcessEvidence` kernel API (Option C).

Instead, extend the kernel with a single additive function whose responsibility is:

> Ensure a belief exists for `(scenario, claim)` inside one transaction and return its identifier.

Design the API around **intent** rather than implementation details. Prefer a name such as `EnsureBelief` or `EnsureBeliefExists` over `EnterBeliefIfNew`.

Only return whether a belief was newly created if a real caller requires that information. Otherwise return only the belief identifier.

This is the only permitted modification to the frozen kernel because it strengthens the architectural boundary: uniqueness and transactional correctness remain owned by the kernel rather than leaking into the wiring layer.

Revise the Wave 3 implementation plan accordingly, then resubmit it for review.
```

I think this gives us the best balance of **correctness**, **clean architecture**, and **MVP scope**.
