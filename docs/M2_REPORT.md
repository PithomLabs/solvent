# M2 Report — Invariant Test Suite Green

Work Package 04. Companion to `docs/M2_TRANSCRIPT.md` (behavioral receipts) and
`docs/M2_TEST.md` (both `go test` runs). This document is the argument.

---

## 1. Verdict

**M2 is GREEN.** `./scripts/m2_accept.sh` exits 0. **28/28 cases pass**, on two
consecutive runs.

| Deliverable (`plans/M2/plan2_M2_review.md`) | Status |
|---|---|
| `M2_REPORT.md` | ✅ this document |
| `M2_TRANSCRIPT.md` | ✅ 28 rows, all PASS, none skipped |
| `M2_TEST.md` | ✅ both runs captured, both exit 0 |
| `M2_FAILURE.md` | ✅ **absent** — correct on a green run (its machinery was proven separately, §4) |
| Behavioral receipts | ✅ SQLSTATE + constraint name per case, from `*pgconn.PgError` |
| Remaining blockers | **None new.** Two carried items in §6. |

M2 is the first milestone in which the kernel actually ran.

---

## 2. What M2 proves

### The refusals are real, and they surface correctly through Go

| Case | Refusal | SQLSTATE | Constraint |
|---|---|---|---|
| B-09 (I-1) | Promotion with open debt | `23514` | `promoted_is_debt_free` |
| B-10 (I-2) | Promotion of a final-truth claim | `23514` | `promoted_is_debt_free` |
| B-11 (I-3) | Intent on an unpromoted belief | `23503` | `gate` |
| B-12 (I-4) | Retract skipping the cancel | `23514` | `live_requires_promoted` |
| B-24 | Cascade blocked by a foreign-scenario intent | `23514` | `live_requires_promoted` |

B-09, B-10 and B-11 each assert that **`errors.Is` and `errors.As` both succeed on the
same error value**. That is M1's wrap decision validated by execution: the sentinel
contract of §4 holds *and* the raw SQLSTATE and constraint name remain reachable, which
is what M5 will need. Had M1 replaced the driver error with a bare sentinel, these three
cases would have failed on the `errors.As` half.

### The cascade is one transaction — proven, not asserted

B-24 and B-16 are a single execution with two assertions, and together they are the most
consequential result in M2.

A `live` intent in scenario B pointing at a belief in scenario A escapes the
scenario-scoped cancel. The retract then trips `live_requires_promoted` through
`ON UPDATE CASCADE`, and the whole transaction rolls back:

```
B-24  returned 0; sqlstate="23514"; constraint="live_requires_promoted"
B-16  root="promoted" child="promoted"; live intents A=1 B=1; rows changed=0 — rollback verified
```

The in-scenario intent is **still live**. The cancel that had already been issued was
undone with the retract that failed. I-8 is not a claim about the code's shape; it is an
observed property of the database.

This also confirms **M1-R4 exactly as predicted**: the failure mode of D-032's scoping
is a *refusal*, not corruption. The schema still holds the line.

### Everything else

- **B-07** cascade retracts root and descendant, cancels the intent, and `ON UPDATE
  CASCADE` propagates `belief_status='retracted'` into the surviving cancelled row.
- **B-20** a three-link chain returns 3; an unrelated promoted belief is untouched.
- **B-22** two beliefs and three live intents → returns **2**, with 3 intents cancelled.
  `retracted` counts beliefs, never intents (N3).
- **B-08 / B-13** the audit returns 0 per-scenario and 0 globally across every scenario
  the suite created.
- **B-01 … B-06** all eight §4 functions execute to contract, including `RetireDebt`'s
  idempotency on an absent item.

---

## 3. Assumptions discharged

M1 closed with four open Assumptions. **All four are now Verified by execution**, which
was M2's primary job.

| # | Assumption at M1 | Discharged by | Result |
|---|---|---|---|
| D10 | pgx encodes a Go `[]string` into `STRING[]` | **B-17** | Stored debt is element-for-element equal to `kernel.FullDebt`, in order. The pre-decided `'{…}'::TEXT[]` fallback was **not needed**. |
| D11 | `RETURNING id` (UUID) scans into a Go `string` | **B-01** | Returns a well-formed UUID string |
| D12 | Scenario-scoped traversal excludes cross-scenario children | **B-19** | Cascade returns 2; the scenario-B belief across the cross-scenario edge stays `promoted` |
| D9 | `UNION` deduplication terminates on a **cyclic** graph | **B-21** | A two-node cycle terminates and returns 2, in **5 ms** against a 30 s deadline |

D9 is worth dwelling on: D-033's `UNION`-not-`UNION ALL` argument had never been
executed against a cycle. `belief_edge`'s CHECK forbids only self-loops, so a two-node
cycle was the case that would have hung. It terminates.

**M1-R3 is also closed** (B-23): a belief inserted by raw SQL without an explicit debt
list gets exactly `kernel.FullDebt` from the DDL default. Go and DDL have not drifted.

---

## 4. Negative controls

Three mechanisms in this milestone would be worthless if they had never fired. Each was
exercised deliberately rather than assumed:

| Mechanism | Control | Result |
|---|---|---|
| **Database safety guard** (M2-R1) | Ran the suite with `FABLE_TEST_DSN` pointed at `fable` | Refused: *"refusing to reset database "fable": the name must end in `_test`"*. `fable` verified intact afterwards — still 4 tables. |
| **Failure artifact** (D-049) | Added a temporary always-failing case, ran, inspected, removed | `docs/M2_FAILURE.md` produced with the full Expected / Observed / Receipt / Likely invariant / Decision / STOP block; transcript verdict flipped to **BLOCKED**; exit non-zero. Removed afterwards, and a green run deletes any stale artifact. |
| **Retry injection** (B-18) | Built into the case itself as a permanent control | `SHOW inject_retry_errors_enabled` = `on`, and a plain transaction on that pool receives **40001** — *then* `Promote` through `crdb.ExecuteTx` succeeds anyway |

The B-18 control matters most. Without it, a DSN option that silently failed to apply
would have made the case pass while nothing retried — the test would have "proven" retry
handling against an uninjected connection. The control is now part of the case, so that
false positive cannot recur.

---

## 5. Wave summary (N3)

| Wave | Cases | Count | Result |
|---|---|---|---|
| **0** — harness | W0 | 1 | ✅ green |
| **1** — core kernel + retry gate | B-01, B-17, B-23, B-02, B-03, B-04, B-05, B-09, B-10, **B-18**, B-06, B-11, B-12 | 13 | ✅ green |
| **2** — cascade behavior | B-07, B-20, B-22, B-19, **B-24**, **B-16**, B-08, B-13 | 8 | ✅ green |
| **3** — edge cases and closure | B-21, B-14, B-15, W3-Ensure-New, W3-Ensure-Existing, W3-Ensure-DiffScenario | 6 | ✅ green |
| | **total** | **28** | ✅ green ×2 runs |

Wave discipline held (M2-R8): no wave began before the previous one was green. D-048's
promotion of B-18 into Wave 1 paid off in the way it was meant to — the retry mechanism
needed a control added to be trustworthy, and that was discovered on the first day of
implementation rather than after twenty more cases had been built on top of it.

---

## 6. Carried findings (unchanged, non-blocking)

- **R2 — `belief.embedding`.** Contract §7 M2's *"the entire suite passes a second time
  with all embeddings NULL"* is **not literally executable**: there is no embedding
  column. Discharged as ruled — **B-14** asserts the column's absence, so "every
  embedding is NULL" holds by construction, and `m2_accept.sh` runs the whole suite a
  second time, preserving the repeatability the second pass existed to give. **R2 is not
  solved here**; still deferred to M4, and M0's `schema_locked = true` finding still
  applies to whoever adds the column.
- **I-7 is not runtime-executable.** No test proves a negative about code paths. B-15
  records this plainly and the gate re-runs `scripts/check_i7.sh` before the suite —
  7 `ExecuteTx` write sites, 0 raw writes. The transcript row says "NOT
  runtime-executable" rather than claiming coverage M2 does not have.
- **M0 §3.1** — the M5 ruling on displaying `ConstraintName` beside a raw CHECK error is
  still open. M2 reads `ConstraintName` inside tests, which is not a demo surface.

### New information for the architect

**`retry_count` is unavailable, and was deliberately not obtained.** N2 asked for the
metric if CockroachDB exposes it. It does — `crdb_internal.node_transaction_statistics`
has `application_name` and `max_retries` — but on v26.2 `crdb_internal` is **restricted**:

```
ERROR: Access to crdb_internal and system is restricted.
SQLSTATE: 42501
HINT: ... set the session variable allow_unsafe_internals = true (not recommended)
```

Unlocking an interface the database itself calls unsupported, purely to decorate a
transcript, is exactly what N2's *"do not invent instrumentation solely to obtain it"*
rules out. The transcript therefore records
`retry_count = unknown (crdb_internal restricted on v26.2, SQLSTATE 42501; not unlocked)`.
The absence is itself the finding, and B-18's control supplies the evidence the metric
would have: injection was demonstrably live.

---

## 7. Deviations from the plan

**None in scope, cases, or acceptance.** Revision 2 was implemented as written, in wave
order. Two additions were made while building, both strengthening evidence rather than
changing behavior:

1. **B-18 gained an in-case injection control** (`SHOW` + a plain-transaction 40001
   check). The plan specified the injection but not a proof that it applied; without the
   control the case could pass vacuously. This was found by noticing `retry_count`
   returned "unknown" and refusing to accept a green result whose mechanism was
   unverified.
2. **`scripts/check_i7.sh` now excludes `*_test.go`**, as the plan anticipated, so the
   `ExecuteTx` count cannot be perturbed by test files sharing the package directory.

---

## 8. Risks still open at M2's close

| # | Risk | Status |
|---|---|---|
| M2-R1 | The suite drops a database | **Mitigated and proven** — guard fires (§4) |
| M2-R2 | Injection leaking into other cases | **Mitigated** — separate pool, DSN-scoped, closed in `defer`; all other cases use the shared pool |
| M2-R3 | Cycle test hanging | **Retired** — B-21 completes in 5 ms under a 30 s deadline |
| M2-R4 | Audit API is scenario-scoped, invariant is global | **Accepted** — B-13's global form is a raw query; the distinction is explicit in the transcript |
| M2-R5 | Parallel interleaving | **Mitigated** — no `t.Parallel()`; distinct scenario per case; global sweep last |
| M2-R7 | A green suite mistaken for concurrency proof | **Open by design** — see below |
| M2-R8 | Wave discipline eroding | **Retired** — all three waves landed in order, each green before the next |

**M2-R7 restated, because it is the one that matters going forward:** M2 executes
**single-session** behavior only. Nothing here evidences what happens when a cascade and
a new intent race, or whether `READ COMMITTED` permits what `SERIALIZABLE` refuses. That
is **M3**, and a green M2 must not be cited as covering it.

---

## 9. Reproduce

```bash
./scripts/m0_up.sh          # cluster on 26260; fable_test is created by TestMain
./scripts/m2_accept.sh      # isolation → build/vet/gofmt → I-7 → suite ×2
```

The M0 and M1 gates were re-run after M2 landed and both still exit 0 — M2 uses its own
`fable_test` database and does not disturb the evidence those milestones produced.

---

## 10. Stop

M2's acceptance criteria are met. Per D-054 and the review's closing instruction, work
stops here and awaits architectural review. **M3 has not been started.**
