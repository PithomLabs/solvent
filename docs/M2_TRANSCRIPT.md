# M2 Behavioral Transcript

Milestone M2 — the invariant suite, executed against a live CockroachDB cluster.
Every row is a receipt: SQLSTATE and constraint name come from `*pgconn.PgError`,
never from substring matching.

M2 executes **single-session** behavior only. Concurrency is M3 and is not evidenced here.

<!-- volatile:begin -->

| run fact | value |
|---|---|
| generated_at | 2026-09-08T22:50:56Z |
| host | linux |
| dsn | postgresql://root@localhost:26260/fable_kernel_test?sslmode=disable |

<!-- volatile:end -->

## Verdict

**GREEN** — 3/3 cases passed.

## Cases

| id | wave | status | purpose | expected | observed | sqlstate | constraint | invariant | elapsed_ms |
|---|---|---|---|---|---|---|---|---|---|
| CS-1 | cs | PASS | M-01 — RetireDebt with wrong scenario returns ErrBeliefNotFound, zero mutation | ErrBeliefNotFound + debt unchanged | err=belief not found in scenario, debt_before="needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff", debt_after="needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff" | — | — | cross-scenario isolation for RetireDebt | 19 |
| CS-2 | cs | PASS | M-01 — same-scenario RetireDebt is idempotent | both calls succeed + debt item removed | err1=<nil>, err2=<nil>, debt="needProvenanceCheck,needContradictionSweep,needRollbackPlan,needVersionPin,needOperatorSignoff" | — | — | RetireDebt idempotency within scenario | 14 |
| CS-5 | cs | PASS | M-01 — same-scenario Discharge is atomic: discharge row + debt retired | success + 1 discharge row + debt item removed | err=<nil>, discharge_rows=1, debt="needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff" | — | — | Discharge atomicity within scenario | 28 |

## Receipts

### CS-1

```
belief not found in scenario
```

