# Lean 4 Formal Model of Solvent's State Machine — Revised Plan (plan4.md)

This plan specifies the implementation of the Lean 4 formal model and proofs for Solvent's belief and authority state machine, incorporating all review findings from [plan3_review.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/plan3_review.md), [plan2_review.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/plan2_review.md), and [AGENTS.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/AGENTS.md).

The artifact provides machine-checked proofs that the core authority invariants are preserved by valid state transitions in the abstract state machine, complementing CockroachDB's empirical engine-enforced refusal proofs.

---

## Review Findings Addressed in plan4.md

| # | Finding from plan3_review.md | Resolution in plan4.md |
|---|-----------------------------|------------------------|
| 1 | `gate` definition used `b.id`, but `Belief` has no `id` field | `gate` is defined via keyed lookup: `l.intents iid = some intent → l.beliefs intent.beliefId = some b → intent.beliefStatus = b.status`. |
| 2 | `promote` must model composite FK `ON UPDATE CASCADE` | `promote` updates `belief.status := promoted` and cascades `intent.beliefStatus := promoted` to all intents referencing `bid`. Added local theorem `promotion_updates_dependent_intent`. |
| 3 | Mathlib commit hash must be explicitly pinned | `lakefile.lean` pins Mathlib to commit `db584cd6d46c92f209a44c0f1c829460d327499d` matching `leanprover/lean4:v4.33.0`. |
| 4 | Verification check anti-cheating filter | Broadened check to `grep -RniE 'sorry\|admit\|axiom' Solvent/`. |

---

## Architectural & Semantic Division of Labor

```text
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                                   SOLVENT PROOFS                                       │
├───────────────────────────────────────────┬────────────────────────────────────────────┤
│           LEAN 4 FORMAL MODEL             │         COCKROACHDB ENGINE (D3/D4)         │
├───────────────────────────────────────────┼────────────────────────────────────────────┤
│ • Proves SAFE state transitions preserve  │ • Proves UNSAFE state transitions are      │
│   ValidLedger invariants                  │   structurally refused by constraints      │
│ • Machine-checked preservation theorems   │ • Engine SQLSTATE receipts:                │
│ • Proves impossibility of live authority  │   - 23503 · gate                           │
│   on retracted / non-promoted beliefs     │   - 23514 · live_requires_promoted         │
│ • Abstract mathematical state machine     │   - 23514 · promoted_is_debt_free          │
└───────────────────────────────────────────┴────────────────────────────────────────────┘
```

> [!IMPORTANT]
> The Lean model proves that the defined state machine transitions preserve `ValidLedger`. It does **not** claim to prove CockroachDB's engine refusal semantics or provide a refinement proof of the Go implementation.

---

## Proposed Changes

All files will be created under `formal/lean/`, plus a minor documentation section in the root `README.md`. Zero Go or SQL production code is touched.

```
formal/lean/
├── lean-toolchain
├── lakefile.lean
├── README.md
└── Solvent/
    ├── Types.lean
    ├── Invariants.lean
    ├── Transitions.lean
    ├── Preservation.lean
    └── Examples.lean
```

---

### 1. Lake Project Setup

#### [NEW] [formal/lean/lean-toolchain](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/lean-toolchain)
- Exact contents: `leanprover/lean4:v4.33.0`

#### [NEW] [formal/lean/lakefile.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/lakefile.lean)
- Lake configuration with pinned Mathlib dependency:
  ```lean
  import Lake
  open Lake DSL

  package «Solvent» where

  require mathlib from git
    "https://github.com/leanprover-community/mathlib4.git" @ "db584cd6d46c92f209a44c0f1c829460d327499d"

  @[default_target]
  lean_lib «Solvent» where
  ```

#### [NEW] [formal/lean/README.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/README.md)
- Explains the purpose, scope, and boundaries of the formalization.
- Documents the exact theorems proved without `sorry`, `admit`, or `axiom`.
- Explicitly documents the division between Lean preservation proofs and CockroachDB engine refusal proofs.
- Build instructions: `cd formal/lean && lake build`.

---

### 2. Core Vocabulary & Model

#### [NEW] [formal/lean/Solvent/Types.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Types.lean)

```lean
inductive BeliefStatus
  | entered
  | promoted
  | retracted
  deriving DecidableEq, Repr

inductive IntentState
  | live
  | cancelled
  | executed
  deriving DecidableEq, Repr

structure DebtItem where
  id : Nat
  deriving DecidableEq, Repr

abbrev BeliefId := Nat
abbrev IntentId := Nat

inductive EdgeKind
  | derives
  | contradicts
  deriving DecidableEq, Repr

structure Belief where
  status : BeliefStatus
  debt : List DebtItem
  finalTruth : Bool
  deriving Repr

structure ActionIntent where
  state : IntentState
  beliefId : BeliefId
  beliefStatus : BeliefStatus
  deriving Repr

structure Ledger where
  beliefs : BeliefId → Option Belief
  intents : IntentId → Option ActionIntent
  edges : List (BeliefId × BeliefId × EdgeKind)
```

Correspondence to SQL schema:
- `BeliefStatus` $\leftrightarrow$ `belief.status CHECK (...)`
- `IntentState` $\leftrightarrow$ `action_intent.state CHECK (...)`
- `Belief.debt` $\leftrightarrow$ `belief.debt TEXT[]`
- `Belief.finalTruth` $\leftrightarrow$ `belief.final_truth BOOLEAN`
- `ActionIntent.beliefStatus` $\leftrightarrow$ `action_intent.belief_status`
- `Ledger.edges` $\leftrightarrow$ `belief_edge` table

---

### 3. Invariants

#### [NEW] [formal/lean/Solvent/Invariants.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Invariants.lean)

Four formal invariant definitions corresponding directly to the CockroachDB table constraints:

1. **`promoted_is_debt_free`** (`promoted_is_debt_free CHECK`):
   ```lean
   def promoted_is_debt_free (l : Ledger) : Prop :=
     ∀ bid b,
       l.beliefs bid = some b →
       b.status = BeliefStatus.promoted →
       b.debt = [] ∧ b.finalTruth = false
   ```

2. **`gate`** (`gate FOREIGN KEY ... ON UPDATE CASCADE`):
   Keyed lookup ensuring intent's mirrored `beliefStatus` equals the referenced belief's status:
   ```lean
   def gate (l : Ledger) : Prop :=
     ∀ iid intent b,
       l.intents iid = some intent →
       l.beliefs intent.beliefId = some b →
       intent.beliefStatus = b.status
   ```

3. **`live_requires_promoted`** (`live_requires_promoted CHECK`):
   ```lean
   def live_requires_promoted (l : Ledger) : Prop :=
     ∀ iid intent,
       l.intents iid = some intent →
       intent.state = IntentState.live →
       intent.beliefStatus = BeliefStatus.promoted ∧
       ∃ b, l.beliefs intent.beliefId = some b ∧ b.status = BeliefStatus.promoted
   ```

4. **`ValidLedger`**:
   ```lean
   def ValidLedger (l : Ledger) : Prop :=
     promoted_is_debt_free l ∧ gate l ∧ live_requires_promoted l
   ```

---

### 4. Transitions (Non-Tautological)

#### [NEW] [formal/lean/Solvent/Transitions.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Transitions.lean)

Four transitions aligned 1:1 with the Go kernel API (`Promote`, `IntentOnPromoted`, `RetractCascade`, intent cancellation). None of them inspect `ValidLedger` on their candidate output.

1. **`promote : Ledger → BeliefId → Option Ledger`**
   - Refuses if: belief missing, `b.status ≠ entered`, `b.debt ≠ []`, or `b.finalTruth = true`.
   - Updates `b.status := promoted`.
   - **FK Cascade:** Updates `intent.beliefStatus := promoted` for all `intent`s where `intent.beliefId = bid`.

2. **`cancelIntent : Ledger → IntentId → Option Ledger`**
   - Refuses if: intent missing or `intent.state ≠ live`.
   - Updates `intent.state := cancelled`.

3. **`authorizeIntent : Ledger → IntentId → Option Ledger`**
   - Refuses if: intent missing, referenced belief missing, or referenced `b.status ≠ promoted`.
   - Updates `intent.state := live` and `intent.beliefStatus := promoted`.

4. **`retractCascade : Ledger → BeliefId → Option Ledger`**
   - Refuses if: target belief missing.
   - Computes `affectedBeliefs := target + derives descendants`.
   - Cancels affected live intents (`state := cancelled`) and updates their `beliefStatus := retracted`.
   - Retracts affected beliefs (`status := retracted`).

---

### 5. Proofs: Preservation, Impossibility & Cascade

#### [NEW] [formal/lean/Solvent/Preservation.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Preservation.lean)

#### Part 1: Four Preservation Theorems
Non-tautological proofs verifying that state transitions preserve `ValidLedger`:
```lean
theorem promote_preserves_validity :
  ∀ l bid l', ValidLedger l → promote l bid = some l' → ValidLedger l'

theorem cancelIntent_preserves_validity :
  ∀ l iid l', ValidLedger l → cancelIntent l iid = some l' → ValidLedger l'

theorem authorizeIntent_preserves_validity :
  ∀ l iid l', ValidLedger l → authorizeIntent l iid = some l' → ValidLedger l'

theorem retractCascade_preserves_validity :
  ∀ l bid l', ValidLedger l → retractCascade l bid = some l' → ValidLedger l'
```

#### Part 2: Authority Impossibility Theorems (Solvent Thesis)
Proves that live authority cannot exist without a currently promoted underlying belief:
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

#### Part 3: Layered Promotion and Cascade Theorems
Layered proofs connecting local status propagation to global invariant satisfaction:

- **Promotion Propagation:**
  ```lean
  theorem promotion_updates_dependent_intent :
    ValidLedger l →
    promote l bid = some l' →
    l'.intents iid = some intent →
    intent.beliefId = bid →
    intent.beliefStatus = BeliefStatus.promoted
  ```

- **Cascade Layer 1 (Status propagation):**
  ```lean
  theorem cascade_retraction_updates_dependent_intent :
    ValidLedger l →
    retractCascade l bid = some l' →
    l'.intents iid = some intent →
    intent.beliefId = bid' →
    bid' ∈ affectedBeliefs l bid →
    intent.beliefStatus = BeliefStatus.retracted
  ```

- **Cascade Layer 2 (Liveness cancellation):**
  ```lean
  theorem cascade_retraction_cannot_leave_live_intent :
    ValidLedger l →
    retractCascade l bid = some l' →
    l'.intents iid = some intent →
    intent.beliefId = bid' →
    bid' ∈ affectedBeliefs l bid →
    intent.state ≠ IntentState.live
  ```

- **Cascade Layer 3 (Global consequences):**
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

---

### 6. Worked Examples

#### [NEW] [formal/lean/Solvent/Examples.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Examples.lean)
- Executable `#check`, `example`, and `#eval` demonstrations covering:
  1. Empty ledger is valid.
  2. Entered belief with debt is valid.
  3. Promoted debt-free belief is valid.
  4. Live intent on promoted belief is valid.
  5. Live intent on entered belief is invalid.
  6. Live intent on retracted belief is impossible.

---

### 7. Root Documentation

#### [MODIFY] [README.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/README.md)
Add the concise formal model section:
```markdown
## Formal model

`formal/lean/` contains a Lean 4 formal model of the belief/authority
state machine and proofs that the core invariants are preserved by
successful state transitions.

This is a formalization of the abstract Solvent semantics, not a
machine-checked refinement proof of the Go/CockroachDB implementation.
```

---

## Verification Plan

### Automated Verification
```bash
# 1. Build Lean project and check all proofs
cd /home/chaschel/Desktop/cockroach/hackathon/go/formal/lean
lake build

# 2. Strict anti-cheating check: verify zero sorry, admit, or axiom usage
grep -RniE 'sorry|admit|axiom' Solvent/ || echo "PASS: zero sorry/admit/axiom found"

# 3. Verify production Go and SQL code are unmodified and tests pass
cd /home/chaschel/Desktop/cockroach/hackathon/go
go build ./...
go vet ./...
task test
```

### Manual Verification
- Verify all theorem names match `plan3_review.md` and `plan4.md`.
- Verify clear comments distinguishing Lean abstract preservation proofs from CockroachDB engine refusal semantics.
- Verify zero modifications to Go or SQL code.
