Plan 9.0 is **very close to implementation-ready**, and the remediation strategy is substantially correct. The three actual security findings are attacked at the right layers: `ClaimIntent` becomes an atomic ownership check, `IntentOnPromoted` becomes scenario-bound inside the transaction, and `AddEvidence` gets both kernel enforcement and REST defense-in-depth. 

I would make **four corrections before letting the coding agent execute it**.

## 1. NEW-01: explicitly verify intent tuple immutability

The proposed fix is strong:

```sql
WHERE id = $1
  AND scenario_id = $2
  AND belief_id = $3
  AND action = $4
  AND state = 'live'
```

That makes the claim atomic. 

But the argument that:

> `CompleteIntent`, `RollbackClaim`, and `CancelIntent` remain unchanged because binding was enforced at claim time

implicitly assumes the claimed intent's:

```text
scenario_id
belief_id
action
```

cannot subsequently be modified.

That needs to be verified explicitly before implementation.

Add to the plan:

> Verify that no reachable code path updates `action_intent.scenario_id`, `action_intent.belief_id`, or `action_intent.action` after creation. These fields must be immutable for claim-time binding to remain authoritative.

This is important because otherwise:

```text
claim validates tuple
        ↓
tuple changes
        ↓
CompleteIntent acts on different semantics
```

could reopen the problem through a different path.

I expect the fields are effectively immutable, but the plan should establish that rather than assume it.

## 2. NEW-01 test count is internally inconsistent

The plan says:

> “NEW-01 regression: 5 tests pass (happy path + 4 rejection cases + exact exploit)” 

That's actually **6 tests**:

1. happy path
2. wrong belief
3. wrong action
4. wrong scenario
5. exact exploit
6. — actually, wait: the listed “4 rejection cases” are wrong-belief, wrong-action, wrong-scenario, and the exact exploit is the fourth rejection case.

So the correct count is:

**5 tests total = 1 happy path + 4 rejection cases.**

The parenthetical should simply say:

> **5 tests: happy path + four rejection cases, including the exact NEW-01 exploit.**

The acceptance criterion itself is therefore fine; only the wording is misleading.

## 3. NEW-03 REST guard should use the same established guard pattern

The proposed REST code is:

```go
if _, err := view.GetSnapshot(...); err != nil {
    ...
}
```



Earlier established handlers used the stronger pattern:

```go
snap, err := view.GetSnapshot(...)
if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
    ...
}
```

The new implementation should use the **same semantic guard**, unless `GetSnapshot`'s contract has since been proven to make the shorter check equivalent.

Otherwise this remediation introduces two subtly different definitions of "belief belongs to scenario."

Given the history of this project, duplicated security predicates should be minimized.

## 4. NEW-02 should explicitly test `AuthorizeAndCreateIntent`, not just `IntentOnPromoted`

The plan correctly notices that both call `createIntentWithinTx`:

> “This protects both `IntentOnPromoted` and `AuthorizeAndCreateIntent`.” 

That's excellent.

But because `AuthorizeAndCreateIntent` is security-critical and previously received special concurrency treatment, I would add a direct regression test:

```text
AuthorizeAndCreateIntent(
    scenarioB,
    beliefA,
    ...
)
→ ErrBeliefNotFound
→ zero action_intent rows
```

Otherwise the helper-level test gives reasonable coverage, but the second security-sensitive caller isn't directly exercised.

This becomes especially valuable because Plan 9.0 is explicitly relying on the shared helper to secure **both paths**.

---

## One thing I especially like

The NEW-02 remediation does not merely patch the eventual `RetractCascade` failure. It prevents the malformed relation from entering the database in the first place:

```text
scenario B + belief A
       ↓
transaction-local EXISTS
       ↓
false
       ↓
ErrBeliefNotFound
       ↓
NO intent
```

That is the right place to stop the attack. 

Likewise NEW-03 moves the invariant into `AddEvidence` itself rather than trusting only the REST endpoint. 

## NEW-05 is also appropriately scoped

Fixing `db:reset` to apply migrations 005–008 is reasonable because the reported defect is a mismatch between the repository's default database bootstrap and the verification script. 

The important thing is to verify afterward:

```bash
task db:reset
bash scripts/mcp_verify.sh
```

against the same default `fable` database, not merely continue using the previously migrated special-purpose database.

## Disposition

**Plan 9.0: APPROVE WITH MINOR CORRECTIONS**

Before execution, amend it to:

1. verify `action_intent` tuple fields are immutable after creation;
2. correct the NEW-01 test wording to **5 total tests**;
3. make the REST `AddEvidence` guard match the established `GetSnapshot` ownership predicate;
4. add a direct `AuthorizeAndCreateIntent` cross-scenario regression test.

After those edits, I would **let the coding agent implement it**.

The central security model is now getting considerably stronger:

```text
creation:
(scenario, belief) must match

execution:
(scenario, intent, belief, action) must match

evidence:
(scenario, belief) must match

mutation:
(scenario, belief) must match
```

That is the coherent invariant the earlier remediation was missing.
