I reviewed Plan 10.1 against the amendments we agreed on. It is now **implementation-ready**, with two small precision improvements I would make before handing it to the coding agent.

### 1. Tighten the NULL/FK explanation

The migration is correct, and the fail-closed behavior is correct. The wording at §11:

> “The composite FK permits NULLs (PostgreSQL/CockroachDB NULL semantics: NULL != NULL passes FK check).”

is slightly imprecise.

The important rule is that a foreign-key constraint does not require a match when a referencing column is `NULL`. So the clean wording is:

```text
Existing pre-authority intents retain NULL target_id/snapshot_id.
The composite FK permits these legacy rows because nullable
referencing columns do not require an FK match.

However, ClaimIntent requires concrete target_id and snapshot_id
in its CAS predicate, so such intents cannot be claimed.

Therefore the migration fails closed without fabricating authority
bindings for legacy intents.
```

That says exactly what you need without relying on the misleading `NULL != NULL` formulation.

### 2. Add one concurrency test for duplicate intent creation

The plan correctly handles the duplicate case with the unique index and `23505` mapping to `ErrDuplicateIntent`. 

I would add one test beyond the existing sequential duplicate test:

```text
TestCS_EXACT_ConcurrentDuplicateIntent

Two concurrent AuthorizeAndCreateIntent calls
for the exact same authority tuple:

    → exactly one intent is created
    → exactly one caller succeeds
    → exactly one caller receives ErrDuplicateIntent
    → exactly one live intent remains
```

This is particularly valuable because the project's central design principle is that concurrency-sensitive security properties should be established by the database, not by lucky application sequencing.

It also complements the new exact-binding CAS nicely.

### Everything else is aligned

The composite FK is now explicitly present in the migration and documented as the durable `(target_id, snapshot_id)` invariant. 

The conflict distinction is now exactly right:

> `ErrDuplicateIntent` → `409 Conflict` and **not** an authorization denial. 

The plan also correctly avoids accidentally turning this into an idempotency project, and explicitly keeps the kernel freeze intact. 

One other detail I like: the plan now explicitly proves the database invariant itself through mismatched and nonexistent `(target_id, snapshot_id)` insertion tests, rather than merely testing Go's execution path. 

### Verdict

**PLAN 10.1 — APPROVE FOR IMPLEMENTATION**, with the two editorial additions above.

The architecture is now clean:

```text
approval
   ↓
target_activation
   ↓
(target_id, snapshot_id)
   ↓
action_intent
   ↓
atomic ClaimIntent
   ↓
executor
```

And the crucial property is enforced at the database boundary, not entrusted to the caller. 

After this change, the kernel-freeze declaration in §19 is defensible: the project has added the final missing durable authority-binding fact rather than another convenience primitive.


## claude

Both amendments landed precisely, and the error-semantics fix is more thorough than what I asked for — I only flagged the HTTP status code distinction, but §12 also correctly extends it to the audit log: *"A duplicate-intent conflict (409) is NOT an authorization denial and must not produce an `ActivityAuthorizationDenied` audit entry."* That's the right generalization, and it's the same discipline that fixed `ReconcileIntent`'s audit ordering several rounds back — a state conflict and a security refusal are different kinds of event, and conflating them in the audit trail would have been misleading in exactly the way that earlier fix was designed to prevent. The composite FK is exactly as specified, with correct rollback (`DROP CONSTRAINT IF EXISTS`), and `TestCS_EXACT_CompositeFKRejectsMismatch`/`RejectsNonexistent` are the right tests to prove it's DB-enforced rather than assumed.

One thing worth naming, not because it's wrong but because it's an unexplained change: acceptance criteria 2 and 3 now resolve wrong-target and wrong-snapshot to `ErrIntentNotLive`, where the original plan specified distinct `ErrIntentTargetMismatch`/`ErrIntentSnapshotMismatch` errors that no longer appear anywhere in this revision. I think this is actually the correct call, not a regression — a single CAS `WHERE` clause matching on `(id, scenario_id, belief_id, action, target_id, snapshot_id)` can only report zero-rows-affected; SQL doesn't tell you which ANDed predicate failed, so distinct error types would have promised a diagnostic precision the mechanism can't actually deliver. Consolidating under `ErrIntentNotLive` — the same error Plan 9.0 already uses for wrong-action and wrong-belief — is honest about what the kernel can actually distinguish. But the document doesn't say this happened, or why; the two error names were in Plan 10.0's tables and are silently absent here. Given how carefully every other decision in this chain has been narrated, I'd want one line added — something like "distinct target/snapshot mismatch errors were considered and dropped: a single CAS predicate cannot report which field caused the non-match, so these fold into the existing `ErrIntentNotLive`" — so a future reader doesn't wonder whether this was an oversight.

With that one line added, this is ready for the fresh adversarial review it schedules for itself in step 19. I don't have anything else to raise.


## chatgpt

Agreed. That is the one documentation gap I would close before the fresh adversarial review.

Add this immediately after the §12 claim-time error-classification discussion:

> **Target/snapshot mismatch error semantics:** Distinct `ErrIntentTargetMismatch` and `ErrIntentSnapshotMismatch` errors were considered and intentionally dropped. `ClaimIntent` uses a single atomic CAS predicate over `(intent_id, scenario_id, belief_id, action, target_id, snapshot_id)`. A zero-row result cannot reliably identify which predicate failed, so target/snapshot mismatches fold into the existing `ErrIntentNotLive` contract, consistent with wrong-action and wrong-belief mismatches. Diagnostic detail may identify the mismatch after the CAS, but must remain advisory and must not affect the security decision.

That makes the evolution from Plan 10.0 to 10.1 explicit rather than leaving the reader to infer why the two sentinel errors disappeared.

With that addition, **Plan 10.1 is ready for the scheduled fresh adversarial review**. The underlying implementation decision remains unchanged: the CAS is the authority decision; any post-failure classification is diagnostic only. 
