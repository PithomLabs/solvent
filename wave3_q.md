# Wave 3 Open Question — Dedup Ownership (B2)

The Technical Lead's rework identified three P0 blockers. B1 and B3 have clear resolutions (thin translation layer, documented visibility model). B2 has three options requiring a decision before the revised plan can be finalized.

---

## The Problem

The current plan performs dedup reads **outside** kernel transactions:

```
Wiring layer:
    SELECT count(*) FROM belief WHERE claim = $1 AND scenario_id = $2
    ↓ (result)
    kernel.EnterBelief(...)   ← separate transaction
```

This is a TOCTOU boundary. Even with SERIALIZABLE isolation, the abstraction leaks.

---

## Options

### Option A — Modify kernel: add `EnterBeliefIfNew`

Add a new kernel function that does find-or-enter inside one `crdb.ExecuteTx`:

```go
// EnterBeliefIfNew inserts a belief only if no belief with the same claim
// exists in the scenario. Returns the existing or new belief ID and whether
// a new belief was created.
func (s *Store) EnterBeliefIfNew(ctx context.Context, scenarioID, claim string, ct ClaimType) (id string, entered bool, err error)
```

**Pros:**
- Cleanest abstraction — dedup lives where transactions live
- No TOCTOU — SELECT and INSERT in the same `crdb.ExecuteTx`
- Consistent with kernel ownership principle

**Cons:**
- Modifies frozen kernel (M0–M2)
- Adds a new SQL statement
- Requires re-verification of kernel test suite

**Risk:** Low — additive change, existing tests pass, new test covers the new function.

---

### Option B — Keep kernel frozen, accept TOCTOU

Keep dedup reads in the wiring layer, outside kernel transactions. Document as accepted MVP risk.

```
Wiring layer:
    SELECT count(*) FROM belief WHERE claim = $1 AND scenario_id = $2
    ↓ (result)
    kernel.EnterBelief(...)   ← separate transaction
```

**Pros:**
- No kernel modification
- Smallest implementation surface
- Fastest to ship

**Cons:**
- TOCTOU boundary exists
- SERIALIZABLE reduces but does not eliminate the window
- Violates "kernel owns correctness" principle

**Risk:** Low in practice (single-agent MVP, no concurrent belief entry for the same claim). But architecturally impure.

---

### Option C — New kernel function `ProcessEvidence`

Add a composite kernel function that wraps enter+evidence+debt retire in one transaction:

```go
// ProcessEvidence enters a new belief (or finds an existing one), attaches
// evidence, and retires the specified debt items — all in one transaction.
func (s *Store) ProcessEvidence(ctx context.Context, scenarioID, claim string, ct ClaimType, evidence EvidenceSpec, debtItems []string) (id string, entered bool, err error)
```

**Pros:**
- Most correct — entire operation is atomic
- No TOCTOU
- Belief + evidence + debt retirement are inseparable

**Cons:**
- Most complex kernel change
- New SQL (multi-statement transaction with conditional logic)
- Larger test surface
- May be over-engineered for MVP

**Risk:** Medium — more code, more tests, more ways to break.

---

## Decision Criteria

| Criterion | A | B | C |
|---|---|---|---|
| Correctness | High | Medium | Highest |
| Implementation effort | Medium | Low | High |
| Kernel modification | Yes (additive) | No | Yes (additive) |
| TOCTOU eliminated | Yes | No | Yes |
| MVP appropriateness | Yes | Yes | Possibly over-engineered |
| Aligns with "kernel owns correctness" | Yes | No | Yes |

---

## My Recommendation

**Option A** — `EnterBeliefIfNew` in the kernel.

It is the cleanest balance: eliminates TOCTOU, aligns with the architectural principle, and is a small additive change to the kernel. Option B is acceptable but architecturally impure. Option C is correct but likely over-scoped for the hackathon MVP.

---

Awaiting decision before writing the revised `WAVE3_IMPLEMENTATION_PLAN.md`.
