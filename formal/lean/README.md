# Solvent Formal Model in Lean 4

This directory contains a Lean 4 machine-checked formal model of Solvent's belief and authority state machine.

> [!IMPORTANT]
> **Scope & Architectural Separation:**
> - **Lean 4:** Formally proves that valid state-machine transitions preserve the core authority invariants (`ValidLedger`), and proves the mathematical impossibility of live authority existing on retracted or non-promoted beliefs.
> - **CockroachDB (D3/D4 Control Experiment):** Empirically proves that unsafe orderings (such as retracting a belief before cancelling dependent live intents) are structurally refused by engine constraints (`23503 · gate`, `23514 · live_requires_promoted`).
> - **Not a Refinement Proof:** This is a formalization of Solvent's abstract state machine and semantics, not a machine-checked refinement proof of the Go runtime or CockroachDB internals.

---

## 1. Formal Model Structure

The formalization is structured into five focused Lean modules under `Solvent/`:

```text
Solvent/
├── Types.lean          -- Core vocabulary: Belief, ActionIntent, Ledger, BeliefStatus, IntentState
├── Invariants.lean     -- The 3 database invariants: promoted_is_debt_free, gate, live_requires_promoted
├── Transitions.lean    -- The 4 state transitions: promote, cancelIntent, authorizeIntent, retractCascade
├── Preservation.lean   -- Preservation theorems, impossibility proofs, and cascade theorems
└── Examples.lean       -- Concrete executable proofs of valid and invalid ledger states
```

---

## 2. Invariants Modeled

The three database-enforced authority invariants correspond directly to the CockroachDB schema constraints:

| Lean Predicate | SQL Invariant | Meaning |
|---|---|---|
| `promoted_is_debt_free` | `promoted_is_debt_free CHECK` | Every promoted belief has empty debt and `finalTruth = false`. |
| `gate` | `gate FOREIGN KEY ... ON UPDATE CASCADE` | For every intent referencing an existing belief, `intent.beliefStatus = belief.status`. |
| `live_requires_promoted` | `live_requires_promoted CHECK` | Every live intent references a belief whose current status is `promoted` and whose `beliefStatus` is `promoted`. |

`ValidLedger l` is defined as the conjunction:
```lean
def ValidLedger (l : Ledger) : Prop :=
  promoted_is_debt_free l ∧ gate l ∧ live_requires_promoted l
```

---

## 3. Formally Proved Theorems

All theorems are proved without `sorry`, `admit`, or unchecked `axiom`s.

### Non-Tautological Transition Preservation
Transitions refuse (return `none`) exclusively on domain preconditions; none inspect `ValidLedger` on output. Lean independently verifies preservation:

- `promote_preserves_validity`: `ValidLedger l → promote l bid = some l' → ValidLedger l'`
- `cancelIntent_preserves_validity`: `ValidLedger l → cancelIntent l iid = some l' → ValidLedger l'`
- `authorizeIntent_preserves_validity`: `ValidLedger l → authorizeIntent l iid = some l' → ValidLedger l'`
- `retractCascade_preserves_validity`: `ValidLedger l → retractCascade l bid = some l' → ValidLedger l'`

### Authority Impossibility (The Solvent Thesis)
- `live_intent_implies_promoted`: Any live intent in a valid ledger must reference a belief whose current status is `promoted`.
- `no_live_intent_on_retracted_belief`: In a valid ledger, no live intent can reference a belief that is `retracted` (proves `False`).

### Layered Promotion and Cascade Invariants
- `promotion_updates_dependent_intent`: Promotion cascades `beliefStatus := promoted` to all dependent intents (`ON UPDATE CASCADE`).
- `cascade_retraction_updates_dependent_intent`: Cascading retraction updates all dependent intents on affected beliefs to `beliefStatus := retracted`.
- `cascade_retraction_cannot_leave_live_intent`: Cascading retraction guarantees no dependent intent on an affected belief remains `live`.
- `cascade_update_preserves_gate`: Status propagation in cascade preserves the composite foreign key invariant (`gate`).
- `cascade_retraction_blocks_live_intent`: Structurally blocks live intents on retracted beliefs.

---

## 4. Build and Verification

### Toolchain Pinning
- **Lean:** `leanprover/lean4:v4.33.0` (pinned in `lean-toolchain`)
- **Mathlib:** commit `db584cd6d46c92f209a44c0f1c829460d327499d` (pinned in `lakefile.lean`)

### Reproducing the Proofs
```bash
cd formal/lean
lake build
```

### Anti-Cheating Verification
Assert zero unproved goals or axioms:
```bash
grep -RniE 'sorry|admit|axiom' Solvent/
```

---

## 5. Abstraction Boundary

The Lean model proves properties of an **abstract state machine**. The following notes identify where the abstraction intentionally diverges from the production Go/CockroachDB implementation.

### 5.1 `authorizeIntent` — abstract update vs production insert

The Lean `authorizeIntent` transition updates an existing intent's `state` to `live` and `beliefStatus` to `promoted`, without checking the intent's current state. The production Go kernel's `IntentOnPromoted` (kernel/kernel.go) performs an `INSERT INTO action_intent`, which creates a new intent row rather than updating an existing one. The composite FK `gate` constraint prevents the insert unless the referenced belief is currently promoted.

The Lean model does not check the intent's current `state` before authorization; the production insert sidesteps this by never encountering an existing row. Both paths preserve the `ValidLedger` invariants, but the Lean transition is broader in what it permits.

### 5.2 `retractCascade` — broader retraction vs promoted-only SQL

The Lean `retractCascade` retracts all beliefs in the affected set regardless of their current `BeliefStatus`. The production SQL (`sqlRetractCascadeRetract` in kernel/sql.go) applies `WHERE status = 'promoted'`, updating only beliefs currently in the `promoted` state.

The Lean abstraction permits a broader retraction than the production SQL behavior; the stated invariants are still preserved. The broader retraction only makes retraction easier (more beliefs retracted → more intents cancelled → fewer live intents on non-promoted beliefs), so the theorems remain sound.

### 5.3 `edges` — list representation vs production uniqueness constraint

The Lean `edges : List (BeliefId × BeliefId × EdgeKind)` does not encode the production `PRIMARY KEY (parent_id, child_id)` uniqueness constraint on `belief_edge`. Duplicate edges may appear in the list. This does not affect the stated invariants because `derivesChildren` uses `filterMap` (preserving duplicates in output) and the invariant predicates use `∈` (set membership, not multiplicity). Duplicate edges may cause `affectedBeliefs` to visit a node more than once, but the `reachableDerives` visited-list check prevents redundant traversal.
