# Lean 4 Formal Model of Solvent's Belief/Authority State Machine — Revised Plan

A Lean 4 + Mathlib formalization of the abstract state machine governing Solvent's belief
promotion, action gating, and cascade retraction. This proves that every successful transition
preserves the three core authority invariants.

> [!IMPORTANT]
> This is a formal model of the **abstract** Solvent semantics. It is **not** a refinement proof
> of the Go/CockroachDB implementation. No Go code or SQL schema will be modified.

## Changes from plan.md

This plan addresses the four findings in [plan_review.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/plan_review.md):

| # | Finding | Resolution |
|---|---------|------------|
| 1 | Transitions must not guard on `ValidLedger` — preservation proofs would be tautological | Each transition refuses only on **domain preconditions** (belief missing, wrong status, debt present, etc.). No transition inspects `ValidLedger` on its candidate output. Lean proves preservation independently. |
| 2 | Cascade proof should model local behavior before proving global invariants | New layered proof structure: first prove `cascade_retraction_updates_dependent_intent` (intent.beliefStatus follows), then `cascade_retraction_cannot_leave_live_intent` (live is impossible), then derive `gate` and `live_requires_promoted` as consequences. |
| 3 | Pin exact Lean + Mathlib revision, not floating `main` | Pin `lean-toolchain` to `leanprover/lean4:v4.33.0`. Pin Mathlib to a specific commit known to build against v4.33.0. Reproducible `lake build` from clone. |
| 4 | Authority impossibility theorems are excellent | Preserved unchanged. |

Additionally, standalone `retract` is restored per [prompt.md §4](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/prompt.md) which explicitly lists five transitions including `retract`.

---

## Design Decisions

| Question | Decision |
|---|---|
| Mathlib dependency | **Yes** — use Mathlib for `Function.update` lemmas; don't expand proof scope beyond what's needed |
| Ledger representation | **Function-based** `BeliefId → Option Belief` — matches keyed DB semantics |
| Standalone `retract` | **Yes** — restored per prompt.md; refuses when a live intent remains attached |
| Graph traversal depth | **Simple recursive descent** over `derives` edges — no cycle detection or SQL CTE reproduction |
| Lean toolchain | **Pinned** — `leanprover/lean4:v4.33.0` + exact Mathlib commit hash |
| Postcondition guards | **Prohibited** — no transition inspects `ValidLedger` on its output |

---

## Proposed Changes

All files live under `formal/lean/`. No production code is touched.

---

### Lake Project Setup

#### [NEW] [lakefile.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/lakefile.lean)

- Lean 4 Lake project named `Solvent`
- Mathlib dependency via `require mathlib from git ...` pinned to a **specific commit hash**
  that builds against `leanprover/lean4:v4.33.0`
- Single library target `Solvent`

#### [NEW] [lean-toolchain](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/lean-toolchain)

- Exact contents: `leanprover/lean4:v4.33.0`
- No floating reference to Mathlib main

**Reproducibility contract:**
```text
git clone → elan toolchain install → lake exe cache get → lake build → same proof
```

#### [NEW] [README.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/README.md)

- Explains this is a formal model, not a refinement proof
- Build instructions: `cd formal/lean && lake exe cache get && lake build`
- Lists the theorems and what they establish
- States the pinned Mathlib revision

---

### Solvent/Types.lean — Core Vocabulary

#### [NEW] [Solvent/Types.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Types.lean)

Types from the prompt, closely mirroring the SQL schema:

```lean
inductive BeliefStatus     -- entered | promoted | retracted
inductive IntentState      -- live | cancelled | executed
inductive EdgeKind         -- derives | contradicts
structure DebtItem         -- { id : Nat }
structure Belief           -- { status, debt, finalTruth }
structure ActionIntent     -- { state, beliefId, beliefStatus }
structure Ledger           -- { beliefs : BeliefId → Option Belief,
                           --   intents : IntentId → Option ActionIntent,
                           --   edges : List (BeliefId × BeliefId × EdgeKind) }
```

Correspondence to SQL:

| Lean type | SQL table/column |
|---|---|
| `BeliefStatus` | `belief.status CHECK (...)` |
| `IntentState` | `action_intent.state CHECK (...)` |
| `Belief.debt` | `belief.debt TEXT[]` |
| `Belief.finalTruth` | `belief.final_truth BOOLEAN` |
| `ActionIntent.beliefStatus` | `action_intent.belief_status` |
| `Ledger.edges` | `belief_edge` table |

---

### Solvent/Invariants.lean — The Three Invariants

#### [NEW] [Solvent/Invariants.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Invariants.lean)

Three invariants corresponding to the named SQL constraints:

| Lean predicate | SQL constraint | Meaning |
|---|---|---|
| `promoted_is_debt_free` | `promoted_is_debt_free CHECK` | Promoted ⇒ empty debt ∧ ¬finalTruth |
| `gate` | `gate FOREIGN KEY ... ON UPDATE CASCADE` | Intent's stored beliefStatus = belief's actual status |
| `live_requires_promoted` | `live_requires_promoted CHECK` | Live intent ⇒ belief is promoted ∧ intent.beliefStatus = promoted |

Combined as:
```lean
def ValidLedger (l : Ledger) : Prop :=
  promoted_is_debt_free l ∧ gate l ∧ live_requires_promoted l
```

Quantification design: each invariant universally quantifies over all possible IDs, with a
hypothesis `l.beliefs bid = some b` (or `l.intents iid = some intent`), so only "existing"
entries are constrained.

---

### Solvent/Transitions.lean — Five Transitions (Non-Tautological Design)

#### [NEW] [Solvent/Transitions.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Transitions.lean)

> [!IMPORTANT]
> **Review finding #1 applied here.** Every transition returns `none` only based on
> **domain/semantic preconditions**. No transition inspects `ValidLedger` on its candidate
> output. The preservation theorems are proved independently by Lean, making them genuinely
> meaningful.

Five transitions (matching [prompt.md §4](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/prompt.md)):

| Function | Go counterpart | Returns `none` when (domain preconditions only) |
|---|---|---|
| `promote` | `Promote` | Belief missing, status ≠ entered, debt non-empty, or finalTruth |
| `retract` | (direct retraction) | Belief missing, or a live intent exists on this belief |
| `cancelIntent` | (implicit in `RetractCascade`) | Intent missing or not live |
| `authorizeIntent` | `IntentOnPromoted` | Intent missing, belief missing, or belief not promoted |
| `retractCascade` | `RetractCascade` | Target belief missing |

**What each transition does NOT check:**

- `promote` does **not** check `ValidLedger` on the output — it checks `status = entered ∧ debt = [] ∧ finalTruth = false`
- `retract` does **not** check `ValidLedger` — it checks whether any live intent references the belief and refuses if so
- `retractCascade` does **not** check `ValidLedger` — it mechanically retracts descendants, cancels affected live intents, and updates their `beliefStatus`
- `authorizeIntent` does **not** check `ValidLedger` — it checks belief exists and is promoted
- `cancelIntent` does **not** check `ValidLedger` — it checks the intent is live

**`retract` semantics (restored per prompt.md):**

A direct retraction of a single belief. Refuses (returns `none`) when any live intent references
this belief — it does not silently create an invalid ledger. Does not traverse descendant edges.

**`retractCascade` semantics:**

1. Find the target belief (return `none` if missing)
2. Compute transitive `derives` descendants via simple recursive list traversal
3. Cancel live intents on the target + all descendants (state → cancelled)
4. Update each affected intent's `beliefStatus` to `retracted`
5. Set target + descendant belief statuses to `retracted`
6. Return `some l'`

Steps 3-4 happen before step 5, matching the Go kernel's ordering constraint (I-8):
cancel first, then retract.

Helper function `descendants : Ledger → BeliefId → List BeliefId` performs a bounded
recursive walk over the `derives` edges in `Ledger.edges`.

---

### Solvent/Preservation.lean — Preservation Theorems

#### [NEW] [Solvent/Preservation.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Preservation.lean)

#### Part 1: Preservation (non-tautological)

Five preservation theorems — one per transition:

```lean
theorem promote_preserves_validity :
  ∀ l bid l', ValidLedger l → promote l bid = some l' → ValidLedger l'

theorem retract_preserves_validity :
  ∀ l bid l', ValidLedger l → retract l bid = some l' → ValidLedger l'

theorem cancelIntent_preserves_validity :
  ∀ l iid l', ValidLedger l → cancelIntent l iid = some l' → ValidLedger l'

theorem authorizeIntent_preserves_validity :
  ∀ l iid l', ValidLedger l → authorizeIntent l iid = some l' → ValidLedger l'

theorem retractCascade_preserves_validity :
  ∀ l bid l', ValidLedger l → retractCascade l bid = some l' → ValidLedger l'
```

Each has the form:
```lean
∀ l x l', ValidLedger l → transition l x = some l' → ValidLedger l'
```

> [!IMPORTANT]
> Because transitions refuse on domain preconditions — not on `ValidLedger` — these proofs are
> **non-tautological**. Lean must independently verify that the mechanical state updates
> preserve all three sub-invariants. This is the key correction from plan.md.

**Proof strategy:** Each proof unfolds the transition definition, matches on the `if`/`match`
branches that produced `some l'`, and then establishes each of the three sub-invariants for `l'`
by case analysis on whether each ID is the modified one or an untouched one. The function-based
`Ledger` representation means "untouched entries" are handled via `Function.update` at a
different key preserving the original value (standard Mathlib pattern).

---

#### Part 2: Authority Impossibility Theorems

Two headline theorems establishing Solvent's core claim:

```lean
theorem live_intent_implies_promoted :
  ValidLedger l →
  l.intents iid = some intent →
  intent.state = IntentState.live →
  ∃ belief,
    l.beliefs intent.beliefId = some belief ∧
    belief.status = BeliefStatus.promoted

theorem no_live_intent_on_retracted_belief :
  ValidLedger l →
  l.intents iid = some intent →
  intent.state = IntentState.live →
  l.beliefs intent.beliefId = some belief →
  belief.status = BeliefStatus.retracted →
  False
```

**Proof strategy:** These follow directly from `live_requires_promoted` + `gate` in
`ValidLedger`. The second is a corollary: if the intent is live, the belief must be promoted
(by the first theorem), but promoted ≠ retracted.

---

#### Part 3: Cascade / Gate Theorems (Layered Local-to-Global)

> [!IMPORTANT]
> **Review finding #2 applied here.** Instead of jumping to global invariant preservation, the
> proof proceeds in three layers: local status propagation → local liveness impossibility →
> global invariants as consequences. This mirrors the D3/D4 experiment.

**Layer 1 — Local status propagation:**

```lean
theorem cascade_retraction_updates_dependent_intent :
  ValidLedger l →
  retractCascade l bid = some l' →
  l'.intents iid = some intent →
  intent.beliefId = bid' →
  bid' ∈ affectedBeliefs l bid →
  intent.beliefStatus = BeliefStatus.retracted
```

*Establishes: when a belief is retracted by cascade, every intent referencing it has its
`beliefStatus` updated to `retracted`.*

**Layer 2 — Local liveness impossibility:**

```lean
theorem cascade_retraction_cannot_leave_live_intent :
  ValidLedger l →
  retractCascade l bid = some l' →
  l'.intents iid = some intent →
  intent.beliefId = bid' →
  bid' ∈ affectedBeliefs l bid →
  intent.state ≠ IntentState.live
```

*Establishes: no live intent can survive on a retracted belief after cascade.*

**Layer 3 — Global invariants as consequences:**

```lean
theorem cascade_update_preserves_gate :
  ValidLedger l →
  retractCascade l bid = some l' →
  gate l'

theorem cascade_retraction_blocks_live_intent :
  ValidLedger l →
  retractCascade l bid = some l' →
  l'.beliefs bid' = some b' →
  b'.status = BeliefStatus.retracted →
  l'.intents iid = some intent →
  intent.beliefId = bid' →
  intent.state ≠ IntentState.live
```

The mathematical narrative mirrors the actual system:
```text
belief status changes (retracted)
      ↓
FK-style status propagation (intent.beliefStatus follows)
      ↓
live intent becomes incompatible (cancelled by cascade)
      ↓
authority invariant forces refusal (live + retracted = impossible)
```

---

### Solvent/Examples.lean — Worked Examples

#### [NEW] [Solvent/Examples.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Examples.lean)

Small `#check` / `example` / `#eval` demonstrations:

| Example | Purpose |
|---|---|
| Empty ledger | `ValidLedger emptyLedger` — the trivial valid state |
| Entered belief with debt | Shows a non-promoted belief carrying debt is valid |
| Promoted debt-free belief | Shows a clean promoted belief is valid |
| Live intent on promoted belief | The happy path |
| Invalid: live intent on entered belief | Demonstrates `live_requires_promoted` catches it |
| Invalid: live intent on retracted belief | Demonstrates the impossibility theorem |

---

### Root README Addition

#### [MODIFY] [README.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/README.md)

Add a small section (not a rewrite):

```markdown
## Formal model

`formal/lean/` contains a Lean 4 formal model of the belief/authority
state machine and proofs that the core invariants are preserved by
successful state transitions.

This is a formalization of the abstract Solvent semantics, not a
machine-checked refinement proof of the Go/CockroachDB implementation.
```

---

## File Summary

| File | Purpose |
|---|---|
| `formal/lean/lakefile.lean` | Lake project config with Mathlib (pinned commit) |
| `formal/lean/lean-toolchain` | Lean version pin: `leanprover/lean4:v4.33.0` |
| `formal/lean/README.md` | What this artifact proves (and doesn't) |
| `formal/lean/Solvent/Types.lean` | Core vocabulary: BeliefStatus, Ledger, etc. |
| `formal/lean/Solvent/Invariants.lean` | Three invariants + ValidLedger |
| `formal/lean/Solvent/Transitions.lean` | promote, retract, cancelIntent, authorizeIntent, retractCascade |
| `formal/lean/Solvent/Preservation.lean` | All preservation + authority + cascade theorems |
| `formal/lean/Solvent/Examples.lean` | Worked examples demonstrating definitions |
| `README.md` (root) | Small section added |

**Total: 8 new files, 1 minor modification.**

---

## Verification Plan

### Automated Tests

```bash
# Lean formal model compiles and all proofs check
cd formal/lean && lake exe cache get && lake build

# Confirm no sorry
grep -r "sorry" formal/lean/Solvent/ && echo "FAIL: sorry found" || echo "PASS: no sorry"

# Go code is untouched
go build ./...
go vet ./...
task test
```

### Manual Verification

- Confirm `lake build` exits 0 with no `sorry` in any file
- Confirm every theorem name listed above appears in the build output
- Confirm no Go or SQL files were modified (beyond the README section)
- Confirm the `lean-toolchain` contains exactly `leanprover/lean4:v4.33.0`
- Confirm `lakefile.lean` pins Mathlib to a specific commit, not `main`

---

## Risk Assessment

| Risk | Mitigation |
|---|---|
| Mathlib commit incompatible with v4.33.0 | Resolve the exact commit from Mathlib's history that last used this toolchain; test with `lake build` before proceeding |
| `retractCascade` non-tautological proof is harder | The cascade mechanically cancels then retracts; the proof unfolds this sequence. No shortcut via ValidLedger postcondition |
| Standalone `retract` proof must show live-intent check is sufficient | `retract` refuses when any live intent exists on the belief; with that precondition discharged, the three invariants hold for the same `Function.update` reasons as `promote` |
| Build time | Mathlib cache via `lake exe cache get`; first build ~5-15 min, subsequent builds seconds |
| Function-based ledger makes universal quantification proofs wordy | Use `Function.update` lemmas from Mathlib; proofs stay mechanical even if verbose |

---

## Open Questions

> [!NOTE]
> No open questions. All design decisions resolved, all review findings addressed.
