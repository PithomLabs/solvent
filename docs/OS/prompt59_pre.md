## prompt

we need to fix Solvent to address the hardcoded debt vocabulary

## answer

Agreed. This is a legitimate **Solvent kernel growth-gate** case because the current hardcoded debt vocabulary directly prevents Solvent from serving as the domain-agnostic epistemic ledger we now require.

The good news is that the fix can be extremely small.

## The correct change

The problem is not that Solvent uses `TEXT[]`. That is already appropriately generic.

The problem is that Solvent currently gives that field a **physics/deployment-specific default vocabulary**:

```sql
debt TEXT[] NOT NULL DEFAULT ARRAY[
  'needProvenanceCheck',
  'needContradictionSweep',
  'needBlastRadius',
  'needRollbackPlan',
  'needVersionPin',
  'needOperatorSignoff'
]
```

That is exactly what should leave the kernel. 

### Change it to

```sql
debt TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[]
```

or the equivalent empty-array default supported by the current CockroachDB/Postgres-compatible implementation.

Then the invariant becomes:

> **Solvent treats debt as opaque obligation identifiers. Solvent never defines what a debt identifier means.**

This is the key.

---

# What moves where

### Solvent

Knows only:

```text
belief.debt = list of opaque debt IDs
```

and enforces:

```text
promoted ⇒ debt is empty
```

It can:

```text
add debt
retire debt
check debt-free
```

but it does **not** know:

```text
needMap
needInvariant
needToyCheck
needNullModel
needObstruction
needFaithfulnessReview
```

Those are EBP/application vocabulary.

### EBP / Coordinator

Defines:

```text
needMap
needInvariant
needToyCheck
needNullModel
needObstruction
needFaithfulnessReview
```

for the BM-IST research application.

Another domain can define:

```text
needReplication
needSampleValidation
needIndependentMeasurement
```

without touching Solvent.

That is exactly the domain-neutrality we were aiming for.

---

# One important refinement

I would **not** introduce a `debt_type` table into Solvent.

That would merely move the vocabulary into another Solvent-owned schema and recreate the same coupling.

Keep:

```text
belief.debt TEXT[]
```

as the minimal generic mechanism.

The semantic registry belongs outside the kernel.

---

# EBP 2.1 then maps cleanly

For BM-IST:

```text
EBP debt item
      ↓
opaque string
      ↓
Solvent belief.debt[]
```

For example:

```json
{
  "type": "belief",
  "claim": "Fisher rigidity holds within H₀",
  "debt": [
    "needNullModel",
    "needObstruction",
    "needFaithfulnessReview"
  ]
}
```

Solvent does not need to know what those strings mean.

It only knows:

```text
len(debt) > 0
```

therefore:

```text
promotion blocked
```

and after the appropriate controlled operations:

```text
len(debt) == 0
```

promotion may proceed.

That preserves the existing database-enforced gate. 

---

# We should also fix `EnterBelief`

The existing contract currently describes `EnterBelief` as automatically inserting the full starting debt. 

That is the second place the hardcoded vocabulary leaks into the kernel.

Change the conceptual contract from:

```text
EnterBelief(...)
→ automatically attach Solvent's debt vocabulary
```

to:

```text
EnterBelief(..., debt []string, ...)
→ persist caller-supplied opaque debt identifiers
```

The coordinator/EBP layer supplies the initial debt set.

For BM-IST:

```text
Capture
  ↓
EBP default debt set
  ↓
Coordinator
  ↓
Solvent.EnterBelief(..., debt)
```

Solvent simply records it.

---

# Retain `RetireDebt` almost unchanged

This is one reason the current design is good.

The existing operation already treats the debt item as a string and removes it from the array. 

That means the generic mechanism already exists.

We mostly need to remove the kernel's semantic ownership of the vocabulary.

---

# The kernel invariant becomes cleaner

I would freeze this:

```text
DEBT IS OPAQUE TO SOLVENT.
```

Solvent may know:

```text debt exists
debt identifier exists
debt was removed
debt array is empty
```

Solvent must not know:

```text why the debt exists
what evidence satisfies it
what the debt means
whether the debt is appropriate for the domain
```

Those belong to EBP/coordinator/human workflow.

---

# This also improves our packet design

The packet no longer needs to translate:

```text
needObstruction
→ Solvent-specific debt vocabulary
```

It simply passes the EBP identifier through.

```text
Agent
  ↓
EBP Research Packet
  ↓
Coordinator validates that `needObstruction`
is valid for the active EBP grammar
  ↓
Solvent stores opaque string
```

That is much cleaner.

---

# What must remain frozen in Solvent

I would **not** change:

```text
belief
belief_edge
evidence
action_intent
promotion gate
retraction cascade
authorization semantics
```

The existing schema already gives us the generic machinery we need. The only kernel growth is:

```text
remove hardcoded debt vocabulary
make debt caller-supplied/opaque
```

This is a very small change relative to the architectural benefit.

---

# Required tests

I would make the Solvent change pass these tests before touching the coordinator:

```text
1. Create belief with arbitrary debt:
   ["needNullModel", "foo", "bar"]
   → accepted.

2. Create belief with empty debt:
   []
   → accepted.

3. Retire arbitrary debt:
   "foo"
   → removed.

4. Unknown debt string:
   → accepted by Solvent.

5. Promotion with any remaining debt:
   → database rejects.

6. Promotion with empty debt:
   → existing promotion gate behavior unchanged.

7. Existing action_intent gate:
   → unchanged.

8. Existing retraction cascade:
   → unchanged.

9. Existing refusal logging:
   → unchanged.

10. Existing frozen behavior outside debt vocabulary:
   → regression-free.
```

The key test is #4:

> **Solvent must accept a debt identifier it has never heard of.**

That proves the kernel is actually domain-agnostic.

---

## The resulting boundary

This is the architecture I would now lock:

```text
                 EBP 2.1
        defines research vocabulary
                 │
                 ▼
       ┌───────────────────┐
       │ Go Coordinator    │
       │                   │
       │ validates meaning │
       │ maps workflow     │
       └─────────┬─────────┘
                 │
        opaque debt IDs
                 │
                 ▼
       ┌───────────────────┐
       │      SOLVENT      │
       │                   │
       │ stores debt       │
       │ gates promotion   │
       │ knows no meaning  │
       └───────────────────┘
```

This is a **much better Solvent**.

It preserves the existing database design rather than expanding it, removes the inappropriate domain vocabulary from the kernel, and makes EBP 2.1 the proper owner of debt semantics.

Most importantly, this is exactly the kind of change that satisfies the kernel growth gate: **a demonstrated architectural need, solved with the smallest possible kernel change.**
