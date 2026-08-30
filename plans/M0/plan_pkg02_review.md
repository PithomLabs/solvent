# Review — Work Package 02 (M0 Implementation Plan)

## Verdict

✅ APPROVED WITH NITS

This is a senior-level implementation plan.

It stays inside the implementation contract, clearly separates verified facts from empirical verification, and most importantly it understands that **M0 exists to retire one architectural risk—not to write software.** The scope discipline is excellent and there is no architectural drift.

---

# Strengths

## S1. Scope discipline is excellent.

The plan never leaks into M1.

It repeatedly states:

- no kernel
- no agents
- no UI
- no redesign

That is exactly the behavior we want.

---

## S2. Correct use of CockroachDB documentation.

I especially like this section:

> "What the docs do not state is whether the child's CHECK is re-evaluated..."

Instead of assuming behavior,

the plan promotes it into an experiment.

That is exactly the correct engineering approach.

---

## S3. Probe design is outstanding.

The probe table is extremely well thought out.

Every probe records:

- SQLSTATE
- constraint
- raw driver message
- expected
- observed

That gives us receipts instead of opinions.

Excellent.

---

## S4. Driver choice is justified.

Choosing pgx because the kernel already classifies SQLSTATE through an interface is exactly the kind of implementation decision that belongs in M0.

It avoids discovering the mismatch in M1.

Excellent.

---

## S5. STOP-and-report policy is consistently enforced.

I found no place where the plan silently works around an architectural discrepancy.

Everything becomes:

- named
- measured
- escalated

Exactly what the implementation contract requires.

---

# Nits (non-blocking)

## N1.

`cmd/m0verify` currently performs several responsibilities:

- schema application
- probing
- transcript generation
- reporting

For M0 this is acceptable.

However, keep very clear package boundaries internally.

Avoid one 1000-line `main.go`.

This is purely maintainability.

---

## N2.

Capture


before and after D-series.

This is probably unnecessary,

but it gives us an extra receipt if CockroachDB rewrites metadata after cascade operations.

Low priority.

---

## N3.

Include elapsed execution time for every probe.

Not for benchmarking.

Simply because,

if one probe unexpectedly becomes slow,

we'll know immediately.

No architectural impact.

---

## N4.

Make transcript generation deterministic.

Sort objects.

Stable timestamps where possible.

Consistent formatting.

This makes future diffs much easier.

---

# Scope Check

✅ No architectural drift detected.

No additional tables.

No additional agents.

No additional feeds.

No redesign.

No speculative implementation.

The implementation remains completely inside M0.

---

# Risk Assessment

## Remaining technical risks

### R1 (Known)

CockroachDB may differ from PostgreSQL on D3/D4.

This is exactly why M0 exists.

No issue.

---

### R2 (Known)

Embedding column disagreement.

Correctly escalated.

Correctly deferred.

Do not fix during M0.

---

### R3 (Known)

Temporary tables.

Correctly deferred.

No action.

---

No new architectural risks were introduced by this plan.

---

# Authorization

Proceed with implementation exactly as planned.

Address the nits only if they are essentially free.

Do not expand scope.

When M0 completes:

STOP.

Do not begin M1.

Return:

- M0_REPORT.md
- M0_TRANSCRIPT.md
- SQLSTATE receipts
- CockroachDB version
- READ COMMITTED verification
- Blocker report (if any)

Await architectural review before any further implementation.

---

# Decision Log

- D-017: M0 implementation plan approved.
- D-018: CockroachDB documentation is authoritative where documented; undocumented behavior must be verified empirically.
- D-019: pgx/v5 remains the sole approved SQL driver for the MVP.
- D-020: Any divergence between CockroachDB and the PostgreSQL proof is a reportable architectural event, not an implementation bug.
- D-021: M0 ends immediately after all acceptance criteria are satisfied; M1 may not begin without explicit approval.


