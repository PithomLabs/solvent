I reviewed Plan 10.0 carefully. The core decision is right, and the Kernel Growth Gate is justified. The plan is close to implementation-ready, but I would make **four corrections before coding**.

### 1. The DB must enforce `(target_id, snapshot_id)` together

This is the most important issue.

The plan currently proposes:

```sql
target_id   UUID REFERENCES authority_target(target_id),
snapshot_id UUID
```

But the plan's security invariant is:

```text
intent.target_id == target_id
AND
intent.snapshot_id == snapshot_id
AND
(target_id, snapshot_id) is a real approved activation
```

A plain `snapshot_id` column does **not** make that relationship durable. The `target_id` FK only proves that the target exists.

Your existing schema already has the stronger relationship:

```text
target_activation(target_id, snapshot_id)
    -> target_snapshot(target_id, snapshot_id)
```

So `action_intent` should have a composite FK, or equivalent DB-enforced relationship, tying the pair to the approved snapshot.

Conceptually:

```sql
FOREIGN KEY (target_id, snapshot_id)
REFERENCES target_snapshot(target_id, snapshot_id)
```

or, preferably if the activation table is the authoritative approval relationship, a composite FK through the structure that actually proves activation.

That matters because Plan 10 correctly argues that this is a **durable security invariant**, not merely application validation. 

Without the composite relationship, the implementation would be slightly weaker than the ADR claims.

---

### 2. `ErrIntentTargetMismatch` / `ErrIntentSnapshotMismatch` need an explicit mechanism

The proposed SQL is essentially:

```text
UPDATE action_intent
SET state='executing'
WHERE ...
  AND target_id = ?
  AND snapshot_id = ?
  AND state='live'
```

A zero-row result does not inherently tell you *why* it failed.

It could be:

```text
intent does not exist
wrong scenario
wrong belief
wrong action
wrong target
wrong snapshot
not live
```

Yet the acceptance criteria require specifically:

```text
ErrIntentTargetMismatch
ErrIntentSnapshotMismatch
```



That means the implementation plan needs to specify how those errors are derived.

I would **not** weaken the atomic CAS. Keep the atomic claim as the authority decision, then, only when the CAS affects zero rows, perform a diagnostic read sufficient to classify the mismatch.

For example:

```text
CAS succeeds
    → executing

CAS fails
    → read immutable intent identity
    → classify mismatch
```

The diagnostic read must never turn into a separate authorization decision. The CAS remains authoritative.

---

### 3. The migration's nullable columns are defensible, but the plan's uniqueness claim is misleading

This:

```sql
CREATE UNIQUE INDEX
  ON action_intent (scenario_id, belief_id, action, target_id, snapshot_id)
  WHERE state = 'live';
```

does **not** guarantee uniqueness for legacy/unbound rows because SQL `NULL` semantics allow multiple rows containing NULLs.

Your plan explicitly relies on nullable columns for migration compatibility:

> "Existing intents without `target_id`/`snapshot_id` will have NULL values." 

That is fine, but then the comment:

```text
Prevent duplicate intents for the same authority.
```

should be narrowed to:

```text
Prevent duplicate bound live intents for the same authority.
```

More importantly, decide explicitly whether this unique index is actually needed. **Exact authority binding does not require uniqueness of live intents.** It is an additional behavioral constraint.

I would avoid introducing unrelated semantics during the kernel-freeze change unless an existing invariant already requires one-live-intent-per-authority.

---

### 4. The "Wrong snapshot" test currently depends on something the schema says cannot happen

The plan says:

> "Target T1 somehow has a different snapshot (impossible due to UNIQUE(target_id), but test defensively)" 

That is useful conceptually, but the executable test needs to be more concrete.

There are really two distinct tests:

**A. Cross-authority substitution — the important real-world case**

```text
I1 = T1/S1
execute I1 against T2/S2
→ DENY
```

This is the actual confused-deputy exploit.

**B. Corrupted/inconsistent binding — database-defense test**

Construct an intent whose stored pair is deliberately inconsistent, or use a direct kernel/database fixture where possible:

```text
I1.target_id   = T1
I1.snapshot_id = S2
```

Then verify the DB relationship itself prevents the invalid state.

That second test is valuable precisely because your thesis is that the **database is the authority**, not merely Go code.

---

## One architectural point I would change

The plan says:

> "The authority identity is `(target_id, snapshot_id)`." 

I would phrase this slightly differently:

```text
Authority instance = approved target activation
Authority identity = (target_id, snapshot_id)
```

Because the actual security object is not merely two UUIDs. It is the **approved activation of a target pointing to an immutable snapshot**.

That distinction will make the later code and documentation cleaner:

```text
target
   ↓ approval
activation
   ↓
snapshot
```

and:

```text
intent ──bound to──> activation (target_id, snapshot_id)
```

This also avoids accidentally making `snapshot_id` look like an independently meaningful authority object.

---

## The overall implementation sequence is sound

The important sequence is correctly captured:

```text
Authorize
   ↓
obtain snapshot_id
   ↓
create intent(target_id, snapshot_id)
   ↓
later Authorize again
   ↓
atomic ClaimIntent(... target_id, snapshot_id)
   ↓
execute snapshot parameters
```



That closes the exact hole demonstrated in the plan's T1 → revoke → T2 substitution scenario. 

And importantly, this really does look like the **last planned kernel expansion**. The resulting kernel primitive is not another abstraction layer; it is making a single durable fact true:

> An execution intent cannot detach itself from the authority instance that created it.

Everything else in the current roadmap can remain above the kernel.

### My disposition

**Plan 10.0: APPROVE WITH CHANGES**

Before handing it to the coding agent, I would require these four edits:

1. **Composite DB enforcement** of the `(target_id, snapshot_id)` authority relationship.
2. Explicit **zero-row CAS error-classification semantics** for the new sentinel errors.
3. Remove or qualify the uniqueness claim around nullable legacy intents; preferably avoid unrelated uniqueness behavior unless justified.
4. Split the wrong-snapshot verification into a real confused-deputy test plus a DB-consistency test.

After those changes, the plan is strong enough to implement. The final acceptance criterion should remain exactly what you are aiming for: **the authority that was approved is the authority that gets executed, and the database makes that relationship durable.**


## claude

This is genuinely the deepest piece of security work in this whole chain — the confused-deputy scenario in §2 (intent created against T1's authority, executed against a *different*, later-approved T2 whose tuple happens to match) is a real gap nothing earlier in this thread caught, and finding it before it shipped rather than after is exactly the value this whole review process is supposed to produce. The Kernel Growth Gate treatment in §5 is done properly — three options honestly compared, and "GATE: PASS" is earned by the actual argument (this binding must survive restarts, retries, and concurrent access; only the DB can enforce that atomically), not asserted. The NULL-fails-closed migration reasoning is exactly right, and I want to note that specifically since it's the kind of design choice that's easy to get backwards: making the new columns nullable for backward compatibility, then relying on SQL's three-valued NULL logic to make pre-migration intents permanently unclaimable rather than trying to backfill them with fabricated values, is the correct call.

One finding, and I think it's a real gap in the migration as written:

## `action_intent.snapshot_id` has no foreign key, and it should have the same composite FK `target_activation` already uses

The migration adds:

```sql
ADD COLUMN target_id   UUID REFERENCES authority_target(target_id),
ADD COLUMN snapshot_id UUID;
```

`target_id` gets a real FK. `snapshot_id` doesn't — it's a bare, unconstrained UUID. But §6 explicitly justifies storing `snapshot_id` at all as "defense-in-depth — if the activation FK is ever compromised, the snapshot_id check still holds," and cites `target_activation`'s existing `FOREIGN KEY (target_id, snapshot_id) → target_snapshot(target_id, snapshot_id)` as the proven pattern for exactly this kind of binding. Without the same composite FK on `action_intent`, that defense-in-depth claim isn't actually true at the schema level — nothing stops an `action_intent` row from being inserted with a `target_id` and `snapshot_id` that don't correspond to each other, or with a `snapshot_id` that doesn't correspond to any real snapshot at all. In the intended code path this can't happen, because both values are read together from the same `authorizeWithinTx` query — but that's application discipline, and the entire premise of this project, restated in nearly every review in this chain, is that application discipline is not the thing that should be carrying a security invariant when the database can carry it instead. The fix is a one-line addition to the same migration:

```sql
ALTER TABLE action_intent
  ADD CONSTRAINT intent_authority_snapshot_fk
  FOREIGN KEY (target_id, snapshot_id) REFERENCES target_snapshot(target_id, snapshot_id);
```

This makes the exact thing §6 claims — "if the same target is somehow re-approved... the snapshot_id would differ" — structurally checkable rather than merely true by construction of the current code, which is the same standard applied everywhere else in this schema (`gate`, `live_requires_promoted`, `target_activation`'s own FK). I'd add this to §11 and to the "Files Expected to Change" migration entry before implementation starts.

One smaller thing worth confirming rather than asserting as a problem: the new `intent_authority_unique` index rejects a second live intent for the identical `(scenario, belief, action, target, snapshot)` tuple. If a client ever legitimately retries `AuthorizeAndCreateIntent` after a timeout where the first call actually succeeded server-side, this constraint would now produce a uniqueness violation rather than an idempotent "return the existing intent" response. Given idempotency-key handling has been explicitly deferred elsewhere in this project's history rather than solved, I don't think this needs solving now — but the error semantics table (§12) should say explicitly what a caller gets back in that case (a clear "an equivalent intent already exists" error, ideally referencing the existing intent id, rather than a raw constraint-violation surfaced up the stack), so it doesn't get discovered as an unhandled case later.

Everything else — the caller inventory correctly identifying `CompleteIntent`/`RollbackClaim`/`CancelIntent` as not needing changes since they operate on already-claimed intents, the field-lineage table in §9 being genuinely exhaustive, and the kernel-freeze rationale in §19 tying this specific fix to "no further kernel growth planned unless a future review identifies a genuinely new atomic invariant" — is sound and consistent with how every other kernel change in this project has been justified. Add the composite FK, and this is ready to implement.


## chatgpt

Agreed. That is the correct final disposition.

The only substantive migration correction I would carry forward is the composite FK:

```sql
ALTER TABLE action_intent
  ADD CONSTRAINT intent_authority_snapshot_fk
  FOREIGN KEY (target_id, snapshot_id)
  REFERENCES target_snapshot(target_id, snapshot_id);
```

That brings `action_intent` up to the same database-enforced standard as the existing authority relationships and makes the claimed exact-binding invariant structural rather than application-dependent.

The retry/uniqueness observation is also worth recording, but **not solving as part of this kernel change**. The plan should explicitly document that `intent_authority_unique` is a uniqueness constraint, not an idempotency mechanism, and that an equivalent live intent created by a retry may surface as a defined conflict rather than being silently created twice.

So I would amend Plan 10.0 in exactly two places:

```text
§11 Schema / DB Design
→ add the composite FK to db/009_exact_authority_binding.sql
→ state that it guarantees target_id/snapshot_id correspondence

§12 Error Semantics
→ define the duplicate-equivalent-intent case explicitly
→ do not introduce idempotency-key semantics
```

With those edits, the **Kernel Growth Gate remains PASS**, the implementation sequence remains valid, and the final **kernel freeze** remains justified. 

The key invariant is now properly expressed at every layer:

```text
approved target
      │
      ▼
target_activation
      │
      ▼
(target_id, snapshot_id)
      │
      ▼
action_intent
      │
      ▼
atomic ClaimIntent
      │
      ▼
external execution
```

That is exactly the kind of invariant Solvent's kernel should own—and, after this change, there should be a very high bar for adding anything else to it.


## claude

Agreed on both amendments as scoped. One precision worth adding to the §12 wording before it's written, since it's the kind of distinction that's easy to lose in a single sentence: the error returned in the duplicate-equivalent-intent case should make clear it's a **conflict, not a denial** — the caller isn't unauthorized, an intent for that exact authority already exists. That distinction matters for whatever's consuming the error downstream (REST status code, MCP error shape): a 409-style "already exists" reads and should be handled very differently from a 403-style "not allowed," and conflating them would make a legitimate retry look like an authorization failure to anything logging or alerting on the response. Small wording point, but worth pinning down now rather than leaving the response shape to be improvised when someone actually hits it.

Otherwise this is right as written, and the closing diagram is a good one to keep attached to the plan going forward — it states the invariant at exactly the level of abstraction that makes "does this new thing belong in the kernel" answerable by inspection: does it sit somewhere on that chain, or does it belong to one of the layers hanging off it. That's a cleaner test than re-deriving the growth-gate argument from scratch each time, and it earns the "very high bar" you're setting after this — the case for extending the kernel just got made carefully, three times over, for a real structural gap. The next one should have to clear the same bar, not a lower one because the door's already open.