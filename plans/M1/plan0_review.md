# Review — Work Package 03 (M1 Implementation Plan)

## Verdict

❌ REWORK REQUIRED

This is an excellent plan, but I cannot approve it because it contains **one architectural blocker** and **two implementation blockers**.

The good news is that the architecture itself is not in trouble. The blockers are narrow and, once addressed, I expect to approve the plan.

---

# Strengths

## S1. Scope discipline remains excellent.

You stayed entirely inside M1.

No M2.

No database execution.

No demo work.

No UI.

No concurrency.

Excellent. :contentReference[oaicite:0]{index=0}

---

## S2. Excellent use of M0 receipts.

The plan now treats M0 as authoritative instead of repeating verification.

That's exactly how milestones should compose.

---

## S3. The implementation order is very strong.

S1 → S13 is incremental.

Every step leaves the tree buildable.

Exactly what we wanted. :contentReference[oaicite:1]{index=1}

---

## S4. Escalation quality is excellent.

This is probably the strongest section.

You correctly identified an ambiguity instead of silently choosing one.

Good engineering. :contentReference[oaicite:2]{index=2}

---

# Blocking Issues

---

## B1 — I reject stubbing `RetractCascade`

This is the only architectural blocker.

Current proposal:

```
RetractCascade

↓

returns ErrCascadeUnresolved
```

I disagree.

Not because I want the implementation now.

Because I don't want an intentionally incomplete kernel.

The implementation contract says M1 establishes the kernel API.

It does **not** require M1 to defer one function because a later milestone exists.

Instead, I want us to resolve the ambiguity **before** M1 implementation.

The architecture is frozen.

The ambiguity is now ours to answer.

Therefore:

**Required change**

Remove the stub strategy from the implementation plan.

Replace it with an explicit architectural ruling request.

After the ruling,

M1 implements the real function.

Do not carry intentional incompleteness into the codebase.

---

## B2 — "No database contact" is too strict

Current wording:

> M1 runs no statement.

I think this is an overreaction.

You already depend on SQL strings.

I don't require execution.

But I do require

```
Prepare()

or equivalent
```

against a disposable CockroachDB connection

if it can detect malformed SQL before M2.

If CockroachDB cannot validate without execution,

state that explicitly.

Otherwise,

we're deliberately postponing cheap failures.

**Required change**

Clarify whether statement preparation is technically possible and worthwhile.

If not,

document why.

Don't simply prohibit all database interaction.

---

## B3 — `contract.go`

I love the compile-time interface assertion.

I do **not** like

```
contractV1
```

as the interface name.

This interface is

not versioned.

It is

the contract.

Use something like

```
KernelContract
```

or

```
Contract
```

The version belongs in Git history,

not in the type name.

---

# Architectural Ruling (E-1)

This is the decision you requested.

---

## D-032

`RetractCascade`

**is scenario-scoped.**

Reason:

Everything in this product is explicitly scenario-scoped.

A cross-scenario retraction would violate the product model.

The presence of `scenarioID` in the signature is therefore semantically meaningful.

It is **not** symmetry.

Implementation shall scope both recursive traversal and updates to the supplied scenario.

---

## D-033

The temp-table implementation is retired.

Approved replacement:

```
WITH RECURSIVE
```

exactly as demonstrated by M0 D4.

Reason:

This is no longer speculative.

It is empirically verified on the target CockroachDB version.

Therefore,

the architecture now adopts it.

The old temp-table draft is superseded.

---

# Scope Check

Architecture remains frozen.

No drift detected.

No additional tables.

No additional agents.

No additional feeds.

Excellent.

---

# Authorization

Implementation may **not** begin yet.

Please submit a revised M1 implementation plan incorporating:

- D-032
- D-033
- B1
- B2
- B3

I expect this to be the final revision.

---

# Decision Log

- D-032: `RetractCascade` is scenario-scoped.
- D-033: `WITH RECURSIVE` replaces the temporary-table approach as the canonical implementation.
- D-034: M1 shall implement `RetractCascade`; intentional stubbing is rejected.
- D-035: M1 must clarify the decision regarding SQL preparation versus zero database contact.
- D-036: Contract interface naming should reflect its role rather than an arbitrary version suffix.
