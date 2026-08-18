# Lean 4 Formal Model of Solvent's Belief/Authority State Machine

A Lean 4 + Mathlib formalization of the abstract state machine governing Solvent's belief promotion, action gating, and cascade retraction. This proves that every successful transition preserves the three core authority invariants.

> [!IMPORTANT]
> This is a formal model of the **abstract** Solvent semantics. It is **not** a refinement proof of the Go/CockroachDB implementation. No Go code or SQL schema will be modified.

## Agreed Design Decisions

| Question | Decision |
|---|---|
| Mathlib dependency | **Yes** — use Mathlib, but don't expand proof scope just because it's available |
| Ledger representation | **Function-based** `BeliefId → Option Belief` — matches keyed DB semantics |
| Standalone `retract` | **No** — only `retractCascade`, matching the actual Go kernel API |
| Graph traversal depth | **Simple recursive descent** over `derives` edges — no cycle detection or SQL CTE reproduction |
| Lean toolchain | **Already installed** — Lean 4.33.0, Lake 5.0.0 via elan |

## Proposed Changes

All files live under `formal/lean/`. No production code is touched.

---

### Lake Project Setup

#### [NEW] [lakefile.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/lakefile.lean)

- Lean 4 Lake project named `Solvent`
- Mathlib dependency via `require mathlib from git ...`
- Single library target `Solvent`

#### [NEW] [lean-toolchain](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/lean-toolchain)

- Pin to `leanprover/lean4:v4.xx.0` (matching the Mathlib-compatible toolchain — will resolve exact version from current Mathlib main)

#### [NEW] [README.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/README.md)

- Explains this is a formal model, not a refinement proof
- Build instructions: `cd formal/lean && lake build`
- Lists the theorems and what they establish

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

The key design for quantification over the function-based ledger: each invariant universally quantifies over all possible IDs, but the hypothesis includes `l.beliefs bid = some b` (or `l.intents iid = some intent`), so only "existing" entries are constrained.

---

### Solvent/Transitions.lean — Four Transitions

#### [NEW] [Solvent/Transitions.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Transitions.lean)

Four transitions (matching the Go kernel API, no standalone `retract`):

| Function | Go counterpart | Returns `none` when |
|---|---|---|
| `promote` | `Promote` | Belief missing, not entered, has debt, or finalTruth |
| `cancelIntent` | (implicit in `RetractCascade`) | Intent missing or not live |
| `authorizeIntent` | `IntentOnPromoted` | Intent missing, belief missing, belief not promoted |
| `retractCascade` | `RetractCascade` | Target missing, or result ledger would be invalid |

**`retractCascade` semantics:**

1. Compute transitive `derives` descendants via simple recursive list traversal
2. Retract those beliefs (status → retracted)
3. Cancel live intents on those beliefs
4. Update each affected intent's `beliefStatus` to `retracted`
5. Return `some l'` only when the resulting ledger is valid

Helper function `descendants : Ledger → BeliefId → List BeliefId` performs a bounded recursive walk over the `derives` edges in `Ledger.edges`.

---

### Solvent/Preservation.lean — Preservation Theorems

#### [NEW] [Solvent/Preservation.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Preservation.lean)

Four preservation theorems — one per transition:

```
theorem promote_preserves_validity
theorem cancelIntent_preserves_validity
theorem authorizeIntent_preserves_validity
theorem retractCascade_preserves_validity
```

Each has the form:
```lean
∀ l x l', ValidLedger l → transition l x = some l' → ValidLedger l'
```

**Proof strategy:** Each transition is small and deterministic. Each proof unfolds the transition definition, matches on the `if`/`match` branches that produced `some l'`, and then establishes each of the three sub-invariants for `l'` by case analysis on whether each ID is the modified one or an untouched one.

The function-based `Ledger` representation means "untouched entries" are handled by showing that the update function (`Function.update`) at a different key preserves the original value. This is a standard Lean/Mathlib pattern.

---

### Solvent/Preservation.lean (continued) — Authority Impossibility Theorems

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

**Proof strategy:** These follow directly from `live_requires_promoted` + `gate` in `ValidLedger`. The second one is a corollary: if the intent is live, the belief must be promoted (by the first theorem), but promoted ≠ retracted.

---

### Solvent/Preservation.lean (continued) — Cascade / Gate Theorems

```lean
-- Status cascade preserves the gate invariant
theorem cascade_update_preserves_gate :
  ValidLedger l →
  cascadeStatusUpdate l bid newStatus = some l' →
  gate l'

-- Cascade retraction blocks live intents
theorem cascade_retraction_blocks_live_intent :
  ValidLedger l →
  retractCascade l bid = some l' →
  l'.beliefs bid' = some b' →
  b'.status = BeliefStatus.retracted →
  l'.intents iid = some intent →
  intent.beliefId = bid' →
  intent.state ≠ IntentState.live
```

The cascade theorems establish the chain:
```
belief status changes → intent beliefStatus follows → live + retracted = impossible
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
| `formal/lean/lakefile.lean` | Lake project config with Mathlib |
| `formal/lean/lean-toolchain` | Lean version pin |
| `formal/lean/README.md` | What this artifact proves (and doesn't) |
| `formal/lean/Solvent/Types.lean` | Core vocabulary: BeliefStatus, Ledger, etc. |
| `formal/lean/Solvent/Invariants.lean` | Three invariants + ValidLedger |
| `formal/lean/Solvent/Transitions.lean` | promote, cancelIntent, authorizeIntent, retractCascade |
| `formal/lean/Solvent/Preservation.lean` | All preservation + authority + cascade theorems |
| `formal/lean/Solvent/Examples.lean` | Worked examples demonstrating definitions |
| `README.md` (root) | Small section added |

**Total: 8 new files, 1 minor modification.**

## Verification Plan

### Automated Tests

```bash
# Lean formal model compiles and all proofs check
cd formal/lean && lake build

# Go code is untouched
go build ./...
go vet ./...
task test
```

### Manual Verification

- Confirm `lake build` exits 0 with no `sorry` in any file
- Confirm every theorem name listed above appears in the build output
- Confirm no Go or SQL files were modified (beyond the README section)
- Grep for `sorry` — must find zero instances

## Risk Assessment

| Risk | Mitigation |
|---|---|
| Mathlib version incompatibility with Lean 4.33.0 | Pin lean-toolchain to match Mathlib's expected version; may need to adjust to Mathlib's current toolchain |
| `retractCascade` proof complexity | Keep the descendant computation simple (finite list, no cycles); the proof doesn't need full graph theory |
| Build time | Mathlib cache via `lake exe cache get` (pre-built oleans); first build ~5-15 min, subsequent builds seconds |
| Function-based ledger makes universal quantification proofs wordy | Use `Function.update` lemmas from Mathlib; proofs stay mechanical even if verbose |

## Open Questions

> [!NOTE]
> No remaining open questions — all five design decisions have been resolved. Ready to proceed.
