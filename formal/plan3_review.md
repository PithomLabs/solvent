**Not quite ready to implement. I would mark this GREEN WITH 3 NITS, two of them important.** The overall plan is now very close: four transitions, explicit Lean-vs-CockroachDB responsibility, non-tautological proofs, and a small isolated formal project are all right. 

### 1. Important: `gate` is currently specified incorrectly

The plan says:

> `gate`: `(i.beliefId = b.id) → i.beliefStatus = b.status`

But the Lean `Belief` structure has **no `id` field**. The ID is the key in:

```lean
beliefs : BeliefId → Option Belief
```

The invariant should be defined through that lookup, conceptually:

```lean
∀ iid intent b,
  l.intents iid = some intent →
  l.beliefs intent.beliefId = some b →
  intent.beliefStatus = b.status
```

That is actually cleaner and more faithful to the composite FK.

### 2. Important: `promote` must model the FK cascade

This is the bigger semantic issue.

The plan currently says `promote` changes a belief from `entered` to `promoted`, but it does not explicitly say that dependent `ActionIntent.beliefStatus` fields are updated. 

But your formal `gate` says:

```text
intent.beliefStatus = belief.status
```

So a valid ledger could contain a non-live/cancelled intent referencing an `entered` belief:

```text
intent.beliefStatus = entered
belief.status       = entered
```

Then `promote` changes:

```text
belief.status = promoted
```

while leaving:

```text
intent.beliefStatus = entered
```

and **gate immediately becomes false**.

The real SQL schema solves this through the composite FK's `ON UPDATE CASCADE`.

Therefore the Lean `promote` transition must model that propagation too:

```text
promote
  ├─ belief.status := promoted
  └─ cascade every dependent intent.beliefStatus := promoted
```

This is actually a great thing for the formal model to prove, because it directly connects the abstract transition to the database mechanism you're demonstrating empirically.

I would add a local theorem analogous to the retraction one:

```lean
theorem promotion_updates_dependent_intent :
  ...
  → intent.beliefStatus = BeliefStatus.promoted
```

and then let that feed into `gate` preservation.

### 3. Pin the actual Mathlib revision in the implementation, not merely conceptually

The plan says the project uses pinned dependency resolution, but the implementation plan should require the **actual commit hash** to be recorded in `lakefile.lean`, not just "pinned." 

That gives you a genuine reproducibility statement:

```text
Lean 4.33.0
+
Mathlib commit abc123...
```

rather than a lockfile whose provenance is ambiguous.

### One minor verification nit

The current check is:

```bash
grep -rn "sorry" Solvent/
```

I'd broaden the anti-cheating check to:

```bash
grep -RniE 'sorry|admit|axiom' Solvent/
```

You don't want a theorem proved using a new unchecked `axiom` and still call the artifact "machine-checked proofs."

### With those fixes, the plan is excellent

The strongest part is the explicit separation:

```text
Lean
→ proves safe transitions preserve ValidLedger

CockroachDB
→ empirically demonstrates unsafe ordering is refused
```

That is exactly the right framing, and the plan now states it directly. 

The four preservation theorems are also now properly non-tautological because the transitions do not inspect `ValidLedger` as an output guard. 

And the cascade proof is nicely structured as:

```text
status propagation
        ↓
liveness impossibility
        ↓
global invariant
```



### Give the coding agent this final instruction

```text
APPROVE WITH 3 NITS BEFORE IMPLEMENTATION.

1. Fix `gate`:
   Belief has no `id` field. Define gate through the keyed lookup:

   l.intents iid = some intent →
   l.beliefs intent.beliefId = some belief →
   intent.beliefStatus = belief.status

2. Model promotion's FK-style cascade:
   `promote` must update `belief.status := promoted` AND propagate
   `beliefStatus := promoted` to every dependent intent, mirroring
   ON UPDATE CASCADE.

   Add a local theorem proving dependent intent status propagation,
   then use it in `promote_preserves_validity`.

3. Pin the exact Mathlib commit hash in `lakefile.lean`.
   Do not rely on a floating revision.

Also strengthen the verification check from `sorry` to:
   grep -RniE 'sorry|admit|axiom' Solvent/

Everything else in plan3 is approved.
```

**Verdict: do not implement quite yet; make those edits, then GREEN to build.**
