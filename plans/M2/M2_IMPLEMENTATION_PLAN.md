# M2_IMPLEMENTATION_PLAN.md — Invariant Test Suite Green — Revision 2

Work Package 04. Revision 1 was **❌ REWORK REQUIRED** (`plans/M2/plan_M2_review.md`): zero
architectural problems, three execution problems. Revision 2 was **✅ APPROVED**
(`plans/M2/plan2_M2_review.md`, D-050…D-054).

## What Revision 2 changed

| Ref | Finding | Change |
|---|---|---|
| **B1 / D-047** | 24 cases is too much risk in one undifferentiated implementation | §5 restructured into **three execution waves**, each a checkpoint that leaves the suite green. Acceptance criteria unchanged. |
| **B2 / D-048** | Retry validation must not be discovered on day 4 | **B-18 moved into Wave 1, immediately after `Promote`.** |
| **B3 / D-049** | Acceptance defines PASS but not FAIL | §6.1: a **standardized architectural failure artifact** — Expected / Observed / Receipt / Likely invariant / Decision / STOP — emitted to `docs/M2_FAILURE.md` and stderr. Each case declares the invariant it defends. |
| **N1** | Print the target before dropping anything | `TestMain` prints the redacted DSN and database name, then drops. |
| **N2** | Log elapsed time on success | `elapsed_ms` on every transcript row; B-21's called out in the report. |
| **N3** | Don't make reviewers infer B-24's rollback | B-24/B-16's transcript row states **`rollback verified`** literally. |

Case IDs are **stable**. The review's `B01–B12 / B13–B20 / B21–B24` split was explicitly an
example, and B2's instruction to move B-18 forward makes contiguous ID ranges impossible.
Waves are therefore defined by membership.

**Input discrepancy, non-blocking.** The prompt lists `docs/M0_PREPARE.md` and `docs/M0_I7.md`
as inputs. Neither exists: the PREPARE sweep was introduced at M1 by D-035/D-039, and the I-7
report is an M1 criterion. M0's equivalent evidence is `docs/M0_TRANSCRIPT.md`.

---

# 1. Objective

M2 is the first behavioral milestone. M0 proved the *schema* enforces the gate; M1 proved the
*kernel compiles* and its SQL type-checks. Neither executed a kernel function.

M2 proves, by execution against a live CockroachDB v26.2 cluster, that:

1. **Every §4 function does what its contract says** — all seven, end to end.
2. **The schema refuses what it must**, through the kernel: 23514 on promotion with open debt
   or final-truth language, 23503 on an intent citing an unpromoted belief.
3. **The refusals surface correctly through Go** — `errors.Is(err, ErrPromotionBlocked)` *and*
   `errors.As(err, &pgErr)` both succeed on the same error.
4. **The cascade is one transaction** — a cascade that fails partway leaves nothing behind,
   not even the cancels it had already issued.
5. **The cascade is scenario-scoped** (D-032) and **terminates on a cyclic graph** (D-033).
6. **40001 is classified as retryable and is retried** — deterministically, single-session.
7. **`AuditLiveOnNonPromoted` returns 0 in every committed state** (I-5).

M2 asserts nothing about concurrent behavior. That is M3.

---

# 2. Scope

## M2 implements

| Artifact | Purpose |
|---|---|
| `internal/testdb/testdb.go` | Cluster helper: reset `fable_test`, apply the frozen DDL, open pools |
| `internal/kernel/suite_test.go` | `TestMain`, recorder, transcript + failure-artifact emission |
| `internal/kernel/kernel_test.go` | The table-driven invariant suite (package `kernel_test`) |
| `scripts/m2_accept.sh` | Gate: M1 checks → `go test` → **second run** → transcripts |
| `docs/M2_TRANSCRIPT.md` | Generated receipts, one row per case |
| `docs/M2_TEST.md` | Captured `go test -v` output, both runs |
| `docs/M2_FAILURE.md` | **Only when a case fails** — the architectural failure artifact |
| `docs/M2_REPORT.md` | Closing report |

One change to an M1 artifact: **`scripts/check_i7.sh` excludes `_test.go`**, so test files
sharing the package directory cannot perturb the `ExecuteTx` count.

## M2 does NOT implement

- **M3 in any form.** No concurrency harness. B-18 is single-session error injection.
- Demo UI, graph visualization, Lambda, Bedrock, agents, evidence feed.
- **The embedding column (R2).** Not solved. See §8.
- Any change to `db/001_schema.sql`, `internal/kernel/*.go` non-test files, or `internal/m0/**`.
- Performance, load, or scale assertions.

---

# 3. Behavioral test matrix

Each case runs in its own fixed scenario UUID. Each emits one transcript row: **id, wave,
purpose, expected, observed, SQLSTATE, constraint, invariant defended, elapsed_ms**.

## Function coverage (contract §4)

| id | function | purpose | expected result |
|---|---|---|---|
| **B-01** | `EnterBelief` | A claim enters unpromoted, fully in debt | UUID returned; `status='entered'`, `final_truth=false`, `debt`=`FullDebt` |
| **B-02** | `AddEvidence` | Evidence attaches without changing belief state | Evidence row present; belief `status`/`debt` unchanged |
| **B-03** | `RetireDebt` | One useful move retires one debt | Named item gone; other five survive in order |
| **B-04** | `RetireDebt` (idempotent) | Retiring an absent item is not an error | Second call `nil`; `debt` unchanged |
| **B-05** | `Promote` | A debt-free, non-final belief reaches the throne | `nil`; `status='promoted'` |
| **B-06** | `IntentOnPromoted` | Intent on a promoted belief is accepted | `nil`; intent `state='live'`, `belief_status='promoted'` |
| **B-07** | `RetractCascade` | Root and descendants un-promote; intents cancel | Returns 2; both `retracted`; intent `cancelled`/`retracted` |
| **B-08** | `AuditLiveOnNonPromoted` | I-5 as a query | Returns `0` after a completed cascade |

## Invariants I-1 … I-8

| id | invariant | expected result |
|---|---|---|
| **B-09** | I-1 | `errors.Is` **and** `errors.As` both true; `23514` / `promoted_is_debt_free`; belief still `entered` |
| **B-10** | I-2 | Same, with `debt='{}'` and `final_truth=true` |
| **B-11** | I-3 | `errors.Is(ErrActionOnUnpromoted)`; `23503` / `gate`; **no intent row** |
| **B-12** | I-4 | Raw `UPDATE … 'retracted'` fails `23514` / `live_requires_promoted`; belief still `promoted` |
| **B-13** | I-5 | Per-case, and once globally across all scenarios: `0` |
| **B-14** | I-6 | No `(belief, embedding)` column, so "every embedding NULL" holds by construction; suite passes twice |
| **B-15** | I-7 | Not runtime-executable — re-asserted by `scripts/check_i7.sh` in the gate |
| **B-16** | I-8 | A cascade blocked partway leaves **no** partial effect. Transcript states **`rollback verified`** |

## Execution tests required by the prompt

| id | topic | expected result |
|---|---|---|
| **B-17** | `[]string` → `STRING[]` | Stored `debt` equals `kernel.FullDebt` element-for-element, in order |
| **B-18** | Retry classification | With `inject_retry_errors_enabled=true` on a **separate pool's DSN**, `Promote` succeeds; error is **not** `ErrPromotionBlocked` |
| **B-19** | Cross-scenario isolation | Cross-scenario edge `a2→b1`; cascade from `a1` returns **2**; `b1` stays `promoted` |
| **B-20** | Recursive cascade correctness | Chain `c1→c2→c3` plus unrelated `c9`; returns **3**; `c9` stays `promoted` |
| **B-21** | Cycle termination | `d1→d2`, `d2→d1`; terminates, returns 2, under deadline; elapsed logged |
| **B-22** | `RowsAffected` semantics | 2 beliefs, 3 live intents → returns **2**; all 3 `cancelled` |
| **B-23** | Debt/DDL drift | Raw insert without a debt list yields `kernel.FullDebt` |
| **B-24** | M1-R4 | Foreign-scenario live intent blocks a scoped cascade: `23514` / `live_requires_promoted` |

B-24 and B-16 are the same execution: B-24 asserts *how* it fails, B-16 that *nothing survived*.

---

# 4. CockroachDB dependencies

| # | Behavior | Status | Basis |
|---|---|---|---|
| D1 | `inject_retry_errors_enabled` injects 40001; proceeds after the 3rd retry when the client uses `SAVEPOINT cockroach_restart` | **Verified** | Docs, *Transaction Retry Error Example*; `crdb.ExecuteTx` uses that savepoint |
| D2 | Session variables settable per-connection via `options=-c <var>=<val>` | **Verified** | Docs. Used instead of `SET` because `database/sql` pools connections |
| D3 | `ExecuteTx` default policy retries with no backoff via `ROLLBACK TO SAVEPOINT` | **Verified** | `crdb/common.go`; default `LimitBackoffRetryPolicy{50}` |
| D4 | 23514 / `promoted_is_debt_free`; 23503 / `gate` | **Verified** | M0 probes C1, C2, D1 |
| D5 | Cascade-then-CHECK refuses retract-with-live-intent | **Verified** | M0 probe D3 |
| D6 | `*pgconn.PgError` carries `ConstraintName` | **Verified** | M0 transcript |
| D7 | `array_remove(STRING[], STRING)` no-ops when absent | **Verified** | Docs |
| D8 | `WITH RECURSIVE` + `UNION` prefixing an `UPDATE` | **Verified** | M0 D4; M1 PREPARE sweep |
| D9 | `UNION` terminates on a **cyclic** graph | **Assumption → discharged by B-21** | Never executed against a cycle |
| D10 | pgx encodes `[]string` into `text[]` | **Assumption → discharged by B-17** | M1 confirmed the expected type only |
| D11 | `RETURNING id` scans into a Go `string` | **Assumption → discharged by B-01** | Preparable; never executed |
| D12 | Scoped traversal excludes cross-scenario children | **Assumption → discharged by B-19** | Implemented at M1; never executed |
| D13 | `DROP`/`CREATE DATABASE` available to the test role | **Verified** | `cmd/m0verify --reset` |
| D14 | Concurrency, isolation levels | **Out of scope** | M3 |
| D15 | Vector/embedding types | **Out of scope** | M4 |

D1, D2 and D10 are all exercised in **Wave 1**, so a wrong foundation stops M2 on day 1.

---

# 5. Implementation order — three execution waves (D-047)

Waves are checkpoints. **Every wave ends with `go test ./...` green**, and no wave begins
before the previous is green.

## Wave 0 — Harness

| Stage | Work |
|---|---|
| W0.1 | `internal/testdb`: reset, apply DDL, open shared and injection pools |
| W0.2 | `TestMain`: safety guard, recorder, transcript + failure writers |
| W0.3 | Smoke test: the four contracted tables exist |

**Safety guard, non-negotiable.** `TestMain` refuses unless the database name ends in `_test`,
and **prints the redacted DSN and database name before dropping anything** (N1).

## Wave 1 — Core kernel and the retry gate

| Stage | Cases |
|---|---|
| W1.1 | B-01, B-17, B-23 |
| W1.2 | B-02 |
| W1.3 | B-03, B-04 |
| W1.4 | B-05, B-09, B-10 |
| **W1.5** | **B-18 (D-048)** |
| W1.6 | B-06, B-11 |
| W1.7 | B-12 |

**Exit:** 13 cases green. **Checkpoint:** if D1, D2 or D10 proved wrong, STOP and escalate.

## Wave 2 — Cascade behavior

| Stage | Cases |
|---|---|
| W2.1 | B-07, B-20, B-22 |
| W2.2 | B-19 |
| W2.3 | **B-24, B-16** |
| W2.4 | B-08, B-13 |

**Exit:** 21 cases green.

## Wave 3 — Edge cases and closure

| Stage | Cases |
|---|---|
| W3.1 | B-21 |
| W3.2 | B-14 |
| W3.3 | B-15; `m2_accept.sh`; `check_i7.sh` excludes `_test.go` |
| W3.4 | `docs/M2_REPORT.md` |

**Exit:** 24 cases green; `./scripts/m2_accept.sh` exits 0.

---

# 6. Acceptance criteria

| # | Criterion |
|---|---|
| A1 | `go test ./...` exits 0 |
| A2 | `go test ./...` exits 0 on a **second consecutive run** without manual cleanup |
| A3 | All cases appear in `docs/M2_TRANSCRIPT.md` with status PASS; zero skipped |
| A4 | B-09, B-10 record `23514` / `promoted_is_debt_free` |
| A5 | B-11 records `23503` / `gate` |
| A6 | B-12, B-24 record `23514` / `live_requires_promoted` |
| A7 | B-09, B-10, B-11 assert `errors.Is` **and** `errors.As` on the same error |
| A8 | B-17 asserts stored `debt` element-wise equal to `kernel.FullDebt`, length 6 |
| A9 | B-19 returns exactly `2`; scenario-B belief reads `promoted` |
| A10 | B-20 returns exactly `3`; unrelated belief reads `promoted` |
| A11 | B-21 returns exactly `2` within a 30-second deadline |
| A12 | B-22 returns exactly `2` with exactly `3` intents `cancelled` |
| A13 | B-16: intents `live`, beliefs `promoted`, 0 rows changed, transcript reads `rollback verified` |
| A14 | B-18 returns nil, belief `promoted`, error not `ErrPromotionBlocked` |
| A15 | B-13's global sweep returns `0` |
| A16 | B-14 returns `0` rows for `(belief, embedding)` |
| A17 | `./scripts/check_i7.sh` exits 0: 6 `ExecuteTx` sites, 0 raw writes |
| A18 | `./scripts/m2_accept.sh` exits 0 end to end |
| A19 | `go build ./...`, `go vet ./...` exit 0; `gofmt -l cmd internal` empty |
| A20 | `M2_TRANSCRIPT.md`, `M2_TEST.md`, `M2_REPORT.md` all present |
| **A21** | On any failure, `docs/M2_FAILURE.md` exists with a complete §6.1 artifact per failed case, and the run exits non-zero |

## 6.1 Failure artifact (D-049)

Every failing case emits, to `docs/M2_FAILURE.md` and stderr:

```
### FAILURE — B-09  (wave 1)

Expected:          refused with SQLSTATE 23514, constraint promoted_is_debt_free
Observed:          statement committed; belief.status = 'promoted', debt = {needMap, …}
Receipt:           <raw driver error verbatim, or the row-state dump if there was no error>
Likely invariant:  I-1 — no belief with status='promoted' has non-empty debt
Decision:          STOP
Escalation:        Architectural review. Do not redesign, do not weaken the invariant,
                   do not adapt the test to the observed behavior.
```

The artifact exists for architectural review, not CI:

- **`Likely invariant` is not guessed** — every case declares it as a field.
- **`Receipt` is verbatim.** Where the failure is that *nothing* went wrong when something
  should have, it is a row-state dump. **A refusal that did not happen is as much a receipt as
  one that did.**
- **`Decision` is always STOP.** There is no "retry with different expectations" path.

The transcript verdict flips to **BLOCKED**, and `m2_accept.sh` exits non-zero without running
the second pass — a suite that failed once has nothing to prove by repeating.

---

# 7. Risks introduced by M2

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| **M2-R1** | **The suite drops a database.** A DSN pointing at `fable` would destroy M0/M1 evidence. | **High** | Hard guard: abort unless the name ends in `_test`; print redacted DSN first. |
| **M2-R2** | Injection leaking into other cases | Medium | Separate pool's DSN only; closed in `defer`; never `SET` on the shared pool. |
| **M2-R3** | Cycle test hanging | Medium | 30s context deadline; elapsed logged on success. |
| **M2-R4** | Audit API scenario-scoped, invariant global | Low | B-13's global form is a raw query; the distinction stays explicit. |
| **M2-R5** | Parallel interleaving | Medium | No `t.Parallel()`; distinct scenario per case; global sweep last. |
| **M2-R6** | Leftover `fable_test` from an aborted run | Low | `TestMain` drops and recreates unconditionally. |
| **M2-R7** | A green suite mistaken for concurrency proof | Medium | Stated in `M2_REPORT.md`: single-session only; the race is M3. |
| **M2-R8** | Wave discipline eroding under time pressure | Medium | Explicit exit conditions; Wave 1 STOP checkpoint; report records the wave each case landed in. |

---

# 8. Escalation

STOP, do not redesign, report receipts via the §6.1 artifact if:

- A refusal arrives with a SQLSTATE other than M0 recorded (D4, D5).
- `errors.Is` and `errors.As` do not both succeed on the same error.
- `RetractCascade` retracts a belief outside its scenario (D-032 violated).
- A blocked cascade leaves any partial effect (I-8 violated — the most serious possible
  finding).
- B-21 does not terminate (D-033 false).
- `[]string` fails to encode (D10) — the `'{…}'::TEXT[]` fallback is pre-decided but changing
  the kernel is an M1 amendment requiring a ruling.

## Carried, non-blocking

- **R2 — `belief.embedding`.** Contract §7 M2's "second time with all embeddings NULL" is not
  literally executable. B-14 asserts the column's absence; A2 runs the suite twice. **R2 is
  not solved here** — deferred to M4; `schema_locked = true` still applies.
- **I-7 is not runtime-executable.** B-15 records this plainly; the static gate re-runs.
- **M0 §3.1** — the M5 `ConstraintName` ruling is still open; nothing here depends on it.

---

# Verification

```bash
./scripts/m0_up.sh          # cluster on 26260 (fable_test created by TestMain)
./scripts/m2_accept.sh      # M1 checks → go test → second go test → transcripts
```

Confirm by reading: 24 rows all PASS; `rollback verified` on B-16; nil error under injection on
B-18; `docs/M2_FAILURE.md` absent on a green run.

Stop at M2's close. **Do not begin M3.**
