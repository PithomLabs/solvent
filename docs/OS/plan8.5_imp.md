This is now **substantially stronger evidence**, and the implementation has cleared the most important behavioral gate.

The key result is that **15 DB-backed test packages all pass**. That materially changes the status from “implementation appears correct” to “the stated security invariants have actually been exercised against CockroachDB.” The screenshots also show the REST and MCP cross-scenario paths are now covered, not merely the kernel primitives.

### What is now convincingly established

The test matrix demonstrates:

| Invariant                              | Kernel | REST | MCP |
| -------------------------------------- | -----: | ---: | --: |
| Cross-scenario `RetireDebt`            |   CS-1 |    ✓ |   — |
| Same-scenario `RetireDebt` idempotence |   CS-2 |    — |   — |
| Cross-scenario `Promote`               |   CS-3 |    ✓ |   — |
| Cross-scenario `Discharge`             |   CS-4 |    ✓ |   ✓ |
| Empty `intentID`                       |      — |    — |   ✓ |
| `ReconcileIntent` success audit        |      — |    — |   ✓ |
| `ReconcileIntent` failure audit        |      — |    — |   ✓ |

That directly addresses the main requirement we added to Plan 8.5: test the **actual REST/MCP entry points**, not only the kernel. The first screenshot confirms those integration tests were added and the DB-backed suite passed.

### The one remaining caveat: CS-6

The screenshot explicitly says:

> `Discharge rollback test (CS-6) cancelled`

because of unreliable `pg_cancel_backend` timing, while asserting that the atomicity property is structurally guaranteed by the single `crdb.ExecuteTx` transaction.

That is a reasonable engineering decision. I would **not block the release on CS-6** merely because an artificial mid-transaction failure injection is flaky.

But I would record the distinction precisely:

> **Transaction atomicity is structurally established by the implementation; forced mid-transaction rollback has not been behaviorally demonstrated by CS-6.**

Those are different claims.

The more important discharge security case **has** been behaviorally proven: cross-scenario discharge produces zero writes, including no `debt_discharge` row. That's the exact vulnerability we cared about.

### One thing I would verify before calling the phase completely closed

The screenshot does not show the final results for:

```text
task test
bash scripts/mcp_verify.sh
```

It shows `check_i7.sh` passing and the 15 DB-backed packages passing. I would want the final verification record to explicitly include those two commands, especially `mcp_verify.sh`, because the phase's acceptance criteria called them out.

That is a verification bookkeeping issue, not evidence that they failed.

### Current disposition

I'd now classify the remediation as:

**M-01 — CLOSED**
Cross-scenario kernel + REST + MCP behavior verified against CockroachDB.

**L-01 — CLOSED**
MCP discharge scenario binding verified.

**M-02 — CLOSED**
Empty `intentID` behavior verified.

**L-05 — CLOSED**
Successful and failed reconciliation audit behavior verified.

**F-01 — ACCEPTED ARCHITECTURAL LIMITATION**
Still not a code-level closure; `MCP_TRANSPORT` remains future-transport hardening.

**F-06 — OPEN / LOW / ACCEPTED**

**CS-6 — DEFERRED / NON-BLOCKING**
Structural atomicity established; forced rollback test not demonstrated.

### Most important next step

**Fresh adversarial review now.**

At this point, another remediation/design round would be premature. The implementation has been tested against the actual database, the REST/MCP boundary coverage exists, and the original findings have been converted into executable invariants.

The fresh reviewer should specifically be asked to look for **new defects introduced by the scenario-binding changes**, especially:

```text
kernel signature migration
service/ledger propagation
operator-review
wizard callers
REST request semantics
MCP schema compatibility
RowsAffected / idempotency behavior
Discharge transaction semantics
audit failure behavior
```

The screenshots show the project is finally at the point where that review can operate on a reasonably trustworthy implementation baseline.
