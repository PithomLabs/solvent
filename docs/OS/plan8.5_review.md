Plan 8.5 is good and appropriately narrow: it moves from structural verification to **actual database-backed behavioral evidence**. The test cases directly correspond to the remediated invariants, especially cross-scenario isolation, discharge consistency, `intentID` ownership, and truthful reconciliation auditing. 

I would approve it **with two important additions before running it**.

## 1. M-01/L-01 need endpoint-level tests, not only kernel tests

This is the biggest remaining gap.

The plan tests:

```text id="1w1u62"
RetireDebt(scenarioB, beliefA)
Promote(scenarioB, beliefA)
Discharge(scenarioB, beliefA)
```

directly against the kernel. 

That proves the new kernel invariant, but M-01 and L-01 were originally discovered as **REST/MCP surface defects**. Plan 8.4 deliberately chose both kernel enforcement **and** handler-level defense-in-depth.

So Plan 8.5 should also test the actual entry points:

```text id="x4zq3d"
REST:
  wrong scenario + belief UUID
    → HTTP error
    → zero mutation

MCP:
  wrong scenario + belief UUID
    → tool error
    → zero mutation
```

For discharge, specifically:

```text id="sqg7ad"
MCP wrong-scenario discharge
→ no debt_discharge row
→ debt unchanged
```

and the corresponding REST case.

Otherwise the test suite can pass while a future refactor accidentally removes the handler guard, leaving the kernel as the only protection. The system would still be secure, but the **defense-in-depth contract** would no longer be verified.

I would add these as integration tests rather than expanding the kernel test file.

## 2. Add one explicit discharge rollback test if practical

The current discharge tests prove the important cross-scenario property:

> error + zero `debt_discharge` rows + zero debt mutation. 

And the implementation keeps the existence check and both writes inside the transaction, so the design is sound.

But the claimed property is stronger:

> **the two constituent writes are atomic.**

The current positive test only proves both succeed together. It does not exercise a failure after one write has logically occurred.

Add a test where the transaction is forced to abort after the first durable operation, then assert:

```text id="1cd1kz"
no debt_discharge row
belief.debt unchanged
```

The exact mechanism for inducing the failure can be chosen from what CockroachDB/test infrastructure already supports. Don't add elaborate test-only infrastructure merely to satisfy this criterion; the important thing is to get one demonstrable rollback case.

## 3. The rest of the plan is solid

The kernel cases are well chosen:

* cross-scenario `RetireDebt`
* same-scenario idempotent `RetireDebt`
* cross-scenario `Promote`
* cross-scenario `Discharge`
* same-scenario atomic `Discharge` 

The authority tests likewise address the exact newly introduced contracts:

```text id="2wdh8v"
ExecuteAction("")
→ error
→ no executor invocation
```

and:

```text id="kv0q3q"
reconcile success
→ completed audit + executed state

reconcile failure
→ failure audit + original error + intent remains live
```



That is the right level of behavioral testing.

## 4. One wording correction

The heading says:

> `Repository: HEAD 5933f44 (with Plan 8.4 changes applied)` 

Once the implementation has actually been applied, the **Git HEAD should presumably have changed**, unless the implementation is uncommitted working-tree changes.

So before execution, establish:

```bash
git status --short
git rev-parse HEAD
git diff --stat
```

and record the actual tested revision/working tree state. Otherwise later reviewers can confuse "Plan 8.4 was applied" with a particular immutable commit.

## 5. The CockroachDB step is correct

Starting the dedicated `solvent-crdb` container and resetting the database is appropriate for this test pass. 

I would just make the safety assumption explicit:

> `solvent-crdb` is the dedicated disposable test database; `task db:reset` is expected to destroy/recreate its test state.

That avoids an accidental destructive reset against anything shared.

## Recommended acceptance additions

Add:

```text
- [ ] REST cross-scenario RetireDebt → error + zero mutation
- [ ] REST cross-scenario Promote → error + zero mutation
- [ ] REST cross-scenario Discharge → error + zero mutation + no discharge row
- [ ] MCP cross-scenario Discharge → error + zero mutation + no discharge row
- [ ] Discharge transaction rollback → no partial durable state
```

The rest of the acceptance criteria already map cleanly to the intended fixes. 

### Verdict

**Plan 8.5: APPROVE WITH TWO ADDITIONS**

The kernel/security behavior is now being tested against real CockroachDB, which is exactly the missing evidence. But because the original defects were discovered at the **REST/MCP boundaries**, the final verification must exercise those boundaries too.

After those additions and a successful DB-backed run, the next step should be the fresh adversarial review—not another design pass.
