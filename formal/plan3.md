# Lean 4 Formal Model of Solvent's State Machine (plan2_review.md Implementation)

This plan implements the Lean 4 formal model and proofs for Solvent's belief and authority state machine, strictly following [plan2_review.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/plan2_review.md), [plan2.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/plan2.md), and [AGENTS.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/AGENTS.md).

The artifact establishes machine-checked proofs that the core authority invariants are preserved by valid state transitions in the abstract state machine, complementing CockroachDB's empirical engine-enforced refusal proofs.

## User Review Required

> [!IMPORTANT]
> **Key Decisions Incorporated from `plan2_review.md`:**
> 1. **Four Transitions Only (Standalone `retract` excluded):** Only `promote`, `cancelIntent`, `authorizeIntent`, and `retractCascade` are modeled, aligning 1:1 with the Go kernel API.
> 2. **Explicit Separation of Responsibilities:**
>    - **Lean:** Proves that the *safe* state transitions (e.g., `retractCascade`) preserve `ValidLedger`.
>    - **CockroachDB (D3/D4):** Proves that *unsafe* orderings are structurally refused by engine constraints (`23503 · gate`, `23514 · live_requires_promoted`).
> 3. **Non-Tautological Transitions:** Transitions refuse exclusively on domain preconditions (no `ValidLedger` postcondition checks).
> 4. **Layered Cascade Proofs:** Local status propagation $\to$ local liveness impossibility $\to$ global invariant preservation.
> 5. **Toolchain Pinning:** `leanprover/lean4:v4.33.0`.
> 6. **Zero Production Code Changes:** No Go or SQL files modified (only root `README.md` addition and new files under `formal/lean/`).

## Open Questions

> [!NOTE]
> None. All review comments and architectural constraints have been fully addressed and specified.

---

## Proposed Changes

All files will be created under `formal/lean/`, plus a minor documentation section in the root `README.md`.

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

### Lean Project Setup

#### [NEW] [lean-toolchain](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/lean-toolchain)
- Pins Lean toolchain to `leanprover/lean4:v4.33.0`.

#### [NEW] [lakefile.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/lakefile.lean)
- Defines package `Solvent` with library target `Solvent`.
- Configured for Lake 5.0.0 with pinned dependency resolution.

#### [NEW] [formal/lean/README.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/README.md)
- Explains the purpose, scope, and boundaries of the formalization.
- Documents the exact theorems proved without `sorry`.
- Clarifies the division of labor between Lean (abstract safety preservation) and CockroachDB (engine refusal).

---

### Core Vocabulary & Model

#### [NEW] [Solvent/Types.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Types.lean)
- Mirrors SQL schema and Go domain models:
  - `inductive BeliefStatus` (`entered`, `promoted`, `retracted`)
  - `inductive IntentState` (`live`, `cancelled`, `executed`)
  - `structure DebtItem` (`id : Nat`)
  - `abbrev BeliefId := Nat`, `abbrev IntentId := Nat`
  - `inductive EdgeKind` (`derives`, `contradicts`)
  - `structure Belief` (`status : BeliefStatus`, `debt : List DebtItem`, `finalTruth : Bool`)
  - `structure ActionIntent` (`state : IntentState`, `beliefId : BeliefId`, `beliefStatus : BeliefStatus`)
  - `structure Ledger` (`beliefs : BeliefId → Option Belief`, `intents : IntentId → Option ActionIntent`, `edges : List (BeliefId × BeliefId × EdgeKind)`)

---

### Invariants

#### [NEW] [Solvent/Invariants.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Invariants.lean)
- Formal definitions matching CockroachDB table constraints:
  - `promoted_is_debt_free`: $\forall b \in \text{beliefs}, b.\text{status} = \text{promoted} \implies b.\text{debt} = [] \wedge b.\text{finalTruth} = \text{false}$
  - `gate`: $\forall i \in \text{intents}, \forall b \in \text{beliefs}, (i.\text{beliefId} = b.\text{id}) \implies i.\text{beliefStatus} = b.\text{status}$
  - `live_requires_promoted`: $\forall i \in \text{intents}, i.\text{state} = \text{live} \implies (i.\text{beliefStatus} = \text{promoted} \wedge \exists b, l.\text{beliefs}(i.\text{beliefId}) = \text{some } b \wedge b.\text{status} = \text{promoted})$
  - `ValidLedger l := promoted_is_debt_free l ∧ gate l ∧ live_requires_promoted l`

---

### Transitions (Non-Tautological)

#### [NEW] [Solvent/Transitions.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Transitions.lean)
- Domain-precondition transitions returning `Option Ledger`:
  1. `promote : Ledger → BeliefId → Option Ledger`
     - Refuses if belief missing, status $\neq$ `entered`, debt $\neq []$, or `finalTruth = true`.
  2. `cancelIntent : Ledger → IntentId → Option Ledger`
     - Refuses if intent missing or status $\neq$ `live`. Sets intent state to `cancelled`.
  3. `authorizeIntent : Ledger → IntentId → Option Ledger`
     - Refuses if intent missing, belief missing, or belief status $\neq$ `promoted`. Updates intent's `beliefStatus` to `promoted`.
  4. `retractCascade : Ledger → BeliefId → Option Ledger`
     - Computes transitive `derives` descendants (`affectedBeliefs`).
     - Cancels all live intents referencing affected beliefs and sets their `beliefStatus` to `retracted`.
     - Sets status of affected beliefs to `retracted`.
     - *No postcondition `ValidLedger` check.*

---

### Proofs: Preservation, Impossibility & Cascade

#### [NEW] [Solvent/Preservation.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Preservation.lean)

- **Part 1: Four State Machine Preservation Theorems**
  - `theorem promote_preserves_validity : ∀ l bid l', ValidLedger l → promote l bid = some l' → ValidLedger l'`
  - `theorem cancelIntent_preserves_validity : ∀ l iid l', ValidLedger l → cancelIntent l iid = some l' → ValidLedger l'`
  - `theorem authorizeIntent_preserves_validity : ∀ l iid l', ValidLedger l → authorizeIntent l iid = some l' → ValidLedger l'`
  - `theorem retractCascade_preserves_validity : ∀ l bid l', ValidLedger l → retractCascade l bid = some l' → ValidLedger l'`

- **Part 2: Authority Impossibility Theorems (Solvent Thesis)**
  - `theorem live_intent_implies_promoted : ValidLedger l → l.intents iid = some intent → intent.state = IntentState.live → ∃ belief, l.beliefs intent.beliefId = some belief ∧ belief.status = BeliefStatus.promoted`
  - `theorem no_live_intent_on_retracted_belief : ValidLedger l → l.intents iid = some intent → intent.state = IntentState.live → l.beliefs intent.beliefId = some belief → belief.status = BeliefStatus.retracted → False`

- **Part 3: Layered Cascade Theorems**
  - *Layer 1 (Status propagation):* `theorem cascade_retraction_updates_dependent_intent`
  - *Layer 2 (Liveness cancellation):* `theorem cascade_retraction_cannot_leave_live_intent`
  - *Layer 3 (Global consistency):* `theorem cascade_update_preserves_gate` and `theorem cascade_retraction_blocks_live_intent`

---

### Worked Examples

#### [NEW] [Solvent/Examples.lean](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Examples.lean)
- Executable `#check`, `example`, and `#eval` demonstrations verifying:
  - Empty ledger validity.
  - Entered belief with debt validity.
  - Promoted debt-free belief validity.
  - Live intent on promoted belief validity.
  - Live intent on entered/retracted belief invalidity.

---

### Root Documentation

#### [MODIFY] [README.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/README.md)
- Adds the concise `## Formal model` section pointing to `formal/lean/`.

---

## Verification Plan

### Automated Verification
```bash
# 1. Build Lean project and verify all proofs check without sorry
cd /home/chaschel/Desktop/cockroach/hackathon/go/formal/lean
lake build

# 2. Verify no sorry or cheat axioms exist in the Lean codebase
grep -rn "sorry" Solvent/ || echo "PASS: zero sorry found"

# 3. Verify production Go and SQL code are unmodified and tests pass
cd /home/chaschel/Desktop/cockroach/hackathon/go
go build ./...
go vet ./...
task test
```

### Manual Verification
- Verify all theorem names match `plan2_review.md` and `plan2.md`.
- Verify clear theorem commentary distinguishing Lean abstract proofs from CockroachDB refusal semantics.
