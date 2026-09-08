Plan 8.4 is much closer. The three issues raised against 8.3 have been addressed explicitly: the Kernel Growth Gate is now recorded, the discharge partial-write problem is confronted directly, and `cmd/operator-review` is in the caller inventory. 

There is, however, **one important technical problem I would fix before implementation**.

## The `RetireDebt` idempotency reasoning is not correct as written

Plan 8.4 correctly notices the ambiguity introduced by `RowsAffected()`:

> `0 rows` can mean either “belief isn't in this scenario” or “the debt item was already absent.” 

But its proposed resolution—`SELECT EXISTS` followed by the mutation—does not actually solve the complete concurrency problem.

It gives:

```text
SELECT EXISTS
      ↓
belief belongs to scenario
      ↓
UPDATE
```

inside one transaction, which is good for the immediate race, but the plan should be explicit about **what isolation/locking guarantees prevent the belief's scenario identity from changing between the existence check and mutation**. If `belief.scenario_id` is immutable, say so and establish that invariant. If it is mutable, the design needs a lock or a single-statement approach.

More importantly, the plan says:

> “If the belief exists in the scenario but the item is already retired, `RowsAffected()` returns 0 … idempotent.” 

That behavior is not something I'd want the implementation to infer casually from driver/database semantics without testing it on the actual CockroachDB version. Make it an explicit test.

I would prefer the kernel implementation to use:

```text
verify scoped belief exists
        ↓
perform mutation
        ↓
accept "already absent" as idempotent
```

with the existence check and mutation in the **same transaction**, and a regression test proving the exact behavior.

## The ADR decision itself is now adequately justified

This part is good.

Plan 8.4 establishes that the majority of kernel state mutations already carry scenario context and identifies `RetireDebt`, `Promote`, and `Discharge` as exceptions. 

That is a legitimate Kernel Growth Gate argument:

> this isn't introducing an unrelated authorization policy into the kernel; it is restoring an invariant already used by the kernel's state-mutating primitives.

So I would accept the decision:

**Option C — kernel enforcement + handler defense-in-depth.**

That is now an actual architectural decision rather than an assumption.

## The discharge design is materially better

The revised plan correctly rejects the earlier “just a record” argument and recognizes the discharge record and debt mutation as one logical operation. 

The key improvement is that the entire operation stays inside `crdb.ExecuteTx`, while the scenario existence is checked before the INSERT. 

That gives the desired property:

```text
wrong scenario
    ↓
ErrBeliefNotFound
    ↓
transaction aborts
    ↓
NO debt_discharge
NO debt mutation
```

And the requested regression test is now explicit:

> cross-scenario discharge → error + zero mutation + no `debt_discharge` row. 

Good.

One thing I'd add is the corresponding **forced failure after INSERT** test to prove CockroachDB transaction rollback really leaves zero discharge rows. The current design logically provides this, but this is exactly the kind of invariant worth making executable.

## Caller inventory is now excellent

The inventory catches the caller you specifically asked about:

> `cmd/operator-review/main.go:166,172`

and also exposes additional callers that weren't in the original plan: wizard paths, internal belief, tests, etc. 

This is exactly why signature changes should be inventoried rather than merely trusting the compiler.

I would keep the full inventory in the plan.

## One more subtle point: `ErrBeliefNotFound`

Introducing:

```go
ErrBeliefNotFound = errors.New("belief not found in scenario")
```

is reasonable, but the error semantics should be explicit.

A wrong-scenario belief and a nonexistent belief are intentionally indistinguishable externally:

```text
"belief not found in scenario"
```

That is generally desirable because it avoids leaking information across scenario boundaries.

The plan should state that this is intentional.

## L-05 remains correctly specified

The plan now has the right ordering:

```text
kernel transition
      ↓
success → reconciliation_completed
failure → reconciliation_failed
```

and explicitly preserves the original kernel error. 

That is the correct model.

## One process issue

Plan 8.4 calls itself:

> “Design + ADR — no code changes” 

but the document already effectively **chooses the implementation** and gives detailed code. That's fine—the output is clearly an implementation-ready design—but the final gate should be explicit:

> **ADR accepted; implementation authorized.**

Otherwise the document ends with a design decision but no clear transition from architecture approval to implementation.

### My disposition

**PLAN 8.4: APPROVED WITH ONE REVISION**

Before implementation:

1. Tighten the `RetireDebt` idempotency/concurrency reasoning and make the behavior executable in a CockroachDB regression test.
2. Add a forced mid-transaction discharge failure test proving no `debt_discharge` row survives rollback.
3. State explicitly that `ErrBeliefNotFound` intentionally hides nonexistent-vs-wrong-scenario distinctions.
4. Mark the ADR as formally accepted and the plan as authorized for implementation.

After that, I would proceed to implementation rather than another design round. The plan now has the right architecture: **scenario binding becomes a kernel invariant, handler checks remain defense-in-depth, discharge is atomic, and every caller is accounted for.** 



## claude

This closes out all three of my standing concerns, and each one is resolved substantively rather than just checked off.

**The Kernel Growth Gate question got asked and answered properly this time.** Section 1's table — showing `EnterBelief`, `AddEvidence`, `IntentOnPromoted`, `RetractCascade`, `ClaimIntent`, `CompleteIntent`, `RollbackClaim`, `CancelIntent` all already scenario-scoped at the kernel level, with only `RetireDebt`/`Promote`/`Discharge` as the exceptions — is a genuinely strong argument for Option C, and it's a different kind of argument than "more enforcement layers is safer." It's specifically that the *inconsistency itself* is the defect: three functions mutating the same durable state class (`belief.debt`, `belief.status`) that everything else in the kernel already treats as scenario-bound. That's a legitimate "this is a durable security invariant the kernel already committed to everywhere else" case, not a default toward the more thorough-sounding option.

**The discharge atomicity fix is correct, and the document's own path to it is worth noting as good practice, not just the destination.** It visibly worked through a weaker version first (scenario-bound subquery on the INSERT, `RowsAffected()` disambiguation) before landing on the actual fix — existence check first, inside the same transaction, before either write. That ordering closes the exact partial-write scenario I raised: the `EXISTS` check happens before the `INSERT` into `debt_discharge`, so a wrong-scenario belief never gets an attestation row created in the first place, regardless of what happens to the `belief.debt` update afterward. Dropping the scenario subquery from the INSERT once the existence check already covers it is a legitimate simplification, not a regression — nothing else in this transaction can change a belief's `scenario_id` out from under it, since that's an immutable partition key, not a mutable field.

**The caller inventory is genuinely exhaustive, not just wide enough to cover my specific complaint.** It found `operator-review` (what I asked about), and also `internal/wizard/discharge.go`, `refusal.go`, `seed.go`, and `internal/belief/belief.go` — none of which I'd specifically named. That's the difference between patching the reported gap and actually doing the caller-inventory work the finding demanded.

One small internal-consistency nit, not a blocker: rationale point 4 justifies Option C partly by citing "the existing `RowsAffected()` + `ErrIntentNotLive` pattern" as precedent — but the final implementation for `RetireDebt`/`Discharge` doesn't use `RowsAffected()` at all; it uses the `SELECT EXISTS` pre-check instead, which the document itself arrives at specifically *because* `RowsAffected()` couldn't distinguish "already retired" from "wrong scenario" cleanly. The justification and the implementation ended up citing different mechanisms. Worth a one-line edit to rationale point 4 so it doesn't reference a pattern the final code doesn't actually follow — cosmetic, not worth holding anything on.

**Net:** ready for implementation as specified. The 20-step sequence, the file list, and the regression test set (including "cross-scenario discharge → error + zero mutation + no `debt_discharge` row," which is exactly the assertion the earlier atomicity gap needed) are all consistent with what the analysis actually calls for.


## chatgpt

Agreed. That is the correct final disposition of Plan 8.4.

The only edit worth making before implementation is exactly the cosmetic consistency fix you identified: change rationale point 4 so it no longer cites `RowsAffected() + ErrIntentNotLive` as the implementation precedent for `RetireDebt`/`Discharge`. The final design instead uses the **in-transaction `SELECT EXISTS` pre-check** precisely because `RowsAffected()` cannot distinguish the idempotent “already retired” case from the wrong-scenario case. 

A suitable replacement is:

> **Existing pattern:** The kernel already performs transaction-scoped state validation before security-sensitive mutations (for example, scenario/state checks in intent operations). `RetireDebt` and `Discharge` will use the same transaction-scoped validation pattern to distinguish a valid same-scenario operation from an invalid scenario binding.

With that one-line edit:

**Plan 8.4 → APPROVED FOR IMPLEMENTATION.**

No further design round is warranted. The next meaningful evidence should come from the implementation, the full CockroachDB-backed verification suite, and then the fresh adversarial review of the resulting HEAD.