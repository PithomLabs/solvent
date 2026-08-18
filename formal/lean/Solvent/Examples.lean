/-
  Solvent: A Transactional Belief Ledger for Autonomous Agents
  Formal Model in Lean 4 — Worked Examples

  This module demonstrates the semantic definitions and theorems with concrete ledger states:
  1. Empty ledger is valid.
  2. Entered belief with debt is valid.
  3. Promoted debt-free belief is valid.
  4. Live intent on promoted belief is valid.
  5. Live intent on entered belief is invalid.
  6. Live intent on retracted belief is impossible.
-/

import Solvent.Types
import Solvent.Invariants
import Solvent.Transitions
import Solvent.Preservation

namespace Solvent.Examples

open Solvent

/-- Example 1: The empty ledger is trivially valid. -/
def emptyLedger : Ledger :=
  { beliefs := fun _ => none,
    intents := fun _ => none,
    edges := [] }

theorem emptyLedger_isValid : ValidLedger emptyLedger := by
  refine ⟨?_, ?_, ?_⟩
  · intro _ _ h; contradiction
  · intro _ _ _ h; contradiction
  · intro _ _ h; contradiction

/-- Example 2: An entered belief carrying debt is valid (debt only blocks promotion). -/
def ledgerWithDebt : Ledger :=
  { beliefs := fun id => if id == 1 then some { status := BeliefStatus.entered, debt := [⟨101⟩], finalTruth := false } else none,
    intents := fun _ => none,
    edges := [] }

theorem ledgerWithDebt_isValid : ValidLedger ledgerWithDebt := by
  refine ⟨?_, ?_, ?_⟩
  · intro bid b hb hprom
    dsimp [ledgerWithDebt] at hb
    split at hb
    · injection hb with hbeq
      subst hbeq
      dsimp at hprom
      contradiction
    · contradiction
  · intro _ _ _ h; contradiction
  · intro _ _ h; contradiction

/-- Example 3: A debt-free promoted belief is valid. -/
def ledgerWithPromoted : Ledger :=
  { beliefs := fun id => if id == 1 then some { status := BeliefStatus.promoted, debt := [], finalTruth := false } else none,
    intents := fun _ => none,
    edges := [] }

theorem ledgerWithPromoted_isValid : ValidLedger ledgerWithPromoted := by
  refine ⟨?_, ?_, ?_⟩
  · intro bid b hb hprom
    dsimp [ledgerWithPromoted] at hb
    split at hb
    · injection hb with hbeq
      subst hbeq
      dsimp
      exact ⟨rfl, rfl⟩
    · contradiction
  · intro _ _ _ h; contradiction
  · intro _ _ h; contradiction

/-- Example 4: A live intent attached to a promoted belief is valid. -/
def ledgerWithLiveIntent : Ledger :=
  { beliefs := fun id => if id == 1 then some { status := BeliefStatus.promoted, debt := [], finalTruth := false } else none,
    intents := fun id => if id == 100 then some { state := IntentState.live, beliefId := 1, beliefStatus := BeliefStatus.promoted } else none,
    edges := [] }

theorem ledgerWithLiveIntent_isValid : ValidLedger ledgerWithLiveIntent := by
  refine ⟨?_, ?_, ?_⟩
  · intro bid b hb hprom
    dsimp [ledgerWithLiveIntent] at hb
    split at hb
    · injection hb with hbeq
      subst hbeq
      exact ⟨rfl, rfl⟩
    · contradiction
  · intro iid intent b hintent hb
    dsimp [ledgerWithLiveIntent] at hintent hb
    split at hintent
    · injection hintent with hieq
      subst hieq
      dsimp at hb
      injection hb with hbeq
      subst hbeq
      rfl
    · contradiction
  · intro iid intent hintent hlive
    dsimp [ledgerWithLiveIntent] at hintent
    split at hintent
    · injection hintent with hieq
      subst hieq
      refine ⟨rfl, ?_⟩
      exact ⟨{ status := BeliefStatus.promoted, debt := [], finalTruth := false }, rfl, rfl⟩
    · contradiction

/-- Example 5: A live intent referencing an `entered` belief is INVALID. -/
def invalidEnteredLiveIntent : Ledger :=
  { beliefs := fun id => if id == 1 then some { status := BeliefStatus.entered, debt := [], finalTruth := false } else none,
    intents := fun id => if id == 100 then some { state := IntentState.live, beliefId := 1, beliefStatus := BeliefStatus.entered } else none,
    edges := [] }

theorem invalidEnteredLiveIntent_isInvalid : ¬ ValidLedger invalidEnteredLiveIntent := by
  intro hval
  have hlrp := hval.2.2 100 { state := IntentState.live, beliefId := 1, beliefStatus := BeliefStatus.entered } (by rfl) (by rfl)
  have hstatus := hlrp.1
  contradiction

/-- Example 6: A live intent referencing a `retracted` belief is IMPOSSIBLE. -/
def invalidRetractedLiveIntent : Ledger :=
  { beliefs := fun id => if id == 1 then some { status := BeliefStatus.retracted, debt := [], finalTruth := false } else none,
    intents := fun id => if id == 100 then some { state := IntentState.live, beliefId := 1, beliefStatus := BeliefStatus.retracted } else none,
    edges := [] }

theorem invalidRetractedLiveIntent_isInvalid : ¬ ValidLedger invalidRetractedLiveIntent := by
  intro hval
  exact no_live_intent_on_retracted_belief
    invalidRetractedLiveIntent 100
    { state := IntentState.live, beliefId := 1, beliefStatus := BeliefStatus.retracted }
    { status := BeliefStatus.retracted, debt := [], finalTruth := false }
    hval rfl rfl rfl rfl

end Solvent.Examples
