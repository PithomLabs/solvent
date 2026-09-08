I would **not approve Plan 8.3 unchanged**. The overall remediation direction is right, but there is one important design flaw in the proposed M-01 fix that should be corrected before implementation.

### 1. The kernel-level scenario binding is the right direction

Moving scenario enforcement into the kernel is preferable to relying solely on handlers. The plan correctly identifies the underlying problem: the mutations currently operate on `belief_id` alone. 

This is especially good:

```text
REST/MCP handler
      ↓
kernel mutation(scenarioID, beliefID, ...)
      ↓
DB invariant
```

rather than:

```text
REST handler → guard
MCP handler → guard
future caller → forgets guard
kernel → trusts caller
```

The latter is exactly how this class of bug comes back.

### 2. But `sqlDischargeInsert` must NOT be treated as harmless

This is the part I would change.

Plan 8.3 says:

> “The INSERT into `debt_discharge` is just a record; the mutation is the `belief.debt` update.” 

That is too narrow for an authority/audit ledger.

A `debt_discharge` row is itself a **durable assertion that a particular obligation was discharged**. If the insert can occur for a cross-scenario belief while the subsequent `belief.debt` update fails, you've still created an integrity violation.

The safe invariant should be:

> **A discharge record can only be created when the referenced belief belongs to the supplied scenario, and the discharge operation must not partially succeed across its constituent writes.**

So I would change the proposed SQL approach to make the insert scenario-bound too.

For example, conceptually:

```sql
INSERT INTO debt_discharge
    (belief_id, obligation_key, instrument_ref, discharged_by)
SELECT
    b.id, $2::STRING, $3::STRING, $4::UUID
FROM belief b
WHERE b.id = $1::UUID
  AND b.scenario_id = $5::UUID
```

Then the surrounding transaction must ensure the corresponding debt-removal update also succeeds.

Even better, the whole `Discharge` operation should be an atomic transaction with both operations sharing the same `(scenario_id, belief_id)` invariant.

The existing plan already recognizes that `Discharge` is composed of an INSERT plus a belief mutation.  The missing step is treating **both** as security-sensitive state changes.

### 3. Do not rely on “zero rows affected” without defining the contract

The plan says the handler-level guard gives a clearer error while the kernel provides authoritative enforcement. 

Good—but the kernel API should explicitly distinguish:

```text
belief absent
belief exists but wrong scenario
mutation failed
```

from:

```text
success
```

Otherwise a cross-scenario UPDATE that affects zero rows can accidentally become a misleading success/no-op.

The implementation should inspect `RowsAffected()` and return a specific not-found / wrong-scope error where appropriate.

### 4. `handleSolventDischarge` is the strongest reason to fix this centrally

The plan correctly identifies L-01 as the same class of defect. 

This makes the kernel-bound invariant even more valuable. Otherwise you'd have:

```text
REST
  └─ guard + kernel scenario binding

MCP retire
  └─ guard + kernel scenario binding

MCP promote
  └─ guard + kernel scenario binding

MCP discharge
  └─ guard + kernel scenario binding
```

That is four opportunities for future divergence.

The kernel should become the final authority on:

> `(scenario_id, belief_id)` must match for mutation.

The handler guards can remain for better errors, but they should explicitly be defense-in-depth.

### 5. M-02 is exactly right

No objection here. The `ExecuteAction` precondition belongs at the service boundary, and the plan correctly makes it mandatory before `PrepareForAction`. 

I would retain the regression test.

### 6. L-05 → Medium is justified

The proposed change is also correct:

```text
kernel transition
       ↓
success → Completed audit
failure → ReconciliationFailed audit
```

rather than logging `ActivityReconciliationCompleted` first. 

The plan should make one thing explicit: **the failure-event audit itself must not obscure the original kernel error**. If audit logging fails, the operation should still return the state-transition error, while the audit failure is separately observable.

### 7. Add one more regression category

The current acceptance tests cover cross-scenario REST/MCP rejection and zero mutation. 

Add:

> **Cross-scenario discharge must produce neither a `debt_discharge` row nor a change to `belief.debt`.**

That catches the exact partial-write failure that the current plan's treatment of `sqlDischargeInsert` could otherwise miss.

Also add the positive counterpart:

> Valid same-scenario discharge creates exactly one discharge record and removes exactly the intended debt item atomically.

### Recommended disposition

I would keep the plan's architecture but amend M-01:

**APPROVE WITH CORRECTION**

Change the discharge design from:

> scenario-bind `sqlDischargeRetireDebt`; leave `sqlDischargeInsert` effectively unscoped

to:

> **scenario-bind the complete `Discharge` transaction, including the durable discharge-record INSERT, and prove cross-scenario attempts produce zero writes.**

Everything else in the plan is substantially on target. The acceptance criteria and invariant list are also aligned with the existing architecture. 

The key principle is worth locking in:

> **A scenario boundary must protect the entire state transition, not merely the final row mutation.**



## claude

Three things here, and I want to lead with the one that's most consistent with this entire thread's own history rather than the two implementation-level gaps.

## The kernel SQL/signature change should go through a Kernel Growth Gate ADR, and it isn't

Every other kernel-touching decision in this project's history — `ClaimIntent`, `CompleteIntent`, the `FOR UPDATE` locks added in Phase 6.4, the composite `AuthorizeAndCreateIntent` primitive in ADR-0002 — went through an explicit ADR asking "is this a genuine atomic security primitive that cannot be expressed outside the kernel." Sometimes the answer was yes and the kernel grew; sometimes it was no and the fix stayed in the service layer. Either way, the question got asked and answered on the record before code changed.

This plan changes kernel SQL (`sqlRetireDebt`, `sqlPromote`, `sqlDischargeRetireDebt`) and kernel function signatures (`RetireDebt`, `Promote`, `Discharge`) — unambiguously the frozen core — and the entire justification is "Step 1: Add scenario_id to kernel SQL," with no ADR, no growth-gate analysis, nothing. What makes this specifically worth pushing on is that the plan itself documents a complete, sufficient, zero-kernel-change alternative in the same breath: "Option B" — the handler-level `view.GetSnapshot()` guard the MCP surface already uses, already reviewed, already accepted. The plan chooses "Option A for kernel SQL, with Option B as defense-in-depth" without ever asking why Option A is needed at all, given Option B alone closes the actual vulnerability the same way MCP already closes it. I'm not saying Option A is wrong — it may well be the better long-term answer, the same way the `FOR UPDATE` locks were the better answer to the intent-creation race even though a service-layer workaround existed. But that conclusion needs to be *reached*, the same way it was reached for every prior kernel change, not defaulted into because it sounded more thorough. Route this through the same ADR process before touching `kernel/sql.go`.

## The discharge fix has a gap the plan's own reasoning talks past

The plan dismisses scoping `sqlDischargeInsert` by scenario with: *"the INSERT into `debt_discharge` is just a record; the mutation is the belief.debt update."* But that's the exact assumption L-05 just proved dangerous one finding earlier — a record that doesn't correspond to what actually happened is worse than no record. Walk through the cross-scenario attack with only `sqlDischargeRetireDebt` scoped: `sqlDischargeInsert` still succeeds unconditionally, creating a `debt_discharge` row asserting "obligation X was discharged for belief Y" — while `sqlDischargeRetireDebt` correctly no-ops (zero rows, scenario mismatch), leaving `belief.debt` untouched. Now the attestation ledger says the obligation was discharged and the belief's actual debt array says it wasn't. If anything downstream — a future audit query, a compliance export, a human trusting the ledger over re-deriving state from `belief.debt` — treats `debt_discharge` as authoritative for "was this discharged," it's wrong, and it's wrong in exactly the misleading-audit-record direction L-05 was just fixed to prevent. This needs either the INSERT scoped the same way (via a join or subquery against `belief.scenario_id`, which the plan's own text acknowledges is awkward but doesn't resolve) or the INSERT and UPDATE wrapped in one transaction where a zero-row UPDATE result aborts the INSERT too. Add a regression test for this specific case — cross-scenario discharge attempt, then assert *no* `debt_discharge` row exists, not just that `belief.debt` is unchanged.

## `cmd/operator-review/main.go` is a caller of these kernel functions and isn't in the Files Modified list

`operator-review` is the CLI humans use to promote beliefs and retire debt — it's been part of this project's design since the earliest agentjacking demo work ("Control: legitimate authority — operator path via `cmd/operator-review`"). If `kernel.Promote`/`kernel.RetireDebt` signatures gain a required `scenarioID` parameter, this tool almost certainly calls them too and would fail to compile without an update — yet it's absent from both the "Files Modified" table and the implementation sequence. Worth confirming directly (`grep -rn "kernel.Promote\|kernel.RetireDebt" cmd/operator-review/`) before this proceeds, since "the Go compiler catches missed callers" (the plan's own stated mitigation for signature-change risk) only helps if someone actually runs the build against the whole tree and reads the errors — it doesn't help if the file was never identified as needing a look in the first place.

**Net:** M-02 and L-05's fixes are clean, well-modeled on existing correct patterns in this codebase, and ready to implement as written. M-01 needs the ADR question answered before the kernel SQL changes proceed, the discharge-atomicity gap closed regardless of which option is chosen, and `operator-review` checked and added to scope.


## chatgpt

Agreed. Those are the right three pushes, and they expose an important process issue: **Plan 8.3 is trying to turn a demonstrated service/API isolation defect into a kernel change without first applying the project's own kernel-growth discipline.**

I would revise the plan before any implementation.

### 1. M-01 must pass the Kernel Growth Gate first

The current plan explicitly identifies a viable zero-kernel-change alternative:

> Option B: add the existing `view.GetSnapshot()` cross-scenario guard at the handler layer. 

Yet it immediately recommends kernel growth without doing the architectural comparison that previous kernel changes required. That is the missing step.

The ADR should answer:

**Question:** Is `(scenario_id, belief_id)` a durable security invariant that requires atomic enforcement at the kernel/DB transition, or is the existing service-layer authorization guard sufficient?

Evaluate at least:

```text
Option A — Kernel enforcement
  RetireDebt(scenarioID, beliefID, ...)
  Promote(scenarioID, beliefID, ...)
  Discharge(scenarioID, beliefID, ...)
  DB predicates enforce scope

Option B — Service/handler enforcement
  GetSnapshot(scenarioID, beliefID)
  kernel primitive remains unchanged
```

And explicitly apply the project's growth rule:

> **Does the new invariant represent a genuinely new atomic security fact/state transition that cannot be safely expressed outside the kernel?**

There is a plausible argument for A: every kernel mutation touching a belief should cryptographically/transactionally respect its scenario, regardless of caller.

There is also a plausible argument for B: scenario authorization may be a service-level policy, while the kernel's job is to mutate a specific already-authorized belief identified by UUID.

**Do not prejudge the answer.** The ADR should decide it.

This is exactly the sort of decision that belongs on record before `kernel/sql.go` and kernel signatures are changed.

### 2. The discharge operation must be treated atomically

I agree completely with your attack sequence.

The proposed Plan 8.3 implementation creates a dangerous possibility:

```text
sqlDischargeInsert
        ↓
SUCCESS
        ↓
sqlDischargeRetireDebt
        ↓
0 rows because scenario mismatch
```

Result:

```text
debt_discharge:    "discharged"
belief.debt:       obligation still present
```

That is an authoritative-state contradiction, not merely a stale record.

And it is especially important because Plan 8.3 is simultaneously correcting **false-positive audit semantics** in L-05.  The plan should not fix false audit evidence in one path while introducing a structurally similar false discharge record in another.

The remediation plan should therefore require:

> **Cross-scenario discharge produces zero durable effects.**

And specifically test:

```text
before:
  belief.debt contains obligation
  no matching debt_discharge row

attempt:
  scenario A + belief from scenario B

after:
  belief.debt unchanged
  no debt_discharge row
```

For the valid case:

```text
scenario matches
    ↓
debt removal + discharge record
    ↓
both commit
```

That strongly suggests the `Discharge` operation needs one transaction encompassing both writes, regardless of whether the final scenario-binding enforcement lives in the kernel or service layer.

The plan should therefore replace its current dismissal of `sqlDischargeInsert` with an explicit **atomicity invariant**.

### 3. `operator-review` absolutely needs to be inspected

This is an obvious scope hole in the existing plan.

Plan 8.3 proposes changing:

```text
RetireDebt(...)
Promote(...)
Discharge(...)
```

signatures and says the compiler will catch missed callers. 

That is a useful safety net, but the implementation inventory should still identify all callers before modification.

The first cheap check is:

```bash
grep -rn \
  -e 'kernel\.Promote' \
  -e 'kernel\.RetireDebt' \
  -e 'kernel\.Discharge' \
  cmd/operator-review cmd api service kernel
```

Also search for method calls without the `kernel.` qualifier where a ledger/kernel object is used.

`cmd/operator-review` should be explicitly added to the plan if it calls these functions.

And because this is the human control path, I'd go one step further: **confirm whether operator-review already derives or validates scenario context before invoking the mutation.** Don't assume it behaves like MCP.

### Revised Plan 8.3 disposition

I would now rename/reframe it as:

> **Plan 8.3 — M-01/M-02/L-01/L-05 Remediation Design + Kernel Growth Gate**

and make the implementation sequence:

```text
1. Kernel Growth Gate ADR for M-01
2. Inventory every RetireDebt/Promote/Discharge caller
3. Decide kernel vs service enforcement from the ADR
4. Design Discharge as an atomic state transition
5. Implement M-01/L-01 according to ADR
6. Implement M-02
7. Implement L-05
8. Add cross-scenario + discharge atomicity regression tests
9. Run full verification with CockroachDB available
10. Fresh adversarial review
```

The key acceptance criterion should become:

> **No mutation surface can alter a belief or create a discharge record using a `(scenario_id, belief_id)` combination that does not belong together, and no discharge attempt can leave a partial durable record.**

That is a much stronger architectural target than “add scenario_id to three SQL statements.”

So I agree with the net assessment: **M-02 and L-05 are ready to implement; M-01 is not yet implementation-ready until the Kernel Growth Gate ADR is answered; discharge atomicity must be explicit; and `cmd/operator-review` must be brought into the caller inventory.**
