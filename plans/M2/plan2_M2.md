# M2_IMPLEMENTATION_PLAN.md — Invariant Test Suite Green — Revision 2

Work Package 04. Revision 1 was **❌ REWORK REQUIRED** (`plans/M2/plan_M2_review.md`): zero
architectural problems, three execution problems. To be written into the repo as
`plans/M2/M2_IMPLEMENTATION_PLAN.md`.

## What Revision 2 changes

| Ref | Finding | Change |
|---|---|---|
| **B1 / D-047** | 24 cases is too much risk in one undifferentiated implementation | §5 is restructured into **three execution waves**, each a checkpoint that leaves the suite green. Acceptance criteria are unchanged, exactly as instructed. |
| **B2 / D-048** | Retry validation must not be discovered on day 4 | **B-18 moves into Wave 1, immediately after `Promote`.** If the retry assumptions are wrong, M2 stops on day 1. |
| **B3 / D-049** | Acceptance defines PASS but not FAIL | New §6.1: a **standardized architectural failure artifact** — Expected / Observed / Receipt / Likely invariant / Decision / STOP — emitted to `docs/M2_FAILURE.md` and to stderr. Each case now declares the invariant it defends so the artifact names it without guesswork. |
| **N1** | Print the target before dropping anything | `TestMain` prints the redacted DSN and database name, then drops. Operator safety. |
| **N2** | Log elapsed time on success, not just on timeout | `elapsed_ms` is a column on every transcript row; B-21's is called out explicitly in the report. |
| **N3** | Don't make reviewers infer B-24's rollback | B-24's transcript row states **`rollback verified`** literally. |

No architectural change. No schema change. No API change. Case IDs are **stable** — the
review's `B01–B12 / B13–B20 / B21–B24` split was explicitly an example, and B2's own
instruction to move B-18 forward makes contiguous ID ranges impossible. Waves are therefore
defined by membership, listed in full below.

**Input discrepancy, non-blocking.** The prompt lists `docs/M0_PREPARE.md` and `docs/M0_I7.md`
as inputs. Neither exists, and neither ever did: the PREPARE sweep was introduced at M1 by
D-035/D-039, and the I-7 report is an M1 criterion. M0's equivalent evidence is
`docs/M0_TRANSCRIPT.md`, which was read in its place.

---

# 1. Objective

M2 is the first behavioral milestone. M0 proved the *schema* enforces the gate; M1 proved the
*kernel compiles* and its SQL type-checks. Neither executed a kernel function.

M2 proves, by execution against a live CockroachDB v26.2 cluster, that:

1. **Every §4 function does what its contract says** — all seven, exercised end to end.
2. **The schema refuses what it must**, through the kernel: promotion with open debt or
   final-truth language (23514), and an action intent on an unpromoted belief (23503).
3. **The refusals surface correctly through Go** — `errors.Is(err, ErrPromotionBlocked)` *and*
   `errors.As(err, &pgErr)` both succeed on the same error, proving M1's wrap decision works
   in practice and that M5 can still reach the raw SQLSTATE.
4. **The cascade is one transaction** — a cascade that fails partway leaves *nothing* behind,
   not even the cancels it had already issued.
5. **The cascade is scenario-scoped** (D-032) and **terminates on a cyclic graph** (D-033's
   `UNION` argument, executed rather than argued).
6. **40001 is classified as retryable and is retried** — proven deterministically in a single
   session, with no concurrency harness.
7. **`AuditLiveOnNonPromoted` returns 0 in every committed state** (I-5).

M2 asserts nothing about concurrent behavior. That is M3.

---

# 2. Scope

## M2 implements

| Artifact | Purpose |
|---|---|
| `internal/testdb/testdb.go` | Cluster helper: reset `fable_test`, apply the frozen DDL, open pools |
| `internal/kernel/suite_test.go` | `TestMain`, the receipt recorder, transcript + failure-artifact emission |
| `internal/kernel/kernel_test.go` | The table-driven invariant suite (package `kernel_test`) |
| `scripts/m2_accept.sh` | Gate: M1 gate → `go test` → **second `go test` run** → transcripts |
| `docs/M2_TRANSCRIPT.md` | Generated receipts, one row per case |
| `docs/M2_TEST.md` | Captured `go test -v` output, both runs |
| `docs/M2_FAILURE.md` | **Written only when a case fails** — the architectural failure artifact (§6.1) |
| `docs/M2_REPORT.md` | Closing report |

One small change to an M1 artifact: **`scripts/check_i7.sh` must exclude `_test.go`**. Its
grep counts `crdb.ExecuteTx` occurrences in `internal/kernel` and expects exactly 6; test files
in that directory would make the count fragile. The gate targets production write paths, so
excluding tests is a correction, not a weakening.

## M2 does NOT implement

- **M3 in any form.** No concurrency harness, no goroutines racing transactions, no
  READ COMMITTED vs SERIALIZABLE cells. The retry test (B-18) is single-session error
  injection, which is not a concurrency harness.
- Demo UI, graph visualization, Lambda, Bedrock, agents, evidence feed.
- **The embedding column (R2).** Not solved. See §8.
- Any change to `db/001_schema.sql` (frozen), `internal/kernel/*.go` non-test files (M1,
  approved), or `internal/m0/**` (frozen at M0 close).
- Performance, load, or scale assertions.

---

# 3. Behavioral test matrix

Every case runs against `fable_test`, in its own fixed scenario UUID so cases cannot interfere.
Every case emits one transcript row: **id, wave, purpose, expected, observed, SQLSTATE,
constraint, invariant defended, elapsed_ms**. The *invariant defended* field is what lets the
failure artifact (§6.1) name the likely invariant automatically.

## Function coverage (contract §4)

| id | function | purpose | expected result | evidence |
|---|---|---|---|---|
| **B-01** | `EnterBelief` | A claim enters unpromoted, fully in debt | Returns a parseable UUID; row has `status='entered'`, `final_truth=false`, `debt` = `FullDebt` | Row read-back; the returned id |
| **B-02** | `AddEvidence` | Evidence attaches without changing belief state | Evidence row exists with the given `provenance_class` and `content_sha256`; belief `status`/`debt` unchanged | Before/after belief snapshot |
| **B-03** | `RetireDebt` | One useful move retires one debt | `debt` loses exactly the named item; the other five survive in order | `debt` array before/after |
| **B-04** | `RetireDebt` (idempotent) | Retiring an absent item is not an error | Second call returns `nil`; `debt` unchanged | `debt` equality across calls |
| **B-05** | `Promote` | A debt-free, non-final belief reaches the throne | Returns `nil`; `status='promoted'` | Row read-back |
| **B-06** | `IntentOnPromoted` | Intent on a promoted belief is accepted | Returns `nil`; intent row `state='live'`, `belief_status='promoted'` | Intent row read-back |
| **B-07** | `RetractCascade` | Root and descendants un-promote; intents cancel | Returns 2; both beliefs `retracted`; the live intent `cancelled` with `belief_status='retracted'` | Row read-back + return value |
| **B-08** | `AuditLiveOnNonPromoted` | I-5 as a query | Returns `0` after a completed cascade | Count |

## Invariants I-1 … I-8 (contract §5)

| id | invariant | purpose | expected result | evidence |
|---|---|---|---|---|
| **B-09** | **I-1** | Promotion with open debt is unrepresentable | `errors.Is(err, ErrPromotionBlocked)` **and** `errors.As(err, &pgErr)` both true; `pgErr.Code == "23514"`; `ConstraintName == "promoted_is_debt_free"`; belief still `entered` | SQLSTATE + constraint + raw message |
| **B-10** | **I-2** | Final-truth language blocks promotion | Same as B-09, with `debt='{}'` and `final_truth=true` | SQLSTATE + constraint |
| **B-11** | **I-3** | A live intent cannot cite an unpromoted belief | `errors.Is(err, ErrActionOnUnpromoted)`; `pgErr.Code == "23503"`; `ConstraintName == "gate"`; **no intent row exists** | SQLSTATE + constraint + row count 0 |
| **B-12** | **I-4** | Retract-with-live-intent, skipping cancel, is refused | Raw `UPDATE belief SET status='retracted'` (not reachable through the kernel, which always cancels first) fails **23514** on `live_requires_promoted`; belief still `promoted` | SQLSTATE + constraint |
| **B-13** | **I-5** | Audit is 0 in every committed state | After *every* mutating case, and once globally across all scenarios, the count is `0` | Per-case count + final global sweep |
| **B-14** | **I-6** | Vectors are never part of belief semantics | `information_schema.columns` has no `(belief, embedding)` row, so "every embedding is NULL" holds by construction; **and the whole suite passes on a second run** | Column-absence query + two green runs |
| **B-15** | **I-7** | Every kernel write goes through `ExecuteTx` | Not executable at runtime — re-asserted by running `scripts/check_i7.sh` inside the M2 gate | I-7 report, regenerated |
| **B-16** | **I-8** | The cascade is ONE transaction | A cascade blocked partway (B-24) leaves **no** partial effect: same-scenario intents still `live`, beliefs still `promoted`. Transcript states **`rollback verified`** (N3). | Full row state after the failed call |

## Execution tests required by the prompt

| id | topic | purpose | expected result | evidence |
|---|---|---|---|---|
| **B-17** | `[]string` → `STRING[]` | Discharge M1-R2/D10 — the last open encoding assumption | `EnterBelief` succeeds and the stored `debt` equals `kernel.FullDebt` element-for-element, in order | `debt` read back as `[]string` |
| **B-18** | Retry classification | 40001 is retryable, is retried, and is never masked by the sentinel wrap | With `inject_retry_errors_enabled=true` on a **separate pool's DSN**, `Promote` on a debt-free belief **succeeds** (crdb retried past the injected errors) and the error is **not** `ErrPromotionBlocked` | Success under injection; belief `promoted` |
| **B-19** | Cross-scenario isolation | D-032 — a scoped cascade must not reach outside its scenario | Scenario A: `a1→a2`, both promoted. A cross-scenario edge `a2→b1` into scenario B. Cascade from `a1` retracts **2**; `b1` stays `promoted` | Return value + `b1` status |
| **B-20** | Recursive cascade correctness | The traversal collects the whole chain and nothing else | Chain `c1→c2→c3` plus an unrelated promoted `c9`. Cascade from `c1` returns **3**; `c9` stays `promoted` | Return value + statuses |
| **B-21** | Cycle termination | D-033's `UNION` argument, executed | Edges `d1→d2` and `d2→d1`. Cascade from `d1` **terminates** and returns 2, under a context deadline. **Elapsed time logged on success** (N2) | Return value + elapsed_ms |
| **B-22** | `RowsAffected` semantics | N3 (M1) — the count is beliefs, never intents | Scenario with **2** beliefs and **3** live intents. `RetractCascade` returns **2** (not 3, not 5); all 3 intents `cancelled` | Return value + intent count |
| **B-23** | Debt/DDL drift | M1-R3 — `FullDebt` (Go) vs the `ARRAY[...]` DEFAULT (DDL) | A belief inserted by raw SQL *without* an explicit debt list has `debt` equal to `kernel.FullDebt` | `debt` comparison |
| **B-24** | M1-R4, the D-032 consequence | A foreign-scenario live intent blocks a scoped cascade — refusal, not corruption | An intent with `scenario_id=B` but `belief_id` in scenario A escapes the scoped cancel; `RetractCascade` on A fails with **23514** on `live_requires_promoted` | SQLSTATE + full row state |

B-24 and B-16 are the same execution: one call, two assertions. B-24 asserts *how* it fails,
B-16 asserts that *nothing survived* the failure.

---

# 4. CockroachDB dependencies

| # | Behavior M2 depends on | Status | Basis |
|---|---|---|---|
| D1 | `inject_retry_errors_enabled=true` makes every statement in an explicit transaction return 40001, and the transaction **proceeds normally after the 3rd retry** when the client uses `SAVEPOINT cockroach_restart` | **Verified** | Docs, *Transaction Retry Error Example → Test transaction retry logic* (v26.2). `crdb.ExecuteTx` uses exactly that savepoint (M1 C1). |
| D2 | A session variable can be set per-connection via the DSN, `options=-c <var>=<val>` | **Verified** | Docs, connection parameters. **This is why B-18 sets it in the DSN rather than with `SET`:** `database/sql` pools connections, so a `SET` on one connection is not guaranteed to be the connection `ExecuteTx` later uses. A DSN option applies to every connection the pool opens. |
| D3 | `crdb.ExecuteTx`'s default policy retries with no backoff, taking the `ROLLBACK TO SAVEPOINT cockroach_restart` path | **Verified** | `crdb/common.go` `ExecuteInTx`; default `LimitBackoffRetryPolicy{RetryLimit: 50}`, zero delay — the path D1 describes |
| D4 | 23514 / `promoted_is_debt_free` on promote; 23503 / `gate` on intent | **Verified** | M0 probes C1, C2, D1 |
| D5 | Cascade-then-CHECK refuses retract-with-live-intent (23514 / `live_requires_promoted`) | **Verified** | M0 probe D3 |
| D6 | `*pgconn.PgError` carries a populated `ConstraintName` | **Verified** | M0 transcript |
| D7 | `array_remove(STRING[], STRING)` removes all matches, no-ops when absent | **Verified** | Docs, array functions |
| D8 | `WITH RECURSIVE` + `UNION` prefixing an `UPDATE` computes transitive descendants | **Verified** | M0 probe D4 (executed); M1 PREPARE sweep (type-checked) |
| D9 | `UNION` deduplication terminates the traversal on a **cyclic** graph | **Assumption → discharged by B-21** | Docs state `UNION` deduplicates and that a recursive subquery must eventually return no rows. No cycle has ever been executed against this schema. |
| D10 | pgx encodes a Go `[]string` into `text[]` | **Assumption → discharged by B-17** | M1 confirmed the *expected* type is `text[]` (`parameter_types = {uuid,text,text,text[]}`) but executed nothing |
| D11 | `RETURNING id` (UUID) scans into a Go `string` | **Assumption → discharged by B-01** | Preparable; never executed |
| D12 | Scenario-scoped traversal excludes cross-scenario children | **Assumption → discharged by B-19** | D-032 implemented at M1; never executed |
| D13 | `DROP DATABASE … CASCADE` / `CREATE DATABASE` available to the test role | **Verified** | Exercised by `cmd/m0verify --reset` throughout M0 |
| D14 | Concurrency, contention, isolation levels | **Out of scope** | M3 |
| D15 | Vector/embedding types | **Out of scope** | M4; R2 deferred |

No item is classified **Risk** or **Blocker**. The four Assumptions are precisely what M2
exists to discharge — and D1/D2/D10 are all exercised in **Wave 1**, so if any of them is
wrong, M2 stops on day 1.

---

# 5. Implementation order — three execution waves (D-047)

Waves are checkpoints, not milestones. **Every wave ends with `go test ./...` green**, and no
wave begins before the previous one is green. Acceptance criteria (§6) are unchanged and apply
to the milestone as a whole.

Waves are defined by **membership**, not by ID range: B-18's promotion into Wave 1 (D-048)
makes contiguous ranges impossible.

## Wave 0 — Harness (prerequisite, no cases)

| Stage | Work |
|---|---|
| **W0.1** | `internal/testdb`: reset `fable_test`, apply the frozen DDL, open the shared pool and the injection pool |
| **W0.2** | `TestMain`: the **safety guard**, the recorder, transcript + failure-artifact writers |
| **W0.3** | Smoke test: the four contracted tables exist in `fable_test` |

**Safety guard, non-negotiable.** `TestMain` **refuses to run** unless the target database name
ends in `_test`, and **prints the redacted DSN and database name before dropping anything**
(N1). The suite drops its database; a mistyped `FABLE_DSN` pointing at `fable`, or at a shared
cluster, would otherwise destroy data. The guard is a hard abort, not a warning.

**Wave 0 exit:** `go test ./...` green with one smoke test.

## Wave 1 — Core kernel and the retry gate

Everything whose failure would invalidate every later case. The three riskiest dependencies
(D10 encoding, D1/D2 retry injection, D11 UUID scan) are all discharged here.

| Stage | Cases | Work |
|---|---|---|
| **W1.1** | B-01, B-17, B-23 | `EnterBelief`; `[]string`→`text[]` encoding; Go/DDL debt drift |
| **W1.2** | B-02 | `AddEvidence` |
| **W1.3** | B-03, B-04 | `RetireDebt` and its idempotency |
| **W1.4** | B-05, B-09, B-10 | `Promote` — success, and both 23514 refusals |
| **W1.5** | **B-18** | **Retry classification (D-048 — immediately after `Promote`)** |
| **W1.6** | B-06, B-11 | `IntentOnPromoted` — success and the 23503 refusal |
| **W1.7** | B-12 | Raw skip-cancel refusal (I-4) |

**Wave 1 exit:** 13 cases green. **Checkpoint decision:** if D1, D2 or D10 proved wrong, STOP
here and escalate per §8 — do not proceed to Wave 2 with a broken foundation.

## Wave 2 — Cascade behavior

| Stage | Cases | Work |
|---|---|---|
| **W2.1** | B-07, B-20, B-22 | Cascade happy path, chain correctness, `RowsAffected` semantics |
| **W2.2** | B-19 | Cross-scenario isolation (D-032 executed) |
| **W2.3** | **B-24, B-16** | Blocked cascade: failure mode **and** single-transaction atomicity |
| **W2.4** | B-08, B-13 | Audit, per-case and the global sweep |

**Wave 2 exit:** 21 cases green. W2.3 is the milestone's most consequential test — a partial
effect surviving a failed cascade means the cascade is not one transaction.

## Wave 3 — Edge cases and closure

| Stage | Cases | Work |
|---|---|---|
| **W3.1** | B-21 | Cycle termination under deadline, elapsed time logged (N2) |
| **W3.2** | B-14 | I-6 column absence |
| **W3.3** | B-15 | `scripts/m2_accept.sh`; `check_i7.sh` excludes `_test.go`; transcript emission |
| **W3.4** | — | `docs/M2_REPORT.md` |

**Wave 3 exit:** 24 cases green; `./scripts/m2_accept.sh` exits 0.

---

# 6. Acceptance criteria

Unchanged from Revision 1, per B1's *"Acceptance stays identical."*

| # | Criterion |
|---|---|
| A1 | `go test ./...` exits 0 |
| A2 | `go test ./...` exits 0 on a **second consecutive run** without manual cleanup (I-6's second pass; also proves the suite is repeatable) |
| A3 | All 24 cases B-01 … B-24 appear in `docs/M2_TRANSCRIPT.md` with status PASS; zero skipped |
| A4 | B-09 and B-10 record SQLSTATE `23514` and constraint `promoted_is_debt_free` |
| A5 | B-11 records SQLSTATE `23503` and constraint `gate` |
| A6 | B-12 and B-24 record SQLSTATE `23514` and constraint `live_requires_promoted` |
| A7 | B-09, B-10, B-11 each assert `errors.Is` **and** `errors.As` succeed on the same error value |
| A8 | B-17 asserts stored `debt` is element-wise equal to `kernel.FullDebt`, same order, length 6 |
| A9 | B-19 returns exactly `2`; the scenario-B belief's status reads `promoted` |
| A10 | B-20 returns exactly `3`; the unrelated belief's status reads `promoted` |
| A11 | B-21 returns exactly `2` and completes within a 30-second context deadline |
| A12 | B-22 returns exactly `2` with exactly `3` intents in state `cancelled` |
| A13 | B-16 asserts, after the failed cascade, that same-scenario intents are `live` and beliefs are `promoted` — count of changed rows is `0`, and the transcript row reads `rollback verified` |
| A14 | B-18 returns a nil error, the belief reads `promoted`, and the error is not `ErrPromotionBlocked` |
| A15 | B-13's global sweep returns `0` live-on-non-promoted rows across **all** scenarios |
| A16 | B-14 returns `0` rows for `(table_name='belief', column_name='embedding')` |
| A17 | `./scripts/check_i7.sh` exits 0 within the M2 gate: 6 `ExecuteTx` sites, 0 raw writes |
| A18 | `./scripts/m2_accept.sh` exits 0 end to end |
| A19 | `go build ./...`, `go vet ./...` exit 0; `gofmt -l cmd internal` is empty |
| A20 | `docs/M2_TRANSCRIPT.md`, `docs/M2_TEST.md`, `docs/M2_REPORT.md` all present |
| **A21** | **On any failure**, `docs/M2_FAILURE.md` exists and contains a complete artifact per §6.1 for every failed case — and the run exits non-zero |

## 6.1 Failure artifact (D-049)

Acceptance criteria above define PASS. This defines FAIL.

**Every failing case emits a standardized architectural failure artifact** — written to
`docs/M2_FAILURE.md` and mirrored to stderr — in exactly this shape:

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

The artifact exists for architectural review, not CI. Three consequences for the
implementation:

- **`Likely invariant` is not guessed.** Every case declares the invariant it defends as a
  field on its table row (§3), so the artifact names it mechanically.
- **`Receipt` is verbatim.** The raw driver error, unedited — or, where the failure is that
  *nothing* went wrong when something should have, a dump of the row state that proves it.
  A refusal that did not happen is as much a receipt as one that did.
- **`Decision` is always STOP.** M2 has no "retry with different expectations" path. A
  behavioral divergence from the contract, M0 or M1 is a reportable architectural event
  (D-020), and the artifact is the report.

The transcript's verdict flips to **BLOCKED**, and `scripts/m2_accept.sh` exits non-zero
without running the second pass — a suite that failed once has nothing to prove by repeating.

---

# 7. Risks introduced by M2

M0's and M1's retired risks are not repeated. M1's open risks (M1-R2, R3, R4, R5) are not
risks here — they are the cases B-17, B-23, B-24/B-16 and B-19/B-20 exist to close.

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| **M2-R1** | **The suite drops a database.** A `FABLE_DSN` pointing at `fable`, or at a shared/cloud cluster, would destroy data that M0 and M1 transcripts were generated against. | **High** | W0.2's hard guard: abort unless the database name ends in `_test`; print the redacted DSN and database name before dropping (N1). |
| **M2-R2** | `inject_retry_errors_enabled` leaking into other cases would make unrelated tests retry or fail confusingly. | Medium | Set only in a **separate pool's DSN** (D2), used by B-18 alone and closed in `defer`. Never `SET` on the shared pool. |
| **M2-R3** | B-21's cycle test hangs forever if `UNION` does not terminate, converting a clear failure into a stuck CI job. | Medium | Explicit 30s context deadline (A11); elapsed time logged on success so creeping regressions are visible (N2). |
| **M2-R4** | `AuditLiveOnNonPromoted` is scenario-scoped by signature, so it cannot express a *global* invariant check. | Low | B-13's global sweep uses a raw query in the test. Recorded so the difference between the API and the invariant stays explicit. |
| **M2-R5** | Cases share one database; `t.Parallel()` could interleave writes across scenarios and make the global audit sweep flaky. | Medium | No `t.Parallel()`. Deterministic order; each case owns a distinct fixed scenario UUID; the global sweep runs last. |
| **M2-R6** | Fixed scenario UUIDs plus per-run reset means a leftover `fable_test` from an aborted run could collide. | Low | `TestMain` drops and recreates unconditionally before any case runs. |
| **M2-R7** | A green suite could be mistaken for proof of concurrent correctness. | Medium | Stated in `M2_REPORT.md`: M2 executes single-session behavior only. The cascade-vs-new-intent race is **M3** and is not evidenced here. |
| **M2-R8** | **Wave discipline could erode under time pressure** — landing Wave 2 cases while Wave 1 is red would reintroduce exactly the risk D-047 exists to remove. | Medium | Each wave has an explicit exit condition (§5) and Wave 1 has a named STOP checkpoint. `M2_REPORT.md` records which wave each case landed in. |

---

# 8. Escalation

If execution behavior differs from `IMPLEMENTATION_CONTRACT.md`, the M0 findings, or the M1
findings — **STOP, do not redesign, report receipts** via the §6.1 artifact. Specifically, M2
halts and reports rather than adapting if any of the following is observed:

- A refusal arrives with a SQLSTATE other than the one M0 recorded (D4, D5).
- `errors.Is` and `errors.As` do not both succeed on the same error (M1's wrap contract).
- `RetractCascade` retracts a belief outside its scenario (D-032 violated).
- A blocked cascade leaves any partial effect (I-8 violated — the most serious possible
  finding, since it would mean the cascade is not one transaction).
- B-21 does not terminate (D-033's `UNION` termination argument false).
- `[]string` fails to encode (D10) — the fallback `'{…}'::TEXT[]` is *pre-decided* but
  changing the kernel is an M1 amendment and requires a ruling, not a quiet edit.

Because D1, D2 and D10 are all exercised in Wave 1, each of these fails fast rather than after
thirteen stages (D-048).

## Carried, non-blocking

- **R2 — `belief.embedding`.** Contract §7 M2 requires *"the entire suite passes a second time
  with all embeddings NULL."* There is no embedding column, so the clause is **not literally
  executable**. M2 discharges it two ways: B-14 asserts the column's absence (making "every
  embedding is NULL" true by construction), and A2 runs the whole suite a second time
  (preserving the repeatability the second pass was there to give). The vacuity is recorded in
  `M2_REPORT.md` rather than glossed. **R2 is not solved here** — still deferred to M4, and
  M0's `schema_locked = true` finding still applies to whoever adds the column.
- **I-7 is not runtime-executable.** No test can prove a *negative* about code paths at
  runtime. M2 re-runs M1's static gate (B-15, A17) and says so plainly rather than claiming
  execution coverage it does not have.
- **M0 §3.1** — the M5 ruling on displaying `ConstraintName` beside a raw CHECK error is still
  open. M2 reads `ConstraintName` inside tests, which is not a demo surface, so nothing here
  depends on that ruling.

No conflict with the contract, M0, or M1 was found at planning time.

---

# Verification

```bash
cd /home/chaschel/Desktop/cockroach/hackathon/go
./scripts/m0_up.sh          # cluster on 26260 (fable_test is created by TestMain)
./scripts/m2_accept.sh      # M1 gate → go test → second go test → transcripts
echo "exit=$?"              # 0 == every M2 criterion green
```

Then confirm by reading, not by trusting the exit code:

- `docs/M2_TRANSCRIPT.md` — 24 rows, all PASS, none skipped; `23514`/`promoted_is_debt_free`
  on B-09 and B-10, `23503`/`gate` on B-11, `23514`/`live_requires_promoted` on B-12 and B-24.
- B-16's row reads **`rollback verified`** with zero rows changed — the atomicity receipt.
- B-18's row shows a nil error under injection — the retry receipt.
- B-21's row shows elapsed_ms well inside the deadline.
- `docs/M2_FAILURE.md` **does not exist** on a green run.
- `docs/M2_TEST.md` — both runs present, both exit 0.
- `docs/M2_REPORT.md` — records which wave each case landed in, and states plainly that M2
  covers single-session behavior only.

Stop at M2's close. **Do not begin M3.**
