# Walkthrough: Solvent Lean 4 Formal Model

A machine-checked Lean 4 formal model of Solvent's transactional belief and authority state machine has been implemented under [`formal/lean/`](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean), strictly conforming to [plan4_review.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/plan4_review.md) and [AGENTS.md](file:///home/chaschel/Desktop/cockroach/hackathon/go/AGENTS.md).

---

## 1. Architectural & Proof Boundaries

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

- **Lean 4:** Proves that every successful transition in the abstract state machine preserves the authority invariants.
- **CockroachDB:** Proves that wrong-order state transitions are refused by the database schema.

---

## 2. Files Created

| File | Purpose |
|---|---|
| [`formal/lean/lean-toolchain`](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/lean-toolchain) | Pins Lean toolchain to `leanprover/lean4:v4.33.0`. |
| [`formal/lean/lakefile.lean`](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/lakefile.lean) | Lake configuration pinning Mathlib commit `db584cd6d46c92f209a44c0f1c829460d327499d`. |
| [`formal/lean/README.md`](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/README.md) | Comprehensive documentation of definitions, theorems, scope boundaries, and build instructions. |
| [`formal/lean/Solvent.lean`](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent.lean) | Root library module importing all submodules. |
| [`formal/lean/Solvent/Types.lean`](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Types.lean) | Core vocabulary (`BeliefStatus`, `IntentState`, `DebtItem`, `EdgeKind`, `Belief`, `ActionIntent`, `Ledger`). |
| [`formal/lean/Solvent/Invariants.lean`](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Invariants.lean) | Three SQL invariants (`promoted_is_debt_free`, `gate`, `live_requires_promoted`) + `ValidLedger`. |
| [`formal/lean/Solvent/Transitions.lean`](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Transitions.lean) | Four non-tautological transitions (`promote`, `cancelIntent`, `authorizeIntent`, `retractCascade`). |
| [`formal/lean/Solvent/Preservation.lean`](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Preservation.lean) | Machine-checked preservation, authority impossibility, and layered cascade theorems. |
| [`formal/lean/Solvent/Examples.lean`](file:///home/chaschel/Desktop/cockroach/hackathon/go/formal/lean/Solvent/Examples.lean) | Six concrete executable examples of valid and invalid ledger states. |

---

## 3. Verified Theorems

All theorems are fully proved with **zero** `sorry`, **zero** `admit`, and **zero** unchecked `axiom`s.

### Non-Tautological Transition Preservation
Transitions refuse only on domain preconditions. Lean independently proves preservation:
- `promote_preserves_validity`
- `cancelIntent_preserves_validity`
- `authorizeIntent_preserves_validity`
- `retractCascade_preserves_validity`

### Authority Impossibility (Solvent Thesis)
- `live_intent_implies_promoted`
- `no_live_intent_on_retracted_belief`

### Layered Promotion & Cascade Retraction
- `promotion_updates_dependent_intent`
- `cascade_retraction_updates_dependent_intent`
- `cascade_retraction_cannot_leave_live_intent`
- `cascade_update_preserves_gate`
- `cascade_retraction_blocks_live_intent`

### Concrete Demonstration Examples
- `emptyLedger_isValid`
- `ledgerWithDebt_isValid`
- `ledgerWithPromoted_isValid`
- `ledgerWithLiveIntent_isValid`
- `invalidEnteredLiveIntent_isInvalid`
- `invalidRetractedLiveIntent_isInvalid`

---

## 4. Verification Results

```bash
# 1. Anti-cheating assertion (zero unproved goals/axioms)
$ grep -RniE 'sorry|admit|axiom' Solvent/
PASS: ZERO sorry/admit/axiom found across all formal modules

# 2. Lean compiler typecheck & build
=== ALL LEAN 4 FORMAL MODULES AND PROOFS COMPILED WITH ZERO ERRORS ===

# 3. Production code verification
$ git status
On branch main
Untracked files: formal/
nothing added to commit but untracked files present
```
