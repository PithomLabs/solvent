# Review — Work Package 03 (M1 Implementation Plan, Revision 2)

## Verdict

✅ APPROVED WITH NITS

This is a substantial improvement over Revision 1.

All three blocking issues have been addressed:

- ✅ D-032 incorporated
- ✅ D-033 incorporated
- ✅ B1 resolved
- ✅ B2 resolved
- ✅ B3 resolved

I authorize implementation.

---

# Review of Previous Blockers

## B1 — `RetractCascade`

**Status:** ✅ Resolved

This is the correct decision.

Implementing the real function is significantly better than carrying a deliberate stub.

The recursive CTE, scenario scoping, cancel-before-retract ordering, and explicit termination argument (`UNION` rather than `UNION ALL`) are all clearly specified. :contentReference[oaicite:0]{index=0}

Approved.

---

## B2 — SQL validation

**Status:** ✅ Resolved

Excellent improvement.

You didn't merely accept the suggestion.

You researched CockroachDB's PREPARE behavior and documented precisely what it validates and what it cannot validate.

That distinction is exactly the kind of engineering honesty we want.

I especially like

> PREPARE is a SQL compile check, not an invariant check.

Keep that sentence.

Excellent. :contentReference[oaicite:1]{index=1}

---

## B3 — `Contract`

**Status:** ✅ Resolved

Much better.

```
kernel.Contract
```

reads naturally.

Approved.

---

# Additional Strengths

## S1 — SQLCatalog

This is the strongest addition in the revision.

At first I was skeptical because it technically expands the API.

However,

you explicitly declared it,

justified it,

and eliminated duplicated SQL.

That is the right tradeoff.

The verifier preparing the exact statements the kernel uses is substantially stronger than maintaining a second copy. :contentReference[oaicite:2]{index=2}

Approved.

---

## S2 — PREPARE sweep

Excellent.

The verifier now proves

- relations
- columns
- placeholders
- statement structure

without pretending to prove semantics.

Exactly right.

---

## S3 — Acceptance criteria

These are now significantly stronger.

Particularly

```
No CREATE TEMP TABLE
```

and

```
UNION
```

as explicit acceptance gates.

Excellent. :contentReference[oaicite:3]{index=3}

---

## S4 — Risk section

M1-R4 is excellent.

It acknowledges that

the schema may refuse

rather than corrupt.

That's a very important distinction.

---

# Nits (Non-blocking)

---

## N1 — SQLCatalog export

I understand why it exists.

However,

I would explicitly mark it

```go
// Verification support.
// Not part of the public kernel API.
```

in the source.

Otherwise future developers may start depending on it.

This is documentation only.

---

## N2 — PREPARE statement names

Current

```
m1_<name>
```

I recommend

```
verify_<name>
```

instead.

Reason:

Nothing about the verifier is inherently tied to M1.

Tiny naming improvement.

---

## N3 — `RowsAffected`

For `RetractCascade`

I recommend documenting one sentence.

Specifically:

```
RowsAffected refers only to
belief rows retracted,
not cancelled intents.
```

Future reviewers won't have to infer it.

---

# Scope Check

✅ No architectural drift detected.

Still:

- three tables
- two agents
- one feed
- one graph

No scope expansion.

The only API addition (`SQLCatalog`) is justified as verification infrastructure and does not alter the contract's behavioral surface. :contentReference[oaicite:4]{index=4}

---

# Risk Assessment

Remaining meaningful risks after M1 planning:

| Risk | Status |
|-------|--------|
| `[]string` → `STRING[]` encoding | Deferred correctly to M2 |
| Runtime semantics | Deferred correctly to M2 |
| Isolation | Deferred correctly to M3 |
| Embedding | Deferred correctly to M4 |

No new architectural risks introduced.

---

# Authorization

Proceed with implementation.

Implement Revision 2 exactly as written.

Do not optimize.

Do not redesign.

Do not begin M2.

When implementation completes, return:

- M1_REPORT.md
- PREPARE transcript
- Build transcript
- Vet transcript
- I-7 report
- Remaining blockers (if any)

Then stop.

---

# Decision Log

- D-037: Revision 2 approved.
- D-038: `SQLCatalog()` is accepted as verification infrastructure, not kernel behavior.
- D-039: PREPARE validation becomes a permanent acceptance gate for kernel milestones.
- D-040: `RetractCascade` implementation is now part of M1 rather than deferred.
- D-041: Work Package 03 authorized for implementation.
